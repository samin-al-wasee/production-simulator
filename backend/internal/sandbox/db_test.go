package sandbox

import (
	"reflect"
	"testing"
)

func newGameV8(t *testing.T) *Game {
	t.Helper()
	r := RulesetV8()
	r.Events = nil
	for i := range r.Kinds {
		r.Kinds[i].UnlockedBy = ""
	}
	g := New(r, 1)
	g.Cash = 1e9
	return g
}

func dbStats(t *testing.T, g *Game, id string) DBStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.DB == nil {
		t.Fatalf("%s has no database stats", id)
	}
	return *s.DB
}

func TestDatabaseCapacityComesFromItsQueries(t *testing.T) {
	g := newGameV8(t)
	db := place(t, g, KindDBPrimary)
	c := g.dbCosts(g.Node(db))
	// 80% reads at 2.5 + 4 pages × 0.005 CPU-ms, 20% writes at 5 + 3 × 0.005.
	cpu := 0.8*(2.5+4*0.005) + 0.2*(5+3*0.005)
	if !near(c.cpuR, 2.52, 1e-9) || !near(c.cpuW, 5.015, 1e-9) || c.hit != 1 {
		t.Fatalf("query costs: %+v", c)
	}
	if got := g.dbCapacity(0); !near(got, 1000/cpu, 1e-6) {
		t.Fatalf("a small database serves 1000 CPU-ms/s ÷ %.3f ms per query: %v", cpu, got)
	}
}

func TestDataGrowthOutgrowsTheBufferPool(t *testing.T) {
	g := newGameV8(t)
	db := place(t, g, KindDBPrimary)
	if c := g.dbCosts(g.Node(db)); c.hit != 1 || c.ioR != 0 {
		t.Fatalf("2,000 users fit in memory: %+v", c)
	}
	g.Users = 200_000 // 200 MB + 200k × 50 KB ≈ 9.97 GB, working set ≈ 2 GB
	c := g.dbCosts(g.Node(db))
	if want := 768 / (g.dataMB() * 0.2); !near(c.hit, want, 1e-9) || !(c.ioR > 0) {
		t.Fatalf("the buffer pool covers %v of the working set: %+v", want, c)
	}
	must(t, g, Command{Type: CmdResize, Node: db, Size: "large"})
	if big := g.dbCosts(g.Node(db)); big.hit != 1 || big.svcR >= c.svcR {
		t.Fatalf("a larger size holds the working set again: %+v", big)
	}
}

func TestAnUnindexedQueryScansAndGetsWorseWithData(t *testing.T) {
	g := newGameV8(t)
	db := place(t, g, KindDBPrimary)
	cfg := *g.Rules.DB
	cfg.Read.Indexed = false
	must(t, g, Command{Type: CmdConfigure, Node: db, DB: &cfg})
	small := g.dbCosts(g.Node(db)).cpuR
	g.Users *= 10
	if big := g.dbCosts(g.Node(db)).cpuR; !(big > small*2) || !near(small, 2.5+(4+g.Rules.DBRuntime.BaseDataMB+2000*50.0/1024)*0.005, 1e-9) {
		t.Fatalf("a scan reads pages in proportion to the data: %v then %v", small, big)
	}
}

func TestHotRowsLockWrites(t *testing.T) {
	g := newGameV8(t)
	app := appWithDeps(t, g)
	db := g.Edges[0].To
	cfg := *g.Rules.DB
	cfg.HotRows, cfg.LockMs = 1, 100 // 10 writes/s at most
	must(t, g, Command{Type: CmdConfigure, Node: db, DB: &cfg})
	c := constant(client(), 20)
	c.Endpoints = []Weight{{"POST /orders", 1}}
	traffic(t, g, app, c)
	warm(g)
	// Lock waits stretch every write, so the app's sync workers fill up
	// before 10 writes/s ever arrive: the slow database stalls the app.
	if st := dbStats(t, g, db); st.Bottleneck != "locks" || !near(st.Capacity, 10, 1e-9) || !(st.LockWaitMs > st.WriteMs/2) {
		t.Fatalf("one hot row held 100 ms caps writes at 10/s: %+v", st)
	}
	if b := appStats(t, g, app).Bottleneck; b != "slots" {
		t.Fatalf("the app runs out of workers waiting on locked rows: %s", b)
	}
}

