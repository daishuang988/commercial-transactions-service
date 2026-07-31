package front

import (
	"fmt"
	"strings"
	"time"

	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MyOrders 我的订单 GET /api/v1/front/orders?role=buyer|seller&status=0|1|2&page=1&limit=10
func MyOrders(c *gin.Context) {
	uid := c.GetInt64("user_id")
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)
	req := model.OrderListReq{Page: page, Limit: limit}
	role := c.Query("role")
	if role == "seller" {
		req.SellerID = &uid
	} else {
		req.BuyerID = &uid
	}
	if s := c.Query("status"); s != "" {
		var st int8
		fmt.Sscanf(s, "%d", &st)
		req.Status = &st
	}
	orders, count, err := repository.ListOrders(req)
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	app.OKWithCount(c, orders, count)
}

// MyOrderDetail 订单详情 GET /api/v1/front/orders/:id
func MyOrderDetail(c *gin.Context) {
	id := parseIntParam(c, "id")
	uid := c.GetInt64("user_id")
	o, err := repository.GetOrderByID(id)
	if err != nil || (o.BuyerID != uid && o.SellerID != uid) {
		app.NotFound(c, "订单不存在")
		return
	}
	app.OK(c, o)
}

// PayOrderReq 付款请求
type PayOrderReq struct {
	Consignee string `json:"consignee"`
	Phone     string `json:"phone"`
	Province  string `json:"province"`
	City      string `json:"city"`
	Area      string `json:"area"`
	Address   string `json:"address"`
	PayImg    string `json:"pay_img"`
}

// PayOrder 买方上传付款凭证 POST /api/v1/front/orders/:id/pay
func PayOrder(c *gin.Context) {
	id := parseIntParam(c, "id")
	uid := c.GetInt64("user_id")
	o, err := repository.GetOrderByID(id)
	if err != nil || o.BuyerID != uid {
		app.NotFound(c, "订单不存在")
		return
	}
	if o.Status != 0 {
		app.BadRequest(c, "订单状态不允许此操作")
		return
	}
	var req PayOrderReq
	c.ShouldBindJSON(&req)
	now := time.Now()
	updates := map[string]interface{}{
		"status": int8(1), "pay_time": now, "updated_at": now,
	}
	if req.Consignee != "" { updates["consignee"] = req.Consignee }
	if req.Phone != ""     { updates["phone"] = req.Phone }
	if req.Province != ""  { updates["province"] = req.Province }
	if req.City != ""      { updates["city"] = req.City }
	if req.Area != ""      { updates["area"] = req.Area }
	if req.Address != ""   { updates["address"] = req.Address }
	if req.PayImg != ""    { updates["pay_img"] = req.PayImg }
	repository.DB.Model(&model.Order{}).Where("id = ?", id).Updates(updates)
	app.OK(c, gin.H{"msg": "付款凭证已提交"})
}

// ConfirmOrder 卖方确认收款 + 触发结算分佣
func ConfirmOrder(c *gin.Context) {
	id := parseIntParam(c, "id")
	uid := c.GetInt64("user_id")
	o, err := repository.GetOrderByID(id)
	if err != nil || o.SellerID != uid {
		app.NotFound(c, "订单不存在")
		return
	}
	if o.Status != 1 {
		app.BadRequest(c, "订单状态不允许此操作")
		return
	}

	now := time.Now()
	repository.DB.Model(&model.Order{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       int8(2),
		"confirm_time": now,
		"updated_at":   now,
	})

	// 结算分佣（同步）
	settleOrder(o)

	app.OK(c, gin.H{"msg": "已确认收款"})
}

// settleOrder 订单结算分佣（即时部分 - 老系统规则）
// 静态收益 1% → 买家个人奖金（即时）
// 直推收益 0.2% → 买家直接上级推广奖金（即时）
// 店长收益 1% → 买家上级中 level>=2 推广奖金（即时）
// 抢单奖励 0.5% → 买家优惠券（凌晨结算，见 SettleDailyCoupons）
func settleOrder(o *repository.OrderDetail) {
	amount := o.TotalMoney
	if amount <= 0 { return }

	staticRate := parseRate(repository.GetConfigStr("static_income_rate"))
	directRate := parseRate(repository.GetConfigStr("direct_referral_rate"))
	managerRate := parseRate(repository.GetConfigStr("store_manager_rate"))

	// 1. 静态收益 → 买家个人奖金（即时）
	if staticRate > 0 {
		addSelfBonus(o.BuyerID, amount*staticRate, "今日收益")
	}

	// 2. 直推收益 → 买家直接上级推广奖金（即时）
	if directRate > 0 {
		if parent := getParentID(o.BuyerID); parent > 0 {
			addShareBonus(parent, amount*directRate, "直推收益")
		}
	}

	// 3. 店长收益 → 买家上级中 level>=2 推广奖金（即时）
	if managerRate > 0 {
		parentID := getParentID(o.BuyerID)
		if storeMgr := getStoreManagerID(o.BuyerID); storeMgr > 0 && storeMgr != parentID {
			addShareBonus(storeMgr, amount*managerRate, "店长收益")
		}
	}
}

