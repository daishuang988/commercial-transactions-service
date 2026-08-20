package admin

import (
	"fmt"
	"strings"
	"time"

	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"
	"commercial-transactions-service/pkg/utils"

	"github.com/xuri/excelize/v2"
	"github.com/gin-gonic/gin"
)

// ============ 系统配置 ============

// ConfigMeta 配置项的完整信息
type ConfigMeta struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Label string `json:"label"` // 中文名称
	Group string `json:"group"` // 分组
	Type  string `json:"type"`  // 类型: text/number/select/richtext/switch
}

var configMetaMap = map[string]ConfigMeta{
	"site_name":               {Label: "站点名称", Group: "基础设置", Type: "text"},
		"notice_content":          {Label: "系统公告", Group: "基础设置", Type: "richtext"},
	"service_phone":           {Label: "客服电话", Group: "基础设置", Type: "text"},
	"flash_sale_days":         {Label: "抢购开放日期(1-7=周一至周日)", Group: "秒杀规则", Type: "text"},
	"flash_sale_start":        {Label: "抢购开始时间", Group: "秒杀规则", Type: "text"},
	"flash_sale_end":          {Label: "抢购结束时间", Group: "秒杀规则", Type: "text"},
	"priority_max_orders":     {Label: "优先抢购最多N单", Group: "秒杀规则", Type: "number"},
	"priority_advance_minutes": {Label: "优先抢购提前N分钟", Group: "秒杀规则", Type: "number"},
	"resell_open":             {Label: "商城寄售开关", Group: "交易规则", Type: "switch"},
	"resell_rate":             {Label: "寄卖金额增值比例", Group: "交易规则", Type: "text"},
	"store_manager_rate":      {Label: "店长收益比率", Group: "交易规则", Type: "text"},
	"direct_referral_rate":    {Label: "直推收益比率(推广奖金)", Group: "交易规则", Type: "text"},
	"static_income_rate":      {Label: "静态收益比率(个人奖金)", Group: "交易规则", Type: "text"},
	"order_reward_rate":       {Label: "抢单奖励比率", Group: "交易规则", Type: "text"},
	"listing_fee_rate":        {Label: "上架费比率", Group: "交易规则", Type: "text"},
	"resell_deadline":         {Label: "无法寄卖结束时间", Group: "交易规则", Type: "text"},
	"resell_product_id":       {Label: "寄卖指定商品ID", Group: "交易规则", Type: "number"},
	"flash_sale_product":      {Label: "抢购页面商品(开关,价格,单位)", Group: "交易规则", Type: "text"},
	"split_threshold":         {Label: "达到N1金额拆分为N2单", Group: "交易规则", Type: "text"},
	"sms_verify":              {Label: "短信校验", Group: "功能开关", Type: "switch"},
	"coupon_withdraw_enable":  {Label: "优惠券提现开关", Group: "功能开关", Type: "switch"},
	"referral_withdraw_enable": {Label: "推荐奖提现开关", Group: "功能开关", Type: "switch"},
	"agreement_user":          {Label: "用户协议", Group: "协议规则", Type: "richtext"},
	"agreement_consignment":   {Label: "购买委托代卖协议", Group: "协议规则", Type: "richtext"},
	"agreement_purchase_notice": {Label: "购买须知", Group: "协议规则", Type: "richtext"},
	"agreement_backup":        {Label: "备用协议", Group: "协议规则", Type: "richtext"},
}

// GetConfig 系统配置 GET /api/v1/admin/config
func GetConfig(c *gin.Context) {
	var rows []struct {
		Key   string `gorm:"column:config_key"`
		Value string `gorm:"column:config_value"`
	}
	repository.DB.Table("system_configs").Find(&rows)

	data := make(map[string]interface{})
	for _, row := range rows {
		data[row.Key] = row.Value
	}
	data["_meta"] = configMetaMap
	app.OK(c, data)
}

// UpdateConfig 更新系统配置 PUT /api/v1/admin/config
func UpdateConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	for k, v := range req {
		repository.DB.Exec("INSERT INTO system_configs (config_key, config_value, updated_at) VALUES(?,?,NOW()) ON DUPLICATE KEY UPDATE config_value=?, updated_at=NOW()", k, v, v)
	}
	app.OK(c, nil)
}

