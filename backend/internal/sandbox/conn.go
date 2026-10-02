package sandbox

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Listener is what a component accepts connections on (v7, ADR-0019).
type Listener struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	TLS      bool   `json:"tls"`
}

// Connection is the client side of an edge: what the caller speaks, how
// many connections each of its replicas keeps open, how long it waits for a
// call, and how often it retries a failed one. It adopts the target's
// listener on connect and follows it when the target changes.
type Connection struct {
	Protocol  string  `json:"protocol"`
	Port      int     `json:"port"`
	TLS       bool    `json:"tls"`
	Pool      int     `json:"pool"`
	TimeoutMs float64 `json:"timeoutMs"`
	Retries   int     `json:"retries,omitempty"`
}

// Call is a route's request to another service's endpoint. An async call is
// sent and not waited for: it loads the service, but its outcome and
// latency do not reach the caller.
type Call struct {
	Service  string `json:"service"`
	Endpoint string `json:"endpoint"`
	Async    bool   `json:"async,omitempty"`
}

// EdgeStats is what one connection carried during a tick: attempts per
// second, failed attempts, and the mean latency of a call over it.
type EdgeStats struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	RPS       float64 `json:"rps"`
	Errors    float64 `json:"errors"`
	RetryRPS  float64 `json:"retryRps,omitempty"`
	LatencyMs float64 `json:"latencyMs"`
	Problem   string  `json:"problem,omitempty"`
	// ok and lat sum each call's attempt success and latency, weighted by
	// weight, until the solve ends.
	ok, lat, weight float64
}

const maxCalls = 8

// callModel reports whether connections have listeners and clients, and
// routes may call services (v7 and later).
func (g *Game) callModel() bool {
	return g.Rules.Listeners != nil
}

// listener is what a node accepts: an application's own configuration, a
// node's override, or its kind's default. ok is false for a kind that
// takes no connections.
func (g *Game) listener(n *Node) (Listener, bool) {
	if g.appModel(n) {
		a := g.appConfig(n)
		return Listener{a.Protocol, a.Port, a.TLS}, true
	}
	if n.Listener != nil {
		return *n.Listener, true
	}
	l, ok := g.Rules.Listeners[n.Kind]
	return l, ok
}

// edge returns the edge from one node to another.
func (g *Game) edge(from, to string) *Edge {
	for i := range g.Edges {
		if g.Edges[i].From == from && g.Edges[i].To == to {
			return &g.Edges[i]
		}
	}
	return nil
}

// adoptConn sets an edge's client side to its target's listener, keeping
// the pool, timeout, and retries already chosen.
func (g *Game) adoptConn(e *Edge, to *Node) {
	l, ok := g.listener(to)
	if !ok {
		return
	}
	c := g.Rules.ConnDefaults
	if e.Conn != nil {
		c = *e.Conn
	}
	c.Protocol, c.Port, c.TLS = l.Protocol, l.Port, l.TLS
	e.Conn = &c
}

// follow re-adopts every connection to a node whose listener changed.
func (g *Game) follow(n *Node) {
	if !g.callModel() {
		return
	}
	for i := range g.Edges {
		if e := &g.Edges[i]; e.To == n.ID && e.Conn != nil {
			g.adoptConn(e, n)
		}
	}
}

// connProblem is why calls over an edge cannot reach its target, or "".
func (g *Game) connProblem(e *Edge, to *Node) string {
	l, ok := g.listener(to)
	if e.Conn == nil || !ok {
		return ""
	}
	c := e.Conn
	switch {
	case c.Protocol != l.Protocol:
		return fmt.Sprintf("protocol error: %s client, %s server", c.Protocol, l.Protocol)
	case c.Port != l.Port:
		return fmt.Sprintf("connection refused: port %d, %s listens on %d", c.Port, to.ID, l.Port)
	case c.TLS && !l.TLS:
		return fmt.Sprintf("TLS handshake failed: %s has no TLS", to.ID)
	case !c.TLS && l.TLS:
		return fmt.Sprintf("TLS required: %s only accepts TLS", to.ID)
	}
	return ""
}

// problems are the contract problems of every edge this solve, by node
// indexes.
func (g *Game) edgeProblems() map[[2]int]string {
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	out := map[[2]int]string{}
	for i := range g.Edges {
		e := &g.Edges[i]
		if p := g.connProblem(e, g.Nodes[idx[e.To]]); p != "" {
			out[[2]int{idx[e.From], idx[e.To]}] = p
		}
	}
	return out
}

// callOK is the chance a call over a connection succeeds when the target
// succeeds with chance p at mean latency ms: within the connection's
// timeout (latency taken as exponential), then after its retries.
func callOK(c *Connection, p, ms float64) (attempt, call float64) {
	attempt = p
	if c != nil && ms > 0 {
		attempt *= 1 - math.Exp(-c.TimeoutMs/ms)
	}
	call = attempt
	if c != nil && c.Retries > 0 {
		call = 1 - math.Pow(1-attempt, float64(c.Retries+1))
	}
	return attempt, call
}

