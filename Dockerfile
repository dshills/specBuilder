# Multi-stage Dockerfile for SpecBuilder
# Builds Go backend and React frontend, serves both from a single container

# =============================================================================
# Stage 1: Build Go backend
# =============================================================================
FROM golang:1.24-alpine AS backend-builder

# Install build dependencies for CGO (required by sqlite3)
RUN apk add --no-cache gcc musl-dev

WORKDIR /build

# Copy go mod files first for better caching
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy source and build
COPY backend/ ./
RUN CGO_ENABLED=1 go build -o specbuilder ./cmd/server

# =============================================================================
# Stage 2: Build React frontend
# =============================================================================
FROM node:20-alpine AS frontend-builder

WORKDIR /build

# Copy package files first for better caching
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Copy source and build
# Set empty API URL so frontend uses relative paths (proxied by nginx)
COPY frontend/ ./
ENV VITE_API_URL=""
RUN npm run build

# =============================================================================
# Stage 3: Production runtime
# =============================================================================
FROM alpine:3.19

# Install nginx and create necessary directories
RUN apk add --no-cache nginx ca-certificates tzdata

# Create non-root user first
RUN adduser -D -u 1000 specbuilder

WORKDIR /app

# Create all directories with proper ownership
RUN mkdir -p /app/data /app/nginx/logs /app/nginx/tmp /app/nginx/html \
    && chown -R specbuilder:specbuilder /app

# Copy backend binary
COPY --from=backend-builder /build/specbuilder ./

# Copy frontend build
COPY --from=frontend-builder /build/dist /app/nginx/html

# Copy nginx configuration
COPY docker/nginx.conf /app/nginx/nginx.conf

# Copy entrypoint script
COPY docker/entrypoint.sh ./

# Set ownership and permissions
RUN chown -R specbuilder:specbuilder /app \
    && chmod +x entrypoint.sh

USER specbuilder

# Expose single port (nginx handles routing)
EXPOSE 3080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:3080/health || exit 1

# Volume for persistent data
VOLUME ["/app/data"]

# Environment variables (API keys should be provided at runtime)
ENV SPECBUILDER_DB_PATH=/app/data/specbuilder.db \
    SPECBUILDER_API_PORT=8081

ENTRYPOINT ["./entrypoint.sh"]
