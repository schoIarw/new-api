package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/limiter"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	ModelRequestRateLimitCountMark        = "MRRL"
	ModelRequestRateLimitSuccessCountMark = "MRRLS"
)

// 检查Redis中的请求限制
func checkRedisRateLimit(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) (bool, error) {
	// 如果maxCount为0，表示不限制
	if maxCount == 0 {
		return true, nil
	}

	// 获取当前计数
	length, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		return false, err
	}

	// 如果未达到限制，允许请求
	if length < int64(maxCount) {
		return true, nil
	}

	// 检查时间窗口
	oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
	oldTime, err := time.Parse(timeFormat, oldTimeStr)
	if err != nil {
		return false, err
	}

	nowTimeStr := time.Now().Format(timeFormat)
	nowTime, err := time.Parse(timeFormat, nowTimeStr)
	if err != nil {
		return false, err
	}
	// 如果在时间窗口内已达到限制，拒绝请求
	subTime := nowTime.Sub(oldTime).Seconds()
	if int64(subTime) < duration {
		rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
		return false, nil
	}

	return true, nil
}

// 记录Redis请求
func recordRedisRequest(ctx context.Context, rdb *redis.Client, key string, maxCount int) {
	// 如果maxCount为0，不记录请求
	if maxCount == 0 {
		return
	}

	now := time.Now().Format(timeFormat)
	rdb.LPush(ctx, key, now)
	rdb.LTrim(ctx, key, 0, int64(maxCount-1))
	rdb.Expire(ctx, key, time.Duration(setting.ModelRequestRateLimitDurationMinutes)*time.Minute)
}

func checkRedisTotalLimit(ctx context.Context, rdb *redis.Client, key string, maxCount int, duration int64) (bool, error) {
	if maxCount <= 0 {
		return true, nil
	}
	tb := limiter.New(ctx, rdb)
	return tb.Allow(
		ctx,
		key,
		limiter.WithCapacity(int64(maxCount)*duration),
		limiter.WithRate(int64(maxCount)),
		limiter.WithRequested(duration),
	)
}

// Redis限流处理器。
// totalMaxCount/successMaxCount 是当前用户在权益组内 all（所有模型）的限制；
// modelTotalMaxCount/modelSuccessMaxCount 是当前模型的额外限制，两类规则同时生效。
// x-user-id（手机号/个人标识）的限制逻辑保持独立。
func redisRateLimitHandler(
	duration int64,
	totalMaxCount, successMaxCount int,
	modelName string,
	modelTotalMaxCount, modelSuccessMaxCount int,
	xUserId string,
	xUserGroupTotalCount, xUserGroupGroupSuccessCount int,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		ctx := context.Background()
		rdb := common.RDB

		// 1. 检查组内 all 成功请求数限制
		successKey := fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, userId)
		allowed, err := checkRedisRateLimit(ctx, rdb, successKey, successMaxCount, duration)
		if err != nil {
			fmt.Println(successKey, "检查成功请求数限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您所在的模型权益组已达到完成请求数限制：%d分钟内最多完成%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount))
			return
		}

		// 2. 检查组内 all 总访问次数限制
		if totalMaxCount > 0 {
			totalKey := fmt.Sprintf("rateLimit:%s", userId)
			allowed, err = checkRedisTotalLimit(ctx, rdb, totalKey, totalMaxCount, duration)
			if err != nil {
				fmt.Println(totalKey, "检查总请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}
			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您所在的模型权益组已达到总访问次数限制：%d分钟内最多访问%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount))
				return
			}
		}

		// 3. 检查组内当前模型的独立限制。模型规则使用独立 key，不与 all 计数混淆。
		modelSuccessKey := ""
		if modelName != "" && (modelTotalMaxCount > 0 || modelSuccessMaxCount > 0) {
			modelScopeKey := fmt.Sprintf("%s:model:%s", userId, modelName)
			modelSuccessKey = fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, modelScopeKey)
			allowed, err = checkRedisRateLimit(ctx, rdb, modelSuccessKey, modelSuccessMaxCount, duration)
			if err != nil {
				fmt.Println(modelSuccessKey, "检查模型完成请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}
			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("模型 %s 已达到完成请求数限制：%d分钟内最多完成%d次", modelName, setting.ModelRequestRateLimitDurationMinutes, modelSuccessMaxCount))
				return
			}

			if modelTotalMaxCount > 0 {
				modelTotalKey := fmt.Sprintf("rateLimit:%s", modelScopeKey)
				allowed, err = checkRedisTotalLimit(ctx, rdb, modelTotalKey, modelTotalMaxCount, duration)
				if err != nil {
					fmt.Println(modelTotalKey, "检查模型总访问次数限制失败:", err.Error())
					abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
					return
				}
				if !allowed {
					abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("模型 %s 已达到总访问次数限制：%d分钟内最多访问%d次，包括失败次数", modelName, setting.ModelRequestRateLimitDurationMinutes, modelTotalMaxCount))
					return
				}
			}
		}

		// 4. 检查 x-user-id 成功请求数限制（手机号/个人标识逻辑保持原有配置方式）
		if xUserId != "" && xUserGroupGroupSuccessCount > 0 {
			xUserSuccessKey := fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, xUserId)
			allowed, err := checkRedisRateLimit(ctx, rdb, xUserSuccessKey, xUserGroupGroupSuccessCount, duration)
			if err != nil {
				fmt.Println(xUserSuccessKey, "检查成功请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}
			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您的个人密钥已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, xUserGroupGroupSuccessCount))
				return
			}

			if xUserGroupTotalCount > 0 {
				totalKey := fmt.Sprintf("rateLimit:%s", xUserId)
				allowed, err = checkRedisTotalLimit(ctx, rdb, totalKey, xUserGroupTotalCount, duration)
				if err != nil {
					fmt.Println(totalKey, "检查总请求数限制失败:", err.Error())
					abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
					return
				}
				if !allowed {
					abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您的个人密钥已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, xUserGroupTotalCount))
					return
				}
			}
		}

		// 5. 处理请求
		c.Next()

		// 6. 请求成功后分别记录 all 和当前模型的完成请求数
		if c.Writer.Status() < 400 {
			recordRedisRequest(ctx, rdb, successKey, successMaxCount)
			if modelSuccessKey != "" {
				recordRedisRequest(ctx, rdb, modelSuccessKey, modelSuccessMaxCount)
			}
		}
	}
}

