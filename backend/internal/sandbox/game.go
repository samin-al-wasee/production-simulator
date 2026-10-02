package sandbox

import (
	"errors"
	"fmt"
	"strings"
)

// Game statuses.
const (
	StatusRunning  = "running"
	StatusBankrupt = "bankrupt"
)

// Command types.
const (
	CmdPlace      = "place"
	CmdRemove     = "remove"
	CmdConnect    = "connect"
	CmdDisconnect = "disconnect"
	CmdResize     = "resize"
	CmdScale      = "scale"
	CmdMove       = "move"
	CmdRespond    = "respond"
	CmdConfigure  = "configure"
)

// InternetID is the fixed traffic source present in every game.
const InternetID = "internet"

// Node is a placed component.
type Node struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Size     string  `json:"size"`
	Replicas int     `json:"replicas"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	// DownReplicas are replicas taken down by events; when every replica is
	// down the component is Down and serves nothing. Both are derived from
	// the active events each time the flow is solved.
	DownReplicas int  `json:"downReplicas,omitempty"`
	Down         bool `json:"down,omitempty"`
	// RateLimited marks an API gateway that blocks most attack traffic.
	RateLimited bool `json:"rateLimited,omitempty"`
	// Backlog is the number of messages waiting in a queue.
	Backlog float64 `json:"backlog,omitempty"`
	// Traffic is the Internet's configuration once the player has set one;
	// until then the ruleset's applies. TrafficSince is the tick it was set,
	// from which its pattern runs.
	Traffic      *TrafficConfig `json:"traffic,omitempty"`
	TrafficSince int            `json:"trafficSince,omitempty"`
	// App is an application instance's configuration once set. StartedAt is
	// the tick it starts in; it is down until CrashedUntil after running out
	// of memory.
	App          *AppConfig `json:"app,omitempty"`
	StartedAt    int        `json:"startedAt,omitempty"`
	CrashedUntil int        `json:"crashedUntil,omitempty"`
	// Client is a traffic component's configuration once set (v6); its
	// pattern also runs from TrafficSince.
	Client *ClientConfig `json:"client,omitempty"`
	// Listener overrides the kind's listener once configured (v7).
	Listener *Listener `json:"listener,omitempty"`
	// DB is a database's configuration once set (v8). A read replica's
	// Backlog is the writes it has not applied yet.
	DB *DBConfig `json:"db,omitempty"`
	// Cache is a cache's configuration once set (v9), and Warmth the share
	// of its hot keys loaded.
	Cache  *CacheConfig `json:"cache,omitempty"`
	Warmth float64      `json:"warmth,omitempty"`
	// Storage is object storage's configuration once set (v10).
	Storage *StorageConfig `json:"storage,omitempty"`
	// Queue and Worker configure a queue and a worker once set (v11);
	// DeadLetters are the messages a queue gave up on.
	Queue       *QueueConfig  `json:"queue,omitempty"`
	Worker      *WorkerConfig `json:"worker,omitempty"`
	DeadLetters float64       `json:"deadLetters,omitempty"`
	// Stream configures an event stream once set (v12); Lags are the
	// events each consumer group has not read yet.
	Stream *StreamConfig      `json:"stream,omitempty"`
	Lags   map[string]float64 `json:"lags,omitempty"`
	// LB, Gateway, and CDN configure the edge once set (v13).
	LB      *LBConfig      `json:"lb,omitempty"`
	Gateway *GatewayConfig `json:"gateway,omitempty"`
	CDN     *CDNConfig     `json:"cdn,omitempty"`
	// Telemetry is what the component reports once configured (v14).
	Telemetry *Telemetry `json:"telemetry,omitempty"`
}

// Edge sends traffic from one node to another. Conn is its client side
// from v7, for every target that listens.
type Edge struct {
	From string      `json:"from"`
	To   string      `json:"to"`
	Conn *Connection `json:"conn,omitempty"`
}

// Command is one player action. Which fields apply depends on Type.
type Command struct {
	Type     string  `json:"type"`
	Action   string  `json:"action,omitempty"`
	Kind     string  `json:"kind,omitempty"`
	Size     string  `json:"size,omitempty"`
	Node     string  `json:"node,omitempty"`
	From     string  `json:"from,omitempty"`
	To       string  `json:"to,omitempty"`
	Replicas int     `json:"replicas,omitempty"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	// Traffic or App is the new configuration for a configure command; App
	// may also come with a place command for an application instance.
	Traffic *TrafficConfig `json:"traffic,omitempty"`
	App     *AppConfig     `json:"app,omitempty"`
	Client  *ClientConfig  `json:"client,omitempty"`
	// Listener configures a node; Connection the edge From → To (v7).
	Listener   *Listener      `json:"listener,omitempty"`
	Connection *Connection    `json:"connection,omitempty"`
	DB         *DBConfig      `json:"db,omitempty"`
	Cache      *CacheConfig   `json:"cache,omitempty"`
	Storage    *StorageConfig `json:"storage,omitempty"`
	Queue      *QueueConfig   `json:"queue,omitempty"`
	Worker     *WorkerConfig  `json:"worker,omitempty"`
	Stream     *StreamConfig  `json:"stream,omitempty"`
	LB         *LBConfig      `json:"lb,omitempty"`
	Gateway    *GatewayConfig `json:"gateway,omitempty"`
	CDN        *CDNConfig     `json:"cdn,omitempty"`
	Telemetry  *Telemetry     `json:"telemetry,omitempty"`
}

