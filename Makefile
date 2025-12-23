.PHONY: all build test run clean backend-build backend-test backend-run frontend-build frontend-test frontend-run

# Default target
all: build

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
