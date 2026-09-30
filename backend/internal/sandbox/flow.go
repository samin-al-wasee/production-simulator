package sandbox

import "math"

// NodeStats is what one node did during a tick.
type NodeStats struct {
	ID string `json:"id"`
	// Offered and Served are operations per second.
	Offered float64 `json:"offered"`
	Served  float64 `json:"served"`
	Dropped float64 `json:"dropped"`
	// Attack is the part of Offered that is attack traffic; Blocked is what
	// a rate limit stopped here.
	Attack      float64 `json:"attack,omitempty"`
	Blocked     float64 `json:"blocked,omitempty"`
	Capacity    float64 `json:"capacity"`
	Utilization float64 `json:"utilization"`
	LatencyMs   float64 `json:"latencyMs"`
	Backlog     float64 `json:"backlog,omitempty"`
	CostPerHour float64 `json:"costPerHour"`
}

// Flow is the result of routing one tick's traffic through the topology.
type Flow struct {
	// RPS is real user traffic; AttackRPS arrives with it and earns nothing.
	RPS            float64     `json:"rps"`
	AttackRPS      float64     `json:"attackRps,omitempty"`
	SuccessRPS     float64     `json:"successRps"`
	ErrorRate      float64     `json:"errorRate"`
	MeanLatencyMs  float64     `json:"meanLatencyMs"`
	P95LatencyMs   float64     `json:"p95LatencyMs"`
	MaxUtilization float64     `json:"maxUtilization"`
	Nodes          []NodeStats `json:"nodes"`
	// Traffic is what the Internet sent, broken down by group, region, and
	// endpoint.
	Traffic Traffic `json:"traffic"`
}

// p95Factor converts a mean latency to a p95 under an exponential latency
// distribution: -ln(0.05).
var p95Factor = -math.Log(0.05)

// capacity is the ops/s a node can serve this tick.
func (g *Game) capacity(n *Node) float64 {
	if n.Kind == KindInternet {
		return math.Inf(1)
	}
	k, _ := g.Rules.Kind(n.Kind)
	s, _ := g.Rules.Size(n.Size)
	return k.Capacity * s.CapacityFactor * float64(g.upReplicas(n)) * factor(g.fx.capacity, n.ID) / factor(g.fx.slow, n.ID)
}

