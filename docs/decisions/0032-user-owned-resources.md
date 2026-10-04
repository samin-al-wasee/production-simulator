# ADR-0032: User-owned sandboxes and progress, with an anonymous fallback

**Status:** accepted
**Date:** 2026-10-04

## Context

[ADR-0030](0030-application-store-and-postgres.md) gave the application layer a Postgres store and named ownership, sandboxes, and progress as its data; [ADR-0031](0031-oauth-identity.md) gave it OAuth sessions. Phase 13.1c is where that data becomes real: a player's saved games and learning progress must belong to the player, and one player must not reach another's.

Two constraints shape how far that can go now. The dashboard has no sign-in UI until 13.1d, so gating the API behind a session would leave the product unusable in the meantime. And the repository's standing rule is that the core runs **without a database** (ADR-0030): the CLI, the simulation packages, and their tests never need one. A design that only works with a session and a database would break both.

## Decision

1. **Owned data is keyed by the session user id.** Saved sandboxes (`sandboxes`) and learning progress (`learning_progress`) each carry `user_id`. Every read, update, and delete filters by it; a row owned by someone else is reported as not found (`404`), never as forbidden, so existence is not confirmed.
2. **A saved sandbox is the replayable record.** It stores the **(ruleset version, seed, command log, tick)** — the same triple a live game is a function of (ADR-0030) — augmented with a name. Resuming replays it; nothing stores a tick stream.
3. **Live games stay in memory,** owned when a session is present; save, resume, list, and delete go through `/api/v1/sandbox/saves`. An anonymous request may create and drive a game but cannot save or resume.
4. **A request without a session persists nothing in store mode.** Learning renders with empty progress, and save/complete/resume answer `401` ("sign in to save"). It is a transient pre-login state, not a second class of user.
5. **Without a store the local behavior is unchanged.** With no `DATABASE_URL` the API has no identity, uses the `.forgelab` progress and save files exactly as before, and stays usable by the CLI and DB-free development. In store mode it never touches `.forgelab`.
6. **Anonymous persistence keeps working only where a store exists to replace it.** Once 13.1d makes sign-in the product's front door, the anonymous in-memory path remains for local/store-less runs.

## Consequences

### Positive

* A player's games and progress are private by construction: the owner is read from the session, never the request, and every owned query is filtered.
* The determinism contract holds: the database stores the replay triple, never simulation truth.
* ADR-0030's "runs without a database" survives: local development, the CLI, and the simulation tests are untouched.
* No sign-in UI is needed to keep the game playable before 13.1d.

### Negative / Trade-offs

* Two persistence paths (store per user, or local file) until 13.1d, and store mode makes anonymous save/progress a `401` — a visible change for anyone running the devcontainer (`DATABASE_URL` set) without signing in. The browser tests run the API with `-database=` to stay on the file path they assert.
* Live-game ownership is process-local: a restart loses every live game, and ownership is not enforced across replicas.
* `sandboxes` and `learning_progress` grow with users; no retention policy yet.

## Alternatives Considered

* **Require sign-in everywhere** — rejected for now: it breaks the CLI, DB-free development, and the browser tests until 13.1d ships the login UI.
* **Per-user files instead of database rows** — rejected: the product is hosted and multi-user; file ownership does not scale and duplicates what ADR-0030 chose Postgres for.
* **Persist live games (tick streams) in the database** — rejected: it would break the replay contract and add volume with no truth.
* **Anonymous users get a real account** — rejected: identity is OAuth-only (ADR-0031); there is no password or guest-account path.

## References

* [ADR-0030](0030-application-store-and-postgres.md) (application store), [ADR-0031](0031-oauth-identity.md) (identity)
* [`docs/architecture.md`](../architecture.md), [`docs/security.md`](../security.md), [`ROADMAP.md`](../../ROADMAP.md) Phase 13
