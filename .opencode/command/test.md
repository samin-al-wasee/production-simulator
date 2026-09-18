---
description: Discover and run the project's test suite.
agent: build
---

Run the test suite for this repository.

Stack: Go backend + Next.js dashboard (+ optional Python later).

1. Detect which parts exist: a Go module (`go.mod`) for the backend, and/or a
   Next.js app with a test script in its `package.json`.
2. Backend: run `go test ./...` from the Go module root, or
   `go test ./... -run $ARGUMENTS` if specific tests/packages are named.
   Frontend: run the app's test script (`npm test`/`pnpm test`/etc.).
3. If a test runner is configured but dependencies are missing, install what's
   needed to run the tests.
4. Report pass/fail clearly; on failure show the output and diagnose the root
   cause. Do not fix failures unless asked.