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

var db *sql.DB
var uid = int64(97872)

func main() {
	var err error
	db, err = sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true")
	if err != nil { fmt.Println("DB:", err); return }
	defer db.Close()
	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS=1")

	importOrders()
	importWithdraws()
	importLogs("coupon")
	importLogs("selfbonus")
	importLogs("sharebonus")
	fmt.Println("全部完成!")
}

func importOrders() {
	raw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_order_select_FULL.json")
	s := string(raw)
	db.Exec("DELETE FROM orders WHERE buyer_id=? OR seller_id=?", uid, uid)
	sellerIDs := map[int64]bool{}
	cnt, sf := 0, 0
	for {
		posB := strings.Index(s[sf:], fmt.Sprintf(`"buyer_id": %d`, uid))
		posS := strings.Index(s[sf:], fmt.Sprintf(`"seller_id": %d`, uid))
		if posB < 0 && posS < 0 { break }
		p := 999999999
		if posB >= 0 { p = posB + sf }
		if posS >= 0 && posS+sf < p { p = posS + sf }
		sf = p + 20
		sBrace := strings.LastIndex(s[:p], "{")
		chunk := s[sBrace : p+3000]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = chunk[:eBrace+1]
		var o map[string]interface{}
		json.Unmarshal([]byte(fixJSON(chunk)), &o)
		sellerIDs[int64(toInt(o["seller_id"]))] = true
		db.Exec(`INSERT INTO orders (id,order_sn,seller_id,buyer_id,merchandise_id,total_money,is_resell,is_show,consignee,phone,province,city,area,address,pay_img,pay_time,buy_time,confirm_time,status,coupon_settled,created_at,updated_at) VALUES(?,?,?,?,1,?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,NOW(),NOW())`,
			toInt(o["id"]), ts(o["order_sn"]), toInt(o["seller_id"]), toInt(o["buyer_id"]),
			toFloat(o["total_money"]), toInt(o["is_resell"]), toInt(o["is_show"]),
			ts(o["consignee"]), ts(o["phone"]), ts(o["province"]), ts(o["city"]), ts(o["area"]), ts(o["address"]),
			ts(o["pay_img"]), pt(ts(o["pay_time"])), pt(ts(o["buy_time"])), pt(ts(o["confirm_time"])), toInt(o["status"]))
		cnt++
	}
	fmt.Printf("订单: %d 条, 涉及卖家: %d 个\n", cnt, len(sellerIDs))
	for sid := range sellerIDs {
		if sid == uid { continue }
		var exists int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE id=?", sid).Scan(&exists)
		if exists == 0 {
			db.Exec("INSERT INTO users (id,username,nickname,mobile,password,salt,sex,avatar,invite,level,pid,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())",
				sid, fmt.Sprintf("ext_%d", sid), fmt.Sprintf("seller%d", sid), "", "", "", 0, "/app/admin/avatar.png", "", 0, 0, 0)
			db.Exec("INSERT INTO user_wallets (user_id,money,coupon,self_bonus,share_bonus,score,poor,updated_at) VALUES(?,0,0,0,0,0,0,NOW())", sid)
		}
	}
	fmt.Println("卖家占位 OK")
}

func importWithdraws() {
	raw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_withdraw_select_FULL.json")
	s := string(raw)
	db.Exec("DELETE FROM withdraws WHERE user_id=?", uid)
	cnt, sf := 0, 0
	for {
		pos := strings.Index(s[sf:], fmt.Sprintf(`"user_id": %d`, uid))
		if pos < 0 { break }
		pos += sf; sf = pos + 20
		sBrace := strings.LastIndex(s[:pos], "{")
		chunk := s[sBrace : pos+2000]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = chunk[:eBrace+1]
		var w map[string]interface{}
		json.Unmarshal([]byte(fixJSON(chunk)), &w)
		ns := 2
		switch toInt(w["status"]) { case 1: ns = 1; case 2: ns = 3 }
		db.Exec("INSERT INTO withdraws (transfer_no,user_id,cate,account_type,currency_type,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,NOW())",
			ts(w["transfer_no"]), toInt(w["user_id"]), toInt(w["cate"]), toInt(w["account_type"]),
			ts(w["currency_type"]), toFloat(w["money"]), toFloat(w["handling_fee"]), toFloat(w["actual_amount"]), ns, ts(w["remark"]))
		cnt++
	}
	fmt.Printf("提现: %d 条\n", cnt)
}

func importLogs(prefix string) {
	raw, _ := os.ReadFile(fmt.Sprintf("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_%s-log_select_FULL.json", prefix))
	s := string(raw)
	table := prefix + "_logs"
	db.Exec("DELETE FROM "+table+" WHERE user_id=?", uid)
	cnt, sf := 0, 0
	for {
		pos := strings.Index(s[sf:], fmt.Sprintf(`"user_id": %d`, uid))
		if pos < 0 { break }
		pos += sf; sf = pos + 20
		sBrace := strings.LastIndex(s[:pos], "{")
		chunk := s[sBrace : pos+2000]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = chunk[:eBrace+1]
		var l map[string]interface{}
		json.Unmarshal([]byte(fixJSON(chunk)), &l)
		db.Exec("INSERT INTO "+table+" (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,NOW())",
			toInt(l["user_id"]), toInt(l["type"]), toFloat(l["money"]), toFloat(l["before"]), toFloat(l["after"]), ts(l["memo"]))
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
