package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// Admission reserves a completion slot until the request finishes; failed
// requests free that slot but still consume a request slot. Shared Redis
// reservations are atomic across gateway replicas.
var phoneAcquireScript = redis.NewScript(`
local maxTotal = tonumber(ARGV[1]); local maxSuccess = tonumber(ARGV[2]); local ttl = tonumber(ARGV[3]);
if maxTotal > 0 and tonumber(redis.call('GET', KEYS[1]) or '0') >= maxTotal then return 1 end
if maxSuccess > 0 and tonumber(redis.call('GET', KEYS[2]) or '0') >= maxSuccess then return 2 end
if maxTotal > 0 then
    if redis.call('INCR', KEYS[1]) == 1 then redis.call('EXPIRE', KEYS[1], ttl) end
end
if maxSuccess > 0 then
    if redis.call('INCR', KEYS[2]) == 1 then redis.call('EXPIRE', KEYS[2], ttl) end
end
return 0
`)
var phoneReleaseScript = redis.NewScript(`
if tonumber(redis.call('GET', KEYS[1]) or '0') > 0 then redis.call('DECR', KEYS[1]) end
return 1
`)

type phoneCounter struct {
	window   int64
	total    int
	reserved int
}

var phoneMemory = struct {
	sync.Mutex
	counters   map[string]phoneCounter
	operations uint64
}{counters: make(map[string]phoneCounter)}

func acquirePhoneMemory(scope string, window int64, pair [2]int) int {
	phoneMemory.Lock()
	defer phoneMemory.Unlock()
	counter := phoneMemory.counters[scope]
	if counter.window != window {
		counter = phoneCounter{window: window}
	}
	if pair[0] > 0 && counter.total >= pair[0] {
		return 1
	}
	if pair[1] > 0 && counter.reserved >= pair[1] {
		return 2
	}
	if pair[0] > 0 {
		counter.total++
	}
	if pair[1] > 0 {
		counter.reserved++
	}
	phoneMemory.counters[scope] = counter
	phoneMemory.operations++
	if len(phoneMemory.counters) > 10000 && phoneMemory.operations%1024 == 0 {
		for key, value := range phoneMemory.counters {
			if value.window < window-2 {
				delete(phoneMemory.counters, key)
			}
		}
	}
	return 0
}

func releasePhoneMemory(scope string, window int64) {
	phoneMemory.Lock()
	defer phoneMemory.Unlock()
	counter, ok := phoneMemory.counters[scope]
	if ok && counter.window == window && counter.reserved > 0 {
		counter.reserved--
		phoneMemory.counters[scope] = counter
	}
}

// identifierCounterScope hashes both the token group and the selected scope.
// "prefix:" and "identifier:" prevent a full ID from sharing a prefix bucket.
func identifierCounterScope(group, counterIdentity string) string {
	return common.GenerateHMAC("user-identifier-rate-limit-v3\x00" + group + "\x00" + counterIdentity)
}

func withPhoneRateLimit(c *gin.Context, group, counterIdentity string, limits [2]int, duration int64, next gin.HandlerFunc) {
	if limits == [2]int{} || counterIdentity == "" {
		next(c)
		return
	}
	if duration <= 0 {
		abortWithOpenAiMessage(c, http.StatusInternalServerError, "invalid_phone_limit_duration")
		return
	}
	// No clear-text user identifiers in Redis or in-memory map keys.
	scope := identifierCounterScope(group, counterIdentity)
	window := time.Now().Unix() / duration
	var verdict int
	var release func()
	if common.RedisEnabled {
		totalKey := fmt.Sprintf("rateLimit:identifier:v3:%s:%d:request", scope, window)
		successKey := fmt.Sprintf("rateLimit:identifier:v3:%s:%d:success", scope, window)
		ctx := c.Request.Context()
		value, err := phoneAcquireScript.Run(ctx, common.RDB, []string{totalKey, successKey}, limits[0], limits[1], duration*2).Int()
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "phone_rate_limit_check_failed")
			return
		}
		verdict = value
		release = func() {
			if limits[1] > 0 {
				// Use an independent context: request cancellation must not strand reservations.
				cleanCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := phoneReleaseScript.Run(cleanCtx, common.RDB, []string{successKey}).Err(); err != nil {
					common.SysLog("failed to release phone completion reservation: " + err.Error())
				}
			}
		}
	} else {
		verdict = acquirePhoneMemory(scope, window, limits)
		release = func() {
			if limits[1] > 0 {
				releasePhoneMemory(scope, window)
			}
		}
	}
	if verdict != 0 {
		abortWithOpenAiMessage(c, http.StatusTooManyRequests, "用户标识已达到当前令牌分组的限流上限")
		return
	}
	// Defer also runs after a panic, but a 2xx streaming header does not prove
	// the upstream response was semantically successful; match existing status semantics.
	defer func() {
		if c.Writer.Status() >= http.StatusBadRequest {
			release()
		}
	}()
	next(c)
}
