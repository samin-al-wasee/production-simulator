package sandbox

import (
	"reflect"
	"testing"
)

func newGameV10(t *testing.T) *Game {
	t.Helper()
	r := RulesetV10()
	r.Events = nil
	g := New(r, 1)
	g.FreeBuild = true
	g.Cash = 1e9
	return g
}

// media builds traffic → app → storage, where every request fetches media.
func media(t *testing.T, g *Game, rps float64) (string, string) {
	t.Helper()
	app := appWithDeps(t, g)
	st := g.Edges[1].To
	c := constant(client(), rps)
	c.Endpoints = []Weight{{"GET /media/:id", 1}}
	traffic(t, g, app, c)
	return app, st
}

func storageStats(t *testing.T, g *Game, id string) StorageStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.Storage == nil {
		t.Fatalf("%s has no storage stats", id)
	}
	return *s.Storage
}

func TestStorageLatencyAndUsagePricing(t *testing.T) {
	g := newGameV10(t)
	_, st := media(t, g, 10)
	warm(g)
	s := storageStats(t, g, st)
	// 200 KB at 80 Mbps: 1.6 Mb ÷ 80 Mbps = 20 ms after a 20 ms first byte.
	if !near(s.TransferMs, 20, 1e-9) || !near(s.FirstByteMs, 20, 1e-9) || !near(s.Gets, 10, 1e-9) {
		t.Fatalf("latency is the first byte plus the transfer: %+v", s)
	}
	gbPerHour := 10 * 200.0 / 1024 / 1024 * 3600
	if !near(s.EgressCost, gbPerHour*0.09, 1e-9) || !near(s.RequestCost, 10*3.6*0.0004, 1e-9) {
		t.Fatalf("egress and requests are priced by use: %+v", s)
	}
	if cost := stats(t, g.Last, st).CostPerHour; !near(cost, s.StorageCost+s.RequestCost+s.RetrievalCost+s.EgressCost, 1e-12) {
		t.Fatalf("storage costs what it was used for: %v", cost)
	}
	if _, err := g.Apply(Command{Type: CmdScale, Node: st, Replicas: 2}); err == nil {
		t.Fatal("object storage scales by prefixes, not replicas")
	}
}

func TestAPrefixThrottlesAndMorePrefixesScale(t *testing.T) {
	g := newGameV10(t)
	app, st := media(t, g, 10)
	// Only storage behind the route, so nothing else limits the app.
	a := appCfg()
	a.Processing, a.MaxConcurrency, a.Workers = ProcessingAsync, 10_000, 4
	a.Routes = []AppRoute{{Endpoint: "GET /media/:id", BaseMs: 5, CPUMs: 2, MemoryMB: 1, RequestKB: 1, ResponseKB: 1, Deps: []string{DepStorage}}}
	setApp(t, g, app, a)
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 50})
	must(t, g, Command{Type: CmdResize, Node: app, Size: "large"})
	c := *g.Node(g.Edges[2].From).Client
	c.Pattern.RPS = 8000
	must(t, g, Command{Type: CmdConfigure, Node: g.Edges[2].From, Client: &c})
	// A pool large enough that the app's connections to storage never hold
	// it back (a small pool would: an overloaded store's latency fills it).
	conn := *g.edge(app, st).Conn
	conn.Pool = 1000
	must(t, g, Command{Type: CmdConfigure, From: app, To: st, Connection: &conn})
	warm(g)
	if s := storageStats(t, g, st); !(s.Throttled > 1000) || s.Capacity != 5500 {
		t.Fatalf("one prefix sustains 5,500 GETs/s and throttles the rest: %+v", s)
	}
	cfg := *g.Rules.Storage
	cfg.Prefixes = 2
	must(t, g, Command{Type: CmdConfigure, Node: st, Storage: &cfg})
	warm(g)
	if s := storageStats(t, g, st); s.Throttled > 1e-9 || s.Capacity != 11000 {
		t.Fatalf("two prefixes double the rate: %+v", s)
	}
}

func TestArchiveIsCheapToKeepAndSlowToRead(t *testing.T) {
	g := newGameV10(t)
	_, st := media(t, g, 10)
	warm(g)
	std := storageStats(t, g, st)
	cfg := *g.Rules.Storage
	cfg.Class = "archive"
	must(t, g, Command{Type: CmdConfigure, Node: st, Storage: &cfg})
	warm(g)
	arc := storageStats(t, g, st)
	if !(arc.StorageCost < std.StorageCost) || !(arc.LatencyMs > 1000) || !(arc.RetrievalCost > 0) {
		t.Fatalf("archive costs less to keep, more to read, and is slow: %+v vs %+v", arc, std)
	}
}

func TestStorageValidatesAndReplays(t *testing.T) {
	r := RulesetV10()
	if p := r.ValidateStorage(StorageConfig{Class: "tape"}); len(p) != 3 {
		t.Fatalf("every problem is listed: %v", p)
	}
	g := New(RulesetV10(), 2)
	app := appWithDeps(t, g)
	traffic(t, g, app, client())
	for range 300 {
		g.Step()
	}
	must(t, g, Command{Type: CmdConfigure, Node: g.Edges[1].To, Storage: &StorageConfig{Class: "infrequent", Prefixes: 2, ObjectKB: 100}})
	for range 100 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Last, g.Last) || again.Cash != g.Cash {
		t.Fatalf("a v10 game must replay exactly: %v", err)
	}
}

func TestRulesetV10Balance(t *testing.T) {
	calm := func() *Ruleset {
		r := RulesetV10()
		r.Events = nil
		return r
	}
	sensible, broke := week(t, calm, 1.5)
	if broke > 0 || sensible <= RulesetV10().StartingCash {
		t.Fatalf("a sensibly provisioned design should stay solvent and grow: cash %.0f, %d bankrupt", sensible, broke)
	}
	if over, _ := week(t, calm, 5); over >= 0.8*sensible {
		t.Fatalf("5× over-provisioning should cost at least 20%% of the profit: %.0f vs %.0f", over, sensible)
	}
}
