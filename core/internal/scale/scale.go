// Package scale computes the deterministic scale factor between the physical
// budget and the virtual cluster (ADR-0003). The factor only relabels
// capacity; it never changes system dynamics.
package scale

import (
	"fmt"
	"math"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
)

// Factor is the per-resource ratio of virtual to physical capacity plus the
// effective factor used to present the whole production view.
type Factor struct {
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Disk   float64 `json:"disk"`
	// Binding names the resource with the largest ratio.
	Binding string `json:"binding"`
	// Effective is the binding ratio rounded up to the next power of two
	// (x1, x2, x4, ... x64, x128), never below 1.
	Effective float64 `json:"effective"`
}

// Compute derives the factor from the allocatable physical budget and the
// total virtual capacity. Every allocatable resource must be positive.
func Compute(physical, virtual budget.Resources) (Factor, error) {
	if physical.CPUCores <= 0 || physical.MemoryBytes == 0 || physical.DiskBytes == 0 {
		return Factor{}, fmt.Errorf("physical budget must be positive in every resource: %+v", physical)
	}
	f := Factor{
		CPU:    virtual.CPUCores / physical.CPUCores,
		Memory: float64(virtual.MemoryBytes) / float64(physical.MemoryBytes),
		Disk:   float64(virtual.DiskBytes) / float64(physical.DiskBytes),
	}
	max, name := f.CPU, "cpu"
	if f.Memory > max {
		max, name = f.Memory, "memory"
	}
	if f.Disk > max {
		max, name = f.Disk, "disk"
	}
	f.Binding = name
	f.Effective = roundUpPow2(max)
	return f, nil
}

func roundUpPow2(v float64) float64 {
	if v <= 1 {
		return 1
	}
	return math.Exp2(math.Ceil(math.Log2(v)))
}

// ToVirtual converts a physical quantity (for example measured RPS) to its
// virtual presentation under the effective factor.
func (f Factor) ToVirtual(physical float64) float64 { return physical * f.Effective }

// ToPhysical converts a virtual quantity to the physical load that
// represents it.
func (f Factor) ToPhysical(virtual float64) float64 { return virtual / f.Effective }

// String renders the factor as it must always be labelled, e.g. "x128".
func (f Factor) String() string { return fmt.Sprintf("x%g", f.Effective) }
