package sandbox

import (
	"fmt"
	"math"
	"strings"
)

// Cache eviction policies.
const (
	EvictLRU  = "lru"
	EvictLFU  = "lfu"
	EvictNone = "none"
)

// CacheConfig is a cache's configuration (v9, ADR-0021).
type CacheConfig struct {
	// Engine is a label: Redis, Memcached.
	Engine         string  `json:"engine"`
	Eviction       string  `json:"eviction"`
	TTLSeconds     float64 `json:"ttlSeconds"`
	ValueKB        float64 `json:"valueKb"`
	MaxConnections int     `json:"maxConnections"`
}

// CacheRuntime holds the constants of the cache model.
type CacheRuntime struct {
	// MemoryShare of a replica's memory holds values.
	MemoryShare float64 `json:"memoryShare"`
	// Requests concentrate on HotKeyShare of the keys.
	HotKeyShare float64 `json:"hotKeyShare"`
	// OpCPUMs and BaseMs are an operation's CPU and unloaded latency.
	OpCPUMs float64 `json:"opCpuMs"`
	BaseMs  float64 `json:"baseMs"`
	// Skew is the exponent of the share of the working set that fits, by
	// eviction policy: the hit ratio is fits^Skew before expiry.
	Skew map[string]float64 `json:"skew"`
}

// CacheStats is what a cache did during a tick, over all its replicas.
type CacheStats struct {
	Health         string  `json:"health"`
	Bottleneck     string  `json:"bottleneck"`
	Capacity       float64 `json:"capacity"`
	HitRatio       float64 `json:"hitRatio"`
	Fits           float64 `json:"fits"`
	Fresh          float64 `json:"fresh"`
	Warmth         float64 `json:"warmth"`
	MemoryUsedMB   float64 `json:"memoryUsedMb"`
	MemoryTotalMB  float64 `json:"memoryTotalMb"`
	Keys           float64 `json:"keys"`
	Hits           float64 `json:"hits"`
	Misses         float64 `json:"misses"`
	Evictions      float64 `json:"evictions"`
	CPUUsed        float64 `json:"cpuUsed"`
	CPUTotal       float64 `json:"cpuTotal"`
	NetworkMbps    float64 `json:"networkMbps"`
	NetworkTotal   float64 `json:"networkTotalMbps"`
	Connections    float64 `json:"connections"`
	MaxConnections int     `json:"maxConnections"`
	Refused        float64 `json:"refused"`
	LatencyMs      float64 `json:"latencyMs"`
}

// cacheRun is what a cache did with a tick's load.
type cacheRun struct {
	served, hit, refused, lat, warmth float64
	stats                             CacheStats
}

func (g *Game) cacheModel(n *Node) bool {
	return n.Kind == KindCache && g.Rules.Cache != nil
}

func (g *Game) cacheConfig(n *Node) *CacheConfig {
	if n.Cache != nil {
		return n.Cache
	}
	return g.Rules.Cache
}

