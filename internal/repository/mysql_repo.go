package repository

import (
	"fmt"
	"strings"

	"commercial-transactions-service/internal/model"

	"gorm.io/gorm"
)

// ─── 用户相关 ───

func GetUserByID(id int64) (*model.User, error) {
	var u model.User
	err := DB.Where("id = ?", id).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func GetUserByUsername(username string) (*model.User, error) {
	var u model.User
	err := DB.Where("username = ? OR mobile = ?", username, username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func GetUserByInviteCode(code string) (*model.User, error) {
	var u model.User
	err := DB.Where("invite = ?", code).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func CreateUser(u *model.User) error {
	return DB.Create(u).Error
}

func GetUserWallet(userID int64) (*model.UserWallet, error) {
	// 从日志表取最新余额（与管理端一致）
	subCoupon := "(SELECT `after` FROM coupon_logs WHERE user_id = ? ORDER BY id DESC LIMIT 1)"
	subSelf := "(SELECT `after` FROM self_bonus_logs WHERE user_id = ? ORDER BY id DESC LIMIT 1)"
	subShare := "(SELECT `after` FROM share_bonus_logs WHERE user_id = ? ORDER BY id DESC LIMIT 1)"

	var w model.UserWallet
	err := DB.Raw(`
		SELECT COALESCE((`+subCoupon+`), w.coupon, 0) as coupon,
		       COALESCE((`+subSelf+`), w.self_bonus, 0) as self_bonus,
		       COALESCE((`+subShare+`), w.share_bonus, 0) as share_bonus,
		       COALESCE(w.money,0) as money, COALESCE(w.score,0) as score, COALESCE(w.poor,0) as poor,
		       w.user_id, w.updated_at
		FROM user_wallets w WHERE w.user_id = ?`,
		userID, userID, userID, userID).Scan(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

type UserWithWallet struct {
	model.User
	Money      float64 `json:"money"`
	Coupon     float64 `json:"coupon"`
	SelfBonus  float64 `json:"self_bonus"`
	ShareBonus float64 `json:"share_bonus"`
	Score      int     `json:"score"`
	Poor       float64 `json:"poor"`
}

func ListUsers(req model.UserListReq) ([]UserWithWallet, int64, error) {
	var users []UserWithWallet
	var count int64

	// 资金字段从日志表实时取最新余额（和老系统前端一致）
	subCoupon := "(SELECT cl.`after` FROM coupon_logs cl WHERE cl.user_id = u.id ORDER BY cl.id DESC LIMIT 1)"
	subSelf := "(SELECT sl.`after` FROM self_bonus_logs sl WHERE sl.user_id = u.id ORDER BY sl.id DESC LIMIT 1)"
	subShare := "(SELECT shl.`after` FROM share_bonus_logs shl WHERE shl.user_id = u.id ORDER BY shl.id DESC LIMIT 1)"

	// 今日/昨日买卖从订单表实时算
	subTodayBuy := "(SELECT COALESCE(SUM(o.total_money),0) FROM orders o WHERE o.buyer_id = u.id AND DATE(o.buy_time) = CURDATE())"
	subTodayBuyCnt := "(SELECT COUNT(*) FROM orders o WHERE o.buyer_id = u.id AND DATE(o.buy_time) = CURDATE())"
	subTodaySell := "(SELECT COALESCE(SUM(o.total_money),0) FROM orders o WHERE o.seller_id = u.id AND DATE(o.buy_time) = CURDATE())"
	subYestSellCnt := "(SELECT COUNT(*) FROM orders o WHERE o.seller_id = u.id AND DATE(o.buy_time) = DATE_SUB(CURDATE(), INTERVAL 1 DAY))"

	selectSQL := fmt.Sprintf(`u.*,
		COALESCE(w.money,0) money,
		COALESCE((%s), w.coupon, 0) coupon,
		COALESCE((%s), w.self_bonus, 0) self_bonus,
		COALESCE((%s), w.share_bonus, 0) share_bonus,
		COALESCE(w.score,0) score,
		COALESCE(w.poor,0) poor,
		COALESCE((%s), u.today_buy_total) today_buy_total,
		COALESCE((%s), u.today_buy_count) today_buy_count,
		COALESCE((%s), u.today_sell_total) today_sell_total,
		COALESCE((%s), u.yesterday_sell_count) yesterday_sell_count`,
		subCoupon, subSelf, subShare, subTodayBuy, subTodayBuyCnt, subTodaySell, subYestSellCnt)

	db := DB.Table("users u").
		Select(selectSQL).
		Joins("LEFT JOIN user_wallets w ON u.id = w.user_id")
	if req.UserID != "" {
		ids := parseInts(req.UserID)
		if len(ids) == 1 {
			db = db.Where("u.id = ?", ids[0])
		} else if len(ids) > 1 {
			db = db.Where("u.id IN ?", ids)
		}
	}
	if req.Mobile != "" {
		mobiles := splitStr(req.Mobile)
		if len(mobiles) == 1 {
			db = db.Where("u.mobile = ?", mobiles[0])
		} else if len(mobiles) > 1 {
			db = db.Where("u.mobile IN ?", mobiles)
		}
	}
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		db = db.Where("u.username LIKE ? OR u.nickname LIKE ? OR u.mobile LIKE ?", kw, kw, kw)
	}
	if req.Level != nil {
		db = db.Where("u.level = ?", *req.Level)
	}
	if req.Status != nil {
		db = db.Where("u.status = ?", *req.Status)
	}
	if req.PID != "" {
		ids := parseInts(req.PID)
		if len(ids) == 1 {
			db = db.Where("u.pid = ?", ids[0])
		} else if len(ids) > 1 {
			db = db.Where("u.pid IN ?", ids)
		}
	}
	db.Count(&count)
	err := db.Order("u.id DESC").Offset((req.Page - 1) * req.Limit).Limit(req.Limit).Find(&users).Error
	return users, count, err
}

// ─── 订单相关 ───

func ListOrders(req model.OrderListReq) ([]model.Order, int64, error) {
	var orders []model.Order
	var count int64
	db := DB.Model(&model.Order{})
	if req.Status != nil {
		db = db.Where("status = ?", *req.Status)
	}
	if req.SellerID != nil {
		db = db.Where("seller_id = ?", *req.SellerID)
	}
	if req.BuyerID != nil {
		db = db.Where("buyer_id = ?", *req.BuyerID)
	}
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		db = db.Where("order_sn LIKE ? OR consignee LIKE ? OR phone LIKE ?", kw, kw, kw)
	}
	db.Count(&count)
	err := db.Order("id DESC").Offset((req.Page-1)*req.Limit).Limit(req.Limit).Find(&orders).Error
	return orders, count, err
}

type OrderDetail struct {
	model.Order
	MerchandiseTitle string `json:"merchandise_title"`
	MerchandiseImage string `json:"merchandise_image"`
}

func GetOrderByID(id int64) (*OrderDetail, error) {
	var o OrderDetail
	err := DB.Table("orders o").
		Select("o.*, m.title as merchandise_title, m.image as merchandise_image").
		Joins("LEFT JOIN merchandises m ON o.merchandise_id = m.id").
		Where("o.id = ?", id).First(&o).Error
	return &o, err
}

func UpdateOrderStatus(id int64, status int8) error {
	return DB.Model(&model.Order{}).Where("id = ?", id).Update("status", status).Error
}

// ─── 商品相关 ───

type GoodWithCategory struct {
	model.Good
	CategoryName string `json:"category_name"`
}

func ListGoods(page, limit int, categoryID *int64, keyword string) ([]GoodWithCategory, int64, error) {
	var goods []GoodWithCategory
	var count int64

	base := DB.Table("goods g").Select("g.*, c.title as category_name").
		Joins("LEFT JOIN categories c ON g.category_id = c.id").
		Where("g.status = 1")

	if categoryID != nil && *categoryID > 0 {
		base = base.Where("g.category_id = ?", *categoryID)
	}
	if keyword != "" {
		base = base.Where("g.title LIKE ?", "%"+keyword+"%")
	}

	base.Count(&count)
	err := base.Order("g.id DESC").Offset((page - 1) * limit).Limit(limit).Find(&goods).Error
	return goods, count, err
}

func GetGoodByID(id int64) (*model.Good, error) {
	var g model.Good
	err := DB.Where("id = ?", id).First(&g).Error
	return &g, err
}

func GetGoodsByIDs(ids []int64) map[int64]*model.Good {
	if len(ids) == 0 { return map[int64]*model.Good{} }
	var goods []model.Good
	DB.Where("id IN ?", ids).Find(&goods)
	result := make(map[int64]*model.Good, len(goods))
	for i := range goods {
		result[goods[i].ID] = &goods[i]
	}
	return result
}

func CreateGood(g *model.Good) error { return DB.Create(g).Error }
func UpdateGood(g *model.Good) error { return DB.Save(g).Error }

// ─── 提现相关 ───

func ListWithdraws(page, limit int, status *int8) ([]model.Withdraw, int64, error) {
	var list []model.Withdraw
	var count int64
	db := DB.Model(&model.Withdraw{})
	if status != nil {
		db = db.Where("status = ?", *status)
	}
	db.Count(&count)
	err := db.Order("id DESC").Offset((page-1)*limit).Limit(limit).Find(&list).Error
	return list, count, err
}

func GetWithdrawByID(id int64) (*model.Withdraw, error) {
	var w model.Withdraw
	err := DB.Where("id = ?", id).First(&w).Error
	return &w, err
}

func UpdateWithdrawStatus(id int64, status int8) error {
	return DB.Model(&model.Withdraw{}).Where("id = ?", id).Update("status", status).Error
}

// ─── 日志相关 ───

func ListCouponLogs(req model.LogListReq) ([]model.CouponLog, int64, error) {
	return listLogs[model.CouponLog](DB, req)
}
func ListSelfBonusLogs(req model.LogListReq) ([]model.SelfBonusLog, int64, error) {
	return listLogs[model.SelfBonusLog](DB, req)
}
func ListShareBonusLogs(req model.LogListReq) ([]model.ShareBonusLog, int64, error) {
	return listLogs[model.ShareBonusLog](DB, req)
}

func listLogs[T any](db *gorm.DB, req model.LogListReq) ([]T, int64, error) {
	var list []T
	var count int64
	q := db.Model(new(T))
	if req.UserID != nil {
		q = q.Where("user_id = ?", *req.UserID)
	}
	if req.Type != nil {
		q = q.Where("type = ?", *req.Type)
	}
	q.Count(&count)
	err := q.Order("id DESC").Offset((req.Page - 1) * req.Limit).Limit(req.Limit).Find(&list).Error
	return list, count, err
}

// ─── 批量删除 ───

func BatchDeleteUsers(ids []int64) error {
	return DB.Where("id IN ?", ids).Delete(&model.User{}).Error
}

// ─── 辅助 ───

func parseInts(s string) []int64 {
	parts := splitStr(s)
	var ids []int64
	for _, p := range parts {
		var id int64
		fmt.Sscanf(p, "%d", &id)
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func splitStr(s string) []string {
	s = strings.NewReplacer(" ", ",", "\t", ",").Replace(s)
	var result []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ─── 管理员 ───

func GetAdminByUsername(username string) (*model.AdminUser, error) {
	var a model.AdminUser
	err := DB.Where("username = ?", username).First(&a).Error
	return &a, err
}

// ─── 菜单规则 ───

func ListRules() ([]model.Rule, error) {
	var rules []model.Rule
	err := DB.Order("weight DESC").Find(&rules).Error
	return rules, err
}

func BuildRuleTree() ([]*model.Rule, error) {
	rules, err := ListRules()
	if err != nil {
		return nil, err
	}
	ruleMap := make(map[int64]*model.Rule)
	var roots []*model.Rule
	for i := range rules {
		ruleMap[rules[i].ID] = &rules[i]
	}
	for i := range rules {
		r := ruleMap[rules[i].ID]
		if r.PID == 0 {
			roots = append(roots, r)
		} else if parent, ok := ruleMap[r.PID]; ok {
			parent.Children = append(parent.Children, r)
		}
	}
	return roots, nil
}

// ─── FlashSale ───

func CountUserTodayOrders(userID int64) int64 {
	var count int64
	DB.Model(&model.FlashSaleRecord{}).
		Where("user_id = ? AND DATE(created_at) = CURDATE()", userID).
		Count(&count)
	return count
}

func ListFlashSaleEvents() ([]model.FlashSaleEvent, error) {
	var events []model.FlashSaleEvent
	err := DB.Where("status IN (0,1)").Order("id DESC").Find(&events).Error
	return events, err
}

func GetFlashSaleEventByProductID(productID int64) (*model.FlashSaleEvent, error) {
	var e model.FlashSaleEvent
	err := DB.Where("product_id = ? AND status IN (0,1)", productID).First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func CreateFlashSaleEvent(e *model.FlashSaleEvent) error { return DB.Create(e).Error }
func UpdateFlashSaleEvent(e *model.FlashSaleEvent) error { return DB.Save(e).Error }

func CreateFlashSaleRecord(r *model.FlashSaleRecord) error { return DB.Create(r).Error }

func BatchCreateFlashSaleRecords(records []model.FlashSaleRecord) error {
	if len(records) == 0 {
		return nil
	}
	return DB.CreateInBatches(records, 100).Error
}

// ─── Dashboard ───

type DashboardStats struct {
	TodayUsers     int64   `json:"today_users"`
	TodayOrders    int64   `json:"today_orders"`
	TodaySales     float64 `json:"today_sales"`
	PendingWithdraw int64  `json:"pending_withdraw"`
	TotalUsers     int64   `json:"total_users"`
	TotalOrders    int64   `json:"total_orders"`
}

func GetDashboardStats() (*DashboardStats, error) {
	var s DashboardStats
	DB.Model(&model.User{}).Where("DATE(created_at) = CURDATE()").Count(&s.TodayUsers)
	DB.Model(&model.Order{}).Where("DATE(created_at) = CURDATE()").Count(&s.TodayOrders)
	DB.Model(&model.Order{}).Where("DATE(created_at) = CURDATE()").Select("COALESCE(SUM(total_money),0)").Scan(&s.TodaySales)
	DB.Model(&model.Withdraw{}).Where("status = 2").Count(&s.PendingWithdraw)
	DB.Model(&model.User{}).Count(&s.TotalUsers)
	DB.Model(&model.Order{}).Count(&s.TotalOrders)
	return &s, nil
}
