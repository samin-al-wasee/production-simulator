package sandbox

import (
	"fmt"
	"math"
	"strings"
)

// KindStream is an event stream: one topic of a Kafka-like log (v12).
const KindStream = "event-stream"

// DepStream is a route's publish to a connected event stream (v12).
const DepStream = "stream"

// EventsEndpoint is the route an application consuming a stream receives.
const EventsEndpoint = "POST /events"

// StreamConfig is an event stream's configuration (v12, ADR-0024).
type StreamConfig struct {
	Partitions     int     `json:"partitions"`
	RetentionHours float64 `json:"retentionHours"`
	MessageKB      float64 `json:"messageKb"`
	// KeySkew is the share of events on the hottest key; a partition holds
	// at least 1 ÷ partitions of them.
	KeySkew float64 `json:"keySkew"`
}

// StreamRuntime holds the constants of the event stream model.
type StreamRuntime struct {
	PartitionMBps float64 `json:"partitionMbps"`
	PublishMs     float64 `json:"publishMs"`
}

// GroupStats is one consumer group: the component an edge from the stream
// reaches, with every replica a member.
type GroupStats struct {
	Consumer   string  `json:"consumer"`
	Consumed   float64 `json:"consumed"`
	Members    int     `json:"members"`
	Active     int     `json:"active"`
	Lag        float64 `json:"lag"`
	LagSeconds float64 `json:"lagSeconds"`
	Lost       float64 `json:"lost"`
}

// StreamStats is what an event stream did during a tick.
type StreamStats struct {
	Health     string       `json:"health"`
	Bottleneck string       `json:"bottleneck"`
	Capacity   float64      `json:"capacity"`
	Produced   float64      `json:"produced"`
	Throttled  float64      `json:"throttled"`
	HotShare   float64      `json:"hotShare"`
	StoredMB   float64      `json:"storedMb"`
	Groups     []GroupStats `json:"groups"`
}

func (g *Game) streamModel(n *Node) bool {
	return n.Kind == KindStream && g.Rules.Stream != nil
}

func (g *Game) streamConfig(n *Node) *StreamConfig {
	if n.Stream != nil {
		return n.Stream
	}
	return g.Rules.Stream
}

// streamCapacity is how many events per second node n accepts: its hottest
// partition's share against a partition's throughput, or its brokers'
// network, whichever is smaller.
func (g *Game) streamCapacity(n *Node) (float64, string, float64) {
	cfg := g.streamConfig(n)
	size, _ := g.Rules.Size(n.Size)
	mb := cfg.MessageKB / 1024
	hot := math.Max(1/float64(cfg.Partitions), cfg.KeySkew)
	parts := g.Rules.StreamRuntime.PartitionMBps / mb / hot
	brokers := size.NetworkMbps / 8 / mb * float64(g.upReplicas(n))
	if brokers < parts {
		return brokers, "brokers", hot
	}
	return parts, "partitions", hot
}

