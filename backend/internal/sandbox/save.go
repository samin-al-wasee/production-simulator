package sandbox

import "fmt"

// Save is everything needed to reproduce a game: the ruleset version, the
// seed, the ordered command log, and how many ticks have run.
type Save struct {
	Ruleset string `json:"ruleset"`
	Seed    int64  `json:"seed"`
	// FreeBuild games start with every kind unlocked.
	FreeBuild bool            `json:"freeBuild,omitempty"`
	Tick      int             `json:"tick"`
	Log       []LoggedCommand `json:"log"`
}

// Save returns the game as a replayable record.
func (g *Game) Save() Save {
	return Save{Ruleset: g.Rules.Version, Seed: g.Seed, FreeBuild: g.FreeBuild, Tick: g.Tick, Log: append([]LoggedCommand(nil), g.Log...)}
}

// Replay rebuilds a game by stepping to each command's tick and applying it.
func Replay(s Save) (*Game, error) {
	rules, err := Rulesets(s.Ruleset)
	if err != nil {
		return nil, err
	}
	g := New(rules, s.Seed)
	g.FreeBuild = s.FreeBuild
	for i, lc := range s.Log {
		if lc.Tick < g.Tick || lc.Tick > s.Tick {
			return nil, fmt.Errorf("command %d: tick %d out of order", i, lc.Tick)
		}
		for g.Tick < lc.Tick && g.Status == StatusRunning {
			g.Step()
		}
		if _, err := g.Apply(lc.Command); err != nil {
			return nil, fmt.Errorf("command %d: %w", i, err)
		}
	}
	for g.Tick < s.Tick && g.Status == StatusRunning {
		g.Step()
	}
	return g, nil
}
