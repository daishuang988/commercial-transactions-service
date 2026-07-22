package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"
	"commercial-transactions-service/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ─── Dashboard ───

// Dashboard 数据看板 GET /api/v1/admin/dashboard
func Dashboard(c *gin.Context) {
	stats, err := repository.GetDashboardStats()
	if err != nil {
		app.InternalError(c, "获取失败")
		return
	}
	app.OK(c, stats)
}

// ─── 用户管理 ───

// ListUsers 用户列表 GET /api/v1/admin/users
func ListUsers(c *gin.Context) {
	var req model.UserListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	users, count, err := repository.ListUsers(req)
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	if users == nil {
		users = []repository.UserWithWallet{}
	}
	app.OKWithCount(c, users, count)
}

// GetUser 用户详情 GET /api/v1/admin/users/:id
func GetUser(c *gin.Context) {
	id := parseIntParam(c, "id")
	u, err := repository.GetUserByID(id)
	if err != nil {
		app.NotFound(c, "用户不存在")
		return
	}
	w, _ := repository.GetUserWallet(id)
	app.OK(c, gin.H{"user": u, "wallet": w})
}

// BatchDeleteUsers 批量删除 POST /api/v1/admin/users/batch-delete
func BatchDeleteUsers(c *gin.Context) {
	var req model.BatchDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		app.BadRequest(c, "请选择要删除的用户")
		return
	}
	if err := repository.BatchDeleteUsers(req.IDs); err != nil {
		app.InternalError(c, "删除失败")
		return
	}
	app.OK(c, nil)
}

// ============ 分类管理 ============

// ListCategories GET /api/v1/admin/categories
func ListCategories(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 20)
	var list []map[string]interface{}
	var count int64
	repository.DB.Table("categories").Count(&count)
	repository.DB.Table("categories").Select("id,title as name,sort,status,updated_at").Order("sort ASC,id ASC").Offset((page - 1) * limit).Limit(limit).Find(&list)
	app.OKWithCount(c, list, count)
}

// CreateCategory POST /api/v1/admin/categories
func CreateCategory(c *gin.Context) {
	var req struct {
		Name   string `json:"name" binding:"required"`
		Sort   int    `json:"sort"`
		Status int8   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "分类名称必填")
		return
	}
	// 重名检查
	var exists int64
	repository.DB.Table("categories").Where("title = ?", req.Name).Count(&exists)
	if exists > 0 {
		app.BadRequest(c, "分类名称已存在")
		return
	}
	now := time.Now()
	repository.DB.Exec("INSERT INTO categories (title,sort,status,created_at,updated_at) VALUES(?,?,?,?,?)",
		req.Name, req.Sort, req.Status, now, now)
	app.OK(c, nil)
}

// UpdateCategory PUT /api/v1/admin/categories/:id
func UpdateCategory(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if v, ok := req["name"]; ok {
		// 重名检查（排除自身）
		var exists int64
		name := fmt.Sprintf("%v", v)
		repository.DB.Table("categories").Where("title = ? AND id != ?", name, id).Count(&exists)
		if exists > 0 {
			app.BadRequest(c, "分类名称已存在")
			return
		}
		updates["title"] = name
	}
	if v, ok := req["sort"]; ok {
		updates["sort"] = v
	}
	if v, ok := req["status"]; ok {
		updates["status"] = v
	}
	repository.DB.Table("categories").Where("id = ?", id).Updates(updates)
	app.OK(c, nil)
}

// UploadImage 图片上传 POST /api/v1/admin/upload
func UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		app.BadRequest(c, "请选择文件")
		return
	}
	if file.Size > 10*1024*1024 {
		app.BadRequest(c, "文件大小不能超过10MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true, ".svg": true}
	if !allowed[ext] {
		app.BadRequest(c, "不支持的图片格式，仅支持 jpg/png/gif/webp/bmp/svg")
		return
	}
	filename := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), utils.RandStr(8), ext)
	savePath := filepath.Join("upload", "image", time.Now().Format("20060102"), filename)
	fullPath := filepath.Join(".", savePath)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		app.InternalError(c, "上传失败")
		return
	}
	app.OK(c, gin.H{"url": "/" + filepath.ToSlash(savePath)})
}

