package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cliffyan/go-web-search-mcp/internal/config"
	"github.com/cliffyan/go-web-search-mcp/internal/engine"
)

const (
	MCPVersion = "2024-11-05"
)

// Handler MCP 请求处理器
type Handler struct {
	config        *config.Config
	engineManager *engine.Manager
}

// NewHandler 创建 MCP 处理器
func NewHandler(cfg *config.Config, em *engine.Manager) *Handler {
	return &Handler{
		config:        cfg,
		engineManager: em,
	}
}

// HandleRequest 处理 MCP JSON-RPC 请求
func (h *Handler) HandleRequest(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	log.Printf("📥 MCP Request: method=%s, id=%v", req.Method, req.ID)

	var result interface{}
	var err error

	switch req.Method {
	case "initialize":
		result = h.handleInitialize()
	case "notifications/initialized":
		// 通知类型，不需要返回结果
		return JSONRPCResponse{} // 空响应，由调用者处理
	case "tools/list":
		result = h.handleToolsList()
	case "tools/call":
		result, err = h.handleToolsCall(ctx, req.Params)
	case "resources/list":
		result = ListResourcesResult{Resources: []interface{}{}}
	case "prompts/list":
		result = ListPromptsResult{Prompts: []interface{}{}}
	default:
		err = fmt.Errorf("unknown method: %s", req.Method)
	}

	if err != nil {
		log.Printf("❌ MCP Error: %v", err)
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32603,
				Message: err.Error(),
			},
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleInitialize 处理初始化请求
func (h *Handler) handleInitialize() InitializeResult {
	return InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: Capability{
			Tools: ToolCapability{ListChanged: false},
		},
		ServerInfo: ServerInfo{
			Name:    h.config.GetMCPServerName(),
			Version: h.config.GetMCPServerVersion(),
		},
	}
}

// handleToolsList 处理工具列表请求
func (h *Handler) handleToolsList() ListToolsResult {
	return ListToolsResult{
		Tools: GetTools(h.config),
	}
}

// handleToolsCall 处理工具调用请求
func (h *Handler) handleToolsCall(ctx context.Context, params interface{}) (*CallToolResult, error) {
	// 解析参数
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	var callParams CallToolParams
	if err := json.Unmarshal(paramsBytes, &callParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal params: %w", err)
	}

	log.Printf("🔧 Tool call: name=%s, args=%v", callParams.Name, callParams.Arguments)

	// 使用配置的工具名称进行匹配
	searchToolName := h.config.GetMCPSearchToolName()

	switch callParams.Name {
	case searchToolName:
		return h.handleSearch(ctx, callParams.Arguments)
	default:
		return &CallToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", callParams.Name)}},
			IsError: true,
		}, nil
	}
}

// handleSearch 处理搜索请求
func (h *Handler) handleSearch(ctx context.Context, args map[string]interface{}) (*CallToolResult, error) {
	// 解析参数
	query, _ := args["query"].(string)
	if query == "" {
		return &CallToolResult{
			Content: []ContentItem{{Type: "text", Text: "query is required"}},
			IsError: true,
		}, nil
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	var engines []string
	if e, ok := args["engines"].([]interface{}); ok {
		for _, eng := range e {
			if s, ok := eng.(string); ok {
				engines = append(engines, s)
			}
		}
	}

	// 执行搜索
	results, err := h.engineManager.Search(ctx, engine.SearchRequest{
		Query:   query,
		Limit:   limit,
		Engines: engines,
	})

	if err != nil {
		return &CallToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Search failed: %v", err)}},
			IsError: true,
		}, nil
	}

	// 格式化结果
	resultJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return &CallToolResult{
			Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Failed to format results: %v", err)}},
			IsError: true,
		}, nil
	}

	return &CallToolResult{
		Content: []ContentItem{{Type: "text", Text: string(resultJSON)}},
	}, nil
}
