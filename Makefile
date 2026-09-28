# ForgeLab — development helper targets.
# These become meaningful as code lands; for now they document the intended
# gate each toolchain must pass. Windows users: run `pwsh` she-lines are
# intentionally avoided; this makefile uses portable targets.

LOCAL_COMPOSE := environments/local/compose.yaml
OBS_COMPOSE := environments/local/compose.observability.yaml
MSG_COMPOSE := environments/local/compose.messaging.yaml
ALL_COMPOSE := -f $(LOCAL_COMPOSE) -f $(OBS_COMPOSE) -f $(MSG_COMPOSE)

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

.PHONY: up
up: ## Start the local lab stack (reverse proxy + app + database)
	docker compose -f $(LOCAL_COMPOSE) up -d --build

.PHONY: up-obs
up-obs: ## Start the local stack plus the observability overlay (Prometheus, Grafana, Loki, Tempo)
	docker compose -f $(LOCAL_COMPOSE) -f $(OBS_COMPOSE) up -d --build

.PHONY: up-msg
up-msg: ## Start the local stack plus the messaging overlay (Redis, RabbitMQ, Kafka)
	docker compose -f $(LOCAL_COMPOSE) -f $(MSG_COMPOSE) up -d --build

.PHONY: up-all
up-all: ## Start the local stack with observability and messaging overlays
	docker compose $(ALL_COMPOSE) up -d --build

.PHONY: smoke-msg
smoke-msg: ## Verify Redis, RabbitMQ (retry + DLQ), and Kafka against a running messaging stack
	sh scripts/messaging-smoke.sh

.PHONY: k8s-up
k8s-up: ## Create the local kind cluster and deploy the Kubernetes base (needs kind + kubectl)
	sh scripts/k8s-up.sh

.PHONY: k8s-down
k8s-down: ## Delete the local kind cluster
	sh scripts/k8s-down.sh

.PHONY: chaos
chaos: ## Run a chaos experiment: make chaos FILE=scenarios/failures/db-outage/experiment.yaml (add DRY=1 to only print the plan)
	cd core && go run ./cmd/forgelab chaos $(if $(DRY),plan,run) $(abspath $(FILE))

.PHONY: loadtest
loadtest: ## Generate load: make loadtest URL=http://localhost:8080/api/work RPS=100 DURATION=30s [PROFILE=ramp RAMP_TO=300]
	cd core && go run ./cmd/forgelab loadtest -url $(URL) -rps $(or $(RPS),50) -duration $(or $(DURATION),10s) -profile $(or $(PROFILE),constant) -ramp-to $(or $(RAMP_TO),0) -scale $(or $(SCALE),1)

.PHONY: backup
backup: ## Back up the local PostgreSQL database to backups/
	sh scripts/db-backup.sh

.PHONY: cloud-validate
cloud-validate: ## Validate the Terraform cloud presets (no credentials, creates nothing; needs terraform)
	sh scripts/cloud-validate.sh

.PHONY: cloud-plan
cloud-plan: ## Plan a cloud preset and run the cost guard: make cloud-plan PRESET=aws (never applies)
	sh scripts/cloud-plan.sh $(PRESET)

.PHONY: serve
serve: ## Serve the core API for the dashboard on 127.0.0.1:8090 (add RUNS=1 to allow starting experiments)
	cd core && go run ./cmd/forgelab serve -repo $(CURDIR) $(if $(RUNS),-enable-runs)

.PHONY: dashboard-dev
dashboard-dev: ## Run the Next.js dashboard on http://localhost:3001 (needs `make serve`; runs npm install first)
	cd dashboard && npm install && npm run dev

.PHONY: down
down: ## Stop and remove the local lab stack
	docker compose $(ALL_COMPOSE) down

.PHONY: ps
ps: ## Show the local lab stack status
	docker compose $(ALL_COMPOSE) ps

.PHONY: logs
logs: ## Tail local lab stack logs
	docker compose $(ALL_COMPOSE) logs -f

.PHONY: check
check: lint test ## Full validation gate (docs + lint + test)