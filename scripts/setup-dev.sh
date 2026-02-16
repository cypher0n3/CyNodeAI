#!/bin/bash
# CyNodeAI Development Setup Script
# This script sets up the local development environment for running the E2E demo.
# Requirements: podman or docker, Go 1.25+, PostgreSQL client (psql)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect container runtime (prefer podman)
detect_runtime() {
    if command -v podman &> /dev/null; then
        echo "podman"
    elif command -v docker &> /dev/null; then
        echo "docker"
    else
        log_error "Neither podman nor docker found. Please install one of them."
        exit 1
    fi
}

RUNTIME=$(detect_runtime)
log_info "Using container runtime: $RUNTIME"

# Configuration
POSTGRES_CONTAINER_NAME="cynodeai-postgres-dev"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_USER="${POSTGRES_USER:-cynodeai}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-cynodeai-dev-password}"
POSTGRES_DB="${POSTGRES_DB:-cynodeai}"

# Orchestrator API config
ORCHESTRATOR_PORT="${ORCHESTRATOR_PORT:-8080}"
JWT_SECRET="${JWT_SECRET:-dev-jwt-secret-change-in-production}"
NODE_PSK="${NODE_PSK:-dev-node-psk-secret}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"

# Node config  
NODE_SLUG="${NODE_SLUG:-dev-node-1}"

export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

# Function to start PostgreSQL
start_postgres() {
    log_info "Starting PostgreSQL container..."
    
    # Check if container already exists
    if $RUNTIME ps -a --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER_NAME}$"; then
        if $RUNTIME ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER_NAME}$"; then
            log_info "PostgreSQL container already running"
            return 0
        else
            log_info "Starting existing PostgreSQL container"
            $RUNTIME start $POSTGRES_CONTAINER_NAME
            sleep 2
            return 0
        fi
    fi

    # Create and start new container
    $RUNTIME run -d \
        --name $POSTGRES_CONTAINER_NAME \
        -e POSTGRES_USER=$POSTGRES_USER \
        -e POSTGRES_PASSWORD=$POSTGRES_PASSWORD \
        -e POSTGRES_DB=$POSTGRES_DB \
        -p $POSTGRES_PORT:5432 \
        -v cynodeai-postgres-data:/var/lib/postgresql/data \
        postgres:16-alpine

    log_info "Waiting for PostgreSQL to be ready..."
    sleep 3
    
    # Wait for PostgreSQL to be ready
    for i in {1..30}; do
        if $RUNTIME exec $POSTGRES_CONTAINER_NAME pg_isready -U $POSTGRES_USER -d $POSTGRES_DB > /dev/null 2>&1; then
            log_info "PostgreSQL is ready"
            return 0
        fi
        sleep 1
    done
    
    log_error "PostgreSQL failed to start within 30 seconds"
    exit 1
}

# Function to stop PostgreSQL
stop_postgres() {
    log_info "Stopping PostgreSQL container..."
    $RUNTIME stop $POSTGRES_CONTAINER_NAME 2>/dev/null || true
}

# Function to clean up PostgreSQL (stop and remove)
clean_postgres() {
    log_info "Cleaning up PostgreSQL container and volume..."
    $RUNTIME stop $POSTGRES_CONTAINER_NAME 2>/dev/null || true
    $RUNTIME rm $POSTGRES_CONTAINER_NAME 2>/dev/null || true
    $RUNTIME volume rm cynodeai-postgres-data 2>/dev/null || true
}

# Function to run migrations (migrations run automatically on server startup)
run_migrations() {
    log_info "Migrations will run automatically when orchestrator-api starts..."
}

# Function to build binaries
build_binaries() {
    log_info "Building binaries..."
    cd "$PROJECT_ROOT"
    
    go build -o bin/orchestrator-api ./cmd/orchestrator-api
    go build -o bin/node ./cmd/node
    
    log_info "Binaries built: bin/orchestrator-api, bin/node"
}

