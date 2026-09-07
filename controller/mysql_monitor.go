package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MySQLSlowQuery 慢查询
type MySQLSlowQuery struct {
	Id      uint64 `json:"id"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	Time    int64  `json:"time"`
	State   string `json:"state"`
	Info    string `json:"info"`
}

// MySQLLockWait 锁等待
type MySQLLockWait struct {
	WaitingPid    uint64 `json:"waiting_pid"`
	WaitingQuery  string `json:"waiting_query"`
	WaitingMode   string `json:"waiting_mode"`
	BlockingPid   uint64 `json:"blocking_pid"`
	BlockingQuery string `json:"blocking_query"`
	BlockingMode  string `json:"blocking_mode"`
	WaitTime      int64  `json:"wait_time"`
}

// MySQLMonitorData 监控数据
type MySQLMonitorData struct {
	// 连接
	ThreadsConnected int `json:"threads_connected"`
	MaxConnections   int `json:"max_connections"`

	// 吞吐（累计值，前端计算速率）
	Questions      int64 `json:"questions"`
	ComCommit      int64 `json:"com_commit"`
	ComRollback    int64 `json:"com_rollback"`
	BytesReceived int64 `json:"bytes_received"`
	BytesSent     int64 `json:"bytes_sent"`

	// InnoDB
	BufferPoolReadRequests int64   `json:"buffer_pool_read_requests"`
	BufferPoolReads        int64   `json:"buffer_pool_reads"`
	BufferPoolHitRate      float64 `json:"buffer_pool_hit_rate"`
	InnodbDeadlocks        int64   `json:"innodb_deadlocks"`

	// 慢查询与临时表
	SlowQueries           int64 `json:"slow_queries"`
	CreatedTmpTablesOnDisk int64 `json:"created_tmp_tables_on_disk"`

	// 慢查询列表
	SlowQueriesList []MySQLSlowQuery `json:"slow_queries_list"`

	// 锁等待列表
	LockWaits []MySQLLockWait `json:"lock_waits"`

	// 采集时间
	Timestamp int64 `json:"timestamp"`
}

// GetMySQLMonitor 获取 MySQL 监控数据
func GetMySQLMonitor(c *gin.Context) {
	if !common.UsingMainDatabase(common.DatabaseTypeMySQL) && !common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "仅支持 MySQL 数据库",
		})
		return
	}

	data := MySQLMonitorData{
		Timestamp: common.GetTimestamp(),
	}

	db := model.DB
	if !common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		db = model.LOG_DB
	}

	// 采集 SHOW GLOBAL STATUS。Threads_connected 是 STATUS 而不是 VARIABLE，
	// 必须随每次刷新从这里读取，否则连接数卡片会长期为 0/旧值。
	statusMap, err := queryMySQLGlobalStatus(db)
	if err != nil {
		common.SysError("mysql monitor: query global status failed: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "查询 MySQL 状态失败: " + err.Error(),
		})
		return
	}

	data.ThreadsConnected = int(statusMap["Threads_connected"])
	data.Questions = statusMap["Questions"]
	data.ComCommit = statusMap["Com_commit"]
	data.ComRollback = statusMap["Com_rollback"]
	data.BytesReceived = statusMap["Bytes_received"]
	data.BytesSent = statusMap["Bytes_sent"]
	data.BufferPoolReadRequests = statusMap["Innodb_buffer_pool_read_requests"]
	data.BufferPoolReads = statusMap["Innodb_buffer_pool_reads"]
	data.InnodbDeadlocks = statusMap["Innodb_deadlocks"]
	data.SlowQueries = statusMap["Slow_queries"]
	data.CreatedTmpTablesOnDisk = statusMap["Created_tmp_tables_on_disk"]

	if data.BufferPoolReadRequests > 0 {
		data.BufferPoolHitRate = (1 - float64(data.BufferPoolReads)/float64(data.BufferPoolReadRequests)) * 100
		if data.BufferPoolHitRate < 0 {
			data.BufferPoolHitRate = 0
		}
	}

	// max_connections 才是 GLOBAL VARIABLE；动态连接数从 STATUS 获取。
	varsMap, err := queryMySQLGlobalVariables(db)
	if err != nil {
		common.SysError("mysql monitor: query global variables failed: " + err.Error())
	}
	data.MaxConnections = varsMap["max_connections"]

	slowQueries, err := queryMySQLSlowQueries(db)
	if err != nil {
		common.SysError("mysql monitor: query slow queries failed: " + err.Error())
	}
	data.SlowQueriesList = slowQueries

	lockWaits, err := queryMySQLLockWaits(db)
	if err != nil {
		common.SysError("mysql monitor: query lock waits failed: " + err.Error())
	}
	data.LockWaits = lockWaits

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// KillMySQLProcess kill 指定 MySQL 进程
func KillMySQLProcess(c *gin.Context) {
	if !common.UsingMainDatabase(common.DatabaseTypeMySQL) && !common.UsingLogDatabase(common.DatabaseTypeMySQL) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "仅支持 MySQL 数据库",
		})
		return
	}

	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的进程 ID",
		})
		return
	}

	db := model.DB
	if !common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		db = model.LOG_DB
	}

	if err := db.Exec("KILL ?", id).Error; err != nil {
		common.SysError("mysql monitor: kill process failed: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "KILL 失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已执行 KILL " + idStr,
	})
}

// queryMySQLGlobalStatus 执行 SHOW GLOBAL STATUS 并解析
func queryMySQLGlobalStatus(db *gorm.DB) (map[string]int64, error) {
	rows, err := db.Raw("SHOW GLOBAL STATUS").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			continue
		}
		var num int64
		for _, ch := range value {
			if ch < '0' || ch > '9' {
				num = -1
				break
			}
			num = num*10 + int64(ch-'0')
		}
		if num >= 0 {
			result[name] = num
		}
	}
	return result, nil
}

// queryMySQLGlobalVariables 执行 SHOW GLOBAL VARIABLES
func queryMySQLGlobalVariables(db *gorm.DB) (map[string]int, error) {
	rows, err := db.Raw("SHOW GLOBAL VARIABLES WHERE Variable_name = 'max_connections'").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			continue
		}
		var num int
		for _, ch := range value {
			if ch < '0' || ch > '9' {
				num = -1
				break
			}
			num = num*10 + int(ch-'0')
		}
		if num >= 0 {
			result[name] = num
		}
	}
	return result, nil
}

// queryMySQLSlowQueries 查询运行时间超过 10 秒的用户进程（排除系统用户）
func queryMySQLSlowQueries(db *gorm.DB) ([]MySQLSlowQuery, error) {
	rows, err := db.Raw("SELECT id, user, host, IFNULL(db, ''), command, time, IFNULL(state, ''), IFNULL(info, '') FROM information_schema.processlist WHERE time > 10 AND command != 'Sleep' AND user NOT IN ('system user', 'event_scheduler') ORDER BY time DESC LIMIT 100").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []MySQLSlowQuery
	for rows.Next() {
		var q MySQLSlowQuery
		if err := rows.Scan(&q.Id, &q.User, &q.Host, &q.DB, &q.Command, &q.Time, &q.State, &q.Info); err != nil {
			continue
		}
		list = append(list, q)
	}
	return list, nil
}

// queryMySQLLockWaits 查询锁等待（排除系统用户）
func queryMySQLLockWaits(db *gorm.DB) ([]MySQLLockWait, error) {
	// 使用 performance_schema.data_lock_waits 查询锁等待
	sql := `SELECT r.THREAD_ID AS waiting_thread, IFNULL(t.trx_query, '') AS waiting_query, r.LOCK_MODE AS waiting_mode, r.BLOCKING_THREAD_ID AS blocking_thread, IFNULL(bt.trx_query, '') AS blocking_query, r.LOCK_MODE AS blocking_mode, IFNULL(TIMESTAMPDIFF(SECOND, t.trx_wait_started, NOW()), 0) AS wait_time FROM performance_schema.data_lock_waits r LEFT JOIN information_schema.innodb_trx t ON r.THREAD_ID = t.trx_mysql_thread_id LEFT JOIN information_schema.innodb_trx bt ON r.BLOCKING_THREAD_ID = bt.trx_mysql_thread_id LEFT JOIN information_schema.processlist pw ON r.THREAD_ID = pw.id LEFT JOIN information_schema.processlist pb ON r.BLOCKING_THREAD_ID = pb.id WHERE IFNULL(pw.user, '') NOT IN ('system user', 'event_scheduler') AND IFNULL(pb.user, '') NOT IN ('system user', 'event_scheduler') LIMIT 50`
	rows, err := db.Raw(sql).Rows()
	if err != nil {
		// performance_schema 可能未启用，返回空列表
		return nil, nil
	}
	defer rows.Close()

	var list []MySQLLockWait
	for rows.Next() {
		var lw MySQLLockWait
		if err := rows.Scan(&lw.WaitingPid, &lw.WaitingQuery, &lw.WaitingMode, &lw.BlockingPid, &lw.BlockingQuery, &lw.BlockingMode, &lw.WaitTime); err != nil {
			continue
		}
		list = append(list, lw)
	}
	return list, nil
}
