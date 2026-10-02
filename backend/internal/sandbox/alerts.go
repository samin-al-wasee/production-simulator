package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// CmdMonitor replaces the game's alert rules and SLOs (v14, ADR-0029).
const CmdMonitor = "monitor"

// Alert states.
const (
	AlertOK      = "ok"
	AlertPending = "pending"
	AlertFiring  = "firing"
	AlertNoData  = "no data"
)

// Metrics an alert or SLO can watch. A component's come from its metrics;
// the system's from the meters, which a monitored front component feeds.
var alertMetrics = []string{"rps", "errors", "error-rate", "latency", "utilization", "p95", "health"}

// AlertRule fires when a metric of a component (or of the system, when Node
// is empty) has been above or below a threshold for ForTicks ticks in a row.
type AlertRule struct {
	Name      string  `json:"name"`
	Node      string  `json:"node,omitempty"`
	Metric    string  `json:"metric"`
	Op        string  `json:"op"`
	Threshold float64 `json:"threshold"`
	ForTicks  int     `json:"forTicks"`
}

// SLO is an availability target over a window of ticks, for a component or
// the system.
type SLO struct {
	Name        string  `json:"name"`
	Node        string  `json:"node,omitempty"`
	Target      float64 `json:"target"`
	WindowTicks int     `json:"windowTicks"`
}

// Monitoring is the player's alert rules and SLOs.
type Monitoring struct {
	Alerts []AlertRule `json:"alerts"`
	SLOs   []SLO       `json:"slos"`
}

// AlertState is a rule's state after the last tick.
type AlertState struct {
	Rule  string  `json:"rule"`
	State string  `json:"state"`
	Value float64 `json:"value"`
	// Since is the tick the condition started holding, or -1.
	Since int `json:"since"`
}

// AlertEvent is an alert that fired, and when it resolved.
type AlertEvent struct {
	Rule     string  `json:"rule"`
	Fired    int     `json:"fired"`
	Resolved *int    `json:"resolved,omitempty"`
	Value    float64 `json:"value"`
}

// SLOStatus is an SLO measured over its window from what was observed.
type SLOStatus struct {
	Name         string  `json:"name"`
	Target       float64 `json:"target"`
	Requests     float64 `json:"requests"`
	Failed       float64 `json:"failed"`
	Availability float64 `json:"availability"`
	// BudgetLeft is the share of the window's error budget not yet spent;
	// BurnRate how fast the last hour spent it (1 spends it in exactly the
	// window).
	BudgetLeft float64 `json:"budgetLeft"`
	BurnRate   float64 `json:"burnRate"`
	// Observed is the share of the window's ticks with data.
	Observed float64 `json:"observed"`
}