// SettleDailyCoupons 凌晨结算当日优惠券（抢单奖励 = 订单金额 × order_reward_rate → 买家）
func SettleDailyCoupons() {
	orderRewardRate := parseRate(repository.GetConfigStr("order_reward_rate"))
	if orderRewardRate <= 0 { return }

	var orders []model.Order
	repository.DB.Where("status = 2 AND coupon_settled = 0").Find(&orders)
	for _, o := range orders {
		addCoupon(o.BuyerID, o.TotalMoney*orderRewardRate, "今日收益")
		repository.DB.Model(&o).Update("coupon_settled", int8(1))
	}
}

// ResellOrder 寄卖 POST /api/v1/front/orders/:id/resell
// 校验: 买方 + status=2 + 未超过寄卖截止时间
func ResellOrder(c *gin.Context) {
	id := parseIntParam(c, "id")
	uid := c.GetInt64("user_id")
	o, err := repository.GetOrderByID(id)
	if err != nil || o.BuyerID != uid {
		app.NotFound(c, "订单不存在")
		return
	}
	if o.Status != 2 {
		app.BadRequest(c, "只有已完成订单才能寄卖")
		return
	}

	// 寄卖窗口检查（格式: "14:45-00:00"，支持跨天，结束<开始表示结束在次日）
	deadlineStr := repository.GetConfigStr("resell_deadline")
	if deadlineStr != "" {
		var sh, sm, eh, em int
		if n, _ := fmt.Sscanf(deadlineStr, "%d:%d-%d:%d", &sh, &sm, &eh, &em); n == 4 {
			now := time.Now().In(repository.CSTLocation())
			start := time.Date(now.Year(), now.Month(), now.Day(), sh, sm, 0, 0, repository.CSTLocation())
			end := time.Date(now.Year(), now.Month(), now.Day(), eh, em, 0, 0, repository.CSTLocation())
			if eh*60+em <= sh*60+sm {
				end = end.Add(24 * time.Hour) // 跨天
			}
			if now.Before(start) || !now.Before(end) {
				app.BadRequest(c, fmt.Sprintf("寄卖窗口为 %s，当前不可寄卖", deadlineStr))
				return
			}
		}
	}

	// TODO: 上架费暂时跳过
	// 寄卖价格 = 原价 × (1 + 增值比例)
	resellRate := parseRate(repository.GetConfigStr("resell_rate"))
	resellPrice := o.TotalMoney * (1 + resellRate)

	// 拆单规则: 达到阈值自动拆分为N单
	splitCount := 1
	splitPrice := resellPrice
	if splitStr := repository.GetConfigStr("split_threshold"); splitStr != "" {
		parts := strings.SplitN(splitStr, ",", 2)
		if len(parts) == 2 {
			var threshold float64
			var count int
			fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &threshold)
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &count)
			if resellPrice >= threshold && count > 1 {
				splitCount = count
				splitPrice = resellPrice / float64(count)
			}
		}
	}

	// 创建寄售商品
	productID := int64(1)
	if pid := repository.GetConfigStr("resell_product_id"); pid != "" {
		fmt.Sscanf(pid, "%d", &productID)
	}
	now := time.Now()
	for i := 0; i < splitCount; i++ {
		repository.DB.Exec(
			"INSERT INTO merchandises (user_id,title,image,price,is_show,status,created_at,updated_at) VALUES(?,?,?,?,1,0,?,?)",
			uid, o.MerchandiseTitle, o.MerchandiseImage, splitPrice, now, now)
	}

	// 生成兑换订单
	repository.DB.Exec("INSERT INTO exchange_orders (user_id,order_sn,total_money,status,created_at,updated_at) VALUES(?,?,?,1,NOW(),NOW())", uid, o.OrderSN, resellPrice)

	// 标记订单已寄卖
	repository.DB.Model(&model.Order{}).Where("id = ?", id).Update("is_resell", int8(1))
	app.OK(c, gin.H{"msg": "寄卖申请已提交"})
}

