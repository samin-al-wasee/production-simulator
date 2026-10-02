package sandbox

import (
	"fmt"
	"math"
	"strings"
)

// QueryProfile is what one query of a kind costs a database: its own CPU and
// the pages it touches when it can use an index.
type QueryProfile struct {
	CPUMs   float64 `json:"cpuMs"`
	Pages   float64 `json:"pages"`
	Indexed bool    `json:"indexed"`
}

// DBConfig is a database's configuration (v8, ADR-0020). It applies to each
// replica of the node and is replaced whole by a configure command.
type DBConfig struct {
	// Engine is a label: PostgreSQL, MySQL.
	Engine         string       `json:"engine"`
	MaxConnections int          `json:"maxConnections"`
	Read           QueryProfile `json:"read"`
	Write          QueryProfile `json:"write"`
	// Writes contend for HotRows rows, each held locked for LockMs.
	HotRows int     `json:"hotRows"`
	LockMs  float64 `json:"lockMs"`
}

// DBRuntime holds the constants of the database model.
type DBRuntime struct {
	// BufferPoolShare of a replica's memory caches pages.
	BufferPoolShare float64 `json:"bufferPoolShare"`
	// Data is BaseDataMB plus KBPerUser for every user; WorkingSetShare of
	// it is read often.
	BaseDataMB      float64 `json:"baseDataMb"`
	KBPerUser       float64 `json:"kbPerUser"`
	WorkingSetShare float64 `json:"workingSetShare"`
	// PageCPUMs is the CPU to process a page; DiskMs a read from disk.
	PageCPUMs float64 `json:"pageCpuMs"`
	DiskMs    float64 `json:"diskMs"`
	// An unindexed query scans ScanPagesPerMB pages per MB of data.
	ScanPagesPerMB float64 `json:"scanPagesPerMb"`
	// A replica applies a write for ApplyShare of its CPU.
	ApplyShare float64 `json:"applyShare"`
	// A replica further behind than LagDegradedSeconds is degraded.
	LagDegradedSeconds float64 `json:"lagDegradedSeconds"`
}

// DBStats is what a database did during a tick, over all its replicas.
type DBStats struct {
	Health string `json:"health"`
	// Bottleneck is cpu, iops, or locks.
	Bottleneck     string  `json:"bottleneck"`
	Capacity       float64 `json:"capacity"`
	CPUUsed        float64 `json:"cpuUsed"`
	CPUTotal       float64 `json:"cpuTotal"`
	IOPSUsed       float64 `json:"iopsUsed"`
	IOPSTotal      float64 `json:"iopsTotal"`
	HitRatio       float64 `json:"hitRatio"`
	DataMB         float64 `json:"dataMb"`
	WorkingSetMB   float64 `json:"workingSetMb"`
	BufferPoolMB   float64 `json:"bufferPoolMb"`
	Connections    float64 `json:"connections"`
	MaxConnections int     `json:"maxConnections"`
	// Refused is queries per second turned away for too many connections.
	Refused    float64 `json:"refused"`
	Reads      float64 `json:"reads"`
	Writes     float64 `json:"writes"`
	ReadMs     float64 `json:"readMs"`
	WriteMs    float64 `json:"writeMs"`
	WaitMs     float64 `json:"waitMs"`
	LockWaitMs float64 `json:"lockWaitMs"`
	// A replica's applied writes per second and how far behind it is.
	Applying   float64 `json:"applying,omitempty"`
	LagSeconds float64 `json:"lagSeconds,omitempty"`
}

// dbRun is what a database did with a tick's load.
type dbRun struct {
	served  float64
	ok      float64 // chance a query arriving succeeds
	lat     vec     // per request class
	backlog float64 // a replica's writes not yet applied
	stats   DBStats
}

func (g *Game) dbModel(n *Node) bool {
	return (n.Kind == KindDBPrimary || n.Kind == KindDBReplica) && g.Rules.DB != nil
}

func (g *Game) dbConfig(n *Node) *DBConfig {
	if n.DB != nil {
		return n.DB
	}
	return g.Rules.DB
}

// dataMB is how much data the databases hold: it grows with the userbase.
func (g *Game) dataMB() float64 {
	rt := g.Rules.DBRuntime
	return rt.BaseDataMB + g.Users*rt.KBPerUser/1024
}

// dbCosts are a query's CPU-ms, disk reads and writes, and service time
// for a read and a write at node n, and the buffer hit ratio.
type dbCosts struct {
	cpuR, ioR, svcR, cpuW, ioW, svcW, hit, pool, ws float64
}

