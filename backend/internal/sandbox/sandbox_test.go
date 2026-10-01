package sandbox

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func newGame(t *testing.T) *Game {
	t.Helper()
	g := New(RulesetV1(), 1)
	g.Cash = 1e9
	return g
}

func must(t *testing.T, g *Game, c Command) string {
	t.Helper()
	id, err := g.Apply(c)
	if err != nil {
		t.Fatalf("%+v: %v", c, err)
	}
	return id
}

func place(t *testing.T, g *Game, kind string) string {
	t.Helper()
	return must(t, g, Command{Type: CmdPlace, Kind: kind})
}

func connect(t *testing.T, g *Game, from, to string) {
	t.Helper()
	must(t, g, Command{Type: CmdConnect, From: from, To: to})
}

// basic builds Internet → app → database, with object storage.
func basic(t *testing.T, g *Game) (app, db string) {
	t.Helper()
	app, db = place(t, g, KindApp), place(t, g, KindDBPrimary)
	st := place(t, g, KindStorage)
	if g.clientModel() {
		// One traffic component per segment captures the whole market.
		for _, ct := range g.Rules.ClientTypes {
			for _, rg := range g.Rules.RegionShares {
				id := place(t, g, KindTraffic)
				connect(t, g, id, app)
				c := *g.Node(id).Client
				c.ClientType, c.Region = ct.Name, rg.Name
				must(t, g, Command{Type: CmdConfigure, Node: id, Client: &c})
			}
		}
	} else {
		connect(t, g, InternetID, app)
	}
	connect(t, g, app, db)
	connect(t, g, app, st)
	return app, db
}

func stats(t *testing.T, s Snapshot, id string) NodeStats {
	t.Helper()
	for _, n := range s.Flow.Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("no stats for %s", id)
	return NodeStats{}
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestEmptyWorldServesNothing(t *testing.T) {
	g := New(RulesetV1(), 1)
	if len(g.Nodes) != 1 || g.Nodes[0].ID != InternetID || g.Cash != 1000 {
		t.Fatalf("new game must hold only the Internet and starting cash: %+v", g)
	}
	s := g.Step()
	if s.Flow.RPS <= 0 || s.Flow.SuccessRPS != 0 || s.Flow.ErrorRate != 1 {
		t.Fatalf("users arrive but nothing serves them: %+v", s.Flow)
	}
	if s.Meters.RevenuePerHour != 0 || g.Satisfaction >= 50 {
		t.Fatalf("no revenue and falling satisfaction expected: %+v", s.Meters)
	}
}

func TestMinimalSystemServesTraffic(t *testing.T) {
	g := newGame(t)
	basic(t, g)
	s := g.Step()
	if s.Flow.ErrorRate > 1e-9 || s.Flow.P95LatencyMs <= 0 || s.Flow.P95LatencyMs > g.Rules.SLOp95Ms {
		t.Fatalf("a small healthy system should serve everything within the SLO: %+v", s.Flow)
	}
	if s.Meters.RevenuePerHour <= 0 || s.Meters.Health < 99 {
		t.Fatalf("unexpected meters: %+v", s.Meters)
	}
}

func TestMissingStorageFailsItsShare(t *testing.T) {
	g := newGame(t)
	app, db := place(t, g, KindApp), place(t, g, KindDBPrimary)
	connect(t, g, InternetID, app)
	connect(t, g, app, db)
	if s := g.Step(); !near(s.Flow.ErrorRate, g.Rules.StorageShare, 1e-9) {
		t.Fatalf("error rate %v, want the storage share %v", s.Flow.ErrorRate, g.Rules.StorageShare)
	}
}

func TestBottleneckMovesAsThePlayerScales(t *testing.T) {
	g := newGame(t)
	app, db := basic(t, g)
	g.Users = 400_000 // ~1,000+ RPS: far beyond one small app instance

	s := g.preview()
	if u := stats(t, s, app).Utilization; u <= 1 || s.Flow.ErrorRate < 0.5 {
		t.Fatalf("one app instance should be saturated: util=%v errors=%v", u, s.Flow.ErrorRate)
	}

	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 40})
	s = g.Last
	if u := stats(t, s, app).Utilization; u >= 1 {
		t.Fatalf("40 replicas should absorb the load: util=%v", u)
	}
	if u := stats(t, s, db).Utilization; u <= 1 {
		t.Fatalf("the database should now be the bottleneck: util=%v", u)
	}

	cache := place(t, g, KindCache)
	must(t, g, Command{Type: CmdDisconnect, From: app, To: db})
	connect(t, g, app, cache)
	connect(t, g, app, db)
	connect(t, g, cache, db)
	s = g.Last
	offered := stats(t, s, db).Offered
	served := stats(t, s, app).Served
	want := served*g.Rules.ReadShare*(1-0.8) + served*(1-g.Rules.ReadShare)
	if !near(offered, want, 1e-6) {
		t.Fatalf("cache should absorb 80%% of reads: db offered %v, want %v", offered, want)
	}
}

func TestBalancerSplitsByCapacityAndSkipsFailedNodes(t *testing.T) {
	g := newGame(t)
	lb := place(t, g, KindLB)
	small := place(t, g, KindApp)
	large := must(t, g, Command{Type: CmdPlace, Kind: KindApp, Size: "large"})
	connect(t, g, InternetID, lb)
	connect(t, g, lb, small)
	connect(t, g, lb, large)
	s := g.preview()
	a, b := stats(t, s, small).Offered, stats(t, s, large).Offered
	if !near(b/a, 6, 1e-9) {
		t.Fatalf("large (6x) should get 6x the load: %v vs %v", b, a)
	}
	inject(g, &Event{Effect: EffectCrash, Hits: []Hit{{Node: small, Replicas: 1}}})
	s = g.preview()
	if !g.Node(small).Down {
		t.Fatal("a component whose only replica crashed is down")
	}
	if stats(t, s, small).Offered != 0 || !near(stats(t, s, large).Offered, s.Flow.RPS, 1e-9) {
		t.Fatal("a failed instance must receive no traffic while a healthy one exists")
	}
}

