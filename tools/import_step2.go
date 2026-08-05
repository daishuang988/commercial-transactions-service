package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	raw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_user_select_FULL.json")
	s := string(raw)

	idRaw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/downline_ids.txt")
	lines := strings.Split(strings.TrimSpace(string(idRaw)), "\n")[1:]
	idSet := map[int64]bool{}
	for _, ln := range lines {
		id, _ := strconv.ParseInt(strings.TrimSpace(ln), 10, 64)
		if id > 0 { idSet[id] = true }
	}

	fmt.Println("SET FOREIGN_KEY_CHECKS=0;")
	sf, n := 0, len(s)
	for {
		if sf >= n { break }
		pos := strings.Index(s[sf:], `"id": `)
		if pos < 0 { break }
		pos += sf; sf = pos + 10
		idRest := s[pos+6:]
		idE := strings.Index(idRest, ",")
		if idE < 0 { continue }
		id, err := strconv.ParseInt(strings.TrimSpace(idRest[:idE]), 10, 64)
		if err != nil || !idSet[id] { continue }

		objStart := strings.LastIndex(s[:pos], "{")
		chunkEnd := pos + 2000
		if chunkEnd > n { chunkEnd = n }
		chunk := s[objStart:chunkEnd]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = chunk[:eBrace+1]

		money := extract(chunk, "money")
		coupon := extract(chunk, "coupon")
		self := extract(chunk, "self_bonus")
		share := extract(chunk, "share_bonus")
		if money == "" { money = "0" }
		if coupon == "" { coupon = "0" }
		if self == "" { self = "0" }
		if share == "" { share = "0" }

		fmt.Printf("INSERT INTO user_wallets (user_id,money,coupon,self_bonus,share_bonus,score,poor,updated_at) VALUES(%d,%s,%s,%s,%s,0,0,NOW());\n",
			id, money, coupon, self, share)
	}
	fmt.Println("SET FOREIGN_KEY_CHECKS=1;")
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
