package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/loadtest"
)

func runLoadtest(args []string) int {
	fs := flag.NewFlagSet("loadtest", flag.ContinueOnError)
	cfg := loadtest.Config{}
	profile := fs.String("profile", string(loadtest.Constant), "constant, ramp, or spike")
	fs.StringVar(&cfg.URL, "url", "", "target URL (required)")
	fs.Float64Var(&cfg.RPS, "rps", 50, "requests per second (start rate for ramp, base rate for spike)")
	fs.Float64Var(&cfg.RampTo, "ramp-to", 0, "end rate for ramp, peak rate for spike")
	fs.DurationVar(&cfg.Duration, "duration", 10*time.Second, "test duration")
	fs.IntVar(&cfg.MaxInFlight, "max-in-flight", 256, "maximum concurrent requests; excess requests are counted as dropped")
	fs.DurationVar(&cfg.Timeout, "timeout", 10*time.Second, "per-request timeout")
	scale := fs.Float64("scale", 1, "scale factor for the virtual RPS column (see forgelab cluster)")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab loadtest -url <url> [-rps N] [-profile constant|ramp|spike] [-ramp-to N] [-duration D] [-scale F] [-json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return exitUsage
	}
	if *scale < 1 {
		fmt.Fprintln(os.Stderr, "forgelab: scale must be >= 1")
		return exitUsage
	}
	cfg.Profile = loadtest.Profile(*profile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := loadtest.Run(ctx, cfg, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitUsage
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			loadtest.Result
			ScaleFactor string  `json:"scaleFactor"`
			VirtualRPS  float64 `json:"virtualRps"`
			ErrorRate   float64 `json:"errorRate"`
		}{res, fmt.Sprintf("x%g", *scale), res.AchievedRPS * *scale, res.ErrorRate()}); err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
		return exitOK
	}

	fmt.Printf("target      %s (%s, %s)\n", cfg.URL, cfg.Profile, cfg.Duration)
	fmt.Printf("sent        %d (dropped %d, transport errors %d)\n", res.Sent, res.Dropped, res.Errors)
	fmt.Printf("rps         %.1f physical, %.1f virtual (x%g, simulated)\n", res.AchievedRPS, res.AchievedRPS**scale, *scale)
	codes := make([]int, 0, len(res.Statuses))
	for c := range res.Statuses {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	for _, c := range codes {
		fmt.Printf("status %d  %d\n", c, res.Statuses[c])
	}
	fmt.Printf("error rate  %.2f%%\n", res.ErrorRate()*100)
	l := res.Latency
	fmt.Printf("latency     min %s  mean %s  p50 %s  p90 %s  p95 %s  p99 %s  max %s\n", l.Min.Round(time.Microsecond), l.Mean.Round(time.Microsecond), l.P50.Round(time.Microsecond), l.P90.Round(time.Microsecond), l.P95.Round(time.Microsecond), l.P99.Round(time.Microsecond), l.Max.Round(time.Microsecond))
	return exitOK
}
