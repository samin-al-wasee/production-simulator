package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Client types: who a traffic component's clients are (ADR-0018).
const (
	ClientWeb    = "web"
	ClientMobile = "mobile"
	ClientAPI    = "api"
	ClientBot    = "bot"
)

// Schemes a client can speak; https needs an application with TLS.
const (
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"
)

// ClientConfig is one traffic component: a single population of clients,
// where it is, how it connects, and what it asks for. It is replaced whole by
// a configure command.
type ClientConfig struct {
	Name       string  `json:"name"`
	ClientType string  `json:"clientType"`
	Region     string  `json:"region"`
	Protocol   string  `json:"protocol"`
	Scheme     string  `json:"scheme"`
	Port       int     `json:"port"`
	KeepAlive  bool    `json:"keepAlive"`
	TimeoutMs  float64 `json:"timeoutMs"`
	Retries    int     `json:"retries,omitempty"`
	Source     string  `json:"source"`
	// Pattern is a load test's volume; the market ignores it.
	Pattern Pattern `json:"pattern"`
	// Endpoints is the request mix, e.g. "GET /products" and its share.
	Endpoints []Weight `json:"endpoints"`
}

// ClientStats is what a traffic component sent and how it fared. Successes
// are requests after retries; failures are attempts, by reason.
type ClientStats struct {
	RPS       float64 `json:"rps"`
	RetryRPS  float64 `json:"retryRps,omitempty"`
	AttackRPS float64 `json:"attackRps,omitempty"`
	Success   float64 `json:"success"`
	// Refused attempts broke the contract and never reached the server.
	Refused  float64 `json:"refused,omitempty"`
	NotFound float64 `json:"notFound,omitempty"`
	Rejected float64 `json:"rejected,omitempty"`
	Timeouts float64 `json:"timeouts,omitempty"`
	Errors   float64 `json:"errors,omitempty"`
	// LatencyMs is the mean of successful attempts.
	LatencyMs   float64 `json:"latencyMs"`
	Concurrency float64 `json:"concurrency"`
	// Problem says why every request fails: not connected, or the contract.
	Problem string `json:"problem,omitempty"`
}

// clientLoad is a traffic component's requests during one solve.
type clientLoad struct {
	node     int
	cfg      *ClientConfig
	unique   float64 // requests per second
	attempts float64 // with retries
	attack   float64
	to       int // the application it sends to, or -1
	problem  string
}

// clientModel reports whether the game's traffic comes from traffic
// components (v6 and later) rather than one Internet.
func (g *Game) clientModel() bool {
	return g.Rules.Client != nil
}

// clientConfig is a traffic component's configuration, or the ruleset's
// until one is set.
func (g *Game) clientConfig(n *Node) *ClientConfig {
	if n.Client != nil {
		return n.Client
	}
	return g.Rules.Client
}

func shareOf(ws []Weight, name string) float64 {
	for _, w := range ws {
		if w.Name == name {
			return w.Share
		}
	}
	return 0
}

// endpointClass puts an endpoint in a request class by its method. v6 has
// no CDN, so no read is cacheable.
func endpointClass(name string) int {
	if method, _, _ := strings.Cut(name, " "); method != "GET" {
		return clsWrite
	}
	return clsRead
}

// clientLoads is every traffic component's volume this tick, in placement
// order. The market's volume is split into segments by client type and
// region; components of one segment share it evenly.
func (g *Game) clientLoads() []clientLoad {
	r := g.Rules
	_, hour := g.clock()
	market := g.Users * r.ActiveShare * g.engagement() * diurnal(hour)
	segment := map[[2]string]int{}
	for _, n := range g.Nodes {
		if c := g.clientConfig(n); n.Kind == KindTraffic && c.Source == SourceMarket {
			segment[[2]string{c.ClientType, c.Region}]++
		}
	}
	var out []clientLoad
	for i, n := range g.Nodes {
		if n.Kind != KindTraffic {
			continue
		}
		c := g.clientConfig(n)
		cl := clientLoad{node: i, cfg: c, to: -1}
		if c.Source == SourceConfigured {
			cl.unique = g.patternRPS(c.Pattern, n.TrafficSince)
		} else {
			cl.unique = market * shareOf(r.ClientTypes, c.ClientType) * shareOf(r.RegionShares, c.Region) /
				float64(segment[[2]string{c.ClientType, c.Region}])
		}
		cl.unique *= g.fx.traffic * factor(g.fx.trafficOn, n.ID)
		cl.attack = cl.unique * (g.fx.attack + g.fx.attackOn[n.ID])
		amp := 0.0
		for k := 0; k <= c.Retries; k++ {
			amp += math.Pow(g.clientFail[n.ID], float64(k))
		}
		cl.attempts = cl.unique * amp
		if to := g.targets(i, KindApp); len(to) == 0 {
			cl.problem = "not connected"
		} else {
			cl.to = to[0]
			cl.problem = g.contract(c, g.Nodes[cl.to])
		}
		out = append(out, cl)
	}
	return out
}

