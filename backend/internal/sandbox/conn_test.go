package sandbox

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func newGameV7(t *testing.T) *Game {
	t.Helper()
	r := RulesetV7()
	r.Events = nil
	for i := range r.Kinds {
		r.Kinds[i].UnlockedBy = ""
	}
	g := New(r, 1)
	g.Cash = 1e9
	return g
}

func edgeStats(t *testing.T, g *Game, from, to string) EdgeStats {
	t.Helper()
	for _, e := range g.Last.Flow.Edges {
		if e.From == from && e.To == to {
			return e
		}
	}
	t.Fatalf("no stats for %s → %s", from, to)
	return EdgeStats{}
}

// service places an application named name with the given routes, plus a
// database it can write to.
func service(t *testing.T, g *Game, name string, routes ...AppRoute) string {
	t.Helper()
	a := appCfg()
	a.Name, a.Routes = name, routes
	id := must(t, g, Command{Type: CmdPlace, Kind: KindApp, App: &a})
	connect(t, g, id, place(t, g, KindDBPrimary))
	return id
}

func TestConnectionsAdoptTheirListenerAndFollowIt(t *testing.T) {
	g := newGameV7(t)
	app, db, cache := place(t, g, KindApp), place(t, g, KindDBPrimary), place(t, g, KindCache)
	connect(t, g, app, db)
	connect(t, g, app, cache)
	if c := g.edge(app, db).Conn; c == nil || c.Protocol != "SQL" || c.Port != 5432 || c.Pool != 20 {
		t.Fatalf("an app connects to a database over SQL on 5432: %+v", c)
	}
	if c := g.edge(app, cache).Conn; c.Protocol != "RESP" || c.Port != 6379 {
		t.Fatalf("an app connects to a cache over RESP: %+v", c)
	}
	must(t, g, Command{Type: CmdConfigure, Node: db, Listener: &Listener{"SQL", 6432, true}})
	if c := g.edge(app, db).Conn; c.Port != 6432 || !c.TLS || c.Pool != 20 {
		t.Fatalf("a connection follows its target's listener and keeps its pool: %+v", c)
	}
	if _, err := g.Apply(Command{Type: CmdConfigure, Node: app, Listener: &Listener{"SQL", 1, false}}); err == nil {
		t.Fatal("an application's listener is its own configuration")
	}
	if _, err := g.Apply(Command{Type: CmdConfigure, From: app, To: db, Connection: &Connection{Protocol: "SQL", Port: 6432, Pool: 0, TimeoutMs: 1000}}); err == nil {
		t.Fatal("a pool of zero is rejected")
	}
}

func TestABrokenConnectionFailsItsCalls(t *testing.T) {
	g := newGameV7(t)
	app := appWithDeps(t, g)
	traffic(t, g, app, constant(client(), 10))
	db := g.Edges[0].To
	warm(g)
	if f := g.Last.Flow; f.ErrorRate > 1e-6 {
		t.Fatalf("a working design serves: %v", f.ErrorRate)
	}
	c := *g.edge(app, db).Conn
	c.Port = 5433
	must(t, g, Command{Type: CmdConfigure, From: app, To: db, Connection: &c})
	warm(g)
	es := edgeStats(t, g, app, db)
	if !strings.Contains(es.Problem, "connection refused") || es.Errors != es.RPS || es.RPS == 0 {
		t.Fatalf("a refused connection fails every call over it: %+v", es)
	}
	if s := stats(t, g.Last, db); s.Offered != 0 {
		t.Fatalf("refused calls never reach the database: %v", s.Offered)
	}
	if g.Last.Flow.SuccessRPS > 1e-9 {
		t.Fatalf("every route needs the database here: %v", g.Last.Flow.SuccessRPS)
	}
}

