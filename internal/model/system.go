package model

import "time"

// AdminUser 管理员（对标老系统，2条）
type AdminUser struct {
	ID          int64      `json:"id"           gorm:"primaryKey"`
	Username    string     `json:"username"     gorm:"column:username"`
	Nickname    string     `json:"nickname"     gorm:"column:nickname"`
	Password    string     `json:"-"            gorm:"column:password"`
	Avatar      string     `json:"avatar"       gorm:"column:avatar"`
	Email       *string    `json:"email"        gorm:"column:email"`
	Mobile      *string    `json:"mobile"       gorm:"column:mobile"`
	Qrcode      string     `json:"qrcode"       gorm:"column:qrcode"`
	Roles       string     `json:"roles"        gorm:"column:roles"`
	ShowToolbar int8       `json:"show_toolbar" gorm:"column:show_toolbar"`
	LoginAt     *time.Time `json:"login_at"     gorm:"column:login_at"`
	Status      *int8      `json:"status"       gorm:"column:status"`
	CreatedAt   time.Time  `json:"created_at"   gorm:"column:created_at"`
	UpdatedAt   time.Time  `json:"updated_at"   gorm:"column:updated_at"`
}

func (AdminUser) TableName() string { return "admin_users" }

// AdminLoginReq 管理端登录
type AdminLoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Withdraw 提现记录（对标老系统，742条）
type Withdraw struct {
	ID           int64     `json:"id"            gorm:"primaryKey"`
	TransferNo   string    `json:"transfer_no"   gorm:"column:transfer_no"`
	UserID       int64     `json:"user_id"       gorm:"column:user_id"`
	Cate         int8      `json:"cate"          gorm:"column:cate"`
	AccountType  int8      `json:"account_type"  gorm:"column:account_type"`
	CurrencyType string    `json:"currency_type" gorm:"column:currency_type"`
	AccountID    int64     `json:"account_id"    gorm:"column:account_id"`
	Money        float64   `json:"money"         gorm:"column:money"`
	HandlingFee  float64   `json:"handling_fee"  gorm:"column:handling_fee"`
	ActualAmount float64   `json:"actual_amount" gorm:"column:actual_amount"`
	Status       int8      `json:"status"        gorm:"column:status"` // 1已打款 2待处理
	Remark       string    `json:"remark"        gorm:"column:remark"`
	CreatedAt    time.Time `json:"created_at"    gorm:"column:created_at"`
	UpdatedAt    time.Time `json:"updated_at"    gorm:"column:updated_at"`
}

func (Withdraw) TableName() string { return "withdraws" }

// WithdrawAccount 收款账户
type WithdrawAccount struct {
	ID          int64     `json:"id"           gorm:"primaryKey"`
	UserID      int64     `json:"user_id"      gorm:"column:user_id"`
	Username    string    `json:"username"     gorm:"column:username"`
	Account     string    `json:"account"      gorm:"column:account"`
	AccountType int8      `json:"account_type" gorm:"column:account_type"`
	Bank        *string   `json:"bank"         gorm:"column:bank"`
	Phone       string    `json:"phone"        gorm:"column:phone"`
	Qrcode      *string   `json:"qrcode"       gorm:"column:qrcode"`
	CreatedAt   time.Time `json:"created_at"   gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at"   gorm:"column:updated_at"`
}

func (WithdrawAccount) TableName() string { return "withdraw_accounts" }

// WithdrawApproveReq 提现审批请求
type WithdrawApproveReq struct {
	Status int8   `json:"status" binding:"required"` // 1通过 3拒绝
	Remark string `json:"remark"`
}

// Rule 菜单规则（对标老系统，145条，树形）
type Rule struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	Title     string    `json:"title"      gorm:"column:title"`
	Name      string    `json:"name"       gorm:"column:name"`
	Icon      string    `json:"icon"       gorm:"column:icon"`
	Key       string    `json:"key"        gorm:"column:key"`
	PID       int64     `json:"pid"        gorm:"column:pid"`
	Href      string    `json:"href"       gorm:"column:href"`
	Type      int8      `json:"type"       gorm:"column:type"` // 0菜单组 1菜单项
	Weight    int       `json:"weight"     gorm:"column:weight"`
	Value     int64     `json:"value"      gorm:"column:value"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
	Children  []*Rule   `json:"children,omitempty" gorm:"-"` // 树形子节点
}

func (Rule) TableName() string { return "rules" }

// Role 角色（对标老系统，1条）
type Role struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	Name      string    `json:"name"       gorm:"column:name"`
	Rules     string    `json:"rules"      gorm:"column:rules"`
	Status    int8      `json:"status"     gorm:"column:status"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Role) TableName() string { return "roles" }

// 财务日志（对标老系统）
type CouponLog struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	UserID    int64     `json:"user_id"    gorm:"column:user_id"`
	Type      int8      `json:"type"       gorm:"column:type"`
	Money     float64   `json:"money"      gorm:"column:money"`
	Before    float64   `json:"before"     gorm:"column:before"`
	After     float64   `json:"after"      gorm:"column:after"`
	Memo      string    `json:"memo"       gorm:"column:memo"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (CouponLog) TableName() string { return "coupon_logs" }

type SelfBonusLog struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	UserID    int64     `json:"user_id"    gorm:"column:user_id"`
	Type      int8      `json:"type"       gorm:"column:type"`
	Money     float64   `json:"money"      gorm:"column:money"`
	Before    float64   `json:"before"     gorm:"column:before"`
	After     float64   `json:"after"      gorm:"column:after"`
	Memo      string    `json:"memo"       gorm:"column:memo"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (SelfBonusLog) TableName() string { return "self_bonus_logs" }

type ShareBonusLog struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	UserID    int64     `json:"user_id"    gorm:"column:user_id"`
	Type      int8      `json:"type"       gorm:"column:type"`
	Money     float64   `json:"money"      gorm:"column:money"`
	Before    float64   `json:"before"     gorm:"column:before"`
	After     float64   `json:"after"      gorm:"column:after"`
	Memo      string    `json:"memo"       gorm:"column:memo"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (ShareBonusLog) TableName() string { return "share_bonus_logs" }

// LogListReq 日志列表查询
type LogListReq struct {
	Page      int    `form:"page"       binding:"required,min=1"`
	Limit     int    `form:"limit"      binding:"required,min=1,max=100"`
	UserID    *int64 `form:"user_id"`
	Type      *int8  `form:"type"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

// Pagination 通用分页参数
type Pagination struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Count int64 `json:"count"`
}
