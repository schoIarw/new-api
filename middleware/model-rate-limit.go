package middleware

import (
	"context"
	"fmt"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/common/limiter"
	"github.com/QuantumNous/new-api/constant"
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

// Redis限流处理器
func redisRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, xUserId string, xUserGroupTotalCount, xUserGroupGroupSuccessCount int) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		ctx := context.Background()
		rdb := common.RDB

		// 1. 检查成功请求数限制
		successKey := fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, userId)
		allowed, err := checkRedisRateLimit(ctx, rdb, successKey, successMaxCount, duration)
		if err != nil {
			fmt.Println(successKey, "检查成功请求数限制失败:", err.Error())
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
			return
		}
		if !allowed {
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您所在的模型权益组已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, successMaxCount))
			return
		}

		//2.检查总请求数限制并记录总请求（当totalMaxCount为0时会自动跳过，使用令牌桶限流器
		if totalMaxCount > 0 {
			totalKey := fmt.Sprintf("rateLimit:%s", userId)
			// 初始化
			tb := limiter.New(ctx, rdb)
			allowed, err = tb.Allow(
				ctx,
				totalKey,
				limiter.WithCapacity(int64(totalMaxCount)*duration),
				limiter.WithRate(int64(totalMaxCount)),
				limiter.WithRequested(duration),
			)

			if err != nil {
				fmt.Println(totalKey, "检查总请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}

			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您所在的模型权益组已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, totalMaxCount))
			}
		}

		// plus. 检查x-user-id成功请求数限制
		if xUserId != "" && xUserGroupGroupSuccessCount > 0 {
			successKey := fmt.Sprintf("rateLimit:%s:%s", ModelRequestRateLimitSuccessCountMark, xUserId)
			allowed, err := checkRedisRateLimit(ctx, rdb, successKey, xUserGroupGroupSuccessCount, duration)
			if err != nil {
				fmt.Println(successKey, "检查成功请求数限制失败:", err.Error())
				abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
				return
			}
			if !allowed {
				abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您的个人密钥已达到请求数限制：%d分钟内最多请求%d次", setting.ModelRequestRateLimitDurationMinutes, xUserGroupGroupSuccessCount))
				return
			}

			if xUserGroupTotalCount > 0 {
				totalKey := fmt.Sprintf("rateLimit:%s", xUserId)
				// 初始化
				tb := limiter.New(ctx, rdb)
				allowed, err = tb.Allow(
					ctx,
					totalKey,
					limiter.WithCapacity(int64(xUserGroupTotalCount)*duration),
					limiter.WithRate(int64(xUserGroupTotalCount)),
					limiter.WithRequested(duration),
				)

				if err != nil {
					fmt.Println(totalKey, "检查总请求数限制失败:", err.Error())
					abortWithOpenAiMessage(c, http.StatusInternalServerError, "rate_limit_check_failed")
					return
				}

				if !allowed {
					abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("您的个人密钥已达到总请求数限制：%d分钟内最多请求%d次，包括失败次数，请检查您的请求是否正确", setting.ModelRequestRateLimitDurationMinutes, xUserGroupTotalCount))
				}
			}

		}

		// 4. 处理请求
		c.Next()

		// 5. 如果请求成功，记录成功请求
		if c.Writer.Status() < 400 {
			recordRedisRequest(ctx, rdb, successKey, successMaxCount)
		}
	}
}

