package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	idRaw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/downline_ids.txt")
	lines := strings.Split(strings.TrimSpace(string(idRaw)), "\n")[1:]
	idSet := map[int64]bool{}
	for _, ln := range lines {
		id, _ := strconv.ParseInt(strings.TrimSpace(ln), 10, 64)
		if id > 0 { idSet[id] = true }
	}

	raw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_order_select_FULL.json")
	s := string(raw)
	n := len(s)

	fmt.Println("SET FOREIGN_KEY_CHECKS=0;")
	cnt := 0
	sf := 0
	for {
		if sf >= n { break }
		pos := strings.Index(s[sf:], `"buyer_id": `)
		if pos < 0 { break }
		pos += sf; sf = pos + 15

		buyerRest := s[pos+12:]
		buyerE := strings.Index(buyerRest, ",")
		if buyerE < 0 { continue }
		buyerID, _ := strconv.ParseInt(strings.TrimSpace(buyerRest[:buyerE]), 10, 64)
		if !idSet[buyerID] { continue }

		objStart := strings.LastIndex(s[:pos], "{")
		chunkEnd := pos + 3000
		if chunkEnd > n { chunkEnd = n }
		chunk := s[objStart:chunkEnd]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = chunk[:eBrace+1]

		isResell := extract(chunk, "is_resell")
		if isResell != "0" { continue } // 只导未寄卖

		id := extract(chunk, "id")
		orderSN := extract(chunk, "order_sn")
		sellerID := extract(chunk, "seller_id")
		totalMoney := extract(chunk, "total_money")
		isShow := extract(chunk, "is_show")
		consignee := extract(chunk, "consignee")
		phone := extract(chunk, "phone")
		province := extract(chunk, "province")
		city := extract(chunk, "city")
		area := extract(chunk, "area")
		address := extract(chunk, "address")
		payImg := extract(chunk, "pay_img")
		payTime := extract(chunk, "pay_time")
		buyTime := extract(chunk, "buy_time")
		confirmTime := extract(chunk, "confirm_time")
		status := extract(chunk, "status")
		mid := extract(chunk, "merchandise_id")

		pt := func(v string) string {
			if v == "" || v == "null" { return "NULL" }
			return "'" + strings.ReplaceAll(v, "'", "''") + "'"
		}

		fmt.Printf("INSERT INTO orders (id,order_sn,seller_id,buyer_id,merchandise_id,total_money,is_resell,is_show,consignee,phone,province,city,area,address,pay_img,pay_time,buy_time,confirm_time,status,coupon_settled,created_at,updated_at) VALUES(%s,'%s',%s,%s,%s,%s,0,%s,'%s','%s','%s','%s','%s','%s','%s',%s,%s,%s,%s,0,NOW(),NOW());\n",
			id, esc(orderSN), sellerID, buyerRest[:buyerE], mid, totalMoney, isShow,
			esc(consignee), esc(phone), esc(province), esc(city), esc(area), esc(address),
			esc(payImg), pt(payTime), pt(buyTime), pt(confirmTime), status)
		cnt++
	}
	fmt.Println("SET FOREIGN_KEY_CHECKS=1;")
	fmt.Fprintf(os.Stderr, "未寄卖订单: %d\n", cnt)
}

func extract(s, key string) string {
	idx := strings.Index(s, `"`+key+`"`)
	if idx < 0 { return "" }
	idx += len(key) + 4
	r := strings.TrimSpace(s[idx:])
	if len(r) > 0 && r[0] == '"' {
		e := strings.Index(r[1:], `"`) + 1
		return r[1:e]
	}
	e := strings.IndexAny(r, ",}\n")
	if e < 0 { e = len(r) }
	return strings.TrimSpace(r[:e])
}
func esc(s string) string { return strings.ReplaceAll(s, "'", "''") }
