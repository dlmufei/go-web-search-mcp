#!/bin/bash

# go-web-search-mcp 运行脚本
# 用法: ./run.sh [build|start|stop|restart|logs|docker]

set -e

APP_NAME="go-web-search-mcp"
BIN_PATH="./bin/${APP_NAME}"
PID_FILE="./${APP_NAME}.pid"

# 颜色输出
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

# 构建
build() {
    log_info "Building ${APP_NAME}..."
    mkdir -p bin
    go build -ldflags="-s -w" -o ${BIN_PATH} ./cmd/server
    log_info "Build completed: ${BIN_PATH}"
}

# 启动
start() {
    if [ -f "${PID_FILE}" ]; then
        PID=$(cat ${PID_FILE})
        if kill -0 ${PID} 2>/dev/null; then
            log_warn "${APP_NAME} is already running (PID: ${PID})"
            return
        fi
    fi

    if [ ! -f "${BIN_PATH}" ]; then
        log_warn "Binary not found, building first..."
        build
    fi

    log_info "Starting ${APP_NAME}..."
    nohup ${BIN_PATH} > logs/${APP_NAME}.log 2>&1 &
    echo $! > ${PID_FILE}
    log_info "${APP_NAME} started (PID: $(cat ${PID_FILE}))"
}

# 停止
stop() {
    if [ ! -f "${PID_FILE}" ]; then
        log_warn "${APP_NAME} is not running"
        return
    fi

    PID=$(cat ${PID_FILE})
    if kill -0 ${PID} 2>/dev/null; then
        log_info "Stopping ${APP_NAME} (PID: ${PID})..."
        kill ${PID}
        rm -f ${PID_FILE}
        log_info "${APP_NAME} stopped"
    else
        log_warn "${APP_NAME} is not running"
        rm -f ${PID_FILE}
    fi
}

# 重启
restart() {
    stop
    sleep 1
    start
}

# 查看日志
logs() {
    if [ -f "logs/${APP_NAME}.log" ]; then
        tail -f logs/${APP_NAME}.log
    else
        log_error "Log file not found"
    fi
}

# Docker 构建和运行
docker_run() {
    log_info "Building Docker image..."
    docker build -t ${APP_NAME}:latest .
    
    log_info "Running Docker container..."
    docker run -d \
        --name ${APP_NAME} \
        -p 3000:3000 \
        -v $(pwd)/config.yaml:/app/config.yaml:ro \
        ${APP_NAME}:latest
    
    log_info "Container started. Access: http://localhost:3000/mcp"
}

# 本地开发运行
dev() {
    log_info "Starting in development mode..."
    log_info "Using config file: config.yaml"
    go run ./cmd/server
}

# 测试搜索功能
test_search() {
    log_info "Testing search functionality..."
    
    # 初始化
    INIT_RESP=$(curl -s -X POST http://localhost:3000/mcp \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}')
    
    SESSION_ID=$(echo $INIT_RESP | grep -o '"mcp-session-id":"[^"]*"' | cut -d'"' -f4 || echo "")
    log_info "Session ID: ${SESSION_ID}"
    
    # 列出工具
    log_info "Listing tools..."
    curl -s -X POST http://localhost:3000/mcp \
        -H "Content-Type: application/json" \
        -H "mcp-session-id: ${SESSION_ID}" \
        -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | jq .
    
    # 执行搜索
    log_info "Executing search..."
    curl -s -X POST http://localhost:3000/mcp \
        -H "Content-Type: application/json" \
        -H "mcp-session-id: ${SESSION_ID}" \
        -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"query":"MCP protocol","limit":5}}}' | jq .
}

# 创建必要目录
mkdir -p logs bin

# 主入口
case "$1" in
    build)
        build
        ;;
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    logs)
        logs
        ;;
    docker)
        docker_run
        ;;
    dev)
        dev
        ;;
    test)
        test_search
        ;;
    *)
        echo "Usage: $0 {build|start|stop|restart|logs|docker|dev|test}"
        echo ""
        echo "Commands:"
        echo "  build   - Build the binary"
        echo "  start   - Start the server in background"
        echo "  stop    - Stop the server"
        echo "  restart - Restart the server"
        echo "  logs    - View server logs"
        echo "  docker  - Build and run Docker container"
        echo "  dev     - Run in development mode"
        echo "  test    - Test search functionality"
        exit 1
        ;;
esac
