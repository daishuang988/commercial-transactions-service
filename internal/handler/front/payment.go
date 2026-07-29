package front

import (
	"time"

	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
)

// PaymentMethods 收款方式列表 GET /api/v1/front/user/payment-methods
func PaymentMethods(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var list []map[string]interface{}
	repository.DB.Table("withdraw_accounts").Where("user_id = ?", uid).Order("id DESC").Find(&list)
	if list == nil { list = []map[string]interface{}{} }
	app.OK(c, list)
}

// AddPaymentMethod 添加/更新收款方式 POST /api/v1/front/user/payment-method
func AddPaymentMethod(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req struct {
		AccountType int8   `json:"account_type" binding:"required"`
		Username    string `json:"username" binding:"required"`
		Account     string `json:"account" binding:"required"`
		Bank        string `json:"bank"`
		Phone       string `json:"phone" binding:"required"`
		Qrcode      string `json:"qrcode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请填写完整信息")
		return
	}
	if req.AccountType != 1 && req.AccountType != 2 {
		app.BadRequest(c, "类型仅支持 1银行卡 2支付宝")
		return
	}

	now := time.Now()
	// 同类型已有则更新，否则新增
	var existID int64
	repository.DB.Table("withdraw_accounts").Select("id").Where("user_id=? AND account_type=?", uid, req.AccountType).Scan(&existID)
	if existID > 0 {
		repository.DB.Exec(
			"UPDATE withdraw_accounts SET username=?, account=?, bank=NULLIF(?,''), phone=?, qrcode=NULLIF(?,''), updated_at=? WHERE id=?",
			req.Username, req.Account, req.Bank, req.Phone, req.Qrcode, now, existID)
	} else {
		repository.DB.Exec(
			"INSERT INTO withdraw_accounts (user_id,username,account,account_type,bank,phone,qrcode,created_at,updated_at) VALUES(?,?,?,?,NULLIF(?,''),?,NULLIF(?,''),?,?)",
			uid, req.Username, req.Account, req.AccountType, req.Bank, req.Phone, req.Qrcode, now, now)
	}

	var result map[string]interface{}
	repository.DB.Table("withdraw_accounts").Where("user_id=? AND account_type=?", uid, req.AccountType).Order("id DESC").Limit(1).Take(&result)
	app.OK(c, result)
}

// DeletePaymentMethod 删除收款方式 DELETE /api/v1/front/user/payment-method/:id
func DeletePaymentMethod(c *gin.Context) {
	uid := c.GetInt64("user_id")
	id := parseIntParam(c, "id")
	if repository.DB.Where("id=? AND user_id=?", id, uid).Delete(nil).RowsAffected == 0 {
		app.NotFound(c, "收款方式不存在")
		return
	}
	app.OK(c, nil)
}
