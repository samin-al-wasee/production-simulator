package metrics

import (
	"testing"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/scale"
)

func TestTranslateLabelsAndScales(t *testing.T) {
	f := scale.Factor{CPU: 128, Memory: 128, Disk: 128, Effective: 128}
	physBudget := budget.Resources{CPUCores: 6, MemoryBytes: 12, DiskBytes: 100}
	used := budget.Resources{CPUCores: 2, MemoryBytes: 4, DiskBytes: 10}
	virtCap := budget.Resources{CPUCores: 768, MemoryBytes: 1536, DiskBytes: 12800}

	d := Translate(f, physBudget, used, 1000, virtCap, 96)

	if d.ScaleFactor != "x128" {
		t.Errorf("scale factor = %q", d.ScaleFactor)
	}
	if d.Physical.Kind != KindPhysical || d.Virtual.Kind != KindVirtual {
		t.Errorf("kinds = %q %q", d.Physical.Kind, d.Virtual.Kind)
	}
	if d.Physical.Used != used || d.Physical.RPS != 1000 {
		t.Errorf("physical must be untouched: %+v", d.Physical)
	}
	if d.Virtual.Used.CPUCores != 256 || d.Virtual.Used.MemoryBytes != 512 || d.Virtual.RPS != 128000 {
		t.Errorf("virtual = %+v", d.Virtual)
	}
	if d.Virtual.Capacity != virtCap || d.Virtual.Nodes != 96 {
		t.Errorf("virtual capacity/nodes = %+v", d.Virtual)
	}
}

func TestTranslatePreservesUtilizationPerResource(t *testing.T) {
	// CPU ratio is far below the binding (memory) ratio; a single factor would
	// push virtual CPU utilization above 100%.
	f := scale.Factor{CPU: 10, Memory: 100, Disk: 50, Effective: 128}
	physBudget := budget.Resources{CPUCores: 10, MemoryBytes: 1000, DiskBytes: 200}
	used := budget.Resources{CPUCores: 5, MemoryBytes: 500, DiskBytes: 100}
	virtCap := budget.Resources{CPUCores: 100, MemoryBytes: 100000, DiskBytes: 10000}

	d := Translate(f, physBudget, used, 10, virtCap, 5)

	if got := d.Virtual.Used.CPUCores / d.Virtual.Capacity.CPUCores; got != 0.5 {
		t.Errorf("cpu utilization = %v, want 0.5", got)
	}
	if got := float64(d.Virtual.Used.MemoryBytes) / float64(d.Virtual.Capacity.MemoryBytes); got != 0.5 {
		t.Errorf("memory utilization = %v, want 0.5", got)
	}
	if got := float64(d.Virtual.Used.DiskBytes) / float64(d.Virtual.Capacity.DiskBytes); got != 0.5 {
		t.Errorf("disk utilization = %v, want 0.5", got)
	}
	if d.Virtual.RPS != 1280 {
		t.Errorf("rps = %v", d.Virtual.RPS)
	}
}
