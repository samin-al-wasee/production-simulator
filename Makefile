# ForgeLab — development helper targets.
# These become meaningful as code lands; for now they document the intended
# gate each toolchain must pass. Windows users: run `pwsh` she-lines are
# intentionally avoided; this makefile uses portable targets.

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: docs
docs: ## Validate that the documentation index links resolve (once a checker exists)

.PHONY: lint-go
lint-go: ## Go: gofmt -l and go vet ./...
	@if [ -f go.mod ]; then \
		gofmt -l . ; \
		go vet ./... ; \
	else \
		echo "No Go module yet — skipping." ; \
	fi

.PHONY: test-go
test-go: ## Go: go test ./...
	@if [ -f go.mod ]; then \
		go test ./... ; \
	else \
		echo "No Go module yet — skipping." ; \
	fi

.PHONY: lint-fe
lint-fe: ## Next.js: lint + typecheck
	@if [ -d dashboard ]; then \
		cd dashboard && npm run lint && npm run typecheck ; \
	else \
		echo "No dashboard app yet — skipping." ; \
	fi

.PHONY: test-fe
test-fe: ## Next.js: test script
	@if [ -d dashboard ]; then \
		cd dashboard && npm test ; \
	else \
		echo "No dashboard app yet — skipping." ; \
	fi

.PHONY: lint
lint: lint-go lint-fe ## Run every available linter/typechecker

.PHONY: test
test: test-go test-fe ## Run every available test suite

.PHONY: check
check: lint test ## Full validation gate (docs + lint + test)