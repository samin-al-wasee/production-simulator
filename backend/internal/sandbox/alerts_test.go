package sandbox

import (
	"reflect"
	"strings"
	"testing"
)

func monitorWith(t *testing.T, g *Game, m Monitoring) {
	t.Helper()
	must(t, g, Command{Type: CmdMonitor, Monitoring: &m})
}

func stateOf(g *Game, rule string) AlertState {
	for _, s := range g.AlertStates() {
		if s.Rule == rule {
			return s
		}
	}
	return AlertState{}
}

func TestAlertsFireOnlyOnWhatIsObserved(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	a := appCfg()
	for i := range a.Routes {
		a.Routes[i].ErrorRate = 0.1
	}
	setApp(t, g, app, a)
	traffic(t, g, app, constant(client(), 20))
	monitorWith(t, g, Monitoring{Alerts: []AlertRule{
		{Name: "app errors", Node: app, Metric: "error-rate", Op: ">", Threshold: 0.05, ForTicks: 2},
		{Name: "system errors", Metric: "error-rate", Op: ">", Threshold: 0.05, ForTicks: 1},
	}})
	g.Step()
	if s := stateOf(g, "app errors"); s.State != AlertNoData {
		t.Fatalf("a rule on an unmonitored component has no data: %+v", s)
	}
	place(t, g, KindMetrics)
	instrument(t, g, app, Telemetry{Metrics: true, ResolutionSeconds: 60, LogLevel: "off", LogSampling: 1})
	g.Step()
	if s := stateOf(g, "app errors"); s.State != AlertPending || !near(s.Value, 0.1, 0.01) {
		t.Fatalf("the condition holds for one tick of two: pending %+v", s)
	}
	if s := stateOf(g, "system errors"); s.State != AlertFiring {
		t.Fatalf("the front app's metrics make the system's meters observed: %+v", s)
	}
	g.Step()
	if s := stateOf(g, "app errors"); s.State != AlertFiring {
		t.Fatalf("held for two ticks, it fires: %+v", s)
	}
	setApp(t, g, app, appCfg())
	g.Step()
	if s := stateOf(g, "app errors"); s.State != AlertOK {
		t.Fatalf("fixed, it resolves: %+v", s)
	}
	var fired []AlertEvent
	for _, e := range g.AlertLog {
		if e.Rule == "app errors" {
			fired = append(fired, e)
		}
	}
	if len(fired) != 1 || fired[0].Resolved == nil || *fired[0].Resolved != g.Tick-1 {
		t.Fatalf("the alert fired once and resolved on the last tick: %+v", fired)
	}
	tl := g.Timeline()
	var kinds []string
	for _, e := range tl {
		kinds = append(kinds, e.Kind+": "+e.Text)
	}
	joined := strings.Join(kinds, "\n")
	if !strings.Contains(joined, "alert: app errors fired") || !strings.Contains(joined, "alert: app errors resolved") || !strings.Contains(joined, "command: configure "+app) {
		t.Fatalf("the timeline lines up alerts and commands:\n%s", joined)
	}
}

func TestSLOsSpendAnErrorBudget(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	a := appCfg()
	for i := range a.Routes {
		a.Routes[i].ErrorRate = 0.02
	}
	setApp(t, g, app, a)
	traffic(t, g, app, constant(client(), 20))
	place(t, g, KindMetrics)
	instrument(t, g, app, Telemetry{Metrics: true, ResolutionSeconds: 60, LogLevel: "off", LogSampling: 1})
	monitorWith(t, g, Monitoring{SLOs: []SLO{{Name: "front", Node: app, Target: 0.99, WindowTicks: 24}}})
	for range 12 {
		g.Step()
	}
	s := g.SLOStatuses()[0]
	// 2% failing against a 1% budget: twice the budget, so it is spent.
	if !near(s.Availability, 0.98, 0.005) || !near(s.BurnRate, 2, 0.1) || !(s.BudgetLeft < 0) || !near(s.Observed, 0.5, 1e-9) {
		t.Fatalf("an SLO spends its budget at twice the allowed rate: %+v", s)
	}
}

func TestMonitoringValidatesAndReplays(t *testing.T) {
	g := newGameV14(t)
	err := g.monitor(&Monitoring{Alerts: []AlertRule{{Name: "", Node: "nope", Metric: "vibes", Op: "=", ForTicks: 0}}, SLOs: []SLO{{Name: "x", Target: 1, WindowTicks: 1}}})
	for _, want := range []string{"name", "no component", "metric", "condition", "hold", "target", "window"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q in %v", want, err)
		}
	}
	if _, err := New(RulesetV13(), 1).Apply(Command{Type: CmdMonitor, Monitoring: &Monitoring{}}); err == nil {
		t.Fatal("v13 has no alerts")
	}
	g = New(RulesetV14(), 3)
	g.FreeBuild = true
	app := appWithDeps(t, g)
	traffic(t, g, app, client())
	place(t, g, KindMetrics)
	instrument(t, g, app, Telemetry{Metrics: true, ResolutionSeconds: 60, LogLevel: "off", LogSampling: 1})
	monitorWith(t, g, Monitoring{Alerts: []AlertRule{{Name: "busy", Node: app, Metric: "rps", Op: ">", Threshold: 0.5, ForTicks: 3}},
		SLOs: []SLO{{Name: "system", Target: 0.999, WindowTicks: 288}}})
	for range 300 {
		g.Step()
	}
	again, err := Replay(g.Save())
	if err != nil || !reflect.DeepEqual(again.AlertLog, g.AlertLog) || !reflect.DeepEqual(again.AlertStates(), g.AlertStates()) || !reflect.DeepEqual(again.SLOStatuses(), g.SLOStatuses()) {
		t.Fatalf("alerts and SLOs replay exactly: %v", err)
	}
}
