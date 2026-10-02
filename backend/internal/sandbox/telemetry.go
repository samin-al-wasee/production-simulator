package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Telemetry backends (v14, ADR-0027).
const (
	KindMetrics = "metrics-store"
	KindLogs    = "log-store"
	KindTraces  = "trace-backend"
)

// Log levels, from quietest.
var LogLevels = []string{"off", "error", "warn", "info", "debug"}

// Telemetry is what a component reports about itself (v14). Nothing is
// reported until the player turns it on.
type Telemetry struct {
	Metrics           bool    `json:"metrics"`
	ResolutionSeconds float64 `json:"resolutionSeconds"`
	LogLevel          string  `json:"logLevel"`
	// LogSampling and TraceSampling are the shares of lines and of
	// requests kept.
	LogSampling   float64 `json:"logSampling"`
	TraceSampling float64 `json:"traceSampling"`
}

// TelemetryRuntime holds the constants of the telemetry model.
type TelemetryRuntime struct {
	// Series is how many metric series a replica of a kind exports.
	Series map[string]float64 `json:"series"`
	// Log lines per request at info and debug; error and warn log failed
	// requests, warn also SlowShare of the rest.
	InfoLines  float64 `json:"infoLines"`
	DebugLines float64 `json:"debugLines"`
	SlowShare  float64 `json:"slowShare"`
	// Sizes, and what each costs the reporting application's CPU.
	SampleBytes float64 `json:"sampleBytes"`
	LineKB      float64 `json:"lineKb"`
	SpanKB      float64 `json:"spanKb"`
	LineCPUMs   float64 `json:"lineCpuMs"`
	SpanCPUMs   float64 `json:"spanCpuMs"`
	// Ingest prices per GB.
	MetricsGB float64 `json:"metricsGb"`
	LogsGB    float64 `json:"logsGb"`
	TracesGB  float64 `json:"tracesGb"`
}

// ObsStats is what can be seen of a component this tick: which signals it
// reports, how much of each reached a backend, and what reporting cost.
type ObsStats struct {
	Metrics bool `json:"metrics"`
	// Coverage is the share of the component's samples, lines, or spans a
	// backend stored; 0 without a backend.
	MetricsCoverage float64 `json:"metricsCoverage"`
	LogLevel        string  `json:"logLevel"`
	LogLines        float64 `json:"logLines"`
	LogCoverage     float64 `json:"logCoverage"`
	TraceSampling   float64 `json:"traceSampling"`
	Spans           float64 `json:"spans"`
	TraceCoverage   float64 `json:"traceCoverage"`
}

// BackendStats is what a telemetry backend took in during a tick.
type BackendStats struct {
	Health   string  `json:"health"`
	Ingest   float64 `json:"ingest"`
	Capacity float64 `json:"capacity"`
	Dropped  float64 `json:"dropped"`
	GBPerDay float64 `json:"gbPerDay"`
	Cost     float64 `json:"cost"`
}

func (g *Game) telemetryModel() bool {
	return g.Rules.Telemetry != nil
}

func backendKind(k string) bool {
	return k == KindMetrics || k == KindLogs || k == KindTraces
}

func (g *Game) telemetry(n *Node) *Telemetry {
	if n.Telemetry != nil {
		return n.Telemetry
	}
	return g.Rules.Telemetry
}

// lineFactor is how many log lines a request makes at a level, given the
// share of requests that failed.
func (rt TelemetryRuntime) lineFactor(level string, fail float64) float64 {
	switch level {
	case "error":
		return fail
	case "warn":
		return fail + (1-fail)*rt.SlowShare
	case "info":
		return rt.InfoLines + fail
	case "debug":
		return rt.DebugLines + fail
	}
	return 0
}

// overheadCPUMs is the CPU an application spends per request reporting,
// before failures are known: its log lines at the level and its spans.
func (g *Game) overheadCPUMs(n *Node, calls int) float64 {
	if !g.telemetryModel() {
		return 0
	}
	t := g.telemetry(n)
	rt := g.Rules.TelemetryRuntime
	lines := rt.lineFactor(t.LogLevel, 0) * t.LogSampling
	return lines*rt.LineCPUMs + t.TraceSampling*float64(1+calls)*rt.SpanCPUMs
}

