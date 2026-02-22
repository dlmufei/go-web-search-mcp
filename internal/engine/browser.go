package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserManager 浏览器管理器（单例）
type BrowserManager struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	browserCtx  context.Context
	cancelFunc  context.CancelFunc
	mu          sync.Mutex
	initialized bool
	proxyURL    string
	headless    bool
}

var (
	browserManagerInstance *BrowserManager
	browserManagerOnce     sync.Once
)

// GetBrowserManager 获取浏览器管理器单例
func GetBrowserManager() *BrowserManager {
	browserManagerOnce.Do(func() {
		browserManagerInstance = &BrowserManager{
			headless: true,
		}
	})
	return browserManagerInstance
}

// findChromePath 查找 Chrome 可执行文件路径
func findChromePath() string {
	// 按优先级检查不同路径
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		}
	case "linux":
		paths = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}
	case "windows":
		paths = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			os.Getenv("LOCALAPPDATA") + `\Google\Chrome\Application\chrome.exe`,
		}
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			log.Printf("🔍 Found Chrome at: %s", p)
			return p
		}
	}

	return ""
}

// Initialize 初始化浏览器
func (bm *BrowserManager) Initialize(proxyURL string, headless bool) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.initialized {
		return nil
	}

	bm.proxyURL = proxyURL
	bm.headless = headless

	// 查找 Chrome 路径
	chromePath := findChromePath()
	if chromePath == "" {
		return fmt.Errorf("Chrome/Chromium not found. Please install Chrome browser")
	}

	// 配置 Chrome 选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// 指定 Chrome 路径
		chromedp.ExecPath(chromePath),

		// 基本配置
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-setuid-sandbox", true),

		// 模拟真实浏览器
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("useAutomationExtension", false),

		// 语言和窗口
		chromedp.Flag("lang", "en-US"),
		chromedp.WindowSize(1920, 1080),

		// User-Agent
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	// 如果配置了代理
	if proxyURL != "" {
		opts = append(opts, chromedp.ProxyServer(proxyURL))
		log.Printf("🌐 Browser using proxy: %s", proxyURL)
	}

	// 创建 allocator context
	bm.allocCtx, bm.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)

	// 创建 browser context
	bm.browserCtx, bm.cancelFunc = chromedp.NewContext(bm.allocCtx,
		chromedp.WithLogf(log.Printf),
	)

	// 启动浏览器（预热）
	if err := chromedp.Run(bm.browserCtx); err != nil {
		return fmt.Errorf("failed to start browser: %w", err)
	}

	bm.initialized = true
	log.Printf("✅ Browser initialized (headless=%v, path=%s)", headless, chromePath)
	return nil
}

// NewTabContext 创建新的标签页上下文
func (bm *BrowserManager) NewTabContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if !bm.initialized {
		// 如果未初始化，使用默认配置初始化
		if err := bm.Initialize("", true); err != nil {
			log.Printf("❌ Failed to initialize browser: %v", err)
			return context.Background(), func() {}
		}
	}

	// 创建新的 tab context
	tabCtx, tabCancel := chromedp.NewContext(bm.browserCtx)

	// 添加超时
	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, timeout)

	// 返回组合的 cancel 函数
	return timeoutCtx, func() {
		timeoutCancel()
		tabCancel()
	}
}

// Close 关闭浏览器
func (bm *BrowserManager) Close() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.cancelFunc != nil {
		bm.cancelFunc()
	}
	if bm.allocCancel != nil {
		bm.allocCancel()
	}
	bm.initialized = false
	log.Printf("🔴 Browser closed")
}

// IsInitialized 检查是否已初始化
func (bm *BrowserManager) IsInitialized() bool {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.initialized
}

// ExecuteWithRetry 带重试的执行
func (bm *BrowserManager) ExecuteWithRetry(ctx context.Context, maxRetries int, actions ...chromedp.Action) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := chromedp.Run(ctx, actions...); err != nil {
			lastErr = err
			log.Printf("⚠️ Browser action failed (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}
		return nil
	}
	return lastErr
}

// WaitAndClick 等待元素并点击
func WaitAndClick(selector string, timeout time.Duration) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.ByQuery),
			chromedp.Click(selector, chromedp.ByQuery),
		)
	}
}

// ScrollToBottom 滚动到页面底部
func ScrollToBottom() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil),
		)
	}
}

// GetPageSource 获取页面源码
func GetPageSource(html *string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		return chromedp.Run(ctx,
			chromedp.OuterHTML("html", html),
		)
	}
}
