package controller

import "testing"

func TestParseRateLimitDashboardPeriods(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "default", raw: "", want: 10},
		{name: "ten", raw: "10", want: 10},
		{name: "twenty", raw: "20", want: 20},
		{name: "forty", raw: "40", want: 40},
		{name: "eighty", raw: "80", want: 80},
		{name: "one is valid for API callers", raw: "1", want: 1},
		{name: "zero", raw: "0", wantErr: true},
		{name: "too large", raw: "81", wantErr: true},
		{name: "not a number", raw: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRateLimitDashboardPeriods(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got periods=%d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("periods=%d, want %d", got, tt.want)
			}
		})
	}
}

func TestRateLimitDashboardWindowRealtime(t *testing.T) {
	start, end, historical, err := rateLimitDashboardWindow(10_000, 0, 60, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if historical {
		t.Fatal("realtime window unexpectedly marked historical")
	}
	if start != 8_800 || end != 10_000 {
		t.Fatalf("window=[%d,%d), want [8800,10000)", start, end)
	}
}

func TestRateLimitDashboardWindowHistorical(t *testing.T) {
	start, end, historical, err := rateLimitDashboardWindow(99_999, 3_600, 60, 40)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !historical {
		t.Fatal("historical window not marked historical")
	}
	if start != 3_600 || end != 6_000 {
		t.Fatalf("window=[%d,%d), want [3600,6000)", start, end)
	}
}

func TestRateLimitDashboardWindowRejectsInvalidPeriodCount(t *testing.T) {
	if _, _, _, err := rateLimitDashboardWindow(10_000, 0, 60, 81); err == nil {
		t.Fatal("expected error for periods > 80")
	}
}
