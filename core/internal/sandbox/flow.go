package sandbox

import "math"

// NodeStats is what one node did during a tick.
type NodeStats struct {
	ID string `json:"id"`
	// Offered and Served are operations per second.
	Offered     float64 `json:"offered"`
	Served      float64 `json:"served"`
	Dropped     float64 `json:"dropped"`
	Capacity    float64 `json:"capacity"`
	Utilization float64 `json:"utilization"`
	LatencyMs   float64 `json:"latencyMs"`
	Backlog     float64 `json:"backlog,omitempty"`
	CostPerHour float64 `json:"costPerHour"`
}

// Flow is the result of routing one tick's traffic through the topology.
type Flow struct {
	RPS            float64     `json:"rps"`
	SuccessRPS     float64     `json:"successRps"`
	ErrorRate      float64     `json:"errorRate"`
	MeanLatencyMs  float64     `json:"meanLatencyMs"`
	P95LatencyMs   float64     `json:"p95LatencyMs"`
	MaxUtilization float64     `json:"maxUtilization"`
	Nodes          []NodeStats `json:"nodes"`
}

// p95Factor converts a mean latency to a p95 under an exponential latency
// distribution: -ln(0.05).
var p95Factor = -math.Log(0.05)

// capacity is the ops/s a node can serve this tick.
func (g *Game) capacity(n *Node) float64 {
	if n.Kind == KindInternet {
		return math.Inf(1)
	}
	if n.Down {
		return 0
	}
	k, _ := g.Rules.Kind(n.Kind)
	s, _ := g.Rules.Size(n.Size)
	return k.Capacity * s.CapacityFactor * float64(n.Replicas)
}

func (g *Game) costPerHour(n *Node) float64 {
	k, _ := g.Rules.Kind(n.Kind)
	s, _ := g.Rules.Size(n.Size)
	return k.CostPerHour * s.CostFactor * float64(n.Replicas)
}

// order returns node indexes in topological order; ties keep placement order.
func (g *Game) order() []int {
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	indeg := make([]int, len(g.Nodes))
	for _, e := range g.Edges {
		indeg[idx[e.To]]++
	}
	var out []int
	done := make([]bool, len(g.Nodes))
	for len(out) < len(g.Nodes) {
		for i := range g.Nodes {
			if done[i] || indeg[i] > 0 {
				continue
			}
			done[i] = true
			out = append(out, i)
			for _, e := range g.Edges {
				if e.From == g.Nodes[i].ID {
					indeg[idx[e.To]]--
				}
			}
			break
		}
	}
	return out
}

// route is a share of a node's served load sent to one downstream node.
type route struct {
	to    int
	share float64
}

// split divides load across targets in proportion to their capacity, the way
// a health-checked balancer would. Failed targets receive nothing unless all
// have failed.
func (g *Game) split(targets []int) []route {
	if len(targets) == 0 {
		return nil
	}
	total := 0.0
	for _, t := range targets {
		total += g.capacity(g.Nodes[t])
	}
	out := make([]route, len(targets))
	for i, t := range targets {
		share := 1 / float64(len(targets))
		if total > 0 {
			share = g.capacity(g.Nodes[t]) / total
		}
		out[i] = route{to: t, share: share}
	}
	return out
}

// plan is how a node's served load leaves it. Each group is a class of work
// (all requests, reads, writes, storage, misses) with the fraction of served
// load it carries and the routes it takes.
type plan struct {
	groups []group
	// local is the fraction of served load completed at this node.
	local float64
	// optional marks the storage group, which multiplies rather than splits.
	storage *group
}

type group struct {
	fraction float64
	routes   []route
}

func (g *Game) targets(i int, kinds ...string) []int {
	var out []int
	for _, e := range g.Edges {
		if e.From != g.Nodes[i].ID {
			continue
		}
		for j, n := range g.Nodes {
			if n.ID != e.To {
				continue
			}
			for _, k := range kinds {
				if n.Kind == k {
					out = append(out, j)
				}
			}
		}
	}
	return out
}

func (g *Game) plan(i int) plan {
	n := g.Nodes[i]
	k, _ := g.Rules.Kind(n.Kind)
	r := g.Rules
	switch n.Kind {
	case KindInternet, KindLB, KindGateway:
		return plan{groups: []group{{1, g.split(g.targets(i, k.ConnectsTo...))}}}
	case KindCDN:
		return plan{local: k.HitRatio, groups: []group{{1 - k.HitRatio, g.split(g.targets(i, k.ConnectsTo...))}}}
	case KindApp:
		reads := g.targets(i, KindCache)
		if len(reads) == 0 {
			reads = g.targets(i, KindDBPrimary, KindDBReplica)
		}
		writes := g.targets(i, KindQueue)
		if len(writes) == 0 {
			writes = g.targets(i, KindDBPrimary)
		}
		return plan{
			groups: []group{
				{r.ReadShare, g.split(reads)},
				{1 - r.ReadShare, g.split(writes)},
			},
			storage: &group{r.StorageShare, g.split(g.targets(i, KindStorage))},
		}
	case KindCache:
		return plan{local: k.HitRatio, groups: []group{{1 - k.HitRatio, g.split(g.targets(i, KindDBPrimary, KindDBReplica))}}}
	case KindWorker:
		return plan{groups: []group{{1, g.split(g.targets(i, KindDBPrimary))}}}
	}
	return plan{local: 1}
}

