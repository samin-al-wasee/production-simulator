package sandbox

import (
	"math"
	"math/rand/v2"
)

// Card effects: which model input an event changes while it is active.
const (
	EffectTraffic    = "traffic"     // real traffic × magnitude
	EffectAttack     = "attack"      // attack traffic = magnitude × real traffic
	EffectCrash      = "crash"       // one replica of one component is down
	EffectZone       = "zone"        // a share (magnitude) of every component's replicas is down
	EffectSlowdown   = "slowdown"    // service time × magnitude, capacity ÷ magnitude
	EffectHitRatio   = "hit-ratio"   // a cache's hit ratio becomes magnitude
	EffectCapacity   = "capacity"    // a component's capacity × magnitude
	EffectCost       = "cost"        // one kind's running cost × magnitude
	EffectThirdParty = "third-party" // a share (magnitude) of requests fails
)

// Card drivers: the world value that raises a card's odds.
const (
	DriverPopularity = "popularity"
	DriverComplexity = "complexity"
)

// Event phases and outcomes.
const (
	PhaseUpcoming   = "upcoming"
	PhaseActive     = "active"
	PhaseRecovering = "recovering"
	PhaseOver       = "over"

	OutcomeRecovered   = "recovered"
	OutcomeUnrecovered = "unrecovered"
)

// Respond actions.
const (
	ActRestart       = "restart"
	ActFailover      = "failover"
	ActRateLimit     = "rate-limit"
	ActLiftRateLimit = "lift-rate-limit"
)

// Card is one kind of event in the deck.
type Card struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Effect string `json:"effect"`
	// PerDay is the expected number of draws per simulated day at a driver
	// factor of 1.
	PerDay float64 `json:"perDay"`
	Driver string  `json:"driver,omitempty"`
	// Targets lists the kinds the card can hit; empty means any placed kind.
	Targets      []string `json:"targets,omitempty"`
	MinTicks     int      `json:"minTicks"`
	MaxTicks     int      `json:"maxTicks"`
	MinMagnitude float64  `json:"minMagnitude"`
	MaxMagnitude float64  `json:"maxMagnitude"`
	// LeadTicks announces the event this long before it starts.
	LeadTicks int `json:"leadTicks,omitempty"`
}

// Hit is a number of replicas an event takes down on one component.
type Hit struct {
	Node     string `json:"node"`
	Replicas int    `json:"replicas"`
}

// Event is a drawn card. It is active for ticks in [Start, End) and judged
// RecoveryTicks after it ends.
type Event struct {
	ID        int     `json:"id"`
	Card      string  `json:"card"`
	Label     string  `json:"label"`
	Effect    string  `json:"effect"`
	Start     int     `json:"start"`
	End       int     `json:"end"`
	Magnitude float64 `json:"magnitude"`
	// Target is a node ID, or a kind for a cost event.
	Target       string  `json:"target,omitempty"`
	Hits         []Hit   `json:"hits,omitempty"`
	Phase        string  `json:"phase"`
	LowestHealth float64 `json:"lowestHealth"`
	Outcome      string  `json:"outcome,omitempty"`
	// LoadTest marks an event that ran into a load test before it was
	// judged; its outcome does not count towards goals.
	LoadTest bool `json:"loadTest,omitempty"`
}

// settledKept is how many judged events a game keeps for its feed.
const settledKept = 30

