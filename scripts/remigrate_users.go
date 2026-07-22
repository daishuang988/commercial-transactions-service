package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/flash_sale?charset=utf8mb4&parseTime=true&loc=Local"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	matches, _ := filepath.Glob("./tools/old_system_migration/output/data/*user_select_FULL.json")
	if len(matches) == 0 {
		fmt.Println("❌ 找不到全量JSON文件")
		os.Exit(1)
	}
	raw, _ := os.ReadFile(matches[0])
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	json.Unmarshal(raw, &wrapper)
	var records []map[string]interface{}
	json.Unmarshal(wrapper.Data, &records)
	fmt.Printf("读取 %d 条用户记录\n", len(records))

	// 清空
	db.Exec("SET FOREIGN_KEY_CHECKS=0")
	db.Exec("TRUNCATE user_contracts")
	db.Exec("TRUNCATE user_wallets")
	db.Exec("DELETE FROM users")
	db.Exec("SET FOREIGN_KEY_CHECKS=1")
	fmt.Println("已清空 users / user_wallets / user_contracts")

	// 批量插入
	userStmt, _ := db.Prepare(`INSERT INTO users
		(id,username,nickname,mobile,password,salt,sex,avatar,invite,level,birthday,
		 is_vip,viptime,is_resell,max_order,today_buy_total,today_buy_count,
		 today_sell_total,yesterday_sell_count,contract,pid,
		 join_time,join_ip,last_time,last_ip,token,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)

	walletStmt, _ := db.Prepare(`INSERT INTO user_wallets
		(user_id,money,coupon,self_bonus,share_bonus,score,poor,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`)

	contractStmt, _ := db.Prepare(`INSERT INTO user_contracts
		(user_id,contract_path,created_at) VALUES(?,?,?)`)

	users, wallets, contracts := 0, 0, 0
	batchSize := 500

	for i := 0; i < len(records); i += batchSize {
		tx, _ := db.Begin()
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]
		for _, r := range batch {
			uid := toInt(r["id"])
			pid := toInt(r["pid"])
			now, _ := time.Parse("2006-01-02 15:04:05", toStr(r["created_at"]))
			upd, _ := time.Parse("2006-01-02 15:04:05", toStr(r["updated_at"]))

			tx.Stmt(userStmt).Exec(
				uid, toStr(r["username"]), toStr(r["nickname"]), toStr(r["mobile"]),
				toStr(r["password"]), toStr(r["salt"]), toInt(r["sex"]), toStr(r["avatar"]),
				toStr(r["invite"]), toInt(r["level"]), nullDate(r["birthday"]),
				toInt(r["is_vip"]), nullDate(r["viptime"]), toInt(r["is_resell"]),
				toInt(r["max_order"]), toFloat(r["today_buy_total"]), int(toInt(r["today_buy_count"])),
				toFloat(r["today_sell_total"]), int(toInt(r["yesterday_sell_count"])),
				toStr(r["contract"]), pid,
				nullDate(r["join_time"]), toStr(r["join_ip"]),
				nullDate(r["last_time"]), toStr(r["last_ip"]),
				nullStr(r["token"]), toInt(r["status"]), now, upd,
			)
			users++

			tx.Stmt(walletStmt).Exec(
				uid, toFloat(r["money"]), toFloat(r["coupon"]),
				toFloat(r["self_bonus"]), toFloat(r["share_bonus"]),
				toInt(r["score"]), toFloat(r["poor"]), upd,
			)
			wallets++

			contract := toStr(r["contract"])
			if contract != "" {
				tx.Stmt(contractStmt).Exec(uid, contract, now)
				contracts++
			}
		}
		tx.Commit()
		fmt.Printf("  进度: %d/%d (用户:%d 钱包:%d 合同:%d)\n", end, len(records), users, wallets, contracts)
	}

	// 验证
	fmt.Println("\n========== 验证 ==========")
	var verify struct {
		TotalUsers int64; TotalWallets int64; TotalContracts int64
		SumMoney float64; SumCoupon float64; SumSB float64; SumShb float64
		SumTBT float64; SumTBC int64; SumTST float64; SumYSC int64
		SumPoor float64
	}
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&verify.TotalUsers)
	db.QueryRow("SELECT COUNT(*) FROM user_wallets").Scan(&verify.TotalWallets)
	db.QueryRow("SELECT COUNT(*) FROM user_contracts").Scan(&verify.TotalContracts)
	db.QueryRow("SELECT COALESCE(SUM(money),0),COALESCE(SUM(coupon),0),COALESCE(SUM(self_bonus),0),COALESCE(SUM(share_bonus),0),COALESCE(SUM(poor),0) FROM user_wallets").Scan(&verify.SumMoney, &verify.SumCoupon, &verify.SumSB, &verify.SumShb, &verify.SumPoor)
	db.QueryRow("SELECT COALESCE(SUM(today_buy_total),0),COALESCE(SUM(today_buy_count),0),COALESCE(SUM(today_sell_total),0),COALESCE(SUM(yesterday_sell_count),0) FROM users").Scan(&verify.SumTBT, &verify.SumTBC, &verify.SumTST, &verify.SumYSC)

	fmt.Printf("  users: %d 条\n", verify.TotalUsers)
	fmt.Printf("  wallets: %d 条\n", verify.TotalWallets)
	fmt.Printf("  contracts: %d 条\n", verify.TotalContracts)
	fmt.Printf("  余额: %.2f  优惠券: %.2f  个人奖金: %.2f  推广奖金: %.2f  poor: %.2f\n", verify.SumMoney, verify.SumCoupon, verify.SumSB, verify.SumShb, verify.SumPoor)
	fmt.Printf("  今日买: %.2f(%d笔)  今日卖: %.2f  昨日卖: %d笔\n", verify.SumTBT, verify.SumTBC, verify.SumTST, verify.SumYSC)

	// 抽查一个已知用户
	var sample struct {
		ID int64; Username string; Nickname string; SB float64; TBT float64; Poor float64
	}
	db.QueryRow("SELECT u.id,u.username,u.nickname,w.self_bonus,u.today_buy_total,w.poor FROM users u JOIN user_wallets w ON u.id=w.user_id WHERE u.id=99929").Scan(&sample.ID, &sample.Username, &sample.Nickname, &sample.SB, &sample.TBT, &sample.Poor)
	fmt.Printf("\n  抽查 99929 %s %s: self_bonus=%.3f today_buy=%.2f poor=%.2f\n", sample.Username, sample.Nickname, sample.SB, sample.TBT, sample.Poor)
}

func toStr(v interface{}) string {
	if v == nil { return "" }
	switch val := v.(type) {
	case string: return val
	case float64:
		if val == float64(int64(val)) { return strconv.FormatInt(int64(val), 10) }
		return strconv.FormatFloat(val, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}

func toInt(v interface{}) int64 {
	if v == nil { return 0 }
	switch val := v.(type) {
	case float64: return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64); return i
	}
	return 0
}

func toFloat(v interface{}) float64 {
	if v == nil { return 0 }
	switch val := v.(type) {
	case float64: return val
	case string:
		f, _ := strconv.ParseFloat(val, 64); return f
	}
	return 0
}

func nullStr(v interface{}) interface{} {
	if v == nil { return nil }
	s := toStr(v)
	if s == "" || s == "null" { return nil }
	return s
}

func nullDate(v interface{}) interface{} {
	if v == nil { return nil }
	s := toStr(v)
	if s == "" || s == "null" || s == "0000-00-00 00:00:00" || strings.Contains(s, "-0001") { return nil }
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil { return nil }
	return t
}
