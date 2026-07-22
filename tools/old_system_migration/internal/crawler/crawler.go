package crawler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/network"
)

// CapturedRequest 捕获的网络请求
type CapturedRequest struct {
	URL         string
	Method      string
	RequestBody string
	StatusCode  int
	Body        []byte
	ContentType string
	Timestamp   time.Time
	RequestID   network.RequestID // chromedp 的请求 ID，用于拿响应体
}

// Crawler 爬虫
type Crawler struct {
	TargetURL    string
	CookieString string // 可选：手动设置的 Cookie
	BrowserPath  string // Edge 或 Chrome 的路径
	Headless     bool

	requests   []*CapturedRequest
	reqByID    map[network.RequestID]*CapturedRequest
	mainCtx    context.Context // 主 chromedp context，用于异步拿 body
	mu         sync.Mutex
}

// New 创建爬虫实例
func New(targetURL string) *Crawler {
	return &Crawler{
		TargetURL: targetURL,
		Headless:  false,
		reqByID:   make(map[network.RequestID]*CapturedRequest),
	}
}

// SetCookie 设置 Cookie（跳过登录）
func (c *Crawler) SetCookie(cookieStr string) {
	c.CookieString = cookieStr
}

// SetBrowserPath 手动指定浏览器路径
func (c *Crawler) SetBrowserPath(path string) {
	c.BrowserPath = path
}

// GetResults 获取所有捕获的请求
func (c *Crawler) GetResults() []*CapturedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]*CapturedRequest, len(c.requests))
	copy(result, c.requests)
	return result
}

// Run 启动爬虫，拦截所有 API 请求
func (c *Crawler) Run(ctx context.Context, duration time.Duration) error {
	// 配置 chromedp 选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "TranslateUI"),
		chromedp.Flag("disable-extensions", true),
		chromedp.WindowSize(1400, 900),
	)

	if c.Headless {
		opts = append(opts,
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
		)
	}

	// 如果指定了浏览器路径
	if c.BrowserPath != "" {
		opts = append(opts, chromedp.ExecPath(c.BrowserPath))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	c.mainCtx = ctx // 保存主 context，供异步 body 抓取使用

	// 启用网络事件监听
	chromedp.ListenTarget(ctx, func(ev any) {
		c.handleNetworkEvent(ev, ctx)
	})

	// 注入 Cookie（如果有的话）
	var actions []chromedp.Action
	if c.CookieString != "" && c.TargetURL != "" {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			cookies := parseCookies(c.CookieString, c.TargetURL)
			for _, cookie := range cookies {
				err := network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithHTTPOnly(cookie.HTTPOnly).
					WithSecure(cookie.Secure).
					Do(ctx)
				if err != nil {
					log.Printf("设置 Cookie 失败: %v", err)
				}
			}
			return nil
		}))
	}

	// 导航到目标页面
	actions = append(actions, chromedp.Navigate(c.TargetURL))

	if err := chromedp.Run(ctx, actions...); err != nil {
		return fmt.Errorf("导航失败: %w", err)
	}

	log.Printf("✅ 浏览器已启动，正在监控网络请求...")
	log.Printf("ℹ️  请在浏览器中操作老系统：翻页、点击、浏览各个功能页面")
	log.Printf("⏰ 监控时长: %v (或按 Ctrl+C 提前结束)", duration)

	// 如果没设置 Cookie，给用户时间手动登录
	if c.CookieString == "" {
		log.Printf("🔐 未设置 Cookie，请在浏览器中手动登录")
	}

	// 等待指定时长，期间所有网络请求都会被拦截
	timer := time.NewTimer(duration)
	select {
	case <-timer.C:
		log.Printf("⏰ 监控时间到，正在生成报告...")
	case <-ctx.Done():
		log.Printf("⚠️ 上下文取消，正在生成报告...")
	}

	return nil
}

// handleNetworkEvent 处理 chromedp 网络事件
func (c *Crawler) handleNetworkEvent(ev any, ctx context.Context) {
	switch ev := ev.(type) {
	case *network.EventRequestWillBeSent:
		c.onRequest(ev)
	case *network.EventResponseReceived:
		c.onResponse(ev, ctx)
	case *network.EventLoadingFinished:
		c.onLoadingFinished(ev, ctx)
	}
}

func (c *Crawler) onRequest(ev *network.EventRequestWillBeSent) {
	// 过滤静态资源
	if isStaticResource(ev.Request.URL) {
		return
	}

	req := &CapturedRequest{
		URL:       ev.Request.URL,
		Method:    ev.Request.Method,
		Timestamp: time.Now(),
		RequestID: ev.RequestID,
	}
	if ev.Request.HasPostData {
		for _, entry := range ev.Request.PostDataEntries {
			req.RequestBody += entry.Bytes
		}
	}

	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.reqByID[ev.RequestID] = req
	c.mu.Unlock()

	log.Printf("📤 [%s] %s", req.Method, truncate(req.URL, 120))
}

func (c *Crawler) onResponse(ev *network.EventResponseReceived, ctx context.Context) {
	c.mu.Lock()
	if req, ok := c.reqByID[ev.RequestID]; ok {
		req.StatusCode = int(ev.Response.Status)
		req.ContentType = ev.Response.MimeType
	}
	c.mu.Unlock()
}

