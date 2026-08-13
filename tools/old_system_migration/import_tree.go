package main

// import_tree.go — 粉丝树数据导入工具（JSON → SQL，不连数据库）
//
// 用法: import_tree.exe <fan_tree_ids.txt> <dataDir> <outDir>
//
// 按粉丝树 654 人过滤老系统爬取数据，生成适配新系统表结构的 INSERT SQL：
//   01_users.sql            用户（密码重置为 123456、清空合同、token 置空）
//   02_user_wallets.sql     钱包（老系统 users 内联字段拆分）
//   03_merchandises.sql     寄售池商品（树内用户全部 + 订单关联的补充商品）
//   04_orders.sql           订单（买卖任一侧在树内；status=2 置 coupon_settled=1 防重复结算）
//   05_withdraw_accounts.sql 收款账户（account_info JSON 解析）
//   06_withdraws.sql        提现（cate→currency_type、status 映射）
//   07_self_bonus_logs.sql  个人奖金明细
//   08_share_bonus_logs.sql 推广奖金明细
//   coupon_logs 暂不导入（等老系统 23:59 结算后 00:10 重新爬取再导）

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const batchSize = 500

// 统计
type stats struct {
	badNumber  int
	badStatus  int
	badCate    int
	badAccount int
}

func (s *stats) report() {
	fmt.Printf("\n[转换告警] 数字解析失败: %d, 订单状态异常: %d, 提现cate异常: %d, account_info解析失败: %d\n",
		s.badNumber, s.badStatus, s.badCate, s.badAccount)
}

// ---------- 通用解析 ----------

type loader struct {
	st   *stats
	tree map[int64]bool
}

// 读 JSON 文件（"data" 数组），数字用 json.Number 保留原样
func loadJSON(path string) []map[string]interface{} {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var doc struct {
		Data []map[string]interface{} `json:"data"`
	}
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		panic(fmt.Sprintf("%s: %v", path, err))
	}
	return doc.Data
}

func asStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func asInt(v interface{}) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case json.Number:
		i, err := strconv.ParseInt(x.String(), 10, 64)
		if err == nil {
			return i
		}
		f, err2 := strconv.ParseFloat(x.String(), 64)
		if err2 == nil {
			return int64(f)
		}
		return 0
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err == nil {
			return i
		}
		f, err2 := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err2 == nil {
			return int64(f)
		}
		return 0
	case float64:
		return int64(x)
	default:
		return 0
	}
}