func (g *Game) dbCosts(n *Node) dbCosts {
	rt := g.Rules.DBRuntime
	cfg := g.dbConfig(n)
	size, _ := g.Rules.Size(n.Size)
	slow := factor(g.fx.slow, n.ID)
	data := g.dataMB()
	c := dbCosts{pool: size.MemoryGB * 1024 * rt.BufferPoolShare, ws: data * rt.WorkingSetShare}
	c.hit = 1.0
	if c.ws > 0 {
		c.hit = math.Min(1, c.pool/c.ws)
	}
	pages := func(q QueryProfile) float64 {
		if q.Indexed {
			return q.Pages
		}
		return q.Pages + rt.ScanPagesPerMB*data
	}
	pr, pw := pages(cfg.Read), pages(cfg.Write)
	c.cpuR = (cfg.Read.CPUMs + pr*rt.PageCPUMs) * slow
	c.ioR = pr * (1 - c.hit)
	c.svcR = c.cpuR + c.ioR*rt.DiskMs*slow
	// A write finds its rows, then logs (one fsync) and dirties its pages.
	c.cpuW = (cfg.Write.CPUMs + pw*rt.PageCPUMs) * slow
	c.ioW = pw*(1-c.hit) + 1 + cfg.Write.Pages
	c.svcW = c.cpuW + c.ioW*rt.DiskMs*slow
	return c
}

// replicated is the write rate a replica applies: what the primaries it
// shares a sender with served last tick.
func (g *Game) replicated(i int) float64 {
	n := g.Nodes[i]
	if n.Kind != KindDBReplica {
		return 0
	}
	senders := map[string]bool{}
	for _, e := range g.Edges {
		if e.To == n.ID {
			senders[e.From] = true
		}
	}
	rate := 0.0
	for _, p := range g.Nodes {
		if p.Kind != KindDBPrimary {
			continue
		}
		for _, e := range g.Edges {
			if e.To == p.ID && senders[e.From] {
				rate += g.lastWrites[p.ID]
				break
			}
		}
	}
	return rate
}

// dbOpen is how many connections the callers' pools open to node i.
func (g *Game) dbOpen(i int) float64 {
	n := g.Nodes[i]
	open := 0.0
	for _, e := range g.Edges {
		if e.To == n.ID && e.Conn != nil {
			if f := g.Node(e.From); f != nil {
				open += float64(e.Conn.Pool * g.upReplicas(f))
			}
		}
	}
	return open
}

// runDB applies the database model to node i's load. Capacity is the
// throughput at which CPU, IOPS, or row locks run out at this tick's
// read/write mix, after a replica has applied its primaries' writes.
func (g *Game) runDB(i int, load vec) dbRun {
	n := g.Nodes[i]
	r := g.Rules
	rt := r.DBRuntime
	cfg := g.dbConfig(n)
	size, _ := r.Size(n.Size)
	up := float64(g.upReplicas(n))
	c := g.dbCosts(n)
	reads, writes := load[clsCacheable]+load[clsRead], load[clsWrite]
	lambda := reads + writes
	fr, fw := 0.8, 0.2
	if lambda > 0 {
		fr, fw = reads/lambda, writes/lambda
	}
	cpuOp, ioOp := fr*c.cpuR+fw*c.cpuW, fr*c.ioR+fw*c.ioW
	cpuTotal, iopsTotal := size.VCPU*1000*up, size.IOPS*up

	// A replica applies its primaries' writes first; reads get the rest.
	run := dbRun{}
	apply := g.replicated(i)
	applyCPU, applyIO := c.cpuW*rt.ApplyShare, 1+cfg.Write.Pages
	applied := apply
	if apply > 0 {
		applied = math.Min(apply, math.Min(cpuTotal/applyCPU, iopsTotal/applyIO))
		run.backlog = math.Max(0, n.Backlog+(apply-applied)*r.TickSeconds)
		run.stats.Applying = applied
		run.stats.LagSeconds = run.backlog / apply
	}
	cpuFree, iopsFree := cpuTotal-applied*applyCPU, iopsTotal-applied*applyIO

	capacity, bottleneck := math.Inf(1), "cpu"
	lockCap := math.Inf(1)
	if cfg.LockMs > 0 && n.Kind == KindDBPrimary {
		lockCap = float64(cfg.HotRows) * 1000 / cfg.LockMs
	}
	for _, l := range []struct {
		name      string
		cap, cost float64
	}{
		{"cpu", cpuFree, cpuOp},
		{"iops", iopsFree, ioOp},
		{"locks", lockCap, fw},
	} {
		if l.cost > 0 && l.cap/l.cost < capacity {
			capacity, bottleneck = math.Max(0, l.cap/l.cost), l.name
		}
	}
	if up == 0 {
		capacity = 0
	}

	run.served = math.Min(lambda, capacity)
	rho := 0.0
	if capacity > 0 {
		rho = lambda / capacity
	} else if lambda > 0 {
		rho = math.Inf(1)
	}
	u := math.Min(rho, 0.99)
	wait := (fr*c.svcR + fw*c.svcW) * u / (1 - u)
	lockWait := 0.0
	if !math.IsInf(lockCap, 1) {
		ul := math.Min(writes/lockCap, 0.99)
		lockWait = cfg.LockMs * ul / (1 - ul)
	}
	readMs, writeMs := math.Min(c.svcR+wait, r.TimeoutMs), math.Min(c.svcW+wait+lockWait, r.TimeoutMs)
	run.lat = vec{readMs, readMs, writeMs}

	open := g.dbOpen(i)
	refused := 0.0
	if maxOpen := float64(cfg.MaxConnections) * up; open > maxOpen {
		refused = 1 - maxOpen/open
	}
	run.ok = 1 - refused
	if lambda > 0 {
		run.ok *= run.served / lambda
	}

	s := &run.stats
	s.Bottleneck, s.Capacity = bottleneck, finite(capacity)
	s.CPUTotal, s.IOPSTotal = cpuTotal/1000, iopsTotal
	s.CPUUsed = (run.served*cpuOp + applied*applyCPU) / 1000
	s.IOPSUsed = run.served*ioOp + applied*applyIO
	s.HitRatio, s.DataMB, s.WorkingSetMB, s.BufferPoolMB = c.hit, g.dataMB(), c.ws, c.pool*up
	s.Connections, s.MaxConnections, s.Refused = open, cfg.MaxConnections, lambda*refused
	s.Reads, s.Writes = reads, writes
	s.ReadMs, s.WriteMs, s.WaitMs, s.LockWaitMs = readMs, writeMs, wait, lockWait
	fail := 1 - run.ok
	switch {
	case n.Down:
		s.Health = HealthStopped
	case fail > 0.2:
		s.Health = HealthUnhealthy
	case rho > 0.85 || fail > 0.01 || s.LagSeconds > rt.LagDegradedSeconds:
		s.Health = HealthDegraded
	default:
		s.Health = HealthHealthy
	}
	return run
}