// LoggedCommand is a command applied before a given tick was simulated.
type LoggedCommand struct {
	Tick    int     `json:"tick"`
	Command Command `json:"command"`
}

// Game is one Sandbox world. It is not safe for concurrent use.
type Game struct {
	Rules *Ruleset
	Seed  int64
	// FreeBuild unlocks every kind from the start; goals are still tracked.
	FreeBuild bool
	Tick      int
	Status    string
	Nodes     []*Node
	Edges     []Edge
	Log       []LoggedCommand

	Cash         float64
	Users        float64
	Satisfaction float64
	Popularity   float64
	negativeFor  int
	nextID       map[string]int

	// Events holds upcoming, active, and recently judged events.
	Events   []*Event
	eventSeq int
	// Achieved is the tick each reached goal was reached at.
	Achieved map[string]int
	streak   map[string]int
	// fx are the effects of the events active at the tick being solved.
	fx effects
	// attemptFail is the share of each class's request attempts that
	// failed last tick, which decides how often clients retry this tick.
	attemptFail vec
	// lastPath is each node's latency per request class last tick, which
	// decides how long an application waits on its dependencies.
	lastPath map[string]vec
	// appCaps is each application's capacity during the current solve.
	appCaps map[*Node]float64
	// clients are the traffic components' loads during the current solve,
	// and clientFail each one's attempt failure rate last tick (v6).
	clients    []clientLoad
	clientFail map[string]float64
	// v7: edgeFail is each connection's attempt failure rate last tick,
	// lastEp each application's latency per endpoint last tick, and the
	// rest belong to the current solve: per-endpoint load at each
	// application, each application's run, and the edges' problems.
	edgeFail     map[string]float64
	edgeFailNext map[string]float64
	lastEp       map[string]map[string]float64
	epLoad       []map[string]float64
	runs         []*appRun
	problem      map[[2]int]string
	edgeRun      map[string]*EdgeStats
	// v8: each database's capacity this solve, and last tick's load at
	// every node and writes at every primary.
	dbCaps     map[*Node]float64
	lastLoad   map[string]vec
	lastWrites map[string]float64
	// v9: each cache's capacity and run this solve.
	cacheCaps map[*Node]float64
	cacheRuns []*cacheRun
	// v11: the share of each queue's deliveries its workers failed last
	// tick.
	queueFail map[string]float64
	// v13: each edge node's forwarding, outcomes per endpoint, and stats
	// this solve.
	fwds     []map[string]fwd
	epOut    []map[string][2]float64
	edgeRuns []*EdgeNodeStats

	Last    Snapshot
	History []Meters
}

// New starts an empty world and starting cash: with the Internet node before
// v6, and with no node at all from v6.
func New(rules *Ruleset, seed int64) *Game {
	g := &Game{
		Rules:        rules,
		Seed:         seed,
		Status:       StatusRunning,
		Nodes:        []*Node{{ID: InternetID, Kind: KindInternet, Size: "small", Replicas: 1}},
		clientFail:   map[string]float64{},
		edgeFail:     map[string]float64{},
		Cash:         rules.StartingCash,
		Users:        rules.StartingUsers,
		Satisfaction: 50,
		Popularity:   rules.StartingPopularity,
		nextID:       map[string]int{},
		Achieved:     map[string]int{},
		streak:       map[string]int{},
	}
	if rules.Client != nil {
		g.Nodes = nil
	}
	g.fx = g.effects()
	g.Last = g.preview()
	return g
}