// 金额：字符串/数字 → 规范小数；失败返回 0 并计数
func (l *loader) asMoney(v interface{}) string {
	s := strings.TrimSpace(asStr(v))
	if s == "" {
		return "0"
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		l.st.badNumber++
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// datetime/date：空或 "0000-00-00" → NULL，其余直接引用
func sqlTime(v interface{}) string {
	s := strings.TrimSpace(asStr(v))
	if s == "" || s == "0000-00-00 00:00:00" || s == "0000-00-00" || s == "null" {
		return "NULL"
	}
	return "'" + s + "'"
}

// 必填时间：空则 NOW()
func sqlTimeRequired(v interface{}) string {
	s := sqlTime(v)
	if s == "NULL" {
		return "NOW()"
	}
	return s
}

// 字符串列：转义单引号/反斜杠
func sqlStr(v interface{}) string {
	s := asStr(v)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "''")
	return "'" + s + "'"
}

func sqlInt(v interface{}) string {
	return strconv.FormatInt(asInt(v), 10)
}

// 可空字符串：空 → NULL
func sqlStrNull(v interface{}) string {
	if asStr(v) == "" {
		return "NULL"
	}
	return sqlStr(v)
}

func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---------- SQL 输出 ----------

type batchWriter struct {
	w     *bufio.Writer
	table string
	cols  []string
	rows  []string
	count int
}

func newBatchWriter(w *bufio.Writer, table string, cols []string) *batchWriter {
	fmt.Fprintf(w, "SET NAMES utf8mb4;\nSET FOREIGN_KEY_CHECKS=0;\n")
	return &batchWriter{w: w, table: table, cols: cols}
}

func (b *batchWriter) add(vals ...string) {
	b.rows = append(b.rows, "("+strings.Join(vals, ",")+")")
	if len(b.rows) >= batchSize {
		b.flush()
	}
}

func (b *batchWriter) flush() {
	if len(b.rows) == 0 {
		return
	}
	fmt.Fprintf(b.w, "INSERT INTO `%s` (`%s`) VALUES\n%s;\n",
		b.table, strings.Join(b.cols, "`,`"), strings.Join(b.rows, ",\n"))
	b.count += len(b.rows)
	b.rows = b.rows[:0]
}

func (b *batchWriter) close() {
	b.flush()
	b.w.Flush()
}

// ---------- 各表导入 ----------

func (l *loader) importUsers(rows []map[string]interface{}, wallet *batchWriter, out *os.File) {
	w := newBatchWriter(bufio.NewWriter(out), "users", []string{
		"id", "username", "nickname", "mobile", "password", "salt", "sex", "avatar", "invite",
		"level", "birthday", "is_vip", "viptime", "is_resell", "is_priority", "max_order",
		"today_buy_total", "today_buy_count", "today_sell_total", "yesterday_sell_count",
		"contract", "pid", "join_time", "join_ip", "last_time", "last_ip", "token",
		"status", "created_at", "updated_at",
	})
	var minID, maxID int64 = 1 << 62, -1
	imported := 0
	for _, r := range rows {
		id := asInt(r["id"])
		if !l.tree[id] {
			continue
		}
		if id < minID {
			minID = id
		}
		if id > maxID {
			maxID = id
		}
		salt := asStr(r["salt"])
		w.add(
			strconv.FormatInt(id, 10),
			sqlStr(r["username"]),
			sqlStr(r["nickname"]),
			sqlStr(r["mobile"]),
			"'"+md5Hex("123456"+salt)+"'", // 密码重置为 123456
			sqlStr(salt),
			sqlInt(r["sex"]),
			sqlStr(r["avatar"]),
			sqlStr(r["invite"]),
			sqlInt(r["level"]),
			sqlTime(r["birthday"]),
			sqlInt(r["is_vip"]),
			sqlTime(r["viptime"]),
			sqlInt(r["is_resell"]),
			"0", // is_priority 老系统无此字段
			sqlInt(r["max_order"]),
			sqlInt(r["today_buy_total"]),
			sqlInt(r["today_buy_count"]),
			sqlInt(r["today_sell_total"]),
			sqlInt(r["yesterday_sell_count"]),
			"''", // contract 不保留老合同
			sqlInt(r["pid"]),
			sqlTime(r["join_time"]),
			sqlStr(r["join_ip"]),
			sqlTime(r["last_time"]),
			sqlStr(r["last_ip"]),
			"NULL", // token 置空强制重新登录
			sqlInt(r["status"]),
			sqlTimeRequired(r["created_at"]),
			sqlTimeRequired(r["updated_at"]),
		)
		// 钱包拆分
		wallet.add(
			strconv.FormatInt(id, 10),
			l.asMoney(r["money"]),
			l.asMoney(r["coupon"]),
			l.asMoney(r["self_bonus"]),
			l.asMoney(r["share_bonus"]),
			sqlInt(r["score"]),
			l.asMoney(r["poor"]),
			sqlTimeRequired(r["updated_at"]),
		)
		imported++
	}
	w.close()
	fmt.Printf("用户: %d 人 (id %d ~ %d), 钱包: %d 条\n", imported, minID, maxID, imported)
}

func (l *loader) importMerch(rows []map[string]interface{}, linkedIDs map[int64]bool) int {
	out, _ := os.Create(outPath("03_merchandises.sql"))
	w := newBatchWriter(bufio.NewWriter(out), "merchandises", []string{
		"id", "old_id", "user_id", "title", "image", "price", "is_show", "status", "created_at", "updated_at",
	})
	pool, linked := 0, 0
	for _, r := range rows {
		id := asInt(r["id"])
		uid := asInt(r["user_id"])
		if !l.tree[uid] && !linkedIDs[id] {
			continue
		}
		if l.tree[uid] {
			pool++
		} else {
			linked++
		}
		oldID := r["old_id"]
		var oldIDStr string
		if oldID == nil || asStr(oldID) == "" {
			oldIDStr = "NULL"
		} else {
			oldIDStr = sqlInt(oldID)
		}
		w.add(
			strconv.FormatInt(id, 10),
			oldIDStr,
			strconv.FormatInt(uid, 10),
			sqlStr(r["title"]),
			sqlStr(r["image"]),
			l.asMoney(r["price"]),
			sqlInt(r["is_show"]),
			sqlInt(r["status"]),
			sqlTimeRequired(r["created_at"]),
			sqlTimeRequired(r["updated_at"]),
		)
	}
	w.close()
	fmt.Printf("寄售商品: %d 条 (树内寄售池 %d + 订单关联补充 %d)\n", pool+linked, pool, linked)
	return pool + linked
}

func (l *loader) importOrders(rows []map[string]interface{}, merchAll map[int64]bool, oldToNew map[int64]int64) map[int64]bool {
	out, _ := os.Create(outPath("04_orders.sql"))
	w := newBatchWriter(bufio.NewWriter(out), "orders", []string{
		"id", "old_id", "order_sn", "seller_id", "buyer_id", "merchandise_id", "total_money",
		"is_resell", "is_show", "consignee", "phone", "province", "city", "area", "address",
		"pay_img", "pay_time", "buy_time", "confirm_time", "status", "coupon_settled",
		"created_at", "updated_at",
	})
	linkedIDs := map[int64]bool{}
	statusDist := map[int64]int{}
	resellDist := map[int64]int{}
	settled := 0
	sellerOut, buyerOut, merchMissing, remapped := 0, 0, 0, 0
	for _, r := range rows {
		buyer := asInt(r["buyer_id"])
		seller := asInt(r["seller_id"])
		if !l.tree[buyer] && !l.tree[seller] {
			continue
		}
		if !l.tree[buyer] {
			buyerOut++
		}
		if !l.tree[seller] {
			sellerOut++
		}
		st := asInt(r["status"])
		statusDist[st]++
		isResell := asInt(r["is_resell"])
		resellDist[isResell]++
		if st < 0 || st > 2 {
			l.st.badStatus++
		}
		settledFlag := 0
		if st == 2 {
			settledFlag = 1 // 老系统已结算，防止新系统重复结算
			settled++
		}
		mid := asInt(r["merchandise_id"])
		if !merchAll[mid] {
			// 老系统经历过多代商品ID，订单可能引用旧代ID → 映射到现役ID
			if nid, ok := oldToNew[mid]; ok && nid > 0 {
				mid = nid
				remapped++
			} else {
				merchMissing++
			}
		}
		linkedIDs[mid] = true
		oldID := r["old_id"]
		var oldIDStr string
		if oldID == nil || asStr(oldID) == "" {
			oldIDStr = "NULL"
		} else {
			oldIDStr = sqlInt(oldID)
		}
		w.add(
			strconv.FormatInt(asInt(r["id"]), 10),
			oldIDStr,
			sqlStr(r["order_sn"]),
			strconv.FormatInt(seller, 10),
			strconv.FormatInt(buyer, 10),
			strconv.FormatInt(mid, 10),
			l.asMoney(r["total_money"]),
			strconv.FormatInt(isResell, 10),
			sqlInt(r["is_show"]),
			sqlStr(r["consignee"]),
			sqlStr(r["phone"]),
			sqlStr(r["province"]),
			sqlStr(r["city"]),
			sqlStr(r["area"]),
			sqlStr(r["address"]),
			sqlStr(r["pay_img"]),
			sqlTime(r["pay_time"]),
			sqlTime(r["buy_time"]),
			sqlTime(r["confirm_time"]),
			strconv.FormatInt(st, 10),
			strconv.Itoa(settledFlag),
			sqlTimeRequired(r["created_at"]),
			sqlTimeRequired(r["updated_at"]),
		)
	}
	w.close()
	fmt.Printf("订单: %d 条, 状态分布 %v, is_resell分布 %v\n", w.count, statusDist, resellDist)
	fmt.Printf("  status=2 置 coupon_settled=1: %d 条\n", settled)
	fmt.Printf("  卖家在树外: %d, 买家在树外: %d\n", sellerOut, buyerOut)
	fmt.Printf("  商品ID旧代映射修正: %d, 商品彻底缺失(老系统同样缺失): %d\n", remapped, merchMissing)
	return linkedIDs
}

func (l *loader) importWithdraws(rows []map[string]interface{}) {
	accOut, _ := os.Create(outPath("05_withdraw_accounts.sql"))
	accW := newBatchWriter(bufio.NewWriter(accOut), "withdraw_accounts", []string{
		"id", "user_id", "username", "account", "account_type", "bank", "phone", "qrcode", "created_at", "updated_at",
	})
	out, _ := os.Create(outPath("06_withdraws.sql"))
	w := newBatchWriter(bufio.NewWriter(out), "withdraws", []string{
		"id", "transfer_no", "user_id", "cate", "account_type", "currency_type", "account_id",
		"money", "handling_fee", "actual_amount", "status", "remark", "created_at", "updated_at",
	})
	accSeen := map[int64]bool{}
	statusDist := map[int64]int{}
	cateDist := map[int64]int{}
	accCount := 0
	for _, r := range rows {
		uid := asInt(r["user_id"])
		if !l.tree[uid] {
			continue
		}
		cate := asInt(r["cate"])
		cateDist[cate]++
		currency := "money"
		switch cate {
		case 2:
			currency = "coupon"
		case 4:
			currency = "share_bonus"
		default:
			l.st.badCate++
		}
		oldStatus := asInt(r["status"])
		statusDist[oldStatus]++
		newStatus := int64(2) // 老0 → 新2 待处理
		switch oldStatus {
		case 0:
			newStatus = 2
		case 1:
			newStatus = 1
		case 2:
			newStatus = 3
		default:
			l.st.badStatus++
		}
		accType := asInt(r["account_type"])
		accID := asInt(r["account_id"])
		// account_info JSON → withdraw_accounts（去重）
		if raw := asStr(r["account_info"]); raw != "" {
			var ai struct {
				ID        int64       `json:"id"`
				UserID    int64       `json:"user_id"`
				Username  string      `json:"username"`
				Account   string      `json:"account"`
				Qrcode    interface{} `json:"qrcode"`
				Phone     string      `json:"phone"`
				CreatedAt string      `json:"created_at"`
				UpdatedAt string      `json:"updated_at"`
			}
			if err := json.Unmarshal([]byte(raw), &ai); err == nil && ai.ID > 0 && !accSeen[ai.ID] {
				accSeen[ai.ID] = true
				accW.add(
					strconv.FormatInt(ai.ID, 10),
					strconv.FormatInt(ai.UserID, 10),
					sqlStr(ai.Username),
					sqlStr(ai.Account),
					strconv.FormatInt(accType, 10),
					"NULL", // bank 支付宝无此字段
					sqlStr(ai.Phone),
					sqlStrNull(ai.Qrcode),
					sqlTimeRequired(ai.CreatedAt),
					sqlTimeRequired(ai.UpdatedAt),
				)
				accCount++
			} else if err != nil {
				l.st.badAccount++
			}
		}
		w.add(
			strconv.FormatInt(asInt(r["id"]), 10),
			sqlStr(r["transfer_no"]),
			strconv.FormatInt(uid, 10),
			strconv.FormatInt(cate, 10),
			strconv.FormatInt(accType, 10),
			"'"+currency+"'",
			strconv.FormatInt(accID, 10),
			l.asMoney(r["money"]),
			l.asMoney(r["handling_fee"]),
			l.asMoney(r["actual_amount"]),
			strconv.FormatInt(newStatus, 10),
			sqlStr(r["remark"]),
			sqlTimeRequired(r["created_at"]),
			sqlTimeRequired(r["updated_at"]),
		)
	}
	accW.close()
	w.close()
	fmt.Printf("提现: %d 条, 老状态分布 %v, cate分布 %v\n", w.count, statusDist, cateDist)
	fmt.Printf("收款账户: %d 个\n", accCount)
}

func (l *loader) importLogs(rows []map[string]interface{}, fileName, table string) {
	out, _ := os.Create(outPath(fileName))
	w := newBatchWriter(bufio.NewWriter(out), table, []string{
		"id", "user_id", "type", "money", "before", "after", "memo", "created_at", "updated_at",
	})
	for _, r := range rows {
		uid := asInt(r["user_id"])
		if !l.tree[uid] {
			continue
		}
		w.add(
			strconv.FormatInt(asInt(r["id"]), 10),
			strconv.FormatInt(uid, 10),
			sqlInt(r["type"]),
			l.asMoney(r["money"]),
			l.asMoney(r["before"]),
			l.asMoney(r["after"]),
			sqlStr(r["memo"]),
			sqlTimeRequired(r["created_at"]),
			sqlTimeRequired(r["updated_at"]),
		)
	}
	w.close()
	fmt.Printf("%s: %d 条\n", table, w.count)
}

// ---------- main ----------

var outDir string

func outPath(name string) string {
	return filepath.Join(outDir, name)
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("用法: import_tree.exe <fan_tree_ids.txt> <dataDir> <outDir>")
		os.Exit(1)
	}
	idsFile, dataDir := os.Args[1], os.Args[2]
	outDir = os.Args[3]
	os.MkdirAll(outDir, 0755)

	st := &stats{}
	l := &loader{st: st}

	// 粉丝树 ID
	idsRaw, err := os.ReadFile(idsFile)
	if err != nil {
		panic(err)
	}
	l.tree = map[int64]bool{}
	for _, line := range strings.Split(string(idsRaw), "\n") {
		var id int64
		fmt.Sscanf(strings.TrimSpace(line), "%d", &id)
		if id > 0 {
			l.tree[id] = true
		}
	}
	fmt.Printf("粉丝树人数: %d\n\n", len(l.tree))

	// 1. 商品表先读入建索引（订单需要校验关联商品是否存在 + 旧代ID映射）
	merchRows := loadJSON(filepath.Join(dataDir, "api.srdsmgs.com_app_admin_merchandise_select_FULL.json"))
	merchAll := map[int64]bool{}
	oldToNew := map[int64]int64{}
	for _, r := range merchRows {
		id := asInt(r["id"])
		merchAll[id] = true
		if oid := asInt(r["old_id"]); oid > 0 {
			oldToNew[oid] = id
		}
	}
	fmt.Printf("老系统商品总数: %d (含旧代ID映射 %d)\n\n", len(merchRows), len(oldToNew))

	// 2. 用户 + 钱包
	userRows := loadJSON(filepath.Join(dataDir, "api.srdsmgs.com_app_admin_user_select_FULL.json"))
	userFound := map[int64]bool{}
	for _, r := range userRows {
		userFound[asInt(r["id"])] = true
	}
	missing := 0
	for id := range l.tree {
		if !userFound[id] {
			missing++
		}
	}
	if missing > 0 {
		fmt.Printf("⚠ 树内 %d 人在老系统用户数据中不存在（可能是已注销用户）\n", missing)
	}
	walletOut, _ := os.Create(outPath("02_user_wallets.sql"))
	walletW := newBatchWriter(bufio.NewWriter(walletOut), "user_wallets", []string{
		"user_id", "money", "coupon", "self_bonus", "share_bonus", "score", "poor", "updated_at",
	})
	userOut, _ := os.Create(outPath("01_users.sql"))
	l.importUsers(userRows, walletW, userOut)
	walletW.close()
	walletOut.Close()

	// 3. 订单
	orderRows := loadJSON(filepath.Join(dataDir, "api.srdsmgs.com_app_admin_order_select_FULL.json"))
	linkedIDs := l.importOrders(orderRows, merchAll, oldToNew)

	// 4. 商品（树内全部 + 订单关联补充）
	l.importMerch(merchRows, linkedIDs)

	// 5. 提现 + 收款账户
	withdrawRows := loadJSON(filepath.Join(dataDir, "api.srdsmgs.com_app_admin_withdraw_select_FULL.json"))
	l.importWithdraws(withdrawRows)

	// 6. 财务明细（coupon_logs 用 00:10 重爬后的最终数据）
	selfRows := loadJSON(filepath.Join(dataDir, "api.srdsmgs.com_app_admin_selfbonus-log_select_FULL.json"))
	l.importLogs(selfRows, "07_self_bonus_logs.sql", "self_bonus_logs")
	shareRows := loadJSON(filepath.Join(dataDir, "api.srdsmgs.com_app_admin_sharebonus-log_select_FULL.json"))
	l.importLogs(shareRows, "08_share_bonus_logs.sql", "share_bonus_logs")
	couponRows := loadJSON(filepath.Join(dataDir, "api.srdsmgs.com_app_admin_coupon-log_select_FULL.json"))
	l.importLogs(couponRows, "09_coupon_logs.sql", "coupon_logs")

	// 最后一个文件恢复外键检查
	f, _ := os.OpenFile(outPath("09_coupon_logs.sql"), os.O_APPEND|os.O_WRONLY, 0644)
	fmt.Fprintln(f, "SET FOREIGN_KEY_CHECKS=1;")
	f.Close()

	st.report()
}