// RulesetV2 is RulesetV1 with the event deck, incident responses, and the
// economy rebalanced for them.
func RulesetV2() *Ruleset {
	r := RulesetV1()
	r.Version = "sandbox/v2"
	// Capacity was so cheap next to revenue that over-provisioning always
	// paid; at this rate a design with about 1.5× headroom earns the most, and
	// the thinner early margin needs a larger starting balance.
	r.RevenuePerRequest = 0.00015
	r.StartingCash = 1500
	r.EventGraceTicks = 288
	r.MaxActiveEvents = 3
	r.RecoveryTicks = 12
	r.RecoveredHealth = 80
	r.RestartTicks = 2
	r.RateLimitBlock = 0.9
	r.RateLimitFalsePositive = 0.01
	r.Events = []Card{
		{Name: "viral-surge", Label: "Viral surge", Effect: EffectTraffic, PerDay: 0.15, Driver: DriverPopularity,
			MinTicks: 24, MaxTicks: 72, MinMagnitude: 3, MaxMagnitude: 10},
		{Name: "marketing-spike", Label: "Marketing spike", Effect: EffectTraffic, PerDay: 0.1,
			MinTicks: 36, MaxTicks: 72, MinMagnitude: 2, MaxMagnitude: 3, LeadTicks: 24},
		{Name: "seasonal-dip", Label: "Seasonal dip", Effect: EffectTraffic, PerDay: 0.1,
			MinTicks: 288, MaxTicks: 288, MinMagnitude: 0.5, MaxMagnitude: 0.7},
		{Name: "instance-crash", Label: "Instance crash", Effect: EffectCrash, PerDay: 0.4, Driver: DriverComplexity,
			MinTicks: 12, MaxTicks: 36, MinMagnitude: 1, MaxMagnitude: 1},
		{Name: "zone-outage", Label: "Zone outage", Effect: EffectZone, PerDay: 0.05, Driver: DriverComplexity,
			MinTicks: 12, MaxTicks: 48, MinMagnitude: 1.0 / 3, MaxMagnitude: 1.0 / 3},
		{Name: "db-slowdown", Label: "Database slowdown", Effect: EffectSlowdown, PerDay: 0.15, Driver: DriverComplexity,
			Targets: []string{KindDBPrimary, KindDBReplica}, MinTicks: 12, MaxTicks: 36, MinMagnitude: 2, MaxMagnitude: 4},
		{Name: "cache-stampede", Label: "Cache stampede", Effect: EffectHitRatio, PerDay: 0.15, Driver: DriverComplexity,
			Targets: []string{KindCache}, MinTicks: 6, MaxTicks: 24, MinMagnitude: 0.1, MaxMagnitude: 0.3},
		{Name: "queue-backlog", Label: "Queue backlog", Effect: EffectCapacity, PerDay: 0.15, Driver: DriverComplexity,
			Targets: []string{KindWorker}, MinTicks: 12, MaxTicks: 36, MinMagnitude: 0.2, MaxMagnitude: 0.4},
		{Name: "ddos", Label: "DDoS attack", Effect: EffectAttack, PerDay: 0.1, Driver: DriverPopularity,
			MinTicks: 12, MaxTicks: 48, MinMagnitude: 3, MaxMagnitude: 10},
		{Name: "cost-spike", Label: "Cost spike", Effect: EffectCost, PerDay: 0.05,
			MinTicks: 288, MaxTicks: 864, MinMagnitude: 1.5, MaxMagnitude: 2.5},
		{Name: "third-party-outage", Label: "Third-party outage", Effect: EffectThirdParty, PerDay: 0.08,
			MinTicks: 12, MaxTicks: 36, MinMagnitude: 0.1, MaxMagnitude: 0.3},
	}
	return r
}

// effects are the model inputs changed by the events active this tick.
type effects struct {
	traffic   float64
	attack    float64
	failShare float64
	down      map[string]int
	slow      map[string]float64
	hitRatio  map[string]float64
	capacity  map[string]float64
	cost      map[string]float64
}

func (g *Game) active(e *Event) bool {
	return g.Tick >= e.Start && g.Tick < e.End
}

func (g *Game) effects() effects {
	fx := effects{
		traffic: 1, down: map[string]int{}, slow: map[string]float64{},
		hitRatio: map[string]float64{}, capacity: map[string]float64{}, cost: map[string]float64{},
	}
	for _, e := range g.Events {
		if !g.active(e) {
			continue
		}
		switch e.Effect {
		case EffectTraffic:
			fx.traffic *= e.Magnitude
		case EffectAttack:
			fx.attack += e.Magnitude
		case EffectCrash, EffectZone:
			for _, h := range e.Hits {
				fx.down[h.Node] += h.Replicas
			}
		case EffectSlowdown:
			fx.slow[e.Target] = factor(fx.slow, e.Target) * e.Magnitude
		case EffectHitRatio:
			fx.hitRatio[e.Target] = e.Magnitude
		case EffectCapacity:
			fx.capacity[e.Target] = factor(fx.capacity, e.Target) * e.Magnitude
		case EffectCost:
			fx.cost[e.Target] = factor(fx.cost, e.Target) * e.Magnitude
		case EffectThirdParty:
			fx.failShare = 1 - (1-fx.failShare)*(1-e.Magnitude)
		}
	}
	return fx
}

