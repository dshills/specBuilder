#!/bin/sh
set -e

echo "=== SpecBuilder Container Starting ==="

# Ensure nginx log files exist
touch /app/nginx/logs/error.log /app/nginx/logs/access.log

# Start the backend in the background
echo "Starting backend on port ${SPECBUILDER_API_PORT:-8081}..."
./specbuilder &
BACKEND_PID=$!

# Wait for backend to be ready
echo "Waiting for backend to initialize..."
for i in $(seq 1 30); do
    if wget -q --spider "http://127.0.0.1:${SPECBUILDER_API_PORT:-8081}/health" 2>/dev/null; then
        echo "Backend is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "Error: Backend failed to start within 30 seconds"
        exit 1
    fi
    sleep 1
done

# Start nginx in the foreground with our config
echo "Starting nginx on port 3080..."
echo "=== SpecBuilder is ready at http://localhost:3080 ==="
exec nginx -c /app/nginx/nginx.conf -e /app/nginx/logs/error.log -g 'daemon off;'
