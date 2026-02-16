#!/bin/bash
# CyNodeAI Development Setup Script
# This script sets up and runs the end-to-end development environment.
# See docs/tech_specs/ for architecture details.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
POSTGRES_CONTAINER_NAME="cynodeai-postgres-dev"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
POSTGRES_DB="${POSTGRES_DB:-cynodeai}"

ORCHESTRATOR_PORT="${ORCHESTRATOR_PORT:-8080}"
NODE_PORT="${NODE_PORT:-8081}"
NODE_SLUG="${NODE_SLUG:-dev-node-01}"
NODE_REGISTRATION_PSK="${NODE_REGISTRATION_PSK:-dev-psk-secret}"
BOOTSTRAP_ADMIN_PASSWORD="${BOOTSTRAP_ADMIN_PASSWORD:-admin123}"
JWT_SECRET="${JWT_SECRET:-dev-jwt-secret-change-in-prod}"

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect container runtime (prefer podman, fallback to docker)
detect_runtime() {
    if command -v podman &> /dev/null; then
        echo "podman"
    elif command -v docker &> /dev/null; then
        echo "docker"
    else
        log_error "Neither podman nor docker found. Please install one."
        exit 1
    fi
}

RUNTIME=$(detect_runtime)
log_info "Using container runtime: $RUNTIME"

# Check if postgres container is already running
check_postgres() {
    if $RUNTIME ps --filter "name=$POSTGRES_CONTAINER_NAME" --format "{{.Names}}" | grep -q "$POSTGRES_CONTAINER_NAME"; then
        return 0
    fi
    return 1
}

# Start PostgreSQL container
start_postgres() {
    log_info "Starting PostgreSQL container..."
    
    # Check if container exists but is stopped
    if $RUNTIME ps -a --filter "name=$POSTGRES_CONTAINER_NAME" --format "{{.Names}}" | grep -q "$POSTGRES_CONTAINER_NAME"; then
        if check_postgres; then
            log_info "PostgreSQL container already running"
            return 0
        else
            log_info "Starting existing PostgreSQL container..."
            $RUNTIME start "$POSTGRES_CONTAINER_NAME"
        fi
    else
        log_info "Creating new PostgreSQL container..."
        $RUNTIME run -d \
            --name "$POSTGRES_CONTAINER_NAME" \
            -p "$POSTGRES_PORT:5432" \
            -e "POSTGRES_PASSWORD=$POSTGRES_PASSWORD" \
            -e "POSTGRES_DB=$POSTGRES_DB" \
            postgres:16
    fi
    
    # Wait for PostgreSQL to be ready
    log_info "Waiting for PostgreSQL to be ready..."
    for i in {1..30}; do
        if $RUNTIME exec "$POSTGRES_CONTAINER_NAME" pg_isready -U postgres &> /dev/null; then
            log_success "PostgreSQL is ready"
            return 0
        fi
        sleep 1
    done
    
    log_error "PostgreSQL failed to start within 30 seconds"
    exit 1
}

# Stop PostgreSQL container
stop_postgres() {
    log_info "Stopping PostgreSQL container..."
    if check_postgres; then
        $RUNTIME stop "$POSTGRES_CONTAINER_NAME"
        log_success "PostgreSQL stopped"
    else
        log_warn "PostgreSQL container not running"
    fi
}

# Remove PostgreSQL container
remove_postgres() {
    log_info "Removing PostgreSQL container..."
    if $RUNTIME ps -a --filter "name=$POSTGRES_CONTAINER_NAME" --format "{{.Names}}" | grep -q "$POSTGRES_CONTAINER_NAME"; then
        $RUNTIME rm -f "$POSTGRES_CONTAINER_NAME"
        log_success "PostgreSQL container removed"
    else
        log_warn "PostgreSQL container does not exist"
    fi
}

# Build the Go binaries
build() {
    log_info "Building Go binaries..."
    go build -o bin/orchestrator-api ./cmd/orchestrator-api
    go build -o bin/node ./cmd/node
    log_success "Build complete"
}

