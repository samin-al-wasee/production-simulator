package report

import (
	"strings"
	"testing"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/loadtest"
)

func result(p95 time.Duration, sent, errs, dropped int) loadtest.Result {
	return loadtest.Result{
		Sent: sent, Errors: errs, Dropped: dropped, Statuses: map[int]int{200: sent - errs},
		AchievedRPS: 100, Latency: loadtest.Latency{Min: time.Millisecond, Mean: 2 * time.Millisecond, P50: 2 * time.Millisecond, P90: 3 * time.Millisecond, P95: p95, P99: 2 * p95, Max: 3 * p95},
	}
}

func TestEvaluate(t *testing.T) {
	slo := SLO{P95: 100 * time.Millisecond, MaxErrorRate: 0.01, MaxDropRate: 0.05}
	good := Evaluate(slo, result(50*time.Millisecond, 1000, 5, 10))
	if len(good) != 3 {
		t.Fatalf("verdicts = %+v", good)
	}
	for _, v := range good {
		if !v.Passed {
			t.Errorf("expected pass: %+v", v)
		}
	}
	bad := Evaluate(slo, result(200*time.Millisecond, 1000, 50, 500))
	for _, v := range bad {
		if v.Passed {
			t.Errorf("expected fail: %+v", v)
		}
	}
	if len(Evaluate(SLO{}, result(time.Second, 10, 5, 5))) != 0 {
		t.Error("zero SLO must not check anything")
	}
}

func TestNewRunScalesAndAggregates(t *testing.T) {
	cfg := loadtest.Config{Profile: loadtest.Ramp, RPS: 10, RampTo: 50, Duration: 5 * time.Second}
	run := NewRun("ramp", cfg, result(50*time.Millisecond, 100, 0, 0), SLO{P95: time.Second}, 128)
	if run.VirtualRPS != 12800 || run.PeakRPS != 50 || !run.Passed {
		t.Fatalf("run = %+v", run)
	}
	flat := NewRun("baseline", loadtest.Config{Profile: loadtest.Constant, RPS: 10, RampTo: 99}, result(time.Millisecond, 10, 0, 0), SLO{}, 0)
	if flat.PeakRPS != 0 || flat.VirtualRPS != 100 {
		t.Fatalf("constant run must ignore RampTo and treat scale < 1 as 1: %+v", flat)
	}
}

func TestFinish(t *testing.T) {
	var b Benchmark
	b.Finish()
	if b.Passed {
		t.Fatal("an empty benchmark must not pass")
	}
	b.Runs = []Run{{Passed: true}, {Passed: true}}
	b.Finish()
	if !b.Passed {
		t.Fatal("all runs passed")
	}
	b.Runs = append(b.Runs, Run{Passed: false})
	b.Finish()
	if b.Passed {
		t.Fatal("one failed run must fail the benchmark")
	}
}

func TestMarkdownIsDeterministicAndLabelled(t *testing.T) {
	slo := SLO{P95: 100 * time.Millisecond, MaxErrorRate: 0.01}
	b := Benchmark{
		Generated: time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
		Target:    "http://localhost:8080/api/work",
		SLO:       slo,
		Environment: &Environment{
			Host:        budget.Resources{CPUCores: 8, MemoryBytes: 16 << 30, DiskBytes: 500 << 30},
			Allocatable: budget.Resources{CPUCores: 6, MemoryBytes: 12 << 30, DiskBytes: 375 << 30},
			Cluster:     "prod-eu", Nodes: 263, ScaleFactor: "x4096", Scale: 4096,
		},
		Runs: []Run{
			NewRun("baseline", loadtest.Config{Profile: loadtest.Constant, RPS: 100, Duration: 10 * time.Second}, result(50*time.Millisecond, 1000, 0, 0), slo, 4096),
			NewRun("spike", loadtest.Config{Profile: loadtest.Spike, RPS: 100, RampTo: 400, Duration: 10 * time.Second}, result(300*time.Millisecond, 1000, 30, 0), slo, 4096),
		},
	}
	b.Finish()
	md := b.Markdown()
	if md != b.Markdown() {
		t.Fatal("rendering is not deterministic")
	}
	for _, want := range []string{
		"**Result:** FAILED", "2026-09-28T12:00:00Z", "scale factor **x4096**", "not hardware measurements",
		"| baseline | constant |", "| spike | spike |", "**FAIL**", "100 rps to 400 rps", "p95 <= 100ms",
		"| 200 | 1000 |", "Virtual RPS is physical RPS multiplied by the scale factor",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n%s", want, md)
		}
	}
	if !strings.Contains(Benchmark{Passed: true, Runs: []Run{{}}}.Markdown(), "**Result:** PASSED") {
		t.Error("passed result not rendered")
	}
	if !strings.Contains(Benchmark{}.Markdown(), "none (informational)") {
		t.Error("empty SLO description")
	}
}
