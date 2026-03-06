BINARY := batchsync
CMD     := ./cmd/batchsync

# ─── Default target ────────────────────────────────────────────────────────────
.DEFAULT_GOAL := help

.PHONY: help build test test-unit test-integration test-pkg test-race vet tidy clean

# ─── Help ──────────────────────────────────────────────────────────────────────
help: ## Show available commands
	@echo ""
	@echo "  BatchSync AI — Available Commands"
	@echo ""
	@echo "  BUILD"
	@grep -E '^build.*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  TEST"
	@grep -E '^test.*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  QUALITY"
	@grep -E '^(vet|tidy).*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "  CLEANUP"
	@grep -E '^clean.*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "    \033[36m%-22s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ─── Build ─────────────────────────────────────────────────────────────────────
build: ## Compile the binary (output: ./batchsync)
	go build -o $(BINARY) $(CMD)

# ─── Tests ─────────────────────────────────────────────────────────────────────

# Runs all tests with the race detector. This is the default test command and
# should always pass before merging any code. Integration tests are skipped
# automatically when their required env vars (GEMINI_API_KEY, SQLSERVER_CONN_STRING)
# are not set.
test: ## Run all tests with race detector (integration tests auto-skipped if env vars missing)
	go test -race ./...

# Runs only unit tests — no external dependencies required (no DB, no Gemini API).
# Useful for fast feedback loops during development.
test-unit: ## Run unit tests only (no external dependencies required)
	go test -race -run 'Test[^_]+$$' ./...

# Runs all tests including integration tests that call Gemini and SQL Server.
# Requires GEMINI_API_KEY and SQLSERVER_CONN_STRING to be set in the environment.
# Verbose output (-v) shows each test name and result.
test-integration: ## Run all tests including Gemini and SQL Server calls (requires env vars)
	@if [ -z "$(GEMINI_API_KEY)" ]; then \
		echo "\033[33mWARN: GEMINI_API_KEY is not set — Gemini integration tests will be skipped\033[0m"; \
	fi
	@if [ -z "$(SQLSERVER_CONN_STRING)" ]; then \
		echo "\033[33mWARN: SQLSERVER_CONN_STRING is not set — database integration tests will be skipped\033[0m"; \
	fi
	go test -race -v ./...

# Runs tests for a single package with verbose output.
# Usage: make test-pkg PKG=./internal/processor/...
test-pkg: ## Run tests for a specific package   e.g: make test-pkg PKG=./internal/processor/...
	@if [ -z "$(PKG)" ]; then \
		echo "\033[31mERROR: PKG is required. Usage: make test-pkg PKG=./internal/processor/...\033[0m"; \
		exit 1; \
	fi
	go test -race -v $(PKG)

# Runs only tests that exercise concurrent code paths — those that have "Race",
# "Goroutine", or "Worker" in their name. Useful to quickly verify concurrency
# behavior without running the full suite.
test-race: ## Run only concurrency-related tests with race detector
	go test -race -v -run 'TestProcesar|TestRace|TestWorker' ./internal/processor/...

# ─── Quality ───────────────────────────────────────────────────────────────────
vet: ## Run go vet static analysis across all packages
	go vet ./...

tidy: ## Clean up go.mod and go.sum (run after any go get or dependency removal)
	go mod tidy

# ─── Cleanup ───────────────────────────────────────────────────────────────────
clean: ## Remove compiled binary and test cache
	go clean -testcache
	rm -f $(BINARY)