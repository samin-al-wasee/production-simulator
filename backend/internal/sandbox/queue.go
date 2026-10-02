package sandbox

import (
	"fmt"
	"math"
	"strings"
)

// QueueConfig is a message queue's configuration (v11, ADR-0023).
type QueueConfig struct {
	// Engine is a label: RabbitMQ, SQS.
	Engine string `json:"engine"`
	// MaxBacklog is how many messages one replica holds.
	MaxBacklog float64 `json:"maxBacklog"`
	// A message a worker has not acknowledged within VisibilitySeconds is
	// delivered again; after MaxDeliveries it goes to the dead-letter queue.
	VisibilitySeconds float64 `json:"visibilitySeconds"`
	MaxDeliveries     int     `json:"maxDeliveries"`
}

// WorkerConfig is a background worker's configuration (v11): how many
// messages each replica handles at once, and what handling one costs and
// calls, like an application's route.
type WorkerConfig struct {
	Concurrency int      `json:"concurrency"`
	Handler     AppRoute `json:"handler"`
}

// QueueStats is what a queue did during a tick.
type QueueStats struct {
	Health    string  `json:"health"`
	Published float64 `json:"published"`
	Rejected  float64 `json:"rejected"`
	Delivered float64 `json:"delivered"`
	// Redelivered are deliveries of messages a worker failed or held past
	// the visibility timeout; DeadLettered left for the dead-letter queue.
	Redelivered  float64 `json:"redelivered"`
	DeadLettered float64 `json:"deadLettered"`
	DeadLetters  float64 `json:"deadLetters"`
	Backlog      float64 `json:"backlog"`
	MaxBacklog   float64 `json:"maxBacklog"`
	// DelaySeconds is how long a message waits: backlog ÷ delivery rate.
	DelaySeconds float64 `json:"delaySeconds"`
	// WorkerFailure is the share of deliveries the workers failed last tick.
	WorkerFailure float64 `json:"workerFailure"`
}

// HandlerEndpoint is the route a worker's handler is; a message is a write.
const HandlerEndpoint = "POST /messages"

func (g *Game) queueModel(n *Node) bool {
	return n.Kind == KindQueue && g.Rules.Queue != nil
}

func (g *Game) queueConfig(n *Node) *QueueConfig {
	if n.Queue != nil {
		return n.Queue
	}
	return g.Rules.Queue
}

func (g *Game) workerConfig(n *Node) *WorkerConfig {
	if n.Worker != nil {
		return n.Worker
	}
	return g.Rules.Worker
}

// workerApp is the application configuration a worker runs as: one route,
// its handler, with as many slots as its concurrency, one process per vCPU,
// no backlog of its own (messages wait in the queue), and the visibility
// timeout of the queue that feeds it as its timeout.
func (g *Game) workerApp(n *Node) *AppConfig {
	w := g.workerConfig(n)
	size, _ := g.Rules.Size(n.Size)
	h := w.Handler
	h.Endpoint = HandlerEndpoint
	timeout := g.Rules.Queue.VisibilitySeconds * 1000
	for _, e := range g.Edges {
		if q := g.Node(e.From); e.To == n.ID && q != nil && q.Kind == KindQueue {
			timeout = g.queueConfig(q).VisibilitySeconds * 1000
			break
		}
	}
	return &AppConfig{
		Name: n.ID, Framework: "Worker", Protocol: "AMQP", Port: 1, Interface: "consumer", Server: "worker",
		Processing: ProcessingAsync, Workers: max(1, int(math.Ceil(size.VCPU))), MaxConcurrency: w.Concurrency,
		Backlog: 0, MaxConnections: 100_000, TimeoutMs: timeout, KeepAlive: true,
		Routes: []AppRoute{h},
	}
}

// redelivery is how many deliveries one published message takes when a
// share f of deliveries fails, at most max: 1 + f + … + f^(max−1).
func redelivery(f float64, maxDeliveries int) float64 {
	amp := 0.0
	for k := 0; k < maxDeliveries; k++ {
		amp += math.Pow(f, float64(k))
	}
	return amp
}

func (g *Game) configureQueue(n *Node, c *QueueConfig) error {
	if c == nil {
		return invalid("configure needs a queue configuration")
	}
	if p := g.Rules.ValidateQueue(*c); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.Queue = c
	return nil
}

func (g *Game) configureWorker(n *Node, c *WorkerConfig) error {
	if c == nil {
		return invalid("configure needs a worker configuration")
	}
	if p := g.Rules.ValidateWorker(*c); len(p) > 0 {
		return invalid("%s", strings.Join(p, "; "))
	}
	n.Worker = c
	return nil
}

// ValidateQueue lists every problem with a queue configuration.
func (r *Ruleset) ValidateQueue(c QueueConfig) []string {
	var p []string
	bad := func(format string, a ...any) { p = append(p, fmt.Sprintf(format, a...)) }
	if len(c.Engine) > maxNameLen {
		bad("engine must be at most %d characters", maxNameLen)
	}
	if !(c.MaxBacklog >= 1 && c.MaxBacklog <= 1e9) {
		bad("max backlog must be between 1 and 1000000000 messages")
	}
	if !(c.VisibilitySeconds >= 1 && c.VisibilitySeconds <= 43_200) {
		bad("visibility timeout must be between 1 s and 12 h")
	}
	if c.MaxDeliveries < 1 || c.MaxDeliveries > 100 {
		bad("max deliveries must be between 1 and 100")
	}
	return p
}

// ValidateWorker lists every problem with a worker configuration.
func (r *Ruleset) ValidateWorker(c WorkerConfig) []string {
	var p []string
	if c.Concurrency < 1 || c.Concurrency > 10_000 {
		p = append(p, "concurrency must be between 1 and 10000")
	}
	h := c.Handler
	h.Endpoint = HandlerEndpoint
	a := *r.App
	a.Routes = []AppRoute{h}
	for _, x := range r.ValidateApp(a) {
		p = append(p, strings.Replace(x, "route "+HandlerEndpoint, "handler", 1))
	}
	return p
}

// RulesetV11 is RulesetV10 with message queues that redeliver and dead-letter
// and background workers that run like applications (Phase 11, ADR-0023).
func RulesetV11() *Ruleset {
	r := RulesetV10()
	r.Version = "sandbox/v11"
	r.Queue = &QueueConfig{Engine: "RabbitMQ", MaxBacklog: 100_000, VisibilitySeconds: 30, MaxDeliveries: 5}
	r.Worker = &WorkerConfig{Concurrency: 10, Handler: AppRoute{
		Endpoint: HandlerEndpoint, BaseMs: 60, CPUMs: 20, MemoryMB: 4, RequestKB: 2, ResponseKB: 0.1, Deps: []string{DepDBWrite}}}
	for i := range r.Kinds {
		if r.Kinds[i].Name == KindWorker {
			r.Kinds[i].ConnectsTo = []string{KindDBPrimary, KindDBReplica, KindCache, KindStorage, KindApp}
		}
	}
	return r
}
