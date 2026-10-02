package sandbox

import (
	"strings"
	"testing"
)

func cause(r *Report, reason, place string) (Cause, bool) {
	for _, c := range r.Causes {
		if c.Reason == reason && c.Place == place {
			return c, true
		}
	}
	return Cause{}, false
}

func TestFailuresAreAttributedToAReasonAndAPlace(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	db := g.Edges[0].To
	a := appCfg()
	for i := range a.Routes {
		a.Routes[i].ErrorRate = 0.1
	}
	setApp(t, g, app, a)
	traffic(t, g, app, constant(client(), 20))
	warm(g)
	r := g.Report()
	c, ok := cause(r, "handler error", app)
	if !ok || !near(c.RPS, 20*0.1, 0.05) || c.Seen {
		t.Fatalf("a tenth of the requests fail in the handler, unseen without logs: %+v", r.Causes)
	}
	// Seen once the app logs errors to a log store.
	place(t, g, KindLogs)
	instrument(t, g, app, Telemetry{ResolutionSeconds: 60, LogLevel: "error", LogSampling: 1})
	warm(g)
	if c, _ := cause(g.Report(), "handler error", app); !c.Seen {
		t.Fatalf("error logs in a store make the cause seen: %+v", c)
	}
	if len(g.Report().Logs) == 0 || g.Report().Logs[0].Level != "error" || !strings.Contains(g.Report().Logs[0].Message, "handler error") {
		t.Fatalf("the error is logged: %+v", g.Report().Logs)
	}

	// Too few database connections: refused at the database, and the app's
	// requests fail on its calls there.
	cfg := *g.Rules.DB
	cfg.MaxConnections = 10
	must(t, g, Command{Type: CmdConfigure, Node: db, DB: &cfg})
	warm(g)
	if c, ok := cause(g.Report(), "too many connections", db); !ok || !(c.RPS > 0) {
		t.Fatalf("refused queries are attributed to the database: %+v", g.Report().Causes)
	}
	if c, ok := cause(g.Report(), "dependency failed", app); !ok || !strings.Contains(c.Detail, db) {
		t.Fatalf("the app's failures name the connection that failed them: %+v", c)
	}
}

func TestAContractProblemIsACause(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	db := g.Edges[0].To
	traffic(t, g, app, constant(client(), 10))
	c := *g.edge(app, db).Conn
	c.Port = 1
	must(t, g, Command{Type: CmdConfigure, From: app, To: db, Connection: &c})
	warm(g)
	if x, ok := cause(g.Report(), "contract", app+" → "+db); !ok || !strings.Contains(x.Detail, "connection refused") {
		t.Fatalf("a broken contract is a cause on its connection: %+v", g.Report().Causes)
	}
}

func TestTracesFollowARequestThroughEveryHop(t *testing.T) {
	g := newGameV14(t)
	orders := service(t, g, "Orders API",
		AppRoute{Endpoint: "POST /orders", BaseMs: 50, CPUMs: 5, MemoryMB: 1, RequestKB: 1, ResponseKB: 1, Deps: []string{DepDBWrite}})
	front := service(t, g, "Front API",
		AppRoute{Endpoint: "POST /checkout", BaseMs: 10, CPUMs: 5, MemoryMB: 1, RequestKB: 1, ResponseKB: 1,
			Calls: []Call{{Service: "Orders API", Endpoint: "POST /orders"}}})
	connect(t, g, front, orders)
	c := constant(client(), 5)
	c.Endpoints = []Weight{{"POST /checkout", 1}}
	traffic(t, g, front, c)
	warm(g)
	r := g.Report()
	if len(r.Traces) != 1 || r.Traces[0].Seen {
		t.Fatalf("one trace per endpoint, unseen without sampling and a backend: %+v", r.Traces)
	}
	root := r.Traces[0].Root
	var call *Span
	for i := range root.Children {
		if root.Children[i].Place == orders {
			call = &root.Children[i]
		}
	}
	if root.Place != front || call == nil || len(call.Children) == 0 || !(call.DurationMs >= 50) || !(root.DurationMs >= call.DurationMs+10) {
		t.Fatalf("the checkout span holds its handler and the orders call, which holds its own: %+v", root)
	}
	place(t, g, KindTraces)
	instrument(t, g, front, Telemetry{ResolutionSeconds: 60, LogLevel: "off", LogSampling: 1, TraceSampling: 0.1})
	warm(g)
	if tr := g.Report().Traces[0]; !tr.Seen || !near(tr.Rate, 0.5, 1e-9) {
		t.Fatalf("sampled at 10%% into a trace backend, 0.5 traces/s are kept: %+v", tr)
	}
}

func TestMonitoredComponentsKeepAHistory(t *testing.T) {
	g := newGameV14(t)
	app := appWithDeps(t, g)
	db := g.Edges[0].To
	traffic(t, g, app, constant(client(), 10))
	place(t, g, KindMetrics)
	instrument(t, g, app, Telemetry{Metrics: true, ResolutionSeconds: 60, LogLevel: "off", LogSampling: 1})
	for range 5 {
		g.Step()
	}
	h := g.NodeHistory()
	if len(h[app]) != 5 || !near(h[app][4].RPS, 10, 1e-9) || len(h[db]) != 0 {
		t.Fatalf("only monitored components keep a history: %d app samples, %d db", len(h[app]), len(h[db]))
	}
}

func TestAMissingDependencyIsNotBlamedOnAConnection(t *testing.T) {
	g := newGameV14(t)
	app := place(t, g, KindApp) // no database, no storage
	traffic(t, g, app, constant(client(), 10))
	warm(g)
	c, ok := cause(g.Report(), "missing dependency", app)
	if !ok || !near(c.RPS, 10, 0.01) || !strings.Contains(c.Detail, "cache") {
		t.Fatalf("requests needing what is not connected fail as a missing dependency: %+v", g.Report().Causes)
	}
	if _, ok := cause(g.Report(), "dependency failed", app); ok {
		t.Fatal("no connection failed them")
	}
}
