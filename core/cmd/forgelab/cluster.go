package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/calibration"
	"github.com/samin-al-wasee/production-simulator/core/internal/metrics"
	"github.com/samin-al-wasee/production-simulator/core/internal/scale"
	"github.com/samin-al-wasee/production-simulator/core/internal/virtualcluster"
)

const gib = float64(1 << 30)

func runCluster(args []string) int {
	fs := flag.NewFlagSet("cluster", flag.ContinueOnError)
	reserve := fs.Float64("reserve", budget.DefaultReserveFraction, "fraction of each resource reserved for the host, in [0, 1)")
	diskPath := fs.String("disk-path", ".", "path whose filesystem capacity is reported")
	rps := fs.Float64("rps", 0, "measured physical requests per second, translated to virtual RPS")
	asJSON := fs.Bool("json", false, "print the dual metrics as JSON")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: forgelab cluster [-reserve <fraction>] [-disk-path <path>] [-rps <n>] [-json] <cluster-file>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitUsage
	}

	cluster, err := virtualcluster.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	host, err := calibration.Detect(*diskPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	b, err := budget.Compute(host, budget.UniformPolicy(*reserve))
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitUsage
	}
	factor, err := scale.Compute(b.Allocatable, cluster.Total())
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	used, err := calibration.Usage(host, *diskPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
		return exitFailed
	}
	dual := metrics.Translate(factor, b.Allocatable,
		budget.Resources{CPUCores: used.CPUCores, MemoryBytes: used.MemoryBytes, DiskBytes: used.DiskBytes},
		*rps, cluster.Total(), cluster.Nodes())

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Cluster string       `json:"cluster"`
			Factor  scale.Factor `json:"factor"`
			Metrics metrics.Dual `json:"metrics"`
		}{cluster.Name, factor, dual}); err != nil {
			fmt.Fprintf(os.Stderr, "forgelab: %v\n", err)
			return exitFailed
		}
		return exitOK
	}

	fmt.Printf("cluster %q: %d virtual nodes, scale factor %s (binding resource: %s)\n\n", cluster.Name, cluster.Nodes(), factor, factor.Binding)
	fmt.Printf("%-14s %22s %22s\n", "", "PHYSICAL (host)", "VIRTUAL ("+factor.String()+", simulated)")
	fmt.Printf("%-14s %10.2f / %-9.2f %10.2f / %-9.2f\n", "cpu (cores)", dual.Physical.Used.CPUCores, dual.Physical.Budget.CPUCores, dual.Virtual.Used.CPUCores, dual.Virtual.Capacity.CPUCores)
	fmt.Printf("%-14s %10.2f / %-9.2f %10.2f / %-9.2f\n", "memory (GiB)", float64(dual.Physical.Used.MemoryBytes)/gib, float64(dual.Physical.Budget.MemoryBytes)/gib, float64(dual.Virtual.Used.MemoryBytes)/gib, float64(dual.Virtual.Capacity.MemoryBytes)/gib)
	fmt.Printf("%-14s %10.2f / %-9.2f %10.2f / %-9.2f\n", "disk (GiB)", float64(dual.Physical.Used.DiskBytes)/gib, float64(dual.Physical.Budget.DiskBytes)/gib, float64(dual.Virtual.Used.DiskBytes)/gib, float64(dual.Virtual.Capacity.DiskBytes)/gib)
	fmt.Printf("%-14s %22.0f %22.0f\n", "rps", dual.Physical.RPS, dual.Virtual.RPS)
	fmt.Println("\nvirtual values are simulated capacity, not hardware measurements")
	return exitOK
}
