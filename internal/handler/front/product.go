package front

import (
	"fmt"

	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
)

// Products 商品列表 GET /api/v1/front/products
func Products(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)

	goods, count, err := repository.ListGoods(page, limit, nil, "")
	if err != nil {
		app.InternalError(c, "获取失败")
		return
	}
	app.OKWithCount(c, goods, count)
}

// ProductDetail 商品详情 GET /api/v1/front/products/:id
func ProductDetail(c *gin.Context) {
	id := parseIntParam(c, "id")
	good, err := repository.GetGoodByID(id)
	if err != nil || good.Status != 1 {
		app.NotFound(c, "商品不存在或已下架")
		return
	}
	// 查 Redis 实时库存
	stock, _ := repository.GetProductStock(c.Request.Context(), id)
	if stock < 0 {
		stock = 0
	}
	app.OK(c, gin.H{
		"product": good,
		"stock":   stock,
	})
}

func queryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// Announcement 系统公告 GET /api/v1/front/announcement
func Announcement(c *gin.Context) {
	var val string
	repository.DB.Table("system_configs").
		Select("config_value").
		Where("config_key = ?", "notice_content").
		Scan(&val)
	app.OK(c, gin.H{"content": val})
}

// Categories 商品分类 GET /api/v1/front/categories（只返回有上架商品的分类）
func Categories(c *gin.Context) {
	var list []map[string]interface{}
	repository.DB.Table("categories c").
		Select("DISTINCT c.*").
		Joins("INNER JOIN goods g ON g.category_id = c.id AND g.status = 1").
		Where("c.status = 1").
		Order("c.sort ASC, c.id ASC").
		Find(&list)
	if list == nil {
		list = []map[string]interface{}{}
	}
	app.OK(c, list)
}

// Merchandises 寄售商品列表 GET /api/v1/front/merchandises?page=1&limit=10
func Merchandises(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)

	var list []map[string]interface{}
	var count int64
	repository.DB.Table("merchandises").Where("status = 0 AND is_show = 1").Count(&count)
	repository.DB.Table("merchandises").
		Where("status = 0 AND is_show = 1").
		Order("id DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&list)
	if list == nil {
		list = []map[string]interface{}{}
	}
	app.OKWithCount(c, list, count)
}

// Agreements 用户协议 GET /api/v1/front/agreements
func Agreements(c *gin.Context) {
	data := make(map[string]string)
	for _, k := range []string{"agreement_user", "agreement_consignment"} {
		var v string
		repository.DB.Table("system_configs").Select("config_value").Where("config_key = ?", k).Scan(&v)
		data[k] = v
	}
	app.OK(c, data)
}

// Banners 轮播图列表 GET /api/v1/front/banners
func Banners(c *gin.Context) {
	var list []map[string]interface{}
	repository.DB.Table("banners").Where("status = 1").Order("sort ASC, id DESC").Find(&list)
	if list == nil {
		list = []map[string]interface{}{}
	}
	app.OK(c, list)
}

func parseIntParam(c *gin.Context, key string) int64 {
	var v int64
	fmt.Sscanf(c.Param(key), "%d", &v)
	return v
}
