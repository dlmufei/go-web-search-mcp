package engine

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

// BrowserGoogleEngine 使用无头浏览器的 Google 搜索引擎
type BrowserGoogleEngine struct {
	proxyURL string
	headless bool
	timeout  time.Duration
}

// NewBrowserGoogleEngine 创建浏览器版 Google 搜索引擎
func NewBrowserGoogleEngine(proxyURL string, headless bool) *BrowserGoogleEngine {
	return &BrowserGoogleEngine{
		proxyURL: proxyURL,
		headless: headless,
		timeout:  60 * time.Second,
	}
}

// Name 返回引擎名称
func (e *BrowserGoogleEngine) Name() string {
	return "browser_google"
}

// Search 使用浏览器执行 Google 搜索
func (e *BrowserGoogleEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
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
func (e *BrowserGoogleEngine) searchPage(ctx context.Context, query string, page int) ([]SearchResult, error) {
	bm := GetBrowserManager()

	// 创建新的 tab 上下文
	tabCtx, cancel := bm.NewTabContext(e.timeout)
	defer cancel()

	// 构建搜索 URL
	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s&start=%d&hl=en",
		url.QueryEscape(query), page*10)

	var html string

	log.Printf("🌐 [BrowserGoogle] Navigating to: %s", searchURL)

	// 执行浏览器操作
	err := chromedp.Run(tabCtx,
		// 导航到搜索页面
		chromedp.Navigate(searchURL),

		// 等待搜索结果加载 (Google 使用 #search 或 #rso)
		chromedp.WaitReady("#search", chromedp.ByID),

		// 等待一小段时间确保页面完全加载
		chromedp.Sleep(2*time.Second),

		// 滚动页面
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight / 2)`, nil),
		chromedp.Sleep(500*time.Millisecond),

		// 获取页面 HTML
		chromedp.OuterHTML("html", &html),
	)

	if err != nil {
		return nil, fmt.Errorf("browser navigation failed: %w", err)
	}

	log.Printf("🔍 [BrowserGoogle] Got page HTML, size: %d bytes", len(html))

	// 解析 HTML
	results, err := e.parseHTML(html)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ [BrowserGoogle] Page %d: found %d results", page, len(results))
	return results, nil
}

// parseHTML 解析 HTML 提取搜索结果
func (e *BrowserGoogleEngine) parseHTML(html string) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	var results []SearchResult

	// Google 搜索结果选择器 - 多种模式
	selectors := []string{
		"div.g",           // 标准结果
		"div[data-ved]",   // 带 data-ved 的结果
		"div.Gx5Zad",      // 另一种结果容器
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			// 避免重复
			if s.Find("div.g").Length() > 0 && selector != "div.g" {
				return
			}

			// 获取链接
			linkEl := s.Find("a[href]").First()
			if linkEl.Length() == 0 {
				return
			}

			href, exists := linkEl.Attr("href")
			if !exists {
				return
			}

			// 过滤 Google 内部链接
			if !strings.HasPrefix(href, "http") || 
			   strings.Contains(href, "google.com") ||
			   strings.Contains(href, "webcache.googleusercontent.com") {
				return
			}

			// 获取标题
			title := ""
			titleEl := s.Find("h3")
			if titleEl.Length() > 0 {
				title = strings.TrimSpace(titleEl.First().Text())
			}
			if title == "" {
				return
			}

			// 获取描述
			description := ""
			// 尝试多种描述选择器
			descSelectors := []string{
				"div[data-sncf]",
				"div.VwiC3b",
				"span.aCOpRe",
				"div.IsZvec",
			}
			for _, descSel := range descSelectors {
				descEl := s.Find(descSel)
				if descEl.Length() > 0 {
					description = strings.TrimSpace(descEl.First().Text())
					if description != "" {
						break
					}
				}
			}

			// 获取来源
			source := ""
			citeEl := s.Find("cite")
			if citeEl.Length() > 0 {
				source = strings.TrimSpace(citeEl.First().Text())
			}

			results = append(results, SearchResult{
				Title:       title,
				URL:         href,
				Description: description,
				Source:      source,
				Engine:      "browser_google",
			})
		})

		if len(results) > 0 {
			break
		}
	}

	// 去重
	seen := make(map[string]bool)
	uniqueResults := make([]SearchResult, 0)
	for _, r := range results {
		if !seen[r.URL] {
			seen[r.URL] = true
			uniqueResults = append(uniqueResults, r)
		}
	}

	return uniqueResults, nil
}