// amplify is how many attempts one call makes over a connection given last
// tick's attempt failure rate f.
func amplify(c *Connection, f float64) float64 {
	amp := 1.0
	if c == nil {
		return amp
	}
	for k := 1; k <= c.Retries; k++ {
		amp += math.Pow(f, float64(k))
	}
	return amp
}

// configureConn replaces the client side of an edge.
func (g *Game) configureConn(from, to string, c *Connection) error {
	e := g.edge(from, to)
	if e == nil || e.Conn == nil {
		return invalid("%s → %s has no connection to configure", from, to)
	}
	if c == nil {
		return invalid("configure needs a connection")
	}
	if p := g.Rules.ValidateConn(*c); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	e.Conn = c
	delete(g.edgeFail, from+">"+to)
	return nil
}

// ValidateConn lists every problem with a connection; none means valid.
func (r *Ruleset) ValidateConn(c Connection) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if !slices.Contains(r.WireProtocols, c.Protocol) {
		bad("protocol must be one of %s", strings.Join(r.WireProtocols, ", "))
	}
	if c.Port < 1 || c.Port > 65535 {
		bad("port must be between 1 and 65535")
	}
	if c.Pool < 1 || c.Pool > 1000 {
		bad("pool must be between 1 and 1000 connections")
	}
	if !(c.TimeoutMs >= 1 && c.TimeoutMs <= 60_000) {
		bad("timeout must be between 1 and 60000 ms")
	}
	if c.Retries < 0 || c.Retries > r.MaxRetries {
		bad("retries must be between 0 and %d", r.MaxRetries)
	}
	return p
}

// ValidateListener lists every problem with a listener.
func (r *Ruleset) ValidateListener(l Listener) []string {
	var p []string
	if !slices.Contains(r.WireProtocols, l.Protocol) {
		p = append(p, fmt.Sprintf("protocol must be one of %s", strings.Join(r.WireProtocols, ", ")))
	}
	if l.Port < 1 || l.Port > 65535 {
		p = append(p, "port must be between 1 and 65535")
	}
	return p
}

// configureListener replaces what a component accepts; its callers follow.
func (g *Game) configureListener(n *Node, l *Listener) error {
	if _, ok := g.Rules.Listeners[n.Kind]; !ok {
		return invalid("%s has no listener to configure", n.Kind)
	}
	if l == nil {
		return invalid("configure needs a listener")
	}
	if p := g.Rules.ValidateListener(*l); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.Listener = l
	g.follow(n)
	return nil
}

// RulesetV7 is RulesetV6 with connections and inter-service calls (Phase 11,
// ADR-0019): every component listens on a protocol and port, every edge has
// a client side, and routes may call other services' endpoints.
func RulesetV7() *Ruleset {
	r := RulesetV6()
	r.Version = "sandbox/v7"
	r.WireProtocols = []string{"HTTP/1.1", "HTTP/2", "gRPC", "SQL", "RESP", "S3", "AMQP"}
	r.Listeners = map[string]Listener{
		KindDBPrimary: {"SQL", 5432, false},
		KindDBReplica: {"SQL", 5432, false},
		KindCache:     {"RESP", 6379, false},
		KindStorage:   {"S3", 443, true},
		KindQueue:     {"AMQP", 5672, false},
	}
	r.ConnDefaults = Connection{Pool: 20, TimeoutMs: 1000}
	r.NetworkHopMs = 0.5
	for i := range r.Kinds {
		if r.Kinds[i].Name == KindApp {
			r.Kinds[i].ConnectsTo = append(r.Kinds[i].ConnectsTo, KindApp)
		}
	}
	r.AppTypes = append(r.AppTypes, microservices()...)
	return r
}

// services are the connected application instances a route's call to a
// named service can reach: service discovery by name.
func (g *Game) services(i int, name string) []int {
	var out []int
	for _, j := range g.targets(i, KindApp) {
		if g.appConfig(g.Nodes[j]).Name == name {
			out = append(out, j)
		}
	}
	return out
}

// lastCall is a call's latency over one target last tick: the service's
// endpoint, or the target's request class.
func (g *Game) lastCall(c callRun, to int) float64 {
	id := g.Nodes[to].ID
	if c.endpoint != "" {
		return g.lastEp[id][c.endpoint]
	}
	return g.lastPath[id][c.cls]
}

// callWall adds to a route's wall time what its calls cost it from last
// tick's latencies: each target's latency plus the network hop, at most the
// connection's timeout, or only the hop when the target refuses it or the
// call is async. It also records the connection time each call holds.
func (g *Game) callWall(i int, rr *routeRun) {
	hop := g.Rules.NetworkHopMs
	rr.pool = map[int]float64{}
	from := g.Nodes[i].ID
	for _, c := range rr.calls {
		for _, x := range c.to {
			ms := hop
			if !c.async && g.problem[[2]int{i, x.to}] == "" {
				ms += g.lastCall(c, x.to)
				if e := g.edge(from, g.Nodes[x.to].ID); e != nil && e.Conn != nil {
					ms = math.Min(ms, e.Conn.TimeoutMs)
				}
			}
			rr.wall += x.share * ms
			rr.pool[x.to] += x.share * ms / 1000
		}
	}
}

