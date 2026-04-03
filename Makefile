# ─── Variables ────────────────────────────────────────────────────────────────

APP_NAME     := remnacore
BIN_DIR      := bin
CMD_DIR      := cmd/$(APP_NAME)
COVERAGE_OUT := coverage.out

# ─── Build & Run ──────────────────────────────────────────────────────────────

.PHONY: build
build: ## Compile the application binary
	go build -o $(BIN_DIR)/$(APP_NAME) ./$(CMD_DIR)

.PHONY: run
run: build ## Build and run the application
	./$(BIN_DIR)/$(APP_NAME)

.PHONY: dev
dev: ## Run with live-reload (requires air: go install github.com/air-verse/air@latest)
	air

# ─── Testing ──────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run unit tests
	go test -race -count=1 ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	go test -race -count=1 -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -func=$(COVERAGE_OUT)

.PHONY: test-integration
test-integration: ## Run integration tests (requires running infrastructure)
	go test -race -count=1 -tags=integration ./...

# ─── Code Quality ─────────────────────────────────────────────────────────────

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

# ─── Docker ───────────────────────────────────────────────────────────────────

.PHONY: up
up: ## Start all containers in detached mode
	docker compose up -d

.PHONY: down
down: ## Stop and remove all containers
	docker compose down

.PHONY: logs
logs: ## Tail container logs
	docker compose logs -f

# ─── Code Generation ─────────────────────────────────────────────────────────

.PHONY: gen
gen: ## Generate sqlc Go code from SQL queries
	sqlc generate

# ─── Database Migrations ─────────────────────────────────────────────────────

.PHONY: migrate
migrate: ## Apply all pending migrations
	atlas migrate apply --env local

.PHONY: migrate-new
migrate-new: ## Create a new migration (usage: make migrate-new name=<migration_name>)
	atlas migrate diff $(name) --env local

# ─── Housekeeping ─────────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts and coverage files
	rm -rf $(BIN_DIR) $(COVERAGE_OUT)

# ─── Help ─────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
