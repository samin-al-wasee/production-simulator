package sandbox

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func newGameV5(t *testing.T) *Game {
	t.Helper()
	r := RulesetV5()
	r.Events = nil
	for i := range r.Kinds {
		r.Kinds[i].UnlockedBy = ""
	}
	g := New(r, 1)
	g.Cash = 1e9
	return g
}

// appCfg returns a copy of the v5 starting application configuration.
func appCfg() AppConfig {
	var c AppConfig
	b, _ := json.Marshal(RulesetV5().App)
	_ = json.Unmarshal(b, &c)
	return c
}

func setApp(t *testing.T, g *Game, id string, c AppConfig) {
	t.Helper()
	must(t, g, Command{Type: CmdConfigure, Node: id, App: &c})
}

// send drives a load test of rps requests per second over the given
// endpoint shares, from one group.
func send(t *testing.T, g *Game, rps float64, endpoints ...Weight) {
	t.Helper()
	tc := loadTest(Pattern{Shape: ShapeConstant, RPS: rps})
	if len(endpoints) > 0 {
		tc.Groups = []TrafficGroup{{Name: "All", Share: 1, Regions: []Weight{{"asia", 1}}, Endpoints: endpoints}}
	}
	mustConfigure(t, g, tc)
}

// warm runs two ticks so the instance has started and has seen last
// tick's dependency latency.
func warm(g *Game) AppStats {
	g.Step()
	g.Step()
	for _, n := range g.Last.Flow.Nodes {
		if n.App != nil {
			return *n.App
		}
	}
	return AppStats{}
}

func appStats(t *testing.T, g *Game, id string) AppStats {
	t.Helper()
	return *stats(t, g.Last, id).App
}

func routeOf(a AppStats, endpoint string) RouteStats {
	for _, r := range a.Routes {
		if r.Endpoint == endpoint {
			return r
		}
	}
	return RouteStats{}
}

func TestAppStartsThenIsHealthy(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	first := appStats(t, g, app)
	if first.Health != HealthStarting {
		t.Fatalf("a new instance starts first: %+v", first)
	}
	after := warm(g)
	// Starting takes 30 of the tick's 300 seconds.
	if after.Health != HealthHealthy || !near(first.Capacity, after.Capacity*0.9, 1e-6) {
		t.Fatalf("then it serves at full capacity: %v then %+v", first.Capacity, after)
	}
}

func TestCapacityEmergesFromTheRoutesAndTheBottleneckMoves(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	send(t, g, 10, Weight{"GET /profile", 1})
	profile := warm(g)
	send(t, g, 10, Weight{"POST /login", 1})
	login := warm(g)
	// One vCPU: 1000 CPU-ms per second over the route's CPU-ms per request
	// (handler, middleware 1.85, TLS handshake 2 / 10 requests).
	if profile.Bottleneck != "cpu" || !near(profile.Capacity, 1000/(10+1.85+0.2), 1e-6) {
		t.Fatalf("GET /profile should be CPU bound at ~83 RPS: %+v", profile)
	}
	if login.Capacity >= profile.Capacity {
		t.Fatalf("a CPU-heavy route serves fewer requests: %v vs %v", login.Capacity, profile.Capacity)
	}

	// One sync worker: one request at a time, each holding it for its wall time.
	c := appCfg()
	c.Workers = 1
	setApp(t, g, app, c)
	if a := warm(g); a.Bottleneck != "slots" {
		t.Fatalf("one sync worker serves one request at a time: %+v", a)
	}
	c.MaxConnections = 2
	setApp(t, g, app, c)
	if a := warm(g); a.Bottleneck != "connections" {
		t.Fatalf("two connections should be the bottleneck: %+v", a)
	}
	c = appCfg()
	c.Routes[5].ResponseKB = 5000 // POST /login now returns 5 MB
	setApp(t, g, app, c)
	if a := warm(g); a.Bottleneck != "network-out" {
		t.Fatalf("5 MB responses should fill the network: %+v", a)
	}
}

func TestMiddlewareAddsTimeAndCPU(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	send(t, g, 10, Weight{"GET /profile", 1})
	c := appCfg()
	c.Middleware = nil
	setApp(t, g, app, c)
	bare := warm(g)
	c.Middleware = []string{"auth", "compression"}
	setApp(t, g, app, c)
	full := warm(g)
	dt := routeOf(full, "GET /profile").LatencyMs - routeOf(bare, "GET /profile").LatencyMs
	if dt < 4 || full.Capacity >= bare.Capacity {
		t.Fatalf("auth and compression add 4 ms and 3 CPU-ms: +%.2f ms, capacity %v → %v", dt, bare.Capacity, full.Capacity)
	}
}

