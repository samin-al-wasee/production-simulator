package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Report explains a tick (Phase 12.2, ADR-0028): why requests failed and
// where, what a request did on its way through, and what the components
// logged. It is derived from the model after the solve and changes nothing
// in it; what is marked Seen is what the player's telemetry captured.
type Report struct {
	Causes []Cause     `json:"causes"`
	Traces []Trace     `json:"traces"`
	Logs   []LogRecord `json:"logs"`
}

// Cause is failures per second of one reason at one place. An Async cause
// (a dead letter, an event lost) fails no user's request.
type Cause struct {
	Reason string  `json:"reason"`
	Place  string  `json:"place"`
	Detail string  `json:"detail,omitempty"`
	RPS    float64 `json:"rps"`
	Async  bool    `json:"async,omitempty"`
	// Seen: the place logs errors to a log store that keeps them.
	Seen bool `json:"seen"`
}

// Span is one step of a request: its place, when it starts and how long it
// takes (the model's mean for requests that succeed), and the chance it
// succeeds.
type Span struct {
	Name       string  `json:"name"`
	Place      string  `json:"place"`
	StartMs    float64 `json:"startMs"`
	DurationMs float64 `json:"durationMs"`
	OK         float64 `json:"ok"`
	Async      bool    `json:"async,omitempty"`
	Children   []Span  `json:"children,omitempty"`
}

// Trace is a request for one endpoint from one traffic component. Rate is
// how many such traces per second the entry component samples.
type Trace struct {
	Source   string  `json:"source"`
	Endpoint string  `json:"endpoint"`
	Rate     float64 `json:"rate"`
	Seen     bool    `json:"seen"`
	Root     Span    `json:"root"`
}

// LogRecord is an aggregated log line: what a component logs, how often.
// Only lines a log store kept are recorded.
type LogRecord struct {
	Level   string  `json:"level"`
	Place   string  `json:"place"`
	Message string  `json:"message"`
	Rate    float64 `json:"rate"`
}

// NodeSample is one tick of a monitored component's metrics.
type NodeSample struct {
	Tick        int     `json:"tick"`
	RPS         float64 `json:"rps"`
	Errors      float64 `json:"errors"`
	LatencyMs   float64 `json:"latencyMs"`
	Utilization float64 `json:"utilization"`
}

const maxSpanDepth = 8

// Report is the last tick's explanation (v14 and later; empty before).
func (g *Game) Report() *Report {
	return g.Last.report
}

// NodeHistory is each monitored component's metrics, oldest first.
func (g *Game) NodeHistory() map[string][]NodeSample {
	out := make(map[string][]NodeSample, len(g.nodeHistory))
	for id, h := range g.nodeHistory {
		out[id] = append([]NodeSample(nil), h...)
	}
	return out
}

func levelRank(l string) int { return slices.Index(LogLevels, l) }

// logsSeen reports whether node n's lines at a level reach a log store.
func logsSeen(n *Node, o *ObsStats, t *Telemetry, level string) bool {
	return o != nil && o.LogCoverage > 0 && levelRank(t.LogLevel) >= levelRank(level)
}