// DeleteCategory DELETE /api/v1/admin/categories/:id
func DeleteCategory(c *gin.Context) {
	id := parseIntParam(c, "id")
	repository.DB.Table("categories").Where("id = ?", id).Delete(nil)
	app.OK(c, nil)
}

// UpdateUserReq 编辑用户请求
type UpdateUserReq struct {
	Nickname    string  `json:"nickname"`
	Mobile      string  `json:"mobile"`
	Level       *int    `json:"level"`       // 0普通用户 1推荐人 2店长 3二代店长
	MaxOrder    *int    `json:"max_order"`   // 当天可抢单数
	Viptime     *string `json:"viptime"`     // VIP截止时间，空=不更新
	Password    string  `json:"password"`    // 不更新请留空
	IsPriority  *int8   `json:"is_priority"` // 是否优先抢购 0否 1是
	IsResell    *int8   `json:"is_resell"`   // 是否可以寄卖 0否 1是
	Status      *int8   `json:"status"`      // 0冻结 1正常
}

// UpdateUser PUT /api/v1/admin/users/:id
func UpdateUser(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Mobile != "" {
		updates["mobile"] = req.Mobile
	}
	if req.Level != nil {
		if *req.Level < 0 || *req.Level > 3 {
			app.BadRequest(c, "用户等级仅支持 0普通用户 1推荐人 2店长 3二代店长")
			return
		}
		updates["level"] = *req.Level
	}
	if req.MaxOrder != nil {
		if *req.MaxOrder < 0 {
			app.BadRequest(c, "可抢单数不能为负数")
			return
		}
		updates["max_order"] = *req.MaxOrder
	}
	if req.Viptime != nil {
		if *req.Viptime == "" {
			updates["viptime"] = nil
		} else {
			t, err := time.Parse("2006-01-02 15:04:05", *req.Viptime)
			if err != nil {
				app.BadRequest(c, "VIP时间格式错误，应为 2026-01-01 00:00:00")
				return
			}
			updates["viptime"] = t
		}
	}
	if req.Password != "" {
		if len(req.Password) < 6 {
			app.BadRequest(c, "密码至少6位")
			return
		}
		salt := utils.RandStr(6)
		updates["password"] = utils.MD5Hash(req.Password + salt)
		updates["salt"] = salt
	}
	if req.IsPriority != nil {
		updates["is_priority"] = *req.IsPriority
	}
	if req.IsResell != nil {
		updates["is_resell"] = *req.IsResell
	}
	if req.Status != nil {
		if *req.Status != 0 && *req.Status != 1 {
			app.BadRequest(c, "状态仅支持 0冻结 1正常")
			return
		}
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		app.BadRequest(c, "没有要更新的字段")
		return
	}
	updates["updated_at"] = time.Now()

	if err := repository.DB.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		app.InternalError(c, "更新失败")
		return
	}
	app.OK(c, nil)
}

// RechargeReq 充值请求
type RechargeReq struct {
	Currency string  `json:"currency" binding:"required"` // money/coupon/self_bonus/share_bonus
	OpType   string  `json:"op_type" binding:"required"`  // add/sub
	Amount   float64 `json:"amount" binding:"required"`
	Desc     string  `json:"desc"`
}

