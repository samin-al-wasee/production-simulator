package sandbox

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func newGameV4(t *testing.T) *Game {
	t.Helper()
	r := RulesetV4()
	r.Events = nil
	for i := range r.Kinds {
		r.Kinds[i].UnlockedBy = ""
	}
	g := New(r, 1)
	g.Cash = 1e9
	return g
}

// defaults returns a copy of the v4 starting configuration.
func defaults() TrafficConfig {
	var tc TrafficConfig
	b, _ := json.Marshal(RulesetV4().Traffic)
	_ = json.Unmarshal(b, &tc)
	return tc
}

func loadTest(p Pattern) TrafficConfig {
	tc := defaults()
	tc.Source = SourceConfigured
	tc.Pattern = p
	return tc
}

func configure(g *Game, tc TrafficConfig) error {
	_, err := g.Apply(Command{Type: CmdConfigure, Node: InternetID, Traffic: &tc})
	return err
}

func mustConfigure(t *testing.T, g *Game, tc TrafficConfig) {
	t.Helper()
	if err := configure(g, tc); err != nil {
		t.Fatal(err)
	}
}

// series steps the game and returns the real RPS of each tick.
func series(g *Game, ticks int) []float64 {
	out := make([]float64, ticks)
	for i := range out {
		out[i] = g.Step().Flow.RPS
	}
	return out
}

func rate(rs []Rate, name string) float64 {
	for _, r := range rs {
		if r.Name == name {
			return r.RPS
		}
	}
	return math.NaN()
}

func TestConstantTrafficIsALabelledLoadTest(t *testing.T) {
	g := newGameV4(t)
	basic(t, g)
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeConstant, RPS: 40}))
	for i, v := range series(g, 30) {
		if v != 40 {
			t.Fatalf("tick %d: constant traffic should stay at 40 RPS, got %v", i, v)
		}
	}
	m := g.Last.Meters
	if !m.LoadTest || g.Last.Flow.Traffic.Source != SourceConfigured {
		t.Fatalf("a configured source must be labelled a load test: %+v", g.Last.Flow.Traffic)
	}
}

func TestGroupsRegionsAndEndpointsSplitTheVolume(t *testing.T) {
	g := newGameV4(t)
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeConstant, RPS: 1000}))
	tr := g.Step().Flow.Traffic
	for name, want := range map[string]float64{"Web users": 700, "Mobile users": 200, "API clients": 80, "Bots": 20} {
		if got := rate(tr.Groups, name); !near(got, want, 1e-9) {
			t.Fatalf("group %s: want %v RPS, got %v", name, want, got)
		}
	}
	for name, want := range map[string]float64{"asia": 400, "europe": 300, "north-america": 200, "south-america": 100} {
		if got := rate(tr.Regions, name); !near(got, want, 1e-9) {
			t.Fatalf("region %s: want %v RPS, got %v", name, want, got)
		}
	}
	// 700×35% + 200×30% + 80×40% + 20×60%
	if got := rate(tr.Endpoints, "GET /products"); !near(got, 349, 1e-9) {
		t.Fatalf("GET /products: want 349 RPS, got %v", got)
	}
	sum := 0.0
	for _, e := range tr.Endpoints {
		sum += e.RPS
	}
	if !near(sum, 1000, 1e-9) || len(tr.Endpoints) != 6 {
		t.Fatalf("endpoints must account for every request: %+v", tr.Endpoints)
	}
}

