package main

import (
	"encoding/json"
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
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	json.Unmarshal(raw, &resp)

	fmt.Println("SET FOREIGN_KEY_CHECKS=0;")
	cnt := 0
	for _, w := range resp.Data {
		uid := int64(toInt(w["user_id"]))
		if !idSet[uid] { continue }

		transferNo := ts(w["transfer_no"])
		cate := toInt(w["cate"])
		acctType := toInt(w["account_type"])
		acctID := toInt(w["account_id"])
		money := toFloat(w["money"])
		fee := toFloat(w["handling_fee"])
		actual := toFloat(w["actual_amount"])
		status := toInt(w["status"])
		remark := ts(w["remark"])
		ca := ts(w["created_at"])

		curType := "coupon"
		if cate == 4 { curType = "share_bonus" }

		ns := 2
		if status == 1 { ns = 1
		} else if status == 2 { ns = 3 }

		if ca == "" { ca = "NOW()" } else { ca = "'" + esc(ca) + "'" }

		fmt.Printf("INSERT INTO withdraws (transfer_no,user_id,cate,account_type,currency_type,account_id,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES('%s',%d,%d,%d,'%s',%d,%.2f,%.2f,%.2f,%d,'%s',%s,NOW());\n",
			esc(transferNo), uid, cate, acctType, curType, acctID, money, fee, actual, ns, esc(remark), ca)
		cnt++
	}
	fmt.Println("SET FOREIGN_KEY_CHECKS=1;")
	fmt.Fprintf(os.Stderr, "%d\n", cnt)
}

func ts(v interface{}) string { return fmt.Sprintf("%v", v) }
func toInt(v interface{}) int {
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" || s == "" { return 0 }
	n, _ := strconv.Atoi(strings.TrimSpace(s)); return n
}
func toFloat(v interface{}) float64 {
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" || s == "" { return 0 }
	var f float64
	fmt.Sscanf(strings.TrimSpace(s), "%f", &f); return f
}
func esc(s string) string { return strings.ReplaceAll(s, "'", "''") }
