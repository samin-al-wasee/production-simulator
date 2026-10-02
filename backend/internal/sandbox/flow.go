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
	// App is an application instance's runtime state (v5 and later).
	App *AppStats `json:"app,omitempty"`
	// Traffic is what a traffic component sent and how it fared (v6).
	Traffic *ClientStats `json:"traffic,omitempty"`
	// DB is a database's runtime state (v8); Cache a cache's (v9).
	DB    *DBStats    `json:"db,omitempty"`
	Cache *CacheStats `json:"cache,omitempty"`
	// Storage is object storage's (v10); Queue a message queue's (v11).
	Storage *StorageStats `json:"storage,omitempty"`
	Queue   *QueueStats   `json:"queue,omitempty"`
	// Stream is an event stream's (v12); Edge a load balancer's,
	// gateway's, or CDN's (v13).
	Stream *StreamStats   `json:"stream,omitempty"`
	Edge   *EdgeNodeStats `json:"edge,omitempty"`
	// Obs is what the player can see of a component, and Backend what a
	// telemetry backend took in (v14).
	Obs     *ObsStats     `json:"obs,omitempty"`
	Backend *BackendStats `json:"backend,omitempty"`
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
	// Edges is what each connection carried (v7).
	Edges []EdgeStats `json:"edges,omitempty"`
}

// p95Factor converts a mean latency to a p95 under an exponential latency
// distribution: -ln(0.05).
var p95Factor = -math.Log(0.05)