func TestEndpointMixDecidesReadsWritesAndStorage(t *testing.T) {
	g := newGameV4(t)
	app, db, cache, st := place(t, g, KindApp), place(t, g, KindDBPrimary), place(t, g, KindCache), place(t, g, KindStorage)
	connect(t, g, InternetID, app)
	connect(t, g, app, cache)
	connect(t, g, cache, db)
	connect(t, g, app, db)
	connect(t, g, app, st)
	tc := loadTest(Pattern{Shape: ShapeConstant, RPS: 10})
	tc.Groups = []TrafficGroup{{Name: "All", Share: 1, Regions: []Weight{{"europe", 1}},
		Endpoints: []Weight{{"GET /profile", 0.5}, {"GET /media/:id", 0.2}, {"POST /orders", 0.3}}}}
	mustConfigure(t, g, tc)
	s := g.Step()
	if got := stats(t, s, cache).Offered; !near(got, 7, 1e-9) {
		t.Fatalf("reads (70%%) should go to the cache: %v", got)
	}
	if got := stats(t, s, st).Offered; !near(got, 2, 1e-9) {
		t.Fatalf("only the storage endpoint (20%%) should fetch objects: %v", got)
	}
	// Writes (3) plus the cache's misses on 7 reads.
	k, _ := g.Rules.Kind(KindCache)
	if got, want := stats(t, s, db).Offered, 3+7*(1-k.HitRatio); !near(got, want, 1e-9) {
		t.Fatalf("database should take writes and cache misses: want %v, got %v", want, got)
	}
}

func TestCDNAnswersOnlyCacheableReads(t *testing.T) {
	g := newGameV4(t)
	cdn, app := place(t, g, KindCDN), place(t, g, KindApp)
	connect(t, g, InternetID, cdn)
	connect(t, g, cdn, app)
	tc := loadTest(Pattern{Shape: ShapeConstant, RPS: 10})
	tc.Groups = []TrafficGroup{{Name: "All", Share: 1, Regions: []Weight{{"asia", 1}},
		Endpoints: []Weight{{"GET /products", 0.6}, {"GET /profile", 0.2}, {"POST /login", 0.2}}}}
	mustConfigure(t, g, tc)
	if got, want := stats(t, g.Step(), app).Offered, 10-6*0.45; !near(got, want, 1e-9) {
		t.Fatalf("the CDN should answer 45%% of the 6 cacheable RPS only: want %v, got %v", want, got)
	}
	tc.Groups[0].Endpoints = []Weight{{"POST /orders", 1}}
	mustConfigure(t, g, tc)
	if got := stats(t, g.Step(), app).Offered; !near(got, 10, 1e-9) {
		t.Fatalf("writes must all pass the CDN: %v", got)
	}
}

func TestPatterns(t *testing.T) {
	cases := []struct {
		name string
		p    Pattern
		want map[int]float64 // tick → RPS; one tick is 5 simulated minutes
	}{
		{"spike", Pattern{Shape: ShapeSpike, RPS: 100, PeakRPS: 500, StartMinutes: 30, Minutes: 60},
			map[int]float64{0: 100, 5: 100, 6: 500, 17: 500, 18: 100, 40: 100}},
		{"ramp up", Pattern{Shape: ShapeRamp, RPS: 100, PeakRPS: 200, Minutes: 50},
			map[int]float64{0: 100, 5: 150, 10: 200, 30: 200}},
		{"ramp down", Pattern{Shape: ShapeRamp, RPS: 200, PeakRPS: 100, Minutes: 50},
			map[int]float64{0: 200, 5: 150, 10: 100, 30: 100}},
		{"burst", Pattern{Shape: ShapeBurst, RPS: 100, PeakRPS: 900, Minutes: 10, PeriodMinutes: 30},
			map[int]float64{0: 900, 1: 900, 2: 100, 5: 100, 6: 900, 8: 100}},
		{"periodic", Pattern{Shape: ShapePeriodic, RPS: 100, PeakRPS: 300, PeriodMinutes: 60},
			map[int]float64{0: 100, 3: 200, 6: 300, 9: 200, 12: 100}},
		// The game starts at 08:00, before the first step, so the last holds.
		{"schedule", Pattern{Shape: ShapeSchedule, Schedule: []ScheduleStep{{9, 1000}, {12, 3000}, {22, 2000}}},
			map[int]float64{0: 2000, 11: 2000, 12: 1000, 48: 3000, 168: 2000, 204: 2000}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newGameV4(t)
			mustConfigure(t, g, loadTest(c.p))
			got := series(g, 205)
			for tick, want := range c.want {
				if !near(got[tick], want, 1e-9) {
					t.Fatalf("tick %d: want %v RPS, got %v", tick, want, got[tick])
				}
			}
		})
	}
}

