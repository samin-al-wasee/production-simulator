# Development Loop

The standard loop for turning a requirement into a merged-quality change. It is agent-agnostic: Claude Code, OpenCode, or a human contributor all follow the same steps. Repository rules in [`AGENTS.md`](../AGENTS.md) always apply; this loop only orders the work.

```text
 requirement ──► plan ──► implement ──► review ──► test ──► document ──┐
     ▲                                                                 │
     └──────────────── next requirement / fix findings ◄───────────────┘
```

Each pass ends with a **checkpoint**: report status to the user and wait for the next requirement or an explicit "continue". Do not chain passes unattended.

## Steps

### 1. Requirement

- Restate the requirement in one or two sentences and list assumptions.
- Read `README.md`, `ROADMAP.md`, and the folder READMEs the change touches.
- Confirm the current roadmap phase authorizes the work. If not, stop and ask (AGENTS.md §13).
- Resolve ambiguity by asking, not guessing.

### 2. Plan

State, before writing anything:

- files to create or change,
- docs to update (use the table in AGENTS.md §5),
- whether an ADR is needed (AGENTS.md §6),
- the verification plan.

For non-trivial work, get the user's approval on the plan.

### 3. Implement

- Smallest diff that satisfies the plan; unrelated cleanups go to a separate change.
- Match neighboring files. No new dependencies without asking.
- Keep simulation core, runtime orchestration, and dashboard separable.

### 4. Review

- Read your own `git diff` first, including untracked files.
- Run a second-pair-of-eyes review: `/review` in OpenCode (`code-reviewer` subagent) or `/code-review` in Claude Code.
- Fix confirmed findings, then return to step 3 only for the fixes.

### 5. Test

- Run `make check` (or the individual targets):
  - Go, from `core/`: `go test ./...`, `gofmt -l .`, `go vet ./...`
  - Next.js, once it exists: `lint` and `typecheck` scripts
- Add or update tests for new behavior; the simulation core must stay testable headlessly.
- Report results honestly. A failing suite is never "probably fine". On failure, diagnose the root cause, fix it, and re-run.

### 6. Document

Docs and code ship together. Walk the AGENTS.md §5 table and update every matching document, plus `CHANGELOG.md` and `ROADMAP.md` when a phase item moves. Docs updated is part of done.

### 7. Repeat

Summarize what changed, what was verified, and what remains. Then either take the next requirement or stop.

## Exit criteria for one pass

- [ ] Plan matched what was built (or deviations are stated)
- [ ] Review findings resolved or explicitly deferred
- [ ] `make check` passes, or failures are reported verbatim
- [ ] Docs, changelog, and roadmap updated
- [ ] No commit, push, branch, or PR made unless the user asked

## Loop limits

- If the same test or review finding fails three fix attempts in a row, stop and report instead of continuing.
- Never run destructive, cloud-costing, or irreversible commands as part of the loop.

## Invoking the loop

| Agent | How |
|---|---|
| OpenCode | `/dev-loop <requirement>` ([`.opencode/command/dev-loop.md`](../.opencode/command/dev-loop.md)) |
| Claude Code | `/dev-loop <requirement>` ([`.claude/commands/dev-loop.md`](../.claude/commands/dev-loop.md)) |
| Any other agent or human | Follow this document top to bottom |

## Autonomous mode

When the user explicitly asks for uninterrupted work (for example "continue until the roadmap is finished, don't ask questions"), the loop runs without approval checkpoints:

- Decide using the source-of-truth priority in AGENTS.md §5; record judgment calls in an ADR or the changelog instead of asking.
- Plan approval and per-pass checkpoints are skipped; the plan is still written down (in the ADR, docs, or final report).
- Documentation conflicts are resolved in favor of the higher-priority source and the resolution is documented in the same change.
- Still never done without an explicit instruction: commit, push, open PRs, change permission settings, run destructive commands.
- Stop conditions still apply: three failed fix attempts on the same problem.

Autonomous mode ends when the user's request is fulfilled or the user says otherwise.