func TestCatchAllRouteAndHandlerErrors(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	tc := loadTest(Pattern{Shape: ShapeConstant, RPS: 10})
	tc.Endpoints = append(tc.Endpoints, Endpoint{Method: "GET", Path: "/health"})
	tc.Groups = []TrafficGroup{{Name: "All", Share: 1, Regions: []Weight{{"asia", 1}},
		Endpoints: []Weight{{"GET /health", 0.5}, {"GET /profile", 0.5}}}}
	mustConfigure(t, g, tc)
	c := appCfg()
	c.Routes[2].ErrorRate = 0.2 // GET /profile
	setApp(t, g, app, c)
	a := warm(g)
	if r := routeOf(a, CatchAll); !near(r.RPS, 5, 1e-9) || r.Errors != 0 {
		t.Fatalf("GET /health has no route of its own, so the catch-all serves it: %+v", r)
	}
	if r := routeOf(a, "GET /profile"); !near(r.Errors, 1, 1e-9) {
		t.Fatalf("a 20%% handler error rate fails 1 of 5 RPS: %+v", r)
	}
}

func TestTimeoutsAndMissingDependencies(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	send(t, g, 2, Weight{"GET /profile", 1})
	c := appCfg()
	c.Processing, c.MaxConcurrency = ProcessingAsync, 100 // nothing queues
	c.Routes[2].BaseMs, c.Routes[2].CPUMs = 2500, 10      // GET /profile now takes 2.5 s
	c.Routes[4].Deps = []string{DepDBWrite, DepQueue}     // POST /orders publishes, but no queue
	setApp(t, g, app, c)
	a := warm(g)
	if r := routeOf(a, "GET /profile"); !near(r.Timeouts, 2, 1e-9) || r.Success != 0 {
		t.Fatalf("a handler slower than the 2 s timeout always times out: %+v", r)
	}
	send(t, g, 2, Weight{"POST /orders", 1})
	a = warm(g)
	if r := routeOf(a, "POST /orders"); !near(r.Errors, 2, 1e-6) {
		t.Fatalf("a call to a missing queue fails the request: %+v", r)
	}
}

func TestOverloadQueuesThenTimesOutThenRejects(t *testing.T) {
	g := newGameV5(t)
	basic(t, g)
	send(t, g, 20)
	light := warm(g)
	send(t, g, 45)
	busy := warm(g)
	if !(light.Queued < busy.Queued && light.WaitMs < busy.WaitMs) || busy.Rejected != 0 {
		t.Fatalf("more load, longer queue and wait, nothing rejected yet: %+v / %+v", light, busy)
	}
	send(t, g, 200)
	over := warm(g)
	c := g.Rules.App
	if !near(over.Queued, float64(c.Backlog), 1e-9) || !near(over.Rejected, 200-over.Capacity, 1e-6) {
		t.Fatalf("past capacity the backlog is full and the excess rejected: %+v", over)
	}
	// Waiting 100 requests at ~56 RPS takes ~1.8 s of a 2 s timeout.
	if over.Timeouts <= 0 || over.Success >= over.Capacity || over.Health != HealthUnhealthy {
		t.Fatalf("a full backlog makes requests time out: %+v", over)
	}
}

func TestRateLimitMiddlewareRejectsAboveItsLimit(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	send(t, g, 30)
	c := appCfg()
	c.Middleware = append(c.Middleware, "rate-limit")
	c.RateLimitRPS = 20
	setApp(t, g, app, c)
	if a := warm(g); !near(a.Rejected, 10, 1e-9) {
		t.Fatalf("30 RPS against a 20 RPS limit rejects 10: %+v", a)
	}
}

