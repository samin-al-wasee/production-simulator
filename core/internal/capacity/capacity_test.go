package capacity

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
)

var testCap = Capacity{
	Compute:  budget.Resources{CPUCores: 100, MemoryBytes: 1000, DiskBytes: 10000},
	Database: Database{MaxConnections: 500, MaxQPS: 20000, StorageBytes: 1 << 40},
}

func TestUtilizeAndSaturation(t *testing.T) {
	d := Demand{
		Compute:       budget.Resources{CPUCores: 85, MemoryBytes: 1000, DiskBytes: 100},
		DBConnections: 500, DBQPS: 5000, DBStorageBytes: 1 << 39,
	}
	u := Utilize(testCap, d)
	if u.CPU != 0.85 || u.Memory != 1 || u.DBQPS != 0.25 || u.DBStorage != 0.5 {
		t.Fatalf("utilization = %+v", u)
	}
	if got, want := u.Saturated(), []string{"memory", "db-connections"}; !reflect.DeepEqual(got, want) {
		t.Errorf("saturated = %v, want %v", got, want)
	}
	if got, want := u.Pressured(), []string{"cpu", "memory", "db-connections"}; !reflect.DeepEqual(got, want) {
		t.Errorf("pressured = %v, want %v", got, want)
	}
}

func TestUtilizeZeroCapacity(t *testing.T) {
	u := Utilize(Capacity{}, Demand{DBQPS: 1})
	if !math.IsInf(u.DBQPS, 1) || u.CPU != 0 {
		t.Fatalf("utilization = %+v", u)
	}
}

func TestTimeToExhaustion(t *testing.T) {
	d, ok, err := TimeToExhaustion(900, 1000, 10)
	if err != nil || !ok || d != 10*time.Second {
		t.Fatalf("got %v %v %v", d, ok, err)
	}
	if d, ok, _ := TimeToExhaustion(1000, 1000, 5); !ok || d != 0 {
		t.Errorf("full: %v %v", d, ok)
	}
	if _, ok, _ := TimeToExhaustion(1, 1000, 0); ok {
		t.Error("zero growth must never exhaust")
	}
	if _, _, err := TimeToExhaustion(1, 1000, -1); err == nil {
		t.Error("expected error for negative growth")
	}
}