# Start the orchestrator API
start_orchestrator() {
    log_info "Starting Orchestrator API on port $ORCHESTRATOR_PORT..."
    
    export DATABASE_URL="postgres://postgres:$POSTGRES_PASSWORD@localhost:$POSTGRES_PORT/$POSTGRES_DB?sslmode=disable"
    export LISTEN_ADDR=":$ORCHESTRATOR_PORT"
    export JWT_SECRET="$JWT_SECRET"
    export NODE_REGISTRATION_PSK="$NODE_REGISTRATION_PSK"
    export BOOTSTRAP_ADMIN_PASSWORD="$BOOTSTRAP_ADMIN_PASSWORD"
    
    ./bin/orchestrator-api &
    ORCHESTRATOR_PID=$!
    echo $ORCHESTRATOR_PID > /tmp/cynodeai-orchestrator.pid
    
    # Wait for server to be ready
    log_info "Waiting for Orchestrator API to be ready..."
    for i in {1..15}; do
        if curl -s "http://localhost:$ORCHESTRATOR_PORT/healthz" > /dev/null 2>&1; then
            log_success "Orchestrator API is ready (PID: $ORCHESTRATOR_PID)"
            return 0
        fi
        sleep 1
    done
    
    log_error "Orchestrator API failed to start"
    exit 1
}

# Start the node
start_node() {
    log_info "Starting Node on port $NODE_PORT..."
    
    export ORCHESTRATOR_URL="http://localhost:$ORCHESTRATOR_PORT"
    export NODE_REGISTRATION_PSK="$NODE_REGISTRATION_PSK"
    export NODE_SLUG="$NODE_SLUG"
    export NODE_NAME="Development Node"
    export NODE_LISTEN_ADDR=":$NODE_PORT"
    export CONTAINER_RUNTIME="$RUNTIME"
    
    ./bin/node &
    NODE_PID=$!
    echo $NODE_PID > /tmp/cynodeai-node.pid
    
    # Wait for server to be ready
    log_info "Waiting for Node to be ready..."
    for i in {1..15}; do
        if curl -s "http://localhost:$NODE_PORT/healthz" > /dev/null 2>&1; then
            log_success "Node is ready (PID: $NODE_PID)"
            return 0
        fi
        sleep 1
    done
    
    log_error "Node failed to start"
    exit 1
}

# Stop all services
stop_services() {
    log_info "Stopping services..."
    
    if [ -f /tmp/cynodeai-node.pid ]; then
        kill $(cat /tmp/cynodeai-node.pid) 2>/dev/null || true
        rm -f /tmp/cynodeai-node.pid
        log_info "Node stopped"
    fi
    
    if [ -f /tmp/cynodeai-orchestrator.pid ]; then
        kill $(cat /tmp/cynodeai-orchestrator.pid) 2>/dev/null || true
        rm -f /tmp/cynodeai-orchestrator.pid
        log_info "Orchestrator stopped"
    fi
    
    log_success "Services stopped"
}

