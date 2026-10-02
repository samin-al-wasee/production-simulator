package sandbox

import (
	"fmt"
	"math"
	"strings"
)

// Traffic sources: where the Internet's request volume comes from.
const (
	// SourceMarket derives volume from the userbase, as the game does.
	SourceMarket = "market"
	// SourceConfigured is a load test: the player declares the volume and its
	// shape over time. It earns nothing and pauses growth and goals.
	SourceConfigured = "configured"
)

// Pattern shapes for a configured source.
const (
	ShapeConstant = "constant"
	ShapeRamp     = "ramp"
	ShapeSpike    = "spike"
	ShapeBurst    = "burst"
	ShapePeriodic = "periodic"
	ShapeSchedule = "schedule"
)

// Limits on a traffic configuration. They bound what a player can declare
// and do not change how a valid configuration is simulated.
const (
	maxGroups    = 8
	maxEndpoints = 12
	maxNameLen   = 40
	shareSlack   = 1e-6
)

var methods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// TrafficConfig is the Internet's configuration: who sends requests, what
// they ask for, and from where. It is replaced whole by a configure command
// and never changed in place.
type TrafficConfig struct {
	Source    string         `json:"source"`
	Pattern   Pattern        `json:"pattern"`
	Endpoints []Endpoint     `json:"endpoints"`
	Groups    []TrafficGroup `json:"groups"`
}

// Endpoint is one kind of request. GET is a read and every other method a
// write; a cacheable read may be answered by a CDN, and a storage request
// also fetches an object from object storage.
type Endpoint struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Cacheable bool   `json:"cacheable,omitempty"`
	Storage   bool   `json:"storage,omitempty"`
}

// Name is how groups refer to the endpoint, e.g. "GET /products".
func (e Endpoint) Name() string { return e.Method + " " + e.Path }

func (e Endpoint) class() int {
	switch {
	case e.Method != "GET":
		return clsWrite
	case e.Cacheable:
		return clsCacheable
	}
	return clsRead
}

// TrafficGroup is one population of clients inside the Internet, with its
// share of the volume and its own request and regional mix.
type TrafficGroup struct {
	Name  string  `json:"name"`
	Share float64 `json:"share"`
	// Retries is how many times a client retries a failed request.
	Retries   int      `json:"retries,omitempty"`
	Endpoints []Weight `json:"endpoints"`
	Regions   []Weight `json:"regions"`
}

// Weight is a named share of a whole; the shares of one list sum to 1.
type Weight struct {
	Name  string  `json:"name"`
	Share float64 `json:"share"`
}

// Pattern is the shape of a configured source's volume over time, in
// requests per second. Minutes are simulated minutes since the traffic was
// last configured; a schedule follows the game clock.
type Pattern struct {
	Shape string  `json:"shape"`
	RPS   float64 `json:"rps"`
	// PeakRPS is a ramp's target, a spike's or burst's peak, and a periodic
	// wave's maximum.
	PeakRPS       float64        `json:"peakRps,omitempty"`
	StartMinutes  float64        `json:"startMinutes,omitempty"`
	Minutes       float64        `json:"minutes,omitempty"`
	PeriodMinutes float64        `json:"periodMinutes,omitempty"`
	Schedule      []ScheduleStep `json:"schedule,omitempty"`
}

// ScheduleStep holds a volume from an hour of the day until the next step.
type ScheduleStep struct {
	Hour float64 `json:"hour"`
	RPS  float64 `json:"rps"`
}

// Rate is a named part of a tick's traffic, in requests per second.
type Rate struct {
	Name string  `json:"name"`
	RPS  float64 `json:"rps"`
}

