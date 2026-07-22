package front

import (
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
)

func queryLogs(c *gin.Context, table string) {
	uid := c.GetInt64("user_id")
	limit := queryInt(c, "limit", 10)

	db := repository.DB.Table(table).Where("user_id = ?", uid)
	if t := c.Query("type"); t != "" {
		db = db.Where("type = ?", t)
	}

	var list []map[string]interface{}
	db.Order("id DESC").Limit(limit).Find(&list)
	if list == nil {
		list = []map[string]interface{}{}
	}
	app.OK(c, list)
}

// CouponLogs 优惠券明细 GET /api/v1/front/logs/coupon?limit=10&type=1
func CouponLogs(c *gin.Context) { queryLogs(c, "coupon_logs") }

// SelfBonusLogs 个人奖金明细 GET /api/v1/front/logs/self-bonus?limit=10&type=2
func SelfBonusLogs(c *gin.Context) { queryLogs(c, "self_bonus_logs") }

// ShareBonusLogs 推广奖金明细 GET /api/v1/front/logs/share-bonus?limit=10
func ShareBonusLogs(c *gin.Context) { queryLogs(c, "share_bonus_logs") }

// MoneyLogs 余额明细 GET /api/v1/front/logs/money?limit=10
func MoneyLogs(c *gin.Context) { queryLogs(c, "money_logs") }

// WithdrawLogs 提现记录 GET /api/v1/front/logs/withdraw?limit=10
func WithdrawLogs(c *gin.Context) {
	uid := c.GetInt64("user_id")
	limit := queryInt(c, "limit", 10)

	var list []map[string]interface{}
	repository.DB.Table("withdraws").Where("user_id = ?", uid).
		Order("id DESC").Limit(limit).Find(&list)
	if list == nil { list = []map[string]interface{}{} }
	app.OK(c, list)
}
