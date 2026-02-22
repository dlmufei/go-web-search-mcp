package engine

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// BingEngine Bing 搜索引擎实现
type BingEngine struct {
	client   *http.Client
	proxyURL string
}

// NewBingEngine 创建 Bing 搜索引擎实例
func NewBingEngine(proxyURL string) *BingEngine {
	jar, _ := cookiejar.New(nil)
	
	transport := &http.Transport{}
	if proxyURL != "" {
		if proxy, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxy)
		}
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Jar:       jar,
		Transport: transport,
	}

	return &BingEngine{
		client:   client,
		proxyURL: proxyURL,
	}
}

// Name 返回引擎名称
func (e *BingEngine) Name() string {
	return "bing"
}

// Search 执行 Bing 搜索
func (e *BingEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	var allResults []SearchResult
	pn := 0

	for len(allResults) < limit {
		results, err := e.searchPage(ctx, query, pn)
		if err != nil {
			if len(allResults) > 0 {
				// 如果已经有一些结果，就返回这些
				break
			}
			return nil, err
		}

		if len(results) == 0 {
			break
		}

		allResults = append(allResults, results...)
		pn++

		if pn > 5 {
			break
		}
	}

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults, nil
}

// searchPage 搜索单页结果
func (e *BingEngine) searchPage(ctx context.Context, query string, page int) ([]SearchResult, error) {
	// 构建请求 URL - 使用国际版
	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s&first=%d&setlang=en",
		url.QueryEscape(query), 1+page*10)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	e.setHeaders(req)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	bodyStr := string(body)
	log.Printf("🔍 Bing response size: %d bytes", len(body))

	// 首先尝试标准解析
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	results := e.parseResults(doc)
	
	// 如果标准解析没有结果，尝试正则匹配
	if len(results) == 0 {
		log.Printf("⚠️ Standard parsing found no results, trying regex extraction")
		results = e.extractResultsWithRegex(bodyStr)
	}

	log.Printf("🔍 Bing page %d: found %d results", page, len(results))
	return results, nil
}

// setHeaders 设置请求头
func (e *BingEngine) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

// parseResults 解析搜索结果
func (e *BingEngine) parseResults(doc *goquery.Document) []SearchResult {
	var results []SearchResult

	// 尝试多种选择器
	selectors := []string{
		"li.b_algo",
		"#b_results > li.b_algo",
		".b_algo",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			titleEl := s.Find("h2")
			linkEl := s.Find("h2 a")

			if titleEl.Length() == 0 || linkEl.Length() == 0 {
				return
			}

			href, exists := linkEl.Attr("href")
			if !exists || !strings.HasPrefix(href, "http") {
				return
			}

			description := ""
			descSelectors := []string{".b_caption p", "p", ".b_algoSlug"}
			for _, descSel := range descSelectors {
				descEl := s.Find(descSel)
				if descEl.Length() > 0 {
					description = strings.TrimSpace(descEl.First().Text())
					if description != "" {
						break
					}
				}
			}

			source := ""
			citeEl := s.Find("cite")
			if citeEl.Length() > 0 {
				source = strings.TrimSpace(citeEl.First().Text())
			}

			results = append(results, SearchResult{
				Title:       strings.TrimSpace(titleEl.Text()),
				URL:         href,
				Description: description,
				Source:      source,
				Engine:      "bing",
			})
		})

		if len(results) > 0 {
			break
		}
	}

	return results
}

// extractResultsWithRegex 使用正则表达式提取结果（备用方案）
func (e *BingEngine) extractResultsWithRegex(html string) []SearchResult {
	var results []SearchResult

	// 尝试匹配 Bing 结果的 URL 和标题模式
	// 这是一个简化的正则，可能需要根据实际情况调整
	urlPattern := regexp.MustCompile(`<a[^>]*href="(https?://[^"]+)"[^>]*>([^<]+)</a>`)
	matches := urlPattern.FindAllStringSubmatch(html, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		
		href := match[1]
		title := strings.TrimSpace(match[2])

		// 过滤掉 Bing 自身的链接和空标题
		if strings.Contains(href, "bing.com") || 
		   strings.Contains(href, "microsoft.com") ||
		   title == "" ||
		   seen[href] {
			continue
		}

		// 简单过滤，只保留看起来像搜索结果的链接
		if !strings.HasPrefix(href, "http") {
			continue
		}

		seen[href] = true
		results = append(results, SearchResult{
			Title:       title,
			URL:         href,
			Description: "",
			Source:      "",
			Engine:      "bing",
		})

		if len(results) >= 10 {
			break
		}
	}

	return results
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