// 内存限流处理器
func memoryRateLimitHandler(
	duration int64,
	totalMaxCount, successMaxCount int,
	modelName string,
	modelTotalMaxCount, modelSuccessMaxCount int,
	xUserId string,
	xUserGroupTotalCount, xUserGroupGroupSuccessCount int,
) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)

	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		totalKey := ModelRequestRateLimitCountMark + userId
		successKey := ModelRequestRateLimitSuccessCountMark + userId

		// 1. 检查组内 all 总访问次数限制
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(totalKey, totalMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// 2. 检查组内 all 完成请求数限制；0 表示不限制
		if successMaxCount > 0 {
			checkKey := successKey + "_check"
			if !inMemoryRateLimiter.Request(checkKey, successMaxCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}
		}

		// 3. 检查具体模型限制
		modelSuccessKey := ""
		if modelName != "" && (modelTotalMaxCount > 0 || modelSuccessMaxCount > 0) {
			modelScopeKey := userId + ":model:" + modelName
			if modelTotalMaxCount > 0 && !inMemoryRateLimiter.Request(ModelRequestRateLimitCountMark+modelScopeKey, modelTotalMaxCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}
			modelSuccessKey = ModelRequestRateLimitSuccessCountMark + modelScopeKey
			if modelSuccessMaxCount > 0 {
				checkKey := modelSuccessKey + "_check"
				if !inMemoryRateLimiter.Request(checkKey, modelSuccessMaxCount, duration) {
					c.Status(http.StatusTooManyRequests)
					c.Abort()
					return
				}
			}
		}

		// 4. 检查 x-user-id 限制
		if xUserId != "" && xUserGroupGroupSuccessCount > 0 {
			xUserTotalKey := ModelRequestRateLimitCountMark + xUserId
			xUserSuccessKey := ModelRequestRateLimitSuccessCountMark + xUserId
			if xUserGroupTotalCount > 0 && !inMemoryRateLimiter.Request(xUserTotalKey, xUserGroupTotalCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}
			checkKey := xUserSuccessKey + "_check"
			if !inMemoryRateLimiter.Request(checkKey, xUserGroupGroupSuccessCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}
		}

		// 5. 处理请求
		c.Next()

		// 6. 成功后记录完成请求
		if c.Writer.Status() < 400 {
			if successMaxCount > 0 {
				inMemoryRateLimiter.Request(successKey, successMaxCount, duration)
			}
			if modelSuccessKey != "" && modelSuccessMaxCount > 0 {
				inMemoryRateLimiter.Request(modelSuccessKey, modelSuccessMaxCount, duration)
			}
		}
	}
}

type rateLimitRequestMeta struct {
	User  string `json:"user"`
	Model string `json:"model"`
}

