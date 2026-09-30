"use client";

import { describeEvent, eventTiming, formatDuration, type GameState, type Ruleset } from "@/lib/sandbox";

const RECENT = 5;

// SandboxEvents lists the events the engine drew: what is coming, what is
// happening, and how recent ones were judged.
export function SandboxEvents({ game, rules }: { game: GameState; rules: Ruleset }) {
  const events = game.events ?? [];
  const live = events.filter((e) => e.phase !== "over");
  const past = events.filter((e) => e.phase === "over").slice(-RECENT).reverse();
  const grace = (rules.eventGraceTicks ?? 0) - game.tick;
  const timing = (e: (typeof events)[number]) => eventTiming(e, game.tick, rules.tickSeconds, rules.recoveryTicks ?? 0);

  return (
    <section className="sb-events" aria-label="Events">
      <h3>Events</h3>
      {live.length === 0 && (
        <span className="sb-hint">
          {grace > 0
            ? `Quiet for now. Surges, outages, and attacks can start in ${formatDuration(grace * rules.tickSeconds)}.`
            : "No incident in progress."}
        </span>
      )}
      {live.map((e) => (
        <div key={e.id} className={`sb-event phase-${e.phase}`} data-card={e.card}>
          <strong>{e.label}</strong>
          <span className="sb-event-phase">{e.phase}</span>
          <div className="sb-event-meta">{describeEvent(e, rules.kinds)}</div>
          <div className="sb-event-meta">{timing(e)}</div>
        </div>
      ))}
      {past.length > 0 && (
        <ul className="sb-event-past">
          {past.map((e) => (
            <li key={e.id} className={e.outcome === "recovered" ? "ok" : "bad"} data-card={e.card}>
              {e.label}: {timing(e)}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