// Recharge 充值/扣减 POST /api/v1/admin/users/:id/recharge
func Recharge(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req RechargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	// 校验货币类型
	allowedCurrencies := map[string]string{
		"money": "余额", "coupon": "优惠券", "self_bonus": "个人奖金", "share_bonus": "推广奖金",
	}
	cnName, ok := allowedCurrencies[req.Currency]
	if !ok {
		app.BadRequest(c, "无效的货币类型")
		return
	}

	// 校验操作类型
	if req.OpType != "add" && req.OpType != "sub" {
		app.BadRequest(c, "操作类型仅支持 add(增加) 或 sub(减少)")
		return
	}

	// 校验金额
	if req.Amount <= 0 {
		app.BadRequest(c, "金额必须大于0")
		return
	}

	// 校验描述长度
	if len([]rune(req.Desc)) > 200 {
		app.BadRequest(c, "描述不能超过200字符")
		return
	}

	// 查当前余额
	wallet, err := repository.GetUserWallet(id)
	if err != nil {
		app.NotFound(c, "用户钱包不存在")
		return
	}

	var currentBalance float64
	switch req.Currency {
	case "money":
		currentBalance = wallet.Money
	case "coupon":
		currentBalance = wallet.Coupon
	case "self_bonus":
		currentBalance = wallet.SelfBonus
	case "share_bonus":
		currentBalance = wallet.ShareBonus
	}

	// 减少时不允许小于0
	if req.OpType == "sub" {
		if currentBalance <= 0 {
			app.Fail(c, app.ErrCodeBalanceNotEnough, cnName+"为0，无法减少")
			return
		}
		if req.Amount > currentBalance {
			app.Fail(c, app.ErrCodeBalanceNotEnough, fmt.Sprintf("%s不足（当前%.2f）", cnName, currentBalance))
			return
		}
	}

	// 执行更新
	newBalance := currentBalance + req.Amount
	if req.OpType == "sub" {
		newBalance = currentBalance - req.Amount
	}

	var updateField string
	switch req.Currency {
	case "money":
		updateField = "money"
	case "coupon":
		updateField = "coupon"
	case "self_bonus":
		updateField = "self_bonus"
	case "share_bonus":
		updateField = "share_bonus"
	}

	repository.DB.Model(&model.UserWallet{}).Where("user_id = ?", id).
		Update(updateField, newBalance).
		Update("updated_at", time.Now())

	// 写日志
	logType := int64(1) // 收入
	if req.OpType == "sub" {
		logType = 2 // 支出
	}

	desc := req.Desc
	if desc == "" {
		opLabel := "增加"
		if req.OpType == "sub" {
			opLabel = "减少"
		}
		desc = fmt.Sprintf("管理员%s%s", opLabel, cnName)
	}

	switch req.Currency {
	case "coupon":
		repository.DB.Exec("INSERT INTO coupon_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,NOW(),NOW())",
			id, logType, req.Amount, currentBalance, newBalance, desc)
	case "self_bonus":
		repository.DB.Exec("INSERT INTO self_bonus_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,NOW(),NOW())",
			id, logType, req.Amount, currentBalance, newBalance, desc)
	case "share_bonus":
		repository.DB.Exec("INSERT INTO share_bonus_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,?,?,?,?,?,NOW(),NOW())",
			id, logType, req.Amount, currentBalance, newBalance, desc)
	}

	app.OK(c, gin.H{
		"currency":  req.Currency,
		"op_type":   req.OpType,
		"amount":    req.Amount,
		"before":    currentBalance,
		"after":     newBalance,
	})
}

