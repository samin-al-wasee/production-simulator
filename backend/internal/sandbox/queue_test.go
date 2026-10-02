package sandbox

import (
	"math"
	"reflect"
	"testing"
)

func newGameV11(t *testing.T) *Game {
	t.Helper()
	r := RulesetV11()
	r.Events = nil
	g := New(r, 1)
	g.FreeBuild = true
	g.Cash = 1e9
	return g
}

// pipeline builds traffic → app (writes go to a queue) → queue → worker →
// database, with rps orders per second.
func pipeline(t *testing.T, g *Game, rps float64) (app, q, w, db string) {
	t.Helper()
	app, q, w, db = place(t, g, KindApp), place(t, g, KindQueue), place(t, g, KindWorker), place(t, g, KindDBPrimary)
	connect(t, g, app, q)
	connect(t, g, q, w)
	connect(t, g, w, db)
	c := constant(client(), rps)
	c.Endpoints = []Weight{{"POST /orders", 1}}
	traffic(t, g, app, c)
	return
}

func queueStats(t *testing.T, g *Game, id string) QueueStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.Queue == nil {
		t.Fatalf("%s has no queue stats", id)
	}
	return *s.Queue
}

func TestAWorkerRunsItsHandlerLikeAnApp(t *testing.T) {
	g := newGameV11(t)
	_, q, w, db := pipeline(t, g, 20)
	warm(g)
	st := appStats(t, g, w)
	if len(st.Routes) != 1 || st.Routes[0].Endpoint != HandlerEndpoint || !near(st.Routes[0].RPS, 20, 1e-9) {
		t.Fatalf("the worker handles every message: %+v", st.Routes)
	}
	if st.Health != HealthHealthy || st.Success < 19.9 {
		t.Fatalf("a healthy worker writes every message to the database: %+v", st)
	}
	if qs := queueStats(t, g, q); qs.Backlog > 1e-6 || qs.Redelivered > 1e-9 || !near(qs.Delivered, 20, 1e-9) {
		t.Fatalf("a keeping-up worker leaves no backlog: %+v", qs)
	}
	if s := stats(t, g.Last, db); !near(s.Offered, 20, 1e-6) {
		t.Fatalf("the worker's writes reach the database: %v", s.Offered)
	}
	// Its capacity is emergent: 1 vCPU ÷ 20 CPU-ms, or 10 slots ÷ its wall time.
	if c := st.Capacity; !(c > 20) || c > 1000/20.0+1e-9 {
		t.Fatalf("worker capacity comes from its handler: %v", c)
	}
}

func TestFailedMessagesAreRedeliveredThenDeadLettered(t *testing.T) {
	g := newGameV11(t)
	_, q, w, _ := pipeline(t, g, 10)
	wc := *g.Rules.Worker
	wc.Handler.ErrorRate = 0.5
	must(t, g, Command{Type: CmdConfigure, Node: w, Worker: &wc})
	for range 4 {
		g.Step()
	}
	qs := queueStats(t, g, q)
	f := qs.WorkerFailure
	if !near(f, 0.5, 0.01) {
		t.Fatalf("the queue learns its workers fail half the messages: %v", f)
	}
	// 5 deliveries: 1 + f + f² + f³ + f⁴ per message, and f⁵ dead-lettered.
	if !near(qs.Delivered, 10*redelivery(f, 5), 1e-6) || !near(qs.DeadLettered, 10*math.Pow(f, 5), 1e-9) {
		t.Fatalf("failed messages come back until they run out of deliveries: %+v", qs)
	}
	if !(qs.DeadLetters > 0) || qs.Health != HealthDegraded {
		t.Fatalf("dead letters pile up and the queue is degraded: %+v", qs)
	}
	// Publishers never see the workers' failures: the queue took the message.
	if g.Last.Flow.ErrorRate > 1e-9 {
		t.Fatalf("a queue decouples publishers from workers: %v", g.Last.Flow.ErrorRate)
	}
}

func TestASlowWorkerBuildsABacklogAndDelay(t *testing.T) {
	g := newGameV11(t)
	_, q, w, _ := pipeline(t, g, 25)
	wc := *g.Rules.Worker
	wc.Concurrency = 1 // 1 slot ÷ ~70 ms ≈ 14 messages/s
	must(t, g, Command{Type: CmdConfigure, Node: w, Worker: &wc})
	for range 3 {
		g.Step()
	}
	qs := queueStats(t, g, q)
	if !(qs.Backlog > 5_000) || !(qs.DelaySeconds > 100) || appStats(t, g, w).Bottleneck != "slots" {
		t.Fatalf("a worker that cannot keep up grows the backlog and the delay: %+v", qs)
	}
	// Only the app's own timeout tail fails, not the queue.
	if g.Last.Flow.ErrorRate > 1e-3 {
		t.Fatalf("until the backlog is full, publishers still succeed: %v", g.Last.Flow.ErrorRate)
	}
}

func TestProcessingPastTheVisibilityTimeoutRedelivers(t *testing.T) {
	g := newGameV11(t)
	_, q, w, _ := pipeline(t, g, 5)
	wc := *g.Rules.Worker
	wc.Handler.BaseMs = 2000 // longer than a 1 s visibility timeout
	must(t, g, Command{Type: CmdConfigure, Node: w, Worker: &wc})
	qc := *g.Rules.Queue
	qc.VisibilitySeconds = 1
	must(t, g, Command{Type: CmdConfigure, Node: q, Queue: &qc})
	for range 3 {
		g.Step()
	}
	if qs := queueStats(t, g, q); !(qs.WorkerFailure > 0.99) || !(qs.DeadLettered > 0) {
		t.Fatalf("a message held past its visibility timeout is delivered again: %+v", qs)
	}
}

func TestQueueAndWorkerValidateAndReplay(t *testing.T) {
	r := RulesetV11()
	if p := r.ValidateQueue(QueueConfig{}); len(p) != 3 {
		t.Fatalf("every queue problem is listed: %v", p)
	}
	if p := r.ValidateWorker(WorkerConfig{Handler: AppRoute{CPUMs: 0}}); len(p) < 2 {
		t.Fatalf("worker problems are listed: %v", p)
	}
	g := New(RulesetV11(), 3)
	g.FreeBuild = true
	pipeline(t, g, 5)
	for range 300 {
		g.Step()
	}
	wc := *g.Rules.Worker
	wc.Concurrency = 2
	must(t, g, Command{Type: CmdConfigure, Node: "worker-1", Worker: &wc})
	for range 100 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Last, g.Last) || !reflect.DeepEqual(again.Nodes, g.Nodes) {
		t.Fatalf("a v11 game must replay exactly: %v", err)
	}
}

func TestTheQueueBacklogEventSlowsWorkers(t *testing.T) {
	g := newGameV11(t)
	_, _, w, _ := pipeline(t, g, 10)
	warm(g)
	full := appStats(t, g, w).Capacity
	g.Events = []*Event{{Effect: EffectCapacity, Start: 0, End: 1000, Magnitude: 0.3, Target: w}}
	g.Last = g.preview()
	if got := appStats(t, g, w).Capacity; !near(got, full*0.3, 1e-9) {
		t.Fatalf("a queue-backlog event cuts the worker's capacity: %v vs %v", got, full)
	}
}
