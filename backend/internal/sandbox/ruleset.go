// Package sandbox is the Production Sandbox engine (Phase 10, ADR-0013): a
// deterministic, model-driven production-system game. A player starts from an
// empty world, places and wires components with commands, and the engine
// advances the world in fixed ticks, deriving traffic, load, latency, errors,
// money, and user sentiment from declared capacities and the topology.
// Nothing is executed; every value is modelled and must be labelled as
// simulated (Principle 15). A game is fully defined by its ruleset version,
// seed, and ordered command log.
package sandbox

import "fmt"

// Component kinds a player can place, plus the fixed traffic source.
const (
	KindInternet  = "internet"
	KindCDN       = "cdn"
	KindLB        = "load-balancer"
	KindGateway   = "api-gateway"
	KindApp       = "app-instance"
	KindWorker    = "worker"
	KindDBPrimary = "db-primary"
	KindDBReplica = "db-replica"
	KindCache     = "cache"
	KindQueue     = "queue"
	KindStorage   = "object-storage"
	// KindTraffic is a population of clients (v6 and later, ADR-0018).
	KindTraffic = "traffic"
)

// Kind declares one placeable component. Capacity and costs are for the
// smallest size and one replica; sizes multiply them.
type Kind struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	// Capacity is operations per second one replica can serve.
	Capacity float64 `json:"capacity"`
	// ServiceMs is the unloaded time to serve one operation.
	ServiceMs float64 `json:"serviceMs"`
	// CostPerHour is charged every tick, per replica.
	CostPerHour float64 `json:"costPerHour"`
	// BuildCost is paid once per replica when placed or scaled up.
	BuildCost float64 `json:"buildCost"`
	// Complexity raises operations cost and the odds of failure events.
	Complexity float64 `json:"complexity"`
	// ConnectsTo lists the kinds this kind may send traffic to.
	ConnectsTo []string `json:"connectsTo"`
	// HitRatio is the share of reads a cache serves itself, or the share of
	// requests a CDN serves from the edge.
	HitRatio float64 `json:"hitRatio,omitempty"`
	// MaxBacklog is the number of messages one queue replica can hold.
	MaxBacklog float64 `json:"maxBacklog,omitempty"`
	// UnlockedBy is the goal that makes the kind placeable; empty means
	// placeable from the start.
	UnlockedBy string `json:"unlockedBy,omitempty"`
}

// Size scales a kind's capacity and costs.
type Size struct {
	Name           string  `json:"name"`
	CapacityFactor float64 `json:"capacityFactor"`
	CostFactor     float64 `json:"costFactor"`
	// An application instance's resources at this size (v5 and later).
	VCPU        float64 `json:"vcpu,omitempty"`
	MemoryGB    float64 `json:"memoryGb,omitempty"`
	NetworkMbps float64 `json:"networkMbps,omitempty"`
	// A database's disk operations per second at this size (v8).
	IOPS float64 `json:"iops,omitempty"`
}

// Tier is a named scale level reached at a userbase.
type Tier struct {
	Name     string  `json:"name"`
	MinUsers float64 `json:"minUsers"`
}

