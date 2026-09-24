package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/log_history_setting"
	"github.com/gin-gonic/gin"
)

func GetLogHistoryMigrationStatus(c *gin.Context) {
	setting := log_history_setting.GetSetting()
	stats, err := model.GetLogHistoryStats(c.Request.Context(), setting.RetentionDays)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	latestTask, err := model.GetLatestSystemTask(model.SystemTaskTypeLogHistoryMigration)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var task any
	if latestTask != nil {
		task = latestTask.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"config":      setting,
			"stats":       stats,
			"latest_task": task,
			"next_cutoff": time.Now().Add(-time.Duration(setting.RetentionDays) * 24 * time.Hour).Unix(),
		},
	})
}

func RunLogHistoryMigration(c *gin.Context) {
	setting := log_history_setting.GetSetting()
	if supported, reason := model.LogHistoryMigrationSupported(); !supported {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": reason})
		return
	}
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeLogHistoryMigration, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": func() string {
			if created {
				return "日志迁移任务已提交"
			}
			return "已有日志迁移任务正在执行"
		}(),
		"data": gin.H{
			"task":           task.ToResponse(),
			"retention_days": setting.RetentionDays,
		},
	})
}
