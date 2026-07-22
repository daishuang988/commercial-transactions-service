package front

import (
	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	"github.com/gin-gonic/gin"
)

// MyOrders 我的订单 GET /api/v1/front/orders
func MyOrders(c *gin.Context) {
	uid := c.GetInt64("user_id")
	page := queryInt(c, "page", 1)
	limit := queryInt(c, "limit", 10)

	orders, count, err := repository.ListOrders(model.OrderListReq{
		Page:  page,
		Limit: limit,
		BuyerID: &uid,
	})
	if err != nil {
		app.InternalError(c, "查询失败")
		return
	}
	app.OKWithCount(c, orders, count)
}

// MyOrderDetail 订单详情 GET /api/v1/front/orders/:id
func MyOrderDetail(c *gin.Context) {
	id := parseIntParam(c, "id")
	o, err := repository.GetOrderByID(id)
	if err != nil || o.BuyerID != c.GetInt64("user_id") {
		app.NotFound(c, "订单不存在")
		return
	}
	app.OK(c, o)
}
