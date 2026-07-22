package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构（对标老系统 {"code":0,"msg":"ok","data":...}）
type Response struct {
	Code  int         `json:"code"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data,omitempty"`
	Count int64       `json:"count,omitempty"` // 分页总数
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Msg: "ok", Data: data})
}

func OKWithCount(c *gin.Context, data interface{}, count int64) {
	c.JSON(http.StatusOK, Response{Code: 0, Msg: "ok", Data: data, Count: count})
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{Code: code, Msg: msg})
}

func FailWithData(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: code, Msg: msg, Data: data})
}

// 常用快捷方法
func BadRequest(c *gin.Context, msg string) {
	Fail(c, 400, msg)
}

func Unauthorized(c *gin.Context, msg string) {
	Fail(c, 401, msg)
}

func Forbidden(c *gin.Context, msg string) {
	Fail(c, 403, msg)
}

func NotFound(c *gin.Context, msg string) {
	Fail(c, 404, msg)
}

func TooManyRequests(c *gin.Context, msg string) {
	Fail(c, 429, msg)
}

func InternalError(c *gin.Context, msg string) {
	Fail(c, 500, msg)
}
