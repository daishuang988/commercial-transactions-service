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

	raw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_withdraw_select_FULL.json")
	s := string(raw)
	n := len(s)

	fmt.Println("SET FOREIGN_KEY_CHECKS=0;")
	cnt := 0
	sf := 0
	for {
		if sf >= n { break }
		pos := strings.Index(s[sf:], "\n      \"user_id\": ")
		if pos < 0 { break }
		pos += sf; sf = pos + 20

		uidRest := s[pos+18:]
		uidE := strings.Index(uidRest, ",")
		if uidE < 0 { continue }
		uid, _ := strconv.ParseInt(strings.TrimSpace(uidRest[:uidE]), 10, 64)
		if !idSet[uid] { continue }

		sBrace := strings.LastIndex(s[:pos], "{")
		chunkEnd := pos + 3000
		if chunkEnd > n { chunkEnd = n }
		chunk := s[sBrace:chunkEnd]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = chunk[:eBrace+1]

		transferNo := extract(chunk, "transfer_no")
		cate := extract(chunk, "cate")
		acctType := extract(chunk, "account_type")
		money := extract(chunk, "money")
		fee := extract(chunk, "handling_fee")
		actual := extract(chunk, "actual_amount")
		status := extract(chunk, "status")
		remark := extract(chunk, "remark")
		createdAt := extract(chunk, "created_at")

		// currency_type from cate
		curType := "coupon"
		if cate == "4" { curType = "share_bonus" }

		// status mapping
		ns := "2"
		if status == "1" { ns = "1"
		} else if status == "2" { ns = "3" }

		if money == "" { money = "0" }
		if fee == "" { fee = "0" }
		if actual == "" { actual = "0" }
		if acctType == "" { acctType = "1" }

		if createdAt == "" { createdAt = "NOW()" } else { createdAt = "'" + esc(createdAt) + "'" }
		acctID := extract(chunk, "account_id")
		if acctID == "" { acctID = "0" }
		fmt.Printf("INSERT INTO withdraws (transfer_no,user_id,cate,account_type,currency_type,account_id,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES('%s',%d,%s,%s,'%s',%s,%s,%s,%s,%s,'%s',%s,NOW());\n",
			esc(transferNo), uid, cate, acctType, curType, acctID, money, fee, actual, ns, esc(remark), createdAt)
		cnt++
	}
	fmt.Println("SET FOREIGN_KEY_CHECKS=1;")
	fmt.Fprintf(os.Stderr, "%d\n", cnt)
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
