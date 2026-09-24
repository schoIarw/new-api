package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrateLogsToHistoryCopiesThenDeletesInBatches(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:log_history_migration_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &LogHistory{}))

	previousDB := LOG_DB
	previousType := common.LogDatabaseType()
	LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		LOG_DB = previousDB
		common.SetLogDatabaseType(previousType)
	})

	require.NoError(t, db.Create(&[]Log{
		{Id: 1, CreatedAt: 10, Type: LogTypeConsume},
		{Id: 2, CreatedAt: 20, Type: LogTypeConsume},
		{Id: 3, CreatedAt: 100, Type: LogTypeConsume},
	}).Error)

	result, err := MigrateLogsToHistory(context.Background(), 100, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.MigratedCount)
	assert.Equal(t, 2, result.BatchCount)

	var logsCount, historyCount int64
	require.NoError(t, db.Model(&Log{}).Count(&logsCount).Error)
	require.NoError(t, db.Model(&LogHistory{}).Count(&historyCount).Error)
	assert.Equal(t, int64(1), logsCount)
	assert.Equal(t, int64(2), historyCount)
}
