package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/calibration"
	"github.com/samin-al-wasee/production-simulator/core/internal/learning"
	"github.com/samin-al-wasee/production-simulator/core/internal/loadtest"
	"github.com/samin-al-wasee/production-simulator/core/internal/report"
	"github.com/samin-al-wasee/production-simulator/core/internal/scale"
	"github.com/samin-al-wasee/production-simulator/core/internal/virtualcluster"
)

func runBenchmark(args []string) int {
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	url := fs.String("url", "", "target URL (required)")
	rps := fs.Float64("rps", 50, "baseline requests per second")
	peak := fs.Float64("peak", 0, "ramp end and spike peak rate (default 4x -rps)")
	stepDuration := fs.Duration("step-duration", 10*time.Second, "duration of each of the three runs (baseline, ramp, spike)")
	maxInFlight := fs.Int("max-in-flight", 256, "maximum concurrent requests")
	cluster := fs.String("cluster", "", "virtual cluster file for the scale factor (default: manifests/cluster.example.yaml when present)")
	p95 := fs.Duration("slo-p95", 250*time.Millisecond, "p95 latency SLO (0 to skip)")
	errRate := fs.Float64("slo-error-rate", 0.01, "maximum error rate as a fraction (0 to skip)")
	dropRate := fs.Float64("slo-drop-rate", 0.05, "maximum dropped-request rate as a fraction (0 to skip)")
	outDir := fs.String("out", "", "report directory (default: reports/ at the repository root)")
	noRecord := fs.Bool("no-record", false, "do not record learning-path progress")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab benchmark -url <url> [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if *url == "" || fs.NArg() != 0 {
		fs.Usage()
		return exitUsage
	}
	if *peak <= 0 {
		*peak = *rps * 4
	}
	root := findRepoRoot()

	env := detectEnvironment(root, *cluster)
	scaleFactor := 1.0
	if env != nil && env.Scale >= 1 {
		scaleFactor = env.Scale
	}
	slo := report.SLO{P95: *p95, MaxErrorRate: *errRate, MaxDropRate: *dropRate}

	profiles := []struct {
		name string
		cfg  loadtest.Config
	}{
		{"baseline", loadtest.Config{Profile: loadtest.Constant, RPS: *rps}},
		{"ramp", loadtest.Config{Profile: loadtest.Ramp, RPS: *rps, RampTo: *peak}},
		{"spike", loadtest.Config{Profile: loadtest.Spike, RPS: *rps, RampTo: *peak}},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	bench := report.Benchmark{Generated: time.Now(), Target: *url, Environment: env, SLO: slo}
	for _, p := range profiles {
		p.cfg.URL, p.cfg.Duration, p.cfg.MaxInFlight = *url, *stepDuration, *maxInFlight
		fmt.Printf("running %s (%s, %s)...\n", p.name, p.cfg.Profile, p.cfg.Duration)
		res, err := loadtest.Run(ctx, p.cfg, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitUsage
		}
		run := report.NewRun(p.name, p.cfg, res, slo, scaleFactor)
		bench.Runs = append(bench.Runs, run)
		fmt.Printf("  sent %d, dropped %d, errors %d, p95 %s, %.1f rps\n", res.Sent, res.Dropped, res.Errors, res.Latency.P95.Round(time.Microsecond), res.AchievedRPS)
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "forgelab: interrupted; no report written")
			return exitFailed
		}
	}
	bench.Finish()

	dir := *outDir
	if dir == "" {
		if root == "" {
			dir = "reports"
		} else {
			dir = filepath.Join(root, "reports")
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	stamp := bench.Generated.UTC().Format("20060102T150405Z")
	mdPath := filepath.Join(dir, "benchmark-"+stamp+".md")
	jsonPath := filepath.Join(dir, "benchmark-"+stamp+".json")
	raw, _ := json.MarshalIndent(bench, "", "  ")
	if err := os.WriteFile(mdPath, []byte(bench.Markdown()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	fmt.Printf("\nreport: %s\n        %s\n", mdPath, jsonPath)

	if !bench.Passed {
		fmt.Println("benchmark: FAILED (an SLO was not met)")
		return exitFailed
	}
	fmt.Println("benchmark: PASSED")
	if !*noRecord {
		recordEvidence(learning.EvidenceBenchmark, "")
	}
	return exitOK
}

// detectEnvironment describes the host and, when a cluster file is available,
// the simulated production. It returns nil when the host cannot be detected.
func detectEnvironment(root, clusterFile string) *report.Environment {
	host, err := calibration.Detect(".")
	if err != nil {
		return nil
	}
	b, err := budget.Compute(host, budget.UniformPolicy(budget.DefaultReserveFraction))
	if err != nil {
		return nil
	}
	env := &report.Environment{Host: b.Host, Allocatable: b.Allocatable}
	if clusterFile == "" && root != "" {
		if candidate := filepath.Join(root, "manifests", "cluster.example.yaml"); fileExists(candidate) {
			clusterFile = candidate
		}
	}
	if clusterFile == "" {
		return env
	}
	c, err := virtualcluster.Load(clusterFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: cluster not used: %v\n", err)
		return env
	}
	f, err := scale.Compute(b.Allocatable, c.Total())
	if err != nil {
		return env
	}
	env.Cluster, env.Nodes, env.ScaleFactor, env.Scale = c.Name, c.Nodes(), f.String(), f.Effective
	return env
}