// factor is a multiplier from an effect map; 1 when the key is unaffected.
func factor(m map[string]float64, key string) float64 {
	if v, ok := m[key]; ok {
		return v
	}
	return 1
}

// upReplicas is how many of a node's replicas are serving this tick.
func (g *Game) upReplicas(n *Node) int {
	if g.Tick < n.CrashedUntil {
		return 0
	}
	return max(0, n.Replicas-g.fx.down[n.ID])
}

// driver is how much the world raises a card's odds, from 0 to 3.
func (g *Game) driver(name string) float64 {
	switch name {
	case DriverPopularity:
		return clamp(g.Popularity/50, 0, 3)
	case DriverComplexity:
		return clamp(g.Complexity()/10, 0, 3)
	}
	return 1
}

// pending counts events that have not ended, including announced ones.
func (g *Game) pending(card string) int {
	n := 0
	for _, e := range g.Events {
		if g.Tick < e.End && (card == "" || e.Card == card) {
			n++
		}
	}
	return n
}

// draw deals this tick's events. Each tick has its own random stream derived
// from the seed, so a replay deals the same cards whatever else happened.
func (g *Game) draw() {
	r := g.Rules
	if len(r.Events) == 0 || g.Tick < r.EventGraceTicks {
		return
	}
	g.fx = g.effects()
	rng := rand.New(rand.NewPCG(uint64(g.Seed), uint64(g.Tick)))
	perTick := r.TickSeconds / 86400
	for _, c := range r.Events {
		roll := rng.Float64()
		if roll >= c.PerDay*perTick*g.driver(c.Driver) || g.pending("") >= r.MaxActiveEvents || g.pending(c.Name) > 0 {
			continue
		}
		if e := g.deal(c, rng); e != nil {
			g.Events = append(g.Events, e)
		}
	}
}

// candidates are the placed nodes a card can hit.
func (g *Game) candidates(c Card) []*Node {
	var out []*Node
	for _, n := range g.Nodes {
		if n.Kind == KindInternet || g.upReplicas(n) == 0 {
			continue
		}
		ok := len(c.Targets) == 0
		for _, k := range c.Targets {
			ok = ok || n.Kind == k
		}
		if ok {
			out = append(out, n)
		}
	}
	return out
}

// deal turns a card into an event against the current world, or returns nil
// when the card has nothing to hit.
func (g *Game) deal(c Card, rng *rand.Rand) *Event {
	e := &Event{
		Card: c.Name, Label: c.Label, Effect: c.Effect, LowestHealth: 100,
		Magnitude: c.MinMagnitude + rng.Float64()*(c.MaxMagnitude-c.MinMagnitude),
	}
	e.Start = g.Tick + c.LeadTicks
	e.End = e.Start + c.MinTicks + rng.IntN(c.MaxTicks-c.MinTicks+1)
	nodes := g.candidates(c)
	switch c.Effect {
	case EffectCrash, EffectSlowdown, EffectHitRatio, EffectCapacity:
		if len(nodes) == 0 {
			return nil
		}
		e.Target = nodes[rng.IntN(len(nodes))].ID
		if c.Effect == EffectCrash {
			e.Hits = []Hit{{Node: e.Target, Replicas: 1}}
		}
	case EffectZone:
		for _, n := range nodes {
			// A zone holds its share of each component's replicas, rounded up.
			e.Hits = append(e.Hits, Hit{Node: n.ID, Replicas: int(math.Ceil(float64(n.Replicas)*e.Magnitude - 1e-9))})
		}
		if len(e.Hits) == 0 {
			return nil
		}
	case EffectCost:
		var kinds []string
		seen := map[string]bool{}
		for _, n := range nodes {
			if !seen[n.Kind] {
				seen[n.Kind] = true
				kinds = append(kinds, n.Kind)
			}
		}
		if len(kinds) == 0 {
			return nil
		}
		e.Target = kinds[rng.IntN(len(kinds))]
	}
	g.eventSeq++
	e.ID = g.eventSeq
	return e
}