// GetAccountInfo 当前管理员信息 GET /api/v1/admin/account/info
func GetAccountInfo(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var admin model.AdminUser
	if err := repository.DB.Where("id = ?", uid).First(&admin).Error; err != nil {
		app.NotFound(c, "管理员不存在")
		return
	}
	app.OK(c, admin)
}

// UpdateAccountInfo 编辑基本信息 PUT /api/v1/admin/account/info
func UpdateAccountInfo(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Mobile   string `json:"mobile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if req.Nickname != "" { updates["nickname"] = req.Nickname }
	if req.Email != "" { updates["email"] = req.Email }
	if req.Mobile != "" { updates["mobile"] = req.Mobile }
	repository.DB.Model(&model.AdminUser{}).Where("id = ?", uid).Updates(updates)
	app.OK(c, nil)
}

// ChangePassword 修改密码 PUT /api/v1/admin/account/password
func ChangePassword(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "密码至少6位")
		return
	}
	var admin model.AdminUser
	if err := repository.DB.Where("id = ?", uid).First(&admin).Error; err != nil {
		app.NotFound(c, "管理员不存在")
		return
	}
	if !utils.CheckPassword(req.OldPassword, admin.Password) && req.OldPassword != admin.Password {
		app.Fail(c, app.ErrCodeOldPasswordWrong, "原密码错误")
		return
	}
	hash, _ := utils.HashPassword(req.NewPassword)
	repository.DB.Model(&model.AdminUser{}).Where("id = ?", uid).Updates(map[string]interface{}{
		"password": hash, "updated_at": time.Now(),
	})
	app.OK(c, nil)
}

// ============ 商品管理补充 ============

// DeleteGood 删除商品 DELETE /api/v1/admin/goods/:id
func DeleteGood(c *gin.Context) {
	id := parseIntParam(c, "id")
	repository.DB.Where("id = ?", id).Delete(&model.Good{})
	app.OK(c, nil)
}

// UpdateGoodStock 设置商品库存 PUT /api/v1/admin/goods/:id/stock
func UpdateGoodStock(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		Stock int `json:"stock" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	repository.DB.Model(&model.Good{}).Where("id = ?", id).Update("stock_num", req.Stock)
	// 同步到 Redis
	repository.InitProductStock(c.Request.Context(), id, req.Stock)
	app.OK(c, nil)
}

// ============ 寄售商品 ============

