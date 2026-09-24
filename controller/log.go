package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
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
	LimitType    string `json:"limit_type"`
	LimitKey     string `json:"limit_key"`
	Label        string `json:"label"`
	Group        string `json:"group"`
	ModelName    string `json:"model_name"`
	UserID       int    `json:"user_id"`
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
	itemsByKey := make(map[string]*RateLimitDashboardItem)
	add := func(item RateLimitDashboardItem, count int64) {
		if existing, ok := itemsByKey[item.LimitKey]; ok {
			existing.Count += count
			return
		}
		item.Count = count
		itemsByKey[item.LimitKey] = &item
	}

	for _, s := range stats {
		groupLabel := s.Group
		if strings.TrimSpace(groupLabel) == "" {
			groupLabel = "(未分组)"
		}
		matched := false

		groupSuccessLimit := setting.ModelRequestRateLimitSuccessCount
		if setting.HasGroupModelRateLimit(s.Group) {
			groupSuccessLimit = 0
			if _, success, found := setting.GetGroupModelRateLimit(s.Group, "all"); found {
				groupSuccessLimit = success
			}
		} else if _, success, found := setting.GetGroupRateLimit(s.Group); found {
			groupSuccessLimit = success
		}
		if groupSuccessLimit > 0 {
			matched = true
			add(RateLimitDashboardItem{
				LimitType:    "group",
				LimitKey:     "group|" + s.Group + "|" + strconv.Itoa(s.UserID),
				Label:        "令牌组 " + groupLabel + " / 用户#" + strconv.Itoa(s.UserID),
				Group:        s.Group,
				UserID:       s.UserID,
				TokenName:    s.TokenName,
				SuccessLimit: groupSuccessLimit,
			}, s.Count)
		}

		if s.ModelName != "" && setting.HasGroupModelRateLimit(s.Group) {
			if _, success, found := setting.GetGroupModelRateLimit(s.Group, s.ModelName); found && success > 0 {
				matched = true
				add(RateLimitDashboardItem{
					LimitType:    "group_model",
					LimitKey:     "group_model|" + s.Group + "|" + strconv.Itoa(s.UserID) + "|" + s.ModelName,
					Label:        "组+模型 " + groupLabel + " / " + s.ModelName + " / 用户#" + strconv.Itoa(s.UserID),
					Group:        s.Group,
					ModelName:    s.ModelName,
					UserID:       s.UserID,
					TokenName:    s.TokenName,
					SuccessLimit: success,
				}, s.Count)
			}
		}

		if strings.TrimSpace(s.Account) != "" {
			limits, configured, active, counterIdentity := setting.ResolveUserIdentifierRateLimit(s.Group, s.Account)
			if configured && active && limits[1] > 0 {
				matched = true
				identifierLabel := s.Account
				if strings.HasPrefix(counterIdentity, "prefix:") {
					identifierLabel = strings.TrimPrefix(counterIdentity, "prefix:") + "*"
				}
				add(RateLimitDashboardItem{
					LimitType:    "group_phone",
					LimitKey:     "group_phone|" + s.Group + "|" + counterIdentity,
					Label:        "组+手机号 " + groupLabel + " / " + identifierLabel,
					Group:        s.Group,
					Account:      identifierLabel,
					TokenName:    s.TokenName,
					SuccessLimit: limits[1],
				}, s.Count)
			}
		}

		if !matched {
			add(RateLimitDashboardItem{
				LimitType: "unconfigured",
				LimitKey:  "unconfigured|" + s.Group + "|" + strconv.Itoa(s.UserID) + "|" + s.TokenName + "|" + s.Account,
				Label:     "未配置阈值 " + groupLabel + " / " + s.TokenName + " / " + s.Account,
				Group:     s.Group,
				UserID:    s.UserID,
				TokenName: s.TokenName,
				Account:   s.Account,
			}, s.Count)
		}
	}

	items := make([]RateLimitDashboardItem, 0, len(itemsByKey))
	for _, item := range itemsByKey {
		item.RateLimited = item.SuccessLimit > 0 && item.Count >= int64(item.SuccessLimit)
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].LimitType != items[j].LimitType {
			return items[i].LimitType < items[j].LimitType
		}
		return items[i].Label < items[j].Label
	})
	return items
}

// GetRateLimitDashboard 限流看板接口。
// 实时模式：periods=N，查询最近 N 个限流周期。
// 历史模式：start_timestamp + periods=N，从指定时间开始连续查询 N 个限流周期。
func GetRateLimitDashboard(c *gin.Context) {
	GetRateLimitDashboardV2(c)
}

func GetPerformanceDashboard(c *gin.Context) {
	now := time.Now().Unix()
	var startTimestamp, endTimestamp int64

	startTS, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTS, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	if startTS > 0 && endTS > 0 {
		if endTS-startTS > 48*3600 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "历史查询时间范围不能超过48小时",
			})
			return
		}
		startTimestamp = startTS
		endTimestamp = endTS
	} else {
		minutes, _ := strconv.Atoi(c.Query("minutes"))
		if minutes <= 0 {
			hours, _ := strconv.Atoi(c.Query("hours"))
			minutes = hours * 60
		}
		if minutes <= 0 {
			minutes = 30
		}
		if minutes > 4*60 {
			minutes = 4 * 60
		}
		startTimestamp = now - int64(minutes)*60
		endTimestamp = now
	}

	ignoreKey := c.Query("ignore_key") == "true"
	bucketSeconds := int64(300)
	if endTimestamp-startTimestamp <= 30*60 {
		bucketSeconds = 60
	}

	stats, err := model.GetPerformanceDashboardStats(startTimestamp, endTimestamp, bucketSeconds, ignoreKey)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":           stats,
			"bucket_seconds":  bucketSeconds,
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