# Run end-to-end tests
test_e2e() {
    log_info "Running end-to-end tests..."
    
    API_URL="http://localhost:$ORCHESTRATOR_PORT"
    
    # Test 1: Health check
    log_info "Test 1: Health check..."
    HEALTH=$(curl -s "$API_URL/healthz")
    if [ "$HEALTH" = "ok" ]; then
        log_success "Health check passed"
    else
        log_error "Health check failed"
        exit 1
    fi
    
    # Test 2: Login as admin
    log_info "Test 2: Login as admin..."
    LOGIN_RESP=$(curl -s -X POST "$API_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"handle\": \"admin\", \"password\": \"$BOOTSTRAP_ADMIN_PASSWORD\"}")
    
    ACCESS_TOKEN=$(echo "$LOGIN_RESP" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    if [ -z "$ACCESS_TOKEN" ]; then
        log_error "Login failed: $LOGIN_RESP"
        exit 1
    fi
    log_success "Login successful, got access token"
    
    # Test 3: Get current user
    log_info "Test 3: Get current user..."
    USER_RESP=$(curl -s "$API_URL/v1/users/me" \
        -H "Authorization: Bearer $ACCESS_TOKEN")
    
    if echo "$USER_RESP" | grep -q '"handle":"admin"'; then
        log_success "Get user passed"
    else
        log_error "Get user failed: $USER_RESP"
        exit 1
    fi
    
    # Test 4: Create a task
    log_info "Test 4: Create a task..."
    TASK_RESP=$(curl -s -X POST "$API_URL/v1/tasks" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"prompt": "Run echo hello world"}')
    
    TASK_ID=$(echo "$TASK_RESP" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    if [ -z "$TASK_ID" ]; then
        log_error "Create task failed: $TASK_RESP"
        exit 1
    fi
    log_success "Task created with ID: $TASK_ID"
    
    # Test 5: Get task
    log_info "Test 5: Get task..."
    GET_TASK_RESP=$(curl -s "$API_URL/v1/tasks/$TASK_ID" \
        -H "Authorization: Bearer $ACCESS_TOKEN")
    
    if echo "$GET_TASK_RESP" | grep -q '"status":"pending"'; then
        log_success "Get task passed"
    else
        log_error "Get task failed: $GET_TASK_RESP"
        exit 1
    fi
    
    # Test 6: Get task result
    log_info "Test 6: Get task result..."
    RESULT_RESP=$(curl -s "$API_URL/v1/tasks/$TASK_ID/result" \
        -H "Authorization: Bearer $ACCESS_TOKEN")
    
    if echo "$RESULT_RESP" | grep -q '"task_id"'; then
        log_success "Get task result passed"
    else
        log_error "Get task result failed: $RESULT_RESP"
        exit 1
    fi
    
    # Test 7: Node health check
    log_info "Test 7: Node health check..."
    NODE_HEALTH=$(curl -s "http://localhost:$NODE_PORT/healthz")
    if [ "$NODE_HEALTH" = "ok" ]; then
        log_success "Node health check passed"
    else
        log_error "Node health check failed"
        exit 1
    fi
    
    # Test 8: Execute sandbox job on node
    log_info "Test 8: Execute sandbox job on node..."
    JOB_RESP=$(curl -s -X POST "http://localhost:$NODE_PORT/v1/worker/jobs:run" \
        -H "Content-Type: application/json" \
        -d '{
            "version": 1,
            "task_id": "test-task-123",
            "job_id": "test-job-456",
            "sandbox": {
                "image": "alpine:latest",
                "command": ["echo", "hello world"],
                "timeout_seconds": 30
            }
        }')
    
    if echo "$JOB_RESP" | grep -q '"status"'; then
        JOB_STATUS=$(echo "$JOB_RESP" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        log_success "Job executed with status: $JOB_STATUS"
    else
        log_warn "Job execution may have failed (container runtime may not be available): $JOB_RESP"
    fi
    
    log_success "All E2E tests completed!"
}

# Show usage
usage() {
    echo "CyNodeAI Development Setup Script"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  start       Start all services (postgres, orchestrator, node)"
    echo "  stop        Stop all services"
    echo "  restart     Restart all services"
    echo "  postgres    Start only PostgreSQL"
    echo "  build       Build Go binaries"
    echo "  test        Run end-to-end tests"
    echo "  clean       Stop services and remove postgres container"
    echo ""
    echo "Environment Variables:"
    echo "  POSTGRES_PORT               PostgreSQL port (default: 5432)"
    echo "  ORCHESTRATOR_PORT           Orchestrator API port (default: 8080)"
    echo "  NODE_PORT                   Node Worker API port (default: 8081)"
    echo "  NODE_SLUG                   Node identifier (default: dev-node-01)"
    echo "  NODE_REGISTRATION_PSK       PSK for node registration"
    echo "  BOOTSTRAP_ADMIN_PASSWORD    Admin user password (default: admin123)"
}

# Main
case "${1:-}" in
    start)
        start_postgres
        build
        start_orchestrator
        sleep 2
        start_node
        echo ""
        log_success "All services started!"
        echo ""
        echo "Orchestrator API: http://localhost:$ORCHESTRATOR_PORT"
        echo "Node Worker API:  http://localhost:$NODE_PORT"
        echo "Admin login:      handle=admin password=$BOOTSTRAP_ADMIN_PASSWORD"
        echo ""
        echo "Run '$0 test' to verify the setup"
        echo "Run '$0 stop' to stop all services"
        ;;
    stop)
        stop_services
        ;;
    restart)
        stop_services
        sleep 1
        start_postgres
        build
        start_orchestrator
        sleep 2
        start_node
        log_success "Services restarted"
        ;;
    postgres)
        start_postgres
        ;;
    build)
        build
        ;;
    test)
        test_e2e
        ;;
    clean)
        stop_services
        stop_postgres
        remove_postgres
        log_success "Cleanup complete"
        ;;
    *)
        usage
        exit 1
        ;;
esac
