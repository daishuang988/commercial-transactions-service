package front

import (
	"context"
	"fmt"
	"sync"
	"time"

	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/internal/service"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
)

// 高频接口短时缓存（避免 1 万人同时请求打爆 DB）
var (
	cachedTime     interface{}
	cachedProducts interface{}
	cacheMu        sync.RWMutex
	cacheExpiry    time.Time
)

func getCachedTime() interface{} {
	cacheMu.RLock()
	if time.Now().Before(cacheExpiry) && cachedTime != nil {
		defer cacheMu.RUnlock()
		return cachedTime
	}
	cacheMu.RUnlock()
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedTime = repository.FlashSaleTimeInfo()
	cachedProducts = nil // 时间变了，商品缓存也失效
	cacheExpiry = time.Now().Add(1 * time.Second)
	return cachedTime
}

func getCachedProducts() interface{} {
	cacheMu.RLock()
	if time.Now().Before(cacheExpiry) && cachedProducts != nil {
		defer cacheMu.RUnlock()
		return cachedProducts
	}
	cacheMu.RUnlock()
	cacheMu.Lock()
	defer cacheMu.Unlock()
	events, _ := repository.ListFlashSaleEvents()
	productIDs := make([]int64, len(events))
	for i, e := range events { productIDs[i] = e.ProductID }
	goodsMap := repository.GetGoodsByIDs(productIDs)

	var products []model.FlashSaleProductResp
	for _, e := range events {
		good := goodsMap[e.ProductID]
		if good == nil || good.Status != 1 { continue }
		stock, _ := repository.GetProductStock(context.Background(), e.ProductID)
		if stock < 0 { stock = 0 }
		originPrice := e.Price
		if good.LinePrice > 0 { originPrice = good.LinePrice }
		status := e.Status
		if status == 1 && stock <= 0 { status = 2 }
		products = append(products, model.FlashSaleProductResp{
			ID: e.ID, Title: good.Title, Image: good.Images,
			Price: e.Price, OriginPrice: originPrice,
			Stock: stock, MaxPerUser: e.MaxPerUser, Status: status,
			StartTime: e.StartTime.Format("2006-01-02 15:04:05"),
			EndTime:   e.EndTime.Format("2006-01-02 15:04:05"),
		})
	}
	result := gin.H{"products": products, "is_open": repository.IsFlashSaleTime(), "time_info": cachedTime}
	cachedProducts = result
	cacheExpiry = time.Now().Add(1 * time.Second)
	return result
}

// FlashSaleBuy 抢购 POST /api/v1/front/flash-sale/buy
func FlashSaleBuy(c *gin.Context) {
	uid := c.GetInt64("user_id")

	var req model.FlashSaleBuyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请选择商品")
		return
	}

	// 1. 查找活动
	event, err := repository.GetFlashSaleEventByProductID(req.ProductID)
	if err != nil || event == nil {
		app.NotFound(c, "秒杀活动不存在")
		return
	}

	// 2. 获取用户信息（优先等级 + 个人限购数）
	user, err := repository.GetUserByID(uid)
	if err != nil || user == nil {
		app.NotFound(c, "用户不存在")
		return
	}
	isPriority := user.IsPriority == 1
	userMaxOrder := user.MaxOrder
	if userMaxOrder <= 0 {
		userMaxOrder = 1 // 兜底至少 1 单
	}

	// 3. 优先用户时间窗口提前
	now := time.Now().In(repository.CSTLocation())
	effectiveStart := event.StartTime
	if isPriority {
		advanceMin := repository.GetConfigInt("priority_advance_minutes", 0)
		effectiveStart = event.StartTime.Add(-time.Duration(advanceMin) * time.Minute)
	}

	// 全局窗口 + 活动时间双重校验
	if !repository.IsFlashSaleTime() && !isPriority {
		app.Fail(c, app.ErrCodeFlashSaleNotInTime, "不在抢购时间段内")
		return
	}
	if now.Before(effectiveStart) {
		app.Fail(c, app.ErrCodeFlashSaleNotInTime, "抢购尚未开始")
		return
	}
	if now.After(event.EndTime) {
		app.Fail(c, app.ErrCodeFlashSaleNotInTime, "抢购已结束")
		return
	}

	// 4. 购限校验: effectiveCap = MIN(系统配置优先上限, 用户个人上限)
	effectiveCap := userMaxOrder
	if isPriority {
		priorityCap := repository.GetConfigInt("priority_max_orders", 0)
		if priorityCap > 0 && priorityCap < effectiveCap {
			effectiveCap = priorityCap
		}
	}

	todayCount := repository.CountUserTodayOrders(uid)
	if int(todayCount) >= effectiveCap {
		app.Fail(c, app.ErrCodeLimitExceeded,
			fmt.Sprintf("今日已抢购 %d 单，已达上限 %d 单", todayCount, effectiveCap))
		return
	}

	// 5. 执行抢购
	result, err := service.ExecuteFlashSale(c.Request.Context(), uid, req.ProductID, 0)
	if err != nil {
		app.InternalError(c, "抢购失败，请重试")
		return
	}

	if !result.Success {
		app.Fail(c, app.ErrCodeSoldOut, result.Msg)
		return
	}

	app.OK(c, gin.H{"msg": "抢购成功！请等待系统确认订单"})
}

// FlashSaleTime 获取秒杀时间信息 GET /api/v1/front/flash-sale/time（3秒缓存）
func FlashSaleTime(c *gin.Context) {
	app.OK(c, getCachedTime())
}

// FlashSaleProducts 秒杀商品列表 GET /api/v1/front/flash-sale/products（3秒缓存）
func FlashSaleProducts(c *gin.Context) {
	app.OK(c, getCachedProducts())
}
