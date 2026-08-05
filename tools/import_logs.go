package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, _ := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true")
	defer db.Close()
	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	defer db.Exec("SET FOREIGN_KEY_CHECKS=1")

	uid := int64(97872)
	prefixes := []string{"coupon", "self_bonus", "share_bonus"}
	for _, p := range prefixes {
		db.Exec("DELETE FROM "+p+"_logs WHERE user_id=?", uid)
		importLogs(db, p, uid)
	}
	fmt.Println("done")
}

func importLogs(db *sql.DB, prefix string, uid int64) {
	raw, _ := os.ReadFile(fmt.Sprintf("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_%s-log_select_FULL.json", prefix))
	s := string(raw)
	table := prefix + "_logs"
	cnt, sf, errs := 0, 0, 0
	for {
		pos := strings.Index(s[sf:], fmt.Sprintf(`"user_id": %d`, uid))
		if pos < 0 { break }
		pos += sf; sf = pos + 20
		sBrace := strings.LastIndex(s[:pos], "{")
		chunk := s[sBrace : pos+2000]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = fixJSON(chunk[:eBrace+1])
		var l map[string]interface{}
		json.Unmarshal([]byte(chunk), &l)
		_, err := db.Exec("INSERT INTO "+table+" (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,?,NOW())",
			uid, toInt(l["type"]), toFloat(l["money"]), toFloat(l["before"]), toFloat(l["after"]), ts(l["memo"]))
		if err != nil { errs++; fmt.Printf("ERR %s: %v\n", prefix, err) } else { cnt++ }
	}
	fmt.Printf("%s: ok=%d err=%d\n", prefix, cnt, errs)
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
	n, _ := strconv.Atoi(strings.TrimSpace(s)); return n
}
func toFloat(v interface{}) float64 {
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" || s == "" || s == "null" { return 0 }
	var f float64
	fmt.Sscanf(strings.TrimSpace(s), "%f", &f); return f
}
