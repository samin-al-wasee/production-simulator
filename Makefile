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
lint-go: ## Go: gofmt -l and go vet ./... (from core/)
	cd core && gofmt -l . && go vet ./...

.PHONY: test-go
test-go: ## Go: go test ./... (from core/)
	cd core && go test ./...

.PHONY: build-core
build-core: ## Build the forgelab CLI into bin/
	cd core && go build -o ../bin/forgelab ./cmd/forgelab

.PHONY: validate
validate: build-core ## Validate a manifest: make validate FILE=manifests/hello.example.yaml
	./bin/forgelab validate $(FILE)

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