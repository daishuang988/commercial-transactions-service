package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	uid := "97872"
	tables := []struct{ prefix, table string }{
		{"coupon", "coupon_logs"},
		{"selfbonus", "self_bonus_logs"},
		{"sharebonus", "share_bonus_logs"},
	}
	for _, t := range tables {
		raw, _ := os.ReadFile(fmt.Sprintf("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_%s-log_select_FULL.json", t.prefix))
		s := string(raw)
		sf := 0
		for {
			pos := strings.Index(s[sf:], fmt.Sprintf(`"user_id": %s`, uid))
			if pos < 0 { break }
			pos += sf
			sf = pos + 20
			o := strings.LastIndex(s[:pos], "{")
			chunk := s[o : pos+2000]
			e := strings.Index(chunk[1:], "}") + 1
			chunk = chunk[:e+1]

			tp := extract(chunk, "type")
			money := extract(chunk, "money")
			before := extract(chunk, "before")
			after := extract(chunk, "after")
			memo := extract(chunk, "memo")
			ca := extract(chunk, "created_at")

			memo = strings.ReplaceAll(memo, "\\", "\\\\")
			memo = strings.ReplaceAll(memo, "'", "\\'")

			fmt.Printf("INSERT INTO %s (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(%s,%s,%s,%s,%s,'%s','%s',NOW());\n",
				t.table, uid, tp, money, before, after, memo, ca)
		}
	}
}

func extract(s, key string) string {
	idx := strings.Index(s, `"`+key+`"`)
	if idx < 0 {
		return ""
	}
	idx += len(key) + 4
	r := strings.TrimSpace(s[idx:])
	if len(r) > 0 && r[0] == '"' {
		e := strings.Index(r[1:], `"`) + 1
		return r[1:e]
	}
	e := strings.IndexAny(r, ",}\n")
	if e < 0 {
		e = len(r)
	}
	return strings.TrimSpace(r[:e])
}