# Function to start orchestrator-api
start_orchestrator() {
    log_info "Starting orchestrator-api on port $ORCHESTRATOR_PORT..."
    cd "$PROJECT_ROOT"
    
    export SERVER_PORT=$ORCHESTRATOR_PORT
    export JWT_SECRET_KEY=$JWT_SECRET
    export NODE_REGISTRATION_PSK=$NODE_PSK
    export ADMIN_HANDLE="${ADMIN_HANDLE:-admin}"
    export ADMIN_PASSWORD=$ADMIN_PASSWORD
    
    ./bin/orchestrator-api &
    ORCHESTRATOR_PID=$!
    echo $ORCHESTRATOR_PID > /tmp/orchestrator-api.pid
    
    sleep 2
    if kill -0 $ORCHESTRATOR_PID 2>/dev/null; then
        log_info "Orchestrator API started (PID: $ORCHESTRATOR_PID)"
    else
        log_error "Failed to start orchestrator-api"
        exit 1
    fi
}

# Function to start node
start_node() {
    log_info "Starting node..."
    cd "$PROJECT_ROOT"
    
    export ORCHESTRATOR_URL="http://localhost:$ORCHESTRATOR_PORT"
    export NODE_PSK=$NODE_PSK
    export NODE_SLUG=$NODE_SLUG
    
    ./bin/node &
    NODE_PID=$!
    echo $NODE_PID > /tmp/cynodeai-node.pid
    
    sleep 2
    if kill -0 $NODE_PID 2>/dev/null; then
        log_info "Node started (PID: $NODE_PID)"
    else
        log_error "Failed to start node"
        exit 1
    fi
}

# Function to stop all services
stop_all() {
    log_info "Stopping all services..."
    
    if [ -f /tmp/orchestrator-api.pid ]; then
        kill $(cat /tmp/orchestrator-api.pid) 2>/dev/null || true
        rm /tmp/orchestrator-api.pid
    fi
    
    if [ -f /tmp/cynodeai-node.pid ]; then
        kill $(cat /tmp/cynodeai-node.pid) 2>/dev/null || true
        rm /tmp/cynodeai-node.pid
    fi
    
    stop_postgres
}

# Function to run E2E demo test
run_e2e_test() {
    log_info "Running E2E demo test..."
    
    BASE_URL="http://localhost:$ORCHESTRATOR_PORT"
    
    # Test 1: Login as admin
    log_info "Test 1: Login as admin..."
    LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"handle\": \"admin\", \"password\": \"$ADMIN_PASSWORD\"}")
    
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.access_token')
    if [ "$ACCESS_TOKEN" == "null" ] || [ -z "$ACCESS_TOKEN" ]; then
        log_error "Login failed: $LOGIN_RESPONSE"
        return 1
    fi
    log_info "Login successful, got access token"
    
    # Test 2: Get current user info
    log_info "Test 2: Get current user info..."
    USER_RESPONSE=$(curl -s -X GET "$BASE_URL/v1/users/me" \
        -H "Authorization: Bearer $ACCESS_TOKEN")
    
    USER_HANDLE=$(echo "$USER_RESPONSE" | jq -r '.handle')
    if [ "$USER_HANDLE" != "admin" ]; then
        log_error "Get user failed: $USER_RESPONSE"
        return 1
    fi
    log_info "User info retrieved: $USER_HANDLE"
    
    # Test 3: Create a task
    log_info "Test 3: Create a task..."
    TASK_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/tasks" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"prompt": "echo Hello from sandbox"}')
    
    TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.id')
    if [ "$TASK_ID" == "null" ] || [ -z "$TASK_ID" ]; then
        log_error "Create task failed: $TASK_RESPONSE"
        return 1
    fi
    log_info "Task created with ID: $TASK_ID"
    
    # Test 4: Get task details
    log_info "Test 4: Get task details..."
    TASK_DETAILS=$(curl -s -X GET "$BASE_URL/v1/tasks/$TASK_ID" \
        -H "Authorization: Bearer $ACCESS_TOKEN")
    
    TASK_STATUS=$(echo "$TASK_DETAILS" | jq -r '.status')
    log_info "Task status: $TASK_STATUS"
    
    # Test 5: Get task result
    log_info "Test 5: Get task result..."
    RESULT_RESPONSE=$(curl -s -X GET "$BASE_URL/v1/tasks/$TASK_ID/result" \
        -H "Authorization: Bearer $ACCESS_TOKEN")
    
    log_info "Task result: $RESULT_RESPONSE"
    
    # Test 6: Node registration (simulate)
    log_info "Test 6: Node registration..."
    NODE_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/nodes/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"psk\": \"$NODE_PSK\",
            \"capability\": {
                \"version\": 1,
                \"reported_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
                \"node\": {\"node_slug\": \"test-e2e-node\"},
                \"platform\": {\"os\": \"linux\", \"arch\": \"amd64\"},
                \"compute\": {\"cpu_cores\": 4, \"ram_mb\": 8192}
            }
        }")
    
    NODE_JWT=$(echo "$NODE_RESPONSE" | jq -r '.auth.node_jwt')
    if [ "$NODE_JWT" == "null" ] || [ -z "$NODE_JWT" ]; then
        log_error "Node registration failed: $NODE_RESPONSE"
        return 1
    fi
    log_info "Node registered successfully"
    
    # Test 7: Report capability with node JWT
    log_info "Test 7: Report capability..."
    CAP_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/nodes/capability" \
        -H "Authorization: Bearer $NODE_JWT" \
        -H "Content-Type: application/json" \
        -d "{
            \"version\": 1,
            \"reported_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
            \"node\": {\"node_slug\": \"test-e2e-node\"},
            \"platform\": {\"os\": \"linux\", \"arch\": \"amd64\"},
            \"compute\": {\"cpu_cores\": 4, \"ram_mb\": 8192}
        }")
    
    # Capability report returns 204 No Content on success
    log_info "Capability report submitted"
    
    # Test 8: Refresh token
    log_info "Test 8: Token refresh..."
    REFRESH_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.refresh_token')
    REFRESH_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/auth/refresh" \
        -H "Content-Type: application/json" \
        -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}")
    
    NEW_ACCESS_TOKEN=$(echo "$REFRESH_RESPONSE" | jq -r '.access_token')
    if [ "$NEW_ACCESS_TOKEN" == "null" ] || [ -z "$NEW_ACCESS_TOKEN" ]; then
        log_error "Token refresh failed: $REFRESH_RESPONSE"
        return 1
    fi
    log_info "Token refreshed successfully"
    
    # Test 9: Logout
    log_info "Test 9: Logout..."
    LOGOUT_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/v1/auth/logout" \
        -H "Authorization: Bearer $NEW_ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"refresh_token\": \"$(echo "$REFRESH_RESPONSE" | jq -r '.refresh_token')\"}")
    
    HTTP_CODE=$(echo "$LOGOUT_RESPONSE" | tail -n 1)
    if [ "$HTTP_CODE" == "204" ]; then
        log_info "Logout successful"
    else
        log_warn "Logout returned unexpected code: $HTTP_CODE"
    fi
    
    log_info "All E2E tests passed!"
    return 0
}

