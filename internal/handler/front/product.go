package front

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"
	"commercial-transactions-service/pkg/utils"

	"github.com/gin-gonic/gin"
)

// Products 商品列表 GET /api/v1/front/products
func Products(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)

	goods, count, err := repository.ListGoods(page, limit, nil, c.Query("keyword"))
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
	var phone string
	repository.DB.Table("system_configs").Select("config_value").Where("config_key=?", "service_phone").Scan(&phone)
	app.OK(c, gin.H{
		"product": good,
		"stock":   stock,
		"service_phone": phone,
		"tags":          []string{"商城自营", "包邮"},
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
// mine=1 时返回当前用户名下的全部寄售商品（含已售），用于卖方仓库展示
func Merchandises(c *gin.Context) {
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)

	var list []map[string]interface{}
	var count int64
	q := repository.DB.Table("merchandises")
	if c.Query("mine") == "1" {
		q = q.Where("user_id = ?", c.GetInt64("user_id"))
	} else {
		q = q.Where("status = 0 AND is_show = 1")
	}
	q.Count(&count)
	q.Order("id DESC").
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
	for _, k := range []string{"agreement_user", "agreement_consignment", "agreement_purchase_notice", "agreement_backup"} {
		var v string
		repository.DB.Table("system_configs").Select("config_value").Where("config_key = ?", k).Scan(&v)
		data[k] = v
	}
	app.OK(c, data)
}

// MerchandiseDetail 寄售商品详情 GET /api/v1/front/merchandises/:id
func MerchandiseDetail(c *gin.Context) {
	id := parseIntParam(c, "id")
	var m map[string]interface{}
	repository.DB.Table("merchandises").Where("id = ? AND status = 0 AND is_show = 1", id).Take(&m)
	if m == nil {
		app.NotFound(c, "商品不存在或已下架")
		return
	}
	var phone string
	repository.DB.Table("system_configs").Select("config_value").Where("config_key=?", "service_phone").Scan(&phone)
	m["service_phone"] = phone
	m["tags"] = []string{"商城自营", "包邮"}
	app.OK(c, m)
}

// MyFans 我的粉丝 GET /api/v1/front/user/fans
func MyFans(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var list []map[string]interface{}
	repository.DB.Raw(`
		SELECT u.id, u.username, u.nickname, u.avatar, u.mobile, u.created_at,
			(SELECT COUNT(*) FROM users WHERE pid = u.id) as invite_count
		FROM users u WHERE u.pid = ?
		ORDER BY u.created_at DESC
	`, uid).Scan(&list)
	if list == nil { list = []map[string]interface{}{} }
	app.OK(c, list)
}

// UploadFile C端文件上传 POST /api/v1/front/upload
func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		app.BadRequest(c, "请选择文件")
		return
	}
	if file.Size > 10*1024*1024 {
		app.BadRequest(c, "文件大小不能超过10MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true}
	if !allowed[ext] {
		app.BadRequest(c, "不支持的图片格式")
		return
	}
	filename := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), utils.RandStr(8), ext)
	savePath := filepath.Join("upload", "image", time.Now().Format("20060102"), filename)
	fullPath := filepath.Join(".", savePath)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		app.InternalError(c, "上传失败")
		return
	}
	app.OK(c, gin.H{"url": "/" + filepath.ToSlash(savePath)})
}

// ContractContent 合同内容（公开，签署页用）GET /api/v1/front/config/contract-content
func ContractContent(c *gin.Context) {
	var url string
	repository.DB.Table("system_configs").Select("config_value").Where("config_key=?", "contract_content").Scan(&url)
	url = strings.TrimSpace(url)
	url = strings.TrimPrefix(url, "<p>")
	url = strings.TrimSuffix(url, "</p>")
	app.OK(c, gin.H{"url": url, "type": "pdf"})
}

