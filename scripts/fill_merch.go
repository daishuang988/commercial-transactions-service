package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true&loc=Local"
	}
	db, _ := sql.Open("mysql", dsn)
	defer db.Close()

	// 读取老系统商品数据
	dataDir := "./tools/old_system_migration/output_new/data"
	matches, _ := filepath.Glob(filepath.Join(dataDir, "*merchandise_select_FULL.json"))
	if len(matches) == 0 {
		fmt.Println("未找到商品数据文件")
		return
	}
	raw, _ := os.ReadFile(matches[0])
	var w struct{ Data json.RawMessage }
	json.Unmarshal(raw, &w)
	var recs []map[string]interface{}
	json.Unmarshal(w.Data, &recs)
	fmt.Printf("老系统商品总数: %d\n", len(recs))

	// 获取缺失的商品ID列表
	rows, _ := db.Query("SELECT DISTINCT o.merchandise_id FROM orders o WHERE o.merchandise_id NOT IN (SELECT id FROM merchandises)")
	missing := make(map[int64]bool)
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		missing[id] = true
	}
	rows.Close()
	fmt.Printf("缺失商品ID数: %d\n", len(missing))

	// 禁FK，插入
	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS=1")

	st, _ := db.Prepare(`INSERT IGNORE INTO merchandises(id,old_id,user_id,title,image,price,is_show,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	count := 0
	for _, r := range recs {
		id := int64(r["id"].(float64))
		if !missing[id] { continue }

		uid := int64(r["user_id"].(float64))
		st.Exec(id, nil, uid,
			s(r["title"]), s(r["image"]), f(r["price"]),
			int64(r["is_show"].(float64)), int64(r["status"].(float64)),
			nd(r["created_at"]), nd(r["updated_at"]))
		count++
	}
	fmt.Printf("补充商品: %d\n", count)

	// 验证
	var remaining int64
	db.QueryRow("SELECT COUNT(DISTINCT merchandise_id) FROM orders WHERE merchandise_id NOT IN (SELECT id FROM merchandises)").Scan(&remaining)
	fmt.Printf("剩余缺失: %d\n", remaining)
}

func s(v interface{}) string {
	if v == nil { return "" }
	switch val := v.(type) {
	case string: return val
	case float64: return fmt.Sprintf("%.0f", val)
	}
	return fmt.Sprintf("%v", v)
}

func f(v interface{}) float64 {
	if v == nil { return 0 }
	switch val := v.(type) {
	case float64: return val
	case string:
		var x float64; fmt.Sscanf(val, "%f", &x); return x
	}
	return 0
}

func nd(v interface{}) interface{} {
	if v == nil { return nil }
	str := s(v)
	if str == "" || str == "null" || str == "0000-00-00 00:00:00" { return nil }
	return str
}
