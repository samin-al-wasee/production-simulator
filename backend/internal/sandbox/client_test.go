package sandbox

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

func newGameV6(t *testing.T) *Game {
	t.Helper()
	r := RulesetV6()
	r.Events = nil
	for i := range r.Kinds {
		r.Kinds[i].UnlockedBy = ""
	}
	g := New(r, 1)
	g.Cash = 1e9
	return g
}

// client is the default client with the web mix the tests reason about.
func client() ClientConfig {
	c := *RulesetV6().Client
	c.Endpoints = []Weight{{"GET /products", 0.35}, {"GET /products/:id", 0.2}, {"GET /profile", 0.1},
		{"GET /media/:id", 0.15}, {"POST /orders", 0.12}, {"POST /login", 0.08}}
	return c
}

// traffic places a traffic component, connects it to app, and then
// configures it, so c replaces what it adopted from the app.
func traffic(t *testing.T, g *Game, app string, c ClientConfig) string {
	t.Helper()
	id := place(t, g, KindTraffic)
	if app != "" {
		connect(t, g, id, app)
	}
	must(t, g, Command{Type: CmdConfigure, Node: id, Client: &c})
	return id
}

func trafficStats(t *testing.T, g *Game, id string) ClientStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.Traffic == nil {
		t.Fatalf("%s has no traffic stats", id)
	}
	return *s.Traffic
}

// constant is a load test of rps requests per second.
func constant(c ClientConfig, rps float64) ClientConfig {
	c.Source = SourceConfigured
	c.Pattern = Pattern{Shape: ShapeConstant, RPS: rps}
	return c
}

// app places an application instance with a database and storage behind it.
func appWithDeps(t *testing.T, g *Game) string {
	t.Helper()
	app, db, st := place(t, g, KindApp), place(t, g, KindDBPrimary), place(t, g, KindStorage)
	connect(t, g, app, db)
	connect(t, g, app, st)
	return app
}

func TestV6StartsEmptyAndTrafficIsAComponent(t *testing.T) {
	g := New(RulesetV6(), 1)
	if len(g.Nodes) != 0 || g.Last.Flow.RPS != 0 {
		t.Fatalf("a v6 game starts empty: %d nodes", len(g.Nodes))
	}
	for _, k := range []string{KindInternet, KindCDN, KindLB, KindGateway} {
		if _, ok := g.Rules.Kind(k); ok {
			t.Errorf("v6 should not offer %s yet", k)
		}
	}
	g.Cash = 1e9
	src := place(t, g, KindTraffic)
	app, other := place(t, g, KindApp), place(t, g, KindApp)
	db := place(t, g, KindDBPrimary)
	for _, c := range []Command{
		{Type: CmdConnect, From: src, To: db},
		{Type: CmdConnect, From: app, To: src},
		{Type: CmdScale, Node: src, Replicas: 2},
		{Type: CmdResize, Node: src, Size: "large"},
	} {
		if _, err := g.Apply(c); err == nil {
			t.Errorf("%+v should be rejected", c)
		}
	}
	connect(t, g, src, app)
	if _, err := g.Apply(Command{Type: CmdConnect, From: src, To: other}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("a traffic component connects to exactly one component: %v", err)
	}
	// Many traffic components may feed one application.
	connect(t, g, place(t, g, KindTraffic), app)
	if s := stats(t, g.Last, src); s.CostPerHour != 0 || g.Complexity() != 1+1+3+0.5*2 {
		t.Fatalf("traffic costs nothing and adds only its connection: cost %v, complexity %v", s.CostPerHour, g.Complexity())
	}
}

