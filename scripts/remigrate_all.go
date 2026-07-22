package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var dataDir = "./tools/old_system_migration/output/data"

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true&loc=Local"
	}
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	db.Exec("SET FOREIGN_KEY_CHECKS=0")

	fmt.Println("═══════════════════════════════════════")
	fmt.Println("  全量数据重迁移（基于老系统完整JSON）")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	migrateUsers()
	migrateGoods()
	migrateCategories()
	migrateMerchandises()
	migrateOrders()
	migrateWithdraws()
	migrateCouponLogs()
	migrateSelfBonusLogs()
	migrateShareBonusLogs()
	migrateAdminUsers()
	migrateRoles()
	migrateRules()
	migrateBanners()
	migrateAds()

	db.Exec("SET FOREIGN_KEY_CHECKS=1")
	fmt.Println()
	fmt.Println("════════════════════════")
	fmt.Println("  全部迁移完成 ✅")
	fmt.Println("════════════════════════")
}

// ─── 通用工具 ───

func readFull(pattern string) []map[string]interface{} {
	matches, _ := filepath.Glob(filepath.Join(dataDir, pattern))
	if len(matches) == 0 {
		fmt.Printf("  ⚠️ 未找到 %s\n", pattern)
		return nil
	}
	raw, _ := os.ReadFile(matches[0])
	var w struct{ Data json.RawMessage }
	json.Unmarshal(raw, &w)
	var recs []map[string]interface{}
	json.Unmarshal(w.Data, &recs)
	return recs
}