// contract is why a traffic component's requests cannot reach an
// application, or "" when both sides agree.
func (g *Game) contract(c *ClientConfig, app *Node) string {
	a := g.appConfig(app)
	switch {
	case c.Protocol != a.Protocol:
		return fmt.Sprintf("protocol error: %s clients, %s server", c.Protocol, a.Protocol)
	case c.Port != a.Port:
		return fmt.Sprintf("connection refused: port %d, the app listens on %d", c.Port, a.Port)
	case c.Scheme == SchemeHTTPS && !a.TLS:
		return "TLS handshake failed: https to an app without TLS"
	case c.Scheme == SchemeHTTP && a.TLS:
		return "TLS required: http to an app that only accepts https"
	}
	return ""
}

// inputs are the traffic components whose requests reach application i.
func (g *Game) inputs(i int) []clientLoad {
	var out []clientLoad
	for _, cl := range g.clients {
		if cl.to == i && cl.problem == "" {
			out = append(out, cl)
		}
	}
	return out
}

// clientMix splits each request class at application i into endpoints, in
// the shares its inputs send them. Without inputs it is measured at the
// ruleset's default population.
func (g *Game) clientMix(i int) ([]mixEntry, vec) {
	w := map[string]float64{}
	var names []string
	add := func(eps []Weight, v float64) {
		for _, e := range eps {
			if _, ok := w[e.Name]; !ok {
				names = append(names, e.Name)
			}
			w[e.Name] += v * e.Share
		}
	}
	for _, cl := range g.inputs(i) {
		add(cl.cfg.Endpoints, cl.attempts+cl.attack)
	}
	var total vec
	for _, name := range names {
		total[endpointClass(name)] += w[name]
	}
	if total.sum() == 0 {
		w, names, total = map[string]float64{}, nil, vec{}
		add(routeEndpoints(g.appConfig(g.Nodes[i])), 1)
		for _, name := range names {
			total[endpointClass(name)] += w[name]
		}
	}
	var out []mixEntry
	for _, name := range names {
		if c := endpointClass(name); w[name] > 0 {
			out = append(out, mixEntry{name, c, w[name] / total[c]})
		}
	}
	sum := total.sum()
	for c := range total {
		total[c] /= sum
	}
	return out, total
}

// routeEndpoints is one endpoint per route of an application, catch-all
// aside, in the routes' typical shares (equal when none is set). Shares are
// kept to hundredths of a percent so the form shows them exactly; the first
// takes the remainder.
func routeEndpoints(a *AppConfig) []Weight {
	var out []Weight
	total := 0.0
	for _, r := range a.Routes {
		if r.Endpoint != CatchAll && len(out) < maxEndpoints {
			out = append(out, Weight{r.Endpoint, r.Share})
			total += r.Share
		}
	}
	if len(out) == 0 {
		return []Weight{{"GET /", 1}}
	}
	rest := 0.0
	for k := range out {
		w := 1 / float64(len(out))
		if total > 0 {
			w = out[k].Share / total
		}
		out[k].Share = math.Floor(w*1e4) / 1e4
		if k > 0 {
			rest += out[k].Share
		}
	}
	out[0].Share = 1 - rest
	return out
}

// adopt matches a traffic component to the application it is connected to,
// when it connects and whenever the application is reconfigured: its
// protocol, port, scheme, and keep-alive, and one endpoint per route. Who the
// clients are, their timeout, retries, and source stay theirs, and the player
// can change any of it afterwards.
func (g *Game) adopt(n, app *Node) {
	a := g.appConfig(app)
	c := *g.clientConfig(n)
	c.Protocol, c.Port, c.KeepAlive = a.Protocol, a.Port, a.KeepAlive
	c.Scheme = SchemeHTTP
	if a.TLS {
		c.Scheme = SchemeHTTPS
	}
	c.Endpoints = routeEndpoints(a)
	n.Client = &c
	delete(g.clientFail, n.ID)
}

