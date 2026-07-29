package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	cookie := flag.String("cookie", "", "老系统 Cookie (PHPSID=xxx)")
	baseURL := flag.String("base", "https://www.srdsmgs.com", "老系统域名")
	outputDir := flag.String("output", "./output/downloaded_images", "图片输出目录")
	flag.Parse()

	if *cookie == "" {
		log.Fatal("请提供 -cookie")
	}

	images := loadImagePaths()
	fmt.Printf("共 %d 张独立图片\n", len(images))
	os.MkdirAll(*outputDir, 0755)

	// 带 Cookie 的 HTTP 客户端
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 先访问一次首页让 Cookie 生效
	req, _ := http.NewRequest("GET", *baseURL, nil)
	req.Header.Set("Cookie", *cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		fmt.Printf("首页状态: %d\n", resp.StatusCode)
	}

	success := 0
	for i, img := range images {
		imgURL := *baseURL + img.Path
		localPath := filepath.Join(*outputDir, filepath.FromSlash(img.Path))
		os.MkdirAll(filepath.Dir(localPath), 0755)

		if fi, err := os.Stat(localPath); err == nil && fi.Size() > 100 {
			fmt.Printf("[%d/%d] ✓ %s (已存在)\n", i+1, len(images), img.Path)
			success++
			continue
		}

		fmt.Printf("[%d/%d] %s → ", i+1, len(images), img.Path)

		req, _ := http.NewRequest("GET", imgURL, nil)
		req.Header.Set("Cookie", *cookie)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Referer", *baseURL+"/")
		req.Header.Set("Sec-Fetch-Dest", "image")
		req.Header.Set("Sec-Fetch-Mode", "no-cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("请求失败: %v\n", err)
			continue
		}

		if resp.StatusCode != 200 {
			fmt.Printf("HTTP %d\n", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "image/") {
			fmt.Printf("非图片 (%s, %d bytes)\n", ct, len(data))
			continue
		}

		os.WriteFile(localPath, data, 0644)
		fmt.Printf("OK %d bytes\n", len(data))
		success++

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("\n完成: %d/%d 成功\n", success, len(images))
}

type ImageInfo struct {
	Path string
}

func loadImagePaths() []ImageInfo {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true&loc=Local"
	}
	db, _ := sql.Open("mysql", dsn)
	defer db.Close()

	var images []ImageInfo
	seen := map[string]bool{}

	for _, q := range []string{
		"SELECT DISTINCT image FROM merchandises WHERE image != '' AND image != '/app/admin/avatar.png'",
		"SELECT DISTINCT images FROM goods WHERE images != ''",
		"SELECT DISTINCT avatar FROM users WHERE avatar != '' AND avatar != '/assets/img/avatar.png'",
	} {
		rows, _ := db.Query(q)
		for rows.Next() {
			var p string
			rows.Scan(&p)
			if !seen[p] { seen[p] = true; images = append(images, ImageInfo{Path: p}) }
		}
		rows.Close()
	}
	return images
}
