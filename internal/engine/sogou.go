package engine

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SogouEngine 搜狗搜索引擎实现
type SogouEngine struct {
	client   *http.Client
	proxyURL string
}

// NewSogouEngine 创建搜狗搜索引擎实例
func NewSogouEngine(proxyURL string) *SogouEngine {
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

	return &SogouEngine{
		client:   client,
		proxyURL: proxyURL,
	}
}

// Name 返回引擎名称
func (e *SogouEngine) Name() string {
	return "sogou"
}

// Search 执行搜狗搜索
func (e *SogouEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	var allResults []SearchResult
	page := 1

	for len(allResults) < limit {
		results, err := e.searchPage(ctx, query, page)
		if err != nil {
			if len(allResults) > 0 {
				log.Printf("⚠️ Sogou: Error on page %d, returning %d results collected so far: %v", page, len(allResults), err)
				break
			}
			return nil, err
		}

		if len(results) == 0 {
			log.Printf("⚠️ Sogou: No more results at page %d, ending early", page)
			break
		}

		allResults = append(allResults, results...)
		page++

		// 限制最多搜索5页
		if page > 5 {
			break
		}

		// 添加延迟避免触发限制
		if page <= 5 && len(allResults) < limit {
			time.Sleep(300 * time.Millisecond)
		}
	}

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults, nil
}

// searchPage 搜索单页结果（使用移动端页面，更稳定）
func (e *SogouEngine) searchPage(ctx context.Context, query string, page int) ([]SearchResult, error) {
	// 使用移动端 WAP 页面，更不容易触发反爬
	params := url.Values{}
	params.Set("keyword", query)
	if page > 1 {
		params.Set("page", fmt.Sprintf("%d", page))
	}

	searchURL := fmt.Sprintf("https://wap.sogou.com/web/searchList.jsp?%s", params.Encode())

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
	log.Printf("🔍 Sogou response size: %d bytes", len(body))

	// 检查是否被重定向到反爬页面
	if strings.Contains(bodyStr, "antispider") || strings.Contains(bodyStr, "验证码") {
		return nil, fmt.Errorf("sogou rate limited: anti-spider triggered")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	results := e.parseResults(doc)
	log.Printf("🔍 Sogou page %d: found %d results", page, len(results))

	return results, nil
}

// setHeaders 设置请求头
func (e *SogouEngine) setHeaders(req *http.Request) {
	// 使用移动端 User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://wap.sogou.com/")
}

// parseResults 解析搜索结果
func (e *SogouEngine) parseResults(doc *goquery.Document) []SearchResult {
	var results []SearchResult

	// 搜狗移动端结果在 .vrResult 容器中
	doc.Find(".vrResult").Each(func(i int, s *goquery.Selection) {
		result := e.parseResultItem(s)
		if result != nil {
			results = append(results, *result)
		}
	})

	return results
}

// parseResultItem 解析单个搜索结果项
func (e *SogouEngine) parseResultItem(s *goquery.Selection) *SearchResult {
	// 查找标题 - 多种选择器
	var title string
	var href string

	// 尝试不同的标题选择器
	titleSelectors := []string{
		".vr-tit a",
		".title__titleText_287f",
		".video-desc__videoTitle_812e",
		"h3 a",
		".major-title a",
	}

	for _, selector := range titleSelectors {
		titleEl := s.Find(selector).First()
		if titleEl.Length() > 0 {
			title = strings.TrimSpace(titleEl.Text())
			href, _ = titleEl.Attr("href")
			if title != "" && href != "" {
				break
			}
		}
	}

	// 如果还没找到，尝试从 a.resultLink 获取
	if title == "" || href == "" {
		linkEl := s.Find("a.resultLink").First()
		if linkEl.Length() > 0 {
			title = strings.TrimSpace(linkEl.Text())
			href, _ = linkEl.Attr("href")
		}
	}

	// 跳过没有标题或链接的结果
	if title == "" || href == "" {
		return nil
	}

	// 处理相对路径
	if !strings.HasPrefix(href, "http") {
		if strings.HasPrefix(href, "/") {
			href = "https://wap.sogou.com" + href
		} else if strings.HasPrefix(href, "./") {
			href = "https://wap.sogou.com/web/" + strings.TrimPrefix(href, "./")
		}
	}

	// 跳过内部链接
	if e.isInternalLink(href, title) {
		return nil
	}

	// 获取描述
	description := ""
	descSelectors := []string{
		".title-summary",
		".clamp2",
		".result-summary-exp",
		".video-desc__descContent_812e",
	}

	for _, selector := range descSelectors {
		descEl := s.Find(selector).First()
		if descEl.Length() > 0 {
			description = strings.TrimSpace(descEl.Text())
			if description != "" {
				break
			}
		}
	}

	// 获取来源
	source := ""
	sourceEl := s.Find(".citeurl span").First()
	if sourceEl.Length() > 0 {
		source = strings.TrimSpace(sourceEl.Text())
	}

	// 如果没有来源，尝试从 URL 提取
	if source == "" {
		// 尝试从 href 中提取真实 URL
		if realURL := e.extractRealURL(href); realURL != "" {
			if parsedURL, err := url.Parse(realURL); err == nil {
				source = parsedURL.Host
			}
		}
	}

	// 清理标题中的 em 标签残留
	title = strings.ReplaceAll(title, "<em>", "")
	title = strings.ReplaceAll(title, "</em>", "")

	// 清理描述中的 em 标签残留
	description = strings.ReplaceAll(description, "<em>", "")
	description = strings.ReplaceAll(description, "</em>", "")

	// 限制描述长度
	if len(description) > 500 {
		description = description[:500] + "..."
	}

	return &SearchResult{
		Title:       title,
		URL:         href,
		Description: description,
		Source:      source,
		Engine:      "sogou",
	}
}

// extractRealURL 从搜狗跳转链接中提取真实 URL
func (e *SogouEngine) extractRealURL(href string) string {
	// 搜狗的链接格式可能包含 url= 参数
	if u, err := url.Parse(href); err == nil {
		if realURL := u.Query().Get("url"); realURL != "" {
			if decoded, err := url.QueryUnescape(realURL); err == nil {
				return decoded
			}
			return realURL
		}
	}
	return href
}

// isInternalLink 判断是否为内部链接
func (e *SogouEngine) isInternalLink(href, title string) bool {
	// 过滤搜狗内部链接
	internalPatterns := []string{
		"sogou.com/web/searchList",
		"sogou.com/link?",
		"sogou.com/tx?",
		"sogou.com/v?",
		"antispider",
	}

	for _, pattern := range internalPatterns {
		if strings.Contains(href, pattern) {
			return true
		}
	}

	// 过滤广告
	adKeywords := []string{
		"广告",
		"推广",
	}

	for _, keyword := range adKeywords {
		if strings.Contains(title, keyword) {
			return true
		}
	}

	return false
}
