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