// 内存限流处理器
func memoryRateLimitHandler(duration int64, totalMaxCount, successMaxCount int, xUserId string, xUserGroupTotalCount, xUserGroupGroupSuccessCount int) gin.HandlerFunc {
	inMemoryRateLimiter.Init(time.Duration(setting.ModelRequestRateLimitDurationMinutes) * time.Minute)

	return func(c *gin.Context) {
		userId := strconv.Itoa(c.GetInt("id"))
		totalKey := ModelRequestRateLimitCountMark + userId
		successKey := ModelRequestRateLimitSuccessCountMark + userId

		// 1. 检查总请求数限制（当totalMaxCount为0时跳过）
		if totalMaxCount > 0 && !inMemoryRateLimiter.Request(totalKey, totalMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// 2. 检查成功请求数限制
		// 使用一个临时key来检查限制，这样可以避免实际记录
		checkKey := successKey + "_check"
		if !inMemoryRateLimiter.Request(checkKey, successMaxCount, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}

		// plus. 检查x-user-id成功请求数限制
		if xUserId != "" && xUserGroupGroupSuccessCount > 0 {

			totalKey := ModelRequestRateLimitCountMark + xUserId
			successKey := ModelRequestRateLimitSuccessCountMark + xUserId
			// 1. 检查总请求数限制（当totalMaxCount为0时跳过）
			if xUserGroupTotalCount > 0 && !inMemoryRateLimiter.Request(totalKey, xUserGroupTotalCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}

			// 2. 检查成功请求数限制
			// 使用一个临时key来检查限制，这样可以避免实际记录
			checkKey := successKey + "_check"
			if !inMemoryRateLimiter.Request(checkKey, xUserGroupGroupSuccessCount, duration) {
				c.Status(http.StatusTooManyRequests)
				c.Abort()
				return
			}
		}

		// 3. 处理请求
		c.Next()

		// 4. 如果请求成功，记录到实际的成功请求计数中
		if c.Writer.Status() < 400 {
			inMemoryRateLimiter.Request(successKey, successMaxCount, duration)
		}
	}
}

// ModelRequestRateLimit 模型请求限流中间件
func ModelRequestRateLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 在每个请求时检查是否启用限流
		if !setting.ModelRequestRateLimitEnabled {
			c.Next()
			return
		}

		// 计算限流参数
		duration := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
		totalMaxCount := setting.ModelRequestRateLimitCount
		successMaxCount := setting.ModelRequestRateLimitSuccessCount

		// 获取分组  20260721修改：优先获取山东项目自定义用户
		xUserId := subNumber(model.GetUser(c))
		xUserGroupTotalCount, xUserGroupGroupSuccessCount, found := setting.GetGroupRateLimit(xUserId)
		if !found {
			//未发现配置，设置为-1 限流处理器会略过对xUserId的限流检验
			xUserGroupTotalCount = -1
			xUserGroupGroupSuccessCount = -1
		}

		group := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		if group == "" {
			group = common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		}

		//获取分组的限流配置 20260721备注：这个配置，在 系统设置-速率限制设置-分组速率限制 直接配置即可，它并没有检验group是不是真的存在，也可以直接修改options表 key='ModelRequestRateLimitGroup'的value值
		groupTotalCount, groupSuccessCount, found := setting.GetGroupRateLimit(group)
		if found {
			totalMaxCount = groupTotalCount
			successMaxCount = groupSuccessCount
		}
		userId := strconv.Itoa(c.GetInt("id"))

		logger.LogInfo(c,
			fmt.Sprintf("测试限流:  x-user-id=%s, id=%s, groupTotalCount=%d, groupSuccessCount=%d, xUserIdTotalCount=%d, xUserIdSuccessCount=%d",
				group,
				userId,
				groupTotalCount,
				groupSuccessCount,
				xUserGroupTotalCount,
				xUserGroupGroupSuccessCount))

		// 根据存储类型选择并执行限流处理器
		if common.RedisEnabled {
			redisRateLimitHandler(duration, totalMaxCount, successMaxCount, xUserId, xUserGroupTotalCount, xUserGroupGroupSuccessCount)(c)
		} else {
			memoryRateLimitHandler(duration, totalMaxCount, successMaxCount, xUserId, xUserGroupTotalCount, xUserGroupGroupSuccessCount)(c)
		}
	}
}

func subNumber(s string) string {
	if len(s) <= 11 {
		return s
	}
	return s[:11]
}