// Traffic is what the Internet sent during a tick. Group, region, and
// endpoint volumes are requests before retries.
type Traffic struct {
	Source string `json:"source"`
	// RPS is real requests per second; RetryRPS is the extra attempts
	// clients made after failures.
	RPS      float64 `json:"rps"`
	RetryRPS float64 `json:"retryRps,omitempty"`
	// Concurrency is requests in flight: attempts per second × mean latency
	// (Little's law).
	Concurrency float64 `json:"concurrency"`
	Groups      []Rate  `json:"groups,omitempty"`
	Regions     []Rate  `json:"regions,omitempty"`
	Endpoints   []Rate  `json:"endpoints,omitempty"`
	// ClientTypes and Components break v6 traffic down by client type and
	// by traffic component.
	ClientTypes []Rate `json:"clientTypes,omitempty"`
	Components  []Rate `json:"components,omitempty"`
}

// traffic is the Internet's configuration, or nil for a ruleset without one.
func (g *Game) traffic() *TrafficConfig {
	if n := g.Node(InternetID); n != nil && n.Traffic != nil {
		return n.Traffic
	}
	return g.Rules.Traffic
}

// loadTest reports whether the player is driving traffic themselves.
func (g *Game) loadTest() bool {
	if g.clientModel() {
		for _, n := range g.Nodes {
			if n.Kind == KindTraffic && g.clientConfig(n).Source == SourceConfigured {
				return true
			}
		}
		return false
	}
	tc := g.traffic()
	return tc != nil && tc.Source == SourceConfigured
}

// patternRPS is a configured source's volume at the current tick, for a
// pattern applied at tick since.
func (g *Game) patternRPS(p Pattern, since int) float64 {
	m := float64(g.Tick-since) * g.Rules.TickSeconds / 60
	switch p.Shape {
	case ShapeRamp:
		return p.RPS + (p.PeakRPS-p.RPS)*math.Min(m/p.Minutes, 1)
	case ShapeSpike:
		if m >= p.StartMinutes && m < p.StartMinutes+p.Minutes {
			return p.PeakRPS
		}
	case ShapeBurst:
		if math.Mod(m, p.PeriodMinutes) < p.Minutes {
			return p.PeakRPS
		}
	case ShapePeriodic:
		return p.RPS + (p.PeakRPS-p.RPS)*(1-math.Cos(2*math.Pi*m/p.PeriodMinutes))/2
	case ShapeSchedule:
		_, hour := g.clock()
		// Before the first step of the day the last step still holds.
		v := p.Schedule[len(p.Schedule)-1].RPS
		for _, s := range p.Schedule {
			if s.Hour <= hour {
				v = s.RPS
			}
		}
		return v
	}
	return p.RPS
}

// groupLoad is one group's requests per second by class, and the part of
// each class that also fetches from object storage.
type groupLoad struct {
	retries int
	load    [nClass]float64
	storage [nClass]float64
}

// groups splits a tick's real volume into its groups. A ruleset without a
// traffic configuration sends one group with its fixed read and storage
// shares, which is the request mix of rulesets v1 to v3.
func (g *Game) groups(rps float64) []groupLoad {
	if g.clientModel() {
		return nil
	}
	tc := g.traffic()
	if tc == nil {
		r := g.Rules
		var gl groupLoad
		gl.load[clsRead] = rps * r.ReadShare
		gl.load[clsWrite] = rps * (1 - r.ReadShare)
		for c := range gl.load {
			gl.storage[c] = gl.load[c] * r.StorageShare
		}
		return []groupLoad{gl}
	}
	eps := map[string]Endpoint{}
	for _, e := range tc.Endpoints {
		eps[e.Name()] = e
	}
	out := make([]groupLoad, len(tc.Groups))
	for i, grp := range tc.Groups {
		gl := groupLoad{retries: grp.Retries}
		for _, w := range grp.Endpoints {
			e := eps[w.Name]
			v := rps * grp.Share * w.Share
			gl.load[e.class()] += v
			if e.Storage {
				gl.storage[e.class()] += v
			}
		}
		out[i] = gl
	}
	return out
}

