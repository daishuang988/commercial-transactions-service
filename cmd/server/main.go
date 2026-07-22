package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"commercial-transactions-service/internal/config"
	"commercial-transactions-service/internal/handler/admin"
	"commercial-transactions-service/internal/handler/front"
	"commercial-transactions-service/internal/middleware"
	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/internal/service"
	"commercial-transactions-service/pkg/app"
	"commercial-transactions-service/pkg/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	middleware.InitJWT(&cfg.JWT)
	front.Init(cfg)
	repository.InitMySQL(&cfg.MySQL)
	repository.InitRedis(&cfg.Redis)
	repository.InitMemoryStore() // 内存版秒杀（Redis不可用时降级）
	defer repository.Close()

	// 启动异步落库Worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartOrderWorker(ctx, cfg.FlashSale.WorkerCount, cfg.FlashSale.BatchSize, cfg.FlashSale.BatchIntervalMs)

	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.RequestLogger(), middleware.CORS())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		dbOK := repository.DB != nil && repository.DB.Raw("SELECT 1").Error == nil
		redisOK := repository.RDB != nil && repository.RDB.Ping(c.Request.Context()).Err() == nil
		if dbOK && redisOK {
			c.String(http.StatusOK, "ok")
		} else {
			c.String(http.StatusServiceUnavailable, fmt.Sprintf("db=%v redis=%v", dbOK, redisOK))
		}
	})

	// 静态文件（上传的图片等）
	r.Static("/upload", "./upload")

	// ============ C端 ============
	fg := r.Group("/api/v1/front")
	{
		fg.POST("/auth/login", front.Login)
		fg.POST("/auth/register-v2", front.RegisterV2)
		fg.POST("/auth/reset-password", front.ResetPassword)
		fg.GET("/flash-sale/time", front.FlashSaleTime)
		fg.GET("/banners", front.Banners)
		fg.GET("/agreements", front.Agreements)
		fg.GET("/categories", front.Categories)
		fg.GET("/announcement", front.Announcement)

		auth := fg.Group("", middleware.AuthRequired())
		{
			// 秒杀
			auth.GET("/flash-sale/products", front.FlashSaleProducts)
			auth.POST("/flash-sale/buy", middleware.RateLimiter(500), front.FlashSaleBuy)

			// 商品
			auth.GET("/products", front.Products)
			auth.GET("/products/:id", front.ProductDetail)
			auth.GET("/merchandises", front.Merchandises)

			// 订单
			auth.GET("/orders", front.MyOrders)
			auth.GET("/orders/:id", front.MyOrderDetail)

			// 用户
			auth.GET("/user/profile", front.Profile)
			auth.GET("/user/wallet", front.Wallet)
			auth.PUT("/user/password", front.ChangePassword)
			auth.GET("/logs/coupon", front.CouponLogs)
			auth.GET("/logs/self-bonus", front.SelfBonusLogs)
			auth.GET("/logs/share-bonus", front.ShareBonusLogs)
		}
	}

	// ============ 管理端 ============
	ag := r.Group("/api/v1/admin")
	{
		ag.POST("/auth/login", adminLogin)

		auth := ag.Group("", middleware.AdminAuthRequired())
		{
			// Dashboard
			auth.GET("/dashboard", admin.Dashboard)

			// 系统配置 & 账户
			auth.GET("/config", admin.GetConfig)
			auth.PUT("/config", admin.UpdateConfig)
			auth.GET("/account/info", admin.GetAccountInfo)
			auth.PUT("/account/info", admin.UpdateAccountInfo)
			auth.PUT("/account/password", admin.ChangePassword)

			// 用户管理
			auth.GET("/users", admin.ListUsers)
			auth.GET("/users/:id", admin.GetUser)
			auth.PUT("/users/:id", admin.UpdateUser)
			auth.PUT("/users/:id/status", admin.UpdateUserStatus)
			auth.PUT("/users/:id/parent", admin.UpdateUserParent)
			auth.POST("/users/:id/recharge", admin.Recharge)
			auth.POST("/users/batch-delete", admin.BatchDeleteUsers)

			// 订单管理
			auth.GET("/orders", admin.ListOrders)
			auth.POST("/orders/search", admin.SearchOrders)
			auth.GET("/orders/:id", admin.GetOrder)
			auth.PUT("/orders/:id/status", admin.UpdateOrderStatus)

			// 兑换订单
			auth.GET("/exchange-orders", admin.ListExchangeOrders)

			// 文件上传
			auth.POST("/upload", admin.UploadImage)

			// 分类管理
			auth.GET("/categories", admin.ListCategories)
			auth.POST("/categories", admin.CreateCategory)
			auth.PUT("/categories/:id", admin.UpdateCategory)
			auth.DELETE("/categories/:id", admin.DeleteCategory)

			// 商品管理
			auth.GET("/goods", admin.ListGoods)
			auth.POST("/goods", admin.CreateGood)
			auth.PUT("/goods/:id", admin.UpdateGood)
			auth.DELETE("/goods/:id", admin.DeleteGood)
			auth.PUT("/goods/:id/stock", admin.UpdateGoodStock)

			// 寄售商品
			auth.GET("/merchandises", admin.ListMerchandises)
			auth.POST("/merchandises/search", admin.SearchMerchandises)
			auth.POST("/merchandises", admin.CreateMerchandise)
			auth.PUT("/merchandises/:id/status", admin.UpdateMerchandiseStatus)
			auth.DELETE("/merchandises/:id", admin.DeleteMerchandise)

			// 提现管理
			auth.GET("/withdraws", admin.ListWithdraws)
			auth.POST("/withdraws/search", admin.SearchWithdraws)
			auth.PUT("/withdraws/:id/approve", admin.ApproveWithdraw)

			// 财务日志
			auth.GET("/logs/money", admin.ListMoneyLogs)
			auth.GET("/logs/coupon", admin.ListCouponLogs)
			auth.GET("/logs/self-bonus", admin.ListSelfBonusLogs)
			auth.GET("/logs/share-bonus", admin.ListShareBonusLogs)
			auth.POST("/logs/money/search", admin.SearchMoneyLogs)
			auth.POST("/logs/coupon/search", admin.SearchCouponLogs)
			auth.POST("/logs/self-bonus/search", admin.SearchSelfBonusLogs)
			auth.POST("/logs/share-bonus/search", admin.SearchShareBonusLogs)

			// 菜单规则
			auth.GET("/rules", admin.ListRules)
			auth.GET("/rules/tree", admin.RuleTree)

			// 权限管理
			auth.GET("/admins", admin.ListAdmins)
			auth.POST("/admins", admin.CreateAdmin)
			auth.PUT("/admins/:id", admin.UpdateAdmin)
			auth.PUT("/admins/:id/password", admin.ResetAdminPassword)
			auth.DELETE("/admins/:id", admin.DeleteAdmin)
			auth.GET("/roles", admin.ListRoles)
			auth.POST("/roles", admin.CreateRole)
			auth.PUT("/roles/:id", admin.UpdateRole)
			auth.DELETE("/roles/:id", admin.DeleteRole)

			// 内容管理
			auth.GET("/banners", admin.ListBanners)
			auth.POST("/banners", admin.CreateBanner)
			auth.PUT("/banners/:id", admin.UpdateBanner)
			auth.DELETE("/banners/:id", admin.DeleteBanner)
			auth.POST("/banners/batch-delete", admin.BatchDeleteBanners)
			auth.GET("/ads", admin.ListAds)
			auth.POST("/ads", admin.CreateAd)
			auth.PUT("/ads/:id", admin.UpdateAd)
			auth.DELETE("/ads/:id", admin.DeleteAd)

			// 秒杀管理
			auth.GET("/flash-sale/events", admin.ListFlashSaleEvents)
			auth.POST("/flash-sale/events", admin.CreateFlashSaleEvent)
		}
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("🚀 服务启动: %s", addr)

	go func() {
		if err := r.Run(addr); err != nil {
			log.Fatalf("启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭...")
	cancel()
	time.Sleep(2 * time.Second)
}

func adminLogin(c *gin.Context) {
	var req model.AdminLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}

	adminUser, err := repository.GetAdminByUsername(req.Username)
	if err != nil {
		// 用户名查不到，试试手机号
		adminUser, err = repository.GetAdminByUsername(req.Username)
	}
	if err == nil && adminUser == nil {
		err = fmt.Errorf("not found")
	}
	if err != nil {
		app.Fail(c, app.ErrCodeUserNotFound, "管理员不存在")
		return
	}
	// 禁用检查
	if adminUser.Status != nil && *adminUser.Status == 0 {
		app.Fail(c, app.ErrCodeUserFrozen, "该账号已被禁用，请联系超管")
		return
	}

	// 密码校验（bcrypt 或老系统明文）
	if !utils.CheckPassword(req.Password, adminUser.Password) && req.Password != adminUser.Password {
		app.Fail(c, app.ErrCodePasswordWrong, "密码错误")
		return
	}

	token, err := middleware.GenerateToken(adminUser.ID, adminUser.Username, true, front.Cfg.JWT.ExpireHours)
	if err != nil {
		app.InternalError(c, "生成Token失败")
		return
	}

	app.OK(c, gin.H{
		"token":    token,
		"user_id":  adminUser.ID,
		"username": adminUser.Username,
		"nickname": adminUser.Nickname,
	})
}
