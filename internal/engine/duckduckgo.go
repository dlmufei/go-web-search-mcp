package engine

import (
	"context"
	"encoding/json"
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

// DuckDuckGoEngine DuckDuckGo 搜索引擎实现
type DuckDuckGoEngine struct {
	client   *http.Client
	proxyURL string
}

// NewDuckDuckGoEngine 创建 DuckDuckGo 搜索引擎实例
func NewDuckDuckGoEngine(proxyURL string) *DuckDuckGoEngine {
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

	return &DuckDuckGoEngine{
		client:   client,
		proxyURL: proxyURL,
	}
}

// Name 返回引擎名称
func (e *DuckDuckGoEngine) Name() string {
	return "duckduckgo"
}

// Search 执行 DuckDuckGo 搜索
func (e *DuckDuckGoEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// DuckDuckGo HTML 版本
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

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

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	results := e.parseResults(doc, limit)
	log.Printf("🔍 DuckDuckGo: found %d results for query '%s'", len(results), query)

	return results, nil
}

// setHeaders 设置请求头
func (e *DuckDuckGoEngine) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
}

// parseResults 解析搜索结果
func (e *DuckDuckGoEngine) parseResults(doc *goquery.Document, limit int) []SearchResult {
	var results []SearchResult

	// DuckDuckGo HTML 版本的结果在 .result 类中
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= limit {
			return
		}

		// 获取标题和链接
		titleEl := s.Find(".result__title")
		linkEl := s.Find(".result__a")

		if titleEl.Length() == 0 || linkEl.Length() == 0 {
			return
		}

		href, exists := linkEl.Attr("href")
		if !exists {
			return
		}

		// DuckDuckGo 的链接可能是相对的或者经过编码的
		if strings.HasPrefix(href, "//duckduckgo.com/l/") {
			// 解析重定向 URL
			if parsed, err := url.Parse("https:" + href); err == nil {
				href = parsed.Query().Get("uddg")
			}
		}

		if href == "" || !strings.HasPrefix(href, "http") {
			return
		}

		// 获取描述
		description := ""
		descEl := s.Find(".result__snippet")
		if descEl.Length() > 0 {
			description = strings.TrimSpace(descEl.Text())
		}

		// 获取来源
		source := ""
		sourceEl := s.Find(".result__url")
		if sourceEl.Length() > 0 {
			source = strings.TrimSpace(sourceEl.Text())
		}

		results = append(results, SearchResult{
			Title:       strings.TrimSpace(titleEl.Text()),
			URL:         href,
			Description: description,
			Source:      source,
			Engine:      "duckduckgo",
		})
	})

	return results
}

// DuckDuckGoInstantAnswer 使用 DuckDuckGo Instant Answer API（备用方案）
type DuckDuckGoInstantAnswer struct {
	Abstract       string `json:"Abstract"`
	AbstractText   string `json:"AbstractText"`
	AbstractSource string `json:"AbstractSource"`
	AbstractURL    string `json:"AbstractURL"`
	RelatedTopics  []struct {
		FirstURL string `json:"FirstURL"`
		Text     string `json:"Text"`
	} `json:"RelatedTopics"`
}

// SearchInstantAnswer 使用 Instant Answer API（备用）
func (e *DuckDuckGoEngine) SearchInstantAnswer(ctx context.Context, query string) (*DuckDuckGoInstantAnswer, error) {
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var answer DuckDuckGoInstantAnswer
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return nil, err
	}

	return &answer, nil
}
