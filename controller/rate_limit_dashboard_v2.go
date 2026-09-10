package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

const (
	defaultRateLimitDashboardPeriods = 10
	maxRateLimitDashboardPeriods     = 80
)

func parseRateLimitDashboardPeriods(raw string) (int, error) {
	if raw == "" {
		return defaultRateLimitDashboardPeriods, nil
	}
	periods, err := strconv.Atoi(raw)
	if err != nil || periods < 1 || periods > maxRateLimitDashboardPeriods {
		return 0, errors.New("周期数必须在1到80之间")
	}
	return periods, nil
}

// rateLimitDashboardWindow resolves the query window for both modes.
// Realtime: [now-periods*duration, now)
// Historical: [start, start+periods*duration)
func rateLimitDashboardWindow(now, startTimestamp, durationSec int64, periods int) (queryStart, queryEnd int64, historical bool, err error) {
	if durationSec <= 0 {
		return 0, 0, false, errors.New("限流周期配置必须大于0")
	}
	if periods < 1 || periods > maxRateLimitDashboardPeriods {
		return 0, 0, false, errors.New("周期数必须在1到80之间")
	}

	span := int64(periods) * durationSec
	if startTimestamp > 0 {
		if startTimestamp > math.MaxInt64-span {
			return 0, 0, false, errors.New("历史查询开始时间无效")
		}
		return startTimestamp, startTimestamp + span, true, nil
	}
	return now - span, now, false, nil
}

func buildRateLimitPeriodData(
	queryStart int64,
	durationSec int64,
	periods int,
	historical bool,
	stats []model.RateLimitPeriodGroupStat,
) []RateLimitPeriodData {
	statsByPeriod := make(map[int64][]model.RateLimitGroupStat, periods)
	for _, s := range stats {
		if s.PeriodIndex < 0 || s.PeriodIndex >= int64(periods) {
			continue
		}
		statsByPeriod[s.PeriodIndex] = append(statsByPeriod[s.PeriodIndex], model.RateLimitGroupStat{
			TokenName: s.TokenName,
			Account:   s.Account,
			Count:     s.Count,
		})
	}

	periodData := make([]RateLimitPeriodData, 0, periods)
	if historical {
		// Historical mode is naturally ordered from the selected start time forward.
		for i := 0; i < periods; i++ {
			idx := int64(i)
			start := queryStart + idx*durationSec
			periodData = append(periodData, RateLimitPeriodData{
				PeriodIndex:    idx,
				StartTimestamp: start,
				EndTimestamp:   start + durationSec,
				Items:          buildRateLimitItems(statsByPeriod[idx]),
			})
		}
		return periodData
	}

	// Realtime keeps the existing semantic: period_index=0 is the latest period,
	// period_index=1 is the previous period, etc. The SQL query itself is oldest-first,
	// so translate the forward bucket index here.
	for p := 0; p < periods; p++ {
		forwardIdx := int64(periods - 1 - p)
		start := queryStart + forwardIdx*durationSec
		periodData = append(periodData, RateLimitPeriodData{
			PeriodIndex:    int64(p),
			StartTimestamp: start,
			EndTimestamp:   start + durationSec,
			Items:          buildRateLimitItems(statsByPeriod[forwardIdx]),
		})
	}
	return periodData
}

// GetRateLimitDashboardV2 implements the period-count based dashboard API.
// Realtime: periods=N, where N is the number of most recent rate-limit periods.
// Historical: start_timestamp + periods=N. The end time is derived server-side.
func GetRateLimitDashboardV2(c *gin.Context) {
	periods, err := parseRateLimitDashboardPeriods(c.Query("periods"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	durationSec := int64(setting.ModelRequestRateLimitDurationMinutes * 60)
	startTS, err := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	if c.Query("start_timestamp") != "" && err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "历史查询开始时间无效"})
		return
	}

	queryStart, queryEnd, historical, err := rateLimitDashboardWindow(
		time.Now().Unix(),
		startTS,
		durationSec,
		periods,
	)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	stats, err := model.GetRateLimitPeriodGroupStats(queryStart, queryEnd, durationSec)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	periodData := buildRateLimitPeriodData(queryStart, durationSec, periods, historical, stats)

	mode := "realtime"
	if historical {
		mode = "historical"
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"mode":             mode,
			"duration_minutes": setting.ModelRequestRateLimitDurationMinutes,
			"period_count":     periods,
			"start_timestamp":  queryStart,
			"end_timestamp":    queryEnd,
			"periods":          periodData,
		},
	})
}
