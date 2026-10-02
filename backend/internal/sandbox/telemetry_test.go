package sandbox

import (
	"reflect"
	"testing"
)

func newGameV14(t *testing.T) *Game {
	t.Helper()
	r := RulesetV14()
	r.Events = nil
	g := New(r, 1)
	g.FreeBuild = true
	g.Cash = 1e9
	return g
}

func obs(t *testing.T, g *Game, id string) ObsStats {
	t.Helper()
	s := stats(t, g.Last, id)
	if s.Obs == nil {
		t.Fatalf("%s has no observability stats", id)
	}
	return *s.Obs
}

func instrument(t *testing.T, g *Game, id string, tel Telemetry) {
	t.Helper()
	must(t, g, Command{Type: CmdConfigure, Node: id, Telemetry: &tel})
}

func TestNothingIsSeenUntilItIsInstrumented(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	traffic(t, g, app, constant(client(), 20))
	warm(g)
	if o := obs(t, g, app); o.Metrics || o.MetricsCoverage != 0 || o.LogLines != 0 || g.Last.Meters.Monitored {
		t.Fatalf("a new component reports nothing, and the system's meters are dark: %+v", o)
	}
	on := *g.Rules.Telemetry
	on.Metrics = true
	instrument(t, g, app, on)
	if o := obs(t, g, app); !o.Metrics || o.MetricsCoverage != 0 || g.Last.Meters.Monitored {
		t.Fatalf("metrics with no store to keep them are still not seen: %+v", o)
	}
	store := place(t, g, KindMetrics)
	if o := obs(t, g, app); o.MetricsCoverage != 1 || !g.Last.Meters.Monitored {
		t.Fatalf("once a store keeps the front component's metrics, the system is monitored: %+v", o)
	}
	b := *stats(t, g.Last, store).Backend
	if want := 40.0 / 60; !near(b.Ingest, want, 1e-9) || b.Dropped != 0 || !(b.Cost > g.costPerHour(g.Node(store))) {
		t.Fatalf("40 series every 60 s; the store costs its replica plus its ingest: %+v", b)
	}
}

func TestLogsCostIngestAndASmallStoreDropsThem(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	traffic(t, g, app, constant(client(), 20))
	store := place(t, g, KindLogs)
	tel := *g.Rules.Telemetry
	tel.LogLevel = "info"
	instrument(t, g, app, tel)
	warm(g)
	o := obs(t, g, app)
	if !near(o.LogLines, 20, 0.01) || o.LogCoverage != 1 {
		t.Fatalf("info logs a line per request: %+v", o)
	}
	cheap := stats(t, g.Last, store).CostPerHour
	tel.LogLevel = "debug"
	instrument(t, g, app, tel)
	warm(g)
	if c := stats(t, g.Last, store).CostPerHour; !(c > cheap) {
		t.Fatalf("debug logs cost more to keep: %v vs %v", c, cheap)
	}
	// 6 lines per request at 600 req/s overflow a small log store.
	g2 := newGameV14(t)
	app2 := appWithDeps(t, g2)
	a := appCfg()
	a.Processing, a.MaxConcurrency = ProcessingAsync, 5000
	setApp(t, g2, app2, a)
	must(t, g2, Command{Type: CmdScale, Node: app2, Replicas: 20})
	must(t, g2, Command{Type: CmdResize, Node: app2, Size: "large"})
	traffic(t, g2, app2, constant(client(), 600))
	small := place(t, g2, KindLogs)
	instrument(t, g2, app2, tel)
	warm(g2)
	if b := stats(t, g2.Last, small).Backend; !(b.Dropped > 0) || b.Health != HealthUnhealthy || !(obs(t, g2, app2).LogCoverage < 1) {
		t.Fatalf("a saturated log store drops lines: %+v", b)
	}
}

func TestReportingCostsTheAppCPU(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	traffic(t, g, app, constant(client(), 20))
	warm(g)
	quiet := appStats(t, g, app).CPUUsed
	tel := *g.Rules.Telemetry
	tel.LogLevel, tel.TraceSampling = "debug", 1
	instrument(t, g, app, tel)
	warm(g)
	if loud := appStats(t, g, app).CPUUsed; !(loud > quiet*1.01) {
		t.Fatalf("debug logs and full tracing cost CPU: %v vs %v", loud, quiet)
	}
	// Every request traced: one span here and one per connection it uses.
	if o := obs(t, g, app); !near(o.Spans, 20*3, 1e-6) || o.TraceCoverage != 0 {
		t.Fatalf("spans are sampled requests × hops, and need a backend: %+v", o)
	}
}

func TestTelemetryValidatesAndReplays(t *testing.T) {
	if p := RulesetV14().ValidateTelemetry(Telemetry{LogLevel: "loud", LogSampling: 2, TraceSampling: -1}); len(p) != 4 {
		t.Fatalf("every problem is listed: %v", p)
	}
	g := New(RulesetV14(), 7)
	g.FreeBuild = true
	app := appWithDeps(t, g)
	src := traffic(t, g, app, client())
	place(t, g, KindMetrics)
	place(t, g, KindLogs)
	for range 200 {
		g.Step()
	}
	instrument(t, g, app, Telemetry{Metrics: true, ResolutionSeconds: 15, LogLevel: "warn", LogSampling: 0.5, TraceSampling: 0.1})
	for range 100 {
		g.Step()
	}
	if _, err := g.Apply(Command{Type: CmdConfigure, Node: src, Telemetry: &Telemetry{ResolutionSeconds: 60, LogLevel: "off"}}); err == nil {
		t.Fatal("traffic reports no telemetry")
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.Last, g.Last) || again.Cash != g.Cash {
		t.Fatalf("a v14 game must replay exactly: %v", err)
	}
}