// Ruleset is the versioned data behind a game. Changing any value changes
// how saved games replay, so a change needs a new version.
type Ruleset struct {
	Version string `json:"version"`
	Kinds   []Kind `json:"kinds"`
	Sizes   []Size `json:"sizes"`
	Tiers   []Tier `json:"tiers"`

	TickSeconds  float64 `json:"tickSeconds"`
	StartHour    float64 `json:"startHour"`
	StartingCash float64 `json:"startingCash"`
	// BankruptcyGraceTicks is how long cash may stay negative.
	BankruptcyGraceTicks int `json:"bankruptcyGraceTicks"`
	MaxReplicas          int `json:"maxReplicas"`

	// Request mix at an application instance.
	ReadShare    float64 `json:"readShare"`
	StorageShare float64 `json:"storageShare"`

	TimeoutMs float64 `json:"timeoutMs"`
	SLOp95Ms  float64 `json:"sloP95Ms"`

	RevenuePerRequest        float64 `json:"revenuePerRequest"`
	OpsCostPerComplexityHour float64 `json:"opsCostPerComplexityHour"`

	StartingUsers float64 `json:"startingUsers"`
	MarketSize    float64 `json:"marketSize"`
	ActiveShare   float64 `json:"activeShare"`
	// BaseEngagement is requests per second per active user at 50% satisfaction.
	BaseEngagement     float64 `json:"baseEngagement"`
	GrowthRate         float64 `json:"growthRate"`
	ChurnRate          float64 `json:"churnRate"`
	StartingPopularity float64 `json:"startingPopularity"`
	// SatisfactionPull and PopularityPull are the per-tick smoothing toward
	// the instantaneous quality score.
	SatisfactionPull float64 `json:"satisfactionPull"`
	PopularityPull   float64 `json:"popularityPull"`

	// Goals are the missions a game can reach; a ruleset without goals
	// tracks none and locks no kind.
	Goals []Goal `json:"goals,omitempty"`

	// Events is the event deck; a ruleset without cards draws no events.
	Events []Card `json:"events,omitempty"`
	// EventGraceTicks is how long a new game runs before the first draw.
	EventGraceTicks int `json:"eventGraceTicks,omitempty"`
	MaxActiveEvents int `json:"maxActiveEvents,omitempty"`
	// RecoveryTicks is how long after an event ends its outcome is judged:
	// recovered when health is at least RecoveredHealth by then.
	RecoveryTicks   int     `json:"recoveryTicks,omitempty"`
	RecoveredHealth float64 `json:"recoveredHealth,omitempty"`
	// RestartTicks is how long a restarted component takes to come back.
	RestartTicks int `json:"restartTicks,omitempty"`
	// A rate-limited gateway blocks this share of attack traffic, and this
	// share of real users by mistake.
	RateLimitBlock         float64 `json:"rateLimitBlock,omitempty"`
	RateLimitFalsePositive float64 `json:"rateLimitFalsePositive,omitempty"`

	// Traffic is the Internet's starting configuration; a ruleset without one
	// uses ReadShare and StorageShare and cannot be configured.
	Traffic *TrafficConfig `json:"traffic,omitempty"`
	// Regions are the abstract places traffic can come from.
	Regions []string `json:"regions,omitempty"`
	// MaxRetries bounds a traffic group's retries.
	MaxRetries int `json:"maxRetries,omitempty"`
	// MaxTrafficRPS bounds a configured source's rate.
	MaxTrafficRPS float64 `json:"maxTrafficRps,omitempty"`

	// App is a new application instance's configuration; a ruleset without
	// one uses the kind's flat capacity and cannot configure applications.
	App        *AppConfig   `json:"app,omitempty"`
	AppRuntime AppRuntime   `json:"appRuntime,omitzero"`
	Middleware []Middleware `json:"middleware,omitempty"`
	Frameworks []Framework  `json:"frameworks,omitempty"`
	// AppStacks and AppTypes are the templates offered when an application
	// instance is placed: how it serves, and what it serves.
	AppStacks []AppStack `json:"appStacks,omitempty"`
	AppTypes  []AppType  `json:"appTypes,omitempty"`

	// Client is a new traffic component's configuration; a ruleset with one
	// starts empty and takes its traffic from traffic components. Its market
	// is split by ClientTypes and RegionShares.
	Client       *ClientConfig `json:"client,omitempty"`
	ClientTypes  []Weight      `json:"clientTypes,omitempty"`
	RegionShares []Weight      `json:"regionShares,omitempty"`
	// Protocols are the wire protocols both sides of a connection share.
	Protocols []string `json:"protocols,omitempty"`

	// From v7: every protocol a connection may speak, the listener of each
	// kind that takes connections, a new connection's client side, and the
	// latency a call adds by crossing the network.
	WireProtocols []string            `json:"wireProtocols,omitempty"`
	Listeners     map[string]Listener `json:"listeners,omitempty"`
	ConnDefaults  Connection          `json:"connDefaults,omitzero"`
	NetworkHopMs  float64             `json:"networkHopMs,omitempty"`

	// DB is a new database's configuration from v8; DBRuntime the model's
	// constants.
	DB        *DBConfig `json:"db,omitempty"`
	DBRuntime DBRuntime `json:"dbRuntime,omitzero"`
	// Cache is a new cache's configuration from v9.
	Cache        *CacheConfig `json:"cache,omitempty"`
	CacheRuntime CacheRuntime `json:"cacheRuntime,omitzero"`
	// Storage is new object storage's configuration from v10.
	Storage        *StorageConfig `json:"storage,omitempty"`
	StorageRuntime StorageRuntime `json:"storageRuntime,omitzero"`
	// Queue and Worker configure new queues and workers from v11.
	Queue  *QueueConfig  `json:"queue,omitempty"`
	Worker *WorkerConfig `json:"worker,omitempty"`
	// Stream configures new event streams from v12.
	Stream        *StreamConfig `json:"stream,omitempty"`
	StreamRuntime StreamRuntime `json:"streamRuntime,omitzero"`
}

