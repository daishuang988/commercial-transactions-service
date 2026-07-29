package front

import (
	"fmt"
	"time"

	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
)

// WithdrawRequest 提现请求
type WithdrawRequest struct {
	Amount      float64 `json:"amount"       binding:"required"`
	Currency    string  `json:"currency"     binding:"required"` // coupon / share_bonus
	AccountType int8    `json:"account_type" binding:"required"` // 1银行卡 2支付宝
}

// Withdraw 用户提现 POST /api/v1/front/withdraw
func Withdraw(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		app.BadRequest(c, "请输入有效提现金额")
		return
	}

	// 检查开关
	if req.Currency == "coupon" && repository.GetConfigInt("coupon_withdraw_enable", 0) == 0 {
		app.BadRequest(c, "优惠券提现暂未开放")
		return
	}
	if req.Currency == "share_bonus" && repository.GetConfigInt("referral_withdraw_enable", 0) == 0 {
		app.BadRequest(c, "推广奖金提现暂未开放")
		return
	}
	if req.Currency != "coupon" && req.Currency != "share_bonus" {
		app.BadRequest(c, "提现类型不支持")
		return
	}
	if req.AccountType != 1 && req.AccountType != 2 {
		app.BadRequest(c, "账户类型仅支持 1银行卡 2支付宝")
		return
	}

	// 校验收款账户是否存在
	var accountID int64
	repository.DB.Table("withdraw_accounts").Select("id").Where("user_id=? AND account_type=?", uid, req.AccountType).Scan(&accountID)
	if accountID == 0 {
		typeName := map[int8]string{1: "银行卡", 2: "支付宝"}
		app.BadRequest(c, fmt.Sprintf("未添加%s收款方式，请先添加", typeName[req.AccountType]))
		return
	}

	// 查当前余额
	wallet, err := repository.GetUserWallet(uid)
	if err != nil {
		app.NotFound(c, "钱包不存在")
		return
	}
	var balance float64
	if req.Currency == "coupon" {
		balance = wallet.Coupon
	} else {
		balance = wallet.ShareBonus
	}
	if req.Amount > balance {
		app.Fail(c, app.ErrCodeBalanceNotEnough, fmt.Sprintf("可提现余额不足(当前%.2f)", balance))
		return
	}

	// 创建提现记录
	now := time.Now()
	transferNo := fmt.Sprintf("%d%07d", now.UnixMilli(), uid%10000000)
	repository.DB.Exec(
		"INSERT INTO withdraws (transfer_no,user_id,cate,account_type,currency_type,account_id,money,handling_fee,actual_amount,status,remark,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)",
		transferNo, uid, 1, req.AccountType, req.Currency, accountID, req.Amount, 0, req.Amount, 2, "用户提现", now, now)

	// 写日志扣减
	if req.Currency == "coupon" {
		before := wallet.Coupon
		after := before - req.Amount
		repository.DB.Exec(
			"INSERT INTO coupon_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,2,?,?,?,?,NOW(),NOW())",
			uid, req.Amount, before, after, "用户提现")
		repository.DB.Exec("UPDATE user_wallets SET coupon = ?, updated_at = NOW() WHERE user_id = ?", after, uid)
	} else {
		before := wallet.ShareBonus
		after := before - req.Amount
		repository.DB.Exec(
			"INSERT INTO share_bonus_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,2,?,?,?,?,NOW(),NOW())",
			uid, req.Amount, before, after, "用户提现")
		repository.DB.Exec("UPDATE user_wallets SET share_bonus = ?, updated_at = NOW() WHERE user_id = ?", after, uid)
	}

	app.OK(c, gin.H{"msg": "提现申请已提交", "transfer_no": transferNo})
}
