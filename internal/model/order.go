package model

import "time"

// Order 订单表（对标老系统，86,650条）
type Order struct {
	ID             int64      `json:"id"              gorm:"primaryKey"`
	OldID          *int64     `json:"old_id"          gorm:"column:old_id"`
	OrderSN        string     `json:"order_sn"        gorm:"column:order_sn"`
	SellerID       int64      `json:"seller_id"       gorm:"column:seller_id"`
	BuyerID        int64      `json:"buyer_id"        gorm:"column:buyer_id"`
	MerchandiseID  int64      `json:"merchandise_id"  gorm:"column:merchandise_id"`
	TotalMoney     float64    `json:"total_money"     gorm:"column:total_money"`
	IsResell       int8       `json:"is_resell"       gorm:"column:is_resell"`
	IsShow         int8       `json:"is_show"         gorm:"column:is_show"`
	Consignee      string     `json:"consignee"       gorm:"column:consignee"`
	Phone          string     `json:"phone"           gorm:"column:phone"`
	Province       string     `json:"province"        gorm:"column:province"`
	City           string     `json:"city"            gorm:"column:city"`
	Area           string     `json:"area"            gorm:"column:area"`
	Address        string     `json:"address"         gorm:"column:address"`
	PayImg         string     `json:"pay_img"         gorm:"column:pay_img"`

	PayTime        *time.Time `json:"pay_time"        gorm:"column:pay_time"`
	BuyTime        *time.Time `json:"buy_time"        gorm:"column:buy_time"`     // 抢单时间
	ConfirmTime    *time.Time `json:"confirm_time"    gorm:"column:confirm_time"`
	Status         int8       `json:"status"          gorm:"column:status"`       // 1待确认 2已完成
	CreatedAt      time.Time  `json:"created_at"      gorm:"column:created_at"`
	UpdatedAt      time.Time  `json:"updated_at"      gorm:"column:updated_at"`
}

func (Order) TableName() string { return "orders" }

// OrderListReq 订单列表查询（对标老系统 /order/select）
type OrderListReq struct {
	Page        int    `form:"page"         binding:"required,min=1"`
	Limit       int    `form:"limit"        binding:"required,min=1,max=100"`
	Status      *int8  `form:"status"`       // 筛选：订单状态
	SellerID    *int64 `form:"seller_id"`    // 筛选：卖家
	BuyerID     *int64 `form:"buyer_id"`     // 筛选：买家
	Keyword     string `form:"keyword"`      // 搜索：订单号/收货人/电话
	StartTime   string `form:"start_time"`   // 时间范围
	EndTime     string `form:"end_time"`
}

// FlashSaleRecord 抢购记录（Redis→MySQL异步写入）
type FlashSaleRecord struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	UserID    int64     `json:"user_id"    gorm:"column:user_id"`
	ProductID int64     `json:"product_id" gorm:"column:product_id"`
	EventID   int64     `json:"event_id"   gorm:"column:event_id"`
	Price     float64   `json:"price"      gorm:"column:price"`
	Status    int8      `json:"status"     gorm:"column:status"` // 1抢购成功
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}

func (FlashSaleRecord) TableName() string { return "flash_sale_records" }

// FlashSaleEvent 秒杀活动
type FlashSaleEvent struct {
	ID          int64     `json:"id"           gorm:"primaryKey"`
	ProductID   int64     `json:"product_id"   gorm:"column:product_id"`
	Stock       int       `json:"stock"        gorm:"column:stock"`
	Price       float64   `json:"price"        gorm:"column:price"`
	StartTime   time.Time `json:"start_time"   gorm:"column:start_time"`
	EndTime     time.Time `json:"end_time"     gorm:"column:end_time"`
	MaxPerUser  int       `json:"max_per_user" gorm:"column:max_per_user"`
	Status      int8      `json:"status"       gorm:"column:status"` // 0未开始 1进行中 2已结束
	CreatedAt   time.Time `json:"created_at"   gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at"   gorm:"column:updated_at"`
}

func (FlashSaleEvent) TableName() string { return "flash_sale_events" }
