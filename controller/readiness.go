package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Readiness deliberately returns no dependency details to unauthenticated
// callers. Unlike /api/status, it verifies the primary database and Redis
// before reporting that the management API has recovered.
func Readiness(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if model.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	sqlDB, err := model.DB.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	if common.RedisEnabled {
		if common.RDB == nil || common.RDB.Ping(ctx).Err() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
