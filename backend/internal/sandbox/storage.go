package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// StorageConfig is object storage's configuration (v10, ADR-0022).
type StorageConfig struct {
	Class    string  `json:"class"`
	Prefixes int     `json:"prefixes"`
	ObjectKB float64 `json:"objectKb"`
}

// StorageClass is one storage class's latency and prices.
type StorageClass struct {
	Name        string  `json:"name"`
	Label       string  `json:"label"`
	FirstByteMs float64 `json:"firstByteMs"`
	// Prices: per GB stored per month, per 1,000 GETs, per GB retrieved.
	GBMonth     float64 `json:"gbMonth"`
	PerThousand float64 `json:"perThousand"`
	RetrievalGB float64 `json:"retrievalGb"`
}

// StorageRuntime holds the constants of the object storage model.
type StorageRuntime struct {
	Classes []StorageClass `json:"classes"`
	// GetsPerPrefix is the request rate one prefix sustains.
	GetsPerPrefix float64 `json:"getsPerPrefix"`
	// A transfer runs at TransferMbps per request.
	TransferMbps float64 `json:"transferMbps"`
	// Stored data is BaseGB plus MBPerUser for every user.
	BaseGB    float64 `json:"baseGb"`
	MBPerUser float64 `json:"mbPerUser"`
	EgressGB  float64 `json:"egressGb"`
}

// StorageStats is what object storage did during a tick.
type StorageStats struct {
	Health      string  `json:"health"`
	Class       string  `json:"class"`
	Capacity    float64 `json:"capacity"`
	Gets        float64 `json:"gets"`
	Throttled   float64 `json:"throttled"`
	FirstByteMs float64 `json:"firstByteMs"`
	TransferMs  float64 `json:"transferMs"`
	LatencyMs   float64 `json:"latencyMs"`
	StoredGB    float64 `json:"storedGb"`
	EgressMbps  float64 `json:"egressMbps"`
	// Costs per hour by part.
	StorageCost   float64 `json:"storageCost"`
	RequestCost   float64 `json:"requestCost"`
	RetrievalCost float64 `json:"retrievalCost"`
	EgressCost    float64 `json:"egressCost"`
}

type storageRun struct {
	served, lat, cost float64
	stats             StorageStats
}

func (g *Game) storageModel(n *Node) bool {
	return n.Kind == KindStorage && g.Rules.Storage != nil
}

func (g *Game) storageConfig(n *Node) *StorageConfig {
	if n.Storage != nil {
		return n.Storage
	}
	return g.Rules.Storage
}

func (r *Ruleset) storageClass(name string) (StorageClass, bool) {
	for _, c := range r.StorageRuntime.Classes {
		if c.Name == name {
			return c, true
		}
	}
	return StorageClass{}, false
}

// runStorage applies the object storage model to node i's load: a request
// rate per prefix, latency from the first byte and the transfer, and usage
// pricing for what is stored, requested, retrieved, and sent out.
func (g *Game) runStorage(i int, load vec) storageRun {
	n := g.Nodes[i]
	r := g.Rules
	rt := r.StorageRuntime
	cfg := g.storageConfig(n)
	class, _ := r.storageClass(cfg.Class)
	gets := load.sum()
	capacity := float64(cfg.Prefixes) * rt.GetsPerPrefix * float64(g.upReplicas(n))
	run := storageRun{served: math.Min(gets, capacity)}
	slow := factor(g.fx.slow, n.ID)
	transfer := cfg.ObjectKB * 8 / 1000 / rt.TransferMbps * 1000
	u := 0.0
	if capacity > 0 {
		u = math.Min(gets/capacity, 0.99)
	}
	run.lat = math.Min((class.FirstByteMs*slow+transfer)/(1-u), r.TimeoutMs)

	gb := run.served * cfg.ObjectKB / 1024 / 1024 * 3600 // GB per hour
	s := &run.stats
	s.Class, s.Capacity, s.Gets, s.Throttled = cfg.Class, finite(capacity), gets, gets-run.served
	s.FirstByteMs, s.TransferMs, s.LatencyMs = class.FirstByteMs*slow, transfer, run.lat
	s.StoredGB = rt.BaseGB + g.Users*rt.MBPerUser/1024
	s.EgressMbps = run.served * cfg.ObjectKB * 8 / 1000
	s.StorageCost = s.StoredGB * class.GBMonth / 730
	s.RequestCost = run.served * 3600 / 1000 * class.PerThousand
	s.RetrievalCost = gb * class.RetrievalGB
	s.EgressCost = gb * rt.EgressGB
	run.cost = (s.StorageCost + s.RequestCost + s.RetrievalCost + s.EgressCost) * factor(g.fx.cost, n.Kind)
	switch fail := s.Throttled / math.Max(gets, 1e-300); {
	case n.Down:
		s.Health = HealthStopped
	case fail > 0.2:
		s.Health = HealthUnhealthy
	case u > 0.85 || fail > 0.01:
		s.Health = HealthDegraded
	default:
		s.Health = HealthHealthy
	}
	return run
}

func (g *Game) configureStorage(n *Node, c *StorageConfig) error {
	if c == nil {
		return invalid("configure needs a storage configuration")
	}
	if p := g.Rules.ValidateStorage(*c); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.Storage = c
	return nil
}

// ValidateStorage lists every problem with a storage configuration.
func (r *Ruleset) ValidateStorage(c StorageConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	names := []string{}
	for _, x := range r.StorageRuntime.Classes {
		names = append(names, x.Name)
	}
	if !slices.Contains(names, c.Class) {
		bad("class must be one of %s", strings.Join(names, ", "))
	}
	if c.Prefixes < 1 || c.Prefixes > 1000 {
		bad("prefixes must be between 1 and 1000")
	}
	if !(c.ObjectKB >= 1 && c.ObjectKB <= 1e6) {
		bad("object size must be between 1 KB and 1000000 KB")
	}
	return p
}

// RulesetV10 is RulesetV9 with object storage as a priced, rate-limited
// service (Phase 11, ADR-0022).
func RulesetV10() *Ruleset {
	r := RulesetV9()
	r.Version = "sandbox/v10"
	r.Storage = &StorageConfig{Class: "standard", Prefixes: 1, ObjectKB: 200}
	r.StorageRuntime = StorageRuntime{
		Classes: []StorageClass{
			{Name: "standard", Label: "Standard", FirstByteMs: 20, GBMonth: 0.023, PerThousand: 0.0004},
			{Name: "infrequent", Label: "Infrequent access", FirstByteMs: 30, GBMonth: 0.0125, PerThousand: 0.001, RetrievalGB: 0.01},
			{Name: "archive", Label: "Archive", FirstByteMs: 2000, GBMonth: 0.004, PerThousand: 0.01, RetrievalGB: 0.03},
		},
		GetsPerPrefix: 5500, TransferMbps: 80, BaseGB: 1, MBPerUser: 2, EgressGB: 0.09,
	}
	return r
}
