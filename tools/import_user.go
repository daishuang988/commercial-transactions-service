package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil { fmt.Println("DB:", err); return }
	defer db.Close()
	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS=1")

	uid := int64(97872)

	// 1. User
	userRaw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_user_select_FULL.json")
	userStr := string(userRaw)
	idx := strings.Index(userStr, fmt.Sprintf(`"id": %d,`, uid))
	if idx < 0 { fmt.Println("用户不存在"); return }
	startBrace := strings.LastIndex(userStr[:idx], "{")
	userChunk := userStr[startBrace : idx+2000]
	endBrace := strings.Index(userChunk, "}")
	userChunk = fixJSON(userChunk[:endBrace+1])
	var u map[string]interface{}
	json.Unmarshal([]byte(userChunk), &u)

	fmt.Printf("用户: %.0f %s %s\n", u["id"], u["username"], u["nickname"])

	// 先删
	db.Exec("DELETE FROM orders WHERE buyer_id=? OR seller_id=?", uid, uid)
	db.Exec("DELETE FROM user_wallets WHERE user_id=?", uid)
	db.Exec("DELETE FROM users WHERE id=?", uid)

	// 插入用户
	pid := toInt(u["pid"])
	level := toInt(u["level"])
	status := toInt(u["status"])
	isResell := toInt(u["is_resell"])
	isVip := toInt(u["is_vip"])
	maxOrder := toInt(u["max_order"])
	tbt := toFloat(u["today_buy_total"])
	tbc := toInt(u["today_buy_count"])
	tst := toFloat(u["today_sell_total"])
	ysc := toInt(u["yesterday_sell_count"])

	joinTime := pt(ts(u["join_time"]))
	lastTime := pt(ts(u["last_time"]))
	_, err = db.Exec(`INSERT INTO users (id,username,nickname,mobile,password,salt,sex,avatar,invite,level,birthday,is_vip,viptime,is_resell,is_priority,max_order,today_buy_total,today_buy_count,today_sell_total,yesterday_sell_count,contract,pid,join_time,join_ip,last_time,last_ip,token,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())`,
		uid, ts(u["username"]), ts(u["nickname"]), ts(u["mobile"]), ts(u["password"]), ts(u["salt"]),
		toInt(u["sex"]), ts(u["avatar"]), ts(u["invite"]),
		level, nil, isVip, nil, isResell, 0, maxOrder, tbt, tbc, tst, ysc,
		"", pid, joinTime, "", lastTime, "", "", status)
	if err != nil { fmt.Println("用户失败:", err); return }
	fmt.Println("用户 OK")

	// 2. Wallet
	wm, wc, ws, wsh := toFloat(u["money"]), toFloat(u["coupon"]), toFloat(u["self_bonus"]), toFloat(u["share_bonus"])
	_, err = db.Exec("INSERT INTO user_wallets (user_id,money,coupon,self_bonus,share_bonus,score,poor,updated_at) VALUES(?,?,?,?,?,0,0,NOW())", uid, wm, wc, ws, wsh)
	fmt.Printf("钱包: %.2f %.2f %.2f %.2f\n", wm, wc, ws, wsh)
	if err != nil { fmt.Println("钱包失败:", err) }

	// 3. Orders
	orderRaw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_order_select_FULL.json")
	orderStr := string(orderRaw)
	oc := 0
	sellerIDs := map[int64]bool{}
	searchFrom := 0
	for {
		posB := strings.Index(orderStr[searchFrom:], fmt.Sprintf(`"buyer_id": %d`, uid))
		posS := strings.Index(orderStr[searchFrom:], fmt.Sprintf(`"seller_id": %d`, uid))
		if posB < 0 && posS < 0 { break }
		p := -1
		if posB >= 0 { p = posB + searchFrom }
		if posS >= 0 { ps := posS + searchFrom; if p < 0 || ps < p { p = ps } }
		searchFrom = p + 20
		objStart := strings.LastIndex(orderStr[:p], "{")
		chunk := orderStr[objStart:min(p+3000, len(orderStr))]
		end := strings.Index(chunk[1:], "}") + 1
		chunk = fixJSON(chunk[:end+1])
		var o map[string]interface{}
		json.Unmarshal([]byte(chunk), &o)

		oid := int64(toInt(o["id"]))
		sellerID := int64(toInt(o["seller_id"]))
		buyerID := int64(toInt(o["buyer_id"]))
		sellerIDs[sellerID] = true
		tm := toFloat(o["total_money"])
		isR := toInt(o["is_resell"])
		isS := toInt(o["is_show"])
		oStatus := toInt(o["status"])

		db.Exec("INSERT INTO orders (id,order_sn,seller_id,buyer_id,merchandise_id,total_money,is_resell,is_show,consignee,phone,province,city,area,address,pay_img,pay_time,buy_time,confirm_time,status,coupon_settled,created_at,updated_at) VALUES(?,?,?,?,1,?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,NOW(),NOW())",
			oid, ts(o["order_sn"]), sellerID, buyerID, tm, isR, isS,
			ts(o["consignee"]), ts(o["phone"]), ts(o["province"]), ts(o["city"]), ts(o["area"]), ts(o["address"]),
			ts(o["pay_img"]), pt(ts(o["pay_time"])), pt(ts(o["buy_time"])), pt(ts(o["confirm_time"])), oStatus)
		oc++
	}
	fmt.Printf("订单: %d 条\n", oc)

	// Seller placeholders
	for sid := range sellerIDs {
		if sid == uid { continue }
		var exists int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE id=?", sid).Scan(&exists)
		if exists == 0 {
			db.Exec("INSERT INTO users (id,username,nickname,mobile,password,salt,sex,avatar,invite,level,pid,status,created_at,updated_at,is_resell,is_vip,is_priority,max_order,today_buy_total,today_buy_count,today_sell_total,yesterday_sell_count,contract,join_ip,last_ip) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW(),0,0,0,0,0,0,0,0,'','','')",
				sid, fmt.Sprintf("ext_%d", sid), fmt.Sprintf("seller%d", sid), "", "", "", 0, "/app/admin/avatar.png", "", 0, 0, 0)
			db.Exec("INSERT INTO user_wallets (user_id,money,coupon,self_bonus,share_bonus,score,poor,updated_at) VALUES(?,0,0,0,0,0,0,NOW())", sid)
		}
	}
	fmt.Println("卖家占位 OK")

	// 4. Withdraws
	db.Exec("DELETE FROM withdraws WHERE user_id=?", uid)
	importWithdraws(db, uid)
	// 5. Logs
	db.Exec("DELETE FROM coupon_logs WHERE user_id=?", uid)
	db.Exec("DELETE FROM self_bonus_logs WHERE user_id=?", uid)
	db.Exec("DELETE FROM share_bonus_logs WHERE user_id=?", uid)
	importLogs(db, "coupon", uid)
	importLogs(db, "selfbonus", uid)
	importLogs(db, "sharebonus", uid)

	fmt.Println("全部完成!")
}