func TestQueueBuildsBacklogUntilWorkersDrainIt(t *testing.T) {
	g := newGame(t)
	app, db := basic(t, g)
	q := place(t, g, KindQueue)
	connect(t, g, app, q)
	g.Step()
	if g.Node(q).Backlog <= 0 {
		t.Fatal("writes without workers should pile up in the queue")
	}
	w := place(t, g, KindWorker)
	connect(t, g, q, w)
	connect(t, g, w, db)
	for i := 0; i < 3; i++ {
		g.Step()
	}
	if g.Node(q).Backlog != 0 {
		t.Fatalf("a worker should drain the backlog, got %v", g.Node(q).Backlog)
	}
	if stats(t, g.Last, db).Offered <= 0 || g.Last.Flow.ErrorRate > 1e-9 {
		t.Fatalf("workers should write to the database: %+v", g.Last.Flow)
	}
}

func TestCommandValidation(t *testing.T) {
	g := New(RulesetV1(), 1)
	app := place(t, g, KindApp)
	gw1, gw2 := place(t, g, KindGateway), place(t, g, KindLB)
	cases := []Command{
		{Type: CmdPlace, Kind: KindInternet},
		{Type: CmdPlace, Kind: "mainframe"},
		{Type: CmdPlace, Kind: KindApp, Size: "huge"},
		{Type: CmdConnect, From: app, To: InternetID},
		{Type: CmdConnect, From: InternetID, To: "nope"},
		{Type: CmdRemove, Node: InternetID},
		{Type: CmdScale, Node: app, Replicas: 0},
		{Type: CmdPlace, Kind: KindDBPrimary, Size: "large"}, // costs more than the cash left
		{Type: "teleport"},
	}
	for _, c := range cases {
		if _, err := g.Apply(c); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: want ErrInvalid, got %v", c, err)
		}
	}
	connect(t, g, gw1, gw2)
	if _, err := g.Apply(Command{Type: CmdConnect, From: gw2, To: gw1}); !errors.Is(err, ErrInvalid) {
		t.Fatal("a loop must be rejected")
	}
	connect(t, g, InternetID, app)
	must(t, g, Command{Type: CmdRemove, Node: app})
	if len(g.Edges) != 1 {
		t.Fatalf("removing a node removes its edges: %+v", g.Edges)
	}
}

func TestSpendingAndRunningCosts(t *testing.T) {
	g := New(RulesetV1(), 1)
	place(t, g, KindDBPrimary)
	if g.Cash != 800 {
		t.Fatalf("placing a database costs 200, cash %v", g.Cash)
	}
	s := g.Step()
	want := 800 - s.Meters.CostPerHour*g.Rules.TickSeconds/3600
	if !near(g.Cash, want, 1e-9) || s.Meters.CostPerHour <= 4 {
		t.Fatalf("cash %v, want %v (cost/h %v includes operations overhead)", g.Cash, want, s.Meters.CostPerHour)
	}
}

func TestBankruptcyEndsTheGame(t *testing.T) {
	g := New(RulesetV1(), 1)
	db := place(t, g, KindDBPrimary)
	must(t, g, Command{Type: CmdScale, Node: db, Replicas: 4})
	for i := 0; i < 5000 && g.Status == StatusRunning; i++ {
		g.Step()
	}
	if g.Status != StatusBankrupt {
		t.Fatalf("an idle, expensive system must go bankrupt; cash %v", g.Cash)
	}
	if _, err := g.Apply(Command{Type: CmdRemove, Node: db}); err == nil {
		t.Fatal("a finished game accepts no commands")
	}
}

func TestHealthySystemGrowsUsers(t *testing.T) {
	g := newGame(t)
	app, _ := basic(t, g)
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 5})
	start := g.Users
	for i := 0; i < 288; i++ {
		g.Step()
	}
	if g.Users <= start || g.Satisfaction < 90 || g.Popularity <= g.Rules.StartingPopularity {
		t.Fatalf("users %v→%v satisfaction %v popularity %v", start, g.Users, g.Satisfaction, g.Popularity)
	}
	if len(g.History) != 288 || g.Last.Meters.Day != 2 {
		t.Fatalf("history %d day %d", len(g.History), g.Last.Meters.Day)
	}
}

func TestReplayIsDeterministic(t *testing.T) {
	g := newGame(t)
	g.Cash = g.Rules.StartingCash
	app, db := basic(t, g)
	for i := 0; i < 50; i++ {
		g.Step()
	}
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 2})
	q := place(t, g, KindQueue)
	connect(t, g, app, q)
	for i := 0; i < 30; i++ {
		g.Step()
	}
	w := place(t, g, KindWorker)
	connect(t, g, q, w)
	connect(t, g, w, db)
	for i := 0; i < 20; i++ {
		g.Step()
	}

	r, err := Replay(g.Save())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Last.Meters, g.Last.Meters) || !reflect.DeepEqual(r.Last.Flow, g.Last.Flow) ||
		!reflect.DeepEqual(r.Nodes, g.Nodes) || !reflect.DeepEqual(r.Edges, g.Edges) {
		t.Fatalf("replay diverged:\n%+v\n%+v", r.Last.Meters, g.Last.Meters)
	}
}