// runCache applies the cache model to node i's load. The hit ratio is the
// share of the working set that fits, skewed by the eviction policy, times
// the chance a hot key is read again before it expires, never more than the
// share of hot keys already loaded.
func (g *Game) runCache(i int, load vec) cacheRun {
	n := g.Nodes[i]
	r := g.Rules
	rt := r.CacheRuntime
	cfg := g.cacheConfig(n)
	size, _ := r.Size(n.Size)
	up := float64(g.upReplicas(n))
	ws := g.dataMB() * r.DBRuntime.WorkingSetShare
	mem := size.MemoryGB * 1024 * rt.MemoryShare
	keys := ws * 1024 / cfg.ValueKB
	hot := math.Max(1, keys*rt.HotKeyShare)
	ops := load.sum()

	run := cacheRun{warmth: n.Warmth}
	if up == 0 {
		run.warmth = 0
	}
	fits := 1.0
	if ws > 0 {
		fits = math.Min(1, mem/ws)
	}
	fresh := 1 - math.Exp(-cfg.TTLSeconds*ops/hot)
	run.hit = math.Min(math.Pow(fits, rt.Skew[cfg.Eviction])*fresh, run.warmth)
	if v, ok := g.fx.hitRatio[n.ID]; ok {
		run.hit = math.Min(run.hit, v)
	}
	// Every miss loads one key, hottest first.
	run.warmth = math.Min(1, run.warmth+ops*(1-run.hit)*r.TickSeconds/hot)
	if up == 0 {
		run.warmth = 0
	}

	cpuCap := size.VCPU * 1000 * up / rt.OpCPUMs
	netCap := math.Inf(1)
	if mb := cfg.ValueKB * 8 / 1000; mb > 0 {
		netCap = size.NetworkMbps * up / mb
	}
	capacity, bottleneck := cpuCap, "cpu"
	if netCap < capacity {
		capacity, bottleneck = netCap, "network"
	}
	if up == 0 {
		capacity = 0
	}
	run.served = math.Min(ops, capacity)
	u := 0.0
	if capacity > 0 {
		u = math.Min(ops/capacity, 0.99)
	}
	run.lat = math.Min(rt.BaseMs*factor(g.fx.slow, n.ID)/(1-u), r.TimeoutMs)

	open := g.dbOpen(i)
	if maxOpen := float64(cfg.MaxConnections) * up; open > maxOpen {
		run.refused = 1 - maxOpen/open
	}

	s := &run.stats
	s.Bottleneck, s.Capacity = bottleneck, finite(capacity)
	s.HitRatio, s.Fits, s.Fresh, s.Warmth = run.hit, fits, fresh, run.warmth
	s.MemoryTotalMB = mem * up
	s.MemoryUsedMB = math.Min(ws, mem) * run.warmth
	s.Keys = keys
	s.Hits, s.Misses = run.served*run.hit, run.served*(1-run.hit)
	if ws > mem {
		// A full cache evicts a key for every key a miss loads.
		s.Evictions = s.Misses
	}
	s.CPUTotal, s.CPUUsed = size.VCPU*up, run.served*rt.OpCPUMs/1000
	s.NetworkTotal, s.NetworkMbps = size.NetworkMbps*up, run.served*cfg.ValueKB*8/1000
	s.Connections, s.MaxConnections, s.Refused = open, cfg.MaxConnections, ops*run.refused
	s.LatencyMs = run.lat
	fail := run.refused
	if ops > 0 {
		fail = 1 - (1-run.refused)*run.served/ops
	}
	switch {
	case n.Down:
		s.Health = HealthStopped
	case fail > 0.2:
		s.Health = HealthUnhealthy
	case run.warmth < 1:
		s.Health = HealthStarting
	case u > 0.85 || fail > 0.01:
		s.Health = HealthDegraded
	default:
		s.Health = HealthHealthy
	}
	return run
}

func (g *Game) configureCache(n *Node, c *CacheConfig) error {
	if c == nil {
		return invalid("configure needs a cache configuration")
	}
	if p := g.Rules.ValidateCache(*c); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.Cache = c
	return nil
}

// ValidateCache lists every problem with a cache configuration.
func (r *Ruleset) ValidateCache(c CacheConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if len(c.Engine) > maxNameLen {
		bad("engine must be at most %d characters", maxNameLen)
	}
	if _, ok := r.CacheRuntime.Skew[c.Eviction]; !ok {
		bad("eviction must be %q, %q, or %q", EvictLRU, EvictLFU, EvictNone)
	}
	if !(c.TTLSeconds >= 1 && c.TTLSeconds <= 7*86400) {
		bad("TTL must be between 1 s and 7 days")
	}
	if !(c.ValueKB >= 0.1 && c.ValueKB <= 1024) {
		bad("value size must be between 0.1 and 1024 KB")
	}
	if c.MaxConnections < 1 || c.MaxConnections > 100_000 {
		bad("max connections must be between 1 and 100000")
	}
	return p
}

// RulesetV9 is RulesetV8 with caches as modelled stores (Phase 11, ADR-0021).
func RulesetV9() *Ruleset {
	r := RulesetV8()
	r.Version = "sandbox/v9"
	r.Cache = &CacheConfig{Engine: "Redis", Eviction: EvictLRU, TTLSeconds: 300, ValueKB: 2, MaxConnections: 10_000}
	r.CacheRuntime = CacheRuntime{MemoryShare: 0.9, HotKeyShare: 0.05, OpCPUMs: 0.02, BaseMs: 0.2,
		Skew: map[string]float64{EvictLRU: 0.5, EvictLFU: 0.4, EvictNone: 1}}
	return r
}
