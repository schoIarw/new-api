package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

// RateLimitDashboardItem 限流看板单条数据
type RateLimitDashboardItem struct {
	TokenName    string `json:"token_name"`
	Account      string `json:"account"`
	Count        int64  `json:"count"`
	SuccessLimit int    `json:"success_limit"`
	RateLimited  bool   `json:"rate_limited"`
}

// RateLimitPeriodData 单个限流周期的统计结果
type RateLimitPeriodData struct {
	PeriodIndex    int64                    `json:"period_index"`
	StartTimestamp int64                    `json:"start_timestamp"`
	EndTimestamp   int64                    `json:"end_timestamp"`
	Items          []RateLimitDashboardItem `json:"items"`
}

// buildRateLimitItems 将原始统计数据转换为看板条目，并标记是否限流
func buildRateLimitItems(stats []model.RateLimitGroupStat) []RateLimitDashboardItem {
	// 按 token_name 聚合总请求数（token 级限流跨 account 生效）
	tokenTotalCount := make(map[string]int64)
	for _, s := range stats {
		tokenTotalCount[s.TokenName] += s.Count
	}

	items := make([]RateLimitDashboardItem, 0, len(stats))
	for _, s := range stats {
		item := RateLimitDashboardItem{
			TokenName:    s.TokenName,
			Account:      s.Account,
			Count:        s.Count,
			SuccessLimit: 0,
			RateLimited:  false,
		}
		// 匹配分组速率限制配置：优先按 token_name 匹配（跨 account 聚合计数），其次按 account 匹配
		if _, successLimit, found := setting.GetGroupRateLimit(s.TokenName); found {
			item.SuccessLimit = successLimit
			if successLimit > 0 && tokenTotalCount[s.TokenName] >= int64(successLimit) {
				item.RateLimited = true
			}
		} else if _, successLimit, found := setting.GetGroupRateLimit(s.Account); found {
			item.SuccessLimit = successLimit
			if successLimit > 0 && s.Count >= int64(successLimit) {
				item.RateLimited = true
			}
		}
		items = append(items, item)
	}
	return items
}

// GetRateLimitDashboard 限流看板接口
// 实时模式：periods=N（查询最近 N 个周期，N>=1），自动刷新
// 历史模式：start_timestamp + end_timestamp（按时间段查询，按限流周期统计呈现）
func GetRateLimitDashboard(c *gin.Context) {
	durationSec := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
	now := time.Now().Unix()

	startTS, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTS, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	var periodStats []RateLimitPeriodData

	if startTS > 0 && endTS > 0 {
		// 历史模式：按时间段切分为限流周期
		periodStart := (startTS / durationSec) * durationSec
		idx := int64(0)
		for periodStart < endTS {
			periodEnd := periodStart + durationSec
			stats, err := model.GetRateLimitGroupStats(periodStart, periodEnd)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			periodStats = append(periodStats, RateLimitPeriodData{
				PeriodIndex:    idx,
				StartTimestamp: periodStart,
				EndTimestamp:   periodEnd,
				Items:          buildRateLimitItems(stats),
			})
			periodStart += durationSec
			idx++
		}
	} else {
		// 实时模式：查询最近 N 个周期
		periods, _ := strconv.Atoi(c.Query("periods"))
		if periods < 1 {
			periods = 1
		}
		for p := 0; p < periods; p++ {
			endTimestamp := now - int64(p)*durationSec
			startTimestamp := endTimestamp - durationSec
			stats, err := model.GetRateLimitGroupStats(startTimestamp, endTimestamp)
			if err != nil {
				common.ApiError(c, err)
				return
			}
			periodStats = append(periodStats, RateLimitPeriodData{
				PeriodIndex:    int64(p),
				StartTimestamp: startTimestamp,
				EndTimestamp:   endTimestamp,
				Items:          buildRateLimitItems(stats),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"duration_minutes": setting.ModelRequestRateLimitDurationMinutes,
			"periods":          periodStats,
		},
	})
}

// GetModelDashboard 模型看板接口
// 实时模式：hours=N（查询最近 N 小时，5 分钟粒度），自动刷新
// 历史模式：start_timestamp + end_timestamp（按时间段查询，5 分钟粒度）
func GetModelDashboard(c *gin.Context) {
	now := time.Now().Unix()
	var startTimestamp, endTimestamp int64

	startTS, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTS, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	if startTS > 0 && endTS > 0 {
		startTimestamp = startTS
		endTimestamp = endTS
	} else {
		hours, _ := strconv.Atoi(c.Query("hours"))
		if hours <= 0 {
			hours = 8
		}
		if hours > 96 {
			hours = 96
		}
		startTimestamp = now - int64(hours)*3600
		endTimestamp = now
	}

	ignoreKey := c.Query("ignore_key") == "true"

	stats, err := model.GetModelDashboardStats(startTimestamp, endTimestamp, ignoreKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":           stats,
			"start_timestamp": startTimestamp,
			"end_timestamp":   endTimestamp,
		},
	})
}

func GetPerformanceDashboard(c *gin.Context) {
	now := time.Now().Unix()
	var startTimestamp, endTimestamp int64

	startTS, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTS, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	if startTS > 0 && endTS > 0 {
		startTimestamp = startTS
		endTimestamp = endTS
	} else {
		hours, _ := strconv.Atoi(c.Query("hours"))
		if hours <= 0 {
			hours = 8
		}
		if hours > 96 {
			hours = 96
		}
		startTimestamp = now - int64(hours)*3600
		endTimestamp = now
	}

	ignoreKey := c.Query("ignore_key") == "true"

	stats, err := model.GetPerformanceDashboardStats(startTimestamp, endTimestamp, ignoreKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":           stats,
			"start_timestamp": startTimestamp,
			"end_timestamp":   endTimestamp,
		},
	})
}

// DeleteHistoryLogs is the legacy synchronous log cleanup endpoint (DELETE /api/log/).
// It deletes directly instead of going through the async system task. It is kept only
// for the classic frontend; the default frontend uses POST /api/system-task/log-cleanup.
// TODO: remove this handler (and its route) once the classic frontend is removed.
func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLogBatch(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}
