package sandbox

import (
	"reflect"
	"testing"
)

func newGameV12(t *testing.T) *Game {
	t.Helper()
	r := RulesetV12()
	r.Events = nil
	g := New(r, 1)
	g.FreeBuild = true
	g.Cash = 1e9
	return g
}

// eventApp places an application whose POST /orders writes to a database
// and publishes to a stream, with rps orders per second.
func eventApp(t *testing.T, g *Game, rps float64) (app, stream string) {
	t.Helper()
	a := appCfg()
	a.Name = "Orders API"
	a.Processing, a.MaxConcurrency = ProcessingAsync, 1000
	a.Routes = []AppRoute{{Endpoint: "POST /orders", BaseMs: 10, CPUMs: 2, MemoryMB: 1, RequestKB: 1, ResponseKB: 1, Deps: []string{DepStream}}}
	app = must(t, g, Command{Type: CmdPlace, Kind: KindApp, App: &a})
	stream = place(t, g, KindStream)
	connect(t, g, app, stream)
	c := constant(client(), rps)
	c.Endpoints = []Weight{{"POST /orders", 1}}
	traffic(t, g, app, c)
	return app, stream
}

func streamStats(t *testing.T, g *Game, id string) StreamStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.Stream == nil {
		t.Fatalf("%s has no stream stats", id)
	}
	return *s.Stream
}

func groupOf(t *testing.T, st StreamStats, consumer string) GroupStats {
	t.Helper()
	for _, gs := range st.Groups {
		if gs.Consumer == consumer {
			return gs
		}
	}
	t.Fatalf("no group %s in %+v", consumer, st.Groups)
	return GroupStats{}
}

func TestEveryConsumerGroupReadsEveryEvent(t *testing.T) {
	g := newGameV12(t)
	_, stream := eventApp(t, g, 20)
	w := place(t, g, KindWorker)
	connect(t, g, stream, w)
	connect(t, g, w, place(t, g, KindDBPrimary))
	tp := g.Rules.AppTypes[len(g.Rules.AppTypes)-1] // Order events, with POST /events
	a := appCfg()
	a.Name, a.Routes = "Analytics API", tp.Routes
	analytics := must(t, g, Command{Type: CmdPlace, Kind: KindApp, App: &a})
	connect(t, g, stream, analytics)
	connect(t, g, analytics, place(t, g, KindDBPrimary))
	warm(g)
	st := streamStats(t, g, stream)
	if !near(st.Produced, 20, 1e-9) || len(st.Groups) != 2 {
		t.Fatalf("the stream takes every event: %+v", st)
	}
	if gw, ga := groupOf(t, st, w), groupOf(t, st, analytics); !near(gw.Consumed, 20, 1e-9) || !near(ga.Consumed, 20, 1e-9) {
		t.Fatalf("both groups read all 20 events/s (fan-out, unlike a queue): %+v %+v", gw, ga)
	}
	var events RouteStats
	for _, r := range appStats(t, g, analytics).Routes {
		if r.Endpoint == EventsEndpoint {
			events = r
		}
	}
	if !near(events.RPS, 20, 1e-9) {
		t.Fatalf("an application consumes events as POST /events: %+v", events)
	}
}

func TestPartitionsCapThroughputAndConsumers(t *testing.T) {
	g := newGameV12(t)
	_, stream := eventApp(t, g, 10)
	n := g.Node(stream)
	// 6 partitions × 10 MB/s ÷ 1 KB events; a small broker's 100 Mbps
	// network is 12.5 MB/s, so the broker is the limit at first.
	if c, b, _ := g.streamCapacity(n); b != "brokers" || !near(c, 100.0/8*1024, 1e-6) {
		t.Fatalf("a small broker's network limits it: %v %s", c, b)
	}
	must(t, g, Command{Type: CmdResize, Node: stream, Size: "large"})
	cfg := *g.Rules.Stream
	cfg.KeySkew = 0.5
	must(t, g, Command{Type: CmdConfigure, Node: stream, Stream: &cfg})
	if c, b, hot := g.streamCapacity(n); b != "partitions" || hot != 0.5 || !near(c, 10*1024/0.5, 1e-6) {
		t.Fatalf("a hot key funnels half the events into one partition: %v %s %v", c, b, hot)
	}
	w := place(t, g, KindWorker)
	connect(t, g, stream, w)
	must(t, g, Command{Type: CmdScale, Node: w, Replicas: 8})
	warm(g)
	if gs := groupOf(t, streamStats(t, g, stream), w); gs.Members != 8 || gs.Active != 6 {
		t.Fatalf("8 consumers on 6 partitions leave 2 idle: %+v", gs)
	}
}

func TestASlowGroupLagsAndLosesEventsPastRetention(t *testing.T) {
	g := newGameV12(t)
	_, stream := eventApp(t, g, 50)
	cfg := *g.Rules.Stream
	cfg.RetentionHours = 1
	must(t, g, Command{Type: CmdConfigure, Node: stream, Stream: &cfg})
	w := place(t, g, KindWorker)
	wc := *g.Rules.Worker
	wc.Concurrency = 1 // about 16 events/s
	wc.Handler.Deps = nil
	must(t, g, Command{Type: CmdConfigure, Node: w, Worker: &wc})
	connect(t, g, stream, w)
	g.Step()
	early := groupOf(t, streamStats(t, g, stream), w)
	if !(early.Lag > 0) || !(early.LagSeconds > 0) || early.Lost != 0 {
		t.Fatalf("a slow group falls behind: %+v", early)
	}
	for range 24 {
		g.Step()
	}
	late := groupOf(t, streamStats(t, g, stream), w)
	if !(late.Lost > 0) || !near(late.Lag, 50*3600, 1e-6) || streamStats(t, g, stream).Health != HealthUnhealthy {
		t.Fatalf("after an hour behind, events expire unread: %+v", late)
	}
}

func TestStreamValidatesAndReplays(t *testing.T) {
	if p := RulesetV12().ValidateStream(StreamConfig{KeySkew: 2}); len(p) != 4 {
		t.Fatalf("every problem is listed: %v", p)
	}
	a := appCfg()
	a.Routes[0].Deps = []string{DepStream}
	if p := RulesetV11().ValidateApp(a); len(p) == 0 {
		t.Fatal("v11 has no streams to publish to")
	}
	g := New(RulesetV12(), 5)
	g.FreeBuild = true
	_, stream := eventApp(t, g, 10)
	connect(t, g, stream, place(t, g, KindWorker))
	for range 200 {
		g.Step()
	}
	cfg := *g.Rules.Stream
	cfg.Partitions = 2
	must(t, g, Command{Type: CmdConfigure, Node: stream, Stream: &cfg})
	for range 100 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Last, g.Last) || !reflect.DeepEqual(again.Nodes, g.Nodes) {
		t.Fatalf("a v12 game must replay exactly: %v", err)
	}
}
