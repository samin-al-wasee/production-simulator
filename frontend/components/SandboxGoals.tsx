"use client";

import { conditionText, goalProgress, type GameState, type Ruleset } from "@/lib/sandbox";

const SHOWN = 3;

// SandboxGoals shows the next goals the engine is checking and how close the
// game is to each; reaching one can unlock component kinds.
export function SandboxGoals({ game, rules }: { game: GameState; rules: Ruleset }) {
  const goals = game.goals ?? [];
  if (goals.length === 0) return null;
  const reached = goals.filter((g) => g.achievedAt !== undefined).length;
  const open = goals.filter((g) => g.achievedAt === undefined && (!g.requires || goals.some((r) => r.id === g.requires && r.achievedAt !== undefined)));
  const label = (kind: string) => rules.kinds.find((k) => k.name === kind)?.label ?? kind;

  return (
    <section className="sb-goals" aria-label="Goals">
      <h3>
        Goals <span className="sb-goal-count">{reached}/{goals.length}</span>
      </h3>
      {game.meters.loadTest && (
        <span className="sb-hint">Paused during the load test: goals count real users only.</span>
      )}
      {open.length === 0 && <span className="sb-hint">Every goal reached.</span>}
      {open.slice(0, SHOWN).map((g) => (
        <div key={g.id} className="sb-goal" data-goal={g.id} title={g.description}>
          <strong>{g.title}</strong>
          {g.unlocks && g.unlocks.length > 0 && <span className="sb-goal-unlocks">unlocks {g.unlocks.map(label).join(", ")}</span>}
          <div className="sb-util" aria-hidden>
            <span className="ok" style={{ width: `${goalProgress(g) * 100}%` }} />
          </div>
          {g.conditions.map((c) => (
            <div key={c.label} className={`sb-event-meta ${c.met ? "ok" : ""}`}>
              {c.met ? "✓ " : ""}
              {conditionText(c)}
            </div>
          ))}
        </div>
      ))}
    </section>
  );
}