// release returns a traffic component that lost its connection to asking
// for nothing: the ruleset's connection settings and no endpoints. Who its
// clients are, their timeout, retries, and source stay theirs.
func (g *Game) release(n *Node) {
	d := g.Rules.Client
	c := *g.clientConfig(n)
	c.Protocol, c.Port, c.Scheme, c.KeepAlive = d.Protocol, d.Port, d.Scheme, d.KeepAlive
	c.Endpoints = []Weight{}
	n.Client = &c
	delete(g.clientFail, n.ID)
}

// keepAlive is the share of application i's connections that both sides
// keep alive: all or none before v6, by its inputs' attempts from v6.
func (g *Game) keepAlive(i int, cfg *AppConfig) float64 {
	if !cfg.KeepAlive {
		return 0
	}
	if !g.clientModel() {
		return 1
	}
	on, all := 0.0, 0.0
	for _, cl := range g.inputs(i) {
		v := cl.attempts + cl.attack
		all += v
		if cl.cfg.KeepAlive {
			on += v
		}
	}
	if all == 0 {
		return 1
	}
	return on / all
}

// finishClient is how a traffic component's attempts fared at its
// application: the chance one succeeds and the mean latency of those that
// do, under the shorter of the client's and the server's timeouts.
func (g *Game) finishClient(cl clientLoad, apps []*appRun) (float64, float64, ClientStats) {
	st := ClientStats{RPS: cl.unique, RetryRPS: cl.attempts - cl.unique, AttackRPS: cl.attack, Problem: cl.problem}
	if cl.problem != "" {
		st.Refused = cl.attempts
		return 0, 0, st
	}
	run := apps[cl.to]
	cfg := g.appConfig(g.Nodes[cl.to])
	timeout := math.Min(cl.cfg.TimeoutMs, cfg.TimeoutMs)
	ok, lat := 0.0, 0.0
	for _, e := range cl.cfg.Endpoints {
		v := cl.attempts * e.Share
		rte, _ := g.route(cfg, e.Name)
		k, has := run.byRoute[rte.Endpoint]
		if !has || v == 0 {
			continue
		}
		rr := run.routes[k]
		if rr.notFound {
			st.NotFound += v
			continue
		}
		sv := run.servedShare()
		p, late, ms := rr.outcome(sv, run.waitMs, timeout)
		ok += e.Share * p
		lat += e.Share * p * ms
		st.Rejected += v * (1 - sv)
		st.Timeouts += v * sv * late
		st.Errors += math.Max(0, v*(1-p)-v*(1-sv)-v*sv*late)
	}
	if ok > 0 {
		lat /= ok
	}
	st.LatencyMs = lat
	return ok, lat, st
}

// clientBreakdown sums every traffic component's real volume by client
// type, region, endpoint, and component, in ruleset and first-seen order.
func (g *Game) clientBreakdown(t *Traffic) {
	types, regions, endpoints := map[string]float64{}, map[string]float64{}, map[string]float64{}
	var names []string
	for _, cl := range g.clients {
		types[cl.cfg.ClientType] += cl.unique
		regions[cl.cfg.Region] += cl.unique
		for _, e := range cl.cfg.Endpoints {
			if _, ok := endpoints[e.Name]; !ok {
				names = append(names, e.Name)
			}
			endpoints[e.Name] += cl.unique * e.Share
		}
		t.Components = append(t.Components, Rate{Name: g.Nodes[cl.node].ID, RPS: cl.unique})
	}
	for _, w := range g.Rules.ClientTypes {
		if v, ok := types[w.Name]; ok {
			t.ClientTypes = append(t.ClientTypes, Rate{Name: w.Name, RPS: v})
		}
	}
	for _, w := range g.Rules.RegionShares {
		if v, ok := regions[w.Name]; ok {
			t.Regions = append(t.Regions, Rate{Name: w.Name, RPS: v})
		}
	}
	for _, name := range names {
		t.Endpoints = append(t.Endpoints, Rate{Name: name, RPS: endpoints[name]})
	}
}

// configureClient replaces a traffic component's configuration and restarts
// its pattern clock.
func (g *Game) configureClient(n *Node, c *ClientConfig) error {
	if c == nil {
		return invalid("configure needs a client configuration")
	}
	if problems := g.Rules.ValidateClient(*c); len(problems) > 0 {
		return invalid("%s", strings.Join(problems, "; "))
	}
	n.Client = c
	n.TrafficSince = g.Tick
	// Clients remember failures only under the configuration they saw them in.
	delete(g.clientFail, n.ID)
	return nil
}

// validEndpoint reports whether a name is like "GET /path".
func validEndpoint(name string) bool {
	method, path, _ := strings.Cut(name, " ")
	return methods[method] && strings.HasPrefix(path, "/") && !strings.ContainsAny(path, " \t") && len(path) <= maxNameLen
}

