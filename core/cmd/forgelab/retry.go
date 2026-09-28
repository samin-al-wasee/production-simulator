package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/retry"
)

func runRetry(args []string) int {
	fs := flag.NewFlagSet("retry", flag.ContinueOnError)
	p := retry.Policy{}
	fs.IntVar(&p.MaxAttempts, "max-attempts", 5, "total delivery attempts including the first")
	fs.DurationVar(&p.BaseDelay, "base", time.Second, "delay before the first retry")
	fs.Float64Var(&p.Factor, "factor", 2, "backoff multiplier (>= 1)")
	fs.DurationVar(&p.MaxDelay, "max-delay", 30*time.Second, "delay cap (0 = none)")
	failures := fs.Int("failures", 100, "number of initial attempts that fail")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab retry [-max-attempts N] [-base D] [-factor F] [-max-delay D] [-failures N]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return exitUsage
	}
	out, err := retry.Simulate(p, *failures)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitUsage
	}
	fmt.Printf("%-10s %-12s %s\n", "attempt", "if it fails", "wait")
	for a := 1; a <= out.Attempts; a++ {
		if a > *failures {
			fmt.Printf("%-10d %-12s -\n", a, "succeeds")
			break
		}
		d := p.Decide(a)
		wait := d.Delay.String()
		if d.Action == retry.DeadLetter {
			wait = "-"
		}
		fmt.Printf("%-10d %-12s %s\n", a, d.Action, wait)
	}
	if out.DeadLettered {
		fmt.Printf("\ndead-lettered after %d attempts (%s of backoff)\n", out.Attempts, out.Total)
	} else {
		fmt.Printf("\nprocessed on attempt %d (%s of backoff)\n", out.Attempts, out.Total)
	}
	return exitOK
}