func TestConnectingAdoptsTheApp(t *testing.T) {
	g := newGameV6(t)
	src := place(t, g, KindTraffic)
	if c := g.clientConfig(g.Node(src)); len(c.Endpoints) != 0 || g.Node(src).Client != nil {
		t.Fatalf("a new traffic component asks for nothing: %+v", c)
	}
	app := appWithDeps(t, g)
	a := appCfg()
	a.Protocol, a.Port, a.TLS, a.KeepAlive = "HTTP/2", 9000, false, false
	setApp(t, g, app, a)
	connect(t, g, src, app)
	c := g.Node(src).Client
	if c == nil || c.Protocol != "HTTP/2" || c.Port != 9000 || c.Scheme != SchemeHTTP || c.KeepAlive {
		t.Fatalf("connecting adopts the app's protocol, port, scheme, and keep-alive: %+v", c)
	}
	if len(c.Endpoints) != 6 || c.Endpoints[0].Name != "GET /products" || c.Endpoints[5].Name != "POST /login" {
		t.Fatalf("one endpoint per route, the catch-all aside: %+v", c.Endpoints)
	}
	if p := g.Rules.ValidateClient(*c); len(p) != 0 || c.TimeoutMs != g.Rules.Client.TimeoutMs || c.ClientType != ClientWeb {
		t.Fatalf("the adopted configuration is valid and keeps the client's own choices: %v %+v", p, c)
	}
	if ts := trafficStats(t, g, src); ts.Problem != "" || !(ts.Success > 0) {
		t.Fatalf("an adopted component is served: %+v", ts)
	}

	// Reconfiguring the app updates its traffic; losing the connection,
	// by disconnecting or removing the app, returns it to asking for nothing.
	a.Port, a.TLS = 8443, true
	a.Routes = append([]AppRoute{{Endpoint: "GET /health", BaseMs: 1, CPUMs: 1}}, a.Routes...)
	setApp(t, g, app, a)
	if c := g.Node(src).Client; c.Port != 8443 || c.Scheme != SchemeHTTPS || len(c.Endpoints) != 7 || c.Endpoints[0].Name != "GET /health" {
		t.Fatalf("a connected component follows its app: %+v", c)
	}
	named := *g.Node(src).Client
	named.Name, named.TimeoutMs = "Shoppers", 3000
	must(t, g, Command{Type: CmdConfigure, Node: src, Client: &named})
	must(t, g, Command{Type: CmdDisconnect, From: src, To: app})
	d := g.Rules.Client
	if c := g.Node(src).Client; len(c.Endpoints) != 0 || c.Port != d.Port || c.Protocol != d.Protocol || c.Name != "Shoppers" || c.TimeoutMs != 3000 {
		t.Fatalf("a disconnected component asks for nothing and keeps who its clients are: %+v", c)
	}
	connect(t, g, src, app)
	must(t, g, Command{Type: CmdRemove, Node: app})
	if c := g.Node(src).Client; len(c.Endpoints) != 0 || c.Port != d.Port {
		t.Fatalf("removing the app releases its traffic: %+v", c)
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Node(src).Client, g.Node(src).Client) {
		t.Fatalf("adoption and release replay: %v", err)
	}
}

func TestMarketVolumeIsSplitBySegment(t *testing.T) {
	g := newGameV6(t)
	app := appWithDeps(t, g)
	web := traffic(t, g, app, client())
	_, hour := g.clock()
	market := g.Users * g.Rules.ActiveShare * g.engagement() * diurnal(hour)
	if got, want := trafficStats(t, g, web).RPS, market*0.7*0.35; !near(got, want, 1e-9) {
		t.Fatalf("web in asia takes 70%% × 35%% of the market: %v, want %v", got, want)
	}

	// A second component of the same segment halves it; another segment adds.
	g2 := newGameV6(t)
	app2 := appWithDeps(t, g2)
	a, b := traffic(t, g2, app2, client()), traffic(t, g2, app2, client())
	m := client()
	m.ClientType, m.Region = ClientMobile, "europe"
	mobile := traffic(t, g2, app2, m)
	ra, rb, rm := trafficStats(t, g2, a).RPS, trafficStats(t, g2, b).RPS, trafficStats(t, g2, mobile).RPS
	if !near(ra, rb, 1e-12) || !near(rm/(ra*2), (0.2*0.25)/(0.7*0.35), 1e-9) {
		t.Fatalf("segment split: %v %v %v", ra, rb, rm)
	}
	if f := g2.Last.Flow; !near(f.RPS, ra+rb+rm, 1e-9) || len(f.Traffic.Components) != 3 || len(f.Traffic.ClientTypes) != 2 {
		t.Fatalf("meters aggregate every traffic component: %+v", f.Traffic)
	}
}

