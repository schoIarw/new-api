package setting

import "testing"

func TestModelRequestRateLimitGroupSupportsModelScopedConfig(t *testing.T) {
	original := ModelRequestRateLimitGroup2JSONString()
	defer func() {
		_ = UpdateModelRequestRateLimitGroupByJSONString(original)
	}()

	config := `{
		"testgroup": {
			"all": [10, 5],
			"qwen35-27b": [6, 3],
			"unlimited-model": [0, 0]
		},
		"18946512326": [20, 8]
	}`

	if err := CheckModelRequestRateLimitGroup(config); err != nil {
		t.Fatalf("config should be valid: %v", err)
	}
	if err := UpdateModelRequestRateLimitGroupByJSONString(config); err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	if !HasGroupModelRateLimit("testgroup") {
		t.Fatal("testgroup should be detected as model-scoped group")
	}

	allTotal, allSuccess, found := GetGroupModelRateLimit("testgroup", "all")
	if !found || allTotal != 10 || allSuccess != 5 {
		t.Fatalf("unexpected all limit: found=%v total=%d success=%d", found, allTotal, allSuccess)
	}

	modelTotal, modelSuccess, found := GetGroupModelRateLimit("testgroup", "qwen35-27b")
	if !found || modelTotal != 6 || modelSuccess != 3 {
		t.Fatalf("unexpected model limit: found=%v total=%d success=%d", found, modelTotal, modelSuccess)
	}

	zeroTotal, zeroSuccess, found := GetGroupModelRateLimit("testgroup", "unlimited-model")
	if !found || zeroTotal != 0 || zeroSuccess != 0 {
		t.Fatalf("[0,0] must be preserved as unlimited: found=%v total=%d success=%d", found, zeroTotal, zeroSuccess)
	}

	phoneTotal, phoneSuccess, found := GetGroupRateLimit("18946512326")
	if !found || phoneTotal != 20 || phoneSuccess != 8 {
		t.Fatalf("phone direct limit changed: found=%v total=%d success=%d", found, phoneTotal, phoneSuccess)
	}
}

func TestModelRequestRateLimitGroupAllowsMissingAll(t *testing.T) {
	config := `{"testgroup":{"qwen35-27b":[10,5]}}`
	if err := CheckModelRequestRateLimitGroup(config); err != nil {
		t.Fatalf("group without all should be valid: %v", err)
	}
	if err := UpdateModelRequestRateLimitGroupByJSONString(config); err != nil {
		t.Fatalf("update config failed: %v", err)
	}
	if !HasGroupModelRateLimit("testgroup") {
		t.Fatal("group should still be recognized as model-scoped")
	}
	if _, _, found := GetGroupModelRateLimit("testgroup", "all"); found {
		t.Fatal("missing all must remain missing and mean no all-model limit")
	}
}

func TestModelRequestRateLimitGroupRejectsInvalidValues(t *testing.T) {
	tests := []string{
		`{"testgroup":{"all":[10]}}`,
		`{"testgroup":{"all":[-1,5]}}`,
		`{"18946512326":[10,-1]}`,
	}

	for _, config := range tests {
		if err := CheckModelRequestRateLimitGroup(config); err == nil {
			t.Fatalf("expected invalid config to fail: %s", config)
		}
	}
}
