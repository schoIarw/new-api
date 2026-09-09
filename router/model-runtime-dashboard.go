package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// SetModelRuntimeDashboardRouter 注册模型看板专用接口。
// 该接口仅访问 Prometheus，不属于日志查询链路，也不会读取 logs 表。
func SetModelRuntimeDashboardRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/model-dashboard/metrics", middleware.AdminAuth(), controller.GetVLLMModelRuntimeDashboard)
	}
}
