package mcp

import (
	"github.com/cliffyan/go-web-search-mcp/internal/config"
)

// GetTools 获取所有 MCP 工具定义
func GetTools(cfg *config.Config) []Tool {
	// 构建引擎枚举列表
	engineEnum := config.ValidEngines

	return []Tool{
		{
			Name:        cfg.GetMCPSearchToolName(),
			Description: cfg.GetMCPSearchToolDescription(),
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "The search query string",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of results to return (default: 10)",
						Default:     10,
					},
					"engines": {
						Type:        "array",
						Description: "Search engines to use. Available: bing, baidu, duckduckgo, google. Default uses the configured default engine.",
						Items:       &Items{Type: "string"},
						Enum:        engineEnum,
					},
				},
				Required: []string{"query"},
			},
		},
		// TODO: 后续添加更多工具
		// {
		// 	Name:        "fetchArticle",
		// 	Description: "Fetch the full content of an article from a URL",
		// 	InputSchema: InputSchema{...},
		// },
	}
}