func TestServicesCallEachOthersEndpoints(t *testing.T) {
	g := newGameV7(t)
	orders := service(t, g, "Orders API",
		AppRoute{Endpoint: "POST /orders", BaseMs: 200, CPUMs: 10, MemoryMB: 1, RequestKB: 1, ResponseKB: 1, Deps: []string{DepDBWrite}},
		AppRoute{Endpoint: "GET /orders/:id", BaseMs: 10, CPUMs: 5, MemoryMB: 1, RequestKB: 1, ResponseKB: 1})
	front := service(t, g, "Storefront API",
		AppRoute{Endpoint: "POST /checkout", BaseMs: 20, CPUMs: 10, MemoryMB: 1, RequestKB: 1, ResponseKB: 1,
			Calls: []Call{{Service: "Orders API", Endpoint: "POST /orders"}}},
		AppRoute{Endpoint: "GET /", BaseMs: 10, CPUMs: 5, MemoryMB: 1, RequestKB: 1, ResponseKB: 1})
	connect(t, g, front, orders)
	c := constant(client(), 8)
	c.Endpoints = []Weight{{"POST /checkout", 0.5}, {"GET /", 0.5}}
	traffic(t, g, front, c)
	warm(g)
	routes := appStats(t, g, orders).Routes
	if len(routes) != 1 || routes[0].Endpoint != "POST /orders" || !near(routes[0].RPS, 4, 1e-9) {
		t.Fatalf("the orders service receives exactly what the storefront calls: %+v", routes)
	}
	var checkout RouteStats
	for _, r := range appStats(t, g, front).Routes {
		if r.Endpoint == "POST /checkout" {
			checkout = r
		}
	}
	// The connection's 1 s timeout cuts off the slow tail of the calls
	// (latency taken as exponential): exp(−1000 / call latency).
	es := edgeStats(t, g, front, orders)
	if want := 4 * (1 - expNeg(1000/es.LatencyMs)); checkout.LatencyMs < 200+g.Rules.NetworkHopMs || !near(checkout.Success, want, 1e-6) {
		t.Fatalf("a call's latency, hop, and timeout reach its caller's route: %+v, want success %v", checkout, want)
	}
	if !near(es.RPS, 4, 1e-9) || !near(es.Errors, 4-checkout.Success, 1e-6) || es.LatencyMs < 200 {
		t.Fatalf("the connection carries the calls: %+v", es)
	}

	// Without the service, the call fails and so does its route.
	must(t, g, Command{Type: CmdDisconnect, From: front, To: orders})
	warm(g)
	for _, r := range appStats(t, g, front).Routes {
		if r.Endpoint == "POST /checkout" && r.Success > 1e-9 {
			t.Fatalf("a call with no service to reach fails: %+v", r)
		}
	}
}

func TestAsyncCallsDoNotWaitOrFail(t *testing.T) {
	g := newGameV7(t)
	notify := service(t, g, "Notify API",
		AppRoute{Endpoint: "POST /notify", BaseMs: 500, CPUMs: 5, MemoryMB: 1, RequestKB: 1, ResponseKB: 1, ErrorRate: 1})
	front := service(t, g, "Storefront API",
		AppRoute{Endpoint: "POST /signup", BaseMs: 20, CPUMs: 10, MemoryMB: 1, RequestKB: 1, ResponseKB: 1,
			Calls: []Call{{Service: "Notify API", Endpoint: "POST /notify", Async: true}}})
	connect(t, g, front, notify)
	c := constant(client(), 10)
	c.Endpoints = []Weight{{"POST /signup", 1}}
	traffic(t, g, front, c)
	warm(g)
	if r := appStats(t, g, front).Routes[0]; !near(r.Success, 10, 1e-6) || r.LatencyMs > 100 {
		t.Fatalf("an async call neither waits nor fails its caller: %+v", r)
	}
	if n := appStats(t, g, notify); !near(n.Routes[0].RPS, 10, 1e-9) || n.Success != 0 {
		t.Fatalf("the async call still loads its service: %+v", n)
	}
}

func TestAConnectionPoolCanBeTheBottleneck(t *testing.T) {
	g := newGameV7(t)
	app := appWithDeps(t, g)
	a := appCfg()
	a.Processing, a.MaxConcurrency = ProcessingAsync, 1000
	setApp(t, g, app, a)
	db := g.Edges[0].To
	c := *g.edge(app, db).Conn
	c.Pool = 1
	must(t, g, Command{Type: CmdConfigure, From: app, To: db, Connection: &c})
	g.Events = []*Event{{Effect: EffectSlowdown, Start: 0, End: 1000, Magnitude: 4, Target: db}}
	traffic(t, g, app, constant(client(), 30))
	for range 4 {
		g.Step()
	}
	if b := appStats(t, g, app).Bottleneck; b != "pool:"+db {
		t.Fatalf("one connection to a slow database limits the app: %s", b)
	}
}