// ListMerchandises 寄售商品列表 GET /api/v1/admin/merchandises
// SearchMerchandises 寄售商品筛选 POST /api/v1/admin/merchandises/search
func SearchMerchandises(c *gin.Context) {
	var req struct {
		Page        int    `json:"page"`
		Limit       int    `json:"limit"`
		ID          int64  `json:"id"`
		UserID      int64  `json:"user_id"`
		Keyword     string `json:"keyword"`
		IsShow      *int   `json:"is_show"`
		Status      *int   `json:"status"`
		CreateStart string `json:"create_start"`
		CreateEnd   string `json:"create_end"`
		UpdateStart string `json:"update_start"`
		UpdateEnd   string `json:"update_end"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Page = 1
		req.Limit = 10
	}
	if req.Page < 1 { req.Page = 1 }
	if req.Limit < 1 || req.Limit > 100 { req.Limit = 20 }

	var args []interface{}
	where := " WHERE 1=1"
	if req.ID > 0 {
		where += " AND id = ?"
		args = append(args, req.ID)
	}
	if req.UserID > 0 {
		where += " AND user_id = ?"
		args = append(args, req.UserID)
	}
	if req.Keyword != "" {
		where += " AND title LIKE ?"
		args = append(args, "%"+req.Keyword+"%")
	}
	if req.IsShow != nil {
		where += " AND is_show = ?"
		args = append(args, *req.IsShow)
	}
	if req.Status != nil {
		where += " AND status = ?"
		args = append(args, *req.Status)
	}
	if req.CreateStart != "" {
		where += " AND created_at >= ?"
		args = append(args, padTime(req.CreateStart, false))
	}
	if req.CreateEnd != "" {
		where += " AND created_at <= ?"
		args = append(args, padTime(req.CreateEnd, true))
	}
	if req.UpdateStart != "" {
		where += " AND updated_at >= ?"
		args = append(args, padTime(req.UpdateStart, false))
	}
	if req.UpdateEnd != "" {
		where += " AND updated_at <= ?"
		args = append(args, padTime(req.UpdateEnd, true))
	}

	args = append(args, req.Limit, (req.Page-1)*req.Limit)
	var list []model.Merchandise
	var count int64
	countSQL := "SELECT count(*) FROM merchandises" + where
	listSQL := "SELECT * FROM merchandises" + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	repository.DB.Raw(countSQL, args[:len(args)-2]...).Scan(&count)
	repository.DB.Raw(listSQL, args...).Scan(&list)
	if list == nil { list = []model.Merchandise{} }
	app.OKWithCount(c, list, count)
}

func ListMerchandises(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)
	var list []model.Merchandise
	var count int64
	repository.DB.Model(&model.Merchandise{}).Count(&count)
	repository.DB.Order("id DESC").Offset((page-1)*limit).Limit(limit).Find(&list)
	if list == nil { list = []model.Merchandise{} }
	app.OKWithCount(c, list, count)
}

// CreateMerchandise POST /api/v1/admin/merchandises
func CreateMerchandise(c *gin.Context) {
	var m model.Merchandise
	if err := c.ShouldBindJSON(&m); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	repository.DB.Create(&m)
	app.OK(c, m)
}

// DeleteMerchandise DELETE /api/v1/admin/merchandises/:id
func DeleteMerchandise(c *gin.Context) {
	id := parseIntParam(c, "id")
	repository.DB.Where("id = ?", id).Delete(&model.Merchandise{})
	app.OK(c, nil)
}

// UpdateMerchandiseStatus PUT /api/v1/admin/merchandises/:id/status
func UpdateMerchandiseStatus(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct {
		Status  int8 `json:"status" binding:"required"`
		IsShow  *int8 `json:"is_show"`
	}
	c.ShouldBindJSON(&req)
	updates := map[string]interface{}{"status": req.Status}
	if req.IsShow != nil {
		updates["is_show"] = *req.IsShow
	}
	repository.DB.Model(&model.Merchandise{}).Where("id = ?", id).Updates(updates)
	app.OK(c, nil)
}

// ============ 兑换订单 ============

// ListExchangeOrders GET /api/v1/admin/exchange-orders
func ListExchangeOrders(c *gin.Context) {
	// 老系统无数据，返回空
	app.OKWithCount(c, []interface{}{}, 0)
}

// ============ 余额变动日志 ============

// ListMoneyLogs GET /api/v1/admin/logs/money
func ListMoneyLogs(c *gin.Context) {
	// 老系统无数据，返回空
	app.OKWithCount(c, []interface{}{}, 0)
}

// ============ 管理员管理 ============

// ListAdmins GET /api/v1/admin/admins
func ListAdmins(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var admins []model.AdminUser
	// 运营只能看到自己，超管看全部
	var current model.AdminUser
	if repository.DB.Where("id = ?", uid).First(&current).Error == nil && current.Roles == "1" {
		repository.DB.Order("id ASC").Find(&admins)
	} else {
		repository.DB.Where("id = ?", uid).Find(&admins)
	}
	app.OK(c, admins)
}

// UpdateAdmin PUT /api/v1/admin/admins/:id
// 支持三个场景共用：编辑基本信息、修改密码、禁用/启用
func UpdateAdmin(c *gin.Context) {
	if !checkSuperAdmin(c) { app.Forbidden(c, "仅超管可操作"); return }
	id := parseIntParam(c, "id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}

	// 密码哈希处理
	if pw, ok := req["password"]; ok {
		pwStr := fmt.Sprintf("%v", pw)
		if pwStr != "" {
			if len(pwStr) < 6 {
				app.BadRequest(c, "密码至少6位")
				return
			}
			hash, err := utils.HashPassword(pwStr)
			if err != nil {
				app.InternalError(c, "密码加密失败")
				return
			}
			req["password"] = hash
		} else {
			delete(req, "password") // 空密码不更新
		}
	}

	// 用户名重复检查
	if un, ok := req["username"]; ok {
		unStr := fmt.Sprintf("%v", un)
		if unStr != "" {
			var existing int64
			repository.DB.Model(&model.AdminUser{}).Where("username = ? AND id != ?", unStr, id).Count(&existing)
			if existing > 0 {
				app.Fail(c, app.ErrCodeUserExists, "该用户名已被其他管理员使用")
				return
			}
		}
	}

	if len(req) == 0 {
		app.BadRequest(c, "没有要更新的字段")
		return
	}
	req["updated_at"] = time.Now()
	repository.DB.Model(&model.AdminUser{}).Where("id = ?", id).Updates(req)
	app.OK(c, nil)
}

// CreateAdmin POST /api/v1/admin/admins
func CreateAdmin(c *gin.Context) {
	if !checkSuperAdmin(c) { app.Forbidden(c, "仅超管可操作"); return }
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Nickname string `json:"nickname"`
		Roles    string `json:"roles"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	// 检查用户名是否已注册
	var existing int64
	repository.DB.Model(&model.AdminUser{}).Where("username = ?", req.Username).Count(&existing)
	if existing > 0 {
		app.Fail(c, app.ErrCodeUserExists, "该用户名已注册")
		return
	}
	hash, _ := utils.HashPassword(req.Password)
	now := time.Now()
	nickname := req.Nickname
	if nickname == "" { nickname = req.Username }
	a := &model.AdminUser{
		Username:  req.Username,
		Nickname:  nickname,
		Mobile:    &req.Username,
		Password:  hash,
		Avatar:    "/app/admin/avatar.png",
		Roles:     req.Roles,
		Status:    int8Ptr(1),
		CreatedAt: now,
		UpdatedAt: now,
	}
	repository.DB.Create(a)
	app.OK(c, a)
}

// DeleteAdmin 删除管理员 DELETE /api/v1/admin/admins/:id
func DeleteAdmin(c *gin.Context) {
	if !checkSuperAdmin(c) { app.Forbidden(c, "仅超管可操作"); return }

	uid := c.GetInt64("user_id")
	id := parseIntParam(c, "id")

	// 不能删除自己
	if uid == id {
		app.BadRequest(c, "不能删除自己的账号")
		return
	}

	// 检查目标管理员是否存在
	var admin model.AdminUser
	if err := repository.DB.Where("id = ?", id).First(&admin).Error; err != nil {
		app.NotFound(c, "管理员不存在")
		return
	}

	// 检查是否为最后一个超管
	if admin.Roles == "1" {
		var superCount int64
		repository.DB.Model(&model.AdminUser{}).Where("roles = ?", "1").Count(&superCount)
		if superCount <= 1 {
			app.BadRequest(c, "至少保留一个超级管理员")
			return
		}
	}

	repository.DB.Where("id = ?", id).Delete(&model.AdminUser{})
	app.OK(c, nil)
}

func int8Ptr(v int8) *int8 { return &v }

// ResetAdminPassword 重置管理员密码 PUT /api/v1/admin/admins/:id/password
func ResetAdminPassword(c *gin.Context) {
	uid := c.GetInt64("user_id")
	targetID := parseIntParam(c, "id")
	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "密码至少6位")
		return
	}
	// 检查权限：超管可改所有人，运营只能改自己
	var current model.AdminUser
	repository.DB.Where("id = ?", uid).First(&current)
	if current.Roles != "1" && uid != targetID {
		app.Forbidden(c, "仅超管可修改他人密码")
		return
	}
	hash, _ := utils.HashPassword(req.Password)
	repository.DB.Model(&model.AdminUser{}).Where("id = ?", targetID).Updates(map[string]interface{}{
		"password": hash, "updated_at": time.Now(),
	})
	app.OK(c, nil)
}

