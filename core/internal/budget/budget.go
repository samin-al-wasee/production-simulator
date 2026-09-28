// Package budget computes the physical resource budget ForgeLab may spend on
// real containers and processes: the calibrated host minus a reserve kept for
// the host itself. It is the Physical Resource Budget stage of the Resource
// Virtualization Engine (ADR-0003) and is pure and deterministic.
package budget

import (
	"errors"
	"fmt"

	"github.com/samin-al-wasee/production-simulator/core/internal/calibration"
)

// DefaultReserveFraction is the share of each resource kept for the host.
const DefaultReserveFraction = 0.25

// ErrExceeded is returned when demands do not fit in the budget.
var ErrExceeded = errors.New("physical budget exceeded")

// Resources is an amount of physical CPU, memory, and disk.
type Resources struct {
	CPUCores    float64 `json:"cpuCores"`
	MemoryBytes uint64  `json:"memoryBytes"`
	DiskBytes   uint64  `json:"diskBytes"`
}

// Policy controls how much of the host is held back. Each fraction is in
// [0, 1).
type Policy struct {
	ReserveCPU    float64
	ReserveMemory float64
	ReserveDisk   float64
}

// UniformPolicy reserves the same fraction of every resource.
func UniformPolicy(fraction float64) Policy {
	return Policy{ReserveCPU: fraction, ReserveMemory: fraction, ReserveDisk: fraction}
}

// Validate reports whether every fraction is in [0, 1).
func (p Policy) Validate() error {
	for name, f := range map[string]float64{"cpu": p.ReserveCPU, "memory": p.ReserveMemory, "disk": p.ReserveDisk} {
		if f < 0 || f >= 1 {
			return fmt.Errorf("reserve fraction for %s must be in [0, 1), got %v", name, f)
		}
	}
	return nil
}

// Budget splits a host into the reserve kept for the host and the
// allocatable amount available to ForgeLab workloads.
type Budget struct {
	Host        Resources `json:"host"`
	Reserved    Resources `json:"reserved"`
	Allocatable Resources `json:"allocatable"`
}

// Compute derives the budget for host under policy.
func Compute(host calibration.Host, p Policy) (Budget, error) {
	if err := p.Validate(); err != nil {
		return Budget{}, err
	}
	h := Resources{CPUCores: host.CPUCores, MemoryBytes: host.MemoryBytes, DiskBytes: host.DiskBytes}
	r := Resources{
		CPUCores:    h.CPUCores * p.ReserveCPU,
		MemoryBytes: uint64(float64(h.MemoryBytes) * p.ReserveMemory),
		DiskBytes:   uint64(float64(h.DiskBytes) * p.ReserveDisk),
	}
	return Budget{
		Host:     h,
		Reserved: r,
		Allocatable: Resources{
			CPUCores:    h.CPUCores - r.CPUCores,
			MemoryBytes: h.MemoryBytes - r.MemoryBytes,
			DiskBytes:   h.DiskBytes - r.DiskBytes,
		},
	}, nil
}

// Remaining subtracts demands from the allocatable amount. It returns
// ErrExceeded, wrapped with the offending resource, if any resource would go
// negative.
func (b Budget) Remaining(demands ...Resources) (Resources, error) {
	left := b.Allocatable
	for _, d := range demands {
		if d.CPUCores > left.CPUCores {
			return Resources{}, fmt.Errorf("%w: cpu demand %.2f cores exceeds %.2f remaining", ErrExceeded, d.CPUCores, left.CPUCores)
		}
		if d.MemoryBytes > left.MemoryBytes {
			return Resources{}, fmt.Errorf("%w: memory demand %d bytes exceeds %d remaining", ErrExceeded, d.MemoryBytes, left.MemoryBytes)
		}
		if d.DiskBytes > left.DiskBytes {
			return Resources{}, fmt.Errorf("%w: disk demand %d bytes exceeds %d remaining", ErrExceeded, d.DiskBytes, left.DiskBytes)
		}
		left.CPUCores -= d.CPUCores
		left.MemoryBytes -= d.MemoryBytes
		left.DiskBytes -= d.DiskBytes
	}
	return left, nil
}
