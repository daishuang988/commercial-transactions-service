package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var dataDir = "./tools/old_system_migration/output_new/data"

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
	fmt.Println("  定向迁移：袁小华(94694) 粉丝树")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	// 1. 加载所有用户，构建粉丝树
	allUsers := loadAllUsers()
	treeUsers, treeIDs := buildFanTree(allUsers, 94694)
	treeIDs[94694] = true // 包含袁小华本人
	treeUsers = append(treeUsers, allUsers[94694])
	fmt.Printf("粉丝树: %d 人 (含袁小华)\n", len(treeUsers))

	// 2. 迁移用户 + 钱包 + 合同
	migrateUsers(treeUsers)

	// 3. 迁移订单
	migrateOrders(treeIDs)

	// 4. 迁移寄售商品
	migrateMerchandises(treeIDs)

	// 5. 迁移提现
	migrateWithdraws(treeIDs)

	// 6. 迁移财务日志
	migrateCouponLogs(treeIDs)
	migrateSelfBonusLogs(treeIDs)
	migrateShareBonusLogs(treeIDs)

	db.Exec("SET FOREIGN_KEY_CHECKS=1")
	fmt.Println()
	fmt.Println("════════════════════════")
	fmt.Println("  迁移完成 ✅")
	fmt.Println("════════════════════════")
}

// ─── 通用工具 ───

func s(v interface{}) string {
	if v == nil { return "" }
	switch val := v.(type) {
	case string: return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%v", val)
	}
	return fmt.Sprintf("%v", v)
}

func i(v interface{}) int64 {
	if v == nil { return 0 }
	switch val := v.(type) {
	case float64: return int64(val)
	case string:
		var x int64
		fmt.Sscanf(val, "%d", &x)
		return x
	}
	return 0
}

func f(v interface{}) float64 {
	if v == nil { return 0 }
	switch val := v.(type) {
	case float64: return val
	case string:
		var x float64
		fmt.Sscanf(val, "%f", &x)
		return x
	}
	return 0
}


func mapOldLevel(old int64) int64 {
	if old >= 1 { return old - 1 }
	return old
}
func nd(v interface{}) interface{} {
	if v == nil { return nil }
	str := s(v)
	if str == "" || str == "null" || str == "0000-00-00 00:00:00" { return nil }
	return str
}

func nstr(v interface{}) interface{} {
	if v == nil { return nil }
	str := s(v)
	if str == "" || str == "null" { return nil }
	return str
}

// ─── 数据加载 ───

func loadAllUsers() map[int64]map[string]interface{} {
	recs := readFull("*user_select_FULL.json")
	users := make(map[int64]map[string]interface{})
	for _, r := range recs {
		id := i(r["id"])
		users[id] = r
	}
	fmt.Printf("  加载 %d 个用户\n", len(users))
	return users
}

func readFull(pattern string) []map[string]interface{} {
	matches, _ := filepath.Glob(filepath.Join(dataDir, pattern))
	if len(matches) == 0 {
		fmt.Printf("  ⚠️ 未找到 %s\n", pattern)
		return nil
	}
	raw, _ := os.ReadFile(matches[0])
	var w struct {
		Data json.RawMessage `json:"data"`
	}
	json.Unmarshal(raw, &w)
	var recs []map[string]interface{}
	json.Unmarshal(w.Data, &recs)
	return recs
}

func buildFanTree(users map[int64]map[string]interface{}, rootID int64) ([]map[string]interface{}, map[int64]bool) {
	var result []map[string]interface{}
	ids := make(map[int64]bool)
	visited := make(map[int64]bool)
	var dfs func(pid int64)
	dfs = func(pid int64) {
		for id, u := range users {
			if visited[id] { continue }
			parentID := i(u["pid"])
			if parentID == pid {
				visited[id] = true
				ids[id] = true
				result = append(result, u)
				dfs(id)
			}
		}
	}
	dfs(rootID)
	return result, ids
}

// ─── 用户 + 钱包 + 合同 ───