// finishCall is a call's chance to succeed and its latency, once its
// targets are solved: refused by a broken contract, else the target's
// outcome over the network hop within the connection's timeout and after
// its retries. An async call always "succeeds" for the caller and costs it
// only the hop. Each connection's stats are recorded on the way.
func (g *Game) finishCall(n *Node, rr *routeRun, c callRun, s, t []vec) (float64, float64) {
	hop := g.Rules.NetworkHopMs
	i := slices.Index(g.Nodes, n)
	ds, dt := 0.0, 0.0
	for _, x := range c.to {
		key := n.ID + ">" + g.Nodes[x.to].ID
		es := g.edgeStat(i, x.to)
		sent := rr.rate * x.share
		es.weight += sent
		var p, ms float64
		switch {
		case g.problem[[2]int{i, x.to}] != "":
			es.Problem = g.problem[[2]int{i, x.to}]
			continue
		case c.endpoint != "":
			o := g.runs[x.to].out[c.endpoint]
			p, ms = o[0], o[1]
		default:
			p, ms = s[x.to][c.cls], t[x.to][c.cls]
		}
		ms += hop
		conn := g.edge(n.ID, g.Nodes[x.to].ID).Conn
		attempt, call := callOK(conn, p, ms)
		g.edgeFailNext[key] = 1 - attempt
		es.ok += sent * attempt
		es.lat += sent * ms
		if conn != nil {
			ms = math.Min(ms, conn.TimeoutMs)
		}
		if c.async {
			call, ms = 1, hop
		}
		ds += x.share * call
		dt += x.share * call * ms
	}
	if len(c.to) == 0 && c.async {
		return 1, 0
	}
	if ds > 0 {
		dt /= ds
	}
	return ds, dt
}

// edgeStat is the stats of the connection from node i to node j this solve.
func (g *Game) edgeStat(i, j int) *EdgeStats {
	key := g.Nodes[i].ID + ">" + g.Nodes[j].ID
	es, ok := g.edgeRun[key]
	if !ok {
		es = &EdgeStats{From: g.Nodes[i].ID, To: g.Nodes[j].ID}
		g.edgeRun[key] = es
	}
	return es
}

// edgeStats lists every connection's stats in edge order. Calls from an
// application carry their own outcomes; any other connection fares as its
// target does for the load it carried.
func (g *Game) edgeStats(load []vec, s, t []vec) []EdgeStats {
	idx := map[string]int{}
	for i, n := range g.Nodes {
		idx[n.ID] = i
	}
	var out []EdgeStats
	for _, e := range g.Edges {
		es, ok := g.edgeRun[e.From+">"+e.To]
		if !ok {
			es = &EdgeStats{From: e.From, To: e.To}
		}
		if p := g.problem[[2]int{idx[e.From], idx[e.To]}]; p != "" {
			es.Problem = p
		}
		switch {
		case es.Problem != "":
			es.Errors = es.RPS
		case es.weight > 0:
			es.Errors = es.RPS * (1 - es.ok/es.weight)
			es.LatencyMs = es.lat / es.weight
		default:
			j := idx[e.To]
			okW, latW := 0.0, 0.0
			for c := range nClass {
				okW += load[j][c] * s[j][c]
				latW += load[j][c] * s[j][c] * t[j][c]
			}
			if total := load[j].sum(); total > 0 {
				es.Errors = es.RPS * (1 - okW/total)
			}
			if okW > 0 {
				es.LatencyMs = latW / okW
			}
		}
		out = append(out, *es)
	}
	return out
}

// epMix splits each request class at application i into endpoints, in the
// shares they arrive this solve from traffic and from other services (v7),
// in name order. Without any it is measured at its own routes' shares.
func (g *Game) epMix(i int) ([]mixEntry, vec) {
	w := g.epLoad[i]
	names := make([]string, 0, len(w))
	for name, v := range w {
		if v > 0 {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		w = map[string]float64{}
		for _, e := range routeEndpoints(g.appConfig(g.Nodes[i])) {
			w[e.Name] += e.Share
			names = append(names, e.Name)
		}
	}
	slices.Sort(names)
	names = slices.Compact(names)
	var total vec
	for _, name := range names {
		total[endpointClass(name)] += w[name]
	}
	var out []mixEntry
	for _, name := range names {
		c := endpointClass(name)
		out = append(out, mixEntry{name, c, w[name] / total[c]})
	}
	sum := total.sum()
	for c := range total {
		total[c] /= sum
	}
	return out, total
}