// ValidateClient lists every problem with a traffic component's
// configuration; none means it is valid. Nothing is corrected.
func (r *Ruleset) ValidateClient(c ClientConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if name := strings.TrimSpace(c.Name); name == "" || name != c.Name || len(name) > maxNameLen {
		bad("name must be 1 to %d characters without surrounding spaces", maxNameLen)
	}
	known := func(label, v string, ws []Weight) {
		if shareOf(ws, v) == 0 {
			bad("unknown %s %q", label, v)
		}
	}
	known("client type", c.ClientType, r.ClientTypes)
	known("region", c.Region, r.RegionShares)
	if !slices.Contains(r.Protocols, c.Protocol) {
		bad("protocol must be one of %s", strings.Join(r.Protocols, ", "))
	}
	if c.Scheme != SchemeHTTP && c.Scheme != SchemeHTTPS {
		bad("scheme must be %q or %q", SchemeHTTP, SchemeHTTPS)
	}
	if c.Port < 1 || c.Port > 65535 {
		bad("port must be between 1 and 65535")
	}
	if !(c.TimeoutMs >= 1 && c.TimeoutMs <= 120_000) {
		bad("timeout must be between 1 and 120000 ms")
	}
	if c.Retries < 0 || c.Retries > r.MaxRetries {
		bad("retries must be between 0 and %d", r.MaxRetries)
	}
	switch c.Source {
	case SourceConfigured:
		p = append(p, r.validatePattern(c.Pattern)...)
	case SourceMarket:
	default:
		bad("source must be %q or %q", SourceMarket, SourceConfigured)
	}
	if len(c.Endpoints) == 0 || len(c.Endpoints) > maxEndpoints {
		bad("declare 1 to %d endpoints", maxEndpoints)
	}
	seen := map[string]bool{}
	var shares []float64
	for i, e := range c.Endpoints {
		switch {
		case !validEndpoint(e.Name):
			bad("endpoint %d: must be like \"GET /path\" (GET, POST, PUT, PATCH, DELETE)", i+1)
		case seen[e.Name]:
			bad("%s is listed twice", e.Name)
		}
		seen[e.Name] = true
		shares = append(shares, e.Share)
	}
	return append(p, sumsToOne("endpoint shares", shares)...)
}

// RulesetV6 is RulesetV5 with traffic components (Phase 11, ADR-0018): a
// new game is empty; each traffic component is one client population that
// connects to one application under a contract.
func RulesetV6() *Ruleset {
	r := RulesetV5()
	r.Version = "sandbox/v6"
	// The CDN, load balancer, and gateway return with their own contracts.
	kinds := []Kind{{Name: KindTraffic, Label: "Traffic", ConnectsTo: []string{KindApp}}}
	for _, k := range r.Kinds {
		switch k.Name {
		case KindInternet, KindCDN, KindLB, KindGateway:
		default:
			kinds = append(kinds, k)
		}
	}
	r.Kinds = kinds
	r.Traffic = nil
	r.Protocols = []string{"HTTP/1.1", "HTTP/2", "gRPC"}
	r.ClientTypes = []Weight{{ClientWeb, 0.7}, {ClientMobile, 0.2}, {ClientAPI, 0.08}, {ClientBot, 0.02}}
	r.RegionShares = []Weight{{"asia", 0.35}, {"europe", 0.25}, {"north-america", 0.2}, {"south-america", 0.1}, {"africa", 0.05}, {"oceania", 0.05}}
	r.Client = &ClientConfig{
		Name: "Web users", ClientType: ClientWeb, Region: "asia",
		Protocol: "HTTP/1.1", Scheme: SchemeHTTPS, Port: 8000, KeepAlive: true, TimeoutMs: 10_000,
		Source: SourceMarket, Pattern: Pattern{Shape: ShapeConstant, RPS: 100},
		// It asks for nothing until it is connected and adopts the app's routes.
		Endpoints: []Weight{},
	}
	r.AppStacks, r.AppTypes = appStacks(r.App), appTypes()
	// A new instance is the e-commerce API on the default stack.
	r.App.Routes = r.AppTypes[0].Routes
	for i, gl := range r.Goals {
		if gl.ID == "scale-out" {
			r.Goals[i].Description = "Serve from two or more app replicas."
			r.Goals[i].Conditions = []Condition{
				{Type: CondKind, Label: "App ops/s", Name: KindApp, Stat: "served", Min: at(0.001)},
				{Type: CondKind, Label: "App replicas", Name: KindApp, Stat: "replicas", Min: at(2)},
			}
		}
	}
	return r
}
