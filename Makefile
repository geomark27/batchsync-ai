BINARY := batchsync
CMD     := ./cmd/batchsync

.PHONY: help build test test-integration vet tidy clean

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary (output: ./batchsync)
	go build -o $(BINARY) $(CMD)

test: ## Run all unit tests with race detector
	go test -race ./...

test-integration: ## Run all tests including Gemini API calls (requires GEMINI_API_KEY)
	go test -race -v ./...

test-pkg: ## Run tests for a single package  e.g: make test-pkg PKG=./internal/ai/...
	go test -race -v $(PKG)

vet: ## Run go vet linter
	go vet ./...

tidy: ## Clean up go.mod and go.sum
	go mod tidy

clean: ## Remove compiled binary
	rm -f $(BINARY)