func TestContractMismatchesFailAtTheDoor(t *testing.T) {
	cases := []struct {
		name   string
		change func(*ClientConfig, *AppConfig)
		want   string
	}{
		{"protocol", func(c *ClientConfig, _ *AppConfig) { c.Protocol = "HTTP/2" }, "protocol error"},
		{"port", func(c *ClientConfig, _ *AppConfig) { c.Port = 8080 }, "connection refused"},
		{"https to plain", func(_ *ClientConfig, a *AppConfig) { a.TLS = false }, "TLS handshake failed"},
		{"http to TLS", func(c *ClientConfig, _ *AppConfig) { c.Scheme = SchemeHTTP }, "TLS required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGameV6(t)
			app := appWithDeps(t, g)
			c, a := constant(client(), 10), appCfg()
			tc.change(&c, &a)
			setApp(t, g, app, a)
			src := traffic(t, g, app, c)
			warm(g)
			ts := trafficStats(t, g, src)
			if !strings.Contains(ts.Problem, tc.want) || ts.Refused != 10 || g.Last.Flow.SuccessRPS != 0 {
				t.Fatalf("want %q and every request refused: %+v", tc.want, ts)
			}
			if s := stats(t, g.Last, app); s.Offered != 0 {
				t.Fatalf("refused requests never reach the app: offered %v", s.Offered)
			}
		})
	}

	g := newGameV6(t)
	src := traffic(t, g, "", constant(client(), 10))
	if ts := trafficStats(t, g, src); ts.Problem != "not connected" || ts.Refused != 10 {
		t.Fatalf("an unconnected component fails everything: %+v", ts)
	}
}

func TestMissingRouteIsNotFound(t *testing.T) {
	g := newGameV6(t)
	app := appWithDeps(t, g)
	a := appCfg()
	a.Routes = a.Routes[:1] // only GET /products, no catch-all
	setApp(t, g, app, a)
	src := traffic(t, g, app, constant(client(), 100))
	st := warm(g)
	ts := trafficStats(t, g, src)
	if !near(ts.NotFound, 65, 1e-9) || !near(ts.Success, 35, 1e-6) {
		t.Fatalf("the 65%% without a route should fail with 404: %+v", ts)
	}
	found := false
	for _, r := range st.Routes {
		found = found || (r.Endpoint == "POST /orders" && r.NotFound && r.Success == 0)
	}
	if !found {
		t.Fatalf("the app should report the missing route: %+v", st.Routes)
	}
}

func TestClientTimeoutAndKeepAlive(t *testing.T) {
	g := newGameV6(t)
	app := appWithDeps(t, g)
	a := appCfg()
	a.Workers = 1
	setApp(t, g, app, a)
	c := constant(client(), 20)
	impatient := c
	impatient.TimeoutMs = 50
	slow, fast := traffic(t, g, app, c), traffic(t, g, app, impatient)
	st := warm(g)
	patient, short := trafficStats(t, g, slow), trafficStats(t, g, fast)
	if !(short.Timeouts > patient.Timeouts && short.Success < patient.Success) {
		t.Fatalf("a shorter client timeout fails more: %+v vs %+v", short, patient)
	}
	if st.Success <= patient.Success+short.Success+1e-9 {
		t.Fatalf("the server counts as served some requests their client gave up on: server %v, clients %v + %v", st.Success, patient.Success, short.Success)
	}

	// A client without keep-alive opens a connection per request.
	g2 := newGameV6(t)
	app2 := appWithDeps(t, g2)
	traffic(t, g2, app2, c)
	kept := warm(g2).Connections
	g3 := newGameV6(t)
	app3 := appWithDeps(t, g3)
	noKA := c
	noKA.KeepAlive = false
	traffic(t, g3, app3, noKA)
	if open := warm(g3).Connections; open >= kept {
		t.Fatalf("without keep-alive no connection idles: %v vs %v", open, kept)
	}
	if cpu, base := appStats(t, g3, app3).CPUUsed, appStats(t, g2, app2).CPUUsed; cpu <= base {
		t.Fatalf("without keep-alive every request pays the TLS handshake: %v vs %v", cpu, base)
	}
}

