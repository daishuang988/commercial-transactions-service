package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"commercial-transactions-service/internal/config"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT载荷
type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	Token    string `json:"token"`
	jwt.RegisteredClaims
}

var jwtSecret []byte

func InitJWT(cfg *config.JWTConfig) {
	jwtSecret = []byte(cfg.Secret)
}

// GenerateToken 生成JWT Token
func GenerateToken(userID int64, username string, isAdmin bool, expireHours int, tokenVer string) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
		Token:    tokenVer,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken 解析JWT
func ParseToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{},
		func(t *jwt.Token) (interface{}, error) { return jwtSecret, nil })
	if err != nil || !token.Valid {
		return nil, err
	}
	return token.Claims.(*JWTClaims), nil
}

// AuthRequired C端认证中间件
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			app.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}
		claims, err := ParseToken(token)
		if err != nil {
			app.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}
		if claims.IsAdmin {
			app.Forbidden(c, "请使用用户账号登录")
			c.Abort()
			return
		}
		// C端校验token版本（密码修改后旧token失效）
		if claims.Token != "" {
			var storedToken string
			repository.DB.Table("users").Select("token").Where("id = ?", claims.UserID).Scan(&storedToken)
			if storedToken != claims.Token {
				app.Unauthorized(c, "密码已修改，请重新登录")
				c.Abort()
				return
			}
		}
		// 检查账号是否冻结
		var userStatus int8
		repository.DB.Table("users").Select("status").Where("id = ?", claims.UserID).Scan(&userStatus)
		if userStatus == 0 {
			app.Fail(c, app.ErrCodeUserFrozen, "账号已冻结，请联系管理员")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// AdminAuthRequired 管理端认证中间件
func AdminAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			app.Unauthorized(c, "请先登录")
			c.Abort()
			return
		}
		claims, err := ParseToken(token)
		if err != nil {
			app.Unauthorized(c, "登录已过期，请重新登录")
			c.Abort()
			return
		}
		if !claims.IsAdmin {
			app.Forbidden(c, "无管理员权限")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return c.Query("token")
}

// RateLimiter 简单令牌桶限流（应用层）
func RateLimiter(maxPerSec int) gin.HandlerFunc {
	tokens := make(chan struct{}, maxPerSec)
	// 填充令牌
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(maxPerSec))
		defer ticker.Stop()
		for range ticker.C {
			select {
			case tokens <- struct{}{}:
			default:
			}
		}
	}()

	return func(c *gin.Context) {
		select {
		case <-tokens:
			c.Next()
		default:
			app.TooManyRequests(c, "请求过于频繁，请稍后再试")
			c.Abort()
		}
	}
}

// Recovery 异常恢复
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, app.Response{
					Code: 500,
					Msg:  "服务器内部错误",
				})
			}
		}()
		c.Next()
	}
}

// RequestLogger 请求日志（记录每个请求的方法/路径/状态/耗时/IP）
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		uid, _ := c.Get("user_id")
		log.Printf("[%s] %s %s %d %v ip=%s uid=%v",
			start.Format("2006-01-02 15:04:05"),
			c.Request.Method, c.Request.URL.Path,
			c.Writer.Status(), latency,
			c.ClientIP(), uid)
	}
}

// CORS 跨域
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With,X-Contract-Signed")
		c.Header("Access-Control-Expose-Headers", "X-Contract-Signed")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
		// 后置：确保所有响应带 charset
		ct := c.Writer.Header().Get("Content-Type")
		if ct == "application/json" || ct == "application/json; charset=utf-8" {
			c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
	}
}

// ContractRequired 合同签署检查中间件（C端需认证路由用）
func ContractRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		uid := c.GetInt64("user_id")
		var signature string
		repository.DB.Table("user_contracts").Select("contract_path").Where("user_id=? AND contract_path IS NOT NULL AND contract_path != ''", uid).Order("id DESC").Scan(&signature)
		signed := signature != ""

		// 已签：直接放行
		if signed {
			c.Header("X-Contract-Signed", "true")
			c.Next()
			return
		}

		// 未签白名单：个人信息 + 合同 + 上传 + 配置
		isAllow := strings.Contains(path, "/contract") ||
			strings.Contains(path, "/user/profile") ||
			strings.Contains(path, "/upload") ||
			strings.Contains(path, "/config/") ||
			strings.Contains(path, "/flash-sale/time")
		if isAllow {
			c.Header("X-Contract-Signed", "false")
			c.Next()
			return
		}

		c.Header("X-Contract-Signed", "false")
		c.JSON(http.StatusOK, app.Response{Code: app.ErrCodeContractUnsigned, Msg: "请签署平台用户合同"})
		c.Abort()
	}
}
