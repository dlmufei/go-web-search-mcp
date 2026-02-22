package engine

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/cliffyan/go-web-search-mcp/internal/config"
)

// Manager 搜索引擎管理器
type Manager struct {
	engines map[string]SearchEngine
	config  *config.Config
	mu      sync.RWMutex
}

// NewManager 创建搜索引擎管理器
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		engines: make(map[string]SearchEngine),
		config:  cfg,
	}

	// 初始化搜索引擎
	m.initEngines()

	return m
}

// initEngines 初始化所有搜索引擎
func (m *Manager) initEngines() {
	proxyURL := ""
	if m.config.IsUseProxy() {
		proxyURL = m.config.GetProxyURL()
	}

	// 注册 HTTP 版搜索引擎
	m.RegisterEngine(NewBingEngine(proxyURL))
	m.RegisterEngine(NewDuckDuckGoEngine(proxyURL))
	m.RegisterEngine(NewBaiduEngine(proxyURL))
	m.RegisterEngine(NewSogouEngine(proxyURL))

	// 注册浏览器版搜索引擎（如果启用）
	if m.config.IsBrowserEnabled() {
		headless := m.config.IsBrowserHeadless()
		m.RegisterEngine(NewBrowserBingEngine(proxyURL, headless))
		m.RegisterEngine(NewBrowserGoogleEngine(proxyURL, headless))
		m.RegisterEngine(NewBrowserBaiduEngine(proxyURL, headless))
		log.Printf("🌐 Browser engines enabled (headless=%v)", headless)
	}

	log.Printf("✅ Initialized %d search engine(s): %v", len(m.engines), m.GetEngineNames())
}

// RegisterEngine 注册搜索引擎
func (m *Manager) RegisterEngine(engine SearchEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engines[engine.Name()] = engine
	log.Printf("📝 Registered search engine: %s", engine.Name())
}

// GetEngine 获取搜索引擎
func (m *Manager) GetEngine(name string) (SearchEngine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	engine, ok := m.engines[name]
	return engine, ok
}

// GetEngineNames 获取所有引擎名称
func (m *Manager) GetEngineNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.engines))
	for name := range m.engines {
		names = append(names, name)
	}
	return names
}

// Search 执行搜索（支持多引擎）
func (m *Manager) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	// 确定使用的引擎
	engines := req.Engines
	if len(engines) == 0 {
		engines = []string{m.config.GetDefaultSearchEngine()}
	}

	// 设置默认 limit
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	var allResults []SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex
	var lastErr error

	for _, engineName := range engines {
		// 检查引擎是否被允许
		if !m.config.IsEngineAllowed(engineName) {
			log.Printf("⚠️ Engine %s is not allowed, skipping", engineName)
			continue
		}

		engine, ok := m.GetEngine(engineName)
		if !ok {
			log.Printf("⚠️ Engine %s not found, skipping", engineName)
			continue
		}

		wg.Add(1)
		go func(eng SearchEngine) {
			defer wg.Done()

			results, err := eng.Search(ctx, req.Query, limit)
			if err != nil {
				log.Printf("❌ Search with %s failed: %v", eng.Name(), err)
				mu.Lock()
				lastErr = err
				mu.Unlock()
				return
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()

			log.Printf("✅ Search with %s returned %d results", eng.Name(), len(results))
		}(engine)
	}

	wg.Wait()

	if len(allResults) == 0 && lastErr != nil {
		return nil, fmt.Errorf("all searches failed, last error: %w", lastErr)
	}

	return allResults, nil
}