// observe works out what every component reported this tick, what the
// backends took in, and what that cost, and attaches it to the stats.
func (g *Game) observe(stats []NodeStats) {
	r := g.Rules
	rt := r.TelemetryRuntime
	samples, lines, spans := make([]float64, len(g.Nodes)), make([]float64, len(g.Nodes)), make([]float64, len(g.Nodes))
	var ingest [3]float64
	for i, n := range g.Nodes {
		if n.Kind == KindTraffic || backendKind(n.Kind) {
			continue
		}
		t := g.telemetry(n)
		s := stats[i]
		rps := s.Offered
		failed := s.Dropped
		if s.App != nil {
			failed = s.App.Errors + s.App.Timeouts + s.App.Rejected
		}
		fail := 0.0
		if rps > 0 {
			fail = math.Min(1, failed/rps)
		}
		if t.Metrics {
			series := rt.Series[n.Kind]
			if series == 0 {
				series = rt.Series["default"]
			}
			samples[i] = series * float64(n.Replicas) / t.ResolutionSeconds
		}
		lines[i] = rt.lineFactor(t.LogLevel, fail) * rps * t.LogSampling
		out := 0
		for _, e := range g.Edges {
			if e.From == n.ID {
				out++
			}
		}
		spans[i] = t.TraceSampling * rps * float64(1+out)
		ingest[0] += samples[i]
		ingest[1] += lines[i]
		ingest[2] += spans[i]
	}
	// Each signal goes to the backends of its kind, split by capacity; what
	// they cannot take is dropped.
	coverage := [3]float64{}
	for k, kind := range []string{KindMetrics, KindLogs, KindTraces} {
		capacity := 0.0
		var backends []int
		for i, n := range g.Nodes {
			if n.Kind == kind {
				backends = append(backends, i)
				capacity += g.capacity(n)
			}
		}
		if capacity > 0 {
			coverage[k] = math.Min(1, capacity/math.Max(ingest[k], 1e-300))
		}
		size := [3]float64{rt.SampleBytes / 1e9, rt.LineKB / 1024 / 1024, rt.SpanKB / 1024 / 1024}[k]
		price := [3]float64{rt.MetricsGB, rt.LogsGB, rt.TracesGB}[k]
		for _, i := range backends {
			n := g.Nodes[i]
			c := g.capacity(n)
			share := 0.0
			if capacity > 0 {
				share = c / capacity
			}
			in := ingest[k] * share
			stored := math.Min(in, c)
			b := &BackendStats{Ingest: in, Capacity: finite(c), Dropped: in - stored, GBPerDay: stored * size * 86400}
			b.Cost = g.costPerHour(n) + stored*size*3600*price
			switch {
			case n.Down:
				b.Health = HealthStopped
			case b.Dropped > 0.2*math.Max(in, 1e-300):
				b.Health = HealthUnhealthy
			case in > 0.85*c:
				b.Health = HealthDegraded
			default:
				b.Health = HealthHealthy
			}
			stats[i].Backend, stats[i].CostPerHour = b, b.Cost
			stats[i].Offered, stats[i].Served, stats[i].Dropped = in, stored, in-stored
			stats[i].Capacity, stats[i].Utilization = finite(c), finite(in/math.Max(c, 1e-300))
		}
	}
	for i, n := range g.Nodes {
		if n.Kind == KindTraffic || backendKind(n.Kind) {
			continue
		}
		t := g.telemetry(n)
		o := &ObsStats{Metrics: t.Metrics, LogLevel: t.LogLevel, LogLines: lines[i], TraceSampling: t.TraceSampling, Spans: spans[i]}
		if t.Metrics {
			o.MetricsCoverage = coverage[0]
		}
		if lines[i] > 0 || t.LogLevel != "off" {
			o.LogCoverage = coverage[1]
		}
		if t.TraceSampling > 0 {
			o.TraceCoverage = coverage[2]
		}
		stats[i].Obs = o
	}
}

// monitored reports whether the system's technical meters can be seen: a
// component traffic reaches first reports metrics that a store keeps.
func (g *Game) monitored(stats []NodeStats) bool {
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	for _, e := range g.Edges {
		if f := g.Node(e.From); f != nil && f.Kind == KindTraffic {
			if o := stats[idx[e.To]].Obs; o != nil && o.Metrics && o.MetricsCoverage > 0 {
				return true
			}
		}
	}
	return false
}

func (g *Game) configureTelemetry(n *Node, t *Telemetry) error {
	if n.Kind == KindTraffic || backendKind(n.Kind) {
		return invalid("%s reports no telemetry of its own", n.Kind)
	}
	if t == nil {
		return invalid("configure needs a telemetry configuration")
	}
	if p := g.Rules.ValidateTelemetry(*t); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.Telemetry = t
	return nil
}

// ValidateTelemetry lists every problem with a telemetry configuration.
func (r *Ruleset) ValidateTelemetry(t Telemetry) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if !(t.ResolutionSeconds >= 10 && t.ResolutionSeconds <= 300) {
		bad("metric resolution must be between 10 and 300 s")
	}
	if !slices.Contains(LogLevels, t.LogLevel) {
		bad("log level must be one of %s", strings.Join(LogLevels, ", "))
	}
	if !(t.LogSampling >= 0 && t.LogSampling <= 1) {
		bad("log sampling must be between 0%% and 100%%")
	}
	if !(t.TraceSampling >= 0 && t.TraceSampling <= 1) {
		bad("trace sampling must be between 0%% and 100%%")
	}
	return p
}

// RulesetV14 is RulesetV13 with telemetry the player configures and pays
// for (Phase 12, ADR-0027).
func RulesetV14() *Ruleset {
	r := RulesetV13()
	r.Version = "sandbox/v14"
	r.Telemetry = &Telemetry{Metrics: false, ResolutionSeconds: 60, LogLevel: "off", LogSampling: 1, TraceSampling: 0}
	r.TelemetryRuntime = TelemetryRuntime{
		Series: map[string]float64{"default": 20, KindApp: 40, KindWorker: 30, KindDBPrimary: 50, KindDBReplica: 50,
			KindCache: 30, KindQueue: 20, KindStream: 30, KindLB: 20, KindGateway: 30, KindCDN: 20, KindStorage: 10},
		InfoLines: 1, DebugLines: 6, SlowShare: 0.05,
		SampleBytes: 2, LineKB: 0.5, SpanKB: 1,
		LineCPUMs: 0.02, SpanCPUMs: 0.05,
		MetricsGB: 0.3, LogsGB: 0.5, TracesGB: 0.3,
	}
	r.Kinds = append(r.Kinds,
		Kind{Name: KindMetrics, Label: "Metrics store", Capacity: 20_000, ServiceMs: 1, CostPerHour: 1.5, BuildCost: 50, Complexity: 1},
		Kind{Name: KindLogs, Label: "Log store", Capacity: 3_000, ServiceMs: 1, CostPerHour: 2, BuildCost: 60, Complexity: 1},
		Kind{Name: KindTraces, Label: "Trace backend", Capacity: 5_000, ServiceMs: 1, CostPerHour: 1.5, BuildCost: 50, Complexity: 1},
	)
	return r
}