// ============ 角色管理 ============

// checkSuperAdmin 检查是否为超管
func checkSuperAdmin(c *gin.Context) bool {
	uid := c.GetInt64("user_id")
	var admin model.AdminUser
	if err := repository.DB.Where("id = ?", uid).First(&admin).Error; err != nil {
		return false
	}
	return admin.Roles == "1"
}

// ListRoles GET /api/v1/admin/roles
func ListRoles(c *gin.Context) {
	if !checkSuperAdmin(c) { app.Forbidden(c, "仅超管可访问"); return }
	var roles []model.Role
	repository.DB.Find(&roles)
	app.OK(c, roles)
}

// CreateRole POST /api/v1/admin/roles
func CreateRole(c *gin.Context) {
	if !checkSuperAdmin(c) { app.Forbidden(c, "仅超管可访问"); return }
	var r model.Role
	if err := c.ShouldBindJSON(&r); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	r.Status = 1 // 默认启用
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	repository.DB.Create(&r)
	app.OK(c, r)
}

// UpdateRole PUT /api/v1/admin/roles/:id
func UpdateRole(c *gin.Context) {
	if !checkSuperAdmin(c) { app.Forbidden(c, "仅超管可访问"); return }
	id := parseIntParam(c, "id")
	var r model.Role
	if err := c.ShouldBindJSON(&r); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	r.ID = id
	r.UpdatedAt = time.Now()
	repository.DB.Model(&model.Role{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name": r.Name, "rules": r.Rules, "status": r.Status, "updated_at": r.UpdatedAt,
	})
	app.OK(c, nil)
}

