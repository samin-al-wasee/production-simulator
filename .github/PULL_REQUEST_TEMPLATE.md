## Summary

What does this change do and why? One or two sentences.

## Type of change

- [ ] docs (documentation only)
- [ ] feat (new capability)
- [ ] fix (defect)
- [ ] refactor
- [ ] test
- [ ] chore
- [ ] infra

## Documentation-first

Per `AGENTS.md` §5, docs ship with code. List every doc touched:

- [ ] `docs/architecture.md`
- [ ] `docs/component-catalog.md`
- [ ] `docs/scenarios.md`
- [ ] `docs/decisions/` (ADR — required if this touches architecture/boundaries)
- [ ] `ROADMAP.md` (if a phase moved)
- [ ] `CHANGELOG.md` (if user-relevant)
- Other: ___

## Verification

- [ ] Go: `go test ./...`, `gofmt -l`, `go vet ./...` (when a Go module exists)
- [ ] Next.js: `lint` + `typecheck` (when the dashboard app exists)
- [ ] Manual verification steps recorded

## Related issues

Closes #...