func TestConfiguringRestartsThePatternClock(t *testing.T) {
	g := newGameV4(t)
	series(g, 10)
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeSpike, RPS: 1, PeakRPS: 9, Minutes: 10}))
	if got := series(g, 3); !reflect.DeepEqual(got, []float64{9, 9, 1}) {
		t.Fatalf("a spike starting at 0 should run from the configure tick: %v", got)
	}
}

func TestZeroTrafficIsQuiet(t *testing.T) {
	g := newGameV4(t)
	basic(t, g)
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeConstant}))
	f := g.Step().Flow
	if f.RPS != 0 || f.SuccessRPS != 0 || f.ErrorRate != 0 || f.P95LatencyMs != 0 || f.Traffic.Concurrency != 0 {
		t.Fatalf("zero traffic should serve and fail nothing: %+v", f)
	}
	for _, n := range f.Nodes {
		if n.Offered != 0 || math.IsNaN(n.Utilization) {
			t.Fatalf("no node should see load: %+v", n)
		}
	}
}

func TestConcurrencyFollowsLittlesLaw(t *testing.T) {
	g := newGameV4(t)
	basic(t, g)
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeConstant, RPS: 20}))
	f := g.Step().Flow
	if want := 20 * f.MeanLatencyMs / 1000; f.Traffic.Concurrency <= 0 || !near(f.Traffic.Concurrency, want, 1e-9) {
		t.Fatalf("concurrency should be RPS × mean latency: want %v, got %v", want, f.Traffic.Concurrency)
	}
}

func TestRetriesAmplifyLoadAndRecoverRequests(t *testing.T) {
	// Internet → app → database with no object storage: the 20% of requests
	// that need storage fail on every attempt, so retrying them only adds load.
	build := func(retries int) *Game {
		g := newGameV4(t)
		app, db := place(t, g, KindApp), place(t, g, KindDBPrimary)
		connect(t, g, InternetID, app)
		connect(t, g, app, db)
		tc := loadTest(Pattern{Shape: ShapeConstant, RPS: 10})
		tc.Groups = []TrafficGroup{{Name: "All", Share: 1, Retries: retries, Regions: []Weight{{"asia", 1}},
			Endpoints: []Weight{{"GET /profile", 0.8}, {"GET /media/:id", 0.2}}}}
		mustConfigure(t, g, tc)
		return g
	}
	none := build(0)
	series(none, 3)
	if f := none.Last.Flow; f.Traffic.RetryRPS != 0 || !near(f.ErrorRate, 0.2, 1e-9) {
		t.Fatalf("without retries a fifth of requests fail and nothing is retried: %+v", f)
	}
	two := build(2)
	first := two.Step().Flow
	if first.Traffic.RetryRPS != 0 {
		t.Fatalf("clients retry only after failures have been seen: %+v", first.Traffic)
	}
	f := two.Step().Flow
	// The 2 RPS of storage requests fail every attempt and are sent 3 times.
	if !near(f.Traffic.RetryRPS, 4, 1e-9) || !near(stats(t, two.Last, "app-instance-1").Offered, 14, 1e-9) {
		t.Fatalf("each failing request should be sent twice more: %+v", f.Traffic)
	}
	// The storage requests fail on every attempt, so retries do not help them.
	if !near(f.ErrorRate, 0.2, 1e-9) {
		t.Fatalf("retrying a request that always fails still fails: %v", f.ErrorRate)
	}
}

func TestRetriesRecoverTransientFailures(t *testing.T) {
	// A third-party outage fails 30% of attempts independently each time.
	g := newGameV4(t)
	basic(t, g)
	tc := loadTest(Pattern{Shape: ShapeConstant, RPS: 10})
	tc.Groups = []TrafficGroup{{Name: "All", Share: 1, Retries: 1, Regions: []Weight{{"asia", 1}},
		Endpoints: []Weight{{"GET /profile", 1}}}}
	mustConfigure(t, g, tc)
	inject(g, &Event{Effect: EffectThirdParty, Start: 0, End: 100, Magnitude: 0.3})
	f := g.Step().Flow
	if !near(f.ErrorRate, 0.09, 1e-9) {
		t.Fatalf("one retry should cut a 30%% failure to 9%%: %v", f.ErrorRate)
	}
}

