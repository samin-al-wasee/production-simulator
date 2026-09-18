---
description: Discover and run the project's linters and typecheckers.
agent: build
---

Run the linters/typecheckers for this repository.

Stack: Go backend + Next.js dashboard (+ optional Python later).

1. Backend (Go module): run `gofmt -l` over the module, `go vet ./...`, and
   `staticcheck ./...` if installed (install via `go install` if missing).
2. Frontend (Next.js): run the app's `lint` and `typecheck` scripts
   (`eslint` + `tsc --noEmit`). Use `$ARGUMENTS` to scope to specific
   files/directories when given.
3. Report every issue with file and line references.

Do not fix issues unless asked. If a toolchain piece is absent, propose the
obvious minimal choice instead of inventing one.