// consume runs node i's stream for a tick: it accepts what fits, and every
// consumer group reads all of it, as fast as its members and the partitions
// let it, carrying its lag to the next tick. It returns the accepted rate
// and, per consumer, the rate delivered.
func (g *Game) consume(i int, offered float64) (float64, map[int]float64, *StreamStats, map[string]float64) {
	n := g.Nodes[i]
	r := g.Rules
	cfg := g.streamConfig(n)
	dt := r.TickSeconds
	capacity, bottleneck, hot := g.streamCapacity(n)
	if n.Down {
		capacity = 0
	}
	accepted := math.Min(offered, capacity)
	st := &StreamStats{Bottleneck: bottleneck, Capacity: finite(capacity), Produced: offered, Throttled: offered - accepted, HotShare: hot}
	st.StoredMB = accepted * cfg.MessageKB / 1024 * cfg.RetentionHours * 3600
	out := map[int]float64{}
	lags := map[string]float64{}
	keep := cfg.RetentionHours * 3600
	for _, j := range g.targets(i, KindWorker, KindApp) {
		c := g.Nodes[j]
		members := g.upReplicas(c)
		active := min(members, cfg.Partitions)
		drain := 0.0
		if members > 0 {
			// Members beyond the partitions sit idle.
			drain = g.capacity(c) * float64(active) / float64(members)
		}
		lag := n.Lags[c.ID]
		consumed := math.Min(drain, lag/dt+accepted)
		lag += (accepted - consumed) * dt
		// Events older than the retention are deleted unread.
		lost := 0.0
		if limit := accepted * keep; lag > limit {
			lost = (lag - limit) / dt
			lag = limit
		}
		lags[c.ID] = lag
		out[j] = consumed
		gs := GroupStats{Consumer: c.ID, Consumed: consumed, Members: members, Active: active, Lag: lag, Lost: lost}
		if consumed > 0 {
			gs.LagSeconds = lag / consumed
		} else if lag > 0 {
			gs.LagSeconds = math.Inf(1)
		}
		gs.LagSeconds = finite(gs.LagSeconds)
		st.Groups = append(st.Groups, gs)
	}
	fail := st.Throttled / math.Max(offered, 1e-300)
	lost := 0.0
	for _, gs := range st.Groups {
		lost += gs.Lost
	}
	switch {
	case n.Down:
		st.Health = HealthStopped
	case fail > 0.2 || lost > 0.01:
		st.Health = HealthUnhealthy
	case fail > 0.01 || offered > 0.85*capacity:
		st.Health = HealthDegraded
	default:
		st.Health = HealthHealthy
		for _, gs := range st.Groups {
			if gs.LagSeconds > 60 {
				st.Health = HealthDegraded
			}
		}
	}
	return accepted, out, st, lags
}

func (g *Game) configureStream(n *Node, c *StreamConfig) error {
	if c == nil {
		return invalid("configure needs a stream configuration")
	}
	if p := g.Rules.ValidateStream(*c); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.Stream = c
	return nil
}

// ValidateStream lists every problem with a stream configuration.
func (r *Ruleset) ValidateStream(c StreamConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if c.Partitions < 1 || c.Partitions > 1000 {
		bad("partitions must be between 1 and 1000")
	}
	if !(c.RetentionHours >= 1 && c.RetentionHours <= 24*30) {
		bad("retention must be between 1 hour and 30 days")
	}
	if !(c.MessageKB >= 0.1 && c.MessageKB <= 1024) {
		bad("message size must be between 0.1 and 1024 KB")
	}
	if !(c.KeySkew >= 0 && c.KeySkew <= 1) {
		bad("key skew must be between 0%% and 100%%")
	}
	return p
}

// RulesetV12 is RulesetV11 with event streams (Phase 11, ADR-0024).
func RulesetV12() *Ruleset {
	r := RulesetV11()
	r.Version = "sandbox/v12"
	r.Stream = &StreamConfig{Partitions: 6, RetentionHours: 24, MessageKB: 1, KeySkew: 0}
	r.StreamRuntime = StreamRuntime{PartitionMBps: 10, PublishMs: 3}
	r.WireProtocols = append(r.WireProtocols, "Kafka")
	r.Listeners[KindStream] = Listener{"Kafka", 9092, false}
	r.Kinds = append(r.Kinds, Kind{Name: KindStream, Label: "Event stream", Capacity: 0, ServiceMs: 3,
		CostPerHour: 3, BuildCost: 100, Complexity: 2.5, ConnectsTo: []string{KindWorker, KindApp}})
	for i := range r.Kinds {
		switch r.Kinds[i].Name {
		case KindApp, KindWorker:
			r.Kinds[i].ConnectsTo = append(r.Kinds[i].ConnectsTo, KindStream)
		}
	}
	r.AppTypes = append(r.AppTypes, AppType{Name: "Order events",
		Description: "Orders written to the database and published as events for any service that wants them.",
		Routes: []AppRoute{
			appRoute("POST /orders", 40, 30, 3, 4, 2, 0.6, DepDBWrite, DepStream),
			appRoute("GET /orders/:id", 15, 10, 1, 1, 3, 0.4, DepDBRead),
			{Endpoint: EventsEndpoint, BaseMs: 10, CPUMs: 5, MemoryMB: 1, RequestKB: 1, ResponseKB: 0.1},
		}})
	return r
}
