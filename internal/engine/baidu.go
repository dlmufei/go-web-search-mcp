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

// BaiduEngine 百度搜索引擎实现
type BaiduEngine struct {
	client   *http.Client
	proxyURL string
}

// NewBaiduEngine 创建百度搜索引擎实例
func NewBaiduEngine(proxyURL string) *BaiduEngine {
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

	return &BaiduEngine{
		client:   client,
		proxyURL: proxyURL,
	}
}

// Name 返回引擎名称
func (e *BaiduEngine) Name() string {
	return "baidu"
}

// Search 执行百度搜索
func (e *BaiduEngine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// 首先访问百度主页获取 cookie
	if err := e.warmup(ctx); err != nil {
		log.Printf("⚠️ Baidu warmup failed: %v", err)
	}

	var allResults []SearchResult
	pn := 0

	for len(allResults) < limit {
		results, err := e.searchPage(ctx, query, pn)
		if err != nil {
			// 检查是否是验证码限制错误
			if strings.Contains(err.Error(), "captcha") || strings.Contains(err.Error(), "rate limited") {
				if len(allResults) > 0 {
					log.Printf("⚠️ Baidu: Rate limited, returning %d results collected so far", len(allResults))
					break
				}
				return nil, fmt.Errorf("baidu rate limited: %w", err)
			}
			if len(allResults) > 0 {
				break
			}
			return nil, err
		}

		if len(results) == 0 {
			log.Printf("⚠️ Baidu: No more results at page %d, ending early", pn/10)
			break
		}

		allResults = append(allResults, results...)
		pn += 10 // 百度每页10条结果

		// 限制最多搜索5页
		if pn > 40 {
			break
		}

		// 添加延迟避免触发限制
		if pn < 40 && len(allResults) < limit {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults, nil
}

// warmup 访问百度主页获取初始 cookie
func (e *BaiduEngine) warmup(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.baidu.com/", nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	log.Printf("🔍 Baidu warmup completed, cookies established")
	return nil
}

// searchPage 搜索单页结果
func (e *BaiduEngine) searchPage(ctx context.Context, query string, pn int) ([]SearchResult, error) {
	// 构建请求 URL - 使用更完整的参数来模拟真实浏览器请求
	params := url.Values{}
	params.Set("wd", query)
	params.Set("pn", fmt.Sprintf("%d", pn))
	params.Set("ie", "utf-8")
	params.Set("mod", "1")
	params.Set("isbd", "1")
	params.Set("isid", "f7ba1776007bcf9e")
	params.Set("oq", query)
	params.Set("tn", "88093251_62_hao_pg")
	params.Set("usm", "1")
	params.Set("fenlei", "256")
	params.Set("rsv_idx", "1")
	params.Set("rsv_pq", "f7ba1776007bcf9e")
	params.Set("rsv_t", "8179fxGiNMUh/0dXHrLsJXPlKYbkj9S5QH6rOLHY6pG6OGQ81YqzRTIGjjeMwEfiYQTSiTQIhCJj")
	params.Set("bs", query)
	params.Set("_ss", "1")
	params.Set("f4s", "1")
	params.Set("csor", "5")
	params.Set("_cr1", "30385")

	searchURL := fmt.Sprintf("https://www.baidu.com/s?%s", params.Encode())

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
	log.Printf("🔍 Baidu response size: %d bytes", len(body))

	// 检查是否被重定向到验证码页面
	if strings.Contains(bodyStr, "wappass.baidu.com") || 
		strings.Contains(bodyStr, "captcha") || 
		strings.Contains(bodyStr, "百度安全验证") ||
		strings.Contains(bodyStr, "安全验证") {
		log.Printf("⚠️ Baidu: Detected captcha/verification page, trying mobile approach")
		mobileResults, err := e.searchPageMobile(ctx, query, pn)
		if err != nil {
			return nil, fmt.Errorf("baidu rate limited/captcha required: %w", err)
		}
		if len(mobileResults) == 0 {
			return nil, fmt.Errorf("baidu rate limited: captcha required, please try again later or use a proxy")
		}
		return mobileResults, nil
	}

	// 检查是否有 content_left 容器
	if !strings.Contains(bodyStr, "content_left") {
		log.Printf("⚠️ Baidu: No content_left found in response, response preview: %s", bodyStr[:min(len(bodyStr), 500)])
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	results := e.parseResults(doc)
	log.Printf("🔍 Baidu page %d: found %d results", pn/10, len(results))

	return results, nil
}

// setHeaders 设置请求头
func (e *BaiduEngine) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Cookie", "BAIDUID=auto; BIDUPSID=auto") // 基础 cookie
}

// parseResults 解析搜索结果
func (e *BaiduEngine) parseResults(doc *goquery.Document) []SearchResult {
	var results []SearchResult

	// 百度搜索结果在 #content_left 容器中
	doc.Find("#content_left").Children().Each(func(i int, s *goquery.Selection) {
		result := e.parseResultItem(s)
		if result != nil {
			results = append(results, *result)
		}
	})

	return results
}

// parseResultItem 解析单个搜索结果项
func (e *BaiduEngine) parseResultItem(s *goquery.Selection) *SearchResult {
	// 查找标题元素 - 百度的标题通常在 h3
	titleEl := s.Find("h3")
	if titleEl.Length() == 0 {
		return nil
	}

	title := strings.TrimSpace(titleEl.Text())
	if title == "" {
		return nil
	}

	// 获取链接 - 从 h3 内的 a 标签或直接从第一个 a 标签获取
	var href string
	linkEl := titleEl.Find("a").First()
	if linkEl.Length() == 0 {
		linkEl = s.Find("a").First()
	}
	if linkEl.Length() > 0 {
		href, _ = linkEl.Attr("href")
	}

	// 跳过没有链接或非 http 链接的结果
	if href == "" || !strings.HasPrefix(href, "http") {
		return nil
	}

	// 跳过百度内部链接（广告、相关搜索等）
	if e.isInternalLink(href, title) {
		return nil
	}

	// 获取描述 - 优先使用 aria-label 属性
	description := ""
	// 尝试从 .c-font-normal.c-color-text 获取 aria-label
	descEl := s.Find(".c-font-normal.c-color-text").First()
	if descEl.Length() > 0 {
		if ariaLabel, exists := descEl.Attr("aria-label"); exists && ariaLabel != "" {
			description = strings.TrimSpace(ariaLabel)
		}
	}

	// 备选：从 .cos-row 获取
	if description == "" {
		cosRow := s.Find(".cos-row").First()
		if cosRow.Length() > 0 {
			description = strings.TrimSpace(cosRow.Text())
		}
	}

	// 备选：从 .c-abstract 获取
	if description == "" {
		abstractEl := s.Find(".c-abstract").First()
		if abstractEl.Length() > 0 {
			description = strings.TrimSpace(abstractEl.Text())
		}
	}

	// 获取来源
	source := ""
	sourceEl := s.Find(".cosc-source").First()
	if sourceEl.Length() > 0 {
		source = strings.TrimSpace(sourceEl.Text())
	}

	// 如果没有来源，从 URL 提取域名
	if source == "" {
		if parsedURL, err := url.Parse(href); err == nil {
			source = parsedURL.Host
		}
	}

	// 限制描述长度
	if len(description) > 500 {
		description = description[:500] + "..."
	}

	return &SearchResult{
		Title:       title,
		URL:         href,
		Description: description,
		Source:      source,
		Engine:      "baidu",
	}
}

// isInternalLink 判断是否为百度内部链接
func (e *BaiduEngine) isInternalLink(href, title string) bool {
	// 过滤百度内部链接
	internalDomains := []string{
		"baidu.com/s?",        // 相关搜索
		"baidu.com/baidu.php", // 广告链接
		"baidu.com/link?",     // 跳转链接（这个是正常的，不过滤）
		"tieba.baidu.com",     // 贴吧（可选保留）
		"zhidao.baidu.com",    // 知道（可选保留）
		"baike.baidu.com",     // 百科（可选保留）
	}

	// 只过滤广告和相关搜索链接
	for _, domain := range internalDomains[:2] {
		if strings.Contains(href, domain) {
			return true
		}
	}

	// 过滤明显的广告标题
	adKeywords := []string{
		"广告",
		"推广",
		"想在此推广",
	}

	for _, keyword := range adKeywords {
		if strings.Contains(title, keyword) {
			return true
		}
	}

	return false
}

// searchPageMobile 使用移动端页面搜索（备选方案）
func (e *BaiduEngine) searchPageMobile(ctx context.Context, query string, pn int) ([]SearchResult, error) {
	// 使用移动端 URL
	params := url.Values{}
	params.Set("word", query)
	params.Set("pn", fmt.Sprintf("%d", pn))

	searchURL := fmt.Sprintf("https://m.baidu.com/s?%s", params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create mobile request failed: %w", err)
	}

	// 设置移动端请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mobile request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read mobile body failed: %w", err)
	}

	bodyStr := string(body)
	log.Printf("🔍 Baidu mobile response size: %d bytes", len(body))

	// 检查移动端是否也触发了验证码
	if strings.Contains(bodyStr, "wappass.baidu.com") ||
		strings.Contains(bodyStr, "captcha") ||
		strings.Contains(bodyStr, "安全验证") {
		return nil, fmt.Errorf("mobile captcha required")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("parse mobile HTML failed: %w", err)
	}

	return e.parseMobileResults(doc), nil
}

// parseMobileResults 解析移动端搜索结果
func (e *BaiduEngine) parseMobileResults(doc *goquery.Document) []SearchResult {
	var results []SearchResult

	// 移动端结果选择器
	doc.Find(".c-result, .result, [data-log]").Each(func(i int, s *goquery.Selection) {
		// 获取标题
		var title string
		titleEl := s.Find(".c-title, .c-title-text, h3").First()
		if titleEl.Length() > 0 {
			title = strings.TrimSpace(titleEl.Text())
		}

		if title == "" {
			return
		}

		// 获取链接
		var href string
		linkEl := s.Find("a").First()
		if linkEl.Length() > 0 {
			href, _ = linkEl.Attr("href")
		}

		// 如果 href 是相对路径，补全
		if href != "" && !strings.HasPrefix(href, "http") {
			if strings.HasPrefix(href, "/") {
				href = "https://m.baidu.com" + href
			}
		}

		if href == "" || !strings.HasPrefix(href, "http") {
			return
		}

		// 跳过广告
		if e.isInternalLink(href, title) {
			return
		}

		// 获取描述
		description := ""
		descEl := s.Find(".c-abstract, .c-span-last, .c-line-clamp2").First()
		if descEl.Length() > 0 {
			description = strings.TrimSpace(descEl.Text())
		}

		// 获取来源
		source := ""
		sourceEl := s.Find(".c-showurl, .c-color-source").First()
		if sourceEl.Length() > 0 {
			source = strings.TrimSpace(sourceEl.Text())
		}
		if source == "" {
			if parsedURL, err := url.Parse(href); err == nil {
				source = parsedURL.Host
			}
		}

		results = append(results, SearchResult{
			Title:       title,
			URL:         href,
			Description: description,
			Source:      source,
			Engine:      "baidu",
		})
	})

	return results
}
