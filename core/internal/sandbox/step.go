package sandbox

import "math"

// Meters are the world's headline values after a tick. All are simulated.
type Meters struct {
	Tick int     `json:"tick"`
	Day  int     `json:"day"`
	Hour float64 `json:"hour"`

	RPS         float64 `json:"rps"`
	SuccessRPS  float64 `json:"successRps"`
	Users       float64 `json:"users"`
	ActiveUsers float64 `json:"activeUsers"`
	// Engagement is requests per second per active user.
	Engagement float64 `json:"engagement"`

	P95LatencyMs float64 `json:"p95LatencyMs"`
	ErrorRate    float64 `json:"errorRate"`
	Health       float64 `json:"health"`
	Satisfaction float64 `json:"satisfaction"`
	Popularity   float64 `json:"popularity"`
	Complexity   float64 `json:"complexity"`
	Tier         string  `json:"tier"`

	RevenuePerHour float64 `json:"revenuePerHour"`
	CostPerHour    float64 `json:"costPerHour"`
	Cash           float64 `json:"cash"`
}

// Snapshot is the full view of a tick: meters and per-node flow.
type Snapshot struct {
	Meters Meters `json:"meters"`
	Flow   Flow   `json:"flow"`

	backlog []float64
}

const historyLimit = 576

// clock returns the simulated day (from 1) and hour of the current tick.
func (g *Game) clock() (int, float64) {
	sec := g.Rules.StartHour*3600 + float64(g.Tick)*g.Rules.TickSeconds
	return int(sec/86400) + 1, math.Mod(sec, 86400) / 3600
}

// diurnal is the share of peak traffic at an hour: 1.0 at 15:00, 0.2 at 03:00.
func diurnal(hour float64) float64 {
	return 0.6 + 0.4*math.Sin(2*math.Pi*(hour-9)/24)
}

func (g *Game) engagement() float64 {
	return g.Rules.BaseEngagement * (0.5 + g.Satisfaction/100)
}

func (g *Game) rps() float64 {
	_, hour := g.clock()
	return g.Users * g.Rules.ActiveShare * g.engagement() * diurnal(hour)
}

// Complexity sums each component's weight, extra replicas, and connections.
func (g *Game) Complexity() float64 {
	c := 0.5 * float64(len(g.Edges))
	for _, n := range g.Nodes {
		if n.Kind == KindInternet {
			continue
		}
		k, _ := g.Rules.Kind(n.Kind)
		c += k.Complexity + 0.2*float64(n.Replicas-1)
	}
	return c
}

// latencyFactor is 1 within the SLO and falls to 0 at five times the SLO.
func (g *Game) latencyFactor(p95 float64) float64 {
	slo := g.Rules.SLOp95Ms
	return clamp(1-(p95-slo)/(4*slo), 0, 1)
}

// quality is the instantaneous experience score (0-100) users react to.
func (g *Game) quality(f Flow) float64 {
	return 100 * (1 - f.ErrorRate) * g.latencyFactor(f.P95LatencyMs)
}

func (g *Game) meters(f Flow) Meters {
	day, hour := g.clock()
	cost := 0.0
	for _, n := range f.Nodes {
		cost += n.CostPerHour
	}
	cx := g.Complexity()
	cost += cx * g.Rules.OpsCostPerComplexityHour
	overload := clamp((f.MaxUtilization-0.8)/0.2, 0, 1)
	health := 100 * (0.5*(1-f.ErrorRate) + 0.3*g.latencyFactor(f.P95LatencyMs) + 0.2*(1-overload))
	tier := ""
	for _, t := range g.Rules.Tiers {
		if g.Users >= t.MinUsers {
			tier = t.Name
		}
	}
	return Meters{
		Tick: g.Tick, Day: day, Hour: hour,
		RPS: f.RPS, SuccessRPS: f.SuccessRPS,
		Users: g.Users, ActiveUsers: g.Users * g.Rules.ActiveShare, Engagement: g.engagement(),
		P95LatencyMs: f.P95LatencyMs, ErrorRate: f.ErrorRate, Health: health,
		Satisfaction: g.Satisfaction, Popularity: g.Popularity, Complexity: cx, Tier: tier,
		RevenuePerHour: f.SuccessRPS * 3600 * g.Rules.RevenuePerRequest, CostPerHour: cost, Cash: g.Cash,
	}
}

// Step simulates one tick and returns its snapshot.
func (g *Game) Step() Snapshot {
	if g.Status != StatusRunning {
		return g.Last
	}
	r := g.Rules
	snap := g.solve()
	f := snap.Flow
	for i, n := range g.Nodes {
		n.Backlog = snap.backlog[i]
	}

	before := g.meters(f)
	hours := r.TickSeconds / 3600
	g.Cash += (before.RevenuePerHour - before.CostPerHour) * hours

	g.Satisfaction += (g.quality(f) - g.Satisfaction) * r.SatisfactionPull
	g.Popularity += (g.Satisfaction - g.Popularity) * r.PopularityPull
	growth := r.GrowthRate * g.Popularity / 100 * g.Users * (1 - g.Users/r.MarketSize)
	churn := g.Users * r.ChurnRate * math.Pow(1-g.Satisfaction/100, 2)
	g.Users = math.Max(1, g.Users+growth-churn)

	if g.Cash < 0 {
		g.negativeFor++
		if g.negativeFor > r.BankruptcyGraceTicks {
			g.Status = StatusBankrupt
		}
	} else {
		g.negativeFor = 0
	}

	snap.Meters = g.meters(f)
	g.Tick++
	g.Last = snap
	g.History = append(g.History, snap.Meters)
	if len(g.History) > historyLimit {
		g.History = g.History[len(g.History)-historyLimit:]
	}
	return snap
}

// preview solves the current tick without advancing the world, so a
// command's effect is visible before the next tick runs.
func (g *Game) preview() Snapshot {
	snap := g.solve()
	snap.Meters = g.meters(snap.Flow)
	return snap
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