// buildReport explains the solve that produced stats and t.
func (g *Game) buildReport(stats []NodeStats, t []vec) *Report {
	r := &Report{}
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	seen := func(id string) bool {
		i, ok := idx[id]
		return ok && logsSeen(g.Nodes[i], stats[i].Obs, g.telemetry(g.Nodes[i]), "error")
	}
	add := func(reason, place, detail string, rps float64, async bool) {
		if rps > 1e-6 {
			r.Causes = append(r.Causes, Cause{Reason: reason, Place: place, Detail: detail, RPS: rps, Async: async, Seen: seen(place)})
		}
	}
	for i, n := range g.Nodes {
		s := stats[i]
		if n.Kind == KindTraffic || backendKind(n.Kind) {
			continue
		}
		if n.Down {
			why := "down"
			if g.Tick < n.CrashedUntil {
				why = "crashed: out of memory"
			}
			add(why, n.ID, "every replica is down", s.Offered, false)
			continue
		}
		switch {
		case s.App != nil:
			g.appCauses(i, s, add)
		case s.DB != nil:
			add("too many connections", n.ID, fmt.Sprintf("%.0f open, %d allowed", s.DB.Connections, s.DB.MaxConnections), s.DB.Refused, false)
			add("overloaded", n.ID, "bottleneck "+s.DB.Bottleneck, s.Dropped, false)
		case s.Cache != nil:
			add("too many connections", n.ID, "", s.Cache.Refused, false)
			add("overloaded", n.ID, "bottleneck "+s.Cache.Bottleneck, s.Dropped, false)
		case s.Storage != nil:
			add("throttled", n.ID, "request rate above the prefixes'", s.Storage.Throttled, false)
		case s.Edge != nil:
			add("not found", n.ID, "no gateway route", s.Edge.NotFound, false)
			add("rate limited", n.ID, "429 above the limit", s.Edge.Limited, false)
			add("overloaded", n.ID, "", s.Dropped, false)
		case s.Queue != nil:
			add("queue full", n.ID, "publishes rejected", s.Queue.Rejected, false)
			add("dead-lettered", n.ID, "workers failed every delivery", s.Queue.DeadLettered, true)
		case s.Stream != nil:
			add("throttled", n.ID, "above the partitions' throughput", s.Stream.Throttled, false)
			for _, gs := range s.Stream.Groups {
				add("events lost", n.ID, gs.Consumer+" fell behind the retention", gs.Lost, true)
			}
		default:
			add("overloaded", n.ID, "", s.Dropped, false)
		}
	}
	if g.edgeRun != nil {
		for _, e := range g.Edges {
			if es, ok := g.edgeRun[e.From+">"+e.To]; ok && es.Problem != "" {
				c := Cause{Reason: "contract", Place: e.From + " → " + e.To, Detail: es.Problem, RPS: es.RPS, Seen: seen(e.From) || seen(e.To)}
				if c.RPS > 1e-6 {
					r.Causes = append(r.Causes, c)
				}
			}
		}
	}
	for _, cl := range g.clients {
		if cl.problem == "not connected" && cl.attempts > 0 {
			r.Causes = append(r.Causes, Cause{Reason: "not connected", Place: g.Nodes[cl.node].ID, Detail: "the traffic reaches no component", RPS: cl.attempts})
		}
	}
	if f := g.fx.failShare; f > 0 {
		front := false
		for _, cl := range g.clients {
			if cl.to >= 0 && seen(g.Nodes[cl.to].ID) {
				front = true
			}
		}
		total := 0.0
		for _, cl := range g.clients {
			total += cl.attempts
		}
		r.Causes = append(r.Causes, Cause{Reason: "third-party outage", Place: "outside", Detail: "a provider the requests depend on fails", RPS: total * f, Seen: front})
	}
	slices.SortStableFunc(r.Causes, func(a, b Cause) int {
		switch {
		case a.RPS > b.RPS:
			return -1
		case a.RPS < b.RPS:
			return 1
		}
		return strings.Compare(a.Place, b.Place)
	})
	r.Traces = g.traces(stats, t)
	r.Logs = g.logs(stats, r.Causes)
	return r
}

// appCauses attributes an application's failures: rejected at the door,
// timed out, its handler's own errors, routes it has none for, and calls
// that failed, named by the connections that failed them.
func (g *Game) appCauses(i int, s NodeStats, add func(reason, place, detail string, rps float64, async bool)) {
	n := g.Nodes[i]
	a := s.App
	run := g.runs[i]
	cfg := g.appConfig(n)
	add("rejected", n.ID, "backlog full, or rate-limited", a.Rejected, false)
	add("timed out", n.ID, fmt.Sprintf("waits past the %s ms timeout", trimFloat(cfg.TimeoutMs)), a.Timeouts, false)
	if run == nil {
		return
	}
	handler, deps, missing, absent := 0.0, 0.0, 0.0, 0.0
	var nothing []string
	sv := run.servedShare()
	for _, rr := range run.routes {
		if rr.notFound {
			missing += rr.rate * sv
			continue
		}
		_, late, _ := rr.outcome(sv, run.waitMs, cfg.TimeoutMs)
		done := rr.rate * sv * (1 - late)
		handler += done * rr.route.ErrorRate
		// A route calling something nothing is connected for always fails;
		// the rest fail with the calls that failed.
		lacks := false
		for _, c := range rr.calls {
			if len(c.to) == 0 && !c.async {
				lacks = true
				if !slices.Contains(nothing, c.what) {
					nothing = append(nothing, c.what)
				}
			}
		}
		if lacks {
			absent += done * (1 - rr.route.ErrorRate)
		} else {
			deps += done * (1 - rr.route.ErrorRate) * (1 - rr.ok)
		}
	}
	add("not found", n.ID, "no route for the endpoint", missing, false)
	add("missing dependency", n.ID, "nothing connected for "+strings.Join(nothing, ", "), absent, false)
	add("handler error", n.ID, "the route's own error rate", handler, false)
	var failing []string
	for _, e := range g.Edges {
		if es, ok := g.edgeRun[e.From+">"+e.To]; ok && e.From == n.ID && es.RPS > 0 && es.Errors/es.RPS > 0.001 {
			failing = append(failing, e.To)
		}
	}
	detail := "a call it depends on failed"
	if len(failing) > 0 {
		detail = "calls to " + strings.Join(failing, ", ") + " failed"
	}
	add("dependency failed", n.ID, detail, deps, false)
}

