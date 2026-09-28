package budget

import (
	"errors"
	"testing"

	"github.com/samin-al-wasee/production-simulator/core/internal/calibration"
)

var testHost = calibration.Host{CPUCores: 8, MemoryBytes: 16 << 30, DiskBytes: 100 << 30}

func TestComputeDefault(t *testing.T) {
	b, err := Compute(testHost, UniformPolicy(DefaultReserveFraction))
	if err != nil {
		t.Fatal(err)
	}
	want := Resources{CPUCores: 6, MemoryBytes: 12 << 30, DiskBytes: 75 << 30}
	if b.Allocatable != want {
		t.Fatalf("allocatable = %+v, want %+v", b.Allocatable, want)
	}
	if b.Reserved.CPUCores != 2 || b.Reserved.MemoryBytes != 4<<30 {
		t.Fatalf("reserved = %+v", b.Reserved)
	}
}

func TestComputeZeroReserve(t *testing.T) {
	b, err := Compute(testHost, UniformPolicy(0))
	if err != nil {
		t.Fatal(err)
	}
	if b.Allocatable != b.Host {
		t.Fatalf("allocatable %+v != host %+v", b.Allocatable, b.Host)
	}
}

func TestPolicyValidate(t *testing.T) {
	for _, f := range []float64{-0.1, 1, 1.5} {
		if _, err := Compute(testHost, UniformPolicy(f)); err == nil {
			t.Errorf("expected error for fraction %v", f)
		}
	}
}

func TestRemaining(t *testing.T) {
	b, _ := Compute(testHost, UniformPolicy(0.25))
	left, err := b.Remaining(
		Resources{CPUCores: 1, MemoryBytes: 2 << 30, DiskBytes: 5 << 30},
		Resources{CPUCores: 2, MemoryBytes: 4 << 30},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := Resources{CPUCores: 3, MemoryBytes: 6 << 30, DiskBytes: 70 << 30}
	if left != want {
		t.Fatalf("left = %+v, want %+v", left, want)
	}
}

func TestRemainingExceeded(t *testing.T) {
	b, _ := Compute(testHost, UniformPolicy(0.25))
	cases := map[string]Resources{
		"cpu":    {CPUCores: 7},
		"memory": {MemoryBytes: 13 << 30},
		"disk":   {DiskBytes: 76 << 30},
	}
	for name, d := range cases {
		if _, err := b.Remaining(d); !errors.Is(err, ErrExceeded) {
			t.Errorf("%s: err = %v, want ErrExceeded", name, err)
		}
	}
}
