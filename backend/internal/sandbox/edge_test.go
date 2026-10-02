package sandbox

import (
	"reflect"
	"strings"
	"testing"
)

func newGameV13(t *testing.T) *Game {
	t.Helper()
	r := RulesetV13()
	r.Events = nil
	g := New(r, 1)
	g.FreeBuild = true
	g.Cash = 1e9
	return g
}

// named places an application called name, with a database and storage.
func named(t *testing.T, g *Game, name string) string {
	t.Helper()
	a := appCfg()
	a.Name = name
	id := must(t, g, Command{Type: CmdPlace, Kind: KindApp, App: &a})
	connect(t, g, id, place(t, g, KindDBPrimary))
	connect(t, g, id, place(t, g, KindStorage))
	return id
}

// front places traffic connected to node to, as a load test of rps.
func front(t *testing.T, g *Game, to string, rps float64) string {
	t.Helper()
	src := place(t, g, KindTraffic)
	connect(t, g, src, to)
	c := constant(*g.Node(src).Client, rps)
	must(t, g, Command{Type: CmdConfigure, Node: src, Client: &c})
	return src
}

func edgeOf(t *testing.T, g *Game, id string) EdgeNodeStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.Edge == nil {
		t.Fatalf("%s has no edge stats", id)
	}
	return *s.Edge
}

func TestTrafficAdoptsThroughALoadBalancer(t *testing.T) {
	g := newGameV13(t)
	lb := place(t, g, KindLB)
	src := place(t, g, KindTraffic)
	connect(t, g, src, lb)
	if c := g.Node(src).Client; c.Port != 443 || c.Scheme != SchemeHTTPS || len(c.Endpoints) != 0 {
		t.Fatalf("traffic takes the balancer's listener, with no app behind yet: %+v", c)
	}
	app := named(t, g, "API")
	connect(t, g, lb, app)
	if c := g.Node(src).Client; len(c.Endpoints) != 6 {
		t.Fatalf("once an app is behind, the traffic in front adopts its routes: %+v", c.Endpoints)
	}
	c := constant(*g.Node(src).Client, 20)
	must(t, g, Command{Type: CmdConfigure, Node: src, Client: &c})
	warm(g)
	total := 0.0
	for _, r := range appStats(t, g, app).Routes {
		total += r.RPS
	}
	if f := g.Last.Flow; f.ErrorRate > 1e-6 || !near(total, 20, 1e-6) {
		t.Fatalf("requests pass through the balancer, per endpoint: %+v", f)
	}
	c.Port = 8443
	must(t, g, Command{Type: CmdConfigure, Node: src, Client: &c})
	if p := trafficStats(t, g, src).Problem; !strings.Contains(p, "connection refused: port 8443, "+lb+" listens on 443") {
		t.Fatalf("the contract is the balancer's: %q", p)
	}
}

func TestBalancingAlgorithmsAndHealthChecks(t *testing.T) {
	g := newGameV13(t)
	lb := place(t, g, KindLB)
	small, large := named(t, g, "API"), named(t, g, "API")
	must(t, g, Command{Type: CmdResize, Node: large, Size: "large"})
	connect(t, g, lb, small)
	connect(t, g, lb, large)
	front(t, g, lb, 120)
	warm(g)
	share := func() (float64, float64) {
		st := edgeOf(t, g, lb).Targets
		return st[0].Share, st[1].Share
	}
	// A large sync app gains CPU, not worker slots: under twice the capacity.
	if s, l := share(); !(l > 1.5*s) {
		t.Fatalf("least connections follows capacity: %v vs %v", s, l)
	}
	must(t, g, Command{Type: CmdConfigure, Node: lb, LB: &LBConfig{Algorithm: AlgoRoundRobin, HealthChecks: true}})
	warm(g)
	if s, l := share(); !near(s, 0.5, 1e-9) || !near(l, 0.5, 1e-9) || g.Last.Flow.ErrorRate < 0.01 {
		t.Fatalf("round robin ignores size and overloads the small app: %v %v, errors %v", s, l, g.Last.Flow.ErrorRate)
	}

	// A crashed target: health checks pull it; without them it keeps its
	// share, and those requests fail.
	g.Events = []*Event{{Effect: EffectCrash, Start: 0, End: 1000, Hits: []Hit{{Node: small, Replicas: 1}}}}
	must(t, g, Command{Type: CmdConfigure, Node: lb, LB: &LBConfig{Algorithm: AlgoLeastConn, HealthChecks: true}})
	warm(g)
	if s, _ := share(); s != 0 {
		t.Fatalf("a health-checked balancer sends nothing to a dead target: %v", s)
	}
	checked := g.Last.Flow.ErrorRate
	must(t, g, Command{Type: CmdConfigure, Node: lb, LB: &LBConfig{Algorithm: AlgoLeastConn, HealthChecks: false}})
	warm(g)
	if s, _ := share(); !(s > 0.3) || !(g.Last.Flow.ErrorRate > checked) {
		t.Fatalf("without health checks a dead target draws its share and fails it: %v, errors %v", s, g.Last.Flow.ErrorRate)
	}
}

