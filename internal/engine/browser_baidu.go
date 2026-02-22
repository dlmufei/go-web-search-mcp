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

// BrowserBaiduEngine 使用无头浏览器的 Baidu 搜索引擎
type BrowserBaiduEngine struct {
	proxyURL string
	headless bool
	timeout  time.Duration
}

// NewBrowserBaiduEngine 创建浏览器版 Baidu 搜索引擎
func NewBrowserBaiduEngine(proxyURL string, headless bool) *BrowserBaiduEngine {
	return &BrowserBaiduEngine{
		proxyURL: proxyURL,
		headless: headless,
		timeout:  60 * time.Second,
	}
}

// Name 返回引擎名称
func (e *BrowserBaiduEngine) Name() string {
	return "browser_baidu"
}

// Search 使用浏览器执行 Baidu 搜索
func (e *BrowserBaiduEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
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
func (e *BrowserBaiduEngine) searchPage(ctx context.Context, query string, page int) ([]SearchResult, error) {
	bm := GetBrowserManager()

	// 创建新的 tab 上下文
	tabCtx, cancel := bm.NewTabContext(e.timeout)
	defer cancel()

	// 构建搜索 URL
	searchURL := fmt.Sprintf("https://www.baidu.com/s?wd=%s&pn=%d",
		url.QueryEscape(query), page*10)

	var html string

	log.Printf("🌐 [BrowserBaidu] Navigating to: %s", searchURL)

	// 执行浏览器操作
	err := chromedp.Run(tabCtx,
		// 导航到搜索页面
		chromedp.Navigate(searchURL),

		// 等待搜索结果加载
		chromedp.WaitVisible("#content_left", chromedp.ByID),

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

	log.Printf("🔍 [BrowserBaidu] Got page HTML, size: %d bytes", len(html))

	// 解析 HTML
	results, err := e.parseHTML(html)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ [BrowserBaidu] Page %d: found %d results", page, len(results))
	return results, nil
}

// parseHTML 解析 HTML 提取搜索结果
func (e *BrowserBaiduEngine) parseHTML(html string) ([]SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	var results []SearchResult

	// 百度搜索结果选择器
	doc.Find("div.result, div.result-op, div.c-container").Each(func(i int, s *goquery.Selection) {
		// 获取标题和链接
		titleEl := s.Find("h3 a")
		if titleEl.Length() == 0 {
			titleEl = s.Find("a[href]").First()
		}
		if titleEl.Length() == 0 {
			return
		}

		title := strings.TrimSpace(titleEl.Text())
		if title == "" {
			return
		}

		href, exists := titleEl.Attr("href")
		if !exists {
			return
		}

		// 百度的链接可能是跳转链接
		if !strings.HasPrefix(href, "http") {
			return
		}

		// 获取描述
		description := ""
		descSelectors := []string{
			"div.c-abstract",
			"span.c-abstract",
			"div.c-span-last",
			"div.content-right_8Zs40",
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
		sourceSelectors := []string{
			"span.c-showurl",
			"a.c-showurl",
			"span.source_1Vdff",
		}
		for _, srcSel := range sourceSelectors {
			srcEl := s.Find(srcSel)
			if srcEl.Length() > 0 {
				source = strings.TrimSpace(srcEl.First().Text())
				if source != "" {
					break
				}
			}
		}

		// 过滤广告和无效结果
		if strings.Contains(title, "广告") {
			return
		}

		results = append(results, SearchResult{
			Title:       title,
			URL:         href,
			Description: description,
			Source:      source,
			Engine:      "browser_baidu",
		})
	})

	return results, nil
}
