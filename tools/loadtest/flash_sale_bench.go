package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	ok     int64
	fail   int64
	total  int64
	latencies []int64
	mu     sync.Mutex
}

var (
	concurrency = flag.Int("c", 100, "并发数")
	duration    = flag.Int("d", 10, "持续时间(秒)")
	baseURL     = flag.String("url", "http://localhost:8080", "服务地址")
	mobile      = flag.String("mobile", "13701142651", "测试手机号")
	password    = flag.String("password", "666666", "测试密码")
	productID   = flag.Int64("product", 1, "抢购商品ID")
)

func main() {
	flag.Parse()

	// 登录获取 Token
	token := login()
	if token == "" {
		fmt.Println("❌ 登录失败，无法执行压测")
		return
	}
	fmt.Printf("✅ 登录成功 (并发=%d 持续=%ds)\n\n", *concurrency, *duration)

	// 压测三个接口
	fmt.Println("═══════════════════════════════════════════")
	bench("GET  /flash-sale/time", *duration, func() (*http.Request, error) {
		req, _ := http.NewRequest("GET", *baseURL+"/api/v1/front/flash-sale/time", nil)
		return req, nil
	})

	bench("GET  /flash-sale/products", *duration, func() (*http.Request, error) {
		req, _ := http.NewRequest("GET", *baseURL+"/api/v1/front/flash-sale/products", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		return req, nil
	})

	bench("POST /flash-sale/buy", *duration, func() (*http.Request, error) {
		body := fmt.Sprintf(`{"product_id":%d}`, *productID)
		req, _ := http.NewRequest("POST", *baseURL+"/api/v1/front/flash-sale/buy",
			bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		return req, nil
	})
}

func login() string {
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, *mobile, *password)
	resp, err := http.Post(*baseURL+"/api/v1/front/auth/login", "application/json",
		bytes.NewBufferString(body))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var r struct {
		Data struct{ Token string `json:"token"` } `json:"data"`
	}
	json.Unmarshal(b, &r)
	return r.Data.Token
}

func bench(name string, dur int, makeReq func() (*http.Request, error)) {
	var r result
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 启动并发 worker
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			for {
				select {
				case <-stop:
					return
				default:
					req, err := makeReq()
					if err != nil {
						atomic.AddInt64(&r.fail, 1)
						atomic.AddInt64(&r.total, 1)
						continue
					}
					start := time.Now()
					resp, err := client.Do(req)
					elapsed := time.Since(start).Microseconds()
					atomic.AddInt64(&r.total, 1)

					r.mu.Lock()
					r.latencies = append(r.latencies, elapsed)
					if len(r.latencies) > 10000 {
						r.latencies = r.latencies[:10000]
					}
					r.mu.Unlock()

					if err != nil {
						atomic.AddInt64(&r.fail, 1)
					} else {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						if resp.StatusCode == 200 {
							atomic.AddInt64(&r.ok, 1)
						} else {
							atomic.AddInt64(&r.fail, 1)
						}
					}
				}
			}
		}()
	}

	time.Sleep(time.Duration(dur) * time.Second)
	close(stop)
	wg.Wait()

	// 统计
	ok := atomic.LoadInt64(&r.ok)
	fail := atomic.LoadInt64(&r.fail)
	total := atomic.LoadInt64(&r.total)
	qps := float64(total) / float64(dur)

	r.mu.Lock()
	lats := make([]int64, len(r.latencies))
	copy(lats, r.latencies)
	r.mu.Unlock()
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })

	p50 := int64(0)
	p99 := int64(0)
	if len(lats) > 0 {
		p50 = lats[len(lats)*50/100]
		p99 = lats[len(lats)*99/100]
	}

	fmt.Printf("%s\n", name)
	fmt.Printf("  总请求: %d  |  成功: %d  |  失败: %d\n", total, ok, fail)
	fmt.Printf("  QPS: %.0f  |  P50: %dμs  |  P99: %dμs\n", qps, p50, p99)
	fmt.Printf("  成功率: %.1f%%\n", float64(ok)/float64(total)*100)
	fmt.Println(strings.Repeat("─", 43))
}