// RulesetV1 is the first Sandbox ruleset.
func RulesetV1() *Ruleset {
	return &Ruleset{
		Version: "sandbox/v1",
		Kinds: []Kind{
			{Name: KindInternet, Label: "Internet", ConnectsTo: []string{KindCDN, KindLB, KindGateway, KindApp}},
			{Name: KindCDN, Label: "CDN", Capacity: 2000, ServiceMs: 5, CostPerHour: 3, BuildCost: 100, Complexity: 1, HitRatio: 0.3,
				ConnectsTo: []string{KindLB, KindGateway, KindApp}},
			{Name: KindLB, Label: "Load balancer", Capacity: 5000, ServiceMs: 1, CostPerHour: 1.5, BuildCost: 50, Complexity: 1,
				ConnectsTo: []string{KindGateway, KindApp}},
			{Name: KindGateway, Label: "API gateway", Capacity: 2000, ServiceMs: 3, CostPerHour: 2, BuildCost: 80, Complexity: 2,
				ConnectsTo: []string{KindLB, KindApp}},
			{Name: KindApp, Label: "Application instance", Capacity: 50, ServiceMs: 40, CostPerHour: 2, BuildCost: 60, Complexity: 1,
				ConnectsTo: []string{KindCache, KindDBPrimary, KindDBReplica, KindQueue, KindStorage}},
			{Name: KindWorker, Label: "Background worker", Capacity: 40, ServiceMs: 60, CostPerHour: 1.5, BuildCost: 50, Complexity: 1.5,
				ConnectsTo: []string{KindDBPrimary}},
			{Name: KindDBPrimary, Label: "Database primary", Capacity: 300, ServiceMs: 8, CostPerHour: 4, BuildCost: 200, Complexity: 3},
			{Name: KindDBReplica, Label: "Database read replica", Capacity: 300, ServiceMs: 8, CostPerHour: 3.5, BuildCost: 150, Complexity: 2},
			{Name: KindCache, Label: "Cache", Capacity: 5000, ServiceMs: 1, CostPerHour: 2, BuildCost: 80, Complexity: 2, HitRatio: 0.8,
				ConnectsTo: []string{KindDBPrimary, KindDBReplica}},
			{Name: KindQueue, Label: "Message queue", Capacity: 1000, ServiceMs: 2, CostPerHour: 1.5, BuildCost: 60, Complexity: 2.5, MaxBacklog: 100000,
				ConnectsTo: []string{KindWorker}},
			{Name: KindStorage, Label: "Object storage", Capacity: 1000, ServiceMs: 20, CostPerHour: 1, BuildCost: 30, Complexity: 1},
		},
		Sizes: []Size{
			{Name: "small", CapacityFactor: 1, CostFactor: 1},
			{Name: "medium", CapacityFactor: 2.5, CostFactor: 2.2},
			{Name: "large", CapacityFactor: 6, CostFactor: 5},
		},
		Tiers: []Tier{
			{Name: "garage", MinUsers: 0},
			{Name: "startup", MinUsers: 10_000},
			{Name: "scale-up", MinUsers: 100_000},
			{Name: "enterprise", MinUsers: 1_000_000},
			{Name: "hyperscale", MinUsers: 10_000_000},
		},
		TickSeconds:              300,
		StartHour:                8,
		StartingCash:             1000,
		BankruptcyGraceTicks:     288,
		MaxReplicas:              50,
		ReadShare:                0.8,
		StorageShare:             0.1,
		TimeoutMs:                2000,
		SLOp95Ms:                 300,
		RevenuePerRequest:        0.0005,
		OpsCostPerComplexityHour: 0.1,
		StartingUsers:            2000,
		MarketSize:               50_000_000,
		ActiveShare:              0.1,
		BaseEngagement:           0.05,
		GrowthRate:               0.004,
		ChurnRate:                0.01,
		StartingPopularity:       10,
		SatisfactionPull:         0.1,
		PopularityPull:           0.02,
	}
}

// Rulesets returns the ruleset for a version.
func Rulesets(version string) (*Ruleset, error) {
	switch version {
	case "sandbox/v1":
		return RulesetV1(), nil
	case "sandbox/v2":
		return RulesetV2(), nil
	case "sandbox/v3":
		return RulesetV3(), nil
	case "sandbox/v4":
		return RulesetV4(), nil
	case "sandbox/v5":
		return RulesetV5(), nil
	case "sandbox/v6":
		return RulesetV6(), nil
	case "sandbox/v7":
		return RulesetV7(), nil
	case "sandbox/v8":
		return RulesetV8(), nil
	case "sandbox/v9":
		return RulesetV9(), nil
	case "sandbox/v10":
		return RulesetV10(), nil
	case "sandbox/v11":
		return RulesetV11(), nil
	case "sandbox/v12":
		return RulesetV12(), nil
	}
	return nil, fmt.Errorf("unknown ruleset %q", version)
}

// Latest returns the ruleset new games are played with.
func Latest() *Ruleset {
	return RulesetV12()
}

// Kind returns a kind by name.
func (r *Ruleset) Kind(name string) (Kind, bool) {
	for _, k := range r.Kinds {
		if k.Name == name {
			return k, true
		}
	}
	return Kind{}, false
}

// Size returns a size by name.
func (r *Ruleset) Size(name string) (Size, bool) {
	for _, s := range r.Sizes {
		if s.Name == name {
			return s, true
		}
	}
	return Size{}, false
}

func (k Kind) connects(to string) bool {
	for _, c := range k.ConnectsTo {
		if c == to {
			return true
		}
	}
	return false
}
