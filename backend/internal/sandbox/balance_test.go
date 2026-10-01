package sandbox

import (
	"math"
	"testing"
)

// provision scales every component so its offered load would use 70% of its
// capacity after multiplying by headroom. Databases and storage never shrink.
func provision(g *Game, headroom float64) {
	for _, n := range g.Nodes {
		if n.Kind == KindInternet || n.Kind == KindTraffic {
			continue
		}
		var offered float64
		for _, s := range g.Last.Flow.Nodes {
			if s.ID == n.ID {
				offered = s.Offered
			}
		}
		k, _ := g.Rules.Kind(n.Kind)
		size, _ := g.Rules.Size(n.Size)
		want := int(math.Ceil(offered * headroom / (0.7 * k.Capacity * size.CapacityFactor)))
		want = max(1, min(want, g.Rules.MaxReplicas))
		if n.Kind == KindDBPrimary || n.Kind == KindStorage {
			want = max(want, n.Replicas)
		}
		if want != n.Replicas {
			// A player who cannot afford a replica simply waits.
			_, _ = g.Apply(Command{Type: CmdScale, Node: n.ID, Replicas: want})
		}
	}
}

// week plays the basic design for seven days, re-provisioning every hour,
// and returns the mean final cash and the number of bankruptcies.
func week(t *testing.T, rules func() *Ruleset, headroom float64) (float64, int) {
	t.Helper()
	const seeds = 10
	cash, bankrupt := 0.0, 0
	for seed := int64(1); seed <= seeds; seed++ {
		g := New(rules(), seed)
		basic(t, g)
		for i := 0; i < 288*7 && g.Status == StatusRunning; i++ {
			if i%12 == 0 {
				provision(g, headroom)
			}
			g.Step()
		}
		if g.Status == StatusBankrupt {
			bankrupt++
		}
		cash += g.Cash / seeds
	}
	return cash, bankrupt
}

// TestRulesetV2Balance pins the economy the v2 tuning was chosen for: sensible
// headroom pays best, both under- and over-provisioning cost money, and the
// event deck costs a player who never responds to it.
func TestRulesetV2Balance(t *testing.T) {
	calm := func() *Ruleset {
		r := RulesetV2()
		r.Events = nil
		return r
	}
	sensible, broke := week(t, RulesetV2, 1.5)
	if broke > 0 || sensible <= RulesetV2().StartingCash {
		t.Fatalf("a sensibly provisioned design should stay solvent and grow: cash %.0f, %d bankrupt", sensible, broke)
	}
	if none, _ := week(t, RulesetV2, 1); none >= sensible {
		t.Fatalf("no headroom should lose money at peaks: %.0f vs %.0f", none, sensible)
	}
	if over, _ := week(t, RulesetV2, 5); over >= 0.7*sensible {
		t.Fatalf("5× over-provisioning should cost at least 30%% of the profit: %.0f vs %.0f", over, sensible)
	}
	if quiet, _ := week(t, calm, 1.5); quiet <= sensible {
		t.Fatalf("unanswered events should cost money: %.0f with events, %.0f without", sensible, quiet)
	}
}
