package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/samin-al-wasee/production-simulator/core/internal/costguard"
)

const defaultCostGuardRules = "environments/cloud/cost-guard.yaml"

func runCostGuard(args []string) int {
	if len(args) < 1 || args[0] != "check" {
		fmt.Fprintln(os.Stderr, "usage: forgelab costguard check [-rules <file>] <plan.json>")
		fmt.Fprintln(os.Stderr, "  plan.json is the output of: terraform show -json <planfile>")
		return exitUsage
	}
	fs := flag.NewFlagSet("costguard check", flag.ContinueOnError)
	rulesPath := fs.String("rules", defaultCostGuardRules, "cost-guard rules file")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab costguard check [-rules <file>] <plan.json>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args[1:]); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}

	rules, err := costguard.LoadRules(*rulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	raw, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: read plan: %v\n", err)
		return exitFailed
	}
	rep, err := costguard.Evaluate(rules, raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}

	fmt.Printf("%-52s %10s  %s\n", "resource", "USD/month", "basis")
	for _, l := range rep.Lines {
		fmt.Printf("%-52s %10.2f  %s\n", l.Address, l.MonthlyUSD, l.Note)
	}
	fmt.Printf("%-52s %10.2f  ceiling %.2f\n", "estimated total", rep.MonthlyUSD, rules.Spec.MaxMonthlyUSD)
	for _, w := range rep.Warnings {
		fmt.Printf("warning [%s] %s: %s\n", w.Rule, w.Address, w.Message)
	}
	if !rep.OK() {
		for _, v := range rep.Violations {
			if v.Address != "" {
				fmt.Printf("VIOLATION [%s] %s: %s\n", v.Rule, v.Address, v.Message)
			} else {
				fmt.Printf("VIOLATION [%s] %s\n", v.Rule, v.Message)
			}
		}
		fmt.Println("\ncost guard: plan REJECTED; do not apply")
		return exitFailed
	}
	fmt.Println("\ncost guard: plan accepted (estimate only; review the plan before applying)")
	return exitOK
}
