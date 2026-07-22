package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"commercial-transactions-service/internal/analyzer"
	"commercial-transactions-service/internal/crawler"
	"commercial-transactions-service/internal/exporter"
	"commercial-transactions-service/internal/generator"
)

func main() {
	targetURL := flag.String("url", "", "老系统首页 URL")
	cookie := flag.String("cookie", "", "登录后的 Cookie")
	browser := flag.String("browser", "edge", "浏览器: edge 或 chrome")
	duration := flag.Duration("duration", 5*time.Minute, "监控时长")
	headless := flag.Bool("headless", false, "无头模式")
	outputDir := flag.String("output", "./output", "输出目录")
	simple := flag.Bool("simple", false, "简单模式")
	fullsync := flag.Bool("fullsync", false, "全量同步模式：自动翻页拉全部数据")
	apiListFile := flag.String("api-list", "", "API 列表文件")
	flag.Parse()

	if *targetURL == "" && !*simple && !*fullsync {
		fmt.Println("用法:")
		fmt.Println("  crawl -url URL -cookie COOKIE -duration 5m         浏览器抓包")
		fmt.Println("  crawl -simple -api-list apis.txt -cookie COOKIE     单页抓取")
		fmt.Println("  crawl -fullsync -api-list apis.txt -cookie COOKIE   全量翻页抓取 ← 新功能")
		os.Exit(1)
	}

	log.SetFlags(log.Ltime)
	os.MkdirAll(*outputDir, 0755)

	var capturedAPIs []*exporter.CapturedAPI
	var analysisResults []*analyzer.AnalysisResult

	if *fullsync {
		capturedAPIs = runFullSync(*apiListFile, *cookie, *outputDir)
	} else if *simple {
		capturedAPIs = runSimpleMode(*apiListFile, *cookie)
	} else {
		capturedAPIs = runBrowserMode(*targetURL, *cookie, *browser, *headless, *duration)
	}

	if len(capturedAPIs) == 0 {
		log.Fatal("❌ 没有捕获到任何 API 请求")
	}

	log.Printf("📊 共捕获 %d 个 API 请求", len(capturedAPIs))

	// 分析 & 生成 DDL
	grouped := groupByEndpoint(capturedAPIs)
	log.Printf("📋 发现 %d 个独立接口", len(grouped))

	for endpoint, group := range grouped {
		var bodies [][]byte
		for _, api := range group {
			if len(api.Body) > 0 {
				bodies = append(bodies, api.Body)
			}
		}
		if len(bodies) == 0 {
			continue
		}
		result := analyzer.AnalyzeResponse(endpoint, group[0].Method, bodies)
		if len(result.Schemas) > 0 && len(result.Schemas[0].Columns) > 0 {
			analysisResults = append(analysisResults, result)
			log.Printf("   ✅ %s (%d列)", result.TableName, len(result.Schemas[0].Columns))
		}
	}

	if len(analysisResults) > 0 {
		ddl := generator.GenerateDDL(analysisResults)
		ddlFile := filepath.Join(*outputDir, "schema.sql")
		os.WriteFile(ddlFile, []byte(ddl), 0644)
		log.Printf("📄 DDL: %s", ddlFile)

		report := generator.GenerateReport(analysisResults)
		os.WriteFile(filepath.Join(*outputDir, "analysis_report.md"), []byte(report), 0644)
	}

	exportResult, _ := exporter.ExportAll(*outputDir, analysisResults, capturedAPIs)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║        🎉 抓取完成！                 ║")
	fmt.Printf("║  接口数: %-3d   请求数: %-4d      ║\n", len(grouped), len(capturedAPIs))
	fmt.Printf("║  推断表: %-3d                        ║\n", len(analysisResults))
	fmt.Println("╠══════════════════════════════════════╣")
	fmt.Printf("║  • %-32s ║\n", *outputDir+"/schema.sql")
	fmt.Printf("║  • %-32s ║\n", *outputDir+"/data/ (全量JSON)")
	fmt.Printf("║  • %-32s ║\n", exportResult.EndpointsFile)
	fmt.Println("╚══════════════════════════════════════╝")
}

// ─── 全量同步模式：自动翻页 ───

type apiResponse struct {
	Code  int             `json:"code"`
	Msg   string          `json:"msg"`
	Count int             `json:"count"`
	Data  json.RawMessage `json:"data"`
}