func TestEachAppMixesItsOwnInputs(t *testing.T) {
	g := newGameV6(t)
	reads, writes := appWithDeps(t, g), appWithDeps(t, g)
	r, w := constant(client(), 10), constant(client(), 10)
	r.Endpoints = []Weight{{"GET /products", 1}}
	w.Endpoints = []Weight{{"POST /orders", 1}}
	traffic(t, g, reads, r)
	traffic(t, g, writes, w)
	if rs, ws := warm(g).Routes, appStats(t, g, writes).Routes; len(rs) != 1 || rs[0].Endpoint != "GET /products" || len(ws) != 1 || ws[0].Endpoint != "POST /orders" {
		t.Fatalf("each app serves its own inputs' endpoints: %+v / %+v", rs, ws)
	}
}

func TestRetriesPerComponent(t *testing.T) {
	g := newGameV6(t)
	app := appWithDeps(t, g)
	a := appCfg()
	a.Routes[0].ErrorRate = 0.5
	setApp(t, g, app, a)
	c := constant(client(), 10)
	c.Endpoints = []Weight{{"GET /products", 1}}
	retrying := c
	retrying.Retries = 2
	once, again := traffic(t, g, app, c), traffic(t, g, app, retrying)
	warm(g)
	o, r := trafficStats(t, g, once), trafficStats(t, g, again)
	if o.RetryRPS != 0 || !near(r.RetryRPS, 10*(0.5+0.25), 0.05) || !(r.Success > o.Success) {
		t.Fatalf("only the retrying component retries, and recovers more: %+v vs %+v", r, o)
	}
}

func TestTrafficEventsHitChosenComponents(t *testing.T) {
	g := newGameV6(t)
	app := appWithDeps(t, g)
	ids := []string{traffic(t, g, app, client()), traffic(t, g, app, client()), traffic(t, g, app, client())}
	card := RulesetV6().Events[0] // viral surge
	for seed := uint64(1); seed < 20; seed++ {
		e := g.deal(card, rand.New(rand.NewPCG(seed, 1)))
		if len(e.Targets) == 0 || len(e.Targets) > 3 || e.Target != "" {
			t.Fatalf("a traffic card picks one or more traffic components: %+v", e)
		}
	}
	before := trafficStats(t, g, ids[1]).RPS
	g.Events = []*Event{{Card: card.Name, Effect: EffectTraffic, Start: 0, End: 100, Magnitude: 3, Targets: []string{ids[0]}}}
	g.Last = g.preview()
	if hit, missed := trafficStats(t, g, ids[0]).RPS, trafficStats(t, g, ids[1]).RPS; !near(hit, 3*before, 1e-9) || !near(missed, before, 1e-9) {
		t.Fatalf("the surge acts only on its target: %v %v (was %v)", hit, missed, before)
	}
	crash := RulesetV6().Events[3]
	for seed := uint64(1); seed < 20; seed++ {
		if e := g.deal(crash, rand.New(rand.NewPCG(seed, 1))); e != nil && strings.HasPrefix(e.Target, KindTraffic) {
			t.Fatalf("component cards never hit a traffic component: %+v", e)
		}
	}
}