func parseRate(s string) float64 {
	if s == "" { return 0 }
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func getParentID(userID int64) int64 {
	u, err := repository.GetUserByID(userID)
	if err != nil || u.PID <= 0 { return 0 }
	return u.PID
}

func getStoreManagerID(userID int64) int64 {
	u, err := repository.GetUserByID(userID)
	if err != nil { return 0 }
	// 沿 pid 链向上找 level>=2 的店长
	currentID := u.PID
	for i := 0; i < 10; i++ { // 最多找10层
		if currentID <= 0 { break }
		p, err := repository.GetUserByID(currentID)
		if err != nil { break }
		if p.Level >= 2 { return p.ID }
		currentID = p.PID
	}
	return 0
}

func addCoupon(userID int64, money float64, memo string) {
	repository.DB.Transaction(func(tx *gorm.DB) error {
		before := getWalletBalance(tx, userID, "coupon")
		after := before + money
		if err := tx.Exec(
			"INSERT INTO coupon_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,1,?,?,?,?,NOW(),NOW())",
			userID, money, before, after, memo).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE user_wallets SET coupon = ?, updated_at = NOW() WHERE user_id = ?", after, userID).Error
	})
}

func addSelfBonus(userID int64, money float64, memo string) {
	repository.DB.Transaction(func(tx *gorm.DB) error {
		before := getWalletBalance(tx, userID, "self_bonus")
		after := before + money
		if err := tx.Exec(
			"INSERT INTO self_bonus_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,1,?,?,?,?,NOW(),NOW())",
			userID, money, before, after, memo).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE user_wallets SET self_bonus = ?, updated_at = NOW() WHERE user_id = ?", after, userID).Error
	})
}

func addShareBonus(userID int64, money float64, memo string) {
	repository.DB.Transaction(func(tx *gorm.DB) error {
		before := getWalletBalance(tx, userID, "share_bonus")
		after := before + money
		if err := tx.Exec(
			"INSERT INTO share_bonus_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,1,?,?,?,?,NOW(),NOW())",
			userID, money, before, after, memo).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE user_wallets SET share_bonus = ?, updated_at = NOW() WHERE user_id = ?", after, userID).Error
	})
}

func deductCoupon(userID int64, money float64, memo string) {
	repository.DB.Transaction(func(tx *gorm.DB) error {
		before := getWalletBalance(tx, userID, "coupon")
		after := before - money
		if err := tx.Exec(
			"INSERT INTO coupon_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,2,?,?,?,?,NOW(),NOW())",
			userID, money, before, after, memo).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE user_wallets SET coupon = ?, updated_at = NOW() WHERE user_id = ?", after, userID).Error
	})
}

func getWalletBalance(tx *gorm.DB, userID int64, field string) float64 {
	var v float64
	tx.Raw("SELECT COALESCE("+field+", 0) FROM user_wallets WHERE user_id = ?", userID).Scan(&v)
	return v
}

// CancelOrder 取消订单 POST /api/v1/front/orders/:id/cancel
// 仅 status=0(待付款) 可取消：订单→已取消(3)，寄售商品→回滚待售
func CancelOrder(c *gin.Context) {
	id := parseIntParam(c, "id")
	uid := c.GetInt64("user_id")

	o, err := repository.GetOrderByID(id)
	if err != nil || o.BuyerID != uid {
		app.NotFound(c, "订单不存在")
		return
	}
	if o.Status != 0 {
		app.BadRequest(c, "只有待付款订单才能取消")
		return
	}

	now := time.Now()

	// 订单→已取消
	repository.DB.Model(&model.Order{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": int8(3), "updated_at": now,
	})

	// 寄售商品→回滚待售
	repository.DB.Model(&model.Merchandise{}).Where("id = ?", o.MerchandiseID).Updates(map[string]interface{}{
		"status": int8(0), "is_show": int8(1), "updated_at": now,
	})

	// 还原当日抢购计数
	todayKey := fmt.Sprintf("flash:user:daily:%d:%s", uid, now.Format("20060102"))
	repository.RDB.Decr(c.Request.Context(), todayKey)

	app.OK(c, gin.H{"msg": "订单已取消"})
}