// breakdown reports a tick's real volume by group, region, and endpoint.
func (g *Game) breakdown(t *Traffic) {
	if g.clientModel() {
		g.clientBreakdown(t)
		return
	}
	tc := g.traffic()
	if tc == nil {
		return
	}
	regions := map[string]float64{}
	endpoints := map[string]float64{}
	for _, grp := range tc.Groups {
		v := t.RPS * grp.Share
		t.Groups = append(t.Groups, Rate{Name: grp.Name, RPS: v})
		for _, w := range grp.Regions {
			regions[w.Name] += v * w.Share
		}
		for _, w := range grp.Endpoints {
			endpoints[w.Name] += v * w.Share
		}
	}
	for _, r := range g.Rules.Regions {
		if v, ok := regions[r]; ok {
			t.Regions = append(t.Regions, Rate{Name: r, RPS: v})
		}
	}
	for _, e := range tc.Endpoints {
		t.Endpoints = append(t.Endpoints, Rate{Name: e.Name(), RPS: endpoints[e.Name()]})
	}
}

// configure replaces the Internet's traffic configuration and restarts its
// pattern clock.
func (g *Game) configure(c Command) error {
	if n := g.Node(c.Node); n != nil && g.telemetryModel() && c.Telemetry != nil {
		return g.configureTelemetry(n, c.Telemetry)
	}
	if n := g.Node(c.Node); n != nil && n.Kind == KindApp {
		return g.configureApp(n, c.App)
	}
	if n := g.Node(c.Node); n != nil && n.Kind == KindTraffic {
		return g.configureClient(n, c.Client)
	}
	if n := g.Node(c.Node); n != nil && g.edgeModel(n) && (c.LB != nil || c.Gateway != nil || c.CDN != nil) {
		return g.configureEdge(n, c)
	}
	if n := g.Node(c.Node); n != nil && g.streamModel(n) && c.Stream != nil {
		return g.configureStream(n, c.Stream)
	}
	if n := g.Node(c.Node); n != nil && g.queueModel(n) && c.Queue != nil {
		return g.configureQueue(n, c.Queue)
	}
	if n := g.Node(c.Node); n != nil && n.Kind == KindWorker && g.Rules.Worker != nil && c.Worker != nil {
		return g.configureWorker(n, c.Worker)
	}
	if n := g.Node(c.Node); n != nil && g.storageModel(n) && c.Storage != nil {
		return g.configureStorage(n, c.Storage)
	}
	if n := g.Node(c.Node); n != nil && g.cacheModel(n) && c.Cache != nil {
		return g.configureCache(n, c.Cache)
	}
	if n := g.Node(c.Node); n != nil && g.dbModel(n) && c.DB != nil {
		return g.configureDB(n, c.DB)
	}
	if g.callModel() && c.Connection != nil {
		return g.configureConn(c.From, c.To, c.Connection)
	}
	if n := g.Node(c.Node); n != nil && g.callModel() && c.Listener != nil {
		return g.configureListener(n, c.Listener)
	}
	if c.Node != InternetID {
		return invalid("only the Internet and application instances can be configured")
	}
	if g.Rules.Traffic == nil {
		return invalid("ruleset %s has no traffic configuration", g.Rules.Version)
	}
	if c.Traffic == nil {
		return invalid("configure needs a traffic configuration")
	}
	if problems := g.Rules.Validate(*c.Traffic); len(problems) > 0 {
		return invalid("%s", strings.Join(problems, "; "))
	}
	n := g.Node(InternetID)
	n.Traffic = c.Traffic
	n.TrafficSince = g.Tick
	// Clients remember failures only under the configuration they saw them in.
	g.attemptFail = vec{}
	return nil
}