func (c *Crawler) onLoadingFinished(ev *network.EventLoadingFinished, ctx context.Context) {
	c.mu.Lock()
	req, ok := c.reqByID[ev.RequestID]
	c.mu.Unlock()
	if !ok {
		return
	}

	// 只抓 API 响应体
	if !isAPICall(req.URL, req.ContentType) {
		return
	}

	// 使用主 context（不会因为回调结束而过期）
	mainCtx := c.mainCtx
	requestID := ev.RequestID
	go func() {
		body, err := network.GetResponseBody(requestID).Do(mainCtx)
		if err != nil {
			log.Printf("⚠️ 获取body失败 [%s]: %v", truncate(req.URL, 60), err)
			return
		}
		c.mu.Lock()
		req.Body = []byte(body)
		c.mu.Unlock()
		log.Printf("📥 [%d] %s (%d bytes)", req.StatusCode, truncate(req.URL, 70), len(body))
	}()
}

// ─── 辅助函数 ───

// isStaticResource 判断是否为静态资源（需要过滤掉）
func isStaticResource(url string) bool {
	staticExts := []string{
		".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot", ".webp", ".mp4", ".mp3", ".avi",
		".map", ".json.map", ".js.map",
	}
	urlLower := strings.ToLower(url)
	for _, ext := range staticExts {
		if strings.Contains(urlLower, ext) {
			return true
		}
	}
	// 常见静态域名/CDN
	staticDomains := []string{
		"cdn.", "static.", "assets.", "images.", "fonts.",
		"google-analytics", "googletagmanager", "baidu.com/hm.js",
	}
	for _, d := range staticDomains {
		if strings.Contains(urlLower, d) {
			return true
		}
	}
	return false
}

// isAPICall 判断是否是 API 调用
func isAPICall(url, mimeType string) bool {
	urlLower := strings.ToLower(url)

	// 1. 返回 JSON/XML 的一定是 API
	if strings.Contains(mimeType, "application/json") ||
		strings.Contains(mimeType, "text/json") ||
		strings.Contains(mimeType, "application/xml") {
		return true
	}

	// 2. 常见 API 路径模式
	apiPatterns := []string{"/api/", "/apis/", "/v1/", "/v2/", "/v3/", "/rest/",
		"/ajax/", "/graphql", "/rpc/", "/gateway/", "/app/admin/", "/admin/"}
	for _, p := range apiPatterns {
		if strings.Contains(urlLower, p) {
			return true
		}
	}

	// 3. 动态路径关键词（PHP/Java 等常见命名）
	apiKeywords := []string{"select", "list", "page", "query", "search", "get",
		"index", "info", "permission", "create", "update", "delete", "save",
		"edit", "add", "detail", "config", "rule", "account", "user", "order",
		"product", "goods", "coupon", "contract", "wallet", "stat", "export"}
	for _, kw := range apiKeywords {
		if strings.Contains(urlLower, "/"+kw) || strings.Contains(urlLower, "/"+kw+"/") {
			return true
		}
	}

	// 4. URL 不含文件扩展名，可能是 API
	lastSlash := strings.LastIndex(url, "/")
	if lastSlash > 0 {
		lastPart := url[lastSlash+1:]
		if len(lastPart) > 0 && !strings.Contains(lastPart, ".") {
			// 有可能是 API 路径
			return true
		}
	}

	// 5. 带 query 参数且无扩展名
	if strings.Contains(url, "?") && !strings.Contains(url, ".php?") && !strings.Contains(url, ".html?") {
		return true
	}

	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// CookiePair 简单的 Cookie 键值对
type CookiePair struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	HTTPOnly bool
	Secure   bool
}

func parseCookies(cookieStr, targetURL string) []CookiePair {
	var cookies []CookiePair
	// 从 URL 提取域名
	domain := ""
	if strings.Contains(targetURL, "://") {
		parts := strings.SplitN(targetURL, "://", 2)
		domainPart := strings.SplitN(parts[1], "/", 2)[0]
		domainPart = strings.SplitN(domainPart, ":", 2)[0]
		domain = domainPart
	}

	for _, pair := range strings.Split(cookieStr, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			cookies = append(cookies, CookiePair{
				Name:   strings.TrimSpace(kv[0]),
				Value:  strings.TrimSpace(kv[1]),
				Domain: domain,
				Path:   "/",
			})
		}
	}
	return cookies
}

// ─── 备用方案：直接 HTTP 爬虫（不需要浏览器） ───

// SimpleCrawl 用 HTTP 客户端直接调接口（需要用户提供 Cookie 和具体的 API URL 列表）
func SimpleCrawl(apiURLs []string, cookie string) []*CapturedRequest {
	var requests []*CapturedRequest

	for _, url := range apiURLs {
		log.Printf("📤 请求: %s", truncate(url, 100))
		// 实际请求在 main.go 中处理，这里只记录 URL
		requests = append(requests, &CapturedRequest{
			URL:       url,
			Method:    "GET",
			Timestamp: time.Now(),
		})
	}

	return requests
}
