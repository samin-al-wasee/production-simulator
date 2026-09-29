package sandbox

import (
	"errors"
	"fmt"
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
	// Down marks a failed component; it serves nothing.
	Down bool `json:"down,omitempty"`
	// Backlog is the number of messages waiting in a queue.
	Backlog float64 `json:"backlog,omitempty"`
}

// Edge sends traffic from one node to another.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Command is one player action. Which fields apply depends on Type.
type Command struct {
	Type     string  `json:"type"`
	Kind     string  `json:"kind,omitempty"`
	Size     string  `json:"size,omitempty"`
	Node     string  `json:"node,omitempty"`
	From     string  `json:"from,omitempty"`
	To       string  `json:"to,omitempty"`
	Replicas int     `json:"replicas,omitempty"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
}

// LoggedCommand is a command applied before a given tick was simulated.
type LoggedCommand struct {
	Tick    int     `json:"tick"`
	Command Command `json:"command"`
}

// Game is one Sandbox world. It is not safe for concurrent use.
type Game struct {
	Rules  *Ruleset
	Seed   int64
	Tick   int
	Status string
	Nodes  []*Node
	Edges  []Edge
	Log    []LoggedCommand

	Cash         float64
	Users        float64
	Satisfaction float64
	Popularity   float64
	negativeFor  int
	nextID       map[string]int

	Last    Snapshot
	History []Meters
}

// New starts an empty world: the Internet node and starting cash.
func New(rules *Ruleset, seed int64) *Game {
	g := &Game{
		Rules:        rules,
		Seed:         seed,
		Status:       StatusRunning,
		Nodes:        []*Node{{ID: InternetID, Kind: KindInternet, Size: "small", Replicas: 1}},
		Cash:         rules.StartingCash,
		Users:        rules.StartingUsers,
		Satisfaction: 50,
		Popularity:   rules.StartingPopularity,
		nextID:       map[string]int{},
	}
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
	size := c.Size
	if size == "" {
		size = g.Rules.Sizes[0].Name
	}
	s, ok := g.Rules.Size(size)
	if !ok {
		return "", invalid("unknown size %q", size)
	}
	if err := g.spend(k.BuildCost*s.CostFactor, k.Label); err != nil {
		return "", err
	}
	g.nextID[k.Name]++
	n := &Node{ID: fmt.Sprintf("%s-%d", k.Name, g.nextID[k.Name]), Kind: k.Name, Size: size, Replicas: 1, X: c.X, Y: c.Y}
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
	if g.reaches(to, from) {
		return invalid("connecting %s to %s would create a loop", from, to)
	}
	g.Edges = append(g.Edges, Edge{From: from, To: to})
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
