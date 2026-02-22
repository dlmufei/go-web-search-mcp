# go-web-search-mcp

一个使用 Go 实现的 MCP (Model Context Protocol) 网络搜索服务器，支持多搜索引擎聚合搜索。

## 功能特性

- 🔍 **多引擎搜索**: 支持 Bing、DuckDuckGo 等搜索引擎
- 🚀 **高性能**: Go 原生协程实现，内存占用低，启动快速
- 🔌 **MCP 协议**: 完整支持 MCP 协议，兼容 StreamableHTTP、SSE 传输
- 🌐 **HTTP 代理**: 支持配置 HTTP 代理解决网络访问限制
- 🐳 **Docker 部署**: 提供 Dockerfile，一键部署
- 📝 **无需 API Key**: 通过网页爬取获取搜索结果，无需申请 API
- 📄 **YAML 配置**: 使用 YAML 文件进行配置，简单直观
- 🔧 **自定义工具名**: 支持自定义 MCP 工具名称和描述

## 快速开始

### 本地运行

```bash
# 克隆项目
cd go-web-search-mcp

# 下载依赖
go mod tidy

# 运行开发模式
./run.sh dev

# 或者构建后运行
./run.sh build
./run.sh start
```

### Docker 部署

```bash
# 使用 docker-compose
docker-compose up -d

# 或者手动构建运行
docker build -t go-web-search-mcp .
docker run -d --name go-web-search-mcp -p 3456:3456 -v $(pwd)/config.yaml:/app/config.yaml:ro go-web-search-mcp
```

## 配置

服务通过 YAML 配置文件进行配置。默认会按以下顺序搜索配置文件：

1. 环境变量 `CONFIG_FILE` 指定的路径
2. 当前目录下的 `config.yaml` 或 `config.yml`
3. `configs/config.yaml` 或 `configs/config.yml`

### 配置文件示例 (config.yaml)

```yaml
# 服务器配置
server:
  port: 3456
  host: "0.0.0.0"
  cors:
    enabled: false
    origin: "*"

# 搜索引擎配置
search:
  default_engine: "duckduckgo"
  allowed_engines: []

# 代理配置
proxy:
  enabled: false
  url: "http://127.0.0.1:7890"

# MCP 协议配置
mcp:
  server_name: "go-web-search-mcp"
  server_version: "1.0.0"
  tools:
    search_name: "search"
    search_description: "Search the web using multiple engines..."
```

### 配置说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `server.port` | int | `3456` | HTTP 服务端口 |
| `server.host` | string | `0.0.0.0` | 监听地址 |
| `server.cors.enabled` | bool | `false` | 是否启用 CORS |
| `server.cors.origin` | string | `*` | CORS 允许的来源 |
| `search.default_engine` | string | `duckduckgo` | 默认搜索引擎 |
| `search.allowed_engines` | []string | `[]` | 允许的搜索引擎列表（空表示全部允许） |
| `proxy.enabled` | bool | `false` | 是否启用 HTTP 代理 |
| `proxy.url` | string | `http://127.0.0.1:7890` | 代理服务器地址 |
| `mcp.server_name` | string | `go-web-search-mcp` | MCP 服务器名称 |
| `mcp.server_version` | string | `1.0.0` | MCP 服务器版本 |
| `mcp.tools.search_name` | string | `search` | 搜索工具名称（可自定义） |
| `mcp.tools.search_description` | string | ... | 搜索工具描述（可自定义） |

### 指定配置文件路径

可以通过环境变量指定配置文件路径：

```bash
CONFIG_FILE=/path/to/config.yaml ./bin/go-web-search-mcp
```

## MCP 客户端配置

### StreamableHTTP（推荐）

```json
{
  "mcpServers": {
    "go-web-search-mcp": {
      "type": "streamableHttp",
      "url": "http://localhost:3456/mcp"
    }
  }
}
```

### SSE（兼容模式）

```json
{
  "mcpServers": {
    "go-web-search-mcp": {
      "type": "sse",
      "url": "http://localhost:3456/sse"
    }
  }
}
```

### Cherry Studio

```json
{
  "mcpServers": {
    "go-web-search-mcp": {
      "name": "Go Web Search",
      "type": "streamableHttp",
      "description": "Multi-engine web search with MCP",
      "isActive": true,
      "baseUrl": "http://localhost:3456/mcp"
    }
  }
}
```

## API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/mcp` | POST | MCP JSON-RPC 请求 |
| `/mcp` | GET | MCP SSE 流（需要 session-id） |
| `/mcp` | DELETE | 关闭会话 |
| `/sse` | GET | SSE 连接（兼容旧客户端） |
| `/health` | GET | 健康检查 |

## MCP 工具

### search（默认名称，可通过配置自定义）

搜索网络内容。

**参数：**
- `query` (string, required): 搜索关键词
- `limit` (number, optional): 返回结果数量，默认 10
- `engines` (array, optional): 使用的搜索引擎列表

**示例：**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "search",
    "arguments": {
      "query": "MCP protocol",
      "limit": 5,
      "engines": ["duckduckgo"]
    }
  }
}
```

**返回：**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "[{\"title\":\"...\",\"url\":\"...\",\"description\":\"...\",\"engine\":\"duckduckgo\"}]"
      }
    ]
  }
}
```

## 自定义工具名称

如果你需要自定义 MCP 工具的名称（例如避免与其他 MCP 服务器冲突），可以在配置文件中修改：

```yaml
mcp:
  tools:
    search_name: "web_search"  # 自定义工具名称
    search_description: "使用多引擎搜索网页内容"  # 自定义描述
```

修改后，MCP 客户端需要使用新的工具名称来调用：

```json
{
  "method": "tools/call",
  "params": {
    "name": "web_search",  // 使用自定义名称
    "arguments": { "query": "golang" }
  }
}
```

## 测试

```bash
# 启动服务器
./run.sh dev

# 在另一个终端运行测试
./run.sh test

# 或手动测试
curl -X POST http://localhost:3456/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"golang","limit":3}}}'
```

## 项目结构

```
go-web-search-mcp/
├── cmd/server/
│   └── main.go              # 程序入口
├── internal/
│   ├── config/
│   │   └── config.go        # 配置管理（YAML 加载）
│   ├── engine/
│   │   ├── types.go         # 类型定义
│   │   ├── bing.go          # Bing 搜索引擎
│   │   ├── duckduckgo.go    # DuckDuckGo 搜索引擎
│   │   └── manager.go       # 引擎管理器
│   ├── mcp/
│   │   ├── types.go         # MCP 类型定义
│   │   ├── tools.go         # 工具定义
│   │   └── handler.go       # 请求处理
│   └── server/
│       └── server.go        # HTTP 服务器
├── config.yaml              # 配置文件
├── Dockerfile
├── docker-compose.yml
├── run.sh
├── go.mod
└── README.md
```

## TODO

- [ ] 添加更多搜索引擎（百度、Google）
- [ ] 实现文章内容抓取工具
- [ ] 添加搜索结果缓存
- [ ] 支持 STDIO 传输模式
- [ ] 添加单元测试

## License

MIT