// ErrInvalid wraps every rejected command.
var ErrInvalid = errors.New("invalid command")

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Node returns a node by ID.
func (g *Game) Node(id string) *Node {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// Apply validates and applies a command before the next tick and records it
// in the log. For a place command it returns the new node's ID.
func (g *Game) Apply(c Command) (string, error) {
	if g.Status != StatusRunning {
		return "", invalid("game is %s", g.Status)
	}
	id, err := g.apply(c)
	if err != nil {
		return "", err
	}
	g.Log = append(g.Log, LoggedCommand{Tick: g.Tick, Command: c})
	g.Last = g.preview()
	return id, nil
}

func (g *Game) apply(c Command) (string, error) {
	switch c.Type {
	case CmdPlace:
		return g.place(c)
	case CmdRemove:
		return "", g.remove(c.Node)
	case CmdConnect:
		return "", g.connect(c.From, c.To)
	case CmdDisconnect:
		return "", g.disconnect(c.From, c.To)
	case CmdResize:
		return "", g.resize(c.Node, c.Size)
	case CmdScale:
		return "", g.scale(c.Node, c.Replicas)
	case CmdRespond:
		return "", g.respond(c)
	case CmdConfigure:
		return "", g.configure(c)
	case CmdMove:
		n := g.Node(c.Node)
		if n == nil {
			return "", invalid("no node %q", c.Node)
		}
		n.X, n.Y = c.X, c.Y
		return "", nil
	}
	return "", invalid("unknown command type %q", c.Type)
}

func (g *Game) spend(amount float64, what string) error {
	if amount > g.Cash {
		return invalid("%s costs %.2f, cash is %.2f", what, amount, g.Cash)
	}
	g.Cash -= amount
	return nil
}

func (g *Game) place(c Command) (string, error) {
	k, ok := g.Rules.Kind(c.Kind)
	if !ok || k.Name == KindInternet {
		return "", invalid("cannot place kind %q", c.Kind)
	}
	if k.UnlockedBy != "" && !g.Reached(k.UnlockedBy) && !g.FreeBuild {
		gl, _ := g.Rules.Goal(k.UnlockedBy)
		return "", invalid("%s is locked until the goal %q", k.Label, gl.Title)
	}
	size := c.Size
	if size == "" {
		size = g.Rules.Sizes[0].Name
	}
	s, ok := g.Rules.Size(size)
	if !ok {
		return "", invalid("unknown size %q", size)
	}
	// An application instance may be placed with its configuration, which
	// is checked before anything is paid.
	if c.App != nil {
		if k.Name != KindApp || g.Rules.App == nil {
			return "", invalid("only an application instance can be placed with a configuration, in ruleset v5 and later")
		}
		if problems := g.Rules.ValidateApp(*c.App); len(problems) > 0 {
			return "", invalid("%s", strings.Join(problems, "; "))
		}
	}
	if err := g.spend(k.BuildCost*s.CostFactor, k.Label); err != nil {
		return "", err
	}
	g.nextID[k.Name]++
	n := &Node{ID: fmt.Sprintf("%s-%d", k.Name, g.nextID[k.Name]), Kind: k.Name, Size: size, Replicas: 1, X: c.X, Y: c.Y, App: c.App}
	if g.appModel(n) {
		n.StartedAt = g.Tick
	}
	g.Nodes = append(g.Nodes, n)
	return n.ID, nil
}

func (g *Game) remove(id string) error {
	if id == InternetID {
		return invalid("the Internet cannot be removed")
	}
	idx := -1
	for i, n := range g.Nodes {
		if n.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return invalid("no node %q", id)
	}
	g.Nodes = append(g.Nodes[:idx], g.Nodes[idx+1:]...)
	kept := g.Edges[:0]
	for _, e := range g.Edges {
		if e.From != id && e.To != id {
			kept = append(kept, e)
		} else if f := g.Node(e.From); f != nil && f.Kind == KindTraffic {
			g.release(f)
		}
	}
	g.Edges = kept
	return nil
}

func (g *Game) hasEdge(from, to string) bool {
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}

func (g *Game) connect(from, to string) error {
	f, t := g.Node(from), g.Node(to)
	if f == nil || t == nil {
		return invalid("connect needs two existing nodes")
	}
	fk, _ := g.Rules.Kind(f.Kind)
	if !fk.connects(t.Kind) {
		return invalid("%s cannot send traffic to %s", f.Kind, t.Kind)
	}
	if g.hasEdge(from, to) {
		return invalid("%s is already connected to %s", from, to)
	}
	if f.Kind == KindTraffic {
		for _, e := range g.Edges {
			if e.From == from {
				return invalid("a traffic component connects to exactly one component; disconnect %s from %s first", from, e.To)
			}
		}
	}
	if g.reaches(to, from) {
		return invalid("connecting %s to %s would create a loop", from, to)
	}
	g.Edges = append(g.Edges, Edge{From: from, To: to})
	if f.Kind == KindTraffic && g.clientModel() {
		g.adopt(f, t)
	}
	if g.edgeModel(f) && t.Kind == KindApp {
		// Traffic in front of this edge node with nothing to ask for yet
		// takes the routes of the application now behind it.
		for _, e := range g.Edges {
			if src := g.Node(e.From); e.To == f.ID && src.Kind == KindTraffic && len(g.clientConfig(src).Endpoints) == 0 {
				g.adopt(src, f)
			}
		}
	}
	if f.Kind != KindTraffic && g.callModel() {
		g.adoptConn(&g.Edges[len(g.Edges)-1], t)
	}
	return nil
}

func (g *Game) reaches(from, to string) bool {
	seen := map[string]bool{}
	stack := []string{from}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == to {
			return true
		}
		if seen[cur] {
			continue
		}
		seen[cur] = true
		for _, e := range g.Edges {
			if e.From == cur {
				stack = append(stack, e.To)
			}
		}
	}
	return false
}