// UpdateUserParent 修改上级 PUT /api/v1/admin/users/:id/parent
func UpdateUserParent(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		PID int64 `json:"pid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "上级ID必须为数字")
		return
	}
	if req.PID <= 0 {
		app.BadRequest(c, "上级ID无效")
		return
	}
	// 检查上级是否存在
	var count int64
	repository.DB.Model(&model.User{}).Where("id = ?", req.PID).Count(&count)
	if count == 0 {
		app.BadRequest(c, "该上级ID在系统中不存在")
		return
	}
	if req.PID == id {
		app.BadRequest(c, "上级不能是自己")
		return
	}
	if err := repository.DB.Model(&model.User{}).Where("id = ?", id).Update("pid", req.PID).Error; err != nil {
		app.InternalError(c, "修改失败")
		return
	}
	app.OK(c, nil)
}

// UpdateUserStatus 冻结/解冻 PUT /api/v1/admin/users/:id/status
func UpdateUserStatus(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		Status int8 `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	if err := repository.DB.Model(&model.User{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		app.InternalError(c, "操作失败")
		return
	}
	app.OK(c, nil)
}

// ─── 订单管理 ───

// SearchOrders 订单筛选 POST /api/v1/admin/orders/search
func SearchOrders(c *gin.Context) {
	var req struct {
		Page         int    `json:"page"`
		Limit        int    `json:"limit"`
		OrderSN      string `json:"order_sn"`
		SellerID     int64  `json:"seller_id"`
		BuyerID      int64  `json:"buyer_id"`
		Status       *int   `json:"status"`
		IsResell     *int   `json:"is_resell"`
		IsShow       *int   `json:"is_show"`
		BuyStart     string `json:"buy_start"`
		BuyEnd       string `json:"buy_end"`
		ConfirmStart string `json:"confirm_start"`
		ConfirmEnd   string `json:"confirm_end"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page, req.Limit = 1, 10
	}
	if req.Page < 1 { req.Page = 1 }
	if req.Limit < 1 || req.Limit > 100 { req.Limit = 20 }

	where := " WHERE 1=1"
	if req.OrderSN != "" {
		where += fmt.Sprintf(" AND order_sn = '%s'", req.OrderSN)
	}
	if req.SellerID > 0 {
		where += fmt.Sprintf(" AND seller_id = %d", req.SellerID)
	}
	if req.BuyerID > 0 {
		where += fmt.Sprintf(" AND buyer_id = %d", req.BuyerID)
	}
	if req.Status != nil {
		where += fmt.Sprintf(" AND status = %d", *req.Status)
	}
	if req.IsResell != nil {
		where += fmt.Sprintf(" AND is_resell = %d", *req.IsResell)
	}
	if req.IsShow != nil {
		where += fmt.Sprintf(" AND is_show = %d", *req.IsShow)
	}
	if req.BuyStart != "" {
		where += fmt.Sprintf(" AND buy_time >= '%s'", padTime(req.BuyStart, false))
	}
	if req.BuyEnd != "" {
		where += fmt.Sprintf(" AND buy_time <= '%s'", padTime(req.BuyEnd, true))
	}
	if req.ConfirmStart != "" {
		where += fmt.Sprintf(" AND confirm_time >= '%s'", padTime(req.ConfirmStart, false))
	}
	if req.ConfirmEnd != "" {
		where += fmt.Sprintf(" AND confirm_time <= '%s'", padTime(req.ConfirmEnd, true))
	}

	var list []model.Order
	var count int64
	repository.DB.Raw("SELECT count(*) FROM orders" + where).Scan(&count)
	repository.DB.Raw(fmt.Sprintf("SELECT * FROM orders%s ORDER BY id DESC LIMIT %d OFFSET %d", where, req.Limit, (req.Page-1)*req.Limit)).Scan(&list)
	if list == nil { list = []model.Order{} }
	app.OKWithCount(c, list, count)
}

// ListOrders 订单列表 GET /api/v1/admin/orders
func ListOrders(c *gin.Context) {
	var req model.OrderListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	orders, count, err := repository.ListOrders(req)
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	app.OKWithCount(c, orders, count)
}

// GetOrder 订单详情 GET /api/v1/admin/orders/:id
func GetOrder(c *gin.Context) {
	id := parseIntParam(c, "id")
	o, err := repository.GetOrderByID(id)
	if err != nil {
		app.NotFound(c, "订单不存在")
		return
	}
	app.OK(c, o)
}

// UpdateOrderStatus PUT /api/v1/admin/orders/:id/status
func UpdateOrderStatus(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		Status int8 `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	if err := repository.UpdateOrderStatus(id, req.Status); err != nil {
		app.InternalError(c, "操作失败")
		return
	}
	app.OK(c, nil)
}

// ─── 商品管理 ───

// ListGoods 商品列表 GET /api/v1/admin/goods
func ListGoods(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)
	var categoryID *int64
	if v := c.Query("category_id"); v != "" {
		id := int64(queryInt(c, "category_id", 0))
		if id > 0 {
			categoryID = &id
		}
	}
	keyword := c.Query("keyword")
	goods, count, err := repository.ListGoods(page, limit, categoryID, keyword)
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	app.OKWithCount(c, goods, count)
}

// CreateGood POST /api/v1/admin/goods
func CreateGood(c *gin.Context) {
	var g model.Good
	if err := c.ShouldBindJSON(&g); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	if err := repository.CreateGood(&g); err != nil {
		app.InternalError(c, "创建失败")
		return
	}
	app.OK(c, g)
}

// UpdateGood PUT /api/v1/admin/goods/:id
func UpdateGood(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	delete(req, "id")
	req["updated_at"] = time.Now()
	if err := repository.DB.Model(&model.Good{}).Where("id = ?", id).Updates(req).Error; err != nil {
		app.InternalError(c, "更新失败")
		return
	}
	app.OK(c, nil)
}

// ─── 提现管理 ───

type WithdrawWithUser struct {
	model.Withdraw
	Nickname    string `json:"-" gorm:"column:nickname"`
	Mobile      string `json:"-" gorm:"column:mobile"`
	AcctName    string `json:"-" gorm:"column:acct_username"`
	AcctAccount string `json:"-" gorm:"column:acct_account"`
	AcctBank    string `json:"-" gorm:"column:acct_bank"`
	AcctPhone   string `json:"-" gorm:"column:acct_phone"`
	AcctQrcode  string `json:"-" gorm:"column:acct_qrcode"`
}

func (w WithdrawWithUser) MarshalJSON() ([]byte, error) {
	type Alias WithdrawWithUser

	acctInfo := map[string]interface{}{
		"id":       w.AccountID,
		"user_id":  w.UserID,
		"username": w.AcctName,
		"account":  w.AcctAccount,
		"bank":     w.AcctBank,
		"phone":    w.AcctPhone,
		"qrcode":   w.AcctQrcode,
	}

	return json.Marshal(struct {
		Alias
		AccountInfo map[string]interface{} `json:"account_info"`
		User        map[string]interface{} `json:"user"`
	}{
		Alias:       Alias(w),
		AccountInfo: acctInfo,
		User: map[string]interface{}{
			"id":       w.UserID,
			"nickname": w.Nickname,
			"mobile":   w.Mobile,
		},
	})
}

// SearchWithdraws 提现筛选 POST /api/v1/admin/withdraws/search
func SearchWithdraws(c *gin.Context) {
	var req struct {
		Page        int    `json:"page"`
		Limit       int    `json:"limit"`
		TransferNo  string `json:"transfer_no"`
		Currency    string `json:"currency_type"`
		Account     string `json:"account"`
		Status      *int   `json:"status"`
		CreateStart string `json:"create_start"`
		CreateEnd   string `json:"create_end"`
		UpdateStart string `json:"update_start"`
		UpdateEnd   string `json:"update_end"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page, req.Limit = 1, 10
	}
	if req.Page < 1 { req.Page = 1 }
	if req.Limit < 1 || req.Limit > 100 { req.Limit = 20 }

	where := " WHERE 1=1"
	if req.TransferNo != "" {
		where += fmt.Sprintf(" AND transfer_no LIKE '%%%s%%'", req.TransferNo)
	}
	if req.Currency != "" {
		where += fmt.Sprintf(" AND currency_type = '%s'", req.Currency)
	}
	if req.Account != "" {
		where += fmt.Sprintf(" AND (account_id IN (SELECT id FROM withdraw_accounts WHERE account LIKE '%%%s%%' OR username LIKE '%%%s%%') OR user_id IN (SELECT id FROM users WHERE username LIKE '%%%s%%' OR nickname LIKE '%%%s%%'))",
			req.Account, req.Account, req.Account, req.Account)
	}
	if req.Status != nil {
		where += fmt.Sprintf(" AND status = %d", *req.Status)
	}
	if req.CreateStart != "" {
		where += fmt.Sprintf(" AND created_at >= '%s'", padTime(req.CreateStart, false))
	}
	if req.CreateEnd != "" {
		where += fmt.Sprintf(" AND created_at <= '%s'", padTime(req.CreateEnd, true))
	}
	if req.UpdateStart != "" {
		where += fmt.Sprintf(" AND updated_at >= '%s'", padTime(req.UpdateStart, false))
	}
	if req.UpdateEnd != "" {
		where += fmt.Sprintf(" AND updated_at <= '%s'", padTime(req.UpdateEnd, true))
	}

	var list []WithdrawWithUser
	var count int64
	repository.DB.Raw("SELECT count(*) FROM withdraws w" + where).Scan(&count)
	repository.DB.Raw(fmt.Sprintf("SELECT w.*, u.nickname, u.mobile, a.username acct_username, a.account acct_account, a.bank acct_bank, a.phone acct_phone, a.qrcode acct_qrcode FROM withdraws w LEFT JOIN users u ON w.user_id = u.id LEFT JOIN withdraw_accounts a ON w.account_id = a.id%s ORDER BY w.id DESC LIMIT %d OFFSET %d", where, req.Limit, (req.Page-1)*req.Limit)).Scan(&list)
	if list == nil { list = []WithdrawWithUser{} }
	app.OKWithCount(c, list, count)
}

// ListWithdraws 提现列表 GET /api/v1/admin/withdraws
func ListWithdraws(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)
	var status *int8
	if s := c.Query("status"); s != "" {
		v := int8(queryInt(c, "status", 0))
		status = &v
	}
	list, count, err := repository.ListWithdraws(page, limit, status)
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	app.OKWithCount(c, list, count)
}

// ApproveWithdraw 提现审批 PUT /api/v1/admin/withdraws/:id/approve
func ApproveWithdraw(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req model.WithdrawApproveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	if err := repository.UpdateWithdrawStatus(id, req.Status); err != nil {
		app.InternalError(c, "操作失败")
		return
	}
	app.OK(c, nil)
}

// ─── 财务日志 ───

// SearchCouponLogs 优惠券明细 POST /api/v1/admin/logs/coupon/search
func SearchCouponLogs(c *gin.Context) { searchLogs(c, "coupon_logs") }

// SearchSelfBonusLogs 个人奖金明细 POST /api/v1/admin/logs/self-bonus/search
func SearchSelfBonusLogs(c *gin.Context) { searchLogs(c, "self_bonus_logs") }

// SearchShareBonusLogs 推广奖金明细 POST /api/v1/admin/logs/share-bonus/search
func SearchShareBonusLogs(c *gin.Context) { searchLogs(c, "share_bonus_logs") }

// SearchMoneyLogs 余额明细 POST /api/v1/admin/logs/money/search
func SearchMoneyLogs(c *gin.Context) { searchLogs(c, "money_logs") }

func searchLogs(c *gin.Context, table string) {
	var req struct {
		Page    int             `json:"page"`
		Limit   int             `json:"limit"`
		UserID  json.Number     `json:"user_id"`
		Type    *int            `json:"type"`
		Keyword string          `json:"keyword"`
		Start   string          `json:"start_time"`
		End     string          `json:"end_time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page, req.Limit = 1, 10
	}
	if req.Page < 1 { req.Page = 1 }
	if req.Limit < 1 || req.Limit > 100 { req.Limit = 20 }

	uid, _ := req.UserID.Int64()

	where := " WHERE 1=1"
	if uid > 0 {
		where += fmt.Sprintf(" AND user_id = %d", uid)
	}
	if req.Type != nil {
		where += fmt.Sprintf(" AND type = %d", *req.Type)
	}
	if req.Keyword != "" {
		where += fmt.Sprintf(" AND memo LIKE '%%%s%%'", req.Keyword)
	}
	if req.Start != "" {
		where += fmt.Sprintf(" AND created_at >= '%s'", padTime(req.Start, false))
	}
	if req.End != "" {
		where += fmt.Sprintf(" AND created_at <= '%s'", padTime(req.End, true))
	}

	var list []map[string]interface{}
	var count int64
	sql := fmt.Sprintf("SELECT l.*, u.nickname FROM %s l LEFT JOIN users u ON l.user_id = u.id%s ORDER BY l.id DESC LIMIT %d OFFSET %d",
		table, where, req.Limit, (req.Page-1)*req.Limit)
	repository.DB.Raw("SELECT count(*) FROM "+table+where).Scan(&count)
	repository.DB.Raw(sql).Scan(&list)
	if list == nil { list = []map[string]interface{}{} }
	app.OKWithCount(c, list, count)
}

// ListCouponLogs 优惠券明细 GET /api/v1/admin/logs/coupon
func ListCouponLogs(c *gin.Context) {
	var req model.LogListReq
	c.ShouldBindQuery(&req)
	list, count, _ := repository.ListCouponLogs(req)
	app.OKWithCount(c, list, count)
}

// ListSelfBonusLogs 个人奖金明细 GET /api/v1/admin/logs/self-bonus
func ListSelfBonusLogs(c *gin.Context) {
	var req model.LogListReq
	c.ShouldBindQuery(&req)
	list, count, _ := repository.ListSelfBonusLogs(req)
	app.OKWithCount(c, list, count)
}

// ListShareBonusLogs 推广奖金明细 GET /api/v1/admin/logs/share-bonus
func ListShareBonusLogs(c *gin.Context) {
	var req model.LogListReq
	c.ShouldBindQuery(&req)
	list, count, _ := repository.ListShareBonusLogs(req)
	app.OKWithCount(c, list, count)
}

// ─── 菜单规则 ───

// ListRules 菜单规则列表 GET /api/v1/admin/rules
func ListRules(c *gin.Context) {
	rules, err := repository.ListRules()
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	app.OK(c, rules)
}

// RuleTree 菜单树 GET /api/v1/admin/rules/tree（运营看不到权限管理）
func RuleTree(c *gin.Context) {
	uid := c.GetInt64("user_id")
	tree, err := repository.BuildRuleTree()
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}

	// 按当前管理员的角色权限过滤菜单树
	allowedIDs := getAllowedRuleIDs(uid)
	if allowedIDs != nil {
		filtered := filterRuleTree(tree, allowedIDs)
		if filtered == nil {
			filtered = make([]*model.Rule, 0)
		}
		tree = filtered
	}
	app.OK(c, tree)
}

