.PHONY: help build build-dashboard run-dashboard clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go binary (without dashboard build)
	go build -o sblite .

build-dashboard: ## Build the React dashboard and embed it in the Go binary
	cd dashboard && npm run build
	cp -r dashboard/dist internal/dashboard/assets/
	go build -o sblite .

run-dashboard: ## Run the Go backend with dashboard for development
	go run . serve

dev: ## Run both dashboard dev server and Go backend (use two terminals)
	@echo "Run these commands in two separate terminals:"
	@echo "  Terminal 1: make run-dashboard"
	@echo "  Terminal 2: cd dashboard && npm run dev"

clean: ## Clean build artifacts
	rm -f sblite
	rm -rf dashboard/dist
	rm -rf internal/dashboard/assets/dist

test: ## Run tests
	go test ./...

e2e: ## Run e2e tests (requires server to be running)
	cd e2e && npm test