func TestSentimentHoldsWithoutTrafficAndLoadTestsPause(t *testing.T) {
	g := newGameV6(t)
	for range 12 {
		g.Step()
	}
	if g.Satisfaction != 50 || g.Popularity != g.Rules.StartingPopularity {
		t.Fatalf("an empty world must not gain sentiment: %v %v", g.Satisfaction, g.Popularity)
	}
	app := appWithDeps(t, g)
	traffic(t, g, app, client())
	traffic(t, g, app, constant(client(), 5))
	g.Step()
	if m := g.Last.Meters; !m.LoadTest || m.RevenuePerHour != 0 {
		t.Fatalf("any load-testing component puts the game in a load test: %+v", m)
	}
}

func TestClientValidation(t *testing.T) {
	r := RulesetV6()
	if p := r.ValidateClient(client()); len(p) != 0 {
		t.Fatalf("the default client must be valid: %v", p)
	}
	c := client()
	c.Name, c.ClientType, c.Region, c.Protocol, c.Scheme = " ", "fax", "mars", "SMTP", "ftp"
	c.Port, c.TimeoutMs, c.Retries, c.Source = 0, 0, 9, "tv"
	c.Endpoints = []Weight{{"FETCH /x", 0.5}, {"GET /a", 0.2}, {"GET /a", 0.2}}
	p := strings.Join(r.ValidateClient(c), "\n")
	for _, want := range []string{"name", "client type", "region", "protocol", "scheme", "port", "timeout", "retries", "source", "endpoint 1", "listed twice", "sum to 100%"} {
		if !strings.Contains(p, want) {
			t.Errorf("missing problem %q in:\n%s", want, p)
		}
	}
	a := appCfg()
	a.Protocol = "SMTP"
	a.Routes = a.Routes[:1]
	if p := strings.Join(r.ValidateApp(a), "\n"); !strings.Contains(p, "protocol") || strings.Contains(p, CatchAll) {
		t.Fatalf("v6 checks the app's protocol and makes the catch-all optional: %s", p)
	}
}

func TestV6ReplaysAndKeepsV5(t *testing.T) {
	g := New(RulesetV6(), 9)
	app := appWithDeps(t, g)
	src := traffic(t, g, app, client())
	for range 300 {
		g.Step()
	}
	must(t, g, Command{Type: CmdDisconnect, From: src, To: app})
	for range 300 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.Last, g.Last) || again.Cash != g.Cash || !reflect.DeepEqual(again.Events, g.Events) {
		t.Fatal("a v6 game must replay exactly")
	}
	if v5 := New(RulesetV5(), 1); len(v5.Nodes) != 1 || v5.Nodes[0].ID != InternetID {
		t.Fatal("v5 keeps the Internet")
	}
}

func TestRulesetV6Balance(t *testing.T) {
	calm := func() *Ruleset {
		r := RulesetV6()
		r.Events = nil
		return r
	}
	sensible, broke := week(t, calm, 1.5)
	if broke > 0 || sensible <= RulesetV6().StartingCash {
		t.Fatalf("a sensibly provisioned design should stay solvent and grow: cash %.0f, %d bankrupt", sensible, broke)
	}
	if none, _ := week(t, calm, 1); none >= sensible {
		t.Fatalf("no headroom should lose money at peaks: %.0f vs %.0f", none, sensible)
	}
	if over, _ := week(t, calm, 5); over >= 0.75*sensible {
		t.Fatalf("5× over-provisioning should cost at least 25%% of the profit: %.0f vs %.0f", over, sensible)
	}
	withEvents, broke := week(t, RulesetV6, 1.5)
	if broke > 0 || withEvents >= sensible {
		t.Fatalf("unanswered events should cost money without bankrupting: %.0f vs %.0f calm", withEvents, sensible)
	}
}