func (g *Game) disconnect(from, to string) error {
	for i, e := range g.Edges {
		if e.From == from && e.To == to {
			g.Edges = append(g.Edges[:i], g.Edges[i+1:]...)
			if f := g.Node(from); f.Kind == KindTraffic {
				g.release(f)
			}
			return nil
		}
	}
	return invalid("%s is not connected to %s", from, to)
}

func (g *Game) placeable(id string) (*Node, Kind, error) {
	n := g.Node(id)
	if n == nil || n.ID == InternetID {
		return nil, Kind{}, invalid("no placeable node %q", id)
	}
	if n.Kind == KindTraffic {
		return nil, Kind{}, invalid("a traffic component has no size or replicas")
	}
	if g.storageModel(n) {
		return nil, Kind{}, invalid("object storage is a managed service: it scales by prefixes, not sizes or replicas")
	}
	if g.edgeModel(n) && n.Kind == KindCDN {
		return nil, Kind{}, invalid("a CDN is a managed service: it has no sizes or replicas")
	}
	k, _ := g.Rules.Kind(n.Kind)
	return n, k, nil
}

func (g *Game) resize(id, size string) error {
	n, k, err := g.placeable(id)
	if err != nil {
		return err
	}
	to, ok := g.Rules.Size(size)
	if !ok {
		return invalid("unknown size %q", size)
	}
	from, _ := g.Rules.Size(n.Size)
	if diff := k.BuildCost * (to.CostFactor - from.CostFactor) * float64(n.Replicas); diff > 0 {
		if err := g.spend(diff, "resize"); err != nil {
			return err
		}
	}
	n.Size = size
	return nil
}

func (g *Game) scale(id string, replicas int) error {
	n, k, err := g.placeable(id)
	if err != nil {
		return err
	}
	if replicas < 1 || replicas > g.Rules.MaxReplicas {
		return invalid("replicas must be between 1 and %d", g.Rules.MaxReplicas)
	}
	s, _ := g.Rules.Size(n.Size)
	if added := replicas - n.Replicas; added > 0 {
		if err := g.spend(k.BuildCost*s.CostFactor*float64(added), "scale"); err != nil {
			return err
		}
	}
	n.Replicas = replicas
	return nil
}