// Validate lists every problem with a traffic configuration; none means it
// is valid. Nothing is corrected.
func (r *Ruleset) Validate(tc TrafficConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }

	switch tc.Source {
	case SourceConfigured:
		p = append(p, r.validatePattern(tc.Pattern)...)
	case SourceMarket:
		// The market ignores the pattern, so it is checked once it is used.
	default:
		bad("source must be %q or %q", SourceMarket, SourceConfigured)
	}

	if len(tc.Endpoints) == 0 || len(tc.Endpoints) > maxEndpoints {
		bad("declare 1 to %d endpoints", maxEndpoints)
	}
	eps := map[string]bool{}
	for i, e := range tc.Endpoints {
		name := e.Name()
		switch {
		case !methods[e.Method]:
			bad("endpoint %d: method must be one of GET, POST, PUT, PATCH, DELETE", i+1)
		case !strings.HasPrefix(e.Path, "/") || strings.ContainsAny(e.Path, " \t"):
			bad("endpoint %d: path must start with / and have no spaces", i+1)
		case len(e.Path) > maxNameLen:
			bad("endpoint %d: path must be at most %d characters", i+1, maxNameLen)
		case e.Cacheable && e.Method != "GET":
			bad("%s: only GET requests can be cacheable", name)
		case eps[name]:
			bad("%s is declared twice", name)
		}
		eps[name] = true
	}

	regions := map[string]bool{}
	for _, name := range r.Regions {
		regions[name] = true
	}
	if len(tc.Groups) == 0 || len(tc.Groups) > maxGroups {
		bad("declare 1 to %d traffic groups", maxGroups)
	}
	seen := map[string]bool{}
	var shares []float64
	for i, grp := range tc.Groups {
		label := fmt.Sprintf("group %d", i+1)
		name := strings.TrimSpace(grp.Name)
		switch {
		case name == "" || name != grp.Name || len(name) > maxNameLen:
			bad("%s: name must be 1 to %d characters without surrounding spaces", label, maxNameLen)
		case seen[strings.ToLower(name)]:
			bad("group %q is declared twice", name)
		default:
			label = fmt.Sprintf("group %q", name)
		}
		seen[strings.ToLower(name)] = true
		shares = append(shares, grp.Share)
		if grp.Retries < 0 || grp.Retries > r.MaxRetries {
			bad("%s: retries must be between 0 and %d", label, r.MaxRetries)
		}
		p = append(p, weights(label+" endpoints", grp.Endpoints, eps)...)
		p = append(p, weights(label+" regions", grp.Regions, regions)...)
	}
	p = append(p, sumsToOne("traffic group shares", shares)...)
	return p
}

// weights checks a share list against the names it may use.
func weights(label string, ws []Weight, known map[string]bool) []string {
	var p []string
	if len(ws) == 0 {
		return []string{label + ": list at least one"}
	}
	seen := map[string]bool{}
	var shares []float64
	for _, w := range ws {
		switch {
		case !known[w.Name]:
			p = append(p, fmt.Sprintf("%s: unknown %q", label, w.Name))
		case seen[w.Name]:
			p = append(p, fmt.Sprintf("%s: %q is listed twice", label, w.Name))
		}
		seen[w.Name] = true
		shares = append(shares, w.Share)
	}
	return append(p, sumsToOne(label, shares)...)
}

func sumsToOne(label string, shares []float64) []string {
	sum := 0.0
	for _, s := range shares {
		if s < 0 || s > 1 || math.IsNaN(s) {
			return []string{label + ": each share must be between 0% and 100%"}
		}
		sum += s
	}
	if math.Abs(sum-1) > shareSlack {
		return []string{fmt.Sprintf("%s must sum to 100%% (now %s%%)", label, trimFloat(sum*100))}
	}
	return nil
}