// costPerHour charges every replica, including those an event took down.
func (g *Game) costPerHour(n *Node) float64 {
	k, _ := g.Rules.Kind(n.Kind)
	s, _ := g.Rules.Size(n.Size)
	return k.CostPerHour * s.CostFactor * float64(n.Replicas) * factor(g.fx.cost, n.Kind)
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

// Request classes. The solver routes each separately, because the
// components ahead of an application treat them differently: a CDN answers
// only cacheable reads, and an application sends reads and writes to
// different places.
const (
	clsCacheable = iota // a read a CDN may answer
	clsRead             // a read that must reach the origin
	clsWrite
	nClass
)

// vec is a load (ops/s) or a per-operation value for each request class.
type vec [nClass]float64

func (v vec) sum() float64 {
	return v[clsCacheable] + v[clsRead] + v[clsWrite]
}

// plan is how a node's served load of one class leaves it. Each group is a
// class of work (all requests, reads, writes, misses) with the fraction of
// served load it carries and the routes it takes.
type plan struct {
	groups []group
	// local is the fraction of served load completed at this node.
	local float64
	// storage is the fetch from object storage that an application makes
	// for a fraction of its requests; it multiplies rather than splits.
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

// plan is how node i routes class c. storage is the share of that class at
// the node that also fetches from object storage.
func (g *Game) plan(i, c int, storage float64) plan {
	n := g.Nodes[i]
	k, _ := g.Rules.Kind(n.Kind)
	switch n.Kind {
	case KindInternet, KindLB, KindGateway:
		return plan{groups: []group{{1, g.split(g.targets(i, k.ConnectsTo...))}}}
	case KindCDN:
		// Without a traffic configuration every request counts as cacheable,
		// as in rulesets v1 to v3.
		hit := 0.0
		if c == clsCacheable || g.Rules.Traffic == nil {
			hit = k.HitRatio
		}
		return plan{local: hit, groups: []group{{1 - hit, g.split(g.targets(i, k.ConnectsTo...))}}}
	case KindApp:
		var to []int
		if c == clsWrite {
			if to = g.targets(i, KindQueue); len(to) == 0 {
				to = g.targets(i, KindDBPrimary)
			}
		} else if to = g.targets(i, KindCache); len(to) == 0 {
			to = g.targets(i, KindDBPrimary, KindDBReplica)
		}
		return plan{
			groups:  []group{{1, g.split(to)}},
			storage: &group{storage, g.split(g.targets(i, KindStorage))},
		}
	case KindCache:
		hit := k.HitRatio
		if v, ok := g.fx.hitRatio[n.ID]; ok {
			hit = v
		}
		return plan{local: hit, groups: []group{{1 - hit, g.split(g.targets(i, KindDBPrimary, KindDBReplica))}}}
	case KindWorker:
		return plan{groups: []group{{1, g.split(g.targets(i, KindDBPrimary))}}}
	}
	return plan{local: 1}
}

// solve routes the current tick's traffic under the active events. It
// changes nothing in the world except each node's derived DownReplicas and
// Down, and returns the flow, each queue's backlog after the tick, and the
// share of attempts that failed.
// Attack traffic takes capacity like real traffic, but only real requests
// count towards success, errors, and revenue.
func (g *Game) solve() Snapshot {
	r := g.Rules
	g.fx = g.effects()
	for _, n := range g.Nodes {
		n.DownReplicas = min(n.Replicas, g.fx.down[n.ID])
		n.Down = n.Kind != KindInternet && n.DownReplicas == n.Replicas
	}
	rps := g.rps()
	attackRPS := rps * g.fx.attack
	order := g.order()
	dt := r.TickSeconds

	// Clients retry failed attempts. How often depends on last tick's
	// failure rate for each class, so a struggling system receives more load
	// the next tick, and a request that always fails is retried in full.
	groups := g.groups(rps)
	var unique, attempts, attemptStore vec
	for _, gl := range groups {
		for c := range nClass {
			amp := 0.0
			for k := 0; k <= gl.retries; k++ {
				amp += math.Pow(g.attemptFail[c], float64(k))
			}
			unique[c] += gl.load[c]
			attempts[c] += gl.load[c] * amp
			attemptStore[c] += gl.storage[c] * amp
		}
	}

	n := len(g.Nodes)
	load := make([]vec, n)
	store := make([]vec, n)
	offered := make([]float64, n)
	attack := make([]float64, n)
	served := make([]float64, n)
	blocked := make([]float64, n)
	backlog := make([]float64, n)
	plans := make([][nClass]plan, n)
	// Attack traffic asks for what real users ask for, and never retries.
	for c := range nClass {
		load[0][c] = attempts[c] + unique[c]*g.fx.attack
		if unique[c] > 0 {
			store[0][c] = load[0][c] * attemptStore[c] / attempts[c]
		}
	}
	attack[0] = attackRPS
	// share is the attack share of a node's offered load.
	share := func(i int) float64 {
		if offered[i] == 0 {
			return 0
		}
		return attack[i] / offered[i]
	}
	storeShare := func(i, c int) float64 {
		if load[i][c] > 0 {
			return store[i][c] / load[i][c]
		}
		if r.Traffic == nil {
			return r.StorageShare
		}
		return 0
	}

	// Pass 1: push load downstream in topological order.
	for _, i := range order {
		nd := g.Nodes[i]
		c := g.capacity(nd)
		offered[i] = load[i].sum()
		for cl := range nClass {
			// Only a CDN and an application treat classes differently.
			if cl == 0 || nd.Kind == KindCDN || nd.Kind == KindApp {
				plans[i][cl] = g.plan(i, cl, storeShare(i, cl))
			} else {
				plans[i][cl] = plans[i][0]
			}
		}
		if nd.Kind == KindQueue {
			k, _ := r.Kind(nd.Kind)
			workers := g.split(g.targets(i, KindWorker))
			drain := 0.0
			for _, w := range workers {
				drain += g.capacity(g.Nodes[w.to])
			}
			room := math.Max(0, k.MaxBacklog*float64(nd.Replicas)-nd.Backlog)
			if nd.Down {
				// A failed queue keeps its messages but neither takes nor
				// delivers any until it is back.
				room, drain = 0, 0
			}
			accepted := math.Min(offered[i], math.Min(c, drain+room/dt))
			drained := math.Min(drain, nd.Backlog/dt+accepted)
			backlog[i] = nd.Backlog + (accepted-drained)*dt
			served[i] = accepted
			for _, w := range workers {
				load[w.to][clsWrite] += drained * w.share
				attack[w.to] += drained * w.share * share(i)
			}
			plans[i] = [nClass]plan{{local: 1}, {local: 1}, {local: 1}}
			continue
		}
		served[i] = math.Min(offered[i], c)
		out, a := served[i], share(i)
		if nd.RateLimited {
			// A rate limit blocks most attack traffic and a few real users.
			stop := a*r.RateLimitBlock + (1-a)*r.RateLimitFalsePositive
			blocked[i] = served[i] * stop
			out = served[i] - blocked[i]
			if out > 0 {
				a = served[i] * a * (1 - r.RateLimitBlock) / out
			}
		}
		// Dropping and blocking take every class alike.
		keep := 0.0
		if offered[i] > 0 {
			keep = out / offered[i]
		}
		for cl := range nClass {
			p := plans[i][cl]
			v, st := load[i][cl]*keep, store[i][cl]*keep
			send := func(grp group, storage bool) {
				for _, rt := range grp.routes {
					f := grp.fraction * rt.share
					load[rt.to][cl] += v * f
					attack[rt.to] += v * f * a
					if storage {
						store[rt.to][cl] += st * f
					}
				}
			}
			// An application makes the storage fetch itself; other nodes
			// pass the need for it on.
			for _, grp := range p.groups {
				send(grp, p.storage == nil)
			}
			if p.storage != nil {
				send(*p.storage, false)
			}
		}
	}

	// Pass 2: from the leaves up, for each class, the chance an operation
	// entering a node succeeds (s) and the mean latency of those that do (t).
	s := make([]vec, n)
	t := make([]vec, n)
	stats := make([]NodeStats, n)
	maxU := 0.0
	for o := len(order) - 1; o >= 0; o-- {
		i := order[o]
		nd := g.Nodes[i]
		c := g.capacity(nd)
		own, u := 0.0, 0.0
		frac := 1.0
		if nd.Kind != KindInternet {
			k, _ := r.Kind(nd.Kind)
			if c > 0 {
				u = offered[i] / c
			} else if offered[i] > 0 {
				u = math.Inf(1)
			}
			own = math.Min(k.ServiceMs*factor(g.fx.slow, nd.ID)/(1-math.Min(u, 0.99)), r.TimeoutMs)
			if offered[i] > 0 {
				frac = served[i] / offered[i]
			} else if c == 0 {
				frac = 0
			}
			maxU = math.Max(maxU, u)
		}
		if nd.RateLimited {
			frac *= 1 - r.RateLimitFalsePositive
		}
		for cl := range nClass {
			p := plans[i][cl]
			succ, lat := p.local, 0.0
			for _, grp := range p.groups {
				gs, gt := 0.0, 0.0
				for _, rt := range grp.routes {
					gs += rt.share * s[rt.to][cl]
					gt += rt.share * s[rt.to][cl] * t[rt.to][cl]
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
					ss += rt.share * s[rt.to][cl]
					st += rt.share * s[rt.to][cl] * t[rt.to][cl]
				}
				if ss > 0 {
					st /= ss
				}
				succ *= (1 - p.storage.fraction) + p.storage.fraction*ss
				lat += p.storage.fraction * st
			}
			s[i][cl] = frac * succ
			t[i][cl] = own + lat
		}
		stats[i] = NodeStats{
			ID: nd.ID, Offered: offered[i], Served: served[i], Dropped: offered[i] - served[i],
			Attack: attack[i], Blocked: blocked[i],
			Capacity: finite(c), Utilization: finite(u), LatencyMs: own, Backlog: backlog[i],
		}
		if nd.Kind != KindInternet {
			stats[i].CostPerHour = g.costPerHour(nd)
		}
	}

	// An attempt's chance of success by class; a third-party outage fails
	// its share of requests whatever the design.
	var ok, fail vec
	okAll, latAll := 0.0, 0.0
	for cl := range nClass {
		ok[cl] = s[0][cl] * (1 - g.fx.failShare)
		if attempts[cl] > 0 {
			fail[cl] = 1 - ok[cl]
		}
		okAll += attempts[cl] * s[0][cl]
		latAll += attempts[cl] * s[0][cl] * t[0][cl]
	}
	mean := 0.0
	if okAll > 0 {
		mean = latAll / okAll
	}
	// A request fails only when every attempt its client makes fails.
	success := 0.0
	for _, gl := range groups {
		for cl := range nClass {
			success += gl.load[cl] * (1 - math.Pow(1-ok[cl], float64(gl.retries+1)))
		}
	}
	f := Flow{RPS: rps, AttackRPS: attackRPS, SuccessRPS: success, MeanLatencyMs: mean, MaxUtilization: finite(maxU), Nodes: stats}
	if rps > 0 {
		f.ErrorRate = 1 - success/rps
	}
	f.P95LatencyMs = math.Min(mean*p95Factor, r.TimeoutMs)
	f.Traffic = Traffic{Source: SourceMarket, RPS: rps, RetryRPS: attempts.sum() - unique.sum(), Concurrency: attempts.sum() * mean / 1000}
	if g.loadTest() {
		f.Traffic.Source = SourceConfigured
	}
	g.breakdown(&f.Traffic)
	return Snapshot{Flow: f, backlog: backlog, attemptFail: fail}
}

func finite(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
