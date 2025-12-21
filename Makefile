.PHONY: build run dev test clean sqlc migrate help

# Variables
BINARY_NAME=chatbot-api
BUILD_DIR=./bin

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet

# Build the application
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/api

# Run the application
run: build
	@echo "Running..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode with hot reload (requires air)
dev:
	@echo "Starting development server..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not installed. Run: go install github.com/air-verse/air@latest"; \
		$(GORUN) ./cmd/api; \
	fi

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Generate SQLC code
sqlc:
	@echo "Generating SQLC code..."
	@if command -v sqlc > /dev/null; then \
		sqlc generate; \
	else \
		echo "sqlc not installed. Run: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest"; \
		exit 1; \
	fi

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Create database
db-create:
	@echo "Creating database..."
	createdb -U postgres chatbot || true

# Run migrations
db-migrate:
	@echo "Running migrations..."
	psql -U postgres -d chatbot -f migrations/001_initial.sql

# Reset database
db-reset: db-drop db-create db-migrate

# Drop database
db-drop:
	@echo "Dropping database..."
	dropdb -U postgres chatbot --if-exists

# Install development tools
tools:
	@echo "Installing development tools..."
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Show help
help:
	@echo "Available commands:"
	@echo "  make build         - Build the application"
	@echo "  make run           - Build and run the application"
	@echo "  make dev           - Run with hot reload (requires air)"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make sqlc          - Generate SQLC code"
	@echo "  make tidy          - Tidy go modules"
	@echo "  make deps          - Download dependencies"
	@echo "  make vet           - Run go vet"
	@echo "  make lint          - Run linter"
	@echo "  make db-create     - Create database"
	@echo "  make db-migrate    - Run migrations"
	@echo "  make db-reset      - Reset database"
	@echo "  make db-drop       - Drop database"
	@echo "  make tools         - Install development tools"