func runFullSync(apiListFile, cookie, outputDir string) []*exporter.CapturedAPI {
	data, err := os.ReadFile(apiListFile)
	if err != nil {
		log.Fatalf("无法读取 API 列表: %v", err)
	}

	var urls []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var results []*exporter.CapturedAPI
	dataDir := filepath.Join(outputDir, "data")
	os.MkdirAll(dataDir, 0755)

	for i, url := range urls {
		log.Printf("━━━ [%d/%d] %s ━━━", i+1, len(urls), url)

		// 判断是否需要翻页（select 类接口）
		if strings.Contains(url, "/select") || strings.Contains(url, "/list") {
			results = append(results, paginateAndFetch(client, url, cookie, dataDir)...)
		} else {
			// 单次请求
			result := singleFetch(client, url, "GET", cookie, dataDir)
			if result != nil {
				results = append(results, result)
			}
		}
	}

	return results
}

func paginateAndFetch(client *http.Client, baseURL, cookie, dataDir string) []*exporter.CapturedAPI {
	limit := 100
	var results []*exporter.CapturedAPI
	var allData []json.RawMessage

	// 第一页：探测总量
	firstURL := fmt.Sprintf("%s?page=1&limit=%d", baseURL, limit)
	firstResp := singleFetchWithBody(client, firstURL, cookie)

	if firstResp == nil {
		return results
	}

	// 尝试解析 count
	totalCount := 0
	var firstParsed apiResponse
	if err := json.Unmarshal(firstResp.Body, &firstParsed); err == nil {
		totalCount = firstParsed.Count

		// 合并第一页数据
		if len(firstParsed.Data) > 0 {
			var pageData []json.RawMessage
			if err := json.Unmarshal(firstParsed.Data, &pageData); err == nil {
				allData = append(allData, pageData...)
			}
		}
	}

	results = append(results, firstResp)

	if totalCount == 0 {
		log.Printf("   📄 无数据 (count=0)，跳过翻页")
		return results
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))
	log.Printf("   📊 总量=%d 每页=%d 总页数=%d", totalCount, limit, totalPages)

	if totalPages <= 1 {
		// 只有一页，直接保存合并数据
		saveMergedData(allData, baseURL, dataDir, totalCount)
		return results
	}

	// 翻页：从第 2 页开始
	for page := 2; page <= totalPages; page++ {
		pageURL := fmt.Sprintf("%s?page=%d&limit=%d", baseURL, page, limit)

		resp := singleFetchWithBody(client, pageURL, cookie)
		if resp == nil {
			log.Printf("   ⚠️ 第%d页请求失败，停止翻页", page)
			break
		}
		results = append(results, resp)

		var parsed apiResponse
		if err := json.Unmarshal(resp.Body, &parsed); err == nil && len(parsed.Data) > 0 {
			var pageData []json.RawMessage
			if err := json.Unmarshal(parsed.Data, &pageData); err == nil {
				allData = append(allData, pageData...)
			}
		}

		// 每 10 页输出一次进度
		if page%10 == 0 || page == totalPages {
			log.Printf("   📖 翻页中... %d/%d (%.0f%%)", page, totalPages, float64(page)/float64(totalPages)*100)
		}

		// 礼貌延迟，不冲垮老系统
		time.Sleep(100 * time.Millisecond)
	}

	// 保存合并后的全量数据
	saveMergedData(allData, baseURL, dataDir, totalCount)
	log.Printf("   ✅ 完成！共抓取 %d 条记录", len(allData))

	return results
}

func saveMergedData(allData []json.RawMessage, baseURL, dataDir string, totalCount int) {
	if len(allData) == 0 {
		return
	}

	// 构造完整的响应格式（模拟单页响应结构）
	wrapper := map[string]interface{}{
		"code":  0,
		"msg":   "ok",
		"count": totalCount,
		"data":  allData,
	}

	merged, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		log.Printf("   ⚠️ 合并数据序列化失败: %v", err)
		return
	}

	safeName := sanitizeFilename(baseURL)
	mergedFile := filepath.Join(dataDir, safeName+"_FULL.json")
	if err := os.WriteFile(mergedFile, merged, 0644); err != nil {
		log.Printf("   ⚠️ 保存失败: %v", err)
	} else {
		fileSize := float64(len(merged)) / 1024 / 1024
		log.Printf("   💾 全量数据已保存: %s (%.1f MB)", mergedFile, fileSize)
	}
}

