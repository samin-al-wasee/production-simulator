package sandbox

import "math"

// Condition types.
const (
	CondMeter = "meter" // a meter, or its change over Over ticks
	CondKind  = "kind"  // a statistic over every placed component of a kind
	CondEvent = "event" // how many events of a card were judged recovered
)

// Goal is a mission the engine checks after every tick. It is reached when
// every condition holds for HoldTicks ticks in a row (at least one), after
// the goal it Requires. Reaching a goal is permanent and can unlock kinds.
type Goal struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Requires    string      `json:"requires,omitempty"`
	HoldTicks   int         `json:"holdTicks,omitempty"`
	Conditions  []Condition `json:"conditions"`
}

// Condition bounds one value with Min and/or Max.
type Condition struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	// Name is a meter, a kind, or an event card ("" for any card).
	Name string `json:"name,omitempty"`
	// Stat is the kind statistic: served (sum, ops/s), utilization (max),
	// replicas (sum), or backlog (sum).
	Stat string `json:"stat,omitempty"`
	// Over makes a meter condition bound the change over this many ticks.
	Over int `json:"over,omitempty"`
	// Below counts only events whose lowest health fell below it.
	Below float64  `json:"below,omitempty"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	// Unit is "ratio" for a value where 1 means 100%; empty for a count.
	Unit string `json:"unit,omitempty"`
}

// ConditionStatus is a condition with its current value.
type ConditionStatus struct {
	Label string   `json:"label"`
	Unit  string   `json:"unit,omitempty"`
	Value float64  `json:"value"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Met   bool     `json:"met"`
}

// GoalStatus is a goal as the player sees it.
type GoalStatus struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Requires    string            `json:"requires,omitempty"`
	Unlocks     []string          `json:"unlocks,omitempty"`
	AchievedAt  *int              `json:"achievedAt,omitempty"`
	Conditions  []ConditionStatus `json:"conditions"`
}

func at(v float64) *float64 { return &v }

// RulesetV3 is RulesetV2 with goals, and kinds unlocked by reaching them.
func RulesetV3() *Ruleset {
	r := RulesetV2()
	r.Version = "sandbox/v3"
	unlocks := map[string]string{
		KindLB:        "first-request",
		KindCache:     "startup-tier",
		KindDBReplica: "startup-tier",
		KindQueue:     "startup-tier",
		KindWorker:    "startup-tier",
		KindGateway:   "startup-tier",
		KindCDN:       "scale-up-tier",
	}
	for i := range r.Kinds {
		r.Kinds[i].UnlockedBy = unlocks[r.Kinds[i].Name]
	}
	meter := func(label, name string, min, max *float64) Condition {
		return Condition{Type: CondMeter, Label: label, Name: name, Min: min, Max: max}
	}
	kind := func(label, name, stat string, min, max *float64) Condition {
		c := Condition{Type: CondKind, Label: label, Name: name, Stat: stat, Min: min, Max: max}
		if stat == "utilization" {
			c.Unit = "ratio"
		}
		return c
	}
	recovered := func(label, card string, below float64) Condition {
		return Condition{Type: CondEvent, Label: label, Name: card, Below: below, Min: at(1)}
	}
	r.Goals = []Goal{
		{ID: "first-request", Title: "First request", Description: "Serve a successful request.",
			Conditions: []Condition{meter("Successful requests/s", "successRps", at(0.001), nil)}},
		{ID: "profitable-day", Title: "A profitable day", Description: "End a simulated day with more cash than it started with.",
			Conditions: []Condition{{Type: CondMeter, Label: "Cash gained over the last day", Name: "cash", Over: 288, Min: at(0.01)}}},
		{ID: "saturate-app", Title: "Saturation", Description: "Push an application instance to full utilization.",
			Conditions: []Condition{kind("App utilization", KindApp, "utilization", at(1), nil)}},
		{ID: "scale-out", Title: "Scale out", Description: "Serve through a load balancer in front of two or more app replicas.",
			Conditions: []Condition{
				kind("Load balancer ops/s", KindLB, "served", at(0.001), nil),
				kind("App replicas", KindApp, "replicas", at(2), nil),
			}},
		{ID: "database-bottleneck", Title: "The bottleneck moves", Description: "Saturate the database while the apps have room.",
			Conditions: []Condition{
				kind("Database utilization", KindDBPrimary, "utilization", at(1), nil),
				kind("App utilization", KindApp, "utilization", at(0.001), at(0.99)),
			}},
		{ID: "startup-tier", Title: "Startup", Description: "Reach 10,000 users.",
			Conditions: []Condition{meter("Users", "users", at(10_000), nil)}},
		{ID: "cache", Title: "Cache hits", Description: "Serve reads from a cache.",
			Conditions: []Condition{kind("Cache ops/s", KindCache, "served", at(0.001), nil)}},
		{ID: "read-replicas", Title: "Read replicas", Description: "Serve reads from a read replica.",
			Conditions: []Condition{kind("Read replica ops/s", KindDBReplica, "served", at(0.001), nil)}},
		{ID: "backlog", Title: "A backlog", Description: "Let 1,000 messages pile up in a queue.",
			Conditions: []Condition{kind("Messages waiting", KindQueue, "backlog", at(1000), nil)}},
		{ID: "drain-backlog", Title: "Drained", Description: "Size workers to empty the queue again.", Requires: "backlog",
			Conditions: []Condition{
				kind("Worker ops/s", KindWorker, "served", at(0.001), nil),
				kind("Messages waiting", KindQueue, "backlog", nil, at(0.5)),
			}},
		{ID: "scale-up-tier", Title: "Scale-up", Description: "Reach 100,000 users with health of 80 or more.",
			Conditions: []Condition{meter("Users", "users", at(100_000), nil), meter("Health", "health", at(80), nil)}},
		{ID: "healthy-margin", Title: "Healthy margin", Description: "At 100,000 users, keep cost at or below half of revenue for an hour.",
			HoldTicks:  12,
			Conditions: []Condition{meter("Users", "users", at(100_000), nil), {Type: CondMeter, Label: "Cost ÷ revenue", Name: "costToRevenue", Max: at(0.5), Unit: "ratio"}}},
		{ID: "recover-incident", Title: "Back on your feet", Description: "Recover from an event that pushed health below 80.",
			Conditions: []Condition{recovered("Events recovered from", "", 80)}},
		{ID: "weather-ddos", Title: "Weather a DDoS", Description: "Recover from a DDoS attack.",
			Conditions: []Condition{recovered("DDoS attacks recovered from", "ddos", 0)}},
		{ID: "survive-zone-outage", Title: "Survive a zone outage", Description: "Recover from a zone outage.",
			Conditions: []Condition{recovered("Zone outages recovered from", "zone-outage", 0)}},
	}
	return r
}

