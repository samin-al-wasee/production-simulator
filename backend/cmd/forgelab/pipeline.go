package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/samin-al-wasee/production-simulator/backend/internal/pipeline"
)

func runPipeline(args []string) int {
	if len(args) < 1 || args[0] != "run" {
		fmt.Fprintln(os.Stderr, "usage: forgelab pipeline run [-seed N] [-warm-cache] [-fail-step NAME] [-bad-release] [-json] <pipeline-file>")
		return exitUsage
	}
	fs := flag.NewFlagSet("pipeline run", flag.ContinueOnError)
	var opt pipeline.Options
	fs.Int64Var(&opt.Seed, "seed", 1, "seed for flaky steps (same seed, same run)")
	fs.BoolVar(&opt.WarmCache, "warm-cache", false, "use cacheHitDuration where declared")
	fs.StringVar(&opt.FailStep, "fail-step", "", "force the named step to fail")
	fs.BoolVar(&opt.BadRelease, "bad-release", false, "make the deployed version unhealthy so deploy analysis fails")
	asJSON := fs.Bool("json", false, "print the run as JSON")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab pipeline run [flags] <pipeline-file>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}
	p, err := pipeline.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	run, err := pipeline.Simulate(p, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(run); err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
	} else {
		for _, e := range run.Events {
			fmt.Printf("%8s  %-8s %s\n", e.At.Round(time.Second), e.Stage, e.Message)
		}
		fmt.Printf("\npipeline %s: %s in %s (virtual time)\n", run.Pipeline, run.Status, run.Duration.Round(time.Second))
	}
	if run.Status != pipeline.StatusSucceeded {
		return exitFailed
	}
	return exitOK
}
