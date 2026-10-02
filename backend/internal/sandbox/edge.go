package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Load balancing algorithms.
const (
	AlgoRoundRobin = "round-robin"
	AlgoLeastConn  = "least-connections"
)

// LBConfig is a load balancer's configuration (v13, ADR-0025).
type LBConfig struct {
	Algorithm string `json:"algorithm"`
	// Without health checks a failed target keeps receiving its share.
	HealthChecks bool `json:"healthChecks"`
}

// GatewayRoute sends the endpoints under a path prefix to a service.
type GatewayRoute struct {
	Prefix  string `json:"prefix"`
	Service string `json:"service"`
}

// GatewayConfig is an API gateway's configuration (v13).
type GatewayConfig struct {
	Routes []GatewayRoute `json:"routes"`
	// Auth checks every request at the gateway, for a little latency.
	Auth bool `json:"auth"`
	// RateLimitRPS answers requests above it with 429; 0 is no limit.
	RateLimitRPS float64 `json:"rateLimitRps,omitempty"`
}

// CDNConfig is a CDN's configuration (v13).
type CDNConfig struct {
	// Cacheable lists the endpoints the edge may answer; none means every
	// GET endpoint.
	Cacheable  []string `json:"cacheable,omitempty"`
	TTLSeconds float64  `json:"ttlSeconds"`
	// ObjectsPerEndpoint distinct responses each cacheable endpoint has;
	// ObjectKB their mean size.
	ObjectsPerEndpoint float64 `json:"objectsPerEndpoint"`
	ObjectKB           float64 `json:"objectKb"`
}

// EdgeRuntime holds the constants of the edge model.
type EdgeRuntime struct {
	AuthMs float64 `json:"authMs"`
	// A CDN answers a hit in HitMs; it costs per GB served and per 10,000
	// requests.
	HitMs       float64 `json:"hitMs"`
	CDNGB       float64 `json:"cdnGb"`
	CDNPer10k   float64 `json:"cdnPer10k"`
	CDNCapacity float64 `json:"cdnCapacity"`
}

// TargetShare is a target's share of what a balancer or gateway sends.
type TargetShare struct {
	Node    string  `json:"node"`
	RPS     float64 `json:"rps"`
	Share   float64 `json:"share"`
	Healthy bool    `json:"healthy"`
}