func singleFetchWithBody(client *http.Client, url, cookie string) *exporter.CapturedAPI {
	return singleFetch(client, url, "GET", cookie, "")
}

func singleFetch(client *http.Client, url, method, cookie, _ string) *exporter.CapturedAPI {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Printf("   ⚠️ 创建请求失败: %v", err)
		return nil
	}

	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("   ⚠️ 请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("   ⚠️ 读取响应失败: %v", err)
		return nil
	}

	log.Printf("   [%d] %d bytes", resp.StatusCode, len(body))

	return &exporter.CapturedAPI{
		URL:        url,
		Method:     method,
		StatusCode: resp.StatusCode,
		Body:       body,
	}
}

// ─── 简单模式 ───

func runSimpleMode(apiListFile, cookie string) []*exporter.CapturedAPI {
	data, err := os.ReadFile(apiListFile)
	if err != nil {
		log.Fatalf("无法读取 API 列表: %v", err)
	}

	var urls []string
	for _, u := range strings.Split(string(data), "\n") {
		u = strings.TrimSpace(u)
		if u != "" && !strings.HasPrefix(u, "#") {
			urls = append(urls, u)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var results []*exporter.CapturedAPI
	for i, url := range urls {
		log.Printf("[%d/%d] %s", i+1, len(urls), url)
		result := singleFetch(client, url, "GET", cookie, "")
		if result != nil {
			results = append(results, result)
			safeName := sanitizeFilename(url)
			os.WriteFile(filepath.Join("./output/data", safeName+".json"), result.Body, 0644)
		}
	}
	return results
}

// ─── 浏览器模式 ───

func runBrowserMode(targetURL, cookie, browserType string, headless bool, duration time.Duration) []*exporter.CapturedAPI {
	c := crawler.New(targetURL)
	if cookie != "" {
		c.SetCookie(cookie)
	}
	if headless {
		c.Headless = true
	}
	if browserType == "edge" {
		if p := findBrowser("edge"); p != "" {
			c.SetBrowserPath(p)
		}
	} else if browserType == "chrome" {
		if p := findBrowser("chrome"); p != "" {
			c.SetBrowserPath(p)
		}
	}
	ctx := context.Background()
	if err := c.Run(ctx, duration); err != nil {
		log.Printf("❌ 爬虫运行失败: %v", err)
	}
	var result []*exporter.CapturedAPI
	for _, req := range c.GetResults() {
		result = append(result, &exporter.CapturedAPI{
			URL: req.URL, Method: req.Method,
			RequestBody: req.RequestBody,
			StatusCode:  req.StatusCode, Body: req.Body,
		})
	}
	return result
}

// ─── 工具函数 ───

func groupByEndpoint(apis []*exporter.CapturedAPI) map[string][]*exporter.CapturedAPI {
	groups := make(map[string][]*exporter.CapturedAPI)
	for _, api := range apis {
		if len(api.Body) == 0 {
			continue
		}
		normalized := normalizeURL(api.URL)
		groups[normalized] = append(groups[normalized], api)
	}
	return groups
}

func normalizeURL(rawURL string) string {
	if idx := strings.Index(rawURL, "?"); idx > 0 {
		base := rawURL[:idx]
		query := rawURL[idx+1:]
		var kept []string
		for _, pair := range strings.Split(query, "&") {
			kv := strings.SplitN(pair, "=", 2)
			key := strings.ToLower(kv[0])
			if key == "page" || key == "pagenum" || key == "page_num" ||
				key == "pagesize" || key == "size" || key == "limit" ||
				key == "offset" || key == "start" || key == "page_size" ||
				key == "current" || key == "currentpage" || key == "fresh" {
				continue
			}
			kept = append(kept, pair)
		}
		if len(kept) > 0 {
			return base + "?" + strings.Join(kept, "&")
		}
		return base
	}
	return rawURL
}

func sanitizeFilename(url string) string {
	s := url
	s = strings.ReplaceAll(s, "https://", "")
	s = strings.ReplaceAll(s, "http://", "")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "&", "_")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, ":", "_")
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

func findBrowser(browserType string) string {
	candidates := map[string][]string{
		"edge": {
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		},
		"chrome": {
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		},
	}
	for _, p := range candidates[browserType] {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