// traces builds a trace for every endpoint every traffic component asks
// for, through each hop's mean latency.
func (g *Game) traces(stats []NodeStats, t []vec) []Trace {
	var out []Trace
	for _, cl := range g.clients {
		if cl.to < 0 || cl.problem != "" {
			continue
		}
		entry := g.Nodes[cl.to]
		tel := g.telemetry(entry)
		o := stats[cl.to].Obs
		for _, e := range cl.cfg.Endpoints {
			if e.Share <= 0 {
				continue
			}
			root := g.span(cl.to, e.Name, endpointClass(e.Name), 0, 0, stats, t)
			tr := Trace{Source: g.Nodes[cl.node].ID, Endpoint: e.Name, Root: root, Rate: cl.attempts * e.Share * tel.TraceSampling}
			tr.Seen = o != nil && o.TraceCoverage > 0 && tel.TraceSampling > 0
			out = append(out, tr)
		}
	}
	return out
}

// span is the request for endpoint name entering node j at start.
func (g *Game) span(j int, name string, cls int, start float64, depth int, stats []NodeStats, t []vec) Span {
	n := g.Nodes[j]
	hop := g.Rules.NetworkHopMs
	if run := g.runs[j]; run != nil && depth < maxSpanDepth {
		cfg := g.appConfig(n)
		rte, _ := g.route(cfg, name)
		k, ok := run.byRoute[rte.Endpoint]
		if !ok {
			return Span{Name: n.ID + " " + name, Place: n.ID, StartMs: start}
		}
		rr := run.routes[k]
		p, _, ms := rr.outcome(run.servedShare(), run.waitMs, cfg.TimeoutMs)
		sp := Span{Name: n.ID + " " + rte.Endpoint, Place: n.ID, StartMs: start, DurationMs: ms, OK: p}
		wait := math.Max(0, ms-rr.own-rr.depMs)
		if wait > 0.05 {
			sp.Children = append(sp.Children, Span{Name: "queue wait", Place: n.ID, StartMs: start, DurationMs: wait, OK: 1})
		}
		cursor := start + wait
		sp.Children = append(sp.Children, Span{Name: "handler", Place: n.ID, StartMs: cursor, DurationMs: rr.own, OK: 1 - rr.route.ErrorRate})
		cursor += rr.own
		for _, c := range rr.calls {
			if len(c.to) == 0 {
				sp.Children = append(sp.Children, Span{Name: "call " + c.what + ": nothing connected", Place: n.ID, StartMs: cursor, OK: 0, Async: c.async})
				continue
			}
			x := c.to[0]
			for _, r := range c.to {
				if r.share > x.share {
					x = r
				}
			}
			var child Span
			if c.endpoint != "" {
				child = g.span(x.to, c.endpoint, c.cls, cursor+hop, depth+1, stats, t)
			} else {
				target := g.Nodes[x.to]
				child = Span{Name: target.ID + " " + [3]string{"read", "read", "write"}[c.cls], Place: target.ID, StartMs: cursor + hop,
					DurationMs: t[x.to][c.cls], OK: 1}
			}
			if g.problem[[2]int{j, x.to}] != "" {
				child = Span{Name: g.problem[[2]int{j, x.to}], Place: g.Nodes[x.to].ID, StartMs: cursor, OK: 0}
			}
			if c.async {
				child.Async = true
				sp.Children = append(sp.Children, child)
				continue
			}
			sp.Children = append(sp.Children, child)
			cursor += hop + child.DurationMs
		}
		return sp
	}
	var f fwd
	ok := false
	if g.fwds != nil && g.fwds[j] != nil {
		f, ok = g.fwds[j][name]
	}
	if ok && depth < maxSpanDepth {
		o := g.epOut[j][name]
		sp := Span{Name: n.ID + " " + n.Kind, Place: n.ID, StartMs: start, DurationMs: o[1], OK: o[0]}
		if f.hit >= 0.5 {
			sp.Children = append(sp.Children, Span{Name: "edge hit", Place: n.ID, StartMs: start, DurationMs: g.Rules.EdgeRuntime.HitMs, OK: 1})
			return sp
		}
		if len(f.to) > 0 {
			x := f.to[0]
			for _, r := range f.to {
				if r.share > x.share {
					x = r
				}
			}
			own := stats[j].LatencyMs
			sp.Children = append(sp.Children, g.span(x.to, name, cls, start+own, depth+1, stats, t))
		}
		return sp
	}
	return Span{Name: n.ID, Place: n.ID, StartMs: start, DurationMs: t[j][cls], OK: 1}
}