// EdgeNodeStats is what a load balancer, gateway, or CDN did in a tick.
type EdgeNodeStats struct {
	Health  string        `json:"health"`
	Targets []TargetShare `json:"targets,omitempty"`
	// A gateway's unrouted (404) and rate-limited (429) requests per second.
	NotFound float64 `json:"notFound,omitempty"`
	Limited  float64 `json:"limited,omitempty"`
	// A CDN's hits and misses per second, egress, and cost per hour.
	Hits       float64 `json:"hits,omitempty"`
	Misses     float64 `json:"misses,omitempty"`
	HitRatio   float64 `json:"hitRatio,omitempty"`
	EgressMbps float64 `json:"egressMbps,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
}

// fwd is how an edge node forwarded one endpoint this solve: the share it
// answered itself (a CDN hit) or turned away (no route), and where the rest
// went.
type fwd struct {
	hit, lost float64
	to        []route
}

func (g *Game) edgeModel(n *Node) bool {
	return g.Rules.LB != nil && (n.Kind == KindLB || n.Kind == KindGateway || n.Kind == KindCDN)
}

func (g *Game) lbConfig(n *Node) *LBConfig {
	if n.LB != nil {
		return n.LB
	}
	return g.Rules.LB
}

func (g *Game) gatewayConfig(n *Node) *GatewayConfig {
	if n.Gateway != nil {
		return n.Gateway
	}
	return g.Rules.Gateway
}

func (g *Game) cdnConfig(n *Node) *CDNConfig {
	if n.CDN != nil {
		return n.CDN
	}
	return g.Rules.CDN
}

// lbShares splits a balancer's load over its targets. Round robin gives each
// replica the same; least connections follows capacity. With health checks a
// failed target gets nothing; without, it keeps its share and fails it.
func (g *Game) lbShares(n *Node, targets []int) []route {
	cfg := g.lbConfig(n)
	w := make([]float64, len(targets))
	sum, alive, aliveSum := 0.0, 0, 0.0
	for k, t := range targets {
		tn := g.Nodes[t]
		c := g.capacity(tn)
		if cfg.Algorithm == AlgoRoundRobin {
			w[k] = float64(g.upReplicas(tn))
			if !cfg.HealthChecks {
				w[k] = float64(tn.Replicas)
			}
		} else {
			w[k] = c
		}
		if c > 0 {
			alive++
			aliveSum += w[k]
		}
	}
	for k, t := range targets {
		// A failed target looks idle to least connections: without health
		// checks it draws an average share.
		if !cfg.HealthChecks && g.capacity(g.Nodes[t]) == 0 && cfg.Algorithm == AlgoLeastConn && alive > 0 {
			w[k] = aliveSum / float64(alive)
		}
		sum += w[k]
	}
	out := make([]route, len(targets))
	for k, t := range targets {
		share := 1 / float64(len(targets))
		if sum > 0 {
			share = w[k] / sum
		}
		out[k] = route{to: t, share: share}
	}
	return out
}

// gatewayService is the service a gateway routes an endpoint to: the route
// with the longest matching path prefix.
func gatewayService(cfg *GatewayConfig, endpoint string) (string, bool) {
	_, path, _ := strings.Cut(endpoint, " ")
	best, svc := -1, ""
	for _, r := range cfg.Routes {
		if strings.HasPrefix(path, r.Prefix) && len(r.Prefix) > best {
			best, svc = len(r.Prefix), r.Service
		}
	}
	return svc, best >= 0
}

// cdnHit is the share of an endpoint's requests a CDN answers: none unless
// it is cacheable, else the chance its response is still fresh.
func (g *Game) cdnHit(cfg *CDNConfig, endpoint string, rps float64) float64 {
	cacheable := strings.HasPrefix(endpoint, "GET ")
	if len(cfg.Cacheable) > 0 {
		cacheable = slices.Contains(cfg.Cacheable, endpoint)
	}
	if !cacheable || cfg.ObjectsPerEndpoint <= 0 {
		return 0
	}
	return 1 - math.Exp(-cfg.TTLSeconds*rps/cfg.ObjectsPerEndpoint)
}

// forward pushes edge node i's served load on, endpoint by endpoint: a
// balancer by its algorithm, a gateway by path to the named service, a CDN
// its misses to the origin. keep is the share of arriving load it served.
func (g *Game) forward(i int, keep, a float64, load []vec, attack []float64) {
	n := g.Nodes[i]
	k, _ := g.Rules.Kind(n.Kind)
	plan := map[string]fwd{}
	st := &EdgeNodeStats{}
	g.edgeRuns[i] = st
	names := make([]string, 0, len(g.epLoad[i]))
	for name := range g.epLoad[i] {
		names = append(names, name)
	}
	slices.Sort(names)
	targets := g.targets(i, k.ConnectsTo...)
	sent := map[int]float64{}
	for _, name := range names {
		v := g.epLoad[i][name] * keep
		f := fwd{}
		switch n.Kind {
		case KindLB:
			f.to = g.lbShares(n, targets)
		case KindGateway:
			svc, ok := gatewayService(g.gatewayConfig(n), name)
			var to []int
			for _, t := range targets {
				if tn := g.Nodes[t]; ok && (tn.Kind != KindApp || g.appConfig(tn).Name == svc) {
					to = append(to, t)
				}
			}
			if len(to) == 0 {
				f.lost = 1
				st.NotFound += v
			}
			f.to = g.split(to)
		case KindCDN:
			f.hit = g.cdnHit(g.cdnConfig(n), name, v)
			f.to = g.split(targets)
			st.Hits += v * f.hit
			st.Misses += v * (1 - f.hit)
		}
		plan[name] = f
		rest := v * (1 - f.hit - f.lost)
		for _, rt := range f.to {
			w := rest * rt.share
			if g.problem[[2]int{i, rt.to}] == "" {
				load[rt.to][endpointClass(name)] += w
				attack[rt.to] += w * a
				g.epLoad[rt.to][name] += w
			}
			g.edgeStat(i, rt.to).RPS += w
			sent[rt.to] += w
		}
	}
	g.fwds[i] = plan
	for _, t := range targets {
		st.Targets = append(st.Targets, TargetShare{Node: g.Nodes[t].ID, RPS: sent[t], Healthy: g.capacity(g.Nodes[t]) > 0})
	}
	total := 0.0
	for _, t := range targets {
		total += sent[t]
	}
	for k := range st.Targets {
		if total > 0 {
			st.Targets[k].Share = st.Targets[k].RPS / total
		}
	}
}

// epOutcome is the chance a request for an endpoint entering node j
// succeeds, and its latency: an application's from its route, an edge
// node's through its forwarding, once node j is solved.
func (g *Game) epOutcome(j int, name string) (float64, float64) {
	if run := g.runs[j]; run != nil {
		o := run.out[name]
		return o[0], o[1]
	}
	if o, ok := g.epOut[j][name]; ok {
		return o[0], o[1]
	}
	return 0, 0
}

// finishEdge solves edge node i per endpoint, given the share of its load
// it served (frac) and its own latency, and returns its success and latency
// per request class, weighted by what arrived.
func (g *Game) finishEdge(i int, frac, own float64) (vec, vec) {
	n := g.Nodes[i]
	rt := g.Rules.EdgeRuntime
	out := map[string][2]float64{}
	var okC, latC, total vec
	names := make([]string, 0, len(g.fwds[i]))
	for name := range g.fwds[i] {
		names = append(names, name)
	}
	// In name order, so the sums replay exactly.
	slices.Sort(names)
	for _, name := range names {
		f := g.fwds[i][name]
		p, ms := 0.0, 0.0
		if f.hit > 0 {
			p, ms = f.hit, f.hit*rt.HitMs
		}
		for _, x := range f.to {
			if g.problem[[2]int{i, x.to}] != "" {
				continue
			}
			tp, tms := g.epOutcome(x.to, name)
			w := (1 - f.hit - f.lost) * x.share
			p += w * tp
			ms += w * tp * tms
		}
		if p > 0 {
			ms /= p
		}
		p *= frac
		out[name] = [2]float64{p, own + ms}
		c := endpointClass(name)
		v := g.epLoad[i][name]
		okC[c] += v * p
		latC[c] += v * p * (own + ms)
		total[c] += v
	}
	g.epOut[i] = out
	var s, t vec
	for c := range nClass {
		if okC[c] > 0 {
			t[c] = latC[c] / okC[c]
		}
		if total[c] > 0 {
			s[c] = okC[c] / total[c]
		}
	}
	st := g.edgeRuns[i]
	if n.Kind == KindCDN {
		cfg := g.cdnConfig(n)
		served := st.Hits + st.Misses
		if served > 0 {
			st.HitRatio = st.Hits / served
		}
		st.EgressMbps = served * cfg.ObjectKB * 8 / 1000
		gb := served * cfg.ObjectKB / 1024 / 1024 * 3600
		st.Cost = (gb*rt.CDNGB + served*3600/10_000*rt.CDNPer10k) * factor(g.fx.cost, n.Kind)
	}
	fail := 1.0
	if all := total.sum(); all > 0 {
		fail = 1 - (okC.sum() / all)
	}
	switch {
	case n.Down:
		st.Health = HealthStopped
	case total.sum() > 0 && fail > 0.2:
		st.Health = HealthUnhealthy
	case total.sum() > 0 && fail > 0.01:
		st.Health = HealthDegraded
	default:
		st.Health = HealthHealthy
	}
	return s, t
}

func (g *Game) configureEdge(n *Node, c Command) error {
	var p []string
	switch {
	case n.Kind == KindLB && c.LB != nil:
		p = g.Rules.ValidateLB(*c.LB)
		if len(p) == 0 {
			n.LB = c.LB
		}
	case n.Kind == KindGateway && c.Gateway != nil:
		p = g.Rules.ValidateGateway(*c.Gateway)
		if len(p) == 0 {
			n.Gateway = c.Gateway
		}
	case n.Kind == KindCDN && c.CDN != nil:
		p = g.Rules.ValidateCDN(*c.CDN)
		if len(p) == 0 {
			n.CDN = c.CDN
		}
	default:
		return invalid("configure needs a %s configuration", n.Kind)
	}
	if len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	return nil
}

// ValidateLB lists every problem with a load balancer's configuration.
func (r *Ruleset) ValidateLB(c LBConfig) []string {
	if c.Algorithm != AlgoRoundRobin && c.Algorithm != AlgoLeastConn {
		return []string{fmt.Sprintf("algorithm must be %q or %q", AlgoRoundRobin, AlgoLeastConn)}
	}
	return nil
}

// ValidateGateway lists every problem with a gateway's configuration.
func (r *Ruleset) ValidateGateway(c GatewayConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if len(c.Routes) == 0 || len(c.Routes) > maxRoutes {
		bad("declare 1 to %d routes", maxRoutes)
	}
	seen := map[string]bool{}
	for k, rt := range c.Routes {
		switch {
		case !strings.HasPrefix(rt.Prefix, "/") || strings.ContainsAny(rt.Prefix, " \t"):
			bad("route %d: prefix must start with / and have no spaces", k+1)
		case seen[rt.Prefix]:
			bad("prefix %s is routed twice", rt.Prefix)
		}
		seen[rt.Prefix] = true
		if name := strings.TrimSpace(rt.Service); name == "" || len(name) > maxNameLen {
			bad("route %d: name a service", k+1)
		}
	}
	if !(c.RateLimitRPS >= 0 && c.RateLimitRPS <= r.MaxTrafficRPS) {
		bad("the rate limit must be between 0 and %s requests/s", trimFloat(r.MaxTrafficRPS))
	}
	return p
}

// ValidateCDN lists every problem with a CDN's configuration.
func (r *Ruleset) ValidateCDN(c CDNConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	for k, e := range c.Cacheable {
		if !validEndpoint(e) {
			bad("cacheable endpoint %d must be like \"GET /path\"", k+1)
		}
	}
	if !(c.TTLSeconds >= 1 && c.TTLSeconds <= 365*86400) {
		bad("TTL must be between 1 s and a year")
	}
	if !(c.ObjectsPerEndpoint >= 1 && c.ObjectsPerEndpoint <= 1e9) {
		bad("objects per endpoint must be between 1 and 1000000000")
	}
	if !(c.ObjectKB >= 0.1 && c.ObjectKB <= 1e6) {
		bad("object size must be between 0.1 and 1000000 KB")
	}
	return p
}

// RulesetV13 is RulesetV12 with the load balancer, API gateway, and CDN back
// in the catalog, with contracts and configurations (Phase 11, ADR-0025).
func RulesetV13() *Ruleset {
	r := RulesetV12()
	r.Version = "sandbox/v13"
	v1 := RulesetV1()
	unlocks := map[string]string{KindLB: "first-request", KindGateway: "startup-tier", KindCDN: "scale-up-tier"}
	connects := map[string][]string{
		KindLB:      {KindApp, KindGateway},
		KindGateway: {KindApp, KindLB},
		KindCDN:     {KindLB, KindGateway, KindApp},
	}
	for _, name := range []string{KindCDN, KindLB, KindGateway} {
		k, _ := v1.Kind(name)
		k.UnlockedBy, k.ConnectsTo = unlocks[name], connects[name]
		if name == KindCDN {
			// A CDN is priced by use (ADR-0025).
			k.CostPerHour, k.Capacity = 0, 1e6
		}
		r.Kinds = append(r.Kinds, k)
		r.Listeners[name] = Listener{"HTTP/1.1", 443, true}
	}
	for i := range r.Kinds {
		if r.Kinds[i].Name == KindTraffic {
			r.Kinds[i].ConnectsTo = []string{KindApp, KindLB, KindGateway, KindCDN}
		}
	}
	r.LB = &LBConfig{Algorithm: AlgoLeastConn, HealthChecks: true}
	r.Gateway = &GatewayConfig{Routes: []GatewayRoute{{Prefix: "/", Service: r.App.Name}}}
	r.CDN = &CDNConfig{TTLSeconds: 300, ObjectsPerEndpoint: 1000, ObjectKB: 50}
	r.EdgeRuntime = EdgeRuntime{AuthMs: 2, HitMs: 10, CDNGB: 0.02, CDNPer10k: 0.0075, CDNCapacity: 1e6}
	return r
}