func TestTooManyConnectionsAreRefused(t *testing.T) {
	g := newGameV8(t)
	app := appWithDeps(t, g)
	db := g.Edges[0].To
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 6})
	traffic(t, g, app, constant(client(), 20))
	warm(g)
	st := dbStats(t, g, db)
	if st.Connections != 120 || !near(st.Refused, (st.Reads+st.Writes)/6, 1e-9) {
		t.Fatalf("6 replicas × a pool of 20 open 120 connections, 20 over the limit: %+v", st)
	}
}

func TestReplicasApplyEveryWriteAndCanLag(t *testing.T) {
	g := newGameV8(t)
	app := appWithDeps(t, g)
	primary := g.Edges[0].To
	replica := place(t, g, KindDBReplica)
	connect(t, g, app, replica)
	c := constant(client(), 40)
	c.Endpoints = []Weight{{"POST /orders", 0.5}, {"GET /profile", 0.5}}
	traffic(t, g, app, c)
	for range 3 {
		g.Step()
	}
	st := dbStats(t, g, replica)
	if !near(st.Applying, g.lastWrites[primary], 1e-9) || st.Applying == 0 || st.LagSeconds != 0 {
		t.Fatalf("a replica applies its primary's writes and keeps up: %+v", st)
	}
	// Slow the replica down until it cannot keep up: it falls behind.
	g.Events = []*Event{{Effect: EffectSlowdown, Start: 0, End: 1000, Magnitude: 60, Target: replica}}
	for range 3 {
		g.Step()
	}
	if st := dbStats(t, g, replica); !(st.LagSeconds > 10) || st.Health != HealthDegraded && st.Health != HealthUnhealthy {
		t.Fatalf("a replica that cannot apply its writes lags: %+v", st)
	}
}

func TestDatabaseConfigValidatesAndReplays(t *testing.T) {
	r := RulesetV8()
	if p := r.ValidateDB(*r.DB); len(p) != 0 {
		t.Fatalf("the default is valid: %v", p)
	}
	bad := *r.DB
	bad.MaxConnections, bad.Read.CPUMs, bad.HotRows = 0, 0, 0
	if p := r.ValidateDB(bad); len(p) != 3 {
		t.Fatalf("every problem is listed: %v", p)
	}
	g := New(RulesetV8(), 2)
	app := appWithDeps(t, g)
	traffic(t, g, app, client())
	for range 400 {
		g.Step()
	}
	cfg := *g.Rules.DB
	cfg.MaxConnections = 10
	must(t, g, Command{Type: CmdConfigure, Node: g.Edges[0].To, DB: &cfg})
	for range 200 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Last, g.Last) || again.Cash != g.Cash {
		t.Fatalf("a v8 game must replay exactly: %v", err)
	}
}

func TestRulesetV8Balance(t *testing.T) {
	calm := func() *Ruleset {
		r := RulesetV8()
		r.Events = nil
		return r
	}
	sensible, broke := week(t, calm, 1.5)
	if broke > 0 || sensible <= RulesetV8().StartingCash {
		t.Fatalf("a sensibly provisioned design should stay solvent and grow: cash %.0f, %d bankrupt", sensible, broke)
	}
	// The modelled database is a little cheaper per query than v7's flat
	// 300 ops/s, so over-provisioning wastes a little less (ADR-0020).
	if over, _ := week(t, calm, 5); over >= 0.8*sensible {
		t.Fatalf("5× over-provisioning should cost at least 20%% of the profit: %.0f vs %.0f", over, sensible)
	}
}