// TimelineEntry is one thing that happened, for the incident timeline.
type TimelineEntry struct {
	Tick int    `json:"tick"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

const (
	maxRules     = 20
	alertLogKept = 50
)

// observed is a metric's value this tick and whether it was seen.
func (g *Game) observed(node, metric string) (float64, bool) {
	m := g.Last.Meters
	if node == "" {
		if !m.Monitored {
			return 0, false
		}
		switch metric {
		case "rps":
			return m.RPS, true
		case "error-rate":
			return m.ErrorRate, true
		case "errors":
			return m.RPS * m.ErrorRate, true
		case "p95", "latency":
			return m.P95LatencyMs, true
		case "health":
			return m.Health, true
		}
		return 0, false
	}
	for _, s := range g.Last.Flow.Nodes {
		if s.ID != node {
			continue
		}
		if s.Obs == nil || !s.Obs.Metrics || s.Obs.MetricsCoverage == 0 {
			return 0, false
		}
		errs := s.Dropped
		if s.App != nil {
			errs = s.App.Errors + s.App.Timeouts + s.App.Rejected
		}
		switch metric {
		case "rps":
			return s.Offered, true
		case "errors":
			return errs, true
		case "error-rate":
			if s.Offered == 0 {
				return 0, true
			}
			return errs / s.Offered, true
		case "latency", "p95":
			return s.LatencyMs, true
		case "utilization":
			return s.Utilization, true
		}
	}
	return 0, false
}

// evaluateAlerts runs after a tick is simulated: each rule's state moves,
// and alerts fire and resolve.
func (g *Game) evaluateAlerts() {
	if g.Monitoring == nil {
		return
	}
	if g.alertStates == nil {
		g.alertStates = map[string]*AlertState{}
	}
	tick := g.Tick - 1 // the tick just simulated
	for _, r := range g.Monitoring.Alerts {
		st := g.alertStates[r.Name]
		if st == nil {
			st = &AlertState{Rule: r.Name, State: AlertOK, Since: -1}
			g.alertStates[r.Name] = st
		}
		v, ok := g.observed(r.Node, r.Metric)
		st.Value = v
		holds := ok && (r.Op == ">" && v > r.Threshold || r.Op == "<" && v < r.Threshold)
		was := st.State
		switch {
		case !ok:
			st.State, st.Since = AlertNoData, -1
		case !holds:
			st.State, st.Since = AlertOK, -1
		default:
			if st.Since < 0 {
				st.Since = tick
			}
			st.State = AlertPending
			if tick-st.Since+1 >= r.ForTicks {
				st.State = AlertFiring
			}
		}
		if st.State == AlertFiring && was != AlertFiring {
			g.AlertLog = append(g.AlertLog, AlertEvent{Rule: r.Name, Fired: tick, Value: v})
		}
		if was == AlertFiring && st.State != AlertFiring {
			for k := len(g.AlertLog) - 1; k >= 0; k-- {
				if e := &g.AlertLog[k]; e.Rule == r.Name && e.Resolved == nil {
					t := tick
					e.Resolved = &t
					break
				}
			}
		}
	}
	if len(g.AlertLog) > alertLogKept {
		g.AlertLog = g.AlertLog[len(g.AlertLog)-alertLogKept:]
	}
}

// AlertStates lists each rule's state in rule order.
func (g *Game) AlertStates() []AlertState {
	if g.Monitoring == nil {
		return nil
	}
	var out []AlertState
	for _, r := range g.Monitoring.Alerts {
		if st := g.alertStates[r.Name]; st != nil {
			out = append(out, *st)
		} else {
			out = append(out, AlertState{Rule: r.Name, State: AlertNoData, Since: -1})
		}
	}
	return out
}

// SLOStatuses measures every SLO over its window from what was observed:
// the system's meters while monitored, or a component's metrics history.
func (g *Game) SLOStatuses() []SLOStatus {
	if g.Monitoring == nil {
		return nil
	}
	hour := int(math.Round(3600 / g.Rules.TickSeconds))
	var out []SLOStatus
	for _, s := range g.Monitoring.SLOs {
		type sample struct{ rps, errs float64 }
		var samples []sample
		seen := 0
		if s.Node == "" {
			h := g.History
			if len(h) > s.WindowTicks {
				h = h[len(h)-s.WindowTicks:]
			}
			for _, m := range h {
				if m.Monitored {
					samples = append(samples, sample{m.RPS, m.RPS * m.ErrorRate})
					seen++
				}
			}
		} else {
			h := g.nodeHistory[s.Node]
			if len(h) > s.WindowTicks {
				h = h[len(h)-s.WindowTicks:]
			}
			for _, x := range h {
				if x.Tick > g.Tick-s.WindowTicks {
					samples = append(samples, sample{x.RPS, x.Errors})
					seen++
				}
			}
		}
		st := SLOStatus{Name: s.Name, Target: s.Target, Observed: float64(seen) / float64(s.WindowTicks)}
		for _, x := range samples {
			st.Requests += x.rps
			st.Failed += math.Min(x.errs, x.rps)
		}
		budget := (1 - s.Target) * st.Requests
		st.Availability = 1
		if st.Requests > 0 {
			st.Availability = 1 - st.Failed/st.Requests
		}
		st.BudgetLeft = 1
		if budget > 0 {
			st.BudgetLeft = 1 - st.Failed/budget
		}
		recent := samples
		if len(recent) > hour {
			recent = recent[len(recent)-hour:]
		}
		rr, rf := 0.0, 0.0
		for _, x := range recent {
			rr += x.rps
			rf += math.Min(x.errs, x.rps)
		}
		if rr > 0 {
			st.BurnRate = (rf / rr) / (1 - s.Target)
		}
		out = append(out, st)
	}
	return out
}

// Timeline lines up the events, the alerts, and the player's commands, in
// tick order, for the incident review.
func (g *Game) Timeline() []TimelineEntry {
	var out []TimelineEntry
	for _, e := range g.Events {
		out = append(out, TimelineEntry{Tick: e.Start, Kind: "event", Text: e.Label + " starts"})
		if e.End <= g.Tick {
			out = append(out, TimelineEntry{Tick: e.End, Kind: "event", Text: e.Label + " ends"})
		}
	}
	for _, a := range g.AlertLog {
		out = append(out, TimelineEntry{Tick: a.Fired, Kind: "alert", Text: fmt.Sprintf("%s fired (%s)", a.Rule, trimFloat(a.Value))})
		if a.Resolved != nil {
			out = append(out, TimelineEntry{Tick: *a.Resolved, Kind: "alert", Text: a.Rule + " resolved"})
		}
	}
	for _, c := range g.Log {
		if c.Command.Type == CmdMove {
			continue
		}
		text := c.Command.Type
		switch {
		case c.Command.Node != "":
			text += " " + c.Command.Node
		case c.Command.From != "":
			text += " " + c.Command.From + " → " + c.Command.To
		case c.Command.Kind != "":
			text += " " + c.Command.Kind
		}
		if c.Command.Action != "" {
			text += " (" + c.Command.Action + ")"
		}
		out = append(out, TimelineEntry{Tick: c.Tick, Kind: "command", Text: text})
	}
	slices.SortStableFunc(out, func(a, b TimelineEntry) int { return a.Tick - b.Tick })
	if len(out) > 200 {
		out = out[len(out)-200:]
	}
	return out
}

func (g *Game) monitor(m *Monitoring) error {
	if !g.telemetryModel() {
		return invalid("ruleset %s has no alerts or SLOs", g.Rules.Version)
	}
	if m == nil {
		return invalid("monitor needs alert rules and SLOs")
	}
	if p := g.validateMonitoring(*m); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	g.Monitoring = m
	// Rules that changed start over.
	kept := map[string]*AlertState{}
	for _, r := range m.Alerts {
		if st, ok := g.alertStates[r.Name]; ok {
			kept[r.Name] = st
		}
	}
	g.alertStates = kept
	return nil
}

func (g *Game) validateMonitoring(m Monitoring) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if len(m.Alerts) > maxRules || len(m.SLOs) > maxRules {
		bad("at most %d alert rules and %d SLOs", maxRules, maxRules)
	}
	names := map[string]bool{}
	name := func(label, n string) {
		switch {
		case strings.TrimSpace(n) == "" || len(n) > maxNameLen:
			bad("%s: name must be 1 to %d characters", label, maxNameLen)
		case names[n]:
			bad("%q is used twice", n)
		}
		names[n] = true
	}
	node := func(label, id string) {
		if id != "" && g.Node(id) == nil {
			bad("%s: no component %q", label, id)
		}
	}
	for k, r := range m.Alerts {
		label := fmt.Sprintf("alert %d", k+1)
		name(label, r.Name)
		node(label, r.Node)
		if !slices.Contains(alertMetrics, r.Metric) {
			bad("%s: metric must be one of %s", label, strings.Join(alertMetrics, ", "))
		}
		if r.Op != ">" && r.Op != "<" {
			bad("%s: condition must be > or <", label)
		}
		if math.IsNaN(r.Threshold) || math.IsInf(r.Threshold, 0) {
			bad("%s: threshold must be a number", label)
		}
		if r.ForTicks < 1 || r.ForTicks > 288 {
			bad("%s: it must hold for 1 to 288 ticks", label)
		}
	}
	for k, s := range m.SLOs {
		label := fmt.Sprintf("SLO %d", k+1)
		name(label, s.Name)
		node(label, s.Node)
		if !(s.Target >= 0.5 && s.Target < 1) {
			bad("%s: target must be from 50%% to under 100%%", label)
		}
		if s.WindowTicks < 12 || s.WindowTicks > 2016 {
			bad("%s: window must be from 1 hour to 7 days (12 to 2016 ticks)", label)
		}
	}
	return p
}
