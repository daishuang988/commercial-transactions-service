package app

// 业务错误码（对标老系统 code 字段，0=成功）
const (
	ErrCodeSuccess       = 0
	ErrCodeBadRequest    = 400
	ErrCodeUnauthorized  = 401
	ErrCodeForbidden     = 403
	ErrCodeNotFound      = 404
	ErrCodeTooMany       = 429
	ErrCodeInternal      = 500

	// 业务错误码
	ErrCodeUserExists        = 1001 // 用户已存在
	ErrCodeUserNotFound      = 1002 // 用户不存在
	ErrCodePasswordWrong     = 1003 // 密码错误
	ErrCodeUserFrozen        = 1004 // 用户已冻结
	ErrCodeTokenExpired      = 1005 // Token 过期
	ErrCodeTokenInvalid      = 1006 // Token 无效
	ErrCodeOldPasswordWrong  = 1007 // 原密码错误

	ErrCodeStockNotEnough    = 2001 // 库存不足
	ErrCodeSoldOut           = 2002 // 已售罄
	ErrCodeLimitExceeded     = 2003 // 超出限购数量
	ErrCodeAlreadyBought     = 2004 // 已购买过
	ErrCodeFlashSaleClosed   = 2005 // 秒杀未开放
	ErrCodeFlashSaleNotInTime = 2006 // 不在秒杀时间段
	ErrCodeBalanceNotEnough  = 2007 // 余额不足

	ErrCodeOrderNotFound     = 3001 // 订单不存在
	ErrCodeOrderStatusErr    = 3002 // 订单状态异常

	ErrCodeWithdrawNotFound  = 4001 // 提现记录不存在
	ErrCodeWithdrawApproved  = 4002 // 已审批过
	ErrCodeAmountInvalid     = 4003 // 金额无效
)

var errMsgMap = map[int]string{
	ErrCodeUserExists:        "用户已存在",
	ErrCodeUserNotFound:      "用户不存在",
	ErrCodePasswordWrong:     "密码错误",
	ErrCodeUserFrozen:        "账号已冻结",
	ErrCodeTokenExpired:      "登录已过期",
	ErrCodeTokenInvalid:      "Token无效",
	ErrCodeOldPasswordWrong:  "原密码错误",
	ErrCodeStockNotEnough:    "库存不足",
	ErrCodeSoldOut:           "已售罄",
	ErrCodeLimitExceeded:     "超出限购数量",
	ErrCodeAlreadyBought:     "已购买过该商品",
	ErrCodeFlashSaleClosed:   "秒杀活动未开放",
	ErrCodeFlashSaleNotInTime: "不在秒杀时间段(工作日10:00-10:30)",
	ErrCodeBalanceNotEnough:  "余额不足",
	ErrCodeOrderNotFound:     "订单不存在",
	ErrCodeOrderStatusErr:    "订单状态异常",
	ErrCodeWithdrawNotFound:  "提现记录不存在",
	ErrCodeWithdrawApproved:  "该提现已审批",
	ErrCodeAmountInvalid:     "金额无效",
}

func GetErrMsg(code int) string {
	if msg, ok := errMsgMap[code]; ok {
		return msg
	}
	return "未知错误"
}
