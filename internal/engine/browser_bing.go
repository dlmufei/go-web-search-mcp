package engine

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// BrowserBingEngine 使用无头浏览器的 Bing 搜索引擎
type BrowserBingEngine struct {
	proxyURL string
	headless bool
	timeout  time.Duration
}

// NewBrowserBingEngine 创建浏览器版 Bing 搜索引擎
func NewBrowserBingEngine(proxyURL string, headless bool) *BrowserBingEngine {
	return &BrowserBingEngine{
		proxyURL: proxyURL,
		headless: headless,
		timeout:  60 * time.Second,
	}
}

// Name 返回引擎名称
func (e *BrowserBingEngine) Name() string {
	return "browser_bing"
}

// Search 使用浏览器执行 Bing 搜索
func (e *BrowserBingEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// 确保浏览器已初始化
	bm := GetBrowserManager()
	if err := bm.Initialize(e.proxyURL, e.headless); err != nil {
		return nil, fmt.Errorf("failed to initialize browser: %w", err)
	}

	var allResults []SearchResult
	page := 0

	for len(allResults) < limit && page < 3 {
		results, err := e.searchPage(ctx, query, page)
		if err != nil {
			if len(allResults) > 0 {
				break
			}
			return nil, err
		}

		if len(results) == 0 {
			break
		}

		allResults = append(allResults, results...)
		page++
	}

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults, nil
}

// searchPage 搜索单页
func (e *BrowserBingEngine) searchPage(ctx context.Context, query string, page int) ([]SearchResult, error) {
	bm := GetBrowserManager()

	// 创建新的 tab 上下文
	tabCtx, cancel := bm.NewTabContext(e.timeout)
	defer cancel()

	// 构建搜索 URL - 使用国际版 Bing
	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s&first=%d&setlang=en",
		url.QueryEscape(query), 1+page*10)

	var html string

	log.Printf("🌐 [BrowserBing] Navigating to: %s", searchURL)

	// 执行浏览器操作
	err := chromedp.Run(tabCtx,
		// 导航到搜索页面
		chromedp.Navigate(searchURL),

		// 等待搜索结果加载
		chromedp.WaitVisible("#b_results", chromedp.ByID),

		// 等待一小段时间确保页面完全加载
		chromedp.Sleep(2*time.Second),

		// 滚动页面以加载更多内容
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight / 2)`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// 获取页面 HTML
		chromedp.OuterHTML("html", &html),
	)

	if err != nil {
		return nil, fmt.Errorf("browser navigation failed: %w", err)
	}

	log.Printf("🔍 [BrowserBing] Got page HTML, size: %d bytes", len(html))

	// 解析 HTML
	results, err := e.parseHTML(html)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ [BrowserBing] Page %d: found %d results", page, len(results))
	return results, nil
}

// parseHTML 解析 HTML 提取搜索结果
func (e *BrowserBingEngine) parseHTML(html string) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	var results []SearchResult

	// Bing 搜索结果选择器
	doc.Find("li.b_algo").Each(func(i int, s *goquery.Selection) {
		// 获取标题和链接
		titleEl := s.Find("h2 a")
		if titleEl.Length() == 0 {
			return
		}

		title := strings.TrimSpace(titleEl.Text())
		href, exists := titleEl.Attr("href")
		if !exists {
			return
		}

		// 解析真实 URL（处理 Bing 跳转链接）
		realURL := e.extractRealURL(href)
		if realURL == "" || !strings.HasPrefix(realURL, "http") {
			return
		}

		// 过滤 Bing 内部链接
		if strings.Contains(realURL, "bing.com") || strings.Contains(realURL, "microsoft.com") {
			return
		}

		// 获取描述
		description := ""
		descEl := s.Find(".b_caption p")
		if descEl.Length() > 0 {
			description = strings.TrimSpace(descEl.First().Text())
		}

		// 获取来源
		source := ""
		citeEl := s.Find("cite")
		if citeEl.Length() > 0 {
			source = strings.TrimSpace(citeEl.First().Text())
		}

		results = append(results, SearchResult{
			Title:       title,
			URL:         realURL,
			Description: description,
			Source:      source,
			Engine:      "browser_bing",
		})
	})

	return results, nil
}

// extractRealURL 从 Bing 跳转链接中提取真实 URL
func (e *BrowserBingEngine) extractRealURL(href string) string {
	// 如果已经是正常 URL，直接返回
	if !strings.Contains(href, "bing.com/ck/a") {
		return href
	}

	// 解析 Bing 跳转 URL
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}

	// 尝试从 u 参数获取真实 URL（Base64 编码）
	u := parsed.Query().Get("u")
	if u != "" {
		// Bing 使用 a1 前缀 + Base64 编码
		if strings.HasPrefix(u, "a1") {
			u = u[2:] // 移除 a1 前缀
		}
		// Base64 URL 解码
		decoded, err := base64.RawURLEncoding.DecodeString(u)
		if err == nil && len(decoded) > 0 {
			return string(decoded)
		}
	}

	return href
}