func importWithdraws(db *sql.DB, uid int64) {
	raw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_withdraw_select_FULL.json")
	s := string(raw)
	cnt, sf := 0, 0
	for {
		pos := strings.Index(s[sf:], fmt.Sprintf(`"user_id": %d`, uid))
		if pos < 0 { break }
		pos += sf; sf = pos + 20
		objStart := strings.LastIndex(s[:pos], "{")
		chunk := s[objStart:min(pos+2000, len(s))]
		end := strings.Index(chunk[1:], "}") + 1
		chunk = fixJSON(chunk[:end+1])
		var w map[string]interface{}
		json.Unmarshal([]byte(chunk), &w)
		ns := 2
		switch toInt(w["status"]) {
		case 1: ns = 1
		case 2: ns = 3
		}
		db.Exec("INSERT INTO withdraws (id,transfer_no,user_id,cate,account_type,currency_type,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,NOW())",
			toInt(w["id"]), ts(w["transfer_no"]), toInt(w["user_id"]), toInt(w["cate"]), toInt(w["account_type"]),
			ts(w["currency_type"]), toFloat(w["money"]), toFloat(w["handling_fee"]), toFloat(w["actual_amount"]), ns, ts(w["remark"]))
		cnt++
	}
	fmt.Printf("提现: %d 条\n", cnt)
}

func importLogs(db *sql.DB, prefix string, uid int64) {
	raw, _ := os.ReadFile(fmt.Sprintf("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_%s-log_select_FULL.json", prefix))
	s := string(raw)
	table := prefix + "_logs"
	cnt, sf := 0, 0
	for {
		pos := strings.Index(s[sf:], fmt.Sprintf(`"user_id": %d`, uid))
		if pos < 0 { break }
		pos += sf; sf = pos + 20
		objStart := strings.LastIndex(s[:pos], "{")
		chunk := s[objStart:min(pos+2000, len(s))]
		end := strings.Index(chunk[1:], "}") + 1
		chunk = fixJSON(chunk[:end+1])
		var l map[string]interface{}
		json.Unmarshal([]byte(chunk), &l)
		db.Exec("INSERT INTO "+table+" (id,user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,NOW())",
			toInt(l["id"]), toInt(l["user_id"]), toInt(l["type"]), toFloat(l["money"]), toFloat(l["before"]), toFloat(l["after"]), ts(l["memo"]))
		cnt++
	}
	fmt.Printf("%s: %d 条\n", prefix, cnt)
}

func fixJSON(s string) string {
	s = strings.ReplaceAll(s, ": ", ":")
	s = strings.ReplaceAll(s, ":null", `:""`)
	s = strings.ReplaceAll(s, `\/`, "/")
	return s
}
func ts(v interface{}) string { return fmt.Sprintf("%v", v) }
func toInt(v interface{}) int {
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" || s == "" || s == "null" { return 0 }
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
func toFloat(v interface{}) float64 {
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" || s == "" || s == "null" { return 0 }
	var f float64
	fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	return f
}
func pt(s string) *time.Time {
	if s == "" || s == "null" || s == "<nil>" { return nil }
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return &t
}
func min(a,b int) int { if a<b {return a}; return b }
