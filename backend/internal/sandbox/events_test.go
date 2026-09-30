package sandbox

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"
)

func newGameV2(t *testing.T, seed int64) *Game {
	t.Helper()
	g := New(RulesetV2(), seed)
	g.Cash = 1e9
	return g
}

// inject starts an event at the current tick, lasting 100 ticks unless set.
func inject(g *Game, e *Event) *Event {
	e.Start = g.Tick
	if e.End == 0 {
		e.End = g.Tick + 100
	}
	e.LowestHealth = 100
	g.eventSeq++
	e.ID = g.eventSeq
	g.Events = append(g.Events, e)
	g.Last = g.preview()
	return e
}

func respond(g *Game, action, node string) error {
	_, err := g.Apply(Command{Type: CmdRespond, Action: action, Node: node})
	return err
}

// resilient builds Internet → gateway → balancer → 3 apps → cache → primary
// and replica, with a queue, workers, and storage.
func resilient(t *testing.T, g *Game) map[string]string {
	t.Helper()
	ids := map[string]string{}
	for _, k := range []string{KindGateway, KindLB, KindApp, KindCache, KindDBPrimary, KindDBReplica, KindQueue, KindWorker, KindStorage} {
		ids[k] = place(t, g, k)
	}
	must(t, g, Command{Type: CmdScale, Node: ids[KindApp], Replicas: 3})
	connect(t, g, InternetID, ids[KindGateway])
	connect(t, g, ids[KindGateway], ids[KindLB])
	connect(t, g, ids[KindLB], ids[KindApp])
	for _, to := range []string{KindCache, KindDBPrimary, KindDBReplica, KindQueue, KindStorage} {
		connect(t, g, ids[KindApp], ids[to])
	}
	connect(t, g, ids[KindCache], ids[KindDBPrimary])
	connect(t, g, ids[KindCache], ids[KindDBReplica])
	connect(t, g, ids[KindQueue], ids[KindWorker])
	connect(t, g, ids[KindWorker], ids[KindDBPrimary])
	return ids
}

func run(g *Game, ticks int) {
	for i := 0; i < ticks && g.Status == StatusRunning; i++ {
		g.Step()
	}
}

func TestRulesetV1DrawsNoEvents(t *testing.T) {
	g := newGame(t)
	app, _ := basic(t, g)
	run(g, 288*10)
	if len(g.Events) != 0 {
		t.Fatalf("sandbox/v1 has no event deck, got %d events", len(g.Events))
	}
	if err := respond(g, ActRestart, app); !errors.Is(err, ErrInvalid) {
		t.Fatalf("sandbox/v1 has no incident responses: %v", err)
	}
}

func TestDeckIsSeededAndReplayable(t *testing.T) {
	play := func(seed int64) *Game {
		g := New(RulesetV2(), seed)
		resilient(t, g)
		run(g, 288*10)
		return g
	}
	a, b, c := play(7), play(7), play(8)
	if len(a.Events) == 0 {
		t.Fatal("ten days of a complex, popular system should draw events")
	}
	for _, e := range a.Events {
		if e.Start < a.Rules.EventGraceTicks {
			t.Fatalf("no event may start in the first day: %+v", e)
		}
	}
	if !reflect.DeepEqual(a.Events, b.Events) || !reflect.DeepEqual(a.Last, b.Last) {
		t.Fatal("the same seed and commands must deal the same events")
	}
	if reflect.DeepEqual(a.Events, c.Events) {
		t.Fatal("a different seed should deal different events")
	}
	r, err := Replay(a.Save())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Events, a.Events) || !reflect.DeepEqual(r.Last.Meters, a.Last.Meters) {
		t.Fatal("a replay must deal the same events")
	}
}

func TestTrafficEventMultipliesRealTraffic(t *testing.T) {
	g := newGameV2(t, 1)
	basic(t, g)
	before := g.Last.Flow.RPS
	inject(g, &Event{Effect: EffectTraffic, Magnitude: 4})
	if !near(g.Last.Flow.RPS, 4*before, 1e-9) {
		t.Fatalf("rps %v, want 4 × %v", g.Last.Flow.RPS, before)
	}
}