// DeleteRole 删除角色 DELETE /api/v1/admin/roles/:id
func DeleteRole(c *gin.Context) {
	if !checkSuperAdmin(c) { app.Forbidden(c, "仅超管可访问"); return }
	id := parseIntParam(c, "id")

	// 检查角色是否存在
	var role model.Role
	if err := repository.DB.Where("id = ?", id).First(&role).Error; err != nil {
		app.NotFound(c, "角色不存在")
		return
	}

	// 检查是否有管理员正在使用该角色
	var adminCount int64
		roleStr := fmt.Sprintf("%d", id)
	repository.DB.Model(&model.AdminUser{}).Where("roles = ? OR roles LIKE ? OR roles LIKE ? OR roles LIKE ?", roleStr, roleStr+",%", "%,"+roleStr, "%,"+roleStr+",%").Count(&adminCount)
	if adminCount > 0 {
		app.BadRequest(c, fmt.Sprintf("有 %d 个管理员正在使用该角色，请先更换角色后再删除", adminCount))
		return
	}

	repository.DB.Where("id = ?", id).Delete(&model.Role{})
	app.OK(c, nil)
}

// ============ Banner管理 ============

// ListBanners GET /api/v1/admin/banners
func ListBanners(c *gin.Context) {
	var list []map[string]interface{}
	repository.DB.Table("banners").Find(&list)
	if list == nil {
		list = []map[string]interface{}{}
	}
	app.OK(c, list)
}

// CreateBanner POST /api/v1/admin/banners
func CreateBanner(c *gin.Context) {
	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	req["created_at"] = time.Now()
	req["updated_at"] = time.Now()
	repository.DB.Table("banners").Create(req)
	app.OK(c, nil)
}

// DeleteBanner DELETE /api/v1/admin/banners/:id
func DeleteBanner(c *gin.Context) {
	id := parseIntParam(c, "id")
	repository.DB.Table("banners").Where("id = ?", id).Delete(nil)
	app.OK(c, nil)
}

// BatchDeleteBanners 批量删除 POST /api/v1/admin/banners/batch-delete
func BatchDeleteBanners(c *gin.Context) {
	var req struct {
		Ids []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		app.BadRequest(c, "请选择要删除的项")
		return
	}
	repository.DB.Table("banners").Where("id IN ?", req.Ids).Delete(nil)
	app.OK(c, nil)
}

// UpdateBanner PUT /api/v1/admin/banners/:id
func UpdateBanner(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	req["updated_at"] = time.Now()
	repository.DB.Table("banners").Where("id = ?", id).Updates(req)
	app.OK(c, nil)
}

// ============ 广告管理 ============

// ListAds GET /api/v1/admin/ads
func ListAds(c *gin.Context) {
	var list []map[string]interface{}
	repository.DB.Table("ads").Find(&list)
	if list == nil {
		list = []map[string]interface{}{}
	}
	app.OK(c, list)
}

// CreateAd POST /api/v1/admin/ads
func CreateAd(c *gin.Context) {
	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	req["created_at"] = time.Now()
	req["updated_at"] = time.Now()
	repository.DB.Table("ads").Create(req)
	app.OK(c, nil)
}

// DeleteAd DELETE /api/v1/admin/ads/:id
func DeleteAd(c *gin.Context) {
	id := parseIntParam(c, "id")
	repository.DB.Table("ads").Where("id = ?", id).Delete(nil)
	app.OK(c, nil)
}

// UpdateAd PUT /api/v1/admin/ads/:id
func UpdateAd(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req map[string]interface{}
	c.ShouldBindJSON(&req)
	req["updated_at"] = time.Now()
	repository.DB.Table("ads").Where("id = ?", id).Updates(req)
	app.OK(c, nil)
}