// getRateLimitRequestMeta reads the request body once and restores it, preserving the existing
// x-user-id extraction semantics while also making the requested model available before Distribute runs.
func getRateLimitRequestMeta(c *gin.Context) (userId, modelName string) {
	modelName = common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	// Gemini-style paths carry the model in /models/{model}:action.
	if modelName == "" {
		if idx := strings.Index(c.Request.URL.Path, "/models/"); idx >= 0 {
			part := c.Request.URL.Path[idx+len("/models/"):]
			if colon := strings.Index(part, ":"); colon >= 0 {
				part = part[:colon]
			}
			modelName = strings.TrimSpace(part)
		}
	}

	if c.Request.Body != nil {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil {
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			if len(bodyBytes) > 0 {
				var meta rateLimitRequestMeta
				if json.Unmarshal(bodyBytes, &meta) == nil {
					if strings.TrimSpace(meta.User) != "" {
						userId = strings.TrimSpace(meta.User)
					}
					if modelName == "" && strings.TrimSpace(meta.Model) != "" {
						modelName = strings.TrimSpace(meta.Model)
					}
				}
			}
		}
	}

	if userId == "" {
		userId = c.GetHeader("X-User-Id")
	}
	return userId, modelName
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 在每个请求时检查是否启用限流
		if !setting.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		// 计算全局默认限流参数
		duration := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
		totalMaxCount := setting.ModelRequestRateLimitCount
		successMaxCount := setting.ModelRequestRateLimitSuccessCount

		requestUser, requestModel := getRateLimitRequestMeta(c)

		// 手机号/个人标识仍使用顶层数组配置，逻辑保持独立。
		xUserId := subNumber(requestUser)
		xUserGroupTotalCount, xUserGroupGroupSuccessCount, found := setting.GetGroupRateLimit(xUserId)
		if !found {
			xUserGroupTotalCount = -1
			xUserGroupGroupSuccessCount = -1
		}

		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		}

		groupTotalCount, groupSuccessCount := 0, 0
		modelTotalCount, modelSuccessCount := 0, 0

		if setting.HasGroupModelRateLimit(group) {
			// 新对象格式是该组的完整策略。缺失 all 或 all=[0,0] 均表示组内 all 不限制。
			totalMaxCount = 0
			successMaxCount = 0
			if allTotal, allSuccess, allFound := setting.GetGroupModelRateLimit(group, "all"); allFound {
				groupTotalCount = allTotal
				groupSuccessCount = allSuccess
				totalMaxCount = allTotal
				successMaxCount = allSuccess
			}
			if requestModel != "" {
				if mt, ms, modelFound := setting.GetGroupModelRateLimit(group, requestModel); modelFound {
					modelTotalCount = mt
					modelSuccessCount = ms
				}
			}
		} else {
			// 兼容旧的 group:[total,success] 配置。
			if legacyGroupTotal, legacyGroupSuccess, groupFound := setting.GetGroupRateLimit(group); groupFound {
				groupTotalCount = legacyGroupTotal
				groupSuccessCount = legacyGroupSuccess
				totalMaxCount = legacyGroupTotal
				successMaxCount = legacyGroupSuccess
			}

			// 保留旧 token_name 顶层数组覆盖能力，仅在组没有采用新对象格式时生效。
			tokenName := c.GetString("token_name")
			if tokenName != "" {
				if tokenTotalCount, tokenSuccessCount, tokenFound := setting.GetGroupRateLimit(tokenName); tokenFound {
					totalMaxCount = tokenTotalCount
					successMaxCount = tokenSuccessCount
				}
			}
		}

		userId := strconv.Itoa(c.GetInt("id"))
		logger.LogInfo(c,
			fmt.Sprintf("限流: group=%s, model=%s, id=%s, groupAll=[%d,%d], groupModel=[%d,%d], xUserId=%s, xUserLimit=[%d,%d]",
				group,
				requestModel,
				userId,
				groupTotalCount,
				groupSuccessCount,
				modelTotalCount,
				modelSuccessCount,
				xUserId,
				xUserGroupTotalCount,
				xUserGroupGroupSuccessCount))

		// 根据存储类型选择并执行限流处理器
		if common.RedisEnabled {
			redisRateLimitHandler(
				duration,
				totalMaxCount, successMaxCount,
				requestModel, modelTotalCount, modelSuccessCount,
				xUserId, xUserGroupTotalCount, xUserGroupGroupSuccessCount,
			)(c)
		} else {
			memoryRateLimitHandler(
				duration,
				totalMaxCount, successMaxCount,
				requestModel, modelTotalCount, modelSuccessCount,
				xUserId, xUserGroupTotalCount, xUserGroupGroupSuccessCount,
			)(c)
		}
	}
}

func subNumber(s string) string {
	if len(s) <= 11 {
		return s
	}
	return s[:11]
}