func TestLoadTestEarnsNothingAndHoldsTheMarket(t *testing.T) {
	g := newGameV4(t)
	basic(t, g)
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeConstant, RPS: 20}))
	users, sat, pop := g.Users, g.Satisfaction, g.Popularity
	cash := g.Cash
	series(g, 50)
	m := g.Last.Meters
	if m.RevenuePerHour != 0 || g.Cash >= cash || g.Users != users || g.Satisfaction != sat || g.Popularity != pop {
		t.Fatalf("a load test should cost money and change nothing else: %+v", m)
	}
	if len(g.Achieved) != 0 {
		t.Fatalf("a load test should not reach goals: %v", g.Achieved)
	}
	tc := defaults()
	mustConfigure(t, g, tc)
	series(g, 5)
	if g.Last.Meters.LoadTest || g.Last.Meters.RevenuePerHour <= 0 || g.Users == users {
		t.Fatalf("back on the market source the game should earn and grow: %+v", g.Last.Meters)
	}
}

func TestLoadTestsDoNotCountTowardsGoals(t *testing.T) {
	g := newGameV4(t)
	basic(t, g)
	// An event judged after running into a load test is marked and not counted.
	e := inject(g, &Event{Effect: EffectAttack, End: 2, Magnitude: 5})
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeConstant}))
	series(g, 1)
	mustConfigure(t, g, defaults())
	series(g, 2+g.Rules.RecoveryTicks)
	if e.Outcome != OutcomeRecovered || !e.LoadTest {
		t.Fatalf("the event should be judged and marked: %+v", e)
	}
	if v := g.value(Condition{Type: CondEvent, Name: ""}); v != 0 {
		t.Fatalf("an event met during a load test must not count: %v", v)
	}

	// A goal's hold streak starts over after a load test.
	g.streak["healthy-margin"] = 11
	mustConfigure(t, g, loadTest(Pattern{Shape: ShapeConstant}))
	series(g, 1)
	if len(g.streak) != 0 {
		t.Fatalf("a load test must reset goal streaks: %v", g.streak)
	}
}

func TestMarketSourceKeepsTheUserModel(t *testing.T) {
	v3, v4 := New(RulesetV3(), 1), New(RulesetV4(), 1)
	f3, f4 := v3.Step().Flow, v4.Step().Flow
	if f3.RPS != f4.RPS || f4.Traffic.Source != SourceMarket || len(f4.Traffic.Groups) != 4 {
		t.Fatalf("v4 market traffic should match v3 volume, with a breakdown: %+v vs %+v", f3, f4.Traffic)
	}
	if len(f3.Traffic.Groups) != 0 || f3.Traffic.RPS != f3.RPS {
		t.Fatalf("a ruleset without traffic configuration reports only the volume: %+v", f3.Traffic)
	}
}

