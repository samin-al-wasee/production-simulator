# ForgeLab — development helper targets.

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: serve
serve: ## Serve the core API for the dashboard on 127.0.0.1:8090
	cd core && go run ./cmd/forgelab serve -repo $(CURDIR)

.PHONY: dashboard-dev
dashboard-dev: ## Run the dashboard on http://localhost:3001 (needs `make serve`; runs npm install first)
	cd dashboard && npm install && npm run dev

.PHONY: lint-go
lint-go: ## Go: gofmt -l and go vet ./... (from core/)
	cd core && test -z "$$(gofmt -l .)" && go vet ./...

.PHONY: test-go
test-go: ## Go: go test ./... (from core/)
	cd core && go test ./...

.PHONY: build-core
build-core: ## Build the forgelab CLI into bin/
	cd core && go build -o ../bin/forgelab ./cmd/forgelab

.PHONY: lint-fe
lint-fe: ## Dashboard: lint + typecheck
	cd dashboard && npm run lint && npm run typecheck

.PHONY: test-fe
test-fe: ## Dashboard: unit tests
	cd dashboard && npm test

.PHONY: test-e2e
test-e2e: ## Dashboard browser tests (Playwright; starts or reuses the API and dev server)
	cd dashboard && npx playwright install chromium && npm run e2e

.PHONY: lint
lint: lint-go lint-fe ## Run every linter and typechecker

.PHONY: test
test: test-go test-fe ## Run every unit test suite

.PHONY: scan-secrets
scan-secrets: ## Scan the repository for committed credentials
	cd core && go run ./cmd/forgelab security scan-secrets ..

.PHONY: learn
learn: ## Show progress through the learning path
	cd core && go run ./cmd/forgelab learn status

.PHONY: check
check: lint test scan-secrets ## Full validation gate