// solve routes the current tick's traffic without changing the world. It
// returns the flow and each queue's backlog after the tick.
func (g *Game) solve() Snapshot {
	r := g.Rules
	rps := g.rps()
	order := g.order()
	dt := r.TickSeconds

	offered := make([]float64, len(g.Nodes))
	served := make([]float64, len(g.Nodes))
	backlog := make([]float64, len(g.Nodes))
	plans := make([]plan, len(g.Nodes))
	offered[0] = rps

	// Pass 1: push load downstream in topological order.
	for _, i := range order {
		n := g.Nodes[i]
		c := g.capacity(n)
		plans[i] = g.plan(i)
		if n.Kind == KindQueue {
			k, _ := r.Kind(n.Kind)
			workers := g.split(g.targets(i, KindWorker))
			drain := 0.0
			for _, w := range workers {
				drain += g.capacity(g.Nodes[w.to])
			}
			room := math.Max(0, k.MaxBacklog*float64(n.Replicas)-n.Backlog)
			if n.Down {
				room = 0
			}
			accepted := math.Min(offered[i], math.Min(c, drain+room/dt))
			drained := math.Min(drain, n.Backlog/dt+accepted)
			backlog[i] = n.Backlog + (accepted-drained)*dt
			served[i] = accepted
			for _, w := range workers {
				offered[w.to] += drained * w.share
			}
			plans[i] = plan{local: 1}
			continue
		}
		served[i] = math.Min(offered[i], c)
		p := plans[i]
		for _, grp := range append(p.groups, derefGroup(p.storage)...) {
			for _, rt := range grp.routes {
				offered[rt.to] += served[i] * grp.fraction * rt.share
			}
		}
	}

	// Pass 2: from the leaves up, the chance an operation entering a node
	// succeeds (s) and the mean latency of those that do (t).
	s := make([]float64, len(g.Nodes))
	t := make([]float64, len(g.Nodes))
	stats := make([]NodeStats, len(g.Nodes))
	maxU := 0.0
	for o := len(order) - 1; o >= 0; o-- {
		i := order[o]
		n := g.Nodes[i]
		c := g.capacity(n)
		own, u := 0.0, 0.0
		frac := 1.0
		if n.Kind != KindInternet {
			k, _ := r.Kind(n.Kind)
			if c > 0 {
				u = offered[i] / c
			} else if offered[i] > 0 {
				u = math.Inf(1)
			}
			own = math.Min(k.ServiceMs/(1-math.Min(u, 0.99)), r.TimeoutMs)
			if offered[i] > 0 {
				frac = served[i] / offered[i]
			} else if c == 0 {
				frac = 0
			}
			maxU = math.Max(maxU, u)
		}
		p := plans[i]
		succ, lat := p.local, 0.0
		for _, grp := range p.groups {
			gs, gt := 0.0, 0.0
			for _, rt := range grp.routes {
				gs += rt.share * s[rt.to]
				gt += rt.share * s[rt.to] * t[rt.to]
			}
			succ += grp.fraction * gs
			lat += grp.fraction * gt
		}
		if succ > 0 {
			lat /= succ
		}
		if p.storage != nil {
			ss, st := 0.0, 0.0
			for _, rt := range p.storage.routes {
				ss += rt.share * s[rt.to]
				st += rt.share * s[rt.to] * t[rt.to]
			}
			if ss > 0 {
				st /= ss
			}
			succ *= (1 - p.storage.fraction) + p.storage.fraction*ss
			lat += p.storage.fraction * st
		}
		s[i] = frac * succ
		t[i] = own + lat
		stats[i] = NodeStats{
			ID: n.ID, Offered: offered[i], Served: served[i], Dropped: offered[i] - served[i],
			Capacity: finite(c), Utilization: finite(u), LatencyMs: own, Backlog: backlog[i],
		}
		if n.Kind != KindInternet {
			stats[i].CostPerHour = g.costPerHour(n)
		}
	}

	f := Flow{RPS: rps, SuccessRPS: rps * s[0], MeanLatencyMs: t[0], MaxUtilization: finite(maxU), Nodes: stats}
	if rps > 0 {
		f.ErrorRate = 1 - s[0]
	}
	f.P95LatencyMs = math.Min(t[0]*p95Factor, r.TimeoutMs)
	return Snapshot{Flow: f, backlog: backlog}
}

func derefGroup(g *group) []group {
	if g == nil {
		return nil
	}
	return []group{*g}
}

func finite(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