// dbCapacity is node i's throughput at last tick's mix, which the callers
// split their queries by before this tick's load is known.
func (g *Game) dbCapacity(i int) float64 {
	n := g.Nodes[i]
	mix := g.lastLoad[n.ID]
	if mix.sum() == 0 {
		mix = vec{0, 0.8, 0.2}
	}
	return g.runDB(i, mix).stats.Capacity
}

// configureDB replaces a database's configuration.
func (g *Game) configureDB(n *Node, c *DBConfig) error {
	if c == nil {
		return invalid("configure needs a database configuration")
	}
	if p := g.Rules.ValidateDB(*c); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.DB = c
	return nil
}

// ValidateDB lists every problem with a database configuration.
func (r *Ruleset) ValidateDB(c DBConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if len(c.Engine) > maxNameLen {
		bad("engine must be at most %d characters", maxNameLen)
	}
	if c.MaxConnections < 1 || c.MaxConnections > 10_000 {
		bad("max connections must be between 1 and 10000")
	}
	for label, q := range map[string]QueryProfile{"read": c.Read, "write": c.Write} {
		if !(q.CPUMs > 0 && q.CPUMs <= 1000) {
			bad("%s query CPU must be above 0 and at most 1000 ms", label)
		}
		if !(q.Pages >= 0 && q.Pages <= 1e6) {
			bad("%s query pages must be between 0 and 1000000", label)
		}
	}
	if c.HotRows < 1 || c.HotRows > 1_000_000_000 {
		bad("hot rows must be between 1 and 1000000000")
	}
	if !(c.LockMs >= 0 && c.LockMs <= 10_000) {
		bad("lock time must be between 0 and 10000 ms")
	}
	return p
}

// RulesetV8 is RulesetV7 with databases as modelled data stores (Phase 11,
// ADR-0020).
func RulesetV8() *Ruleset {
	r := RulesetV7()
	r.Version = "sandbox/v8"
	iops := map[string]float64{"small": 1000, "medium": 3000, "large": 8000}
	for i := range r.Sizes {
		r.Sizes[i].IOPS = iops[r.Sizes[i].Name]
	}
	r.DB = &DBConfig{
		Engine: "PostgreSQL", MaxConnections: 100,
		Read:    QueryProfile{CPUMs: 2.5, Pages: 4, Indexed: true},
		Write:   QueryProfile{CPUMs: 5, Pages: 3, Indexed: true},
		HotRows: 500, LockMs: 1,
	}
	r.DBRuntime = DBRuntime{BufferPoolShare: 0.75, BaseDataMB: 200, KBPerUser: 50, WorkingSetShare: 0.2,
		PageCPUMs: 0.005, DiskMs: 0.5, ScanPagesPerMB: 1, ApplyShare: 0.5, LagDegradedSeconds: 10}
	return r
}
