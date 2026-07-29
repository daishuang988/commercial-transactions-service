package front

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
	for i, e := range events {
		productIDs[i] = e.ProductID
	}
	goodsMap := repository.GetGoodsByIDs(productIDs)

	var products []model.FlashSaleProductResp
	for _, e := range events {
		good := goodsMap[e.ProductID]
		if good == nil || good.Status != 1 {
			continue
		}
		stock, _ := repository.GetProductStock(context.Background(), e.ProductID)
		if stock < 0 {
			stock = 0
		}
		originPrice := e.Price
		if good.LinePrice > 0 {
			originPrice = good.LinePrice
		}
		status := e.Status
		if status == 1 && stock <= 0 {
			status = 2
		}
		products = append(products, model.FlashSaleProductResp{
			ID: e.ID, Title: good.Title, Image: good.Images,
			Price: e.Price, OriginPrice: originPrice,
			Stock: stock, MaxPerUser: e.MaxPerUser, Status: status,
			StartTime: e.StartTime.Format("2006-01-02 15:04:05"),
			EndTime:   e.EndTime.Format("2006-01-02 15:04:05"),
		})
	}
	// 读取抢购商品配置: "开关,数量,单位" 如 "1,3044,套"
	cfgStr := repository.GetConfigStr("flash_sale_product")
	enabled, showCount, unit := parseProductConfig(cfgStr)
	if enabled == 0 {
		products = nil // 不显示抢购商品
	} else if showCount > 0 && len(products) > showCount {
		products = products[:showCount]
	}

	result := gin.H{
		"products":        products,
		"is_open":         repository.IsFlashSaleTime(),
		"time_info":       cachedTime,
		"product_enabled": enabled,
		"product_count":   showCount,
		"product_unit":    unit,
	}
	cachedProducts = result
	cacheExpiry = time.Now().Add(1 * time.Second)
	return result
	cachedProducts = result
	cacheExpiry = time.Now().Add(1 * time.Second)
	return result
}

// parseProductConfig 解析 flash_sale_product 配置: "1,3044,套" → 开关, 数量, 单位
func parseProductConfig(s string) (int, int, string) {
	if s == "" {
		return 0, 0, ""
	}
	parts := strings.SplitN(s, ",", 3)
	if len(parts) < 3 {
		return 0, 0, ""
	}
	enabled, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	count, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	unit := strings.TrimSpace(parts[2])
	return enabled, count, unit
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
	// max_order=0 表示未配置，不允许抢购
	if userMaxOrder <= 0 {
		app.Fail(c, app.ErrCodeLimitExceeded, "未配置抢购上限，无法抢购")
		return
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

	// 4. 购限校验: 优先用户在优先窗口内受 priority_max_orders 限制，其余时间仅受个人上限
	inPriority := isPriority && isInPriorityWindow()
	effectiveCap := userMaxOrder
	if inPriority {
		priorityCap := repository.GetConfigInt("priority_max_orders", 0)
		if priorityCap > 0 && priorityCap < effectiveCap {
			effectiveCap = priorityCap
		}
	}

	// Redis 原子计数：防止异步落库导致的重复抢购
	todayKey := fmt.Sprintf("flash:user:daily:%d:%s", uid, now.Format("20060102"))
	dailyCount, redisErr := repository.RDB.Incr(c.Request.Context(), todayKey).Result()
	if redisErr != nil {
		app.InternalError(c, "系统繁忙，请重试")
		return
	}
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, repository.CSTLocation())
	repository.RDB.ExpireAt(c.Request.Context(), todayKey, endOfDay)

	if dailyCount > int64(effectiveCap) {
		repository.RDB.Decr(c.Request.Context(), todayKey) // 回滚计数
		app.Fail(c, app.ErrCodeLimitExceeded,
			fmt.Sprintf("今日已抢购 %d 单，已达上限 %d 单", dailyCount-1, effectiveCap))
		return
	}

	// 5. 执行抢购
	result, err := service.ExecuteFlashSale(c.Request.Context(), uid, req.ProductID, event.Price)
	if err != nil {
		repository.RDB.Decr(c.Request.Context(), todayKey) // 回滚计数
		app.InternalError(c, "抢购失败，请重试")
		return
	}

	if !result.Success {
		repository.RDB.Decr(c.Request.Context(), todayKey) // 回滚计数
		app.Fail(c, app.ErrCodeSoldOut, result.Msg)
		return
	}

	// 6. 从寄售商品池取一单可用商品 → 生成订单
	var merc model.Merchandise
	if err := repository.DB.Where("status = 0 AND is_show = 1").Order("RAND()").First(&merc).Error; err != nil {
		app.InternalError(c, "暂无可抢商品")
		return
	}

	// 原子抢占：标记商品已售
	upRes := repository.DB.Model(&model.Merchandise{}).
		Where("id = ? AND status = 0", merc.ID).
		Updates(map[string]interface{}{"status": int8(1), "updated_at": now})
	if upRes.RowsAffected == 0 {
		app.InternalError(c, "商品已被抢走，请重试")
		return
	}

	// 生成订单
	orderSN := fmt.Sprintf("%d%07d", now.UnixMilli(), uid%10000000)
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

	app.OK(c, gin.H{
		"msg":      "抢购成功",
		"order_id": order.ID,
		"order_sn": order.OrderSN,
	})
}