// track follows each event's worst health and judges it RecoveryTicks after
// it ends. It runs after a tick is simulated, before the clock advances.
func (g *Game) track(health float64) {
	r := g.Rules
	for _, e := range g.Events {
		if e.Outcome != "" || g.Tick < e.Start {
			continue
		}
		e.LowestHealth = math.Min(e.LowestHealth, health)
		e.LoadTest = e.LoadTest || g.loadTest()
		if g.Tick >= e.End+r.RecoveryTicks-1 {
			e.Outcome = OutcomeUnrecovered
			if health >= r.RecoveredHealth {
				e.Outcome = OutcomeRecovered
			}
		}
	}
	settled := 0
	for _, e := range g.Events {
		if e.Outcome != "" {
			settled++
		}
	}
	kept := g.Events[:0]
	for _, e := range g.Events {
		if e.Outcome != "" && settled > settledKept {
			settled--
			continue
		}
		kept = append(kept, e)
	}
	g.Events = kept
}

// phases labels each event relative to the next tick to simulate.
func (g *Game) phases() {
	for _, e := range g.Events {
		switch {
		case e.Outcome != "":
			e.Phase = PhaseOver
		case g.Tick < e.Start:
			e.Phase = PhaseUpcoming
		case g.Tick < e.End:
			e.Phase = PhaseActive
		default:
			e.Phase = PhaseRecovering
		}
	}
}

func (g *Game) respond(c Command) error {
	if len(g.Rules.Events) == 0 {
		return invalid("ruleset %s has no incident responses", g.Rules.Version)
	}
	g.fx = g.effects()
	switch c.Action {
	case ActRestart:
		return g.restart(c.Node)
	case ActFailover:
		return g.failover(c.Node)
	case ActRateLimit, ActLiftRateLimit:
		return g.rateLimit(c.Node, c.Action == ActRateLimit)
	}
	return invalid("unknown response %q", c.Action)
}

// restart brings a component's crashed replicas back after RestartTicks. It
// cannot bring back replicas lost with their zone.
func (g *Game) restart(id string) error {
	if _, _, err := g.placeable(id); err != nil {
		return err
	}
	crashed, zoned := false, false
	for _, e := range g.Events {
		if !g.active(e) {
			continue
		}
		for _, h := range e.Hits {
			if h.Node != id {
				continue
			}
			if e.Effect == EffectCrash {
				crashed = true
				e.End = min(e.End, g.Tick+g.Rules.RestartTicks)
			} else {
				zoned = true
			}
		}
	}
	switch {
	case crashed:
		return nil
	case zoned:
		return invalid("%s is down with its zone; a restart cannot bring it back", id)
	}
	return invalid("%s has no crashed replicas to restart", id)
}

// failover promotes the healthiest read replica that shares a sender with a
// database primary; the old primary becomes a replica. Senders that can only
// write to a primary are moved to the new one.
func (g *Game) failover(id string) error {
	p, _, err := g.placeable(id)
	if err != nil {
		return err
	}
	if p.Kind != KindDBPrimary {
		return invalid("failover needs a database primary, %s is a %s", id, p.Kind)
	}
	senders := map[string]bool{}
	var order []string
	for _, e := range g.Edges {
		if e.To == id && !senders[e.From] {
			senders[e.From] = true
			order = append(order, e.From)
		}
	}
	var best *Node
	for _, n := range g.Nodes {
		if n.Kind != KindDBReplica || g.capacity(n) == 0 {
			continue
		}
		shared := false
		for _, e := range g.Edges {
			shared = shared || (e.To == n.ID && senders[e.From])
		}
		if shared && (best == nil || g.capacity(n) > g.capacity(best)) {
			best = n
		}
	}
	if best == nil {
		return invalid("no healthy read replica shares a sender with %s", id)
	}
	p.Kind, best.Kind = KindDBReplica, KindDBPrimary
	for _, from := range order {
		if !g.hasEdge(from, best.ID) {
			g.Edges = append(g.Edges, Edge{From: from, To: best.ID})
		}
		fk, _ := g.Rules.Kind(g.Node(from).Kind)
		if !fk.connects(KindDBReplica) {
			_ = g.disconnect(from, id)
		}
	}
	return nil
}

func (g *Game) rateLimit(id string, on bool) error {
	n, _, err := g.placeable(id)
	if err != nil {
		return err
	}
	if n.Kind != KindGateway {
		return invalid("only an API gateway can rate-limit, %s is a %s", id, n.Kind)
	}
	if n.RateLimited == on {
		return invalid("rate limiting on %s is already %v", id, map[bool]string{true: "on", false: "off"}[on])
	}
	n.RateLimited = on
	return nil
}
