package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Processing models for an application instance.
const (
	// ProcessingSync holds one worker per request for its whole duration,
	// including time spent waiting on dependencies.
	ProcessingSync = "sync"
	// ProcessingAsync parks a request while it waits, so one worker can
	// keep many requests in flight, up to MaxConcurrency.
	ProcessingAsync = "async"
)

// Route dependencies: what a handler calls, in order.
const (
	DepCache   = "cache"    // a cache, or the database when none is connected
	DepDBRead  = "db-read"  // a database primary or read replica
	DepDBWrite = "db-write" // a message queue when one is connected, else a database primary
	DepQueue   = "queue"    // a message queue
	DepStorage = "storage"  // object storage
)

// Health states of an application instance, derived every tick.
const (
	HealthStarting  = "starting"
	HealthHealthy   = "healthy"
	HealthDegraded  = "degraded"
	HealthUnhealthy = "unhealthy"
	HealthStopped   = "stopped"
)

// CatchAll is the route that handles every endpoint without its own route.
const CatchAll = "*"

const maxRoutes = 20

var deps = map[string]bool{DepCache: true, DepDBRead: true, DepDBWrite: true, DepQueue: true, DepStorage: true, DepStream: true}

// AppConfig is one application instance's configuration: a backend web or
// API service. It applies to each replica, and is replaced whole by a
// configure command.
type AppConfig struct {
	// Labels: shown to the player, no effect on the model.
	Name        string `json:"name"`
	Framework   string `json:"framework"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Interface   string `json:"interface"`
	Server      string `json:"server"`

	Processing string `json:"processing"`
	Workers    int    `json:"workers"`
	// MaxConcurrency bounds requests in flight per replica when async; a
	// sync instance has one per worker.
	MaxConcurrency int `json:"maxConcurrency"`
	// Backlog is how many requests per replica may wait for a slot.
	Backlog        int     `json:"backlog"`
	MaxConnections int     `json:"maxConnections"`
	TimeoutMs      float64 `json:"timeoutMs"`
	TLS            bool    `json:"tls"`
	KeepAlive      bool    `json:"keepAlive"`

	// Middleware runs, in order, before every route.
	Middleware []string `json:"middleware"`
	// RateLimitRPS is the per-replica limit of the rate-limit middleware.
	RateLimitRPS float64 `json:"rateLimitRps,omitempty"`
	// Routes handle endpoints; the CatchAll route handles the rest.
	Routes []AppRoute `json:"routes"`
}

// AppRoute is the work one endpoint costs.
type AppRoute struct {
	Endpoint string `json:"endpoint"`
	// BaseMs is the handler's own time; CPUMs the part of it on a CPU.
	BaseMs     float64 `json:"baseMs"`
	CPUMs      float64 `json:"cpuMs"`
	MemoryMB   float64 `json:"memoryMb"`
	RequestKB  float64 `json:"requestKb"`
	ResponseKB float64 `json:"responseKb"`
	// ErrorRate is the share of requests the handler itself fails.
	ErrorRate float64  `json:"errorRate,omitempty"`
	Deps      []string `json:"deps,omitempty"`
	// Share is the part of a typical client's requests this endpoint gets
	// when a traffic component adopts the routes (v6); none means equal.
	Share float64 `json:"share,omitempty"`
	// Calls are requests to other services' endpoints (v7).
	Calls []Call `json:"calls,omitempty"`
}

// Middleware is a stage in the catalog, with its cost per request.
type Middleware struct {
	Name  string  `json:"name"`
	Label string  `json:"label"`
	Ms    float64 `json:"ms"`
	CPUMs float64 `json:"cpuMs"`
}

// Framework is a preset the dashboard offers; the engine never reads it.
type Framework struct {
	Name           string `json:"name"`
	Interface      string `json:"interface"`
	Server         string `json:"server"`
	Processing     string `json:"processing"`
	Workers        int    `json:"workers"`
	MaxConcurrency int    `json:"maxConcurrency"`
}

// AppRuntime holds the constants of the application model.
type AppRuntime struct {
	// WorkerMemoryMB is each worker's resident memory before any request.
	WorkerMemoryMB float64 `json:"workerMemoryMb"`
	// TLSHandshakeCPUMs is the CPU a new TLS connection costs.
	TLSHandshakeCPUMs float64 `json:"tlsHandshakeCpuMs"`
	// With keep-alive a connection carries RequestsPerConnection requests
	// and then stays open, idle, for KeepAliveSeconds.
	RequestsPerConnection float64 `json:"requestsPerConnection"`
	KeepAliveSeconds      float64 `json:"keepAliveSeconds"`
	// StartSeconds is how long a new or recovered instance takes to start.
	StartSeconds float64 `json:"startSeconds"`
}

// AppStats is what an application instance did during a tick. Rates are
// requests per second over all its replicas.
type AppStats struct {
	Health string `json:"health"`
	// Bottleneck is the resource that limits throughput: cpu, slots,
	// connections, network-in, or network-out.
	Bottleneck  string  `json:"bottleneck"`
	Capacity    float64 `json:"capacity"`
	CPUUsed     float64 `json:"cpuUsed"`
	CPUTotal    float64 `json:"cpuTotal"`
	MemoryMB    float64 `json:"memoryMb"`
	MemoryTotal float64 `json:"memoryTotalMb"`
	Active      float64 `json:"active"`
	Queued      float64 `json:"queued"`
	Connections float64 `json:"connections"`
	WaitMs      float64 `json:"waitMs"`
	Success     float64 `json:"success"`
	Errors      float64 `json:"errors"`
	Timeouts    float64 `json:"timeouts"`
	Rejected    float64 `json:"rejected"`
	// OutOfMemory means the instance crashed at the end of the tick.
	OutOfMemory bool         `json:"outOfMemory,omitempty"`
	Routes      []RouteStats `json:"routes"`
}

// RouteStats is one route's share of a tick.
type RouteStats struct {
	Endpoint  string  `json:"endpoint"`
	RPS       float64 `json:"rps"`
	Success   float64 `json:"success"`
	Errors    float64 `json:"errors"`
	Timeouts  float64 `json:"timeouts"`
	Rejected  float64 `json:"rejected"`
	LatencyMs float64 `json:"latencyMs"`
	// NotFound marks an endpoint the instance has no route for (v6).
	NotFound bool `json:"notFound,omitempty"`
}

// appConfig is a node's configuration, or the ruleset's until one is set.
func (g *Game) appConfig(n *Node) *AppConfig {
	if n.Kind == KindWorker && g.Rules.Worker != nil {
		return g.workerApp(n)
	}
	if n.App != nil {
		return n.App
	}
	return g.Rules.App
}

// appModel reports whether a node runs the application model of v5.
func (g *Game) appModel(n *Node) bool {
	return n.Kind == KindApp && g.Rules.App != nil || n.Kind == KindWorker && g.Rules.Worker != nil
}

// mixEntry is an endpoint's request class and its share of that class.
type mixEntry struct {
	name  string
	cls   int
	share float64
}

// endpointMix splits each request class at application i into its
// endpoints: in the shares the Internet sends them before v6, and in its own
// inputs' shares from v6 (ADR-0018).
// ponytail: before v6 nothing between the Internet and an application treats
// endpoints of one class differently; v6 connects traffic straight to an
// application, so carry a per-endpoint vector through the solver once a
// component between them routes or blocks by path.
func (g *Game) endpointMix(i int) ([]mixEntry, vec) {
	if g.callModel() {
		return g.epMix(i)
	}
	if g.clientModel() {
		return g.clientMix(i)
	}
	tc := g.traffic()
	if tc == nil {
		return nil, vec{}
	}
	w := map[string]float64{}
	for _, grp := range tc.Groups {
		for _, e := range grp.Endpoints {
			w[e.Name] += grp.Share * e.Share
		}
	}
	var total vec
	for _, e := range tc.Endpoints {
		total[e.class()] += w[e.Name()]
	}
	var out []mixEntry
	for _, e := range tc.Endpoints {
		if c := e.class(); total[c] > 0 && w[e.Name()] > 0 {
			out = append(out, mixEntry{e.Name(), c, w[e.Name()] / total[c]})
		}
	}
	return out, total
}

// appCapacity is node i's throughput at its request mix, which the
// balancers in front of it split by.
func (g *Game) appCapacity(i int) float64 {
	_, mix := g.endpointMix(i)
	if c := g.runApp(i, mix).capacity; !math.IsInf(c, 1) {
		return c
	}
	return 0
}

// routeRun is one route's load and costs at an instance during a tick.
type routeRun struct {
	route AppRoute
	load  vec // by request class
	rate  float64
	// own is handler plus middleware time; wall adds last tick's
	// dependency latency; cpu, conn, in, and out are per request.
	own, wall, cpu, conn, in, out float64
	calls                         []callRun
	// names are the endpoints this route handles; pool is, per call
	// target, the seconds of an open connection one request holds (v7).
	names []string
	pool  map[int]float64
	// notFound marks an endpoint with no route and no catch-all (v6).
	notFound bool
	// ok is the chance every dependency call succeeds, and depMs their mean
	// latency, once the dependencies are solved.
	ok, depMs float64
}

// callRun is one call a route makes: a dependency of a request class, or a
// service's endpoint (v7), to its targets.
type callRun struct {
	to       []route
	cls      int
	endpoint string
	async    bool
}

// appRun is what an application instance did with a tick's load.
type appRun struct {
	routes   []routeRun
	capacity float64
	limited  float64 // rejected by the rate-limit middleware
	served   float64
	rejected float64
	rho      float64
	waitMs   float64
	queued   float64
	stats    AppStats
	// byRoute indexes routes by their endpoint; out is each endpoint's
	// chance to succeed and latency once finished (v7).
	byRoute map[string]int
	out     map[string][2]float64
}

// servedShare is the share of arriving requests the instance served.
func (run *appRun) servedShare() float64 {
	if lambda := run.served + run.rejected; lambda > 0 {
		return run.served / lambda
	}
	return 0
}

// route is the route that handles an endpoint: its own, else the catch-all.
// Without either the endpoint is not found, which only v6 allows: it costs
// the middleware and fails.
func (g *Game) route(cfg *AppConfig, endpoint string) (AppRoute, bool) {
	all := -1
	for i, r := range cfg.Routes {
		if r.Endpoint == endpoint {
			return r, true
		}
		if r.Endpoint == CatchAll {
			all = i
		}
	}
	if all >= 0 {
		return cfg.Routes[all], true
	}
	return AppRoute{Endpoint: endpoint, ErrorRate: 1}, false
}

// depTargets is where a dependency call goes, and the request class it
// counts as there.
func (g *Game) depTargets(i int, dep string) ([]route, int) {
	switch dep {
	case DepCache:
		if t := g.targets(i, KindCache); len(t) > 0 {
			return g.split(t), clsRead
		}
		return g.split(g.targets(i, KindDBPrimary, KindDBReplica)), clsRead
	case DepDBRead:
		return g.split(g.targets(i, KindDBPrimary, KindDBReplica)), clsRead
	case DepDBWrite:
		// A connected queue takes writes for its workers to make, as in v4.
		if t := g.targets(i, KindQueue); len(t) > 0 {
			return g.split(t), clsWrite
		}
		return g.split(g.targets(i, KindDBPrimary)), clsWrite
	case DepQueue:
		return g.split(g.targets(i, KindQueue)), clsWrite
	case DepStream:
		return g.split(g.targets(i, KindStream)), clsWrite
	}
	return g.split(g.targets(i, KindStorage)), clsRead
}

// startFactor is the share of a tick an instance spends serving: a tick in
// which it starts loses StartSeconds.
func (g *Game) startFactor(n *Node) float64 {
	if n.StartedAt != g.Tick {
		return 1
	}
	return math.Max(0, 1-g.Rules.AppRuntime.StartSeconds/g.Rules.TickSeconds)
}

// runApp applies the application model to node i's load. Capacity is the
// throughput at which the first resource runs out; each resource's use is
// load × per-request cost, so the mix of routes decides which one that is.
func (g *Game) runApp(i int, load vec) appRun {
	n := g.Nodes[i]
	cfg := g.appConfig(n)
	r := g.Rules
	rt := r.AppRuntime
	size, _ := r.Size(n.Size)
	up := float64(g.upReplicas(n))

	mwMs, mwCPU := 0.0, 0.0
	for _, name := range cfg.Middleware {
		for _, m := range r.Middleware {
			if m.Name == name {
				mwMs += m.Ms
				mwCPU += m.CPUMs
			}
		}
	}
	// New connections per request: one each without keep-alive. From v6
	// a connection is kept alive only when its client keeps it alive too.
	perConn, idle := 1.0, 0.0
	if ka := g.keepAlive(i, cfg); ka == 1 {
		perConn, idle = 1/rt.RequestsPerConnection, rt.KeepAliveSeconds/rt.RequestsPerConnection
	} else if ka > 0 {
		perConn, idle = 1-ka+ka/rt.RequestsPerConnection, ka*rt.KeepAliveSeconds/rt.RequestsPerConnection
	}
	tls := 0.0
	if cfg.TLS {
		tls = rt.TLSHandshakeCPUMs * perConn
	}

	run := appRun{byRoute: map[string]int{}}
	if g.callModel() {
		run.out = map[string][2]float64{}
	}
	byRoute := run.byRoute
	mix, _ := g.endpointMix(i)
	for _, m := range mix {
		v := load[m.cls] * m.share
		if v == 0 {
			continue
		}
		rte, found := g.route(cfg, m.name)
		k, ok := byRoute[rte.Endpoint]
		if !ok {
			k = len(run.routes)
			byRoute[rte.Endpoint] = k
			rr := routeRun{route: rte, own: rte.BaseMs + mwMs, cpu: rte.CPUMs + mwCPU + tls,
				in: rte.RequestKB * 8 / 1000, out: rte.ResponseKB * 8 / 1000, notFound: !found}
			rr.wall = rr.own
			for _, d := range rte.Deps {
				to, cls := g.depTargets(i, d)
				rr.calls = append(rr.calls, callRun{to: to, cls: cls})
				if g.callModel() {
					continue
				}
				for _, t := range to {
					rr.wall += t.share * g.lastPath[g.Nodes[t.to].ID][cls]
				}
			}
			if g.callModel() {
				for _, cl := range rte.Calls {
					rr.calls = append(rr.calls, callRun{to: g.split(g.services(i, cl.Service)), cls: endpointClass(cl.Endpoint), endpoint: cl.Endpoint, async: cl.Async})
				}
				g.callWall(i, &rr)
			}
			if g.telemetryModel() {
				// Reporting costs the instance CPU per request (v14).
				rr.cpu += g.overheadCPUMs(n, len(rr.calls))
			}
			rr.conn = rr.wall/1000 + idle
			run.routes = append(run.routes, rr)
		}
		run.routes[k].load[m.cls] += v
		run.routes[k].rate += v
		run.routes[k].names = append(run.routes[k].names, m.name)
	}

	lambda := 0.0
	var cpu, hold, conn, in, out float64
	for _, rr := range run.routes {
		lambda += rr.rate
		cpu += rr.rate * rr.cpu
		hold += rr.rate * rr.wall / 1000
		conn += rr.rate * rr.conn
		in += rr.rate * rr.in
		out += rr.rate * rr.out
	}
	slots := float64(cfg.Workers)
	if cfg.Processing == ProcessingAsync {
		slots = float64(cfg.MaxConcurrency)
	}
	cores := math.Min(float64(cfg.Workers), size.VCPU)
	start := g.startFactor(n)
	run.stats.CPUTotal = size.VCPU * up
	run.stats.MemoryTotal = size.MemoryGB * 1024 * up
	run.stats.Bottleneck = "cpu"
	run.capacity = math.Inf(1)
	if lambda > 0 {
		limits := []struct {
			name      string
			cap, cost float64
		}{
			{"cpu", cores * 1000 * up, cpu},
			{"slots", slots * up, hold},
			{"connections", float64(cfg.MaxConnections) * up, conn},
			{"network-in", size.NetworkMbps * up, in},
			{"network-out", size.NetworkMbps * up, out},
		}
		// Each connection pool holds a connection for every call in flight
		// over it (v7).
		pools := map[int]float64{}
		var targets []int
		for _, rr := range run.routes {
			for x, sec := range rr.pool {
				if _, ok := pools[x]; !ok {
					targets = append(targets, x)
				}
				pools[x] += rr.rate * sec
			}
		}
		slices.Sort(targets)
		for _, x := range targets {
			if e := g.edge(n.ID, g.Nodes[x].ID); e != nil && e.Conn != nil {
				limits = append(limits, struct {
					name      string
					cap, cost float64
				}{"pool:" + g.Nodes[x].ID, float64(e.Conn.Pool) * up, pools[x]})
			}
		}
		// Each limit is capacity ÷ use per request, at this tick's mix.
		for _, l := range limits {
			if c := l.cap / (l.cost / lambda) * start; l.cost > 0 && c < run.capacity {
				run.capacity, run.stats.Bottleneck = c, l.name
			}
		}
	}
	// An event that cuts a component's capacity cuts its throughput (a
	// worker's from v11; it never targets applications).
	if f := factor(g.fx.capacity, n.ID); f != 1 {
		run.capacity *= f
	}
	if up == 0 {
		run.capacity = 0
	}

	accepted := lambda
	if limit := cfg.RateLimitRPS * up; slices.Contains(cfg.Middleware, "rate-limit") && accepted > limit {
		run.limited, accepted = accepted-limit, limit
	}
	meanWall := 0.0
	if lambda > 0 {
		meanWall = hold * 1000 / lambda
	}
	room := float64(cfg.Backlog) * up
	switch {
	case run.capacity == 0:
		run.rejected = accepted
	case accepted < run.capacity:
		// The usual queueing approximation, as for every component.
		run.rho = accepted / run.capacity
		run.served = accepted
		run.waitMs = meanWall * run.rho / (1 - run.rho)
		run.queued = accepted * run.waitMs / 1000
		// ponytail: below capacity a full backlog only caps the wait; turn
		// the excess away (Erlang loss) if small backlogs need it.
		if run.queued > room {
			run.queued, run.waitMs = room, room/accepted*1000
		}
	default:
		// Overload: the backlog fills, waiting requests drain at capacity,
		// and the rest are rejected.
		run.rho = accepted / run.capacity
		run.served = run.capacity
		run.rejected = accepted - run.capacity
		run.queued = room
		run.waitMs = room / run.capacity * 1000
	}
	run.rejected += run.limited

	share := 0.0
	if lambda > 0 {
		share = run.served / lambda
	}
	s := &run.stats
	s.Capacity = finite(run.capacity)
	s.WaitMs = run.waitMs
	s.Queued = run.queued
	s.Rejected = run.rejected
	mem := float64(cfg.Workers) * rt.WorkerMemoryMB * up
	for _, rr := range run.routes {
		v := rr.rate * share
		s.CPUUsed += v * rr.cpu / 1000
		s.Active += v * rr.wall / 1000
		s.Connections += v * rr.conn
		mem += v * rr.wall / 1000 * rr.route.MemoryMB
		if lambda > 0 {
			mem += run.queued * rr.rate / lambda * rr.route.MemoryMB
		}
	}
	s.Connections += run.queued
	s.MemoryMB = mem
	s.OutOfMemory = up > 0 && mem > s.MemoryTotal
	return run
}

// finishApp completes an instance's tick from its dependencies' success (s)
// and latency (t): it returns the success and latency of each request class
// and fills the per-route statistics. A request succeeds when it is served,
// finishes before the timeout, the handler does not fail, and every
// dependency call succeeds; waits are taken as exponential.
func (g *Game) finishApp(n *Node, run *appRun, s, t []vec) (vec, vec) {
	cfg := g.appConfig(n)
	var succ, lat, total vec
	st := &run.stats
	served := run.servedShare()
	for k := range run.routes {
		rr := &run.routes[k]
		ok, depMs := 1.0, 0.0
		for _, c := range rr.calls {
			if g.callModel() {
				ds, dt := g.finishCall(n, rr, c, s, t)
				ok *= ds
				depMs += dt
				continue
			}
			ds, dt := 0.0, 0.0
			for _, x := range c.to {
				ds += x.share * s[x.to][c.cls]
				dt += x.share * s[x.to][c.cls] * t[x.to][c.cls]
			}
			if ds > 0 {
				dt /= ds
			}
			ok *= ds
			depMs += dt
		}
		rr.ok, rr.depMs = ok, depMs
		p, late, ms := rr.outcome(served, run.waitMs, cfg.TimeoutMs)
		if run.out != nil {
			for _, name := range rr.names {
				run.out[name] = [2]float64{p, ms}
			}
		}
		rs := RouteStats{Endpoint: rr.route.Endpoint, RPS: rr.rate, Success: rr.rate * p,
			Timeouts: rr.rate * served * late, Rejected: rr.rate * (1 - served), LatencyMs: ms, NotFound: rr.notFound}
		rs.Errors = math.Max(0, rr.rate-rs.Success-rs.Timeouts-rs.Rejected)
		st.Routes = append(st.Routes, rs)
		st.Success += rs.Success
		st.Errors += rs.Errors
		st.Timeouts += rs.Timeouts
		for c := range nClass {
			succ[c] += rr.load[c] * p
			lat[c] += rr.load[c] * p * ms
			total[c] += rr.load[c]
		}
	}
	for c := range nClass {
		if succ[c] > 0 {
			lat[c] /= succ[c]
		}
		if total[c] > 0 {
			succ[c] /= total[c]
		}
	}
	lambda := run.served + run.rejected
	fail := 0.0
	if lambda > 0 {
		fail = 1 - st.Success/lambda
	}
	switch {
	case n.Down:
		st.Health = HealthStopped
	case st.OutOfMemory:
		st.Health = HealthUnhealthy
	case g.startFactor(n) < 1:
		st.Health = HealthStarting
	case fail > 0.2:
		st.Health = HealthUnhealthy
	case run.rho > 0.85 || fail > 0.01:
		st.Health = HealthDegraded
	default:
		st.Health = HealthHealthy
	}
	return succ, lat
}

// outcome is a route's chance to succeed under a timeout, the chance it
// times out, and the latency of a success, once its dependencies are solved.
// Waits are taken as exponential.
func (rr *routeRun) outcome(served, waitMs, timeout float64) (p, late, ms float64) {
	left := timeout - rr.own - rr.depMs
	late = 1.0
	wait := waitMs
	switch {
	case left <= 0:
	case wait <= 0:
		late = 0
	default:
		late = math.Exp(-left / wait)
		wait = math.Min(wait, left)
	}
	p = served * (1 - late) * (1 - rr.route.ErrorRate) * rr.ok
	ms = rr.own + wait + rr.depMs
	return p, late, ms
}

// configureApp replaces an application instance's configuration.
func (g *Game) configureApp(n *Node, cfg *AppConfig) error {
	if g.Rules.App == nil {
		return invalid("ruleset %s has no application configuration", g.Rules.Version)
	}
	if cfg == nil {
		return invalid("configure needs an application configuration")
	}
	if problems := g.Rules.ValidateApp(*cfg); len(problems) > 0 {
		return invalid("%s", strings.Join(problems, "; "))
	}
	n.App = cfg
	g.follow(n)
	// Its traffic components follow it (v6).
	for _, e := range g.Edges {
		if f := g.Node(e.From); e.To == n.ID && f.Kind == KindTraffic {
			g.adopt(f, n)
		}
	}
	return nil
}

// ValidateApp lists every problem with an application configuration; none
// means it is valid. Nothing is corrected.
func (r *Ruleset) ValidateApp(c AppConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	for label, v := range map[string]string{"name": c.Name, "framework": c.Framework, "version": c.Version,
		"environment": c.Environment, "protocol": c.Protocol, "interface": c.Interface, "server": c.Server} {
		if len(v) > maxNameLen {
			bad("%s must be at most %d characters", label, maxNameLen)
		}
	}
	if c.Port < 1 || c.Port > 65535 {
		bad("port must be between 1 and 65535")
	}
	if c.Processing != ProcessingSync && c.Processing != ProcessingAsync {
		bad("processing must be %q or %q", ProcessingSync, ProcessingAsync)
	}
	ints := []struct {
		label     string
		v, lo, hi int
	}{
		{"workers", c.Workers, 1, 64},
		{"max concurrency", c.MaxConcurrency, 1, 10_000},
		{"backlog", c.Backlog, 0, 100_000},
		{"max connections", c.MaxConnections, 1, 100_000},
	}
	for _, x := range ints {
		if x.v < x.lo || x.v > x.hi {
			bad("%s must be between %d and %d", x.label, x.lo, x.hi)
		}
	}
	if !(c.TimeoutMs >= 1 && c.TimeoutMs <= 60_000) {
		bad("timeout must be between 1 and 60000 ms")
	}
	seen := map[string]bool{}
	for _, m := range c.Middleware {
		known := false
		for _, x := range r.Middleware {
			known = known || x.Name == m
		}
		switch {
		case !known:
			bad("unknown middleware %q", m)
		case seen[m]:
			bad("middleware %q is listed twice", m)
		}
		seen[m] = true
	}
	if seen["rate-limit"] && !(c.RateLimitRPS > 0 && c.RateLimitRPS <= r.MaxTrafficRPS) {
		bad("the rate limit must be above 0 and at most %s requests/s", trimFloat(r.MaxTrafficRPS))
	}
	if len(c.Routes) == 0 || len(c.Routes) > maxRoutes {
		bad("declare 1 to %d routes", maxRoutes)
	}
	routes := map[string]bool{}
	for i, rt := range c.Routes {
		label := fmt.Sprintf("route %d", i+1)
		method, path, _ := strings.Cut(rt.Endpoint, " ")
		switch {
		case rt.Endpoint != CatchAll && (!methods[method] || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t")):
			bad("%s: endpoint must be like \"GET /path\" or %q", label, CatchAll)
		case routes[rt.Endpoint]:
			bad("route %s is declared twice", rt.Endpoint)
		default:
			label = "route " + rt.Endpoint
		}
		routes[rt.Endpoint] = true
		for name, v := range map[string]float64{"base time": rt.BaseMs, "CPU time": rt.CPUMs, "memory": rt.MemoryMB,
			"request size": rt.RequestKB, "response size": rt.ResponseKB} {
			if !(v >= 0 && v <= 1e6) {
				bad("%s: %s must be between 0 and 1000000", label, name)
			}
		}
		if rt.CPUMs > rt.BaseMs {
			bad("%s: CPU time cannot exceed base time", label)
		}
		if !(rt.CPUMs > 0) {
			bad("%s: CPU time must be above 0", label)
		}
		if !(rt.ErrorRate >= 0 && rt.ErrorRate <= 1) {
			bad("%s: error rate must be between 0%% and 100%%", label)
		}
		if !(rt.Share >= 0 && rt.Share <= 1) {
			bad("%s: typical share must be between 0%% and 100%%", label)
		}
		if len(rt.Calls) > 0 && r.Listeners == nil {
			bad("%s: calls to services need ruleset v7 or later", label)
		}
		if len(rt.Calls) > maxCalls {
			bad("%s: at most %d calls", label, maxCalls)
		}
		for k, cl := range rt.Calls {
			if name := strings.TrimSpace(cl.Service); name == "" || name != cl.Service || len(name) > maxNameLen {
				bad("%s: call %d needs a service name of 1 to %d characters", label, k+1, maxNameLen)
			}
			if !validEndpoint(cl.Endpoint) {
				bad("%s: call %d endpoint must be like \"GET /path\"", label, k+1)
			}
		}
		used := map[string]bool{}
		for _, d := range rt.Deps {
			switch {
			case !deps[d] || d == DepStream && r.Stream == nil:
				bad("%s: unknown dependency %q", label, d)
			case used[d]:
				bad("%s: dependency %q is listed twice", label, d)
			}
			used[d] = true
		}
	}
	if r.Client != nil && !slices.Contains(r.Protocols, c.Protocol) {
		bad("protocol must be one of %s", strings.Join(r.Protocols, ", "))
	}
	// From v6 an endpoint without a route fails with 404 instead.
	if len(c.Routes) > 0 && !routes[CatchAll] && r.Client == nil {
		bad("declare a %q route for endpoints without their own", CatchAll)
	}
	return p
}

// RulesetV5 is RulesetV4 with a configurable application instance
// (Phase 11, ADR-0017): its capacity emerges from CPU, workers or
// concurrency slots, connections, and network, under the routes' costs.
func RulesetV5() *Ruleset {
	r := RulesetV4()
	r.Version = "sandbox/v5"
	specs := map[string][3]float64{"small": {1, 1, 100}, "medium": {2, 4, 250}, "large": {4, 8, 500}}
	for i := range r.Sizes {
		s := specs[r.Sizes[i].Name]
		r.Sizes[i].VCPU, r.Sizes[i].MemoryGB, r.Sizes[i].NetworkMbps = s[0], s[1], s[2]
	}
	// From v5 an application's routes decide what fetches from storage.
	for i := range r.Traffic.Endpoints {
		r.Traffic.Endpoints[i].Storage = false
	}
	r.AppRuntime = AppRuntime{WorkerMemoryMB: 150, TLSHandshakeCPUMs: 2, RequestsPerConnection: 10, KeepAliveSeconds: 5, StartSeconds: 30}
	r.Middleware = []Middleware{
		{Name: "request-id", Label: "Request ID", Ms: 0.1, CPUMs: 0.05},
		{Name: "logging", Label: "Logging", Ms: 0.5, CPUMs: 0.3},
		{Name: "cors", Label: "CORS", Ms: 0.1, CPUMs: 0.05},
		{Name: "auth", Label: "Authentication", Ms: 2, CPUMs: 1},
		{Name: "validation", Label: "Validation", Ms: 1, CPUMs: 0.5},
		{Name: "compression", Label: "Compression", Ms: 2, CPUMs: 2},
		{Name: "rate-limit", Label: "Rate limiting", Ms: 0.2, CPUMs: 0.1},
	}
	r.Frameworks = []Framework{
		{Name: "Python / FastAPI", Interface: "ASGI", Server: "Uvicorn", Processing: ProcessingAsync, Workers: 2, MaxConcurrency: 500},
		{Name: "Python / Django", Interface: "WSGI", Server: "Gunicorn", Processing: ProcessingSync, Workers: 4, MaxConcurrency: 4},
		{Name: "Node.js / Express", Interface: "HTTP", Server: "Node", Processing: ProcessingAsync, Workers: 1, MaxConcurrency: 1000},
		{Name: "Node.js / Fastify", Interface: "HTTP", Server: "Node", Processing: ProcessingAsync, Workers: 1, MaxConcurrency: 1000},
		{Name: "Ruby / Rails", Interface: "Rack", Server: "Puma", Processing: ProcessingSync, Workers: 4, MaxConcurrency: 4},
		{Name: "Elixir / Phoenix", Interface: "Plug", Server: "Cowboy", Processing: ProcessingAsync, Workers: 2, MaxConcurrency: 5000},
		{Name: "Java / Spring Boot", Interface: "Servlet", Server: "Tomcat", Processing: ProcessingSync, Workers: 4, MaxConcurrency: 4},
		{Name: "Go / net/http", Interface: "HTTP", Server: "net/http", Processing: ProcessingAsync, Workers: 2, MaxConcurrency: 5000},
	}
	r.App = &AppConfig{
		Name: "API", Framework: "Python / Django", Version: "1.0.0", Environment: "production",
		Protocol: "HTTP/1.1", Port: 8000, Interface: "WSGI", Server: "Gunicorn",
		Processing: ProcessingSync, Workers: 4, MaxConcurrency: 4, Backlog: 100, MaxConnections: 1000,
		TimeoutMs: 2000, TLS: true, KeepAlive: true,
		Middleware: []string{"request-id", "logging", "auth", "validation"},
		Routes: []AppRoute{
			{Endpoint: "GET /products", BaseMs: 20, CPUMs: 15, MemoryMB: 2, RequestKB: 1, ResponseKB: 20, Deps: []string{DepCache}},
			{Endpoint: "GET /products/:id", BaseMs: 15, CPUMs: 12, MemoryMB: 1, RequestKB: 1, ResponseKB: 5, Deps: []string{DepCache}},
			{Endpoint: "GET /profile", BaseMs: 15, CPUMs: 10, MemoryMB: 1, RequestKB: 1, ResponseKB: 2, Deps: []string{DepCache}},
			{Endpoint: "GET /media/:id", BaseMs: 25, CPUMs: 10, MemoryMB: 4, RequestKB: 1, ResponseKB: 200, Deps: []string{DepCache, DepStorage}},
			{Endpoint: "POST /orders", BaseMs: 40, CPUMs: 30, MemoryMB: 3, RequestKB: 4, ResponseKB: 2, Deps: []string{DepDBWrite}},
			{Endpoint: "POST /login", BaseMs: 30, CPUMs: 25, MemoryMB: 1, RequestKB: 1, ResponseKB: 1, Deps: []string{DepCache}},
			{Endpoint: CatchAll, BaseMs: 20, CPUMs: 15, MemoryMB: 2, RequestKB: 1, ResponseKB: 5, Deps: []string{DepCache}},
		},
	}
	return r
}