# Function to show usage
show_usage() {
    echo "CyNodeAI Development Setup Script"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  start-db        Start PostgreSQL container"
    echo "  stop-db         Stop PostgreSQL container"
    echo "  clean-db        Stop and remove PostgreSQL container and volume"
    echo "  migrate         Run database migrations"
    echo "  build           Build orchestrator-api and node binaries"
    echo "  start           Start all services (db, orchestrator-api)"
    echo "  stop            Stop all services"
    echo "  test-e2e        Run E2E demo test"
    echo "  full-demo       Full demo: start all services and run E2E test"
    echo ""
    echo "Environment Variables:"
    echo "  POSTGRES_PORT     PostgreSQL port (default: 5432)"
    echo "  ORCHESTRATOR_PORT Orchestrator API port (default: 8080)"
    echo "  ADMIN_PASSWORD    Admin user password (default: admin123)"
    echo "  NODE_PSK          Node registration PSK (default: dev-node-psk-secret)"
}

# Main script
case "${1:-}" in
    start-db)
        start_postgres
        ;;
    stop-db)
        stop_postgres
        ;;
    clean-db)
        clean_postgres
        ;;
    migrate)
        run_migrations
        ;;
    build)
        build_binaries
        ;;
    start)
        start_postgres
        build_binaries
        run_migrations
        start_orchestrator
        log_info "Services started. API available at http://localhost:$ORCHESTRATOR_PORT"
        log_info "Use '$0 test-e2e' to run the E2E demo test"
        log_info "Use '$0 stop' to stop all services"
        ;;
    stop)
        stop_all
        ;;
    test-e2e)
        run_e2e_test
        ;;
    full-demo)
        start_postgres
        build_binaries
        run_migrations
        start_orchestrator
        sleep 2
        run_e2e_test
        log_info ""
        log_info "Demo completed! Services are still running."
        log_info "Use '$0 stop' to stop all services"
        ;;
    help|--help|-h)
        show_usage
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
