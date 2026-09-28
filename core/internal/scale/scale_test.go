package scale

import (
	"testing"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
)

func TestComputeBindingAndRounding(t *testing.T) {
	phys := budget.Resources{CPUCores: 6, MemoryBytes: 12 << 30, DiskBytes: 100 << 30}
	virt := budget.Resources{CPUCores: 6 * 100, MemoryBytes: 12 << 30 * 50, DiskBytes: 100 << 30 * 3}
	f, err := Compute(phys, virt)
	if err != nil {
		t.Fatal(err)
	}
	if f.Binding != "cpu" || f.CPU != 100 {
		t.Fatalf("binding = %s cpu = %v", f.Binding, f.CPU)
	}
	if f.Effective != 128 || f.String() != "x128" {
		t.Fatalf("effective = %v (%s)", f.Effective, f)
	}
}

func TestComputeFloorsAtOne(t *testing.T) {
	phys := budget.Resources{CPUCores: 8, MemoryBytes: 8 << 30, DiskBytes: 8 << 30}
	virt := budget.Resources{CPUCores: 2, MemoryBytes: 1 << 30, DiskBytes: 1 << 30}
	f, err := Compute(phys, virt)
	if err != nil {
		t.Fatal(err)
	}
	if f.Effective != 1 {
		t.Fatalf("effective = %v", f.Effective)
	}
}

func TestComputeExactPowerOfTwo(t *testing.T) {
	phys := budget.Resources{CPUCores: 1, MemoryBytes: 1, DiskBytes: 1}
	virt := budget.Resources{CPUCores: 64, MemoryBytes: 1, DiskBytes: 1}
	f, _ := Compute(phys, virt)
	if f.Effective != 64 {
		t.Fatalf("effective = %v", f.Effective)
	}
}

func TestComputeRejectsEmptyPhysical(t *testing.T) {
	if _, err := Compute(budget.Resources{}, budget.Resources{CPUCores: 1}); err == nil {
		t.Fatal("expected error")
	}
}

func TestTranslateRoundTrip(t *testing.T) {
	f := Factor{Effective: 1000 / 1}
	if got := f.ToVirtual(1000); got != 1e6 {
		t.Errorf("ToVirtual = %v", got)
	}
	if got := f.ToPhysical(f.ToVirtual(42)); got != 42 {
		t.Errorf("round trip = %v", got)
	}
}