func TestSlowDatabaseSlowsTheAppAndSyncWorkersRunOut(t *testing.T) {
	g := newGameV5(t)
	app, db := basic(t, g)
	send(t, g, 20, Weight{"GET /profile", 1})
	before := warm(g)
	inject(g, &Event{Effect: EffectSlowdown, Target: db, Magnitude: 6})
	slow := warm(g)
	if routeOf(slow, "GET /profile").LatencyMs <= routeOf(before, "GET /profile").LatencyMs {
		t.Fatal("a slower database makes the route slower")
	}
	if slow.Bottleneck != "slots" || slow.Capacity >= before.Capacity {
		t.Fatalf("sync workers wait on the database and run out: %+v", slow)
	}
	c := appCfg()
	c.Processing, c.MaxConcurrency = ProcessingAsync, 500
	setApp(t, g, app, c)
	async := warm(g)
	if async.Bottleneck != "cpu" || async.Capacity <= slow.Capacity {
		t.Fatalf("async keeps serving while it waits: %+v", async)
	}
}

func TestKeepAliveTradesHandshakesForIdleConnections(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	send(t, g, 20)
	on := warm(g)
	c := appCfg()
	c.KeepAlive = false
	setApp(t, g, app, c)
	off := warm(g)
	if off.Capacity >= on.Capacity || off.Connections >= on.Connections {
		t.Fatalf("without keep-alive every request pays a TLS handshake and no connection idles: %+v / %+v", on, off)
	}
}

func TestOutOfMemoryCrashesAndRestarts(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	send(t, g, 40)
	c := appCfg()
	for i := range c.Routes {
		c.Routes[i].MemoryMB = 100
	}
	setApp(t, g, app, c)
	g.Step()
	if a := appStats(t, g, app); !a.OutOfMemory || a.Health != HealthUnhealthy {
		t.Fatalf("queued 50 MB requests exhaust 1 GB: %+v", a)
	}
	g.Step()
	if a := appStats(t, g, app); a.Health != HealthStopped || a.Success != 0 {
		t.Fatalf("the instance is down while it restarts: %+v", a)
	}
	g.Step()
	g.Step()
	// It starts again under the same load and runs out of memory again.
	if a := appStats(t, g, app); a.Success <= 0 || !a.OutOfMemory {
		t.Fatalf("after RestartTicks it serves again, then crashes again: %+v", a)
	}
	// Sixteen sync workers of 150 MB do not fit in 1 GB at all.
	c = appCfg()
	c.Workers = 16
	setApp(t, g, app, c)
	oom := false
	for range 6 {
		g.Step()
		oom = oom || appStats(t, g, app).OutOfMemory
	}
	if !oom {
		t.Fatal("too many workers for the memory crash the instance")
	}
}

func TestReplicasAddCapacity(t *testing.T) {
	g := newGameV5(t)
	app, _ := basic(t, g)
	one := warm(g)
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 3})
	three := warm(g)
	if !near(three.Capacity, 3*one.Capacity, 1e-6) || three.CPUTotal != 3 {
		t.Fatalf("each replica brings its own CPU: %+v", three)
	}
}

func TestAppValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*AppConfig)
		want string
	}{
		{"processing", func(c *AppConfig) { c.Processing = "threads" }, "processing must be"},
		{"no workers", func(c *AppConfig) { c.Workers = 0 }, "workers must be between 1 and 64"},
		{"negative backlog", func(c *AppConfig) { c.Backlog = -1 }, "backlog must be between"},
		{"timeout", func(c *AppConfig) { c.TimeoutMs = 0 }, "timeout must be between"},
		{"port", func(c *AppConfig) { c.Port = 70000 }, "port must be between"},
		{"long name", func(c *AppConfig) { c.Name = strings.Repeat("x", 41) }, "name must be at most 40"},
		{"middleware", func(c *AppConfig) { c.Middleware = []string{"magic"} }, `unknown middleware "magic"`},
		{"rate limit", func(c *AppConfig) { c.Middleware = []string{"rate-limit"} }, "rate limit must be above 0"},
		{"bad endpoint", func(c *AppConfig) { c.Routes[0].Endpoint = "products" }, `endpoint must be like "GET /path"`},
		{"duplicate route", func(c *AppConfig) { c.Routes[1].Endpoint = c.Routes[0].Endpoint }, "route GET /products is declared twice"},
		{"no catch-all", func(c *AppConfig) { c.Routes = c.Routes[:6] }, `declare a "*" route`},
		{"cpu above base", func(c *AppConfig) { c.Routes[0].CPUMs = 30 }, "CPU time cannot exceed base time"},
		{"negative memory", func(c *AppConfig) { c.Routes[0].MemoryMB = -1 }, "memory must be between"},
		{"error rate", func(c *AppConfig) { c.Routes[0].ErrorRate = 2 }, "error rate must be between"},
		{"unknown dep", func(c *AppConfig) { c.Routes[0].Deps = []string{"redis"} }, `unknown dependency "redis"`},
		{"free route", func(c *AppConfig) { c.Routes[0].BaseMs, c.Routes[0].CPUMs = 0, 0 }, "CPU time must be above 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGameV5(t)
			app := place(t, g, KindApp)
			c := appCfg()
			tc.edit(&c)
			_, err := g.Apply(Command{Type: CmdConfigure, Node: app, App: &c})
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error containing %q, got %v", tc.want, err)
			}
			if g.Node(app).App != nil {
				t.Fatal("a rejected configuration changes nothing")
			}
		})
	}
	v4 := New(RulesetV4(), 1)
	v4.Cash = 1e9
	app := place(t, v4, KindApp)
	c := appCfg()
	if _, err := v4.Apply(Command{Type: CmdConfigure, Node: app, App: &c}); err == nil {
		t.Fatal("v4 has no application configuration")
	}
}

