package model

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LogHistory has the same columns as Log and preserves the original log ID.
// Index names are archive-specific because PostgreSQL index names are shared
// across a schema and cannot reuse the explicit names from the logs table.
type LogHistory struct {
	Id                int    `json:"id" gorm:"primaryKey;index:idx_history_created_at_id,priority:1"`
	UserId            int    `json:"user_id" gorm:"index:idx_history_user_id"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_history_created_at_id,priority:2;index:idx_history_type_created_at,priority:2"`
	Type              int    `json:"type" gorm:"index:idx_history_type_created_at,priority:1"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"default:''"`
	TokenName         string `json:"token_name" gorm:"default:''"`
	ModelName         string `json:"model_name" gorm:"default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	UseTime           int    `json:"use_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream"`
	ChannelId         int    `json:"channel"`
	ChannelName       string `json:"channel_name" gorm:"-"`
	TokenId           int    `json:"token_id" gorm:"default:0"`
	Group             string `json:"group"`
	Ip                string `json:"ip" gorm:"default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_history_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_history_upstream_request_id;default:''"`
	Other             string `json:"other"`
	Account           string `json:"account"`
}

func (LogHistory) TableName() string { return "logs_history" }

type LogHistoryMigrationResult struct {
	CutoffTimestamp int64 `json:"cutoff_timestamp"`
	MigratedCount   int64 `json:"migrated_count"`
	BatchCount      int   `json:"batch_count"`
}

type LogHistoryStats struct {
	Supported             bool   `json:"supported"`
	UnsupportedReason     string `json:"unsupported_reason,omitempty"`
	LogsCount             int64  `json:"logs_count"`
	HistoryCount          int64  `json:"history_count"`
	EligibleCount         int64  `json:"eligible_count"`
	OldestLogTimestamp    int64  `json:"oldest_log_timestamp"`
	NewestLogTimestamp    int64  `json:"newest_log_timestamp"`
	OldestHistoryTimestamp int64 `json:"oldest_history_timestamp"`
	NewestHistoryTimestamp int64 `json:"newest_history_timestamp"`
}

func LogHistoryMigrationSupported() (bool, string) {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return false, "ClickHouse 日志库使用 TTL 管理历史数据，不支持迁移到 logs_history"
	}
	return true, ""
}

// MigrateLogsToHistory moves complete batches in a transaction. OnConflict is
// intentionally ignored: a prior interrupted run may already have copied a row,
// and the source row can then be deleted safely because its archive copy exists.
func MigrateLogsToHistory(ctx context.Context, cutoffTimestamp int64, batchSize int) (LogHistoryMigrationResult, error) {
	result := LogHistoryMigrationResult{CutoffTimestamp: cutoffTimestamp}
	if supported, reason := LogHistoryMigrationSupported(); !supported {
		return result, errors.New(reason)
	}
	if cutoffTimestamp <= 0 {
		return result, errors.New("日志迁移截止时间无效")
	}
	if batchSize < 1 {
		batchSize = 1000
	}
	if batchSize > 10000 {
		batchSize = 10000
	}

	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		var logs []Log
		if err := LOG_DB.WithContext(ctx).
			Where("created_at < ?", cutoffTimestamp).
			Order("id ASC").
			Limit(batchSize).
			Find(&logs).Error; err != nil {
			return result, err
		}
		if len(logs) == 0 {
			return result, nil
		}

		ids := make([]int, 0, len(logs))
		history := make([]LogHistory, 0, len(logs))
		for _, logRow := range logs {
			ids = append(ids, logRow.Id)
			history = append(history, LogHistory(logRow))
		}

		if err := LOG_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&history, batchSize).Error; err != nil {
				return err
			}
			return tx.Where("id IN ?", ids).Delete(&Log{}).Error
		}); err != nil {
			return result, err
		}

		result.MigratedCount += int64(len(logs))
		result.BatchCount++
		if len(logs) < batchSize {
			return result, nil
		}
	}
}

func GetLogHistoryStats(ctx context.Context, retentionDays int) (LogHistoryStats, error) {
	stats := LogHistoryStats{}
	stats.Supported, stats.UnsupportedReason = LogHistoryMigrationSupported()
	if !stats.Supported {
		return stats, nil
	}
	if retentionDays < 1 {
		retentionDays = 31
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	queries := []struct {
		table string
		count *int64
		oldest *int64
		newest *int64
	}{
		{table: "logs", count: &stats.LogsCount, oldest: &stats.OldestLogTimestamp, newest: &stats.NewestLogTimestamp},
		{table: "logs_history", count: &stats.HistoryCount, oldest: &stats.OldestHistoryTimestamp, newest: &stats.NewestHistoryTimestamp},
	}
	for _, query := range queries {
		var row struct {
			Count  int64
			Oldest *int64
			Newest *int64
		}
		if err := LOG_DB.WithContext(ctx).Table(query.table).
			Select("count(*) as count, min(created_at) as oldest, max(created_at) as newest").
			Scan(&row).Error; err != nil {
			return stats, err
		}
		*query.count = row.Count
		if row.Oldest != nil {
			*query.oldest = *row.Oldest
		}
		if row.Newest != nil {
			*query.newest = *row.Newest
		}
	}
	if err := LOG_DB.WithContext(ctx).Table("logs").Where("created_at < ?", cutoff).Count(&stats.EligibleCount).Error; err != nil {
		return stats, err
	}
	return stats, nil
}