// getAllowedRuleIDs 获取管理员有权限的规则ID集合，返回nil表示全部权限
func getAllowedRuleIDs(adminID int64) map[int64]bool {
	var admin model.AdminUser
	if repository.DB.Where("id = ?", adminID).First(&admin).Error != nil {
		return map[int64]bool{} // 查不到，无权限
	}

	roleIDs := strings.Split(admin.Roles, ",")
	allowed := make(map[int64]bool)

	for _, roleIDStr := range roleIDs {
		roleIDStr = strings.TrimSpace(roleIDStr)
		if roleIDStr == "" {
			continue
		}
		var role model.Role
		if err := repository.DB.Where("id = ?", roleIDStr).First(&role).Error; err != nil {
			continue
		}
		// "*" 表示全部权限
		rulesStr := strings.Trim(role.Rules, `"`)
		if rulesStr == "*" {
			return nil // nil = 全部
		}
		// 解析 JSON 数组: [1,2,3] 或带引号的 "[1,2,3]"
		rulesStr = strings.Trim(rulesStr, `"`)
		var ids []int64
		if err := json.Unmarshal([]byte(rulesStr), &ids); err == nil {
			for _, id := range ids {
				allowed[id] = true
			}
		}
	}
	return allowed
}

// filterRuleTree 递归过滤菜单树，只保留 allowedIDs 中的节点及其祖先路径
func filterRuleTree(nodes []*model.Rule, allowedIDs map[int64]bool) []*model.Rule {
	var result []*model.Rule
	for _, node := range nodes {
		filteredChildren := filterRuleTree(node.Children, allowedIDs)
		// 当前节点在允许列表中，或有被允许的子节点，则保留
		if allowedIDs[node.ID] || len(filteredChildren) > 0 {
			clone := *node
			clone.Children = filteredChildren
			result = append(result, &clone)
		}
	}
	return result
}

// ─── 秒杀管理 ───

// ListFlashSaleEvents 秒杀活动列表 GET /api/v1/admin/flash-sale/events
func ListFlashSaleEvents(c *gin.Context) {
	events, err := repository.ListFlashSaleEvents()
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	app.OK(c, events)
}

// CreateFlashSaleEvent 创建秒杀 POST /api/v1/admin/flash-sale/events
func CreateFlashSaleEvent(c *gin.Context) {
	var e model.FlashSaleEvent
	if err := c.ShouldBindJSON(&e); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	if err := repository.CreateFlashSaleEvent(&e); err != nil {
		app.InternalError(c, "创建失败")
		return
	}
	// 初始化Redis库存
	repository.InitProductStock(c.Request.Context(), e.ProductID, e.Stock)
	app.OK(c, e)
}

// ─── 辅助 ───

func parseIntParam(c *gin.Context, key string) int64 {
	var v int64
	fmt.Sscanf(c.Param(key), "%d", &v)
	return v
}

func queryInt(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return def
}
