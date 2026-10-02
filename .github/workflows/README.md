# Workflows

CI for the repository itself. Each workflow runs on every push to `main` and can be started by hand from the Actions tab (**Run workflow**).

| Workflow | Checks |
|---|---|
| `backend.yml` | `gofmt -l`, `go vet ./...`, `go test ./...`, secret scan (from `backend/`) |
| `frontend.yml` | `npm ci`, `lint`, `typecheck`, `test`, `build` (from `frontend/`) |

Playwright e2e (`make test-e2e`) is not run in CI; it needs the API and dev server and stays a local check.
