package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// SetModelRuntimeDashboardRouter 注册模型运行看板专用接口。
// 与现有 /api/log/model_dashboard（基于 logs 的旧统计接口）分离，避免改变原日志查询语义。
func SetModelRuntimeDashboardRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	{
		apiRouter.GET("/log/model_runtime_dashboard", middleware.AdminAuth(), controller.GetVLLMModelRuntimeDashboard)
	}
}
