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
	idList := make([]int64, 0, len(idSet))
	for id := range idSet { idList = append(idList, id) }

	fmt.Println("SET FOREIGN_KEY_CHECKS=0;")

	// Withdraws
	wdRaw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_withdraw_select_FULL.json")
	wdStr := string(wdRaw)
	wdCount := 0
	for _, uid := range idList {
		sf := 0
		for {
			pos := strings.Index(wdStr[sf:], fmt.Sprintf(`"user_id": %d`, uid))
			if pos < 0 { break }
			pos += sf; sf = pos + 20
			sBrace := strings.LastIndex(wdStr[:pos], "{")
			chunk := wdStr[sBrace : pos+2000]
			eBrace := strings.Index(chunk[1:], "}") + 1
			chunk = chunk[:eBrace+1]

			status := extract(chunk, "status")
			ns := "2"
			if status == "1" { ns = "1" } else if status == "2" { ns = "3" }

			fmt.Printf("INSERT INTO withdraws (transfer_no,user_id,cate,account_type,currency_type,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES('%s',%d,%s,%s,'%s',%s,%s,%s,%s,'%s','%s',NOW());\n",
				extract(chunk, "transfer_no"), uid, extract(chunk, "cate"), extract(chunk, "account_type"),
				esc(extract(chunk, "currency_type")), extract(chunk, "money"), extract(chunk, "handling_fee"),
				extract(chunk, "actual_amount"), ns, esc(extract(chunk, "remark")),
				extract(chunk, "created_at"))
			wdCount++
		}
	}

	// Logs
	tables := []struct{ prefix, table string }{
		{"coupon", "coupon_logs"},
		{"selfbonus", "self_bonus_logs"},
		{"sharebonus", "share_bonus_logs"},
	}
	for _, t := range tables {
		raw, _ := os.ReadFile(fmt.Sprintf("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_%s-log_select_FULL.json", t.prefix))
		ls := string(raw)
		logCount := 0
		for _, uid := range idList {
			sf := 0
			for {
				pos := strings.Index(ls[sf:], fmt.Sprintf(`"user_id": %d`, uid))
				if pos < 0 { break }
				pos += sf; sf = pos + 20
				sBrace := strings.LastIndex(ls[:pos], "{")
				chunk := ls[sBrace : pos+2000]
				eBrace := strings.Index(chunk[1:], "}") + 1
				chunk = chunk[:eBrace+1]

				fmt.Printf("INSERT INTO %s (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(%d,%s,%s,%s,%s,'%s','%s',NOW());\n",
					t.table, uid, extract(chunk, "type"), extract(chunk, "money"),
					extract(chunk, "before"), extract(chunk, "after"),
					esc(extract(chunk, "memo")), extract(chunk, "created_at"))
				logCount++
			}
		}
		fmt.Fprintf(os.Stderr, "%s: %d\n", t.prefix, logCount)
	}
	fmt.Println("SET FOREIGN_KEY_CHECKS=1;")
	fmt.Fprintf(os.Stderr, "提现: %d\n", wdCount)
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