// logs are the lines each component logs at its level, aggregated, kept by
// a log store: errors from the causes; warnings for routes slower than the
// SLO; one info line per route; a debug line per connection.
func (g *Game) logs(stats []NodeStats, causes []Cause) []LogRecord {
	var out []LogRecord
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	kept := func(i int, level string, rate float64) float64 {
		n := g.Nodes[i]
		t := g.telemetry(n)
		o := stats[i].Obs
		if !logsSeen(n, o, t, level) {
			return 0
		}
		return rate * t.LogSampling * o.LogCoverage
	}
	for _, c := range causes {
		if i, ok := idx[c.Place]; ok {
			if v := kept(i, "error", c.RPS); v > 0 {
				msg := c.Reason
				if c.Detail != "" {
					msg += ": " + c.Detail
				}
				out = append(out, LogRecord{Level: "error", Place: c.Place, Message: msg, Rate: v})
			}
		}
	}
	slo := g.Rules.SLOp95Ms
	for i, n := range g.Nodes {
		s := stats[i]
		if s.App == nil {
			continue
		}
		for _, r := range s.App.Routes {
			if r.LatencyMs > slo/2 {
				if v := kept(i, "warn", r.RPS*g.Rules.TelemetryRuntime.SlowShare); v > 0 {
					out = append(out, LogRecord{Level: "warn", Place: n.ID, Message: fmt.Sprintf("slow %s: %.0f ms", r.Endpoint, r.LatencyMs), Rate: v})
				}
			}
			if v := kept(i, "info", r.Success); v > 0 {
				out = append(out, LogRecord{Level: "info", Place: n.ID, Message: fmt.Sprintf("200 %s in %.0f ms", r.Endpoint, r.LatencyMs), Rate: v})
			}
		}
		for _, e := range g.Edges {
			if es, ok := g.edgeRun[e.From+">"+e.To]; ok && e.From == n.ID && es.RPS > 0 {
				if v := kept(i, "debug", es.RPS); v > 0 {
					out = append(out, LogRecord{Level: "debug", Place: n.ID, Message: fmt.Sprintf("call %s in %.1f ms", e.To, es.LatencyMs), Rate: v})
				}
			}
		}
	}
	return out
}

// recordHistory keeps a sample of every monitored component's metrics.
func (g *Game) recordHistory(stats []NodeStats) {
	if g.nodeHistory == nil {
		g.nodeHistory = map[string][]NodeSample{}
	}
	for _, s := range stats {
		if s.Obs == nil || !s.Obs.Metrics || s.Obs.MetricsCoverage == 0 {
			continue
		}
		errs := s.Dropped
		if s.App != nil {
			errs = s.App.Errors + s.App.Timeouts + s.App.Rejected
		}
		h := append(g.nodeHistory[s.ID], NodeSample{Tick: g.Tick, RPS: s.Offered, Errors: errs, LatencyMs: s.LatencyMs, Utilization: s.Utilization})
		if len(h) > historyLimit {
			h = h[len(h)-historyLimit:]
		}
		g.nodeHistory[s.ID] = h
	}
}
