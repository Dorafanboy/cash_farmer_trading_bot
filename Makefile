# Go parameters
GOBASE := $(shell pwd)
GOBUILD := go build
GOTEST := go test
GOCLEAN := go clean
GOMOD := go mod
LINT := golangci-lint run

# Binary name
BINARY_NAME := bot
CMD_PATH := ./cmd/bot

# Database commands
DB_HOST := localhost
DB_PORT := 5432
DB_USER := postgres
DB_PASSWORD := password
DB_NAME := cash_farmer

.PHONY: all build clean test lint run docker-build infra-run infra-down infra-only-up infra-only-down migrate-up migrate-reset db-shell help

all: build

build: ## Build the application binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) $(CMD_PATH)/main.go

clean: ## Remove previous build
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) ./...

lint: ## Run linters
	@echo "Running linters..."
	$(LINT) ./...

run: build ## Build and run the application (foreground)
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Docker commands
docker-build: ## Build the Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run the application in a Docker container
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 --name $(BINARY_NAME)-instance $(BINARY_NAME):latest

infra-up: ## Start the full application using docker-compose (includes bot build)
	@echo "Starting full docker-compose service..."
	docker-compose up -d --build

infra-down: ## Stop the full application using docker-compose
	@echo "Stopping full docker-compose service..."
	docker-compose down

infra-only-up: ## Start ONLY infrastructure (postgres + redis) - FAST, no bot build
	@echo "Starting infrastructure only (postgres + redis)..."
	docker-compose -f docker-compose.infra.yml up -d

infra-only-down: ## Stop ONLY infrastructure (postgres + redis)
	@echo "Stopping infrastructure only..."
	docker-compose -f docker-compose.infra.yml down

# Database commands
migrate-up: ## Run database migrations using migration files
	@echo "Running database migrations..."
	@for migration in db/migrations/*_*.up.sql; do \
		echo "Applying $$migration..."; \
		PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f "$$migration"; \
	done

migrate-reset: ## Reset database (WARNING: destroys all data)
	@echo "Resetting database..."
	PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	@$(MAKE) migrate-up

db-shell: ## Connect to PostgreSQL shell
	@echo "Connecting to database..."
	PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME)

help: ## Display this help screen
	@echo 'Usage: make <TARGETS>'
	@echo '\nAvailable targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Default target
.DEFAULT_GOAL := help 