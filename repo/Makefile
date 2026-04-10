.PHONY: help build test lint migrate seed run docker-build docker-up docker-down docker-logs clean

BINARY_API=propertyops-api
BINARY_MIGRATE=propertyops-migrate
BINARY_HEALTH=propertyops-healthcheck
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build all binaries
	go build $(LDFLAGS) -o bin/$(BINARY_API) ./cmd/api
	go build $(LDFLAGS) -o bin/$(BINARY_MIGRATE) ./cmd/migrate
	go build $(LDFLAGS) -o bin/$(BINARY_HEALTH) ./cmd/healthcheck

test: ## Run all tests (unit + integration; requires CGO_ENABLED=1 and a C compiler for integration tests)
	go test -v -race -count=1 ./...

test-unit: ## Run unit tests only (no CGO required)
	go test -v -race -count=1 $(shell go list ./... | grep -v propertyops/backend/test)

test-integration: ## Run integration tests with SQLite (requires CGO_ENABLED=1 and GCC/MinGW)
	CGO_ENABLED=1 go test -v -race -count=1 -timeout=300s ./test/...

test-mysql: ## Run integration tests against real MySQL 8.0 (set TEST_MYSQL_DSN first)
	@if [ -z "$(TEST_MYSQL_DSN)" ]; then \
		echo "ERROR: TEST_MYSQL_DSN is not set. Example:"; \
		echo "  TEST_MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/propertyops_test?parseTime=true' make test-mysql"; \
		exit 1; \
	fi
	TEST_MYSQL_DSN=$(TEST_MYSQL_DSN) go test -v -race -count=1 -timeout=300s ./test/... ./internal/...

test-coverage: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run linter
	golangci-lint run ./...

migrate: ## Run database migrations
	go run ./cmd/migrate

seed: ## Seed the database
	go run ./cmd/migrate --seed

run: ## Run the API server locally
	go run ./cmd/api

docker-build: ## Build Docker images
	docker compose -f deploy/docker-compose.yml build

docker-up: ## Start all services
	docker compose -f deploy/docker-compose.yml up -d

docker-down: ## Stop all services
	docker compose -f deploy/docker-compose.yml down

docker-logs: ## Tail API logs
	docker compose -f deploy/docker-compose.yml logs -f api

docker-migrate: ## Run migrations in Docker
	docker compose -f deploy/docker-compose.yml run --rm migrate

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html
