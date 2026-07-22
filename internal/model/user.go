package model

import "time"

// User 用户主表（对标老系统 user 表，36字段，1,843条）
type User struct {
	ID         int64      `json:"id"          gorm:"primaryKey"`
	Username   string     `json:"username"    gorm:"column:username"`
	Nickname   string     `json:"nickname"    gorm:"column:nickname"`
	Mobile     string     `json:"mobile"      gorm:"column:mobile"`
	Password   string     `json:"-"           gorm:"column:password"`
	Salt       string     `json:"-"           gorm:"column:salt"`
	Sex        int8       `json:"sex"         gorm:"column:sex"`
	Avatar     string     `json:"avatar"      gorm:"column:avatar"`
	Invite     string     `json:"invite"      gorm:"column:invite"`     // 推广码
	Level      int8       `json:"level"       gorm:"column:level"`
	Birthday   *time.Time `json:"birthday"    gorm:"column:birthday"`
	IsVip      int8       `json:"is_vip"      gorm:"column:is_vip"`
	Viptime    *time.Time `json:"viptime"     gorm:"column:viptime"`
	IsResell   int8       `json:"is_resell"   gorm:"column:is_resell"`
	IsPriority int8       `json:"is_priority" gorm:"column:is_priority"`
	MaxOrder          int        `json:"max_order"            gorm:"column:max_order"`
	TodayBuyTotal     float64    `json:"today_buy_total"       gorm:"column:today_buy_total"`
	TodayBuyCount     int        `json:"today_buy_count"       gorm:"column:today_buy_count"`
	TodaySellTotal    float64    `json:"today_sell_total"      gorm:"column:today_sell_total"`
	YesterdaySellCount int       `json:"yesterday_sell_count"  gorm:"column:yesterday_sell_count"`
	Contract          string     `json:"contract"              gorm:"column:contract"`
	PID               int64      `json:"pid"                   gorm:"column:pid"` // 上级ID
	JoinTime   *time.Time `json:"join_time"   gorm:"column:join_time"`
	JoinIP     string     `json:"join_ip"     gorm:"column:join_ip"`
	LastTime   *time.Time `json:"last_time"   gorm:"column:last_time"`
	LastIP     string     `json:"last_ip"     gorm:"column:last_ip"`
	Token      string     `json:"token"       gorm:"column:token"`
	Status     int8       `json:"status"      gorm:"column:status"`     // 0冻结 1正常
	CreatedAt  time.Time  `json:"created_at"  gorm:"column:created_at"`
	UpdatedAt  time.Time  `json:"updated_at"  gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }

// UserWallet 用户钱包（从 user 拆出）
type UserWallet struct {
	ID         int64     `json:"id"          gorm:"primaryKey"`
	UserID     int64     `json:"user_id"     gorm:"column:user_id"`
	Money      float64   `json:"money"       gorm:"column:money"`
	Coupon     float64   `json:"coupon"      gorm:"column:coupon"`
	SelfBonus  float64   `json:"self_bonus"  gorm:"column:self_bonus"`
	ShareBonus float64   `json:"share_bonus" gorm:"column:share_bonus"`
	Score      int       `json:"score"       gorm:"column:score"`
	Poor       float64   `json:"poor"        gorm:"column:poor"`
	UpdatedAt  time.Time `json:"updated_at"  gorm:"column:updated_at"`
}

func (UserWallet) TableName() string { return "user_wallets" }

// UserContract 用户合同
type UserContract struct {
	ID           int64     `json:"id"            gorm:"primaryKey"`
	UserID       int64     `json:"user_id"       gorm:"column:user_id"`
	ContractPath string    `json:"contract_path" gorm:"column:contract_path"`
	CreatedAt    time.Time `json:"created_at"    gorm:"column:created_at"`
}

func (UserContract) TableName() string { return "user_contracts" }

// LoginReq 登录请求
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterReq 注册请求
type RegisterReq struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required,min=6"`
	Nickname   string `json:"nickname"`
	Mobile     string `json:"mobile" binding:"required"`
	InviteCode string `json:"invite_code"` // 推荐人推广码
}

// UserListReq 用户列表查询
type UserListReq struct {
	Page    int    `form:"page"     binding:"required,min=1"`
	Limit   int    `form:"limit"    binding:"required,min=1,max=100"`
	Keyword string `form:"keyword"`
	UserID  string `form:"user_id"`  // 支持逗号分隔批量: "1,2,3"
	Mobile  string `form:"mobile"`   // 支持逗号分隔批量
	Level   *int8  `form:"level"`
	Status  *int8  `form:"status"`
	PID     string `form:"pid"`      // 支持逗号分隔批量
}

// BatchDeleteReq 批量删除
type BatchDeleteReq struct {
	IDs []int64 `json:"ids" binding:"required"`
}