func TestAttackTakesCapacityAndRateLimitingBlocksIt(t *testing.T) {
	g := newGameV2(t, 1)
	app, _ := basic(t, g)
	g.Users = 10_000
	g.Last = g.preview()
	calm := g.Last

	inject(g, &Event{Effect: EffectAttack, Magnitude: 5})
	hit := g.Last
	if !near(hit.Flow.AttackRPS, 5*hit.Flow.RPS, 1e-9) || stats(t, hit, app).Attack <= 0 {
		t.Fatalf("attack traffic should arrive with real traffic: %+v", hit.Flow)
	}
	if hit.Flow.ErrorRate <= calm.Flow.ErrorRate+0.3 || hit.Meters.RevenuePerHour >= calm.Meters.RevenuePerHour {
		t.Fatalf("an absorbed attack should crowd out real users: errors %v → %v", calm.Flow.ErrorRate, hit.Flow.ErrorRate)
	}

	gw := place(t, g, KindGateway)
	must(t, g, Command{Type: CmdDisconnect, From: InternetID, To: app})
	connect(t, g, InternetID, gw)
	connect(t, g, gw, app)
	must(t, g, Command{Type: CmdScale, Node: gw, Replicas: 2})
	if err := respond(g, ActRateLimit, app); !errors.Is(err, ErrInvalid) {
		t.Fatal("only a gateway can rate-limit")
	}
	if err := respond(g, ActRateLimit, gw); err != nil {
		t.Fatal(err)
	}
	limited := g.Last
	a := stats(t, limited, app)
	if !near(a.Attack, limited.Flow.AttackRPS*(1-g.Rules.RateLimitBlock), 1e-6) || stats(t, limited, gw).Blocked <= 0 {
		t.Fatalf("the gateway should pass only unblocked attack traffic: app attack %v of %v", a.Attack, limited.Flow.AttackRPS)
	}
	if limited.Flow.ErrorRate >= hit.Flow.ErrorRate/2 || limited.Flow.ErrorRate < g.Rules.RateLimitFalsePositive-1e-9 {
		t.Fatalf("rate limiting should restore most users, minus false positives: errors %v", limited.Flow.ErrorRate)
	}
	if err := respond(g, ActRateLimit, gw); !errors.Is(err, ErrInvalid) {
		t.Fatal("rate limiting twice is rejected")
	}
	if err := respond(g, ActLiftRateLimit, gw); err != nil || g.Node(gw).RateLimited {
		t.Fatalf("lift rate limit: %v", err)
	}
}

func TestCrashTakesAReplicaAndRestartBringsItBack(t *testing.T) {
	g := newGameV2(t, 1)
	app, _ := basic(t, g)
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 2})
	full := stats(t, g.Last, app).Capacity
	inject(g, &Event{Effect: EffectCrash, Hits: []Hit{{Node: app, Replicas: 1}}})
	if n := g.Node(app); n.DownReplicas != 1 || n.Down || !near(stats(t, g.Last, app).Capacity, full/2, 1e-9) {
		t.Fatalf("one of two replicas down should halve capacity: %+v", n)
	}
	if err := respond(g, ActRestart, app); err != nil {
		t.Fatal(err)
	}
	// Node state reflects the last simulated tick.
	run(g, g.Rules.RestartTicks+1)
	if g.Node(app).DownReplicas != 0 {
		t.Fatalf("a restarted component is back after %d ticks", g.Rules.RestartTicks)
	}
	if err := respond(g, ActRestart, app); !errors.Is(err, ErrInvalid) {
		t.Fatal("restarting a healthy component is rejected")
	}
}

func TestZoneOutageTakesAShareOfEveryComponent(t *testing.T) {
	g := newGameV2(t, 1)
	app, db := basic(t, g)
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 3})
	g.Tick = g.Rules.EventGraceTicks
	g.fx = g.effects()
	e := g.deal(RulesetV2().Events[4], rand.New(rand.NewPCG(1, 1)))
	if e == nil || e.Card != "zone-outage" {
		t.Fatalf("zone outage not dealt: %+v", e)
	}
	g.Events = append(g.Events, e)
	g.Last = g.preview()
	if g.Node(app).DownReplicas != 1 || !g.Node(db).Down {
		t.Fatalf("a zone takes ⌈replicas/3⌉: app %+v db %+v", g.Node(app), g.Node(db))
	}
	if err := respond(g, ActRestart, db); !errors.Is(err, ErrInvalid) {
		t.Fatal("a restart cannot bring back a zone")
	}
}

