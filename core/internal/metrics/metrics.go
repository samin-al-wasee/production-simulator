// Package metrics translates physical measurements into the virtual view and
// renders both together (Dual Metrics Mode, ADR-0003). Virtual values are
// always labelled with the scale factor and never presented as hardware
// measurements.
package metrics

import (
	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/scale"
)

// Dual is the pair of views every infrastructure surface must expose.
type Dual struct {
	// ScaleFactor is always present, e.g. "x128".
	ScaleFactor string   `json:"scaleFactor"`
	Physical    Physical `json:"physical"`
	Virtual     Virtual  `json:"virtual"`
}

// Physical holds real host measurements.
type Physical struct {
	Kind   string           `json:"kind"`
	Used   budget.Resources `json:"used"`
	Budget budget.Resources `json:"budget"`
	RPS    float64          `json:"rps"`
}

// Virtual holds simulated production values derived from physical ones.
type Virtual struct {
	Kind     string           `json:"kind"`
	Used     budget.Resources `json:"used"`
	Capacity budget.Resources `json:"capacity"`
	RPS      float64          `json:"rps"`
	Nodes    int              `json:"nodes"`
}

// Kind labels for the two views.
const (
	KindPhysical = "physical-host-measurement"
	KindVirtual  = "virtual-production-simulated"
)

// Translate builds the dual view. physicalUsed is measured on the host;
// virtualCapacity and nodes describe the virtual cluster.
//
// Virtual usage applies each resource's own virtual/physical capacity ratio to
// its physical usage, so utilization is identical in both views and real
// bottlenecks are preserved. Multiplying every resource by the single
// effective factor would overstate resources whose ratio is smaller than the
// binding one. RPS uses the effective factor, the label shown to users.
func Translate(f scale.Factor, physicalBudget, physicalUsed budget.Resources, physicalRPS float64, virtualCapacity budget.Resources, nodes int) Dual {
	return Dual{
		ScaleFactor: f.String(),
		Physical:    Physical{Kind: KindPhysical, Used: physicalUsed, Budget: physicalBudget, RPS: physicalRPS},
		Virtual: Virtual{
			Kind: KindVirtual,
			Used: budget.Resources{
				CPUCores:    physicalUsed.CPUCores * f.CPU,
				MemoryBytes: uint64(float64(physicalUsed.MemoryBytes) * f.Memory),
				DiskBytes:   uint64(float64(physicalUsed.DiskBytes) * f.Disk),
			},
			Capacity: virtualCapacity,
			RPS:      f.ToVirtual(physicalRPS),
			Nodes:    nodes,
		},
	}
}
