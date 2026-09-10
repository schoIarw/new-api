package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// registerRateLimitManagementRoutes exposes the GUI-oriented management layer.
// RootAuth is intentional because these endpoints write global rate-limit policy.
func registerRateLimitManagementRoutes(apiRouter *gin.RouterGroup) {
	route := apiRouter.Group("/rate-limit-management")
	route.Use(middleware.RootAuth())
	{
		route.GET("/", controller.GetRateLimitManagement)
		route.PUT("/defaults", controller.UpdateRateLimitManagementDefaults)
		route.PUT("/category-policies", controller.UpdateRateLimitCategoryPolicies)
		route.PUT("/special-policies", controller.UpdateRateLimitSpecialPolicies)
		route.PUT("/model-category", controller.UpsertRateLimitModelCategory)
		route.DELETE("/model-category", controller.DeleteRateLimitModelCategory)
		route.POST("/rebuild", controller.RebuildRateLimitManagement)
	}
}
