package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/chaos"
)

func runChaos(args []string) int {
	if len(args) < 1 {
		chaosUsage()
		return exitUsage
	}
	switch args[0] {
	case "run":
		return chaosRun(args[1:])
	case "plan":
		return chaosPlan(args[1:])
	default:
		chaosUsage()
		return exitUsage
	}
}

func chaosUsage() {
	fmt.Fprintln(os.Stderr, "usage: forgelab chaos <run|plan> [-dry-run] <experiment-file>")
	fmt.Fprintln(os.Stderr, "  plan   print the inject and revert commands without running anything")
	fmt.Fprintln(os.Stderr, "  run    run the experiment: steady state, inject, verify, revert, verify recovery")
}

func chaosPlan(args []string) int {
	if len(args) != 1 {
		chaosUsage()
		return exitUsage
	}
	e, err := chaos.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	inject, revert, err := chaos.Plan(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	fmt.Printf("experiment: %s\nhypothesis: %s\ninject: %s\nrevert: %s\n", e.Metadata.Name, e.Spec.Hypothesis, strings.Join(inject, " "), strings.Join(revert, " "))
	return exitOK
}

func chaosRun(args []string) int {
	fs := flag.NewFlagSet("chaos run", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "print the plan and exit without touching anything")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		chaosUsage()
		return exitUsage
	}
	if *dry {
		return chaosPlan(fs.Args())
	}
	e, err := chaos.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rep, err := chaos.Run(ctx, e, chaos.Env{
		Exec:  chaos.DockerExecutor{},
		Probe: chaos.HTTPProber{},
		Sleep: time.Sleep,
		Log:   os.Stdout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	if !rep.Passed {
		fmt.Printf("\nexperiment %s FAILED (fault reverted: %v)\n", rep.Experiment, rep.Reverted)
		return exitFailed
	}
	fmt.Printf("\nexperiment %s PASSED: hypothesis held and the system recovered\n", rep.Experiment)
	return exitOK
}
