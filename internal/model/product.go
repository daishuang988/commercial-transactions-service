package model

import "time"

// Good 商品表（对标老系统，2条）
type Good struct {
	ID          int64     `json:"id"           gorm:"primaryKey"`
	CategoryID  int64     `json:"category_id"  gorm:"column:category_id"`
	Title       string    `json:"title"        gorm:"column:title"`
	Images      string    `json:"images"       gorm:"column:images"`
	Price       float64   `json:"price"        gorm:"column:price"`
	LinePrice   float64   `json:"line_price"   gorm:"column:line_price"`
	StockNum    int       `json:"stock_num"    gorm:"column:stock_num"`
	SalesVolume int       `json:"sales_volume" gorm:"column:sales_volume"`
	Content     string    `json:"content"      gorm:"column:content"`
	Notes       *string   `json:"notes"        gorm:"column:notes"`
	Status      int8      `json:"status"       gorm:"column:status"` // 0下架 1上架
	CreatedAt   time.Time `json:"created_at"   gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at"   gorm:"column:updated_at"`
}

func (Good) TableName() string { return "goods" }

// Category 商品分类（对标老系统，1条）
type Category struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	PID       int64     `json:"pid"        gorm:"column:pid"`
	Title     string    `json:"title"      gorm:"column:title"`
	Image     string    `json:"image"      gorm:"column:image"`
	Sort      int       `json:"sort"       gorm:"column:sort"`
	Status    int8      `json:"status"     gorm:"column:status"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Category) TableName() string { return "categories" }

// Merchandise 寄售商品（对标老系统，73,161条）
type Merchandise struct {
	ID        int64     `json:"id"         gorm:"primaryKey"`
	OldID     *int64    `json:"old_id"     gorm:"column:old_id"`
	UserID    int64     `json:"user_id"    gorm:"column:user_id"`
	Title     string    `json:"title"      gorm:"column:title"`
	Image     string    `json:"image"      gorm:"column:image"`
	Price     float64   `json:"price"      gorm:"column:price"`
	IsShow    int8      `json:"is_show"    gorm:"column:is_show"`
	Status    int8      `json:"status"     gorm:"column:status"` // 0待售 1已售
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (Merchandise) TableName() string { return "merchandises" }

// FlashSaleBuyReq 抢购请求
type FlashSaleBuyReq struct {
	ProductID int64 `json:"product_id" binding:"required"`
}

// FlashSaleProductResp 前端展示的秒杀商品
type FlashSaleProductResp struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Image        string  `json:"image"`
	Price        float64 `json:"price"`         // 秒杀价
	OriginPrice  float64 `json:"origin_price"`  // 原价
	Stock        int     `json:"stock"`         // 剩余库存
	MaxPerUser   int     `json:"max_per_user"`  // 每人限购
	Status       int8    `json:"status"`        // 0等待抢购 1立即抢购 2已售罄 3已结束
	StartTime    string  `json:"start_time"`
	EndTime      string  `json:"end_time"`
}
