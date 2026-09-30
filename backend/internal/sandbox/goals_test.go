package sandbox

import (
	"errors"
	"reflect"
	"testing"
)

func newGameV3(t *testing.T) *Game {
	t.Helper()
	g := New(RulesetV3(), 1)
	g.Cash = 1e9
	return g
}

func TestRulesetV3GoalsAreWellFormed(t *testing.T) {
	r := RulesetV3()
	cards := map[string]bool{"": true}
	for _, c := range r.Events {
		cards[c.Name] = true
	}
	for _, k := range r.Kinds {
		if _, ok := r.Goal(k.UnlockedBy); k.UnlockedBy != "" && !ok {
			t.Errorf("%s unlocked by unknown goal %q", k.Name, k.UnlockedBy)
		}
	}
	for _, gl := range r.Goals {
		if _, ok := r.Goal(gl.Requires); gl.Requires != "" && !ok {
			t.Errorf("%s requires unknown goal %q", gl.ID, gl.Requires)
		}
		if len(gl.Conditions) == 0 || gl.Title == "" {
			t.Errorf("%s needs a title and conditions", gl.ID)
		}
		for _, c := range gl.Conditions {
			ok := c.Min != nil || c.Max != nil
			switch c.Type {
			case CondMeter:
				_, known := meterValue(Meters{}, c.Name)
				ok = ok && known
			case CondKind:
				_, known := r.Kind(c.Name)
				ok = ok && known && map[string]bool{"served": true, "utilization": true, "replicas": true, "backlog": true}[c.Stat]
			case CondEvent:
				ok = ok && cards[c.Name]
			default:
				ok = false
			}
			if !ok {
				t.Errorf("%s: bad condition %+v", gl.ID, c)
			}
		}
	}
	if len(RulesetV2().Goals) != 0 {
		t.Fatal("sandbox/v2 has no goals")
	}
}

func TestGoalsUnlockKinds(t *testing.T) {
	g := newGameV3(t)
	if _, err := g.Apply(Command{Type: CmdPlace, Kind: KindLB}); !errors.Is(err, ErrInvalid) {
		t.Fatal("a load balancer is locked until the first request")
	}
	basic(t, g)
	g.Step()
	if !g.Reached("first-request") {
		t.Fatal("a working system reaches the first goal on its first tick")
	}
	place(t, g, KindLB)
	if _, err := g.Apply(Command{Type: CmdPlace, Kind: KindCache}); !errors.Is(err, ErrInvalid) {
		t.Fatal("a cache is locked until the startup tier")
	}
	g.Users = 20_000
	g.Step()
	place(t, g, KindCache)
	var st GoalStatus
	for _, s := range g.Goals() {
		if s.ID == "startup-tier" {
			st = s
		}
	}
	if st.AchievedAt == nil || !reflect.DeepEqual(st.Unlocks, []string{KindGateway, KindWorker, KindDBReplica, KindCache, KindQueue}) || !st.Conditions[0].Met {
		t.Fatalf("startup status: %+v", st)
	}
}

func TestGoalsHoldRequireAndCompare(t *testing.T) {
	g := newGameV3(t)
	app, db := basic(t, g)
	g.Users = 400_000
	must(t, g, Command{Type: CmdScale, Node: app, Replicas: 50})
	g.Step()
	if !g.Reached("database-bottleneck") || g.Reached("saturate-app") {
		t.Fatalf("a saturated database behind idle apps: %v", g.Achieved)
	}
	must(t, g, Command{Type: CmdScale, Node: db, Replicas: 10})

	q := place(t, g, KindQueue)
	w := place(t, g, KindWorker)
	connect(t, g, app, q)
	connect(t, g, w, db)
	g.Step()
	if g.Reached("drain-backlog") || !g.Reached("backlog") {
		t.Fatalf("writes without workers build a backlog: %v", g.Achieved)
	}
	connect(t, g, q, w)
	must(t, g, Command{Type: CmdScale, Node: w, Replicas: 50})
	run(g, 3)
	if !g.Reached("drain-backlog") {
		t.Fatalf("workers drain the backlog: backlog %v", g.Node(q).Backlog)
	}

	if g.Reached("profitable-day") {
		t.Fatal("a profitable day needs a day of history")
	}
	run(g, 288)
	if !g.Reached("profitable-day") {
		t.Fatalf("cash %v should have grown over a day", g.Cash)
	}
}

func TestGoalMustHoldForItsTicks(t *testing.T) {
	r := RulesetV3()
	r.Goals = []Goal{{ID: "steady", Title: "Steady", HoldTicks: 3,
		Conditions: []Condition{{Type: CondMeter, Name: "successRps", Min: at(0.001)}}}}
	g := New(r, 1)
	basic(t, g)
	run(g, 2)
	if g.Reached("steady") {
		t.Fatal("two ticks are not three")
	}
	g.Step()
	if g.Achieved["steady"] != 3 {
		t.Fatalf("reached after three ticks in a row: %v", g.Achieved)
	}
}

func TestEventGoalsAndReplay(t *testing.T) {
	g := newGameV3(t)
	g.Cash = g.Rules.StartingCash
	app, _ := basic(t, g)
	run(g, 3)
	inject(g, &Event{Card: "zone-outage", Effect: EffectZone, End: g.Tick + 3, Hits: []Hit{{Node: app, Replicas: 1}}})
	run(g, 3+g.Rules.RecoveryTicks)
	if !g.Reached("survive-zone-outage") || !g.Reached("recover-incident") || g.Reached("weather-ddos") {
		t.Fatalf("recovering from a zone outage: %v", g.Achieved)
	}

	h := newGameV3(t)
	h.Cash = h.Rules.StartingCash
	basic(t, h)
	run(h, 288*3)
	r, err := Replay(h.Save())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Achieved) == 0 || !reflect.DeepEqual(r.Achieved, h.Achieved) {
		t.Fatalf("a replay reaches the same goals at the same ticks: %v vs %v", r.Achieved, h.Achieved)
	}
}