func migrateUsers(users []map[string]interface{}) {
	us, _ := db.Prepare(`INSERT INTO users(id,username,nickname,mobile,password,salt,sex,avatar,invite,level,birthday,is_vip,viptime,is_resell,is_priority,max_order,today_buy_total,today_buy_count,today_sell_total,yesterday_sell_count,contract,pid,join_time,join_ip,last_time,last_ip,token,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	ws, _ := db.Prepare(`INSERT INTO user_wallets(user_id,money,coupon,self_bonus,share_bonus,score,poor,updated_at) VALUES(?,?,?,?,?,?,?,?)`)

	uc, wc, cc := 0, 0, 0
	for _, r := range users {
		uid := i(r["id"])
		now := nd(r["created_at"])
		upd := nd(r["updated_at"])
		if upd == nil { upd = now }

		us.Exec(uid, s(r["username"]), s(r["nickname"]), s(r["mobile"]),
			s(r["password"]), s(r["salt"]), i(r["sex"]), s(r["avatar"]),
			s(r["invite"]), mapOldLevel(i(r["level"])), nd(r["birthday"]),
			i(r["is_vip"]), nd(r["viptime"]), i(r["is_resell"]),
			int64(0), i(r["max_order"]),
			f(r["today_buy_total"]), i(r["today_buy_count"]),
			f(r["today_sell_total"]), i(r["yesterday_sell_count"]),
			s(r["contract"]), i(r["pid"]),
			nd(r["join_time"]), s(r["join_ip"]),
			nd(r["last_time"]), s(r["last_ip"]),
			nstr(r["token"]), i(r["status"]), now, upd)
		uc++

		ws.Exec(uid, f(r["money"]), f(r["coupon"]), f(r["self_bonus"]), f(r["share_bonus"]),
			i(r["score"]), f(r["poor"]), upd)
		wc++

		}
	}
	fmt.Printf("  ✅ users: %d | wallets: %d | contracts: %d\n", uc, wc, cc)
}

// ─── 订单 ───

func migrateOrders(treeIDs map[int64]bool) {
	recs := readFull("*order_select_FULL.json")
	if recs == nil { return }

	st, _ := db.Prepare(`INSERT INTO orders(id,order_sn,seller_id,buyer_id,merchandise_id,total_money,is_resell,is_show,consignee,phone,province,city,area,address,pay_img,pay_time,buy_time,confirm_time,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)

	count := 0
	for _, r := range recs {
		sellerID := i(r["seller_id"])
		buyerID := i(r["buyer_id"])
		// 买方或卖方在粉丝树中
		if !treeIDs[sellerID] && !treeIDs[buyerID] { continue }

		st.Exec(i(r["id"]), s(r["order_sn"]),
			sellerID, buyerID, i(r["merchandise_id"]),
			f(r["total_money"]), i(r["is_resell"]), i(r["is_show"]),
			s(r["consignee"]), s(r["phone"]),
			s(r["province"]), s(r["city"]), s(r["area"]), s(r["address"]),
			s(r["pay_img"]), nd(r["pay_time"]), nd(r["buy_time"]), nd(r["confirm_time"]),
			i(r["status"]), nd(r["created_at"]), nd(r["updated_at"]))
		count++
	}
	fmt.Printf("  ✅ orders: %d\n", count)
}

// ─── 寄售商品 ───

func migrateMerchandises(treeIDs map[int64]bool) {
	recs := readFull("*merchandise_select_FULL.json")
	if recs == nil { return }

	st, _ := db.Prepare(`INSERT INTO merchandises(id,old_id,user_id,title,image,price,is_show,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`)

	count := 0
	for _, r := range recs {
		uid := i(r["user_id"])
		if !treeIDs[uid] { continue }

		st.Exec(i(r["id"]), nstr(r["old_id"]), uid,
			s(r["title"]), s(r["image"]), f(r["price"]),
			i(r["is_show"]), i(r["status"]),
			nd(r["created_at"]), nd(r["updated_at"]))
		count++
	}
	fmt.Printf("  ✅ merchandises: %d\n", count)
}

// ─── 提现 ───

func migrateWithdraws(treeIDs map[int64]bool) {
	recs := readFull("*withdraw_select_FULL.json")
	if recs == nil { return }

	// 先迁收款账户
	accts := make(map[int64]bool)
	as, _ := db.Prepare(`INSERT IGNORE INTO withdraw_accounts(id,user_id,username,account,account_type,bank,phone,qrcode,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`)
	for _, r := range recs {
		uid := i(r["user_id"])
		if !treeIDs[uid] { continue }
		aid := i(r["account_id"])
		if accts[aid] { continue }
		accts[aid] = true

		acctJSON := s(r["account_info"])
		if acctJSON == "" { continue }
		var a map[string]interface{}
		if json.Unmarshal([]byte(acctJSON), &a) != nil { continue }
		as.Exec(i(a["id"]), i(a["user_id"]),
			s(a["username"]), s(a["account"]),
			i(r["account_type"]), nstr(a["bank"]),
			s(a["phone"]), nstr(a["qrcode"]),
			nd(a["created_at"]), nd(a["updated_at"]))
	}

	ws, _ := db.Prepare(`INSERT INTO withdraws(id,transfer_no,user_id,cate,account_type,currency_type,account_id,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	count := 0
	for _, r := range recs {
		uid := i(r["user_id"])
		if !treeIDs[uid] { continue }

			// 状态映射: 老0待审核→新2待处理, 老1已打款→新1已通过, 老2已驳回→新3已驳回
			oldStatus := i(r["status"])
			newStatus := map[int64]int64{0: 2, 1: 1, 2: 3}[oldStatus]
			if newStatus == 0 { newStatus = oldStatus }
			// 货币类型: 老cate=2→coupon, 老cate=4→share_bonus
			oldCate := i(r["cate"])
			currencyType := ""
			if oldCate == 2 { currencyType = "coupon" }
			if oldCate == 4 { currencyType = "share_bonus" }

		ws.Exec(i(r["id"]), s(r["transfer_no"]), uid,
			i(r["cate"]), i(r["account_type"]),
			currencyType, i(r["account_id"]),
			f(r["money"]), f(r["handling_fee"]), f(r["actual_amount"]),
			newStatus, s(r["remark"]),
			nd(r["created_at"]), nd(r["updated_at"]))
		count++
	}
	fmt.Printf("  ✅ withdraws: %d\n", count)
}

// ─── 财务日志 ───

func migrateCouponLogs(treeIDs map[int64]bool) {
	recs := readFull("*coupon-log_select_FULL.json")
	if recs == nil { return }
	st, _ := db.Prepare("INSERT INTO coupon_logs(id,user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)")
	count := 0
	for _, r := range recs {
		uid := i(r["user_id"])
		if !treeIDs[uid] { continue }
		memo := s(r["memo"])
		if memo == "" {
			memo = strings.TrimSpace(fmt.Sprintf("%v", r["memo"]))
		}
		st.Exec(i(r["id"]), uid, i(r["type"]), f(r["money"]),
			f(r["before"]), f(r["after"]), memo, nd(r["created_at"]), nd(r["updated_at"]))
		count++
	}
	fmt.Printf("  ✅ coupon_logs: %d\n", count)
}

func migrateSelfBonusLogs(treeIDs map[int64]bool) {
	recs := readFull("*selfbonus-log_select_FULL.json")
	if recs == nil { return }
	st, _ := db.Prepare("INSERT INTO self_bonus_logs(id,user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)")
	count := 0
	for _, r := range recs {
		uid := i(r["user_id"])
		if !treeIDs[uid] { continue }
		memo := s(r["memo"])
		if memo == "" {
			memo = strings.TrimSpace(fmt.Sprintf("%v", r["memo"]))
		}
		st.Exec(i(r["id"]), uid, i(r["type"]), f(r["money"]),
			f(r["before"]), f(r["after"]), memo, nd(r["created_at"]), nd(r["updated_at"]))
		count++
	}
	fmt.Printf("  ✅ self_bonus_logs: %d\n", count)
}

func migrateShareBonusLogs(treeIDs map[int64]bool) {
	recs := readFull("*sharebonus-log_select_FULL.json")
	if recs == nil { return }
	st, _ := db.Prepare("INSERT INTO share_bonus_logs(id,user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)")
	count := 0
	for _, r := range recs {
		uid := i(r["user_id"])
		if !treeIDs[uid] { continue }
		memo := s(r["memo"])
		if memo == "" {
			memo = strings.TrimSpace(fmt.Sprintf("%v", r["memo"]))
		}
		st.Exec(i(r["id"]), uid, i(r["type"]), f(r["money"]),
			f(r["before"]), f(r["after"]), memo, nd(r["created_at"]), nd(r["updated_at"]))
		count++
	}
	fmt.Printf("  ✅ share_bonus_logs: %d\n", count)
}
