package log_history_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	DefaultRetentionDays  = 31
	DefaultIntervalMinutes = 1440
	DefaultBatchSize      = 1000
)

type LogHistorySetting struct {
	Enabled         bool `json:"enabled"`
	RetentionDays   int  `json:"retention_days"`
	IntervalMinutes int  `json:"interval_minutes"`
	BatchSize       int  `json:"batch_size"`
}

var logHistorySetting = LogHistorySetting{
	Enabled:         false,
	RetentionDays:   DefaultRetentionDays,
	IntervalMinutes: DefaultIntervalMinutes,
	BatchSize:       DefaultBatchSize,
}

func init() {
	config.GlobalConfig.Register("log_history_setting", &logHistorySetting)
}

func GetSetting() LogHistorySetting {
	setting := logHistorySetting
	if setting.RetentionDays < 1 {
		setting.RetentionDays = DefaultRetentionDays
	}
	if setting.IntervalMinutes < 1 {
		setting.IntervalMinutes = DefaultIntervalMinutes
	}
	if setting.BatchSize < 1 {
		setting.BatchSize = DefaultBatchSize
	}
	if setting.BatchSize > 10000 {
		setting.BatchSize = 10000
	}
	return setting
}
