package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	defaultVLLMPrometheusJob       = "vllm-model-server"
	defaultPrometheusQueryTimeout  = 15
	modelDashboardQueryConcurrency = 4
)

type prometheusRangeResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string   `json:"metric"`
			Values [][]json.RawMessage `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

type ModelRuntimePoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type ModelRuntimeSeries struct {
	ModelName string              `json:"model_name"`
	Points    []ModelRuntimePoint `json:"points"`
}

type ModelRuntimeSummary struct {
	ModelName            string  `json:"model_name"`
	Running              float64 `json:"running"`
	Waiting              float64 `json:"waiting"`
	QueueAvgSeconds      float64 `json:"queue_avg_seconds"`
	QueueP95Seconds      float64 `json:"queue_p95_seconds"`
	PrefillTokensPerSec  float64 `json:"prefill_tokens_per_sec"`
	DecodeTokensPerSec   float64 `json:"decode_tokens_per_sec"`
	PrefixCacheHitRate   float64 `json:"prefix_cache_hit_rate"`
	KVCacheUsage         float64 `json:"kv_cache_usage"`
	TTFTP95Seconds       float64 `json:"ttft_p95_seconds"`
	E2EP95Seconds        float64 `json:"e2e_p95_seconds"`
	RequestPerSec        float64 `json:"request_per_sec"`
	ErrorRate            float64 `json:"error_rate"`
	PreemptionsPerMin    float64 `json:"preemptions_per_min"`
}

type modelMetricDefinition struct {
	Key   string
	Query string
}

// GetVLLMModelRuntimeDashboard 从 Prometheus 查询 vLLM 运行指标。
// 实时模式：hours=1/2/4/8，持续刷新由前端控制。
// 历史模式：start_timestamp + end_timestamp，不限制查询跨度；后端会根据跨度自动放大 step，
// 控制单条时序的点数，避免超长历史查询返回过多采样点。
func GetVLLMModelRuntimeDashboard(c *gin.Context) {
	prometheusURL := strings.TrimSpace(common.GetEnvOrDefaultString("PROMETHEUS_URL", ""))
	if prometheusURL == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "未配置 PROMETHEUS_URL，无法查询模型运行指标",
		})
		return
	}

	parsedURL, err := url.Parse(strings.TrimRight(prometheusURL, "/"))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "PROMETHEUS_URL 配置无效",
		})
		return
	}

	startTimestamp, endTimestamp, historical, err := resolveModelDashboardRange(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	stepSeconds := modelDashboardStepSeconds(endTimestamp-startTimestamp, historical)
	rateWindowSeconds := modelDashboardRateWindowSeconds(stepSeconds)
	job := common.GetEnvOrDefaultString("VLLM_PROMETHEUS_JOB", defaultVLLMPrometheusJob)
	definitions := buildVLLMModelMetricDefinitions(job, rateWindowSeconds)

	timeoutSeconds := common.GetEnvOrDefault("PROMETHEUS_QUERY_TIMEOUT_SECONDS", defaultPrometheusQueryTimeout)
	if timeoutSeconds < 1 {
		timeoutSeconds = defaultPrometheusQueryTimeout
	}

	queryContext, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	bearerToken := strings.TrimSpace(common.GetEnvOrDefaultString("PROMETHEUS_BEARER_TOKEN", ""))
	metricResults := make(map[string][]ModelRuntimeSeries, len(definitions))
	var resultMu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, modelDashboardQueryConcurrency)

	for _, definition := range definitions {
		definition := definition
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			series, queryErr := queryPrometheusRange(
				queryContext,
				client,
				parsedURL.String(),
				bearerToken,
				definition.Query,
				startTimestamp,
				endTimestamp,
				stepSeconds,
			)

			resultMu.Lock()
			defer resultMu.Unlock()
			if queryErr != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("查询 %s 指标失败: %w", definition.Key, queryErr)
				}
				return
			}
			metricResults[definition.Key] = series
		}()
	}

	wg.Wait()
	if firstErr != nil {
		common.SysError("model runtime dashboard: " + firstErr.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": firstErr.Error(),
		})
		return
	}

	summary, modelNames := buildModelRuntimeSummary(metricResults)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"source":              "prometheus",
			"job":                 job,
			"historical":          historical,
			"start_timestamp":     startTimestamp,
			"end_timestamp":       endTimestamp,
			"step_seconds":        stepSeconds,
			"rate_window_seconds": rateWindowSeconds,
			"model_names":         modelNames,
			"summary":             summary,
			"metrics":             metricResults,
		},
	})
}

func resolveModelDashboardRange(c *gin.Context) (startTimestamp int64, endTimestamp int64, historical bool, err error) {
	startTS, startErr := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTS, endErr := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	startProvided := c.Query("start_timestamp") != ""
	endProvided := c.Query("end_timestamp") != ""
	if startProvided || endProvided {
		if startErr != nil || endErr != nil || startTS <= 0 || endTS <= 0 {
			return 0, 0, true, fmt.Errorf("历史查询开始和结束时间必须是有效 Unix 时间戳")
		}
		if endTS <= startTS {
			return 0, 0, true, fmt.Errorf("历史查询结束时间必须晚于开始时间")
		}
		return startTS, endTS, true, nil
	}

	hours, _ := strconv.Atoi(c.Query("hours"))
	if hours <= 0 {
		hours = 1
	}
	if hours > 8 {
		hours = 8
	}
	endTimestamp = time.Now().Unix()
	startTimestamp = endTimestamp - int64(hours)*3600
	return startTimestamp, endTimestamp, false, nil
}

// modelDashboardStepSeconds 控制每条时序的采样点数量。
// 实时查询保持较细粒度；历史查询没有跨度上限，但目标控制在约 720 点以内。
func modelDashboardStepSeconds(durationSeconds int64, historical bool) int64 {
	if durationSeconds <= 0 {
		return 60
	}
	if !historical {
		switch {
		case durationSeconds <= 2*3600:
			return 30
		case durationSeconds <= 4*3600:
			return 60
		default:
			return 120
		}
	}

	rawStep := int64(math.Ceil(float64(durationSeconds) / 720.0))
	friendlySteps := []int64{60, 120, 300, 600, 900, 1800, 3600, 7200, 14400, 21600, 43200, 86400}
	for _, step := range friendlySteps {
		if rawStep <= step {
			return step
		}
	}
	return ((rawStep + 86399) / 86400) * 86400
}

func modelDashboardRateWindowSeconds(stepSeconds int64) int64 {
	window := stepSeconds * 4
	if window < 300 {
		window = 300
	}
	return window
}

func buildVLLMModelMetricDefinitions(job string, rateWindowSeconds int64) []modelMetricDefinition {
	jobMatcher := fmt.Sprintf(`job="%s"`, escapePrometheusLabelValue(job))
	selector := func(extra string) string {
		if extra == "" {
			return "{" + jobMatcher + "}"
		}
		return "{" + jobMatcher + "," + extra + "}"
	}
	window := fmt.Sprintf("%ds", rateWindowSeconds)

	return []modelMetricDefinition{
		{
			Key:   "running",
			Query: fmt.Sprintf(`sum by (model_name) (vllm:num_requests_running%s)`, selector("")),
		},
		{
			Key:   "waiting",
			Query: fmt.Sprintf(`sum by (model_name) (vllm:num_requests_waiting%s)`, selector("")),
		},
		{
			Key: "queue_avg_seconds",
			Query: fmt.Sprintf(
				`sum by (model_name) (rate(vllm:request_queue_time_seconds_sum%s[%s])) / clamp_min(sum by (model_name) (rate(vllm:request_queue_time_seconds_count%s[%s])), 1e-9)`,
				selector(""), window, selector(""), window,
			),
		},
		{
			Key: "queue_p95_seconds",
			Query: fmt.Sprintf(
				`histogram_quantile(0.95, sum by (le, model_name) (rate(vllm:request_queue_time_seconds_bucket%s[%s])))`,
				selector(""), window,
			),
		},
		{
			Key: "prefill_tokens_per_sec",
			Query: fmt.Sprintf(
				`sum by (model_name) (rate(vllm:prompt_tokens_total%s[%s]))`,
				selector(""), window,
			),
		},
		{
			Key: "decode_tokens_per_sec",
			Query: fmt.Sprintf(
				`sum by (model_name) (rate(vllm:generation_tokens_total%s[%s]))`,
				selector(""), window,
			),
		},
		{
			Key: "prefix_cache_hit_rate",
			Query: fmt.Sprintf(
				`100 * sum by (model_name) (rate(vllm:prefix_cache_hits_total%s[%s])) / clamp_min(sum by (model_name) (rate(vllm:prefix_cache_queries_total%s[%s])), 1e-9)`,
				selector(""), window, selector(""), window,
			),
		},
		{
			Key:   "kv_cache_usage",
			Query: fmt.Sprintf(`100 * avg by (model_name) (vllm:kv_cache_usage_perc%s)`, selector("")),
		},
		{
			Key: "ttft_p95_seconds",
			Query: fmt.Sprintf(
				`histogram_quantile(0.95, sum by (le, model_name) (rate(vllm:time_to_first_token_seconds_bucket%s[%s])))`,
				selector(""), window,
			),
		},
		{
			Key: "e2e_p95_seconds",
			Query: fmt.Sprintf(
				`histogram_quantile(0.95, sum by (le, model_name) (rate(vllm:e2e_request_latency_seconds_bucket%s[%s])))`,
				selector(""), window,
			),
		},
		{
			Key: "request_per_sec",
			Query: fmt.Sprintf(
				`sum by (model_name) (rate(vllm:request_success_total%s[%s]))`,
				selector(""), window,
			),
		},
		{
			Key: "error_rate",
			Query: fmt.Sprintf(
				`100 * sum by (model_name) (rate(vllm:request_success_total%s[%s])) / clamp_min(sum by (model_name) (rate(vllm:request_success_total%s[%s])), 1e-9)`,
				selector(`finished_reason="error"`), window, selector(""), window,
			),
		},
		{
			Key: "preemptions_per_min",
			Query: fmt.Sprintf(
				`60 * sum by (model_name) (rate(vllm:num_preemptions_total%s[%s]))`,
				selector(""), window,
			),
		},
	}
}

func escapePrometheusLabelValue(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
	)
	return replacer.Replace(value)
}

func queryPrometheusRange(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	bearerToken string,
	query string,
	startTimestamp int64,
	endTimestamp int64,
	stepSeconds int64,
) ([]ModelRuntimeSeries, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v1/query_range"
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	params := requestURL.Query()
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startTimestamp, 10))
	params.Set("end", strconv.FormatInt(endTimestamp, 10))
	params.Set("step", strconv.FormatInt(stepSeconds, 10))
	requestURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Prometheus HTTP %d", resp.StatusCode)
	}

	var result prometheusRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Status != "success" {
		if result.Error != "" {
			return nil, fmt.Errorf("%s: %s", result.ErrorType, result.Error)
		}
		return nil, fmt.Errorf("Prometheus 返回非 success 状态")
	}
	if result.Data.ResultType != "matrix" {
		return nil, fmt.Errorf("Prometheus 返回了意外的 resultType=%s", result.Data.ResultType)
	}

	series := make([]ModelRuntimeSeries, 0, len(result.Data.Result))
	for _, item := range result.Data.Result {
		modelName := strings.TrimSpace(item.Metric["model_name"])
		if modelName == "" {
			modelName = strings.TrimSpace(item.Metric["model"])
		}
		if modelName == "" {
			continue
		}

		points := make([]ModelRuntimePoint, 0, len(item.Values))
		for _, rawPoint := range item.Values {
			if len(rawPoint) != 2 {
				continue
			}
			var timestamp float64
			var valueText string
			if err := json.Unmarshal(rawPoint[0], &timestamp); err != nil {
				continue
			}
			if err := json.Unmarshal(rawPoint[1], &valueText); err != nil {
				continue
			}
			value, err := strconv.ParseFloat(valueText, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				continue
			}
			points = append(points, ModelRuntimePoint{
				Timestamp: int64(timestamp),
				Value:     value,
			})
		}
		sort.Slice(points, func(i, j int) bool { return points[i].Timestamp < points[j].Timestamp })
		series = append(series, ModelRuntimeSeries{ModelName: modelName, Points: points})
	}

	sort.Slice(series, func(i, j int) bool { return series[i].ModelName < series[j].ModelName })
	return series, nil
}

func buildModelRuntimeSummary(metricResults map[string][]ModelRuntimeSeries) ([]ModelRuntimeSummary, []string) {
	modelSet := make(map[string]struct{})
	latestValues := make(map[string]map[string]float64)

	for metricKey, metricSeries := range metricResults {
		for _, series := range metricSeries {
			modelSet[series.ModelName] = struct{}{}
			if len(series.Points) == 0 {
				continue
			}
			if latestValues[series.ModelName] == nil {
				latestValues[series.ModelName] = make(map[string]float64)
			}
			latestValues[series.ModelName][metricKey] = series.Points[len(series.Points)-1].Value
		}
	}

	modelNames := make([]string, 0, len(modelSet))
	for modelName := range modelSet {
		modelNames = append(modelNames, modelName)
	}
	sort.Strings(modelNames)

	summary := make([]ModelRuntimeSummary, 0, len(modelNames))
	for _, modelName := range modelNames {
		values := latestValues[modelName]
		summary = append(summary, ModelRuntimeSummary{
			ModelName:           modelName,
			Running:             values["running"],
			Waiting:             values["waiting"],
			QueueAvgSeconds:     values["queue_avg_seconds"],
			QueueP95Seconds:     values["queue_p95_seconds"],
			PrefillTokensPerSec: values["prefill_tokens_per_sec"],
			DecodeTokensPerSec:  values["decode_tokens_per_sec"],
			PrefixCacheHitRate:  values["prefix_cache_hit_rate"],
			KVCacheUsage:        values["kv_cache_usage"],
			TTFTP95Seconds:      values["ttft_p95_seconds"],
			E2EP95Seconds:       values["e2e_p95_seconds"],
			RequestPerSec:       values["request_per_sec"],
			ErrorRate:           values["error_rate"],
			PreemptionsPerMin:   values["preemptions_per_min"],
		})
	}
	return summary, modelNames
}
