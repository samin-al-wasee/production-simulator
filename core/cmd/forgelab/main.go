// Command forgelab is the ForgeLab core CLI. It is the thin headless
// interface to ForgeLab's simulation logic; the dashboard never reimplements
// what lives here.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/calibration"
	"github.com/samin-al-wasee/production-simulator/core/internal/manifest"
)

const (
	exitOK          = 0
	exitBadManifest = 1
	exitFailed      = 1
	exitHostFailed  = exitFailed
	exitUsage       = 2
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
	switch os.Args[1] {
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "host":
		os.Exit(runHost(os.Args[2:]))
	case "cluster":
		os.Exit(runCluster(os.Args[2:]))
	case "retry":
		os.Exit(runRetry(os.Args[2:]))
	case "chaos":
		os.Exit(runChaos(os.Args[2:]))
	case "loadtest":
		os.Exit(runLoadtest(os.Args[2:]))
	case "costguard":
		os.Exit(runCostGuard(os.Args[2:]))
	case "pipeline":
		os.Exit(runPipeline(os.Args[2:]))
	case "serve":
		os.Exit(runServe(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "forgelab: unknown command %q\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: forgelab <command> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  validate   validate an application manifest against the JSON Schema")
	fmt.Fprintln(w, "  host       detect host resources and show the physical resource budget")
	fmt.Fprintln(w, "  cluster    show a virtual cluster, its scale factor, and dual physical/virtual metrics")
	fmt.Fprintln(w, "  retry      simulate retry backoff and dead-letter semantics for a message")
	fmt.Fprintln(w, "  chaos      plan or run a fault-injection experiment against a ForgeLab container")
	fmt.Fprintln(w, "  loadtest   generate constant, ramp, or spike HTTP load and report latency percentiles")
	fmt.Fprintln(w, "  costguard  check a Terraform plan (JSON) against the cost-guard rules")
	fmt.Fprintln(w, "  pipeline   simulate a build, test, and deploy pipeline on a virtual clock")
	fmt.Fprintln(w, "  serve      serve the core over HTTP for the dashboard")
}

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	schemaPath := fs.String("schema", manifest.DefaultSchemaPath, "path to the application JSON Schema")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab validate [-schema <path>] <manifest-file>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}
	path := fs.Arg(0)

	validator, err := manifest.NewValidator(*schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitBadManifest
	}

	issues, err := validator.ValidateFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitBadManifest
	}
	if len(issues) > 0 {
		fmt.Fprintln(os.Stderr, "invalid application manifest:")
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  - %s\n", issue)
		}
		return exitBadManifest
	}

	fmt.Printf("valid application manifest: %s\n", path)
	return exitOK
}

func runHost(args []string) int {
	fs := flag.NewFlagSet("host", flag.ContinueOnError)
	reserve := fs.Float64("reserve", budget.DefaultReserveFraction, "fraction of each resource reserved for the host, in [0, 1)")
	diskPath := fs.String("disk-path", ".", "path whose filesystem capacity is reported")
	asJSON := fs.Bool("json", false, "print the budget as JSON")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab host [-reserve <fraction>] [-disk-path <path>] [-json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return exitUsage
	}

	host, err := calibration.Detect(*diskPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitHostFailed
	}
	b, err := budget.Compute(host, budget.UniformPolicy(*reserve))
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitUsage
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(b); err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitHostFailed
		}
		return exitOK
	}

	fmt.Println("physical resources (host, not virtual)")
	fmt.Printf("%-12s %10s %10s %12s\n", "", "host", "reserved", "allocatable")
	fmt.Printf("%-12s %10.2f %10.2f %12.2f\n", "cpu (cores)", b.Host.CPUCores, b.Reserved.CPUCores, b.Allocatable.CPUCores)
	fmt.Printf("%-12s %10.2f %10.2f %12.2f\n", "memory (GiB)", float64(b.Host.MemoryBytes)/gib, float64(b.Reserved.MemoryBytes)/gib, float64(b.Allocatable.MemoryBytes)/gib)
	fmt.Printf("%-12s %10.2f %10.2f %12.2f\n", "disk (GiB)", float64(b.Host.DiskBytes)/gib, float64(b.Reserved.DiskBytes)/gib, float64(b.Allocatable.DiskBytes)/gib)
	return exitOK
}