// padTime 补齐时间：纯日期补时间，已有时间则不动；兼容 URL 中冒号未编码的情况
func padTime(v string, isEnd bool) string {
	v = strings.ReplaceAll(v, "%20", " ")
	v = strings.ReplaceAll(v, "+", " ")
	v = strings.ReplaceAll(v, "%3A", ":")
	if len(v) == 10 {
		if isEnd { return v + " 23:59:59" }
		return v + " 00:00:00"
	}
	return v
}

// ExportUsers 导出用户XLSX GET /api/v1/admin/users/export
func ExportUsers(c *gin.Context) {
	today := time.Now().In(repository.CSTLocation()).Format("01-02")

	type ExportRow struct {
		UserID         int64
		Nickname       string
		TodaySellTotal float64
		TodayBuyTotal  float64
		ParentID       int64
		ParentNickname string
	}
	var rows []ExportRow
	repository.DB.Raw(`
		SELECT u.id AS user_id, u.nickname,
			COALESCE((SELECT COALESCE(SUM(o.total_money),0) FROM orders o WHERE o.seller_id=u.id AND o.status IN (0,1,2) AND DATE(o.created_at)=CURDATE()), 0) AS today_sell_total,
			COALESCE((SELECT COALESCE(SUM(o.total_money),0) FROM orders o WHERE o.buyer_id=u.id AND o.status IN (0,1,2) AND DATE(o.created_at)=CURDATE()), 0) AS today_buy_total,
			COALESCE(u.pid,0) AS parent_id,
			COALESCE((SELECT p.nickname FROM users p WHERE p.id=u.pid), '') AS parent_nickname
		FROM users u
		ORDER BY u.id DESC LIMIT 5000
	`).Scan(&rows)

	f := excelize.NewFile()
	sheet := "用户下单统计表" + time.Now().In(repository.CSTLocation()).Format("20060102")
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"用户ID", "昵称", today + "卖货", today + "买货", "差额", "推广人ID", "推广人"}
	for i, h := range headers { col, _ := excelize.CoordinatesToCellName(i+1, 1); f.SetCellValue(sheet, col, h) }
	for i, r := range rows {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.UserID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.Nickname)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.TodaySellTotal)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.TodayBuyTotal)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.TodaySellTotal-r.TodayBuyTotal)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.ParentID)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.ParentNickname)
	}
	autoWidth(f, sheet, 7)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=用户下单统计表%s.xlsx", time.Now().In(repository.CSTLocation()).Format("20060102")))
	f.Write(c.Writer)
}

// ExportWithdraws 导出提现XLSX GET /api/v1/admin/withdraws/export
func ExportWithdraws(c *gin.Context) {
	var list []map[string]interface{}
	repository.DB.Raw(`SELECT w.*, u.nickname, a.username acct_name, a.account acct_account, a.bank acct_bank, a.phone acct_phone FROM withdraws w LEFT JOIN users u ON w.user_id=u.id LEFT JOIN withdraw_accounts a ON w.account_id=a.id ORDER BY w.id DESC LIMIT 5000`).Scan(&list)

	f := excelize.NewFile()
	sheet := "提现申请" + time.Now().In(repository.CSTLocation()).Format("20060102")
	f.SetSheetName("Sheet1", sheet)
	heads := []string{"提现编号","用户姓名","提现货币","提现账号类型","提现金额","手续费","实际到账金额","提现账号信息","状态","备注","申请时间","更新时间"}
	for i, h := range heads { col, _ := excelize.CoordinatesToCellName(i+1, 1); f.SetCellValue(sheet, col, h) }
	for i, w := range list {
		row := i + 2
		// 货币
		cur := "优惠券"; if fmt.Sprintf("%v", w["currency_type"]) == "share_bonus" { cur = "推广奖金" }
		// 账号类型
		acctType := "银行卡"; if fmt.Sprintf("%v", w["account_type"]) == "2" { acctType = "支付宝" }
		// 状态
		st := "待处理"; switch fmt.Sprintf("%v", w["status"]) { case "1": st = "已通过"; case "3": st = "已驳回" }
		// 账号信息: 收款人/账号/银行/电话
		acctInfo := fmt.Sprintf("%v / %v / %v / %v", w["acct_name"], w["acct_account"], w["acct_bank"], w["acct_phone"])
		vals := []interface{}{w["transfer_no"],w["nickname"],cur,acctType,w["money"],w["handling_fee"],w["actual_amount"],acctInfo,st,w["remark"],w["created_at"],w["updated_at"]}
		for j, v := range vals { col, _ := excelize.CoordinatesToCellName(j+1, row); f.SetCellValue(sheet, col, v) }
	}
	autoWidth(f, sheet, 12)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.xlsx", sheet))
	f.Write(c.Writer)
}

