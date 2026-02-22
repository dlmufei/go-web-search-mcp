package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/cors"

	"github.com/cliffyan/go-web-search-mcp/internal/config"
	"github.com/cliffyan/go-web-search-mcp/internal/engine"
	"github.com/cliffyan/go-web-search-mcp/internal/mcp"
)

// Server MCP HTTP 服务器
type Server struct {
	config        *config.Config
	engineManager *engine.Manager
	mcpHandler    *mcp.Handler
	sessions      map[string]*Session
	sessionsMu    sync.RWMutex
}

// Session 会话信息
type Session struct {
	ID        string
	CreatedAt time.Time
}

// New 创建新的服务器实例
func New(cfg *config.Config, em *engine.Manager) *Server {
	return &Server{
		config:        cfg,
		engineManager: em,
		mcpHandler:    mcp.NewHandler(cfg, em),
		sessions:      make(map[string]*Session),
	}
}

// Start 启动 HTTP 服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// MCP 端点
	mux.HandleFunc("/mcp", s.handleMCP)

	// SSE 端点（兼容旧客户端）
	mux.HandleFunc("/sse", s.handleSSE)

	// 健康检查
	mux.HandleFunc("/health", s.handleHealth)

	// 应用 CORS 中间件
	var handler http.Handler = mux
	if s.config.IsEnableCORS() {
		c := cors.New(cors.Options{
			AllowedOrigins:   []string{s.config.GetCORSOrigin()},
			AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "mcp-session-id"},
			AllowCredentials: true,
		})
		handler = c.Handler(mux)
	}

	addr := fmt.Sprintf("%s:%d", s.config.GetHost(), s.config.GetPort())
	log.Printf("🚀 Starting MCP HTTP server on %s", addr)
	log.Printf("📡 MCP endpoint: http://%s/mcp", addr)
	log.Printf("📡 SSE endpoint: http://%s/sse", addr)
	log.Printf("❤️ Health check: http://%s/health", addr)

	return http.ListenAndServe(addr, handler)
}

// handleMCP 处理 MCP 请求
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleMCPPost(w, r)
	case http.MethodGet:
		s.handleMCPGet(w, r)
	case http.MethodDelete:
		s.handleMCPDelete(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMCPPost 处理 MCP POST 请求
func (s *Server) handleMCPPost(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var req mcp.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, nil, -32700, "Parse error: "+err.Error())
		return
	}

	// 检查 session
	sessionID := r.Header.Get("mcp-session-id")

	// 如果是初始化请求，创建新会话
	if req.Method == "initialize" && sessionID == "" {
		sessionID = uuid.New().String()
		s.sessionsMu.Lock()
		s.sessions[sessionID] = &Session{
			ID:        sessionID,
			CreatedAt: time.Now(),
		}
		s.sessionsMu.Unlock()
		w.Header().Set("mcp-session-id", sessionID)
		log.Printf("📝 Created new session: %s", sessionID)
	}

	// 处理请求
	ctx := r.Context()
	resp := s.mcpHandler.HandleRequest(ctx, req)

	// 对于通知类型，返回 204
	if req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("❌ Failed to encode response: %v", err)
	}
}

// handleMCPGet 处理 MCP GET 请求（SSE 流）
func (s *Server) handleMCPGet(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("mcp-session-id")
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	s.sessionsMu.RLock()
	_, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()

	if !exists {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// SSE 响应
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// 发送初始端点信息
	fmt.Fprintf(w, "event: endpoint\ndata: {\"uri\": \"/mcp\"}\n\n")
	flusher.Flush()

	// 保持连接并定期发送心跳
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// handleMCPDelete 处理 MCP DELETE 请求（关闭会话）
func (s *Server) handleMCPDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("mcp-session-id")
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	s.sessionsMu.Lock()
	delete(s.sessions, sessionID)
	s.sessionsMu.Unlock()

	log.Printf("🗑️ Deleted session: %s", sessionID)
	w.WriteHeader(http.StatusOK)
}

// handleSSE 处理 SSE 端点（兼容旧客户端）
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 创建新会话
	sessionID := uuid.New().String()
	s.sessionsMu.Lock()
	s.sessions[sessionID] = &Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
	}
	s.sessionsMu.Unlock()

	// SSE 响应
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// 发送端点信息
	data := fmt.Sprintf(`{"uri": "/messages?sessionId=%s"}`, sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", data)
	flusher.Flush()

	log.Printf("📡 SSE connection established: %s", sessionID)

	// 保持连接
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			s.sessionsMu.Lock()
			delete(s.sessions, sessionID)
			s.sessionsMu.Unlock()
			log.Printf("📡 SSE connection closed: %s", sessionID)
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// handleHealth 健康检查端点
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"service": s.config.GetMCPServerName(),
		"version": s.config.GetMCPServerVersion(),
		"engines": s.engineManager.GetEngineNames(),
	})
}

// sendError 发送错误响应
func (s *Server) sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.RPCError{
			Code:    code,
			Message: message,
		},
	})
}
