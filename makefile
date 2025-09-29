FRONTEND_DIR = ./web
BACKEND_DIR = .

.PHONY: all build-frontend start-backend lint test

all: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

# Code quality and testing targets
lint:
	@echo "Running Go linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --config .golangci.yml; \
	else \
		echo "golangci-lint not found, running basic checks..."; \
		go vet ./...; \
		go fmt ./...; \
	fi

test:
	@echo "Running tests..."
	@go test -v ./... -race -coverprofile=coverage.out

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v ./... -race -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

build:
	@echo "Building application..."
	@go build -ldflags "-s -w -X 'one-api/common.Version=$(shell git describe --tags)'" -o new-api

clean:
	@echo "Cleaning build artifacts..."
	@rm -f new-api coverage.out coverage.html
	@rm -rf $(FRONTEND_DIR)/dist