// Goal returns a goal by ID.
func (r *Ruleset) Goal(id string) (Goal, bool) {
	for _, gl := range r.Goals {
		if gl.ID == id {
			return gl, true
		}
	}
	return Goal{}, false
}

// Reached reports whether the game has reached a goal.
func (g *Game) Reached(id string) bool {
	_, ok := g.Achieved[id]
	return ok
}

// meterValue reads a meter by its JSON name.
func meterValue(m Meters, name string) (float64, bool) {
	switch name {
	case "users":
		return m.Users, true
	case "activeUsers":
		return m.ActiveUsers, true
	case "rps":
		return m.RPS, true
	case "successRps":
		return m.SuccessRPS, true
	case "errorRate":
		return m.ErrorRate, true
	case "p95LatencyMs":
		return m.P95LatencyMs, true
	case "health":
		return m.Health, true
	case "satisfaction":
		return m.Satisfaction, true
	case "popularity":
		return m.Popularity, true
	case "cash":
		return m.Cash, true
	case "revenuePerHour":
		return m.RevenuePerHour, true
	case "costPerHour":
		return m.CostPerHour, true
	case "costToRevenue":
		if m.RevenuePerHour <= 0 {
			return math.MaxFloat64, true
		}
		return m.CostPerHour / m.RevenuePerHour, true
	}
	return 0, false
}

// value is a condition's current value, from the last simulated tick.
func (g *Game) value(c Condition) float64 {
	switch c.Type {
	case CondMeter:
		v, _ := meterValue(g.Last.Meters, c.Name)
		if c.Over == 0 {
			return v
		}
		if len(g.History) <= c.Over {
			return 0
		}
		then, _ := meterValue(g.History[len(g.History)-1-c.Over], c.Name)
		return v - then
	case CondKind:
		v := 0.0
		for _, n := range g.Nodes {
			if n.Kind != c.Name {
				continue
			}
			var s NodeStats
			for _, st := range g.Last.Flow.Nodes {
				if st.ID == n.ID {
					s = st
				}
			}
			switch c.Stat {
			case "served":
				v += s.Served
			case "utilization":
				v = math.Max(v, s.Utilization)
			case "replicas":
				v += float64(n.Replicas)
			case "backlog":
				v += n.Backlog
			}
		}
		return v
	case CondEvent:
		n := 0
		for _, e := range g.Events {
			if e.Outcome == OutcomeRecovered && !e.LoadTest && (c.Name == "" || e.Card == c.Name) && (c.Below == 0 || e.LowestHealth < c.Below) {
				n++
			}
		}
		// Goals are checked right after events are judged, so an event
		// is counted before the feed can drop it.
		return float64(n)
	}
	return 0
}

func (c Condition) met(v float64) bool {
	return (c.Min == nil || v >= *c.Min) && (c.Max == nil || v <= *c.Max)
}

// checkGoals runs after a tick is simulated, before the clock advances.
func (g *Game) checkGoals() {
	for _, gl := range g.Rules.Goals {
		if g.Reached(gl.ID) {
			continue
		}
		ok := gl.Requires == "" || g.Reached(gl.Requires)
		for _, c := range gl.Conditions {
			ok = ok && c.met(g.value(c))
		}
		if !ok {
			delete(g.streak, gl.ID)
			continue
		}
		g.streak[gl.ID]++
		if g.streak[gl.ID] >= max(1, gl.HoldTicks) {
			g.Achieved[gl.ID] = g.Tick
			delete(g.streak, gl.ID)
		}
	}
}

// Goals returns every goal with its progress, in ruleset order.
func (g *Game) Goals() []GoalStatus {
	out := make([]GoalStatus, 0, len(g.Rules.Goals))
	for _, gl := range g.Rules.Goals {
		st := GoalStatus{ID: gl.ID, Title: gl.Title, Description: gl.Description, Requires: gl.Requires}
		for _, k := range g.Rules.Kinds {
			if k.UnlockedBy == gl.ID {
				st.Unlocks = append(st.Unlocks, k.Name)
			}
		}
		if t, ok := g.Achieved[gl.ID]; ok {
			st.AchievedAt = &t
		}
		for _, c := range gl.Conditions {
			v := g.value(c)
			st.Conditions = append(st.Conditions, ConditionStatus{Label: c.Label, Unit: c.Unit, Value: finite(v), Min: c.Min, Max: c.Max, Met: c.met(v)})
		}
		out = append(out, st)
	}
	return out
}
