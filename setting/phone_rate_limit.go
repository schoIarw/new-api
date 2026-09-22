package setting

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
)

const PhoneRateLimitPoliciesOptionKey = "PhoneRateLimitPolicies"

var phoneNumberPattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

func ValidPhoneNumber(phone string) bool { return phoneNumberPattern.MatchString(phone) }

type PhoneRateLimitGroup struct {
	Default *[2]int           `json:"default,omitempty"`
	Special map[string][2]int `json:"special,omitempty"`
}

type PhoneRateLimitPolicies map[string]PhoneRateLimitGroup

var phonePolicyStore = struct {
	sync.RWMutex
	data PhoneRateLimitPolicies
}{data: PhoneRateLimitPolicies{}}

func phonePair(raw json.RawMessage, scope string) ([2]int, error) {
	var result [2]int
	var values []int
	if err := json.Unmarshal(raw, &values); err != nil {
		return result, fmt.Errorf("%s: %w", scope, err)
	}
	if len(values) != 2 {
		return result, fmt.Errorf("%s: 限流配置必须恰好包含两个整数", scope)
	}
	for _, v := range values {
		if v < 0 || v > math.MaxInt32 {
			return result, fmt.Errorf("%s: 限流值必须在 0 至 2147483647 之间", scope)
		}
	}
	return [2]int{values[0], values[1]}, nil
}

// ParsePhoneRateLimitPolicies validates before any live configuration is changed.
func ParsePhoneRateLimitPolicies(value string) (PhoneRateLimitPolicies, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("手机号策略不能为空")
	}
	var raw map[string]struct {
		Default json.RawMessage            `json:"default"`
		Special map[string]json.RawMessage `json:"special"`
	}
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("手机号策略必须为 JSON 对象")
	}
	result := make(PhoneRateLimitPolicies, len(raw))
	for group, entry := range raw {
		if strings.TrimSpace(group) != group || group == "" {
			return nil, fmt.Errorf("无效的令牌分组名称: %q", group)
		}
		policy := PhoneRateLimitGroup{Special: map[string][2]int{}}
		if len(entry.Default) > 0 {
			pair, err := phonePair(entry.Default, group+"/default")
			if err != nil {
				return nil, err
			}
			policy.Default = &pair
		}
		for phone, pairRaw := range entry.Special {
			if !ValidPhoneNumber(phone) {
				return nil, fmt.Errorf("%s: 特殊手机号格式无效", group)
			}
			pair, err := phonePair(pairRaw, group+"/special")
			if err != nil {
				return nil, err
			}
			policy.Special[phone] = pair
		}
		result[group] = policy
	}
	return result, nil
}

func UpdatePhoneRateLimitPoliciesByJSONString(value string) error {
	parsed, err := ParsePhoneRateLimitPolicies(value)
	if err != nil {
		return err
	}
	phonePolicyStore.Lock()
	phonePolicyStore.data = parsed
	phonePolicyStore.Unlock()
	return nil
}

func PhoneRateLimitPolicies2JSONString() string {
	phonePolicyStore.RLock()
	defer phonePolicyStore.RUnlock()
	encoded, err := json.Marshal(phonePolicyStore.data)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// ResolvePhoneRateLimit: a configured group suppresses the legacy global phone rule,
// even when its default is absent or [0,0]. A special [0,0] overrides its default.
func ResolvePhoneRateLimit(group, phone string) (limits [2]int, configured, active bool) {
	phonePolicyStore.RLock()
	defer phonePolicyStore.RUnlock()
	policy, found := phonePolicyStore.data[group]
	if !found {
		return limits, false, false
	}
	if policy.Default != nil {
		limits = *policy.Default
		active = limits != [2]int{}
	}
	for _, pair := range policy.Special {
		if pair != [2]int{} {
			active = true
			break
		}
	}
	if pair, exists := policy.Special[phone]; exists {
		limits = pair
	}
	return limits, true, active
}
