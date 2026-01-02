.PHONY: all build test run clean backend-build backend-test backend-run frontend-build frontend-test frontend-run help

# Default target
all: build

# Show available targets
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all             Build backend and frontend (default)"
	@echo "  build           Build backend and frontend"
	@echo "  test            Run all tests"
	@echo "  clean           Remove build artifacts"
	@echo "  dev             Show instructions for development"
	@echo "  fmt             Format Go code"
	@echo "  lint            Run golangci-lint on backend"
	@echo "  generate        Run go generate"
	@echo ""
	@echo "Backend:"
	@echo "  backend-build   Build the Go server"
	@echo "  backend-test    Run backend tests"
	@echo "  backend-run     Run backend server"
	@echo ""
	@echo "Frontend:"
	@echo "  frontend-build  Build React frontend"
	@echo "  frontend-test   Run frontend tests"
	@echo "  frontend-run    Start frontend dev server"

# Build both backend and frontend
build: backend-build frontend-build

# Test both backend and frontend
test: backend-test frontend-test

# Clean build artifacts
clean:
	rm -rf backend/bin/
	rm -rf frontend/build/ frontend/dist/
	rm -rf exports/

# Backend targets
backend-build:
	cd backend && go build -o bin/specbuilder ./cmd/server

backend-test:
	cd backend && go test -v ./...

backend-run:
	cd backend && go run ./cmd/server

# Frontend targets
frontend-build:
	cd frontend && npm run build

frontend-test:
	cd frontend && npm test

frontend-run:
	cd frontend && npm start

# Development: run both services
dev:
	@echo "Start backend and frontend in separate terminals:"
	@echo "  make backend-run"
	@echo "  make frontend-run"

# Format code
fmt:
	cd backend && go fmt ./...

# Lint backend
lint:
	cd backend && golangci-lint run ./...

# Generate (placeholder for future code generation)
generate:
	cd backend && go generate ./...
