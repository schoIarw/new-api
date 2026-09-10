package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// RateLimitPeriodGroupStat is a TokenName + Account aggregate for one dashboard period.
// PeriodIndex is zero-based from queryStart: 0 is the oldest period in the requested window.
type RateLimitPeriodGroupStat struct {
	PeriodIndex int64  `json:"period_index" gorm:"column:period_index"`
	TokenName   string `json:"token_name" gorm:"column:token_name"`
	Account     string `json:"account" gorm:"column:account"`
	Count       int64  `json:"count" gorm:"column:count"`
}

// rateLimitPeriodIndexExpr returns a database-specific integer bucket expression.
// startTimestamp and durationSec are validated integer values and are formatted directly
// into the expression to keep GROUP BY portable across supported log databases.
func rateLimitPeriodIndexExpr(startTimestamp, durationSec int64) string {
	switch common.LogDatabaseType() {
	case common.DatabaseTypeClickHouse:
		return fmt.Sprintf("intDiv(created_at - %d, %d)", startTimestamp, durationSec)
	case common.DatabaseTypeMySQL:
		return fmt.Sprintf("FLOOR((created_at - %d) / %d)", startTimestamp, durationSec)
	case common.DatabaseTypePostgreSQL:
		return fmt.Sprintf("FLOOR((created_at - %d)::numeric / %d)", startTimestamp, durationSec)
	default:
		// SQLite: CAST keeps the result an integer even if the underlying expression
		// is represented as a numeric value by a particular SQLite build.
		return fmt.Sprintf("CAST((created_at - %d) / %d AS INTEGER)", startTimestamp, durationSec)
	}
}

// GetRateLimitPeriodGroupStats aggregates all requested dashboard periods in a single SQL query.
// The time range is half-open [startTimestamp, endTimestamp), avoiding duplicate counts on
// adjacent period boundaries.
func GetRateLimitPeriodGroupStats(startTimestamp, endTimestamp, durationSec int64) ([]RateLimitPeriodGroupStat, error) {
	if durationSec <= 0 || endTimestamp <= startTimestamp {
		return nil, errors.New("无效的限流看板统计时间范围")
	}

	periodExpr := rateLimitPeriodIndexExpr(startTimestamp, durationSec)
	selectExpr := fmt.Sprintf(
		"%s as period_index, token_name, account, count(*) as count",
		periodExpr,
	)
	groupExpr := fmt.Sprintf("%s, token_name, account", periodExpr)

	var stats []RateLimitPeriodGroupStat
	err := LOG_DB.Table("logs").
		Select(selectExpr).
		Where("type = ?", LogTypeConsume).
		Where("created_at >= ?", startTimestamp).
		Where("created_at < ?", endTimestamp).
		Group(groupExpr).
		Order("period_index ASC, count DESC").
		Scan(&stats).Error
	if err != nil {
		common.SysError("failed to query rate limit period group stats: " + err.Error())
		return nil, errors.New("查询限流统计数据失败")
	}
	return stats, nil
}
