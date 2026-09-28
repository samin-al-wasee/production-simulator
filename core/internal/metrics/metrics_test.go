package metrics

import (
	"testing"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
	"github.com/samin-al-wasee/production-simulator/core/internal/scale"
)

func TestTranslateLabelsAndScales(t *testing.T) {
	f := scale.Factor{Effective: 128}
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
