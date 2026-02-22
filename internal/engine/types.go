package engine

import "context"

// SearchResult 搜索结果
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Engine      string `json:"engine"`
}

// SearchEngine 搜索引擎接口
type SearchEngine interface {
	// Name 返回引擎名称
	Name() string
	// Search 执行搜索
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query   string   `json:"query"`
	Limit   int      `json:"limit,omitempty"`
	Engines []string `json:"engines,omitempty"`
}