func TestConfiguredAppReplaysThroughJSON(t *testing.T) {
	g := New(RulesetV5(), 5)
	app, _ := basic(t, g)
	series(g, 30)
	c := appCfg()
	c.Processing, c.MaxConcurrency, c.Workers = ProcessingAsync, 300, 2
	c.Routes[0].MemoryMB = 30
	setApp(t, g, app, c)
	send(t, g, 120)
	series(g, 10)
	mustConfigure(t, g, defaults())
	series(g, 300)

	b, _ := json.Marshal(g.Save())
	var s Save
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	r, err := Replay(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Last.Flow, g.Last.Flow) || !reflect.DeepEqual(r.Nodes, g.Nodes) || r.Cash != g.Cash {
		t.Fatalf("replay diverged:\n%+v\n%+v", r.Last.Meters, g.Last.Meters)
	}
}

// TestRulesetV5Balance pins the v5 economy. Without events it keeps the v2
// shape. Overload now collapses (queued requests time out), so events cost
// a design with little headroom more than in v4, and headroom pays more.
func TestRulesetV5Balance(t *testing.T) {
	calm := func() *Ruleset {
		r := RulesetV5()
		r.Events = nil
		return r
	}
	sensible, broke := week(t, calm, 1.5)
	if broke > 0 || sensible <= RulesetV5().StartingCash {
		t.Fatalf("a sensibly provisioned design should stay solvent and grow: cash %.0f, %d bankrupt", sensible, broke)
	}
	if none, _ := week(t, calm, 1); none >= sensible {
		t.Fatalf("no headroom should lose money at peaks: %.0f vs %.0f", none, sensible)
	}
	if over, _ := week(t, calm, 5); over >= 0.75*sensible {
		t.Fatalf("5× over-provisioning should cost at least 25%% of the profit: %.0f vs %.0f", over, sensible)
	}
	withEvents, broke := week(t, RulesetV5, 1.5)
	if broke > 0 || withEvents >= sensible {
		t.Fatalf("unanswered events should cost money without bankrupting: %.0f vs %.0f calm", withEvents, sensible)
	}
	if over, _ := week(t, RulesetV5, 5); over >= withEvents {
		t.Fatalf("with events, sensible headroom should still beat 5×: %.0f vs %.0f", over, withEvents)
	}
}

// A design that works in v4, with writes through a queue and reads through a
// cache and no direct database connection, works with the default routes.
func TestV4DesignsWorkWithTheDefaultRoutes(t *testing.T) {
	g := newGameV5(t)
	app, cache, db := place(t, g, KindApp), place(t, g, KindCache), place(t, g, KindDBPrimary)
	q, w, st := place(t, g, KindQueue), place(t, g, KindWorker), place(t, g, KindStorage)
	for _, e := range [][2]string{{InternetID, app}, {app, cache}, {cache, db}, {app, q}, {q, w}, {w, db}, {app, st}} {
		connect(t, g, e[0], e[1])
	}
	warm(g)
	if f := g.Last.Flow; f.ErrorRate > 1e-9 || stats(t, g.Last, q).Served <= 0 {
		t.Fatalf("reads through the cache and writes through the queue should all succeed: %+v", f)
	}
}