func (r *Ruleset) validatePattern(p Pattern) []string {
	var out []string
	bad := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	rps := func(label string, v float64) {
		if !(v >= 0 && v <= r.MaxTrafficRPS) {
			bad("%s must be between 0 and %s requests/s", label, trimFloat(r.MaxTrafficRPS))
		}
	}
	tick := r.TickSeconds / 60
	positive := func(label string, v float64, least float64) {
		if !(v >= least) || math.IsInf(v, 0) {
			bad("%s must be at least %s minutes", label, trimFloat(least))
		}
	}
	switch p.Shape {
	case ShapeConstant:
		rps("rate", p.RPS)
	case ShapeRamp:
		rps("start rate", p.RPS)
		rps("target rate", p.PeakRPS)
		positive("ramp duration", p.Minutes, tick)
	case ShapeSpike:
		rps("base rate", p.RPS)
		rps("peak rate", p.PeakRPS)
		if !(p.StartMinutes >= 0) || math.IsInf(p.StartMinutes, 0) {
			bad("spike start must be 0 minutes or later")
		}
		positive("spike duration", p.Minutes, tick)
	case ShapeBurst:
		rps("base rate", p.RPS)
		rps("burst rate", p.PeakRPS)
		// Both halves of the cycle must last a tick, or the burst would
		// never (or always) be seen.
		positive("burst duration", p.Minutes, tick)
		positive("time between bursts", p.PeriodMinutes-p.Minutes, tick)
	case ShapePeriodic:
		rps("low rate", p.RPS)
		rps("high rate", p.PeakRPS)
		positive("period", p.PeriodMinutes, 2*tick)
	case ShapeSchedule:
		if len(p.Schedule) == 0 || len(p.Schedule) > 24 {
			bad("a schedule needs 1 to 24 steps")
		}
		for i, s := range p.Schedule {
			if !(s.Hour >= 0 && s.Hour < 24) {
				bad("schedule step %d: hour must be from 0 to under 24", i+1)
			} else if i > 0 && !(s.Hour > p.Schedule[i-1].Hour) {
				bad("schedule step %d: hours must increase", i+1)
			}
			rps(fmt.Sprintf("schedule step %d rate", i+1), s.RPS)
		}
	default:
		bad("pattern shape must be one of constant, ramp, spike, burst, periodic, schedule")
	}
	return out
}

func trimFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}

// RulesetV4 is RulesetV3 with a configurable Internet (Phase 11, ADR-0016):
// traffic groups, an endpoint mix that decides reads, writes, cacheable
// reads, and storage fetches, regions, client retries, and a configured
// load-test source.
func RulesetV4() *Ruleset {
	r := RulesetV3()
	r.Version = "sandbox/v4"
	for i := range r.Kinds {
		if r.Kinds[i].Name == KindCDN {
			// A CDN now answers only cacheable reads; about two thirds of the
			// default mix is cacheable, which keeps the v3 edge share of 30%.
			r.Kinds[i].HitRatio = 0.45
		}
	}
	r.Regions = []string{"asia", "europe", "north-america", "south-america", "africa", "oceania"}
	r.MaxRetries = 3
	r.MaxTrafficRPS = 1_000_000
	regions := []Weight{{"asia", 0.4}, {"europe", 0.3}, {"north-america", 0.2}, {"south-america", 0.1}}
	r.Traffic = &TrafficConfig{
		Source:  SourceMarket,
		Pattern: Pattern{Shape: ShapeConstant, RPS: 100},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/products", Cacheable: true},
			{Method: "GET", Path: "/products/:id", Cacheable: true},
			{Method: "GET", Path: "/profile"},
			{Method: "GET", Path: "/media/:id", Cacheable: true, Storage: true},
			{Method: "POST", Path: "/orders"},
			{Method: "POST", Path: "/login"},
		},
		Groups: []TrafficGroup{
			{Name: "Web users", Share: 0.7, Regions: regions, Endpoints: []Weight{
				{"GET /products", 0.35}, {"GET /products/:id", 0.2}, {"GET /profile", 0.1},
				{"GET /media/:id", 0.15}, {"POST /orders", 0.12}, {"POST /login", 0.08}}},
			{Name: "Mobile users", Share: 0.2, Regions: regions, Endpoints: []Weight{
				{"GET /products", 0.3}, {"GET /products/:id", 0.25}, {"GET /profile", 0.15},
				{"GET /media/:id", 0.1}, {"POST /orders", 0.12}, {"POST /login", 0.08}}},
			{Name: "API clients", Share: 0.08, Regions: regions, Endpoints: []Weight{
				{"GET /products", 0.4}, {"GET /products/:id", 0.3}, {"POST /orders", 0.3}}},
			{Name: "Bots", Share: 0.02, Regions: regions, Endpoints: []Weight{
				{"GET /products", 0.6}, {"GET /products/:id", 0.4}}},
		},
	}
	return r
}