func s(v interface{}) string {
	if v == nil { return "" }
	switch val := v.(type) {
	case string: return val
	case float64:
		if val == float64(int64(val)) { return strconv.FormatInt(int64(val), 10) }
		return strconv.FormatFloat(val, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

func i(v interface{}) int64 {
	if v == nil { return 0 }
	switch val := v.(type) {
	case float64: return int64(val)
	case string: x, _ := strconv.ParseInt(val, 10, 64); return x
	}
	return 0
}

func f(v interface{}) float64 {
	if v == nil { return 0 }
	switch val := v.(type) {
	case float64: return val
	case string: x, _ := strconv.ParseFloat(val, 64); return x
	}
	return 0
}

func nd(v interface{}) interface{} {
	if v == nil { return nil }
	str := s(v)
	if str == "" || str == "null" || str == "0000-00-00 00:00:00" { return nil }
	t, err := time.Parse("2006-01-02 15:04:05", str)
	if err != nil { return nil }
	return t
}

func nstr(v interface{}) interface{} {
	if v == nil { return nil }
	str := s(v)
	if str == "" || str == "null" { return nil }
	return str
}

// ══════════════════════════════════════
// 1. 用户 + 钱包 + 合同
// ══════════════════════════════════════
func migrateUsers() {
	recs := readFull("*user_select_FULL.json")
	if recs == nil { return }

	db.Exec("TRUNCATE user_contracts")
	db.Exec("TRUNCATE user_wallets")
	db.Exec("DELETE FROM users")

	us, _ := db.Prepare(`INSERT INTO users(id,username,nickname,mobile,password,salt,sex,avatar,invite,level,birthday,is_vip,viptime,is_resell,max_order,today_buy_total,today_buy_count,today_sell_total,yesterday_sell_count,contract,pid,join_time,join_ip,last_time,last_ip,token,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	ws, _ := db.Prepare(`INSERT INTO user_wallets(user_id,money,coupon,self_bonus,share_bonus,score,poor,updated_at) VALUES(?,?,?,?,?,?,?,?)`)
	cs, _ := db.Prepare(`INSERT INTO user_contracts(user_id,contract_path,created_at) VALUES(?,?,?)`)

	uc, wc, cc := 0, 0, 0
	for _, r := range recs {
		uid := i(r["id"])
		now, _ := time.Parse("2006-01-02 15:04:05", s(r["created_at"]))
		upd, _ := time.Parse("2006-01-02 15:04:05", s(r["updated_at"]))

		us.Exec(uid, s(r["username"]), s(r["nickname"]), s(r["mobile"]),
			s(r["password"]), s(r["salt"]), i(r["sex"]), s(r["avatar"]),
			s(r["invite"]), i(r["level"]), nd(r["birthday"]),
			i(r["is_vip"]), nd(r["viptime"]), i(r["is_resell"]),
			i(r["max_order"]), f(r["today_buy_total"]), int(i(r["today_buy_count"])),
			f(r["today_sell_total"]), int(i(r["yesterday_sell_count"])),
			s(r["contract"]), i(r["pid"]),
			nd(r["join_time"]), s(r["join_ip"]),
			nd(r["last_time"]), s(r["last_ip"]),
			nstr(r["token"]), i(r["status"]), now, upd)
		uc++

		ws.Exec(uid, f(r["money"]), f(r["coupon"]), f(r["self_bonus"]), f(r["share_bonus"]), i(r["score"]), f(r["poor"]), upd)
		wc++

		contract := s(r["contract"])
		if contract != "" {
			cs.Exec(uid, contract, now)
			cc++
		}
	}
	fmt.Printf("  ✅ users: %d | wallets: %d | contracts: %d\n", uc, wc, cc)
}

// ══════════════════════════════════════
// 2. 商品
// ══════════════════════════════════════
func migrateGoods() {
	recs := readFull("*good_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM goods")
	gs, _ := db.Prepare(`INSERT INTO goods(id,category_id,title,images,price,line_price,stock_num,sales_volume,content,notes,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		gs.Exec(i(r["id"]), i(r["goods_category_id"]), s(r["title"]), s(r["images"]),
			f(r["price"]), f(r["line_price"]), i(r["stock_num"]), i(r["sales_volume"]),
			s(r["content"]), nstr(r["notes"]), i(r["status"]),
			nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ goods: %d\n", len(recs))
}

// ══════════════════════════════════════
// 3. 分类
// ══════════════════════════════════════
func migrateCategories() {
	recs := readFull("*category_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM categories")
	cs, _ := db.Prepare(`INSERT INTO categories(id,pid,title,image,sort,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		cs.Exec(i(r["id"]), i(r["pid"]), s(r["name"]), s(r["image"]),
			i(r["sort"]), i(r["status"]), nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ categories: %d\n", len(recs))
}

// ══════════════════════════════════════
// 4. 寄售商品
// ══════════════════════════════════════
func migrateMerchandises() {
	recs := readFull("*merchandise_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM merchandises")
	ms, _ := db.Prepare(`INSERT INTO merchandises(id,old_id,user_id,title,image,price,is_show,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		ms.Exec(i(r["id"]), nstr(r["old_id"]), i(r["user_id"]), s(r["title"]), s(r["image"]),
			f(r["price"]), i(r["is_show"]), i(r["status"]), nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ merchandises: %d\n", len(recs))
}

// ══════════════════════════════════════
// 5. 订单
// ══════════════════════════════════════
func migrateOrders() {
	recs := readFull("*order_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM orders")
	os, _ := db.Prepare(`INSERT INTO orders(id,old_id,order_sn,seller_id,buyer_id,merchandise_id,total_money,is_resell,is_show,consignee,phone,province,city,area,address,pay_img,pay_time,buy_time,confirm_time,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		os.Exec(i(r["id"]), nstr(r["old_id"]), s(r["order_sn"]),
			i(r["seller_id"]), i(r["buyer_id"]), i(r["merchandise_id"]),
			f(r["total_money"]), i(r["is_resell"]), i(r["is_show"]),
			s(r["consignee"]), s(r["phone"]),
			s(r["province"]), s(r["city"]), s(r["area"]), s(r["address"]),
			s(r["pay_img"]), nd(r["pay_time"]), nd(r["buy_time"]), nd(r["confirm_time"]),
			i(r["status"]), nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ orders: %d\n", len(recs))
}

// ══════════════════════════════════════
// 6. 提现 + 收款账户
// ══════════════════════════════════════
func migrateWithdraws() {
	recs := readFull("*withdraw_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM withdraw_accounts")
	db.Exec("DELETE FROM withdraws")
	ws, _ := db.Prepare(`INSERT INTO withdraws(id,transfer_no,user_id,cate,account_type,account_id,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	as, _ := db.Prepare(`INSERT INTO withdraw_accounts(id,user_id,username,account,account_type,bank,phone,qrcode,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	seen := map[int64]bool{}
	wc, ac := 0, 0
	for _, r := range recs {
		ws.Exec(i(r["id"]), s(r["transfer_no"]), i(r["user_id"]), i(r["cate"]), i(r["account_type"]),
			i(r["account_id"]), f(r["money"]), f(r["handling_fee"]), f(r["actual_amount"]),
			i(r["status"]), s(r["remark"]), nd(r["created_at"]), nd(r["updated_at"]))
		wc++

		aid := i(r["account_id"])
		if !seen[aid] {
			acctJSON := s(r["account_info"])
			if acctJSON != "" {
				var a map[string]interface{}
				if json.Unmarshal([]byte(acctJSON), &a) == nil {
					as.Exec(i(a["id"]), i(a["user_id"]), s(a["username"]), s(a["account"]),
						i(r["account_type"]), nstr(a["bank"]), s(a["phone"]), nstr(a["qrcode"]),
						nd(a["created_at"]), nd(a["updated_at"]))
					ac++
					seen[aid] = true
				}
			}
		}
	}
	fmt.Printf("  ✅ withdraws: %d | accounts: %d\n", wc, ac)
}

// ══════════════════════════════════════
// 7-9. 财务日志
// ══════════════════════════════════════
func migrateCouponLogs() {
	recs := readFull("*coupon-log_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM coupon_logs")
	ls, _ := db.Prepare(`INSERT INTO coupon_logs(id,user_id,type,money,`+"`before`"+`,after,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		ls.Exec(i(r["id"]), i(r["user_id"]), i(r["type"]), f(r["money"]),
			f(r["before"]), f(r["after"]), s(r["memo"]), nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ coupon_logs: %d\n", len(recs))
}

func migrateSelfBonusLogs() {
	recs := readFull("*selfbonus-log_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM self_bonus_logs")
	ls, _ := db.Prepare(`INSERT INTO self_bonus_logs(id,user_id,type,money,`+"`before`"+`,after,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		ls.Exec(i(r["id"]), i(r["user_id"]), i(r["type"]), f(r["money"]),
			f(r["before"]), f(r["after"]), s(r["memo"]), nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ self_bonus_logs: %d\n", len(recs))
}

func migrateShareBonusLogs() {
	recs := readFull("*sharebonus-log_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM share_bonus_logs")
	ls, _ := db.Prepare(`INSERT INTO share_bonus_logs(id,user_id,type,money,`+"`before`"+`,after,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		ls.Exec(i(r["id"]), i(r["user_id"]), i(r["type"]), f(r["money"]),
			f(r["before"]), f(r["after"]), s(r["memo"]), nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ share_bonus_logs: %d\n", len(recs))
}

// ══════════════════════════════════════
// 10-14. 系统表
// ══════════════════════════════════════
func migrateAdminUsers() {
	recs := readFull("*admin_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM admin_users")
	as, _ := db.Prepare(`INSERT INTO admin_users(id,username,nickname,password,avatar,email,mobile,qrcode,roles,show_toolbar,login_at,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		var st *int8
		if v := r["status"]; v != nil {
			x := int8(i(v)); st = &x
		}
		showTb := 0
		if b, ok := r["show_toolbar"].(bool); ok && b { showTb = 1 }
		as.Exec(i(r["id"]), s(r["username"]), s(r["nickname"]), s(r["password"]),
			s(r["avatar"]), nstr(r["email"]), nstr(r["mobile"]), s(r["qrcode"]),
			s(r["roles"]), showTb, nd(r["login_at"]), st,
			nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ admin_users: %d\n", len(recs))
}

func migrateRoles() {
	recs := readFull("*role_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM roles")
	rs, _ := db.Prepare(`INSERT INTO roles(id,name,rules,status,created_at,updated_at) VALUES(?,?,?,?,?,?)`)
	for _, r := range recs {
		rulesJSON, _ := json.Marshal(s(r["rules"]))
		rs.Exec(i(r["id"]), s(r["name"]), string(rulesJSON), i(r["status"]), nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ roles: %d\n", len(recs))
}

func migrateRules() {
	recs := readFull("*rule_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM rules")
	rs, _ := db.Prepare(`INSERT INTO rules(id,title,name,icon,`+"`key`"+`,pid,href,type,weight,value,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		rs.Exec(i(r["id"]), s(r["title"]), s(r["name"]), s(r["icon"]), s(r["key"]),
			i(r["pid"]), s(r["href"]), i(r["type"]), i(r["weight"]), i(r["value"]),
			nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ rules: %d\n", len(recs))
}

func migrateBanners() {
	recs := readFull("*banner_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM banners")
	for _, r := range recs {
		db.Exec(`INSERT INTO banners(image,url,title,sort,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
			s(r["image"]), s(r["url"]), s(r["title"]), i(r["sort"]), i(r["status"]),
			nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ banners: %d\n", len(recs))
}

func migrateAds() {
	recs := readFull("*ad_select_FULL.json")
	if recs == nil { return }
	db.Exec("DELETE FROM ads")
	for _, r := range recs {
		db.Exec(`INSERT INTO ads(image,url,title,sort,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
			s(r["image"]), s(r["url"]), s(r["title"]), i(r["sort"]), i(r["status"]),
			nd(r["created_at"]), nd(r["updated_at"]))
	}
	fmt.Printf("  ✅ ads: %d\n", len(recs))
}