// capacity is the ops/s a node can serve this tick.
func (g *Game) capacity(n *Node) float64 {
	if n.Kind == KindInternet || n.Kind == KindTraffic {
		return math.Inf(1)
	}
	if c, ok := g.appCaps[n]; ok {
		return c
	}
	if c, ok := g.dbCaps[n]; ok {
		return c
	}
	if c, ok := g.cacheCaps[n]; ok {
		return c
	}
	if g.storageModel(n) {
		return float64(g.storageConfig(n).Prefixes) * g.Rules.StorageRuntime.GetsPerPrefix * float64(g.upReplicas(n))
	}
	if g.streamModel(n) {
		c, _, _ := g.streamCapacity(n)
		return c
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
		if run := g.cacheRuns[i]; run != nil {
			hit = run.hit
		}
		return plan{local: hit, groups: []group{{1 - hit, g.split(g.targets(i, KindDBPrimary, KindDBReplica))}}}
	case KindWorker:
		return plan{groups: []group{{1, g.split(g.targets(i, KindDBPrimary))}}}
	case KindTraffic:
		// Requests that break the contract are turned away at the door.
		for _, cl := range g.clients {
			if cl.node == i && cl.problem == "" {
				return plan{groups: []group{{1, []route{{cl.to, 1}}}}}
			}
		}
		return plan{}
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
		n.DownReplicas = n.Replicas - g.upReplicas(n)
		n.Down = n.Kind != KindInternet && n.DownReplicas == n.Replicas
	}
	g.clients = nil
	if g.clientModel() {
		g.clients = g.clientLoads()
	}
	g.problem, g.epLoad, g.edgeRun, g.edgeFailNext = nil, nil, nil, map[string]float64{}
	g.runs = make([]*appRun, len(g.Nodes))
	g.fwds, g.epOut, g.edgeRuns = nil, nil, nil
	if g.Rules.LB != nil {
		g.fwds = make([]map[string]fwd, len(g.Nodes))
		g.epOut = make([]map[string][2]float64, len(g.Nodes))
		g.edgeRuns = make([]*EdgeNodeStats, len(g.Nodes))
	}
	if g.callModel() {
		// Requests are carried per endpoint to every application (v7).
		g.problem = g.edgeProblems()
		g.edgeRun = map[string]*EdgeStats{}
		g.epLoad = make([]map[string]float64, len(g.Nodes))
		for i := range g.epLoad {
			g.epLoad[i] = map[string]float64{}
		}
		for _, cl := range g.clients {
			if cl.problem == "" {
				for _, e := range cl.cfg.Endpoints {
					g.epLoad[cl.to][e.Name] += (cl.attempts + cl.attack) * e.Share
				}
			}
		}
	}
	// An application's capacity depends on its configuration, replicas, and
	// last tick's dependency latency, none of which change during a solve.
	g.appCaps = map[*Node]float64{}
	g.dbCaps = map[*Node]float64{}
	g.cacheCaps = map[*Node]float64{}
	g.cacheRuns = nil
	for i, nd := range g.Nodes {
		if g.appModel(nd) {
			g.appCaps[nd] = g.appCapacity(i)
		}
		if g.dbModel(nd) {
			g.dbCaps[nd] = g.dbCapacity(i)
		}
		if g.cacheModel(nd) {
			mix := g.lastLoad[nd.ID]
			g.cacheCaps[nd] = g.runCache(i, mix).stats.Capacity
		}
	}
	rps := g.rps()
	attackRPS := rps * g.fx.attack
	if g.clientModel() {
		rps, attackRPS = 0, 0
		for _, cl := range g.clients {
			rps += cl.unique
			attackRPS += cl.attack
		}
	}
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
	apps := make([]*appRun, n)
	dbs := make([]*dbRun, n)
	caches := make([]*cacheRun, n)
	storages := make([]*storageRun, n)
	queues := make([]*QueueStats, n)
	streams := make([]*StreamStats, n)
	lagsNext := map[string]map[string]float64{}
	g.cacheRuns = caches
	// Attack traffic asks for what real users ask for, and never retries.
	if g.clientModel() {
		for _, cl := range g.clients {
			for _, e := range cl.cfg.Endpoints {
				load[cl.node][endpointClass(e.Name)] += (cl.attempts + cl.attack) * e.Share
			}
			attack[cl.node] = cl.attack
		}
	} else {
		for c := range nClass {
			load[0][c] = attempts[c] + unique[c]*g.fx.attack
			if unique[c] > 0 {
				store[0][c] = load[0][c] * attemptStore[c] / attempts[c]
			}
		}
		attack[0] = attackRPS
	}
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
		if g.cacheModel(nd) {
			run := g.runCache(i, load[i])
			caches[i] = &run
			g.cacheCaps[nd] = run.stats.Capacity
			c = run.stats.Capacity
		}
		if g.storageModel(nd) {
			run := g.runStorage(i, load[i])
			storages[i] = &run
		}
		offered[i] = load[i].sum()
		if g.dbModel(nd) {
			run := g.runDB(i, load[i])
			dbs[i] = &run
			served[i] = run.served
			backlog[i] = run.backlog
			plans[i] = [nClass]plan{{local: 1}, {local: 1}, {local: 1}}
			continue
		}
		if g.appModel(nd) {
			run := g.runApp(i, load[i])
			apps[i] = &run
			g.runs[i] = &run
			served[i] = run.served
			plans[i] = [nClass]plan{{local: 1}, {local: 1}, {local: 1}}
			a := share(i)
			for _, rr := range run.routes {
				// The server works on every request it accepts, even one
				// its client has given up on.
				v := rr.rate * run.served / (run.served + run.rejected)
				for _, c := range rr.calls {
					for _, x := range c.to {
						w := v * x.share
						if g.callModel() {
							w *= amplify(g.edge(nd.ID, g.Nodes[x.to].ID).Conn, g.edgeFail[nd.ID+">"+g.Nodes[x.to].ID])
							g.edgeStat(i, x.to).RPS += w
							// A refused call is attempted and never arrives.
							if g.problem[[2]int{i, x.to}] != "" {
								continue
							}
							if c.endpoint != "" {
								g.epLoad[x.to][c.endpoint] += w
							}
						}
						load[x.to][c.cls] += w
						attack[x.to] += w * a
					}
				}
			}
			continue
		}
		for cl := range nClass {
			// Only a CDN and an application treat classes differently.
			if cl == 0 || nd.Kind == KindCDN || nd.Kind == KindApp {
				plans[i][cl] = g.plan(i, cl, storeShare(i, cl))
			} else {
				plans[i][cl] = plans[i][0]
			}
		}
		if g.streamModel(nd) {
			// Every consumer group reads every event it can keep up with.
			accepted, out, st, lags := g.consume(i, offered[i])
			served[i] = accepted
			streams[i], lagsNext[nd.ID] = st, lags
			a := share(i)
			for j, v := range out {
				load[j][clsWrite] += v
				attack[j] += v * a
				if g.callModel() {
					g.edgeStat(i, j).RPS += v
					if g.Nodes[j].Kind == KindApp {
						g.epLoad[j][EventsEndpoint] += v
					}
				}
			}
			plans[i] = [nClass]plan{{local: 1}, {local: 1}, {local: 1}}
			continue
		}
		if nd.Kind == KindQueue {
			k, _ := r.Kind(nd.Kind)
			workers := g.split(g.targets(i, KindWorker))
			drain := 0.0
			for _, w := range workers {
				drain += g.capacity(g.Nodes[w.to])
			}
			maxBacklog := k.MaxBacklog * float64(nd.Replicas)
			amp, qc := 1.0, (*QueueConfig)(nil)
			if g.queueModel(nd) {
				// Messages the workers failed come back until they run out
				// of deliveries (v11).
				qc = g.queueConfig(nd)
				maxBacklog = qc.MaxBacklog * float64(nd.Replicas)
				amp = redelivery(g.queueFail[nd.ID], qc.MaxDeliveries)
			}
			room := math.Max(0, maxBacklog-nd.Backlog)
			if nd.Down {
				// A failed queue keeps its messages but neither takes nor
				// delivers any until it is back.
				room, drain = 0, 0
			}
			accepted := math.Min(offered[i], math.Min(c, drain+room/dt))
			if qc != nil {
				accepted = math.Min(offered[i], math.Min(c, drain+room/dt)/amp)
			}
			inflow := accepted * amp
			drained := math.Min(drain, nd.Backlog/dt+inflow)
			backlog[i] = nd.Backlog + (inflow-drained)*dt
			served[i] = accepted
			if qc != nil {
				f := g.queueFail[nd.ID]
				dead := accepted * math.Pow(f, float64(qc.MaxDeliveries))
				queues[i] = &QueueStats{Published: offered[i], Rejected: offered[i] - accepted, Delivered: drained,
					Redelivered: inflow - accepted, DeadLettered: dead, DeadLetters: nd.DeadLetters + dead*dt,
					Backlog: backlog[i], MaxBacklog: maxBacklog, WorkerFailure: f}
				if drained > 0 {
					queues[i].DelaySeconds = backlog[i] / drained
				}
			}
			for _, w := range workers {
				load[w.to][clsWrite] += drained * w.share
				attack[w.to] += drained * w.share * share(i)
				if g.queueModel(nd) {
					g.edgeStat(i, w.to).RPS += drained * w.share
				}
			}
			plans[i] = [nClass]plan{{local: 1}, {local: 1}, {local: 1}}
			continue
		}
		served[i] = math.Min(offered[i], c)
		// A gateway answers requests above its rate limit with 429 (v13).
		limited := 0.0
		if g.edgeModel(nd) && nd.Kind == KindGateway {
			if lim := g.gatewayConfig(nd).RateLimitRPS; lim > 0 && served[i] > lim {
				limited, served[i] = served[i]-lim, lim
			}
		}
		out, a := served[i], share(i)
		if run := caches[i]; run != nil {
			// Refused connections never reach the database behind.
			out *= 1 - run.refused
		}
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
		if g.edgeModel(nd) {
			// From v13 the edge forwards endpoint by endpoint.
			g.forward(i, keep, a, load, attack)
			g.edgeRuns[i].Limited = limited
			continue
		}
		for cl := range nClass {
			p := plans[i][cl]
			v, st := load[i][cl]*keep, store[i][cl]*keep
			send := func(grp group, storage bool) {
				for _, rt := range grp.routes {
					// A connection that breaks its contract carries nothing.
					if g.problem[[2]int{i, rt.to}] != "" {
						continue
					}
					f := grp.fraction * rt.share
					load[rt.to][cl] += v * f
					attack[rt.to] += v * f * a
					if storage {
						store[rt.to][cl] += st * f
					}
					if g.callModel() && nd.Kind != KindTraffic {
						g.edgeStat(i, rt.to).RPS += v * f
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
	clientAt := map[int]int{}
	for k, cl := range g.clients {
		clientAt[cl.node] = k
	}
	clientOK := make([]float64, len(g.clients))
	clientLat := make([]float64, len(g.clients))
	for o := len(order) - 1; o >= 0; o-- {
		i := order[o]
		nd := g.Nodes[i]
		if run := dbs[i]; run != nil {
			for cl := range nClass {
				s[i][cl], t[i][cl] = run.ok, run.lat[cl]
			}
			st := run.stats
			u := 0.0
			if st.Capacity > 0 {
				u = offered[i] / st.Capacity
			} else if offered[i] > 0 {
				u = math.Inf(1)
			}
			stats[i] = NodeStats{ID: nd.ID, Offered: offered[i], Served: run.served, Dropped: offered[i] - run.served, Attack: attack[i],
				Capacity: st.Capacity, Utilization: finite(u), LatencyMs: st.ReadMs, CostPerHour: g.costPerHour(nd), DB: &run.stats}
			maxU = math.Max(maxU, u)
			continue
		}
		if k, ok := clientAt[i]; ok {
			cs := ClientStats{}
			clientOK[k], clientLat[k], cs = g.finishClient(g.clients[k], apps)
			stats[i] = NodeStats{ID: nd.ID, Offered: offered[i], Served: served[i], Attack: attack[i], LatencyMs: clientLat[k], Traffic: &cs}
			continue
		}
		if run := apps[i]; run != nil {
			s[i], t[i] = g.finishApp(nd, run, s, t)
			own := 0.0
			for _, rr := range run.routes {
				own += rr.rate * (rr.own + run.waitMs)
			}
			if lambda := run.served + run.rejected; lambda > 0 {
				own /= lambda
			}
			stats[i] = NodeStats{
				ID: nd.ID, Offered: offered[i], Served: run.served, Dropped: run.rejected, Attack: attack[i],
				Capacity: finite(run.capacity), Utilization: finite(run.rho), LatencyMs: own,
				CostPerHour: g.costPerHour(nd), App: &run.stats,
			}
			maxU = math.Max(maxU, run.rho)
			continue
		}
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
			if run := caches[i]; run != nil {
				own = run.lat
				frac *= 1 - run.refused
			}
			if run := storages[i]; run != nil {
				own = run.lat
			}
			maxU = math.Max(maxU, u)
		}
		if nd.RateLimited {
			frac *= 1 - r.RateLimitFalsePositive
		}
		if g.edgeModel(nd) {
			if nd.Kind == KindGateway && g.gatewayConfig(nd).Auth {
				own += r.EdgeRuntime.AuthMs
			}
			s[i], t[i] = g.finishEdge(i, frac, own)
		}
		for cl := range nClass {
			if g.edgeModel(nd) {
				break
			}
			p := plans[i][cl]
			succ, lat := p.local, 0.0
			for _, grp := range p.groups {
				gs, gt := 0.0, 0.0
				for _, rt := range grp.routes {
					if g.problem[[2]int{i, rt.to}] != "" {
						continue
					}
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
		if run := caches[i]; run != nil {
			stats[i].Cache = &run.stats
		}
		if nd.Kind != KindInternet {
			stats[i].CostPerHour = g.costPerHour(nd)
		}
		if run := storages[i]; run != nil {
			// Object storage is priced by use, not by replica (v10).
			stats[i].Storage, stats[i].CostPerHour = &run.stats, run.cost
		}
		if g.edgeModel(nd) {
			stats[i].Edge = g.edgeRuns[i]
			if nd.Kind == KindCDN {
				// A CDN is priced by use (v13).
				stats[i].CostPerHour = g.edgeRuns[i].Cost
			}
		}
		if st := streams[i]; st != nil {
			stats[i].Stream = st
		}
		if q := queues[i]; q != nil {
			switch fail := q.Rejected / math.Max(q.Published, 1e-300); {
			case nd.Down:
				q.Health = HealthStopped
			case fail > 0.2:
				q.Health = HealthUnhealthy
			case fail > 0.01 || q.DeadLettered > 0.01 || q.Backlog > 0.85*q.MaxBacklog:
				q.Health = HealthDegraded
			default:
				q.Health = HealthHealthy
			}
			stats[i].Queue = q
		}
	}

	// An attempt's chance of success by class; a third-party outage fails
	// its share of requests whatever the design.
	var ok, fail vec
	okAll, latAll := 0.0, 0.0
	for cl := range nClass {
		if g.clientModel() {
			break
		}
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
	clientFail := map[string]float64{}
	if g.clientModel() {
		// Every traffic component adds to the totals (ADR-0018).
		success, okAll, latAll = 0, 0, 0
		for k, cl := range g.clients {
			ok := clientOK[k] * (1 - g.fx.failShare)
			if cl.attempts > 0 {
				clientFail[g.Nodes[cl.node].ID] = 1 - ok
			}
			okAll += cl.attempts * clientOK[k]
			latAll += cl.attempts * clientOK[k] * clientLat[k]
			ts := stats[cl.node].Traffic
			ts.Success = cl.unique * (1 - math.Pow(1-ok, float64(cl.cfg.Retries+1)))
			ts.Concurrency = cl.attempts * clientLat[k] / 1000
			success += ts.Success
		}
		mean = 0
		if okAll > 0 {
			mean = latAll / okAll
		}
		for _, cl := range g.clients {
			unique[clsRead] += cl.unique
			attempts[clsRead] += cl.attempts
		}
	}
	if g.telemetryModel() {
		g.observe(stats)
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
	if g.callModel() {
		for k, cl := range g.clients {
			if cl.to < 0 {
				continue
			}
			ts := stats[cl.node].Traffic
			es := g.edgeStat(cl.node, cl.to)
			es.RPS, es.RetryRPS, es.LatencyMs = cl.attempts+cl.attack, cl.attempts-cl.unique, clientLat[k]
			es.ok, es.lat, es.weight = clientOK[k], clientLat[k], 1
			es.Problem = ts.Problem
		}
		f.Edges = g.edgeStats(load, s, t)
	}
	snap := Snapshot{Flow: f, backlog: backlog, attemptFail: fail, clientFail: clientFail, edgeFail: g.edgeFailNext, epPath: map[string]map[string]float64{}, path: map[string]vec{}}
	if g.Rules.Stream != nil {
		snap.lags = lagsNext
	}
	if g.Rules.Queue != nil {
		// Each queue learns how often its workers failed this tick.
		snap.queueFail, snap.deadLetters = map[string]float64{}, map[string]float64{}
		for i, nd := range g.Nodes {
			if q := queues[i]; q != nil {
				snap.deadLetters[nd.ID] = q.DeadLetters
				f := 0.0
				for _, w := range g.split(g.targets(i, KindWorker)) {
					if run := apps[w.to]; run != nil {
						if lambda := run.served + run.rejected; lambda > 0 {
							f += w.share * (1 - run.stats.Success/lambda)
						}
					}
				}
				snap.queueFail[nd.ID] = f
			}
		}
	}
	if g.Rules.Cache != nil {
		snap.warmth = map[string]float64{}
		for i, nd := range g.Nodes {
			if run := caches[i]; run != nil {
				snap.warmth[nd.ID] = run.warmth
			}
		}
	}
	if g.Rules.DB != nil {
		snap.load, snap.writes = map[string]vec{}, map[string]float64{}
		for i, nd := range g.Nodes {
			snap.load[nd.ID] = load[i]
			if run := dbs[i]; run != nil && nd.Kind == KindDBPrimary && offered[i] > 0 {
				snap.writes[nd.ID] = load[i][clsWrite] * run.served / offered[i]
			}
		}
	}
	for i, run := range g.runs {
		if run != nil && run.out != nil {
			ms := map[string]float64{}
			for name, o := range run.out {
				ms[name] = o[1]
			}
			snap.epPath[g.Nodes[i].ID] = ms
		}
	}
	for i, nd := range g.Nodes {
		snap.path[nd.ID] = t[i]
		if apps[i] != nil && apps[i].stats.OutOfMemory {
			snap.outOfMemory = append(snap.outOfMemory, nd.ID)
		}
	}
	return snap
}

func finite(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0
	}
	return v
}
