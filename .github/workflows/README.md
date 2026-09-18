# Workflows

CI/CD automation for the repository itself.

**Status:** planned. No workflows yet — no runnable code exists to validate. Workflows will be added per roadmap phase (see `ROADMAP.md`), starting with docs/validation checks and the lint+test gate defined in `AGENTS.md` §11 and `.opencode/command/`.

## Planned checks

- Documentation link validation (docs map remains consistent)
- Manifest schema validation (`manifests/application.schema.yaml`)
- Go: `gofmt -l`, `go vet ./...`, `go test ./...` (once a Go module exists)
- Next.js: `lint`, `typecheck`, tests (once the dashboard app exists)