func TestAGatewayRoutesByPathAndRateLimits(t *testing.T) {
	g := newGameV13(t)
	gw := place(t, g, KindGateway)
	products, orders := named(t, g, "Products API"), named(t, g, "Orders API")
	connect(t, g, gw, products)
	connect(t, g, gw, orders)
	must(t, g, Command{Type: CmdConfigure, Node: gw, Gateway: &GatewayConfig{Routes: []GatewayRoute{
		{Prefix: "/products", Service: "Products API"}, {Prefix: "/orders", Service: "Orders API"}}}})
	src := front(t, g, gw, 20)
	c := *g.Node(src).Client
	c.Endpoints = []Weight{{"GET /products", 0.5}, {"POST /orders", 0.3}, {"GET /profile", 0.2}}
	must(t, g, Command{Type: CmdConfigure, Node: src, Client: &c})
	warm(g)
	if p, o := appStats(t, g, products).Routes, appStats(t, g, orders).Routes; len(p) != 1 || p[0].Endpoint != "GET /products" || len(o) != 1 || !near(o[0].RPS, 6, 1e-9) {
		t.Fatalf("each path reaches its service: %+v / %+v", p, o)
	}
	if st := edgeOf(t, g, gw); !near(st.NotFound, 4, 1e-9) || !near(g.Last.Flow.ErrorRate, 0.2, 1e-3) {
		t.Fatalf("an unrouted path is a 404 at the gateway: %+v, errors %v", st, g.Last.Flow.ErrorRate)
	}
	must(t, g, Command{Type: CmdConfigure, Node: gw, Gateway: &GatewayConfig{RateLimitRPS: 10, Routes: []GatewayRoute{{Prefix: "/", Service: "Products API"}}}})
	warm(g)
	if st := edgeOf(t, g, gw); !near(st.Limited, 10, 1e-9) {
		t.Fatalf("above its rate limit the gateway answers 429: %+v", st)
	}
}

func TestACDNAnswersCacheableEndpointsAtTheEdge(t *testing.T) {
	g := newGameV13(t)
	cdn := place(t, g, KindCDN)
	app := named(t, g, "API")
	connect(t, g, cdn, app)
	// The CDN takes the reads; the writes it cannot cache still cost the
	// app about 30 CPU-ms each, so keep them within its 34/s.
	front(t, g, cdn, 50)
	warm(g)
	st := edgeOf(t, g, cdn)
	// GET /products at its share of 100 req/s, over 1,000 objects with a
	// 300 s TTL.
	rps := 50 * g.Node(g.Edges[len(g.Edges)-1].From).Client.Endpoints[0].Share
	want := 1 - expNeg(300*rps/1000.0)
	if !near(st.Hits+st.Misses, 50, 1e-6) || !(st.HitRatio > 0.5) {
		t.Fatalf("the CDN answers most GETs: %+v", st)
	}
	var products RouteStats
	for _, r := range appStats(t, g, app).Routes {
		if r.Endpoint == "GET /products" {
			products = r
		}
	}
	if !near(products.RPS, rps*(1-want), 1e-6) {
		t.Fatalf("only misses reach the origin: %v, want %v", products.RPS, rps*(1-want))
	}
	if c := stats(t, g.Last, cdn).CostPerHour; !near(c, st.Cost, 1e-12) || !(c > 0) {
		t.Fatalf("a CDN is priced by use: %v", c)
	}
	if _, err := g.Apply(Command{Type: CmdScale, Node: cdn, Replicas: 2}); err == nil {
		t.Fatal("a CDN has no replicas")
	}
	if g.Last.Flow.ErrorRate > 1e-3 {
		t.Fatalf("a CDN in front serves every request: %v", g.Last.Flow.ErrorRate)
	}
}

func TestEdgeValidatesAndReplays(t *testing.T) {
	r := RulesetV13()
	if len(r.ValidateLB(LBConfig{Algorithm: "random"})) != 1 || len(r.ValidateGateway(GatewayConfig{})) != 1 || len(r.ValidateCDN(CDNConfig{Cacheable: []string{"x"}})) != 4 {
		t.Fatal("every edge problem is listed")
	}
	g := New(RulesetV13(), 6)
	g.FreeBuild = true
	cdn, lb := place(t, g, KindCDN), place(t, g, KindLB)
	app := named(t, g, "API")
	connect(t, g, cdn, lb)
	connect(t, g, lb, app)
	front(t, g, cdn, 30)
	for range 200 {
		g.Step()
	}
	must(t, g, Command{Type: CmdConfigure, Node: lb, LB: &LBConfig{Algorithm: AlgoRoundRobin}})
	for range 100 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Last, g.Last) || again.Cash != g.Cash {
		t.Fatalf("a v13 game must replay exactly: %v", err)
	}
}
