// Package report turns benchmark results into a reviewable report. It only
// formats: measuring is done by internal/loadtest, capacity by the
// simulation packages. Rendering is deterministic for the same input,
// including the timestamp, which is supplied by the caller.
package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/loadtest"
)

// SLO is the pass/fail threshold set applied to every run. A zero field is
// not checked.
type SLO struct {
	P95          time.Duration `json:"p95"`
	MaxErrorRate float64       `json:"maxErrorRate"`
	MaxDropRate  float64       `json:"maxDropRate"`
}

// Verdict is one SLO check of one run.
type Verdict struct {
	Check  string `json:"check"`
	Want   string `json:"want"`
	Got    string `json:"got"`
	Passed bool   `json:"passed"`
}

// Evaluate checks a result against the SLO.
func Evaluate(slo SLO, r loadtest.Result) []Verdict {
	var v []Verdict
	if slo.P95 > 0 {
		v = append(v, Verdict{"p95 latency", "<= " + slo.P95.String(), r.Latency.P95.Round(time.Microsecond).String(), r.Latency.P95 <= slo.P95})
	}
	if slo.MaxErrorRate > 0 {
		rate := r.ErrorRate()
		v = append(v, Verdict{"error rate", fmt.Sprintf("<= %.2f%%", slo.MaxErrorRate*100), fmt.Sprintf("%.2f%%", rate*100), rate <= slo.MaxErrorRate})
	}
	if slo.MaxDropRate > 0 {
		total := r.Sent + r.Dropped
		rate := 0.0
		if total > 0 {
			rate = float64(r.Dropped) / float64(total)
		}
		v = append(v, Verdict{"dropped requests", fmt.Sprintf("<= %.2f%%", slo.MaxDropRate*100), fmt.Sprintf("%.2f%%", rate*100), rate <= slo.MaxDropRate})
	}
	return v
}

// Environment describes the machine and simulated production behind a run.
type Environment struct {
	Host        budget.Resources `json:"host"`
	Allocatable budget.Resources `json:"allocatable"`
	Cluster     string           `json:"cluster,omitempty"`
	Nodes       int              `json:"nodes,omitempty"`
	ScaleFactor string           `json:"scaleFactor,omitempty"`
	// Scale is the numeric effective factor used for virtual RPS.
	Scale float64 `json:"scale,omitempty"`
}

// Run is one benchmark profile and its outcome.
type Run struct {
	Name       string          `json:"name"`
	Profile    string          `json:"profile"`
	TargetRPS  float64         `json:"targetRps"`
	PeakRPS    float64         `json:"peakRps,omitempty"`
	Duration   time.Duration   `json:"duration"`
	Result     loadtest.Result `json:"result"`
	VirtualRPS float64         `json:"virtualRps"`
	Verdicts   []Verdict       `json:"verdicts"`
	Passed     bool            `json:"passed"`
}

// Benchmark is a complete report.
type Benchmark struct {
	Generated   time.Time    `json:"generated"`
	Target      string       `json:"target"`
	Environment *Environment `json:"environment,omitempty"`
	SLO         SLO          `json:"slo"`
	Runs        []Run        `json:"runs"`
	Passed      bool         `json:"passed"`
}

// NewRun builds a Run from a result, applying the SLO and the scale factor
// (1 when no cluster is simulated).
func NewRun(name string, cfg loadtest.Config, res loadtest.Result, slo SLO, scale float64) Run {
	if scale < 1 {
		scale = 1
	}
	verdicts := Evaluate(slo, res)
	passed := true
	for _, v := range verdicts {
		passed = passed && v.Passed
	}
	peak := 0.0
	if cfg.Profile != loadtest.Constant {
		peak = cfg.RampTo
	}
	return Run{
		Name: name, Profile: string(cfg.Profile), TargetRPS: cfg.RPS, PeakRPS: peak, Duration: cfg.Duration,
		Result: res, VirtualRPS: res.AchievedRPS * scale, Verdicts: verdicts, Passed: passed,
	}
}

// Finish sets the overall verdict.
func (b *Benchmark) Finish() {
	b.Passed = len(b.Runs) > 0
	for _, r := range b.Runs {
		b.Passed = b.Passed && r.Passed
	}
}

func ms(d time.Duration) string {
	return fmt.Sprintf("%.2f ms", float64(d)/float64(time.Millisecond))
}

const gib = float64(1 << 30)

