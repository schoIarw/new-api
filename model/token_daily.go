package model

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// ============================================================
// Materialized summary tables (replacing the views for large data)
// ============================================================

// TokenDailyModelSummary is the materialized table for per-token per-model daily stats.
type TokenDailyModelSummary struct {
	ID                   int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	StatDate             string `json:"stat_date" gorm:"type:date;uniqueIndex:idx_tdms_date_token_model,priority:1;index:idx_tdms_date_key,priority:1;index"`
	TokenID              int    `json:"token_id" gorm:"uniqueIndex:idx_tdms_date_token_model,priority:2;index:idx_tdms_token_id"`
	TokenKey             string `json:"token_key" gorm:"type:varchar(64);index:idx_tdms_date_key,priority:2;index:idx_tdms_token_key"`
	TokenName            string `json:"token_name" gorm:"type:varchar(128);default:''"`
	TokenGroup           string `json:"token_group" gorm:"type:varchar(64);default:''"`
	ModelName             string  `json:"model_name" gorm:"type:varchar(128);uniqueIndex:idx_tdms_date_token_model,priority:3"`
	BillingType           string  `json:"billing_type" gorm:"-"`
	CurrentModelRatio     float64 `json:"current_model_ratio" gorm:"-"`
	CurrentFixedPrice     float64 `json:"current_fixed_price" gorm:"-"`
	CurrentCompletionRatio float64 `json:"current_completion_ratio" gorm:"-"`
	RequestCount          int64   `json:"request_count" gorm:"default:0"`
	TotalQuota            int64   `json:"total_quota" gorm:"default:0;index:idx_tdms_quota"`
	EstimatedUsd          float64 `json:"estimated_usd" gorm:"-"`
	EstimatedCny          float64 `json:"estimated_cny" gorm:"-"`
	TotalPromptTokens     int64   `json:"total_prompt_tokens" gorm:"default:0"`
	TotalCompletionTokens int64  `json:"total_completion_tokens" gorm:"default:0"`
	TotalTokens           int64   `json:"total_tokens" gorm:"default:0"`
	FirstRequestAt       string `json:"first_request_at" gorm:"type:varchar(32);default:"`
	LastRequestAt        string `json:"last_request_at" gorm:"type:varchar(32);default:"`
	UpdatedAt            int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TokenDailyTotalSummary is the materialized table for per-token daily total stats.
type TokenDailyTotalSummary struct {
	ID                   int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	StatDate             string `json:"stat_date" gorm:"type:date;uniqueIndex:idx_tdts_date_token,priority:1;index"`
	TokenID              int    `json:"token_id" gorm:"uniqueIndex:idx_tdts_date_token,priority:2;index:idx_tdts_token_id"`
	TokenKey             string `json:"token_key" gorm:"type:varchar(64);index:idx_tdts_token_key;index:idx_tdts_date_key,priority:2"`
	TokenName            string `json:"token_name" gorm:"type:varchar(128);default:''"`
	TokenGroup           string `json:"token_group" gorm:"type:varchar(64);default:''"`
	ModelsUsed           string `json:"models_used" gorm:"type:text"`
	DistinctModels       int64  `json:"distinct_models" gorm:"default:0"`
	TotalRequests        int64  `json:"total_requests" gorm:"default:0"`
	TotalQuota           int64  `json:"total_quota" gorm:"default:0;index:idx_tdts_quota"`
	EstimatedUsd         float64 `json:"estimated_usd" gorm:"-"`
	EstimatedCny         float64 `json:"estimated_cny" gorm:"-"`
	TotalPromptTokens    int64  `json:"total_prompt_tokens" gorm:"default:0"`
	TotalCompletionTokens int64 `json:"total_completion_tokens" gorm:"default:0"`
	TotalTokens          int64  `json:"total_tokens" gorm:"default:0"`
	DayFirstRequest      string `json:"day_first_request" gorm:"type:varchar(32);default:"`
	DayLastRequest       string `json:"day_last_request" gorm:"type:varchar(32);default:"`
	UpdatedAt            int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName sets the table name for TokenDailyModelSummary
func (TokenDailyModelSummary) TableName() string {
	return "token_daily_model_summary"
}

// TableName sets the table name for TokenDailyTotalSummary
func (TokenDailyTotalSummary) TableName() string {
	return "token_daily_total_summary"
}

// ============================================================
// TokenDailyQueryParams holds query parameters for token daily views
type TokenDailyQueryParams struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	TokenKey  string `json:"token_key"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
}

// TokenDailyResponse holds the paginated response
type TokenDailyResponse struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// ============================================================
// Incremental update cache for daily summaries
// ============================================================

var (
	tokenDailySummaryCache     = make(map[string]*tokenDailySummaryDelta)
	tokenDailySummaryCacheLock sync.Mutex
)

// tokenDailySummaryDelta holds the pending increments for a summary row
type tokenDailySummaryDelta struct {
	modelKey string // composite key for model-level
	totalKey string // composite key for total-level
	// Model-level fields
	statDate    string
	tokenID     int
	tokenKey    string
	tokenName   string
	tokenGroup  string
	modelName   string
	quota       int64
	promptTokens int64
	completionTokens int64
	totalTokens  int64
	count       int64
	createdAt   int64
}

// modelRowKey is the composite key for model-level aggregation in the flush cache
type modelRowKey struct {
	StatDate   string
	TokenID    int
	ModelName  string
}

// modelRow holds the aggregated values for a model-level summary row
type modelRow struct {
	quota       int64
	promptTokens int64
	completionTokens int64
	totalTokens  int64
	count       int64
	firstAt     int64
	lastAt      int64
	tokenKey    string
	tokenName   string
	tokenGroup  string
	TokenID    int
}

// totalRowKey is the composite key for total-level aggregation in the flush cache
type totalRowKey struct {
	StatDate string
	TokenID  int
}

// totalRow holds the aggregated values for a total-level summary row
type totalRow struct {
	quota       int64
	promptTokens int64
	completionTokens int64
	totalTokens  int64
	count       int64
	firstAt     int64
	lastAt      int64
	tokenKey    string
	tokenName   string
	tokenGroup  string
	TokenID    int
	models      map[string]struct{}
}

// modelDeltaKey returns the unique key for a model-level summary row
func modelDeltaKey(statDate string, tokenID int, modelName string) string {
	return statDate + ":" + itoa(tokenID) + ":" + modelName
}

// totalDeltaKey returns the unique key for a total-level summary row
func totalDeltaKey(statDate string, tokenID int) string {
	return statDate + ":" + itoa(tokenID)
}

// simple itoa for int
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// UpdateTokenDailySummary is called when a consume log is recorded.
// It caches the delta and periodically flushes to DB.
func UpdateTokenDailySummary(statDate string, tokenID int, tokenKey string, tokenName string, tokenGroup string, modelName string, quota int, promptTokens int, completionTokens int, totalTokens int, createdAt int64) {
	if !common.DataExportEnabled {
		return
	}
	delta := &tokenDailySummaryDelta{
		statDate:         statDate,
		tokenID:          tokenID,
		tokenKey:         tokenKey,
		tokenName:        tokenName,
		tokenGroup:       tokenGroup,
		modelName:        modelName,
		quota:            int64(quota),
		promptTokens:     int64(promptTokens),
		completionTokens: int64(completionTokens),
		totalTokens:      int64(totalTokens),
		count:            1,
		createdAt:        createdAt,
	}
	mKey := modelDeltaKey(statDate, tokenID, modelName)
	tKey := totalDeltaKey(statDate, tokenID)
	delta.modelKey = mKey
	delta.totalKey = tKey

	tokenDailySummaryCacheLock.Lock()
	defer tokenDailySummaryCacheLock.Unlock()

	if existing, ok := tokenDailySummaryCache[mKey]; ok {
		existing.quota += delta.quota
		existing.promptTokens += delta.promptTokens
		existing.completionTokens += delta.completionTokens
		existing.totalTokens += delta.totalTokens
		existing.count += delta.count
		if createdAt < existing.createdAt {
			existing.createdAt = createdAt
		}
	} else {
		tokenDailySummaryCache[mKey] = delta
	}
}

// FlushTokenDailySummaryCache flushes the cached deltas to the database.
// This should be called periodically by a background goroutine.
func FlushTokenDailySummaryCache() {
	tokenDailySummaryCacheLock.Lock()
	cacheSize := len(tokenDailySummaryCache)
	if cacheSize == 0 {
		tokenDailySummaryCacheLock.Unlock()
		return
	}
	// Swap the cache
	oldCache := tokenDailySummaryCache
	tokenDailySummaryCache = make(map[string]*tokenDailySummaryDelta)
	tokenDailySummaryCacheLock.Unlock()

	common.SysLog("flushing token daily summary cache: " + itoa(len(oldCache)) + " entries")

	// Collect model-level updates
	modelAgg := make(map[modelRowKey]*modelRow)
	// Collect total-level updates
	totalAgg := make(map[totalRowKey]*totalRow)

	for _, d := range oldCache {
		// Model level
		mk := modelRowKey{StatDate: d.statDate, TokenID: d.tokenID, ModelName: d.modelName}
		if mr, ok := modelAgg[mk]; ok {
			mr.quota += d.quota
			mr.promptTokens += d.promptTokens
			mr.completionTokens += d.completionTokens
			mr.totalTokens += d.totalTokens
			mr.count += d.count
			if d.createdAt < mr.firstAt {
				mr.firstAt = d.createdAt
			}
			if d.createdAt > mr.lastAt {
				mr.lastAt = d.createdAt
			}
		} else {
			modelAgg[mk] = &modelRow{
				quota: d.quota, promptTokens: d.promptTokens,
				completionTokens: d.completionTokens, totalTokens: d.totalTokens,
				count: d.count, firstAt: d.createdAt, lastAt: d.createdAt,
				tokenKey: d.tokenKey, tokenName: d.tokenName, tokenGroup: d.tokenGroup,
				TokenID: d.tokenID,
			}
		}

		// Total level
		tk := totalRowKey{StatDate: d.statDate, TokenID: d.tokenID}
		if tr, ok := totalAgg[tk]; ok {
			tr.quota += d.quota
			tr.promptTokens += d.promptTokens
			tr.completionTokens += d.completionTokens
			tr.totalTokens += d.totalTokens
			tr.count += d.count
			if d.createdAt < tr.firstAt {
				tr.firstAt = d.createdAt
			}
			if d.createdAt > tr.lastAt {
				tr.lastAt = d.createdAt
			}
			tr.models[d.modelName] = struct{}{}
		} else {
			totalAgg[tk] = &totalRow{
				quota: d.quota, promptTokens: d.promptTokens,
				completionTokens: d.completionTokens, totalTokens: d.totalTokens,
				count: d.count, firstAt: d.createdAt, lastAt: d.createdAt,
				tokenKey: d.tokenKey, tokenName: d.tokenName, tokenGroup: d.tokenGroup,
				TokenID: d.tokenID,
				models: map[string]struct{}{d.modelName: {}},
			}
		}
	}

	// Resolve empty token keys
	func() {
		tokenIDs := make(map[int]bool)
		for _, mr := range modelAgg {
			if mr.tokenKey == "" && mr.TokenID > 0 {
				tokenIDs[mr.TokenID] = true
			}
		}
		for _, tr := range totalAgg {
			if tr.tokenKey == "" && tr.TokenID > 0 {
				tokenIDs[tr.TokenID] = true
			}
		}
		if len(tokenIDs) > 0 {
			ids := make([]int, 0, len(tokenIDs))
			for id := range tokenIDs {
				ids = append(ids, id)
			}
			var rows []struct {
				ID  int    `gorm:"column:id"`
				Key string `gorm:"column:key"`
			}
			if err := DB.Table("tokens").Select("id, `key`").Where("id IN ?", ids).Find(&rows).Error; err == nil {
				keyMap := make(map[int]string, len(rows))
				for _, r := range rows {
					keyMap[r.ID] = r.Key
				}
				for _, mr := range modelAgg {
					if mr.tokenKey == "" && mr.TokenID > 0 {
						if k, ok := keyMap[mr.TokenID]; ok {
							mr.tokenKey = k
						}
					}
				}
				for _, tr := range totalAgg {
					if tr.tokenKey == "" && tr.TokenID > 0 {
						if k, ok := keyMap[tr.TokenID]; ok {
							tr.tokenKey = k
						}
					}
				}
			}
		}
	}()

	// Flush model-level summaries
	for mk, mr := range modelAgg {
		upsertTokenDailyModelSummary(mk.StatDate, mk.TokenID, mr.tokenKey, mr.tokenName, mr.tokenGroup, mk.ModelName, mr)
	}

	// Flush total-level summaries
	for tk, tr := range totalAgg {
		upsertTokenDailyTotalSummary(tk.StatDate, tk.TokenID, tr.tokenKey, tr.tokenName, tr.tokenGroup, tr)
	}
}

func tsToStr(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func upsertTokenDailyModelSummary(statDate string, tokenID int, tokenKey, tokenName, tokenGroup, modelName string, row *modelRow) {
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) || common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		// Use INSERT ... ON DUPLICATE KEY UPDATE for MySQL
		// Use INSERT ... ON CONFLICT DO UPDATE for PostgreSQL
		if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
			DB.Exec(
				`INSERT INTO token_daily_model_summary (stat_date, token_id, token_key, token_name, token_group, model_name, request_count, total_quota, total_prompt_tokens, total_completion_tokens, total_tokens, first_request_at, last_request_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
					request_count = request_count + VALUES(request_count),
					total_quota = total_quota + VALUES(total_quota),
					total_prompt_tokens = total_prompt_tokens + VALUES(total_prompt_tokens),
					total_completion_tokens = total_completion_tokens + VALUES(total_completion_tokens),
					total_tokens = total_tokens + VALUES(total_tokens),
					first_request_at = LEAST(first_request_at, VALUES(first_request_at)),
					last_request_at = GREATEST(last_request_at, VALUES(last_request_at)),
					token_name = VALUES(token_name),
					token_key = VALUES(token_key),
					token_group = VALUES(token_group)`,
				statDate, tokenID, tokenKey, tokenName, tokenGroup, modelName,
				row.count, row.quota, row.promptTokens, row.completionTokens, row.totalTokens,
				tsToStr(row.firstAt), tsToStr(row.lastAt),
			)
		} else {
			// PostgreSQL
			DB.Exec(
				`INSERT INTO token_daily_model_summary (stat_date, token_id, token_key, token_name, token_group, model_name, request_count, total_quota, total_prompt_tokens, total_completion_tokens, total_tokens, first_request_at, last_request_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
				ON CONFLICT (stat_date, token_id, model_name) DO UPDATE SET
					request_count = token_daily_model_summary.request_count + EXCLUDED.request_count,
					total_quota = token_daily_model_summary.total_quota + EXCLUDED.total_quota,
					total_prompt_tokens = token_daily_model_summary.total_prompt_tokens + EXCLUDED.total_prompt_tokens,
					total_completion_tokens = token_daily_model_summary.total_completion_tokens + EXCLUDED.total_completion_tokens,
					total_tokens = token_daily_model_summary.total_tokens + EXCLUDED.total_tokens,
					first_request_at = LEAST(token_daily_model_summary.first_request_at, EXCLUDED.first_request_at),
					last_request_at = GREATEST(token_daily_model_summary.last_request_at, EXCLUDED.last_request_at),
					token_name = EXCLUDED.token_name,
					token_key = EXCLUDED.token_key,
					token_group = EXCLUDED.token_group`,
				statDate, tokenID, tokenKey, tokenName, tokenGroup, modelName,
				row.count, row.quota, row.promptTokens, row.completionTokens, row.totalTokens,
				tsToStr(row.firstAt), tsToStr(row.lastAt),
			)
		}
	} else {
		// SQLite — use INSERT OR REPLACE
		DB.Exec(
			`INSERT OR REPLACE INTO token_daily_model_summary (stat_date, token_id, token_key, token_name, token_group, model_name, request_count, total_quota, total_prompt_tokens, total_completion_tokens, total_tokens, first_request_at, last_request_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			statDate, tokenID, tokenKey, tokenName, tokenGroup, modelName,
			row.count, row.quota, row.promptTokens, row.completionTokens, row.totalTokens,
			tsToStr(row.firstAt), tsToStr(row.lastAt),
		)
	}
}

func upsertTokenDailyTotalSummary(statDate string, tokenID int, tokenKey, tokenName, tokenGroup string, row *totalRow) {
	// Build models_used string
	models := make([]string, 0, len(row.models))
	for m := range row.models {
		models = append(models, m)
	}
	modelsUsed := ""
	for i, m := range models {
		if i > 0 {
			modelsUsed += ", "
		}
		modelsUsed += m
	}
	distinctModels := int64(len(models))

	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		DB.Exec(
			`INSERT INTO token_daily_total_summary (stat_date, token_id, token_key, token_name, token_group, models_used, distinct_models, total_requests, total_quota, total_prompt_tokens, total_completion_tokens, total_tokens, day_first_request, day_last_request)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				models_used = VALUES(models_used),
				distinct_models = VALUES(distinct_models),
				total_requests = total_requests + VALUES(total_requests),
				total_quota = total_quota + VALUES(total_quota),
				total_prompt_tokens = total_prompt_tokens + VALUES(total_prompt_tokens),
				total_completion_tokens = total_completion_tokens + VALUES(total_completion_tokens),
				total_tokens = total_tokens + VALUES(total_tokens),
				day_first_request = LEAST(day_first_request, VALUES(day_first_request)),
				day_last_request = GREATEST(day_last_request, VALUES(day_last_request)),
				token_name = VALUES(token_name),
				token_key = VALUES(token_key),
				token_group = VALUES(token_group)`,
			statDate, tokenID, tokenKey, tokenName, tokenGroup, modelsUsed, distinctModels,
			row.count, row.quota, row.promptTokens, row.completionTokens, row.totalTokens,
			tsToStr(row.firstAt), tsToStr(row.lastAt),
		)
	} else if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		DB.Exec(
			`INSERT INTO token_daily_total_summary (stat_date, token_id, token_key, token_name, token_group, models_used, distinct_models, total_requests, total_quota, total_prompt_tokens, total_completion_tokens, total_tokens, day_first_request, day_last_request)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			ON CONFLICT (stat_date, token_id) DO UPDATE SET
				models_used = EXCLUDED.models_used,
				distinct_models = EXCLUDED.distinct_models,
				total_requests = token_daily_total_summary.total_requests + EXCLUDED.total_requests,
				total_quota = token_daily_total_summary.total_quota + EXCLUDED.total_quota,
				total_prompt_tokens = token_daily_total_summary.total_prompt_tokens + EXCLUDED.total_prompt_tokens,
				total_completion_tokens = token_daily_total_summary.total_completion_tokens + EXCLUDED.total_completion_tokens,
				total_tokens = token_daily_total_summary.total_tokens + EXCLUDED.total_tokens,
				day_first_request = LEAST(token_daily_total_summary.day_first_request, EXCLUDED.day_first_request),
				day_last_request = GREATEST(token_daily_total_summary.day_last_request, EXCLUDED.day_last_request),
				token_name = EXCLUDED.token_name,
				token_key = EXCLUDED.token_key,
				token_group = EXCLUDED.token_group`,
			statDate, tokenID, tokenKey, tokenName, tokenGroup, modelsUsed, distinctModels,
			row.count, row.quota, row.promptTokens, row.completionTokens, row.totalTokens,
			tsToStr(row.firstAt), tsToStr(row.lastAt),
		)
	} else {
		// SQLite
		DB.Exec(
			`INSERT OR REPLACE INTO token_daily_total_summary (stat_date, token_id, token_key, token_name, token_group, models_used, distinct_models, total_requests, total_quota, total_prompt_tokens, total_completion_tokens, total_tokens, day_first_request, day_last_request)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			statDate, tokenID, tokenKey, tokenName, tokenGroup, modelsUsed, distinctModels,
			row.count, row.quota, row.promptTokens, row.completionTokens, row.totalTokens,
			tsToStr(row.firstAt), tsToStr(row.lastAt),
		)
	}
}

// ============================================================
// Background flush goroutine
// ============================================================

// 注释掉：账单功能写入每30秒逐行写两张汇总表
// StartTokenDailySummaryFlusher starts a background goroutine that periodically
// flushes the in-memory summary cache to the database.
//func StartTokenDailySummaryFlusher() {
//	if !common.DataExportEnabled {
//		return
//	}
//	go func() {
//		for {
//			time.Sleep(30 * time.Second)
//			FlushTokenDailySummaryCache()
//		}
//	}()
//}

// ============================================================
// Query functions using materialized summary tables (optimized path)
// ============================================================

// useTokenDailyDB returns the appropriate DB for querying.
func useTokenDailyDB() *gorm.DB {
	return DB
}

// calculateEstimatedPrices computes estimated_usd and estimated_cny from total_quota.
func calculateEstimatedPrices(totalQuota int64) (usd float64, cny float64) {
	usd = float64(totalQuota) / common.QuotaPerUnit
	cny = usd * operation_setting.USDExchangeRate
	return
}

// enrichModelSummary calculates estimated prices for a TokenDailyModelSummary slice.
func enrichModelSummary(items []TokenDailyModelSummary) {
	for i := range items {
		items[i].EstimatedUsd, items[i].EstimatedCny = calculateEstimatedPrices(items[i].TotalQuota)
	}
}

// enrichTotalSummary calculates estimated prices for a TokenDailyTotalSummary slice.
func enrichTotalSummary(items []TokenDailyTotalSummary) {
	for i := range items {
		items[i].EstimatedUsd, items[i].EstimatedCny = calculateEstimatedPrices(items[i].TotalQuota)
	}
}

// GetTokenDailyModel retrieves paginated data from token_daily_model_summary
func GetTokenDailyModel(params TokenDailyQueryParams) (*TokenDailyResponse, error) {
	var items []TokenDailyModelSummary
	var total int64

	query := useTokenDailyDB().Model(&TokenDailyModelSummary{})
	query = applySummaryFilters(query, params)

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("stat_date DESC, total_quota DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}

		enrichModelSummary(items)

	totalPages := int(total) / params.PageSize
	if int(total)%params.PageSize > 0 {
		totalPages++
	}

	return &TokenDailyResponse{
		Items:      items,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetTokenDailyTotal retrieves paginated data from token_daily_total_summary
func GetTokenDailyTotal(params TokenDailyQueryParams) (*TokenDailyResponse, error) {
	var items []TokenDailyTotalSummary
	var total int64

	query := useTokenDailyDB().Model(&TokenDailyTotalSummary{})
	query = applySummaryFilters(query, params)

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("stat_date DESC, total_quota DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}

		enrichTotalSummary(items)

	totalPages := int(total) / params.PageSize
	if int(total)%params.PageSize > 0 {
		totalPages++
	}

	return &TokenDailyResponse{
		Items:      items,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// applySummaryFilters applies common filters to the summary query
func applySummaryFilters(query *gorm.DB, params TokenDailyQueryParams) *gorm.DB {
	if params.TokenKey != "" {
		query = query.Where("token_key = ?", params.TokenKey)
	}
	if params.StartDate != "" {
		query = query.Where("stat_date >= ?", params.StartDate)
	}
	if params.EndDate != "" {
		query = query.Where("stat_date <= ?", params.EndDate)
	}
	return query
}

// GetTokenDailyModelByUser retrieves data filtered by user's own tokens
func GetTokenDailyModelByUser(userId int, params TokenDailyQueryParams) (*TokenDailyResponse, error) {
	var userTokens []struct {
		Key string
	}
	if err := DB.Raw("SELECT `key` FROM tokens WHERE user_id = ? AND deleted_at IS NULL", userId).Scan(&userTokens).Error; err != nil {
		return nil, err
	}

	if len(userTokens) == 0 {
		return &TokenDailyResponse{
			Items:    []TokenDailyModelSummary{},
			Total:    0,
			Page:     params.Page,
			PageSize: params.PageSize,
		}, nil
	}

	keys := make([]string, len(userTokens))
	for i, t := range userTokens {
		keys[i] = t.Key
	}

	if params.StartDate == "" {
		params.StartDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if params.EndDate == "" {
		params.EndDate = time.Now().Format("2006-01-02")
	}

	var items []TokenDailyModelSummary
	var total int64

	query := useTokenDailyDB().Model(&TokenDailyModelSummary{}).Where("token_key IN ?", keys)
	query = applySummaryFilters(query, params)

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("stat_date DESC, total_quota DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}

		enrichModelSummary(items)

	totalPages := int(total) / params.PageSize
	if int(total)%params.PageSize > 0 {
		totalPages++
	}

	return &TokenDailyResponse{
		Items:      items,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetTokenDailyTotalByUser retrieves data filtered by user's own tokens
func GetTokenDailyTotalByUser(userId int, params TokenDailyQueryParams) (*TokenDailyResponse, error) {
	var userTokens []struct {
		Key string
	}
	if err := DB.Raw("SELECT `key` FROM tokens WHERE user_id = ? AND deleted_at IS NULL", userId).Scan(&userTokens).Error; err != nil {
		return nil, err
	}

	if len(userTokens) == 0 {
		return &TokenDailyResponse{
			Items:    []TokenDailyTotalSummary{},
			Total:    0,
			Page:     params.Page,
			PageSize: params.PageSize,
		}, nil
	}

	keys := make([]string, len(userTokens))
	for i, t := range userTokens {
		keys[i] = t.Key
	}

	if params.StartDate == "" {
		params.StartDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if params.EndDate == "" {
		params.EndDate = time.Now().Format("2006-01-02")
	}

	var items []TokenDailyTotalSummary
	var total int64

	query := useTokenDailyDB().Model(&TokenDailyTotalSummary{}).Where("token_key IN ?", keys)
	query = applySummaryFilters(query, params)

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("stat_date DESC, total_quota DESC").Offset(offset).Limit(params.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}

		enrichTotalSummary(items)

	totalPages := int(total) / params.PageSize
	if int(total)%params.PageSize > 0 {
		totalPages++
	}

	return &TokenDailyResponse{
		Items:      items,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