func TestTrafficValidation(t *testing.T) {
	pattern := func(p Pattern) func(*TrafficConfig) {
		return func(tc *TrafficConfig) {
			tc.Source = SourceConfigured
			tc.Pattern = p
		}
	}
	cases := []struct {
		name string
		edit func(*TrafficConfig)
		want string
	}{
		{"unknown source", func(tc *TrafficConfig) { tc.Source = "moon" }, "source must be"},
		{"negative rps", pattern(Pattern{Shape: ShapeConstant, RPS: -1}), "rate must be between 0 and 1000000"},
		{"rps above the limit", pattern(Pattern{Shape: ShapeConstant, RPS: 1_000_001}), "rate must be between"},
		{"nan rps", pattern(Pattern{Shape: ShapeConstant, RPS: math.NaN()}), "rate must be between"},
		{"unknown shape", pattern(Pattern{Shape: "zigzag"}), "pattern shape must be"},
		{"short ramp", pattern(Pattern{Shape: ShapeRamp, RPS: 1, PeakRPS: 2, Minutes: 1}), "ramp duration must be at least 5 minutes"},
		{"burst never off", pattern(Pattern{Shape: ShapeBurst, RPS: 1, PeakRPS: 2, Minutes: 10, PeriodMinutes: 12}),
			"time between bursts must be at least 5 minutes"},
		{"negative spike start", pattern(Pattern{Shape: ShapeSpike, RPS: 1, PeakRPS: 2, StartMinutes: -5, Minutes: 10}), "spike start"},
		{"short period", pattern(Pattern{Shape: ShapePeriodic, PeriodMinutes: 5}), "period must be at least 10 minutes"},
		{"empty schedule", pattern(Pattern{Shape: ShapeSchedule}), "1 to 24 steps"},
		{"schedule out of order", pattern(Pattern{Shape: ShapeSchedule, Schedule: []ScheduleStep{{12, 1}, {9, 1}}}), "step 2: hours must increase"},
		{"schedule hour 24", pattern(Pattern{Shape: ShapeSchedule, Schedule: []ScheduleStep{{24, 1}}}), "hour must be from 0"},
		{"shares under 100", func(tc *TrafficConfig) { tc.Groups[0].Share = 0.69 }, "traffic group shares must sum to 100% (now 99%)"},
		{"negative share", func(tc *TrafficConfig) { tc.Groups[0].Share, tc.Groups[1].Share = -0.1, 1 }, "each share must be between 0% and 100%"},
		{"no groups", func(tc *TrafficConfig) { tc.Groups = nil }, "declare 1 to 8 traffic groups"},
		{"blank name", func(tc *TrafficConfig) { tc.Groups[0].Name = "  " }, "group 1: name must be"},
		{"padded name", func(tc *TrafficConfig) { tc.Groups[0].Name = " Web" }, "group 1: name must be"},
		{"duplicate name", func(tc *TrafficConfig) { tc.Groups[1].Name = "web USERS" }, `group "web USERS" is declared twice`},
		{"too many retries", func(tc *TrafficConfig) { tc.Groups[0].Retries = 4 }, "retries must be between 0 and 3"},
		{"negative retries", func(tc *TrafficConfig) { tc.Groups[0].Retries = -1 }, "retries must be between 0 and 3"},
		{"unknown endpoint", func(tc *TrafficConfig) { tc.Groups[0].Endpoints[0].Name = "GET /nope" }, `unknown "GET /nope"`},
		{"endpoint mix off", func(tc *TrafficConfig) { tc.Groups[3].Endpoints[0].Share = 0.5 }, `group "Bots" endpoints must sum to 100% (now 90%)`},
		{"unknown region", func(tc *TrafficConfig) { tc.Groups[0].Regions[0].Name = "mars" }, `group "Web users" regions: unknown "mars"`},
		{"no regions", func(tc *TrafficConfig) { tc.Groups[0].Regions = nil }, "regions: list at least one"},
		{"repeated region", func(tc *TrafficConfig) {
			tc.Groups[0].Regions = []Weight{{"asia", 0.5}, {"asia", 0.5}}
		}, `"asia" is listed twice`},
		{"cacheable write", func(tc *TrafficConfig) { tc.Endpoints[4].Cacheable = true }, "only GET requests can be cacheable"},
		{"bad method", func(tc *TrafficConfig) { tc.Endpoints[0].Method = "get" }, "method must be one of"},
		{"bad path", func(tc *TrafficConfig) { tc.Endpoints[0].Path = "products" }, "path must start with /"},
		{"long path", func(tc *TrafficConfig) { tc.Endpoints[0].Path = "/" + strings.Repeat("a", 40) }, "endpoint 1: path must be at most 40 characters"},
		{"duplicate endpoint", func(tc *TrafficConfig) { tc.Endpoints[1] = tc.Endpoints[0] }, "GET /products is declared twice"},
		{"no endpoints", func(tc *TrafficConfig) { tc.Endpoints = nil }, "declare 1 to 12 endpoints"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newGameV4(t)
			tc := defaults()
			c.edit(&tc)
			err := configure(g, tc)
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want an invalid-command error containing %q, got %v", c.want, err)
			}
			if g.Node(InternetID).Traffic != nil || len(g.Log) != 0 {
				t.Fatal("a rejected configuration must change nothing")
			}
		})
	}
}

