# production-simulator

Agent working instructions for this repository.

## Project

Production simulator (product codename: ForgeLab).

## Stack (decided)

- **Backend / simulation core: Go.** Infrastructure tooling and the simulation
  engine live here.
- **Dashboard: Next.js** (TypeScript/web frontend).
- **Analytics/AI: Python (optional, later)** — a separate service/module, not
  woven into the Go core.

## Conventions

- Go: follow `https://github.com/golang-standards/project-layout`-style module
  layout once the repo takes shape; keep simulation core logic in pure Go,
  separable from CLI/API/UI layers.
- Next.js: App Router; keep the dashboard a thin consumer of the backend API
  rather than duplicating simulation logic.
- Update this section with the concrete module/package names and directory
  layout as code lands.

## Workflow rules

- Run `test` and `lint` commands after meaningful changes (see
  `.opencode/command/`).
- Match existing patterns and conventions; check neighboring files before
  writing new ones.
- No comments in code unless they explain non-obvious intent.
- Never run destructive commands without asking.
- Keep the simulator's core simulation logic separable from CLI/UI concerns.