func exportLogXLSX(c *gin.Context, table, sheetName string) {
	var list []map[string]interface{}
	repository.DB.Raw(fmt.Sprintf("SELECT l.*, u.nickname FROM %s l LEFT JOIN users u ON l.user_id=u.id ORDER BY l.id DESC LIMIT 5000", table)).Scan(&list)
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", sheetName)
	for i, h := range []string{"ID","用户ID","用户昵称","类型","金额","变动前","变动后","备注","时间"} {
		col, _ := excelize.CoordinatesToCellName(i+1, 1); f.SetCellValue(sheetName, col, h)
	}
	for i, r := range list {
		row := i + 2
		typeName := "收入"; if fmt.Sprintf("%v", r["type"]) == "2" { typeName = "支出" }
		vals := []interface{}{r["id"],r["user_id"],r["nickname"],typeName,r["money"],r["before"],r["after"],r["memo"],r["created_at"]}
		for j, v := range vals { col, _ := excelize.CoordinatesToCellName(j+1, row); f.SetCellValue(sheetName, col, v) }
	}
	autoWidth(f, sheetName, 9)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.xlsx", sheetName))
	f.Write(c.Writer)
}

func autoWidth(f *excelize.File, sheet string, cols int) {
	for i := 1; i <= cols; i++ { col, _ := excelize.CoordinatesToCellName(i, 1); f.SetColWidth(sheet, col, col, 22) }
}

// ExportCouponLogs / ExportSelfBonusLogs / ExportShareBonusLogs / ExportMoneyLogs
func ExportCouponLogs(c *gin.Context)   { exportLogXLSX(c, "coupon_logs", "优惠券明细"+time.Now().In(repository.CSTLocation()).Format("20060102")) }
func ExportSelfBonusLogs(c *gin.Context) { exportLogXLSX(c, "self_bonus_logs", "个人奖金明细"+time.Now().In(repository.CSTLocation()).Format("20060102")) }
func ExportShareBonusLogs(c *gin.Context) { exportLogXLSX(c, "share_bonus_logs", "推广奖金明细"+time.Now().In(repository.CSTLocation()).Format("20060102")) }
func ExportMoneyLogs(c *gin.Context)      { exportLogXLSX(c, "money_logs", "余额明细"+time.Now().In(repository.CSTLocation()).Format("20060102")) }

// ResetContract 重签合同 POST /api/v1/admin/users/:id/contract/reset
func ResetContract(c *gin.Context) {
	id := parseIntParam(c, "id")
	repository.DB.Where("user_id = ?", id).Delete(&model.UserContract{})
	repository.DB.Model(&model.User{}).Where("id = ?", id).Update("contract", "")
	app.OK(c, gin.H{"msg": "合同已重置，用户可重新签署"})
}

// GetContract 查看用户合同 GET /api/v1/front/user/contract
func GetContract(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var path string
	repository.DB.Table("user_contracts").Select("contract_path").Where("user_id=?", uid).Order("id DESC").Limit(1).Scan(&path)
	app.OK(c, gin.H{"contract": path})
}

// UploadContract 上传用户合同 POST /api/v1/admin/users/:id/contract
func UploadContract(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req struct{ Path string `json:"path" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请提供合同路径")
		return
	}
	now := time.Now()
	repository.DB.Exec("INSERT INTO user_contracts (user_id,contract_path,created_at) VALUES(?,?,?)", id, req.Path, now)
	repository.DB.Model(&model.User{}).Where("id=?", id).Update("contract", req.Path)
	app.OK(c, gin.H{"msg": "合同已上传"})
}

// UpdateMerchandise 编辑寄售商品 PUT /api/v1/admin/merchandises/:id
func UpdateMerchandise(c *gin.Context) {
	id := parseIntParam(c, "id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	delete(req, "id")
	req["updated_at"] = time.Now()
	repository.DB.Model(&model.Merchandise{}).Where("id = ?", id).Updates(req)
	app.OK(c, nil)
}
