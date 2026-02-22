package mcp

// JSON-RPC 请求/响应类型
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP 协议类型
type InitializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    Capability `json:"capabilities"`
	ServerInfo      ServerInfo `json:"serverInfo"`
}

type Capability struct {
	Tools ToolCapability `json:"tools"`
}

type ToolCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// 工具定义
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Default     any      `json:"default,omitempty"`
	Items       *Items   `json:"items,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type Items struct {
	Type string `json:"type"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// 工具调用参数
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// 工具调用结果
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// 资源和提示列表（空实现）
type ListResourcesResult struct {
	Resources []interface{} `json:"resources"`
}

type ListPromptsResult struct {
	Prompts []interface{} `json:"prompts"`
}