// Markdown renders the report.
func (b Benchmark) Markdown() string {
	var w strings.Builder
	verdict := "PASSED"
	if !b.Passed {
		verdict = "FAILED"
	}
	fmt.Fprintf(&w, "# ForgeLab benchmark report\n\n")
	fmt.Fprintf(&w, "- **Result:** %s\n", verdict)
	fmt.Fprintf(&w, "- **Generated:** %s\n", b.Generated.UTC().Format(time.RFC3339))
	fmt.Fprintf(&w, "- **Target:** `%s`\n", b.Target)
	fmt.Fprintf(&w, "- **SLO:** %s\n\n", b.SLO.describe())

	if e := b.Environment; e != nil {
		w.WriteString("## Environment\n\n")
		w.WriteString("| | Host | Allocatable (physical budget) |\n|---|---:|---:|\n")
		fmt.Fprintf(&w, "| CPU cores | %.2f | %.2f |\n", e.Host.CPUCores, e.Allocatable.CPUCores)
		fmt.Fprintf(&w, "| Memory | %.2f GiB | %.2f GiB |\n", float64(e.Host.MemoryBytes)/gib, float64(e.Allocatable.MemoryBytes)/gib)
		fmt.Fprintf(&w, "| Disk | %.2f GiB | %.2f GiB |\n\n", float64(e.Host.DiskBytes)/gib, float64(e.Allocatable.DiskBytes)/gib)
		if e.ScaleFactor != "" {
			fmt.Fprintf(&w, "Simulated production: cluster `%s`, %d virtual nodes, scale factor **%s**. Virtual figures below are simulated capacity derived from physical measurements; they are not hardware measurements.\n\n", e.Cluster, e.Nodes, e.ScaleFactor)
		}
	}

	w.WriteString("## Summary\n\n")
	w.WriteString("| Run | Profile | Sent | Dropped | Errors | Error rate | p50 | p95 | p99 | Physical RPS | Virtual RPS | Verdict |\n")
	w.WriteString("|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, r := range b.Runs {
		v := "pass"
		if !r.Passed {
			v = "**FAIL**"
		}
		fmt.Fprintf(&w, "| %s | %s | %d | %d | %d | %.2f%% | %s | %s | %s | %.1f | %.1f | %s |\n",
			r.Name, r.Profile, r.Result.Sent, r.Result.Dropped, r.Result.Errors, r.Result.ErrorRate()*100,
			ms(r.Result.Latency.P50), ms(r.Result.Latency.P95), ms(r.Result.Latency.P99), r.Result.AchievedRPS, r.VirtualRPS, v)
	}
	w.WriteString("\n")

	for _, r := range b.Runs {
		fmt.Fprintf(&w, "## %s\n\n", r.Name)
		rate := fmt.Sprintf("%g rps", r.TargetRPS)
		if r.PeakRPS > 0 {
			rate += fmt.Sprintf(" to %g rps", r.PeakRPS)
		}
		fmt.Fprintf(&w, "Profile `%s`, %s for %s.\n\n", r.Profile, rate, r.Duration)
		codes := make([]int, 0, len(r.Result.Statuses))
		for c := range r.Result.Statuses {
			codes = append(codes, c)
		}
		sort.Ints(codes)
		if len(codes) > 0 {
			w.WriteString("| Status | Count |\n|---|---:|\n")
			for _, c := range codes {
				fmt.Fprintf(&w, "| %d | %d |\n", c, r.Result.Statuses[c])
			}
			w.WriteString("\n")
		}
		l := r.Result.Latency
		fmt.Fprintf(&w, "Latency: min %s, mean %s, p50 %s, p90 %s, p95 %s, p99 %s, max %s.\n\n", ms(l.Min), ms(l.Mean), ms(l.P50), ms(l.P90), ms(l.P95), ms(l.P99), ms(l.Max))
		if len(r.Verdicts) > 0 {
			w.WriteString("| SLO check | Want | Got | Result |\n|---|---|---|---|\n")
			for _, v := range r.Verdicts {
				res := "pass"
				if !v.Passed {
					res = "**FAIL**"
				}
				fmt.Fprintf(&w, "| %s | %s | %s | %s |\n", v.Check, v.Want, v.Got, res)
			}
			w.WriteString("\n")
		}
	}

	w.WriteString("## Reading this report\n\n")
	w.WriteString("- Latency percentiles use the nearest-rank method over completed requests; the load generator is open-loop, so slow responses do not reduce offered load. Requests skipped because the in-flight limit was reached are reported as dropped.\n")
	w.WriteString("- Virtual RPS is physical RPS multiplied by the scale factor (Dual Metrics Mode) and is a capacity model, not a measurement.\n")
	return w.String()
}

func (s SLO) describe() string {
	var parts []string
	if s.P95 > 0 {
		parts = append(parts, "p95 <= "+s.P95.String())
	}
	if s.MaxErrorRate > 0 {
		parts = append(parts, fmt.Sprintf("error rate <= %.2f%%", s.MaxErrorRate*100))
	}
	if s.MaxDropRate > 0 {
		parts = append(parts, fmt.Sprintf("drops <= %.2f%%", s.MaxDropRate*100))
	}
	if len(parts) == 0 {
		return "none (informational)"
	}
	return strings.Join(parts, ", ")
}