func TestSlowdownStampedeBacklogCostAndThirdParty(t *testing.T) {
	g := newGameV2(t, 1)
	ids := resilient(t, g)
	g.Users = 20_000
	g.Last = g.preview()
	base := g.Last
	db, cache, worker := ids[KindDBPrimary], ids[KindCache], ids[KindWorker]

	slow := inject(g, &Event{Effect: EffectSlowdown, Target: db, Magnitude: 3})
	if s := stats(t, g.Last, db); !near(s.Capacity, stats(t, base, db).Capacity/3, 1e-9) || s.LatencyMs <= 3*stats(t, base, db).LatencyMs {
		t.Fatalf("a slowdown triples service time and cuts capacity: %+v", s)
	}
	slow.End = g.Tick

	stampede := inject(g, &Event{Effect: EffectHitRatio, Target: cache, Magnitude: 0.2})
	if rep := ids[KindDBReplica]; stats(t, g.Last, rep).Offered <= 3*stats(t, base, rep).Offered {
		t.Fatal("a cache stampede should push misses onto the database")
	}
	stampede.End = g.Tick

	backlog := inject(g, &Event{Effect: EffectCapacity, Target: worker, Magnitude: 0.3})
	if !near(stats(t, g.Last, worker).Capacity, 0.3*stats(t, base, worker).Capacity, 1e-9) {
		t.Fatal("a queue backlog event cuts worker capacity")
	}
	backlog.End = g.Tick

	cost := inject(g, &Event{Effect: EffectCost, Target: KindApp, Magnitude: 2})
	if !near(stats(t, g.Last, ids[KindApp]).CostPerHour, 2*stats(t, base, ids[KindApp]).CostPerHour, 1e-9) {
		t.Fatal("a cost spike doubles the kind's running cost")
	}
	cost.End = g.Tick

	inject(g, &Event{Effect: EffectThirdParty, Magnitude: 0.25})
	if !near(g.Last.Flow.ErrorRate, 1-(1-base.Flow.ErrorRate)*0.75, 1e-9) {
		t.Fatalf("a third-party outage fails its share: %v", g.Last.Flow.ErrorRate)
	}
}

func TestFailoverPromotesAReplica(t *testing.T) {
	g := newGameV2(t, 1)
	ids := resilient(t, g)
	primary, replica, worker := ids[KindDBPrimary], ids[KindDBReplica], ids[KindWorker]
	// Writes through the queue are asynchronous; write synchronously so a
	// lost primary fails requests.
	must(t, g, Command{Type: CmdDisconnect, From: ids[KindApp], To: ids[KindQueue]})
	inject(g, &Event{Effect: EffectCrash, Hits: []Hit{{Node: primary, Replicas: 1}}})
	down := g.Last.Flow.ErrorRate
	if down < 0.1 {
		t.Fatalf("writes should fail with the primary down: %v", down)
	}
	if err := respond(g, ActFailover, replica); !errors.Is(err, ErrInvalid) {
		t.Fatal("failover names the primary")
	}
	if err := respond(g, ActFailover, primary); err != nil {
		t.Fatal(err)
	}
	if g.Node(replica).Kind != KindDBPrimary || g.Node(primary).Kind != KindDBReplica {
		t.Fatal("failover swaps the primary and the replica")
	}
	if !g.hasEdge(worker, replica) || g.hasEdge(worker, primary) {
		t.Fatalf("workers write to the new primary: %+v", g.Edges)
	}
	if g.Last.Flow.ErrorRate > 1e-6 {
		t.Fatalf("after failover the system serves everything: %v", g.Last.Flow.ErrorRate)
	}
	if err := respond(g, ActFailover, replica); !errors.Is(err, ErrInvalid) {
		t.Fatal("failover needs a healthy replica to promote")
	}
}

func TestEventOutcomeIsJudgedAnHourAfterItEnds(t *testing.T) {
	g := newGameV2(t, 1)
	app, _ := basic(t, g)
	e := inject(g, &Event{Effect: EffectCrash, End: g.Tick + 6, Hits: []Hit{{Node: app, Replicas: 1}}})
	if e.Phase != PhaseActive {
		t.Fatalf("phase %q", e.Phase)
	}
	run(g, 6)
	if e.Phase != PhaseRecovering || e.LowestHealth > 50 {
		t.Fatalf("after the crash: %+v", e)
	}
	run(g, g.Rules.RecoveryTicks)
	if e.Phase != PhaseOver || e.Outcome != OutcomeRecovered {
		t.Fatalf("a recovered system is judged recovered: %+v", e)
	}
}

func TestADownQueueHoldsItsBacklog(t *testing.T) {
	g := newGameV2(t, 1)
	app, db := basic(t, g)
	q, w := place(t, g, KindQueue), place(t, g, KindWorker)
	connect(t, g, app, q)
	g.Step()
	connect(t, g, q, w)
	connect(t, g, w, db)
	held := g.Node(q).Backlog
	inject(g, &Event{Effect: EffectCrash, Hits: []Hit{{Node: q, Replicas: 1}}})
	g.Step()
	if g.Node(q).Backlog != held || stats(t, g.Last, w).Offered != 0 {
		t.Fatalf("a down queue neither drains nor delivers: backlog %v → %v, worker offered %v", held, g.Node(q).Backlog, stats(t, g.Last, w).Offered)
	}
}
