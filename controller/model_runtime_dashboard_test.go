package controller

import (
	"strings"
	"testing"
)

func TestBuildVLLMModelMetricDefinitionsUsesValidLabelMatchers(t *testing.T) {
	definitions := buildVLLMModelMetricDefinitions("vllm-model-server", 300)
	if len(definitions) == 0 {
		t.Fatal("expected metric definitions")
	}

	var runningQuery string
	var errorRateQuery string
	for _, definition := range definitions {
		switch definition.Key {
		case "running":
			runningQuery = definition.Query
		case "error_rate":
			errorRateQuery = definition.Query
		}
	}

	if !strings.Contains(runningQuery, `job="vllm-model-server"`) {
		t.Fatalf("running query missing job matcher: %s", runningQuery)
	}
	if strings.Contains(runningQuery, `job=\"`) {
		t.Fatalf("running query contains escaped quote outside label value: %s", runningQuery)
	}
	if !strings.Contains(errorRateQuery, `finished_reason="error"`) {
		t.Fatalf("error rate query missing finished_reason matcher: %s", errorRateQuery)
	}
	if !strings.Contains(errorRateQuery, `[300s]`) {
		t.Fatalf("error rate query missing expected rate window: %s", errorRateQuery)
	}
}

func TestModelDashboardStepSeconds(t *testing.T) {
	tests := []struct {
		name       string
		duration   int64
		historical bool
		want       int64
	}{
		{name: "realtime 1h", duration: 3600, historical: false, want: 30},
		{name: "realtime 2h", duration: 2 * 3600, historical: false, want: 30},
		{name: "realtime 4h", duration: 4 * 3600, historical: false, want: 60},
		{name: "realtime 8h", duration: 8 * 3600, historical: false, want: 120},
		{name: "historical 24h", duration: 24 * 3600, historical: true, want: 120},
		{name: "historical 30d", duration: 30 * 24 * 3600, historical: true, want: 3600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modelDashboardStepSeconds(tt.duration, tt.historical); got != tt.want {
				t.Fatalf("modelDashboardStepSeconds() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestModelDashboardRateWindowSeconds(t *testing.T) {
	if got := modelDashboardRateWindowSeconds(30); got != 300 {
		t.Fatalf("30s step rate window = %d, want 300", got)
	}
	if got := modelDashboardRateWindowSeconds(300); got != 1200 {
		t.Fatalf("300s step rate window = %d, want 1200", got)
	}
}
