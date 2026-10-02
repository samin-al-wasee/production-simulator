package sandbox

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/replay.golden")

// frozen are the rulesets whose games must replay exactly as recorded. A
// new ruleset is added here once it is final; changing a frozen one fails.
var frozen = []func() *Ruleset{RulesetV1, RulesetV2, RulesetV3, RulesetV4, RulesetV5, RulesetV6, RulesetV7, RulesetV8, RulesetV9, RulesetV10, RulesetV11, RulesetV12}

// TestRulesetsReplayAsRecorded plays the same design for four simulated days
// under every frozen ruleset and compares each tick's state with the hash
// recorded in testdata/replay.golden. Run `go test -run Replay -update` only
// when a ruleset is frozen for the first time.
func TestRulesetsReplayAsRecorded(t *testing.T) {
	var got strings.Builder
	for _, rs := range frozen {
		for _, seed := range []int64{1, 7} {
			g := New(rs(), seed)
			g.Cash = 1e9
			basic(t, g)
			h := sha256.New()
			for i := 0; i < 288*4; i++ {
				if i%12 == 0 {
					provision(g, 1.2)
				}
				s := g.Step()
				b, _ := json.Marshal(struct {
					S Snapshot
					C float64
					U float64
					E []*Event
				}{s, g.Cash, g.Users, g.Events})
				h.Write(b)
			}
			fmt.Fprintf(&got, "%s %d %x\n", rs().Version, seed, h.Sum(nil))
		}
	}
	const path = "testdata/replay.golden"
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("a frozen ruleset replays differently:\n got\n%s want\n%s", got.String(), want)
	}
}