// ServicePhone 客服电话+短信开关 GET /api/v1/front/config/service-phone
func ServicePhone(c *gin.Context) {
	var phone string
	repository.DB.Table("system_configs").Select("config_value").Where("config_key=?", "service_phone").Scan(&phone)
	app.OK(c, gin.H{
		"phone":           phone,
		"sms_verify":      repository.GetConfigInt("sms_verify", 0),
		"resell_deadline": repository.GetConfigStr("resell_deadline"),
	})
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

// BuyMerchandiseReq 购买寄售商品请求
type BuyMerchandiseReq struct {
	Consignee string `json:"consignee" binding:"required"`
	Phone     string `json:"phone"     binding:"required"`
}

// BuyMerchandise 购买寄售商品 POST /api/v1/front/merchandises/:id/buy
func BuyMerchandise(c *gin.Context) {
	uid := c.GetInt64("user_id")
	mercID := parseIntParam(c, "id")

	// 商城寄售总开关：关闭时禁止购买
	if repository.GetConfigInt("resell_open", 1) == 0 {
		app.Fail(c, app.ErrCodeFlashSaleClosed, "抢购活动已经结束")
		return
	}

	var req BuyMerchandiseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请填写收货人姓名和手机号")
		return
	}

	// 查寄售商品
	var merc model.Merchandise
	if err := repository.DB.Where("id = ? AND status = 0 AND is_show = 1", mercID).First(&merc).Error; err != nil {
		app.NotFound(c, "商品不存在或已售出")
		return
	}

	// 不能买自己的商品
	if merc.UserID == uid {
		app.BadRequest(c, "不能购买自己的商品")
		return
	}

	// 每日限购校验（与 FlashSaleBuy 一致）
	user, err := repository.GetUserByID(uid)
	if err != nil || user == nil {
		app.NotFound(c, "用户不存在")
		return
	}
	userMaxOrder := user.MaxOrder
	if userMaxOrder <= 0 {
		app.Fail(c, app.ErrCodeLimitExceeded, "未配置抢购上限，无法抢购")
		return
	}
	effectiveCap := userMaxOrder
	if user.IsPriority == 1 && isInPriorityWindow() {
		priorityCap := repository.GetConfigInt("priority_max_orders", 0)
		if priorityCap > 0 && priorityCap < effectiveCap {
			effectiveCap = priorityCap
		}
	}

	now := time.Now()
	todayKey := fmt.Sprintf("flash:user:daily:%d:%s", uid, now.Format("20060102"))
	dailyCount, redisErr := repository.RDB.Incr(c.Request.Context(), todayKey).Result()
	if redisErr != nil {
		app.InternalError(c, "系统繁忙，请重试")
		return
	}
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, repository.CSTLocation())
	repository.RDB.ExpireAt(c.Request.Context(), todayKey, endOfDay)

	if dailyCount > int64(effectiveCap) {
		repository.RDB.Decr(c.Request.Context(), todayKey)
		app.Fail(c, app.ErrCodeLimitExceeded,
			fmt.Sprintf("今日已抢购 %d 单，已达上限 %d 单", dailyCount-1, effectiveCap))
		return
	}

	orderSN := fmt.Sprintf("%d%07d", now.UnixMilli(), uid%10000000)

	// 创建订单
	order := model.Order{
		OrderSN:       orderSN,
		SellerID:      merc.UserID,
		BuyerID:       uid,
		MerchandiseID: merc.ID,
		TotalMoney:    merc.Price,
		Consignee:     req.Consignee,
		Phone:         req.Phone,
		IsShow:        1,
		Status:        0, // 待付款
		BuyTime:       &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repository.DB.Create(&order).Error; err != nil {
		app.InternalError(c, "订单创建失败")
		return
	}

	// 寄售商品标记已售（原子更新防并发超卖）
	result := repository.DB.Model(&model.Merchandise{}).
		Where("id = ? AND status = 0", merc.ID).
		Updates(map[string]interface{}{"status": int8(1), "updated_at": now})
	if result.RowsAffected == 0 {
		// 已被人抢先买了，回滚订单
		repository.RDB.Decr(c.Request.Context(), todayKey)
		repository.DB.Delete(&order)
		app.NotFound(c, "商品不存在或已售出")
		return
	}

	app.OK(c, gin.H{
		"msg":      "购买成功",
		"order_id": order.ID,
		"order_sn": order.OrderSN,
	})
}

func parseIntParam(c *gin.Context, key string) int64 {
	var v int64
	fmt.Sscanf(c.Param(key), "%d", &v)
	return v
}
