package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	base := "http://localhost:8080/api/v1/front"
	merchID := 212793

	db, _ := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true&loc=Local")
	rows, _ := db.Query("SELECT id, username FROM users WHERE id != 94694 AND status=1")
	defer rows.Close()

	type userInfo struct{ id int; username string }
	var users []userInfo
	for rows.Next() {
		var u userInfo; rows.Scan(&u.id, &u.username); users = append(users, u)
	}
	fmt.Printf("%d 用户并发抢购 %d\n", len(users), merchID)

	var wg sync.WaitGroup
	client := &http.Client{}
	var mu sync.Mutex
	success, fail, loginFail := 0, 0, 0

	for _, u := range users {
		wg.Add(1)
		go func(u userInfo) {
			defer wg.Done()
			resp, err := client.Post(base+"/auth/login", "application/json",
				bytes.NewBufferString(fmt.Sprintf(`{"username":"%s","password":"123456"}`, u.username)))
			if err != nil || resp == nil { mu.Lock(); loginFail++; mu.Unlock(); return }
			var l struct{ Data struct{ Token string } }
			b, _ := io.ReadAll(resp.Body); resp.Body.Close(); json.Unmarshal(b, &l)
			if l.Data.Token == "" { mu.Lock(); loginFail++; mu.Unlock(); return }

			req, _ := http.NewRequest("POST", fmt.Sprintf("%s/merchandises/%d/buy", base, merchID),
				bytes.NewBufferString(`{"consignee":"t","phone":"1"}`))
			req.Header.Set("Authorization", "Bearer "+l.Data.Token)
			req.Header.Set("Content-Type", "application/json")
			r2, err := client.Do(req)
			if err != nil || r2 == nil { mu.Lock(); fail++; mu.Unlock(); return }
			b2, _ := io.ReadAll(r2.Body); r2.Body.Close()

			mu.Lock()
			if bytes.Contains(b2, []byte("购买成功")) { success++ } else { fail++ }
			mu.Unlock()
		}(u)
	}
	wg.Wait()
	fmt.Printf("成功: %d / 失败: %d / 登录失败: %d\n", success, fail, loginFail)
}