// FlashSaleTime 获取秒杀时间信息 GET /api/v1/front/flash-sale/time（3秒缓存）
func FlashSaleTime(c *gin.Context) {
	app.OK(c, getCachedTime())
}

// isInPriorityWindow 当前是否在优先用户时间窗口内
func isInPriorityWindow() bool {
	now := time.Now().In(repository.CSTLocation())
	advanceMin := repository.GetConfigInt("priority_advance_minutes", 0)
	if advanceMin <= 0 {
		return false
	}
	startStr := repository.GetConfigStr("flash_sale_start")
	endStr := repository.GetConfigStr("flash_sale_end")
	if startStr == "" || endStr == "" {
		return false
	}
	var sh, sm, eh, em int
	fmt.Sscanf(startStr, "%d:%d", &sh, &sm)
	fmt.Sscanf(endStr, "%d:%d", &eh, &em)
	priorityStart := time.Date(now.Year(), now.Month(), now.Day(), sh, sm, 0, 0, repository.CSTLocation()).
		Add(-time.Duration(advanceMin) * time.Minute)
	normalEnd := time.Date(now.Year(), now.Month(), now.Day(), eh, em, 0, 0, repository.CSTLocation())
	return !now.Before(priorityStart) && now.Before(normalEnd)
}

// FlashSaleRemaining 剩余可抢次数 GET /api/v1/front/flash-sale/remaining
func FlashSaleRemaining(c *gin.Context) {
	uid := c.GetInt64("user_id")
	now := time.Now().In(repository.CSTLocation())

	user, err := repository.GetUserByID(uid)
	if err != nil || user == nil {
		app.NotFound(c, "用户不存在")
		return
	}

	maxOrder := user.MaxOrder
	if maxOrder <= 0 {
		app.OK(c, gin.H{
			"max_order":          0,
			"is_priority":        user.IsPriority == 1,
			"in_priority_window": false,
			"priority_max_orders": repository.GetConfigInt("priority_max_orders", 0),
			"effective_cap":       0,
			"today_count":         0,
			"remaining":           0,
		})
		return
	}

	// effectiveCap: 优先用户在优先窗口内受 priority_max_orders 限制，其余时间仅受个人上限
	priorityMaxOrders := repository.GetConfigInt("priority_max_orders", 0)
	inPriority := user.IsPriority == 1 && isInPriorityWindow()
	effectiveCap := maxOrder
	if inPriority {
		if priorityMaxOrders > 0 && priorityMaxOrders < effectiveCap {
			effectiveCap = priorityMaxOrders
		}
	}

	// 从 Redis 读实时计数
	todayKey := fmt.Sprintf("flash:user:daily:%d:%s", uid, now.Format("20060102"))
	val, err := repository.RDB.Get(c.Request.Context(), todayKey).Result()
	todayCount := 0
	if err == nil && val != "" {
		fmt.Sscanf(val, "%d", &todayCount)
	}

	remaining := effectiveCap - todayCount
	if remaining < 0 {
		remaining = 0
	}

	app.OK(c, gin.H{
		"max_order":           maxOrder,
		"is_priority":         user.IsPriority == 1,
		"in_priority_window":  inPriority,
		"priority_max_orders": priorityMaxOrders,
		"effective_cap":       effectiveCap,
		"today_count":         todayCount,
		"remaining":           remaining,
	})
}

// FlashSaleProducts 秒杀商品列表 GET /api/v1/front/flash-sale/products（3秒缓存）
func FlashSaleProducts(c *gin.Context) {
	app.OK(c, getCachedProducts())
}
