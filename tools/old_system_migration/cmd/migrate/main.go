package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var dataDir = "./output/data"
var dsn = os.Getenv("MYSQL_DSN")
func init() {
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true&loc=Local"
	}
}

func main() {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// 执行 DDL
	log.Println("━━━ 创建表结构 ━━━")
	ddl, _ := os.ReadFile("./output/schema.sql")
	for _, stmt := range strings.Split(string(ddl), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			log.Printf("⚠️ DDL跳过: %v", err)
		}
	}
	log.Println("✅ 表结构创建完成")

	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS=1")

	// 按依赖顺序迁移
	migrateUsers(db)
	migrateMerchandises(db)
	migrateOrders(db)
	migrateWithdraws(db)
	migrateCouponLogs(db)
	migrateSelfBonusLogs(db)
	migrateShareBonusLogs(db)
	migrateGoods(db)
	migrateCategories(db)
	migrateAdminUsers(db)
	migrateRoles(db)
	migrateRules(db)
	migrateBanners(db)
	migrateAds(db)
	migrateConfig(db)

	log.Println("━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🎉 全部数据迁移完成！")
}

// ─── JSON 读取辅助 ───

func readFullJSON(filename string) []map[string]any {
	matches, _ := filepath.Glob(filepath.Join(dataDir, filename))
	if len(matches) == 0 {
		log.Printf("⚠️ 文件不存在: %s", filename)
		return nil
	}

	raw, err := os.ReadFile(matches[0])
	if err != nil {
		log.Printf("⚠️ 读取失败: %s: %v", filename, err)
		return nil
	}

	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		log.Printf("⚠️ 解析失败: %s: %v", filename, err)
		return nil
	}

	var records []map[string]any
	if err := json.Unmarshal(wrapper.Data, &records); err != nil {
		log.Printf("⚠️ 数据数组解析失败: %s: %v", filename, err)
		return nil
	}
	return records
}

// ─── 类型辅助 ───

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	}
	return 0
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	}
	return fmt.Sprintf("%v", v)
}

func toInt(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case json.Number:
		i, _ := val.Int64()
		return i
	}
	return 0
}

func nullString(v any) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	s := toString(v)
	if s == "" || s == "null" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(v any) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	s := toString(v)
	if s == "" || s == "null" || s == "0000-00-00 00:00:00" {
		return sql.NullTime{}
	}
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// ─── 批量写入 ───

func bulkInsert(db *sql.DB, table string, columns []string, rows [][]any) {
	if len(rows) == 0 {
		return
	}

	batchSize := 500
	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[i:end]

		placeholders := make([]string, len(batch))
		args := make([]any, 0, len(batch)*len(columns))
		for j := range batch {
			rowPlaceholders := make([]string, len(columns))
			for k := range columns {
				rowPlaceholders[k] = "?"
			}
			placeholders[j] = "(" + strings.Join(rowPlaceholders, ",") + ")"
			args = append(args, batch[j]...)
		}

		query := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s",
			table,
			strings.Join(columns, ","),
			strings.Join(placeholders, ","))

		if _, err := db.Exec(query, args...); err != nil {
			log.Printf("❌ 批量写入 %s 失败 (offset=%d): %v", table, i, err)
			return
		}
	}
	log.Printf("   ✅ %s: %d 条", table, len(rows))
}

// ============================================================
// 各表迁移
// ============================================================

