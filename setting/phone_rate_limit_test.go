package setting

import "testing"

func TestPhoneRateLimitPolicyResolution(t *testing.T) {
	original := PhoneRateLimitPolicies2JSONString()
	defer func() {
		if err := UpdatePhoneRateLimitPoliciesByJSONString(original); err != nil {
			t.Fatal(err)
		}
	}()
	raw := `{"group1":{"default":[10,5],"special":{"18946512326":[100,50],"13800138000":[0,0]}},"group2":{"default":[20,10],"special":{"18946512326":[5,2]}}}`
	if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		group, phone       string
		expected           [2]int
		configured, active bool
	}{
		{"group1", "18946512326", [2]int{100, 50}, true, true},
		{"group1", "13800138000", [2]int{}, true, true},
		{"group1", "13900139000", [2]int{10, 5}, true, true},
		{"group2", "18946512326", [2]int{5, 2}, true, true},
		{"group2", "13900139000", [2]int{20, 10}, true, true},
		{"group3", "18946512326", [2]int{}, false, false},
	}
	for _, tc := range cases {
		got, configured, active := ResolvePhoneRateLimit(tc.group, tc.phone)
		if got != tc.expected || configured != tc.configured || active != tc.active {
			t.Errorf("%s/%s: got %v %v %v", tc.group, tc.phone, got, configured, active)
		}
	}
}

func TestPhoneRateLimitRejectsInvalidConfigWithoutReplacingActivePolicy(t *testing.T) {
	original := PhoneRateLimitPolicies2JSONString()
	defer func() { _ = UpdatePhoneRateLimitPoliciesByJSONString(original) }()
	if err := UpdatePhoneRateLimitPoliciesByJSONString(`{"group1":{"default":[10,5]}}`); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`null`, `{"group1":{"default":[10]}}`, `{"group1":{"default":[-1,5]}}`,
		`{"group1":{"default":[2147483648,5]}}`, `{"group1":{"special":{"123":[5,2]}}}`,
	} {
		if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
		got, _, _ := ResolvePhoneRateLimit("group1", "13900139000")
		if got != [2]int{10, 5} {
			t.Fatalf("failed update replaced active policy: %v", got)
		}
	}
}

func TestIdentifierPrefixLongestMatchAndIndependentGroups(t *testing.T) {
	original := PhoneRateLimitPolicies2JSONString()
	defer func() { _ = UpdatePhoneRateLimitPoliciesByJSONString(original) }()
	raw := `{"group1":{"default":[10,5],"special":{"13701010":[100,50],"13701010202":[20,10],"tenant001":[0,0]}},"group2":{"default":[4,2],"special":{"13701010":[3,1]}}}`
	if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		group, identifier, scope string
		limits                   [2]int
		active                   bool
	}{
		{"group1", "13701010202|appid|ip", "prefix:13701010202", [2]int{20, 10}, true},
		{"group1", "13701010999|other", "prefix:13701010", [2]int{100, 50}, true},
		{"group1", "tenant001|app1", "prefix:tenant001", [2]int{}, true},
		{"group1", "short", "identifier:short", [2]int{10, 5}, true},
		{"group1", "alice|app1", "identifier:alice|app1", [2]int{10, 5}, true},
		{"group1", "alice|app2", "identifier:alice|app2", [2]int{10, 5}, true},
		{"group2", "13701010202|appid|ip", "prefix:13701010", [2]int{3, 1}, true},
		{"group2", "short", "identifier:short", [2]int{4, 2}, true},
	}
	for _, tc := range cases {
		got, configured, active, scope := ResolveUserIdentifierRateLimit(tc.group, tc.identifier)
		if got != tc.limits || !configured || active != tc.active || scope != tc.scope {
			t.Errorf("%s/%s: got limits=%v configured=%v active=%v scope=%q", tc.group, tc.identifier, got, configured, active, scope)
		}
	}
}

func TestIdentifierPrefixValidationAndRuntimeDefense(t *testing.T) {
	original := PhoneRateLimitPolicies2JSONString()
	defer func() { _ = UpdatePhoneRateLimitPoliciesByJSONString(original) }()
	if !ValidUserIdentifierPrefix("abcdefgh") || ValidUserIdentifierPrefix("abcdefg") || !ValidUserIdentifierPrefix("企业用户甲乙丙丁") {
		t.Fatal("prefix length must be at least eight Unicode characters")
	}
	if err := UpdatePhoneRateLimitPoliciesByJSONString(`{"group1":{"default":[10,5]}}`); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"group1":{"special":{"1234567":[50,20]}}}`,
		`{"group1":{"special":{"短前缀":[50,20]}}}`,
		`{"group1":{"special":{"abcdefgh":[-1,2]}}}`,
	} {
		if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err == nil {
			t.Errorf("accepted invalid config: %s", raw)
		}
		got, _, _, _ := ResolveUserIdentifierRateLimit("group1", "short")
		if got != [2]int{10, 5} {
			t.Fatalf("invalid config replaced active rules: %v", got)
		}
	}
	phonePolicyStore.Lock()
	phonePolicyStore.data = PhoneRateLimitPolicies{"group1": {Special: map[string][2]int{"1234567": [2]int{1, 1}}}}
	phonePolicyStore.Unlock()
	got, configured, active, scope := ResolveUserIdentifierRateLimit("group1", "123456789")
	if !configured || active || got != [2]int{} || scope != "identifier:123456789" {
		t.Fatalf("invalid in-memory rule became active: %v %v %v %s", got, configured, active, scope)
	}
}
