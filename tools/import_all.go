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
		if id > 0 {
			idSet[id] = true
		}
	}

	fmt.Println("SET FOREIGN_KEY_CHECKS=0;")

	sf, n := 0, len(s)
	for {
		if sf >= n {
			break
		}
		pos := strings.Index(s[sf:], `"id": `)
		if pos < 0 {
			break
		}
		pos += sf
		sf = pos + 10

		idRest := s[pos+6:]
		idE := strings.Index(idRest, ",")
		if idE < 0 {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(idRest[:idE]), 10, 64)
		if err != nil || !idSet[id] {
			continue
		}

		objStart := strings.LastIndex(s[:pos], "{")
		chunkEnd := pos + 2000
		if chunkEnd > n {
			chunkEnd = n
		}
		chunk := s[objStart:chunkEnd]
		eBrace := strings.Index(chunk[1:], "}") + 1
		chunk = chunk[:eBrace+1]

		vals := map[string]string{
			"username":           "",
			"nickname":           "",
			"mobile":             "",
			"password":           "",
			"salt":               "",
			"sex":                "0",
			"avatar":             "/app/admin/avatar.png",
			"invite":             "",
			"level":              "0",
			"pid":                "0",
			"status":             "1",
			"is_resell":          "0",
			"is_vip":             "0",
			"max_order":          "8",
			"today_buy_total":    "0",
			"today_buy_count":    "0",
			"today_sell_total":   "0",
			"yesterday_sell_count": "0",
			"join_time":          "",
			"last_time":          "",
		}
		for k := range vals {
			v := extract(chunk, k)
			if v != "" {
				vals[k] = v
			}
		}
		if vals["nickname"] == "" {
			vals["nickname"] = vals["mobile"]
		}

		jt := vals["join_time"]; if jt == "" || jt == "null" { jt = "NULL" } else { jt = "'" + esc(jt) + "'" }
		lt := vals["last_time"]; if lt == "" || lt == "null" { lt = "NULL" } else { lt = "'" + esc(lt) + "'" }
		fmt.Printf("INSERT INTO users (id,username,nickname,mobile,password,salt,sex,avatar,invite,level,birthday,is_vip,viptime,is_resell,is_priority,max_order,today_buy_total,today_buy_count,today_sell_total,yesterday_sell_count,contract,pid,join_time,join_ip,last_time,last_ip,token,status,created_at,updated_at) VALUES(%d,'%s','%s','%s','%s','%s',%s,'%s','%s',%s,NULL,%s,NULL,%s,0,%s,%s,%s,%s,%s,'',%s,%s,'',%s,'','',%s,NOW(),NOW());\n",
			id,
			esc(vals["username"]), esc(vals["nickname"]), esc(vals["mobile"]), esc(vals["password"]), esc(vals["salt"]),
			vals["sex"], esc(vals["avatar"]), esc(vals["invite"]),
			vals["level"], vals["is_vip"], vals["is_resell"], vals["max_order"],
			vals["today_buy_total"], vals["today_buy_count"], vals["today_sell_total"], vals["yesterday_sell_count"],
			vals["pid"], jt, lt, vals["status"])

		delete(idSet, id)
	}
	fmt.Println("SET FOREIGN_KEY_CHECKS=1;")
}

func esc(s string) string { return strings.ReplaceAll(s, "'", "''") }

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
