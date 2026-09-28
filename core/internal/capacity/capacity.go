// Package capacity models virtual capacity (CPU, RAM, storage, database) and
// what happens as demand approaches it (ADR-0003). It is pure: the same
// capacity and demand always give the same result.
package capacity

import (
	"fmt"
	"math"
	"time"

	"github.com/samin-al-wasee/production-simulator/core/internal/budget"
)

// Database is the virtual capacity of one database tier.
type Database struct {
	MaxConnections int     `json:"maxConnections"`
	MaxQPS         float64 `json:"maxQps"`
	StorageBytes   uint64  `json:"storageBytes"`
}

// Capacity is the virtual capacity of the production view.
type Capacity struct {
	Compute  budget.Resources `json:"compute"`
	Database Database         `json:"database"`
}

// Demand is the load placed on a Capacity.
type Demand struct {
	Compute        budget.Resources `json:"compute"`
	DBConnections  int              `json:"dbConnections"`
	DBQPS          float64          `json:"dbQps"`
	DBStorageBytes uint64           `json:"dbStorageBytes"`
}

// Utilization is demand divided by capacity per resource (1.0 = full).
type Utilization struct {
	CPU           float64 `json:"cpu"`
	Memory        float64 `json:"memory"`
	Disk          float64 `json:"disk"`
	DBConnections float64 `json:"dbConnections"`
	DBQPS         float64 `json:"dbQps"`
	DBStorage     float64 `json:"dbStorage"`
}

// Saturation thresholds.
const (
	// SaturatedAt is utilization at or above which a resource is saturated.
	SaturatedAt = 1.0
	// WarningAt is utilization at or above which a resource is under pressure.
	WarningAt = 0.8
)

// Utilize computes per-resource utilization. A zero capacity with non-zero
// demand is reported as +Inf, never hidden.
func Utilize(c Capacity, d Demand) Utilization {
	return Utilization{
		CPU:           ratio(d.Compute.CPUCores, c.Compute.CPUCores),
		Memory:        ratio(float64(d.Compute.MemoryBytes), float64(c.Compute.MemoryBytes)),
		Disk:          ratio(float64(d.Compute.DiskBytes), float64(c.Compute.DiskBytes)),
		DBConnections: ratio(float64(d.DBConnections), float64(c.Database.MaxConnections)),
		DBQPS:         ratio(d.DBQPS, c.Database.MaxQPS),
		DBStorage:     ratio(float64(d.DBStorageBytes), float64(c.Database.StorageBytes)),
	}
}

func ratio(demand, capacity float64) float64 {
	if capacity == 0 {
		if demand == 0 {
			return 0
		}
		return math.Inf(1)
	}
	return demand / capacity
}

// Named returns the utilization values keyed by resource name, in a stable
// order given by Resources.
func (u Utilization) Named() map[string]float64 {
	return map[string]float64{
		"cpu": u.CPU, "memory": u.Memory, "disk": u.Disk,
		"db-connections": u.DBConnections, "db-qps": u.DBQPS, "db-storage": u.DBStorage,
	}
}

// Resources lists resource names in a stable order.
var Resources = []string{"cpu", "memory", "disk", "db-connections", "db-qps", "db-storage"}

// Saturated returns the names of resources at or above SaturatedAt, in the
// order of Resources.
func (u Utilization) Saturated() []string { return u.atLeast(SaturatedAt) }

// Pressured returns the names of resources at or above WarningAt, in the
// order of Resources.
func (u Utilization) Pressured() []string { return u.atLeast(WarningAt) }

func (u Utilization) atLeast(threshold float64) []string {
	named := u.Named()
	var out []string
	for _, r := range Resources {
		if named[r] >= threshold {
			out = append(out, r)
		}
	}
	return out
}

// TimeToExhaustion returns how long until used reaches capacity when it
// grows by growthPerSecond. ok is false when growth is not positive
// (never exhausts); a full resource returns zero.
func TimeToExhaustion(used, capacity uint64, growthPerSecond float64) (d time.Duration, ok bool, err error) {
	if growthPerSecond < 0 || math.IsNaN(growthPerSecond) {
		return 0, false, fmt.Errorf("growth must be a non-negative number, got %v", growthPerSecond)
	}
	if used >= capacity {
		return 0, true, nil
	}
	if growthPerSecond == 0 {
		return 0, false, nil
	}
	secs := float64(capacity-used) / growthPerSecond
	if secs > float64(math.MaxInt64)/float64(time.Second) {
		return 0, false, nil
	}
	return time.Duration(secs * float64(time.Second)), true, nil
}
