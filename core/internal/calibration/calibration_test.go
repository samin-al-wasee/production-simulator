package calibration

import (
	"strings"
	"testing"
)

func TestParseMemInfo(t *testing.T) {
	got, err := ParseMemInfo(strings.NewReader("MemFree: 1 kB\nMemTotal:       16384 kB\nBuffers: 2 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := uint64(16384 * 1024); got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestParseMemInfoErrors(t *testing.T) {
	for _, in := range []string{"", "MemFree: 1 kB\n", "MemTotal: abc kB\n"} {
		if _, err := ParseMemInfo(strings.NewReader(in)); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestParseCgroupMemory(t *testing.T) {
	limit, ok, err := ParseCgroupMemory("2147483648\n")
	if err != nil || !ok || limit != 2147483648 {
		t.Fatalf("got %d %v %v", limit, ok, err)
	}
	if _, ok, err := ParseCgroupMemory("max\n"); err != nil || ok {
		t.Fatalf("max: ok=%v err=%v", ok, err)
	}
	if _, _, err := ParseCgroupMemory("x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCgroupCPU(t *testing.T) {
	cores, ok, err := ParseCgroupCPU("150000 100000\n")
	if err != nil || !ok || cores != 1.5 {
		t.Fatalf("got %v %v %v", cores, ok, err)
	}
	if _, ok, err := ParseCgroupCPU("max 100000"); err != nil || ok {
		t.Fatalf("max: ok=%v err=%v", ok, err)
	}
	for _, in := range []string{"", "100000", "a 100000", "100000 0"} {
		if _, _, err := ParseCgroupCPU(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestLimit(t *testing.T) {
	if got := limitFloat(8, 2, true); got != 2 {
		t.Errorf("limit below host: got %v", got)
	}
	if got := limitFloat(8, 16, true); got != 8 {
		t.Errorf("limit above host: got %v", got)
	}
	if got := limitFloat(8, 0, false); got != 8 {
		t.Errorf("no limit: got %v", got)
	}
	if got := limitUint(8, 2, true); got != 2 {
		t.Errorf("uint limit: got %v", got)
	}
}

func TestParseLoadAvg(t *testing.T) {
	got, err := ParseLoadAvg("1.52 0.90 0.40 2/300 12345\n")
	if err != nil || got != 1.52 {
		t.Fatalf("got %v %v", got, err)
	}
	for _, in := range []string{"", "x 1 2"} {
		if _, err := ParseLoadAvg(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestParseMemAvailable(t *testing.T) {
	got, err := ParseMemAvailable(strings.NewReader("MemTotal: 10 kB\nMemAvailable: 4 kB\n"))
	if err != nil || got != 4096 {
		t.Fatalf("got %v %v", got, err)
	}
	if _, err := ParseMemAvailable(strings.NewReader("MemTotal: 10 kB\n")); err == nil {
		t.Fatal("expected error")
	}
}
