package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"commercial-transactions-service/internal/config"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT载荷
type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

var jwtSecret []byte

func InitJWT(cfg *config.JWTConfig) {
	jwtSecret = []byte(cfg.Secret)
}

// GenerateToken 生成JWT Token
func GenerateToken(userID int64, username string, isAdmin bool, expireHours int) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
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
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
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