func TestConnectionRetriesRecoverAndAmplify(t *testing.T) {
	g := newGameV7(t)
	flaky := service(t, g, "Flaky API",
		AppRoute{Endpoint: "GET /x", BaseMs: 5, CPUMs: 1, MemoryMB: 1, RequestKB: 1, ResponseKB: 1, ErrorRate: 0.5})
	front := service(t, g, "Front API",
		AppRoute{Endpoint: "GET /", BaseMs: 5, CPUMs: 1, MemoryMB: 1, RequestKB: 1, ResponseKB: 1,
			Calls: []Call{{Service: "Flaky API", Endpoint: "GET /x"}}})
	connect(t, g, front, flaky)
	c := constant(client(), 10)
	c.Endpoints = []Weight{{"GET /", 1}}
	traffic(t, g, front, c)
	once := warm(g).Routes[0].Success
	conn := *g.edge(front, flaky).Conn
	conn.Retries = 2
	must(t, g, Command{Type: CmdConfigure, From: front, To: flaky, Connection: &conn})
	warm(g)
	st := appStats(t, g, front)
	if !(st.Routes[0].Success > once*1.5) {
		t.Fatalf("retries over the connection recover failed calls: %v vs %v", st.Routes[0].Success, once)
	}
	if rps := appStats(t, g, flaky).Routes[0].RPS; !near(rps, 10*(1+0.5+0.25), 0.1) {
		t.Fatalf("retries multiply the load on the service: %v", rps)
	}
}

func TestCallsValidateAndReplay(t *testing.T) {
	a := appCfg()
	a.Routes[0].Calls = []Call{{Service: "", Endpoint: "FETCH /x"}}
	p := strings.Join(RulesetV7().ValidateApp(a), "\n")
	if !strings.Contains(p, "service name") || !strings.Contains(p, "endpoint must be like") {
		t.Fatalf("calls are validated: %s", p)
	}
	a.Routes[0].Calls = []Call{{Service: "Orders API", Endpoint: "POST /orders"}}
	if p := RulesetV6().ValidateApp(a); len(p) == 0 {
		t.Fatal("v6 has no calls")
	}

	g := New(RulesetV7(), 4)
	app := appWithDeps(t, g)
	src := traffic(t, g, app, client())
	for range 300 {
		g.Step()
	}
	db := g.Edges[1].To
	c := *g.edge(app, db).Conn
	c.Retries, c.TimeoutMs = 1, 200
	must(t, g, Command{Type: CmdConfigure, From: app, To: db, Connection: &c})
	must(t, g, Command{Type: CmdDisconnect, From: src, To: app})
	for range 300 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.Last, g.Last) || again.Cash != g.Cash || !reflect.DeepEqual(again.Edges, g.Edges) {
		t.Fatal("a v7 game must replay exactly")
	}
}

func TestRulesetV7Balance(t *testing.T) {
	calm := func() *Ruleset {
		r := RulesetV7()
		r.Events = nil
		return r
	}
	sensible, broke := week(t, calm, 1.5)
	if broke > 0 || sensible <= RulesetV7().StartingCash {
		t.Fatalf("a sensibly provisioned design should stay solvent and grow: cash %.0f, %d bankrupt", sensible, broke)
	}
	if over, _ := week(t, calm, 5); over >= 0.75*sensible {
		t.Fatalf("5× over-provisioning should cost at least 25%% of the profit: %.0f vs %.0f", over, sensible)
	}
}

func expNeg(x float64) float64 { return math.Exp(-x) }

// The microservice templates make a working system: a storefront calling a
// catalog, orders (which notifies asynchronously), and payments.
func TestMicroserviceTemplatesServeEndToEnd(t *testing.T) {
	g := newGameV7(t)
	r := g.Rules
	stack := r.AppStacks[4].App // Go
	byName := map[string]string{}
	for _, ty := range r.AppTypes {
		switch ty.Name {
		case "Storefront", "Catalog", "Orders", "Payments", "Notifications":
			a := stack
			a.Name, a.Routes = ty.Name+" API", ty.Routes
			byName[ty.Name] = must(t, g, Command{Type: CmdPlace, Kind: KindApp, App: &a})
		}
	}
	for _, from := range []string{"Storefront", "Catalog", "Orders", "Payments"} {
		db := place(t, g, KindDBPrimary)
		connect(t, g, byName[from], db)
	}
	for _, e := range [][2]string{{"Storefront", "Catalog"}, {"Storefront", "Orders"}, {"Storefront", "Payments"}, {"Orders", "Notifications"}} {
		connect(t, g, byName[e[0]], byName[e[1]])
	}
	src := place(t, g, KindTraffic)
	connect(t, g, src, byName["Storefront"])
	load := constant(*g.Node(src).Client, 20)
	must(t, g, Command{Type: CmdConfigure, Node: src, Client: &load})
	warm(g)
	if f := g.Last.Flow; f.ErrorRate > 0.01 || f.SuccessRPS < 19.8 {
		t.Fatalf("the microservice templates should serve end to end: errors %v, success %v", f.ErrorRate, f.SuccessRPS)
	}
	if rps := appStats(t, g, byName["Notifications"]).Routes[0].RPS; !near(rps, 20*0.15, 1e-6) {
		t.Fatalf("every checkout's order notifies once: %v", rps)
	}
}
