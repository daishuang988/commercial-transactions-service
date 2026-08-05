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

	raw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/old_system_migration/output_new2/data/api.srdsmgs.com_app_admin_withdraw_select_FULL.json")
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	json.Unmarshal(raw, &resp)

	// Read our 532 user IDs
	idRaw, _ := os.ReadFile("E:/Commercial_Transactions_Service/tools/downline_ids.txt")
	lines := strings.Split(strings.TrimSpace(string(idRaw)), "\n")[1:]
	idSet := map[int64]bool{}
	for _, ln := range lines {
		id, _ := strconv.ParseInt(strings.TrimSpace(ln), 10, 64)
		if id > 0 { idSet[id] = true }
	}

	db.Exec("DELETE FROM withdraw_accounts")
	cnt := 0
	seen := map[int64]bool{}
	for _, w := range resp.Data {
		uid := int64(toFloat(w["user_id"]))
		if !idSet[uid] { continue }
		aiStr := fmt.Sprintf("%v", w["account_info"])
		if aiStr == "" || aiStr == "<nil>" { continue }

		var ai map[string]interface{}
		json.Unmarshal([]byte(aiStr), &ai)
		acctID := int64(toFloat(ai["id"]))
		if acctID == 0 || seen[acctID] { continue }
		seen[acctID] = true

		username := fmt.Sprintf("%v", ai["username"])
		account := fmt.Sprintf("%v", ai["account"])
		qrcode := fmt.Sprintf("%v", ai["qrcode"])
		phone := fmt.Sprintf("%v", ai["phone"])
		createdAt := fmt.Sprintf("%v", ai["created_at"])

		if createdAt == "" || createdAt == "<nil>" { createdAt = "NOW()" } else { createdAt = "'" + esc(createdAt) + "'" }

		acctType := toFloat(w["account_type"])
		db.Exec("INSERT INTO withdraw_accounts (id,user_id,username,account,account_type,qrcode,phone,created_at,updated_at) VALUES(?,?,?,?,?,?,?,"+createdAt+",NOW())",
			acctID, toFloat(ai["user_id"]), esc(username), esc(account), acctType, esc(qrcode), esc(phone))
		cnt++
	}
	fmt.Printf("导入 %d 条收款账户\n", cnt)
}

func esc(s string) string { return strings.ReplaceAll(s, "'", "''") }
func toFloat(v interface{}) float64 {
	s := fmt.Sprintf("%v", v)
	var f float64
	fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	return f
}
func init() { _ = strconv.Itoa(0) }