func TestTrafficValidationBoundaries(t *testing.T) {
	g := newGameV4(t)
	tc := defaults()
	// 70% + 20% + 8% + 2% is not exactly 1 in floating point.
	tc.Groups[0].Retries = g.Rules.MaxRetries
	tc.Source = SourceConfigured
	tc.Pattern = Pattern{Shape: ShapeBurst, RPS: 0, PeakRPS: g.Rules.MaxTrafficRPS, Minutes: 5, PeriodMinutes: 10}
	mustConfigure(t, g, tc)
	tc.Pattern = Pattern{Shape: ShapeSchedule, Schedule: []ScheduleStep{{0, 0}, {23.99, 1}}}
	mustConfigure(t, g, tc)
	// The market ignores the pattern, so an unfinished one does not block it.
	tc.Source = SourceMarket
	tc.Pattern = Pattern{Shape: ShapeBurst, Minutes: 10, PeriodMinutes: 10}
	mustConfigure(t, g, tc)
	tc.Source = SourceConfigured
	if err := configure(g, tc); err == nil {
		t.Fatal("the same pattern must be rejected once it drives a load test")
	}
	tc.Source = SourceMarket

	// Every problem is reported at once.
	tc.Groups[0].Share, tc.Groups[0].Retries = 0.1, 9
	problems := g.Rules.Validate(tc)
	if len(problems) != 2 {
		t.Fatalf("want both problems, got %q", problems)
	}

	for _, c := range []Command{
		{Type: CmdConfigure, Node: InternetID},
		{Type: CmdConfigure, Node: "app-instance-1", Traffic: &tc},
	} {
		if _, err := g.Apply(c); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%+v should be rejected, got %v", c, err)
		}
	}
	v3 := New(RulesetV3(), 1)
	if err := configure(v3, defaults()); err == nil || !strings.Contains(err.Error(), "has no traffic configuration") {
		t.Fatalf("a ruleset without traffic configuration cannot be configured: %v", err)
	}
}

func TestConfiguredGameReplaysThroughJSON(t *testing.T) {
	g := New(RulesetV4(), 3)
	basic(t, g)
	series(g, 20)
	tc := loadTest(Pattern{Shape: ShapeSpike, RPS: 20, PeakRPS: 120, StartMinutes: 15, Minutes: 30})
	tc.Groups[0].Retries = 2
	mustConfigure(t, g, tc)
	series(g, 40)
	mustConfigure(t, g, defaults())
	series(g, 300)

	b, err := json.Marshal(g.Save())
	if err != nil {
		t.Fatal(err)
	}
	var s Save
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	r, err := Replay(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Last, g.Last) || !reflect.DeepEqual(r.Nodes, g.Nodes) || r.Cash != g.Cash {
		t.Fatalf("replay diverged:\n%+v\n%+v", r.Last.Meters, g.Last.Meters)
	}
}

// TestRulesetV4Balance checks that the default traffic mix keeps the v2
// economy: sensible headroom pays, and none or far too much costs money.
func TestRulesetV4Balance(t *testing.T) {
	sensible, broke := week(t, RulesetV4, 1.5)
	if broke > 0 || sensible <= RulesetV4().StartingCash {
		t.Fatalf("a sensibly provisioned design should stay solvent and grow: cash %.0f, %d bankrupt", sensible, broke)
	}
	if none, _ := week(t, RulesetV4, 1); none >= sensible {
		t.Fatalf("no headroom should lose money at peaks: %.0f vs %.0f", none, sensible)
	}
	if over, _ := week(t, RulesetV4, 5); over >= 0.7*sensible {
		t.Fatalf("5× over-provisioning should cost at least 30%% of the profit: %.0f vs %.0f", over, sensible)
	}
}