func migrateUsers(db *testingDB) {
	log.Println("━━━ 迁移用户数据 ━━━")
	records := readFullJSON("*user_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var userRows, walletRows, contractRows [][]any
	for _, r := range records {
		uid := toInt(r["id"])
		// 修复 pid: 0 保留为 0
		pid := toInt(r["pid"])

		userRows = append(userRows, []any{
			uid,                                   // id
			toString(r["username"]),               // username
			toString(r["nickname"]),               // nickname
			toString(r["mobile"]),                 // mobile
			toString(r["password"]),               // password
			toString(r["salt"]),                   // salt
			toInt(r["sex"]),                       // sex
			toString(r["avatar"]),                 // avatar
			toString(r["invite"]),                 // invite
			toInt(r["level"]),                     // level
			nullString(r["birthday"]),             // birthday
			toInt(r["is_vip"]),                    // is_vip
			nullString(r["viptime"]),              // viptime
			toInt(r["is_resell"]),                 // is_resell
			toInt(r["max_order"]),                 // max_order
			toString(r["contract"]),               // contract
			pid,                                   // pid
			nullString(r["join_time"]),            // join_time (可能为空)
			toString(r["join_ip"]),                // join_ip
			nullString(r["last_time"]),            // last_time
			toString(r["last_ip"]),                // last_ip
			nullString(r["token"]),                // token
			toInt(r["status"]),                    // status
			nullString(r["created_at"]),           // created_at
			nullString(r["updated_at"]),           // updated_at
		})

		walletRows = append(walletRows, []any{
			nil, uid,
			toFloat(r["money"]),
			toFloat(r["coupon"]),
			toFloat(r["self_bonus"]),
			toFloat(r["share_bonus"]),
			toInt(r["score"]),
			toFloat(r["poor"]),
			toString(r["updated_at"]),
		})

		// 合同
		contract := toString(r["contract"])
		if contract != "" {
			contractRows = append(contractRows, []any{
				nil, uid, contract, toString(r["created_at"]),
			})
		}
	}

	bulkInsert2(db, "users", []string{
		"id", "username", "nickname", "mobile", "password", "salt", "sex", "avatar",
		"invite", "level", "birthday", "is_vip", "viptime", "is_resell", "max_order",
		"contract", "pid", "join_time", "join_ip", "last_time", "last_ip", "token",
		"status", "created_at", "updated_at",
	}, userRows)

	bulkInsert2(db, "user_wallets", []string{
		"id", "user_id", "money", "coupon", "self_bonus", "share_bonus", "score", "poor", "updated_at",
	}, walletRows)

	if len(contractRows) > 0 {
		bulkInsert2(db, "user_contracts", []string{
			"id", "user_id", "contract_path", "created_at",
		}, contractRows)
	}
}

func migrateMerchandises(db *testingDB) {
	log.Println("━━━ 迁移寄售商品 ━━━")
	records := readFullJSON("*merchandise_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rows = append(rows, []any{
			toInt(r["id"]),
			nullInt(r["old_id"]),
			toInt(r["user_id"]),
			toString(r["title"]),
			toString(r["image"]),
			toFloat(r["price"]),
			toInt(r["is_show"]),
			toInt(r["status"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, "merchandises", []string{
		"id", "old_id", "user_id", "title", "image", "price",
		"is_show", "status", "created_at", "updated_at",
	}, rows)
}

func migrateOrders(db *testingDB) {
	log.Println("━━━ 迁移订单 (86,650条) ━━━")
	records := readFullJSON("*order_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rows = append(rows, []any{
			toInt(r["id"]),
			nullInt(r["old_id"]),
			toString(r["order_sn"]),
			toInt(r["seller_id"]),
			toInt(r["buyer_id"]),
			toInt(r["merchandise_id"]),
			toFloat(r["total_money"]),
			toInt(r["is_resell"]),
			toInt(r["is_show"]),
			toString(r["consignee"]),
			toString(r["phone"]),
			toString(r["province"]),
			toString(r["city"]),
			toString(r["area"]),
			toString(r["address"]),
			toString(r["pay_img"]),
			nullString(r["pay_time"]),
			nullString(r["buy_time"]),
			nullString(r["confirm_time"]),
			toInt(r["status"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, "orders", []string{
		"id", "old_id", "order_sn", "seller_id", "buyer_id", "merchandise_id",
		"total_money", "is_resell", "is_show", "consignee", "phone",
		"province", "city", "area", "address", "pay_img",
		"pay_time", "buy_time", "confirm_time", "status", "created_at", "updated_at",
	}, rows)
}

func migrateWithdraws(db *testingDB) {
	log.Println("━━━ 迁移提现记录 ━━━")
	records := readFullJSON("*withdraw_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var withdrawRows, acctRows [][]any
	seenAccts := make(map[int64]bool)

	for _, r := range records {
		uid := toInt(r["user_id"])
		acctID := toInt(r["account_id"])

		withdrawRows = append(withdrawRows, []any{
			toInt(r["id"]),
			toString(r["transfer_no"]),
			uid,
			toInt(r["cate"]),
			toInt(r["account_type"]),
			acctID,
			toFloat(r["money"]),
			toFloat(r["handling_fee"]),
			toFloat(r["actual_amount"]),
			toInt(r["status"]),
			toString(r["remark"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})

		// 解析 account_info JSON
		if !seenAccts[acctID] {
			acctJSON := toString(r["account_info"])
			if acctJSON != "" {
				var acct map[string]any
				if err := json.Unmarshal([]byte(acctJSON), &acct); err == nil {
					acctType := toInt(r["account_type"])
					acctRows = append(acctRows, []any{
						toInt(acct["id"]),
						toInt(acct["user_id"]),
						toString(acct["username"]),
						toString(acct["account"]),
						acctType,
						nullString(acct["bank"]),
						toString(acct["phone"]),
						nullString(acct["qrcode"]),
						nullString(acct["created_at"]),
						nullString(acct["updated_at"]),
					})
					seenAccts[acctID] = true
				}
			}
		}
	}

	bulkInsert2(db, "withdraws", []string{
		"id", "transfer_no", "user_id", "cate", "account_type", "account_id",
		"money", "handling_fee", "actual_amount", "status", "remark",
		"created_at", "updated_at",
	}, withdrawRows)

	if len(acctRows) > 0 {
		bulkInsert2(db, "withdraw_accounts", []string{
			"id", "user_id", "username", "account", "account_type",
			"bank", "phone", "qrcode", "created_at", "updated_at",
		}, acctRows)
	}
}

func migrateCouponLogs(db *testingDB) {
	log.Println("━━━ 迁移优惠券明细 (41,433条) ━━━")
	migrateLogTable(db, "*coupon-log_select_FULL.json", "coupon_logs",
		[]string{"id", "user_id", "type", "money", "before", "after", "memo", "created_at", "updated_at"})
}

func migrateSelfBonusLogs(db *testingDB) {
	log.Println("━━━ 迁移个人奖金明细 (139,100条) ━━━")
	migrateLogTable(db, "*selfbonus-log_select_FULL.json", "self_bonus_logs",
		[]string{"id", "user_id", "type", "money", "before", "after", "memo", "created_at", "updated_at"})
}

func migrateShareBonusLogs(db *testingDB) {
	log.Println("━━━ 迁移推广奖金明细 (72,298条) ━━━")
	migrateLogTable(db, "*sharebonus-log_select_FULL.json", "share_bonus_logs",
		[]string{"id", "user_id", "type", "money", "before", "after", "memo", "created_at", "updated_at"})
}

func migrateLogTable(db *testingDB, filePattern, table string, columns []string) {
	records := readFullJSON(filePattern)
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rows = append(rows, []any{
			toInt(r["id"]),
			toInt(r["user_id"]),
			toInt(r["type"]),
			toFloat(r["money"]),
			toFloat(r["before"]),
			toFloat(r["after"]),
			toString(r["memo"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, table, columns, rows)
}

func migrateGoods(db *testingDB) {
	log.Println("━━━ 迁移商品 ━━━")
	records := readFullJSON("*good_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rows = append(rows, []any{
			toInt(r["id"]),
			toInt(r["goods_category_id"]),
			toString(r["title"]),
			toString(r["images"]),
			toFloat(r["price"]),
			toFloat(r["line_price"]),
			toInt(r["stock_num"]),
			toInt(r["sales_volume"]),
			toString(r["content"]),
			nullString(r["notes"]),
			toInt(r["status"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, "goods", []string{
		"id", "category_id", "title", "images", "price", "line_price",
		"stock_num", "sales_volume", "content", "notes", "status",
		"created_at", "updated_at",
	}, rows)
}

func migrateCategories(db *testingDB) {
	log.Println("━━━ 迁移分类 ━━━")
	records := readFullJSON("*category_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rows = append(rows, []any{
			toInt(r["id"]),
			toInt(r["pid"]),
			toString(r["title"]),
			toString(r["image"]),
			toInt(r["sort"]),
			toInt(r["status"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, "categories", []string{
		"id", "pid", "title", "image", "sort", "status", "created_at", "updated_at",
	}, rows)
}

func migrateAdminUsers(db *testingDB) {
	log.Println("━━━ 迁移管理员 ━━━")
	records := readFullJSON("*admin_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rows = append(rows, []any{
			toInt(r["id"]),
			toString(r["username"]),
			toString(r["nickname"]),
			toString(r["password"]),
			toString(r["avatar"]),
			nullString(r["email"]),
			nullString(r["mobile"]),
			toString(r["qrcode"]),
			toString(r["roles"]),
			toBool(r["show_toolbar"]),
			nullString(r["login_at"]),
			nullInt(r["status"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, "admin_users", []string{
		"id", "username", "nickname", "password", "avatar", "email", "mobile",
		"qrcode", "roles", "show_toolbar", "login_at", "status", "created_at", "updated_at",
	}, rows)
}

func migrateRoles(db *testingDB) {
	log.Println("━━━ 迁移角色 ━━━")
	records := readFullJSON("*role_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rules, _ := json.Marshal(toString(r["rules"]))
		rows = append(rows, []any{
			toInt(r["id"]),
			toString(r["name"]),
			string(rules),
			toInt(r["status"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, "roles", []string{
		"id", "name", "rules", "status", "created_at", "updated_at",
	}, rows)
}

func migrateRules(db *testingDB) {
	log.Println("━━━ 迁移菜单规则 ━━━")
	records := readFullJSON("*rule_select_FULL.json")
	if len(records) == 0 {
		return
	}

	var rows [][]any
	for _, r := range records {
		rows = append(rows, []any{
			toInt(r["id"]),
			toString(r["title"]),
			toString(r["name"]),
			toString(r["icon"]),
			toString(r["key"]),
			toInt(r["pid"]),
			toString(r["href"]),
			toInt(r["type"]),
			toInt(r["weight"]),
			toInt(r["value"]),
			nullString(r["created_at"]),
			nullString(r["updated_at"]),
		})
	}

	bulkInsert2(db, "rules", []string{
		"id", "title", "name", "icon", "key", "pid", "href", "type",
		"weight", "value", "created_at", "updated_at",
	}, rows)
}

func migrateBanners(db *testingDB) {
	log.Println("━━━ 迁移Banner ━━━")
	records := readFullJSON("*banner_select_FULL.json")
	if len(records) == 0 {
		return
	}
	// 动态建表并插入（banner表DDL未定义，直接INSERT IGNORE）
	for _, r := range records {
		var keys []string
		var vals []any
		var placeholders []string
		for k, v := range r {
			keys = append(keys, "`"+k+"`")
			vals = append(vals, v)
			placeholders = append(placeholders, "?")
		}
		query := fmt.Sprintf("INSERT IGNORE INTO banners (%s) VALUES (%s)",
			strings.Join(keys, ","), strings.Join(placeholders, ","))
		db.Exec(query, vals...)
	}
	log.Printf("   ✅ banners: %d 条", len(records))
}

func migrateAds(db *testingDB) {
	log.Println("━━━ 迁移广告 ━━━")
	records := readFullJSON("*ad_select_FULL.json")
	if len(records) == 0 {
		return
	}
	for _, r := range records {
		var keys []string
		var vals []any
		var placeholders []string
		for k, v := range r {
			keys = append(keys, "`"+k+"`")
			vals = append(vals, v)
			placeholders = append(placeholders, "?")
		}
		query := fmt.Sprintf("INSERT IGNORE INTO ads (%s) VALUES (%s)",
			strings.Join(keys, ","), strings.Join(placeholders, ","))
		db.Exec(query, vals...)
	}
	log.Printf("   ✅ ads: %d 条", len(records))
}

func migrateConfig(db *testingDB) {
	log.Println("━━━ 迁移系统配置 ━━━")
	// config/get 是单条 JSON 对象，不是数组
	matches, _ := filepath.Glob(filepath.Join(dataDir, "*config_get*"))
	if len(matches) == 0 {
		return
	}
	raw, _ := os.ReadFile(matches[0])
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	json.Unmarshal(raw, &wrapper)

	var config map[string]any
	json.Unmarshal(wrapper.Data, &config)

	var rows [][]any
	for k, v := range config {
		val, _ := json.Marshal(v)
		rows = append(rows, []any{nil, k, string(val), time.Now().Format("2006-01-02 15:04:05")})
	}

	bulkInsert2(db, "system_configs", []string{"id", "config_key", "config_value", "updated_at"}, rows)
}

// ─── 辅助 ───

func nullInt(v any) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: toInt(v), Valid: true}
}

func toBool(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case bool:
		if val {
			return 1
		}
		return 0
	case float64:
		if val != 0 {
			return 1
		}
		return 0
	}
	return 0
}

// 重命名避免与标准库冲突
type testingDB = sql.DB

func bulkInsert2(db *sql.DB, table string, columns []string, rows [][]any) {
	if len(rows) == 0 {
		log.Printf("   ⚠️ %s: 0条数据", table)
		return
	}

	batchSize := 500
	total := 0
	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		batch := rows[i:end]

		placeholders := make([]string, len(batch))
		args := make([]any, 0, len(batch)*len(columns))
		for j := range batch {
			rowPH := make([]string, len(columns))
			for k := range columns {
				rowPH[k] = "?"
			}
			placeholders[j] = "(" + strings.Join(rowPH, ",") + ")"
			args = append(args, batch[j]...)
		}

		// 每个列名加反引号，避免保留字冲突
		quotedCols := make([]string, len(columns))
		for i, c := range columns {
			quotedCols[i] = "`" + c + "`"
		}
		query := fmt.Sprintf("INSERT IGNORE INTO `%s` (%s) VALUES %s",
			table, strings.Join(quotedCols, ","), strings.Join(placeholders, ","))

		if _, err := db.Exec(query, args...); err != nil {
			log.Printf("❌ %s 写入失败 (offset=%d): %v", table, i, err)
			if len(query) > 200 {
				fmt.Printf("   SQL片段: %s...\n", query[:200])
			}
			return
		}
		total += len(batch)

		if total%10000 == 0 || total == len(rows) {
			log.Printf("   📊 %s: %d/%d", table, total, len(rows))
		}
	}
	log.Printf("   ✅ %s: %d 条完成", table, total)
}

// 空函数避免 unused import
var _ = bulkInsert
