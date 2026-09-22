package setting

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode/utf8"
)

const PhoneRateLimitPoliciesOptionKey = "PhoneRateLimitPolicies"

// Special rules use literal prefixes (not regexes or phone-number parsing).
// The minimum is eight Unicode characters, independent of UTF-8 byte length.
func ValidUserIdentifierPrefix(prefix string) bool { return utf8.RuneCountInString(prefix) >= 8 }

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
		return nil, fmt.Errorf("用户标识策略不能为空")
	}
	var raw map[string]struct {
		Default json.RawMessage            `json:"default"`
		Special map[string]json.RawMessage `json:"special"`
	}
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("用户标识策略必须为 JSON 对象")
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
		for prefix, pairRaw := range entry.Special {
			if !ValidUserIdentifierPrefix(prefix) {
				return nil, fmt.Errorf("%s: 特殊用户标识前缀至少需要 8 个字符", group)
			}
			pair, err := phonePair(pairRaw, group+"/special")
			if err != nil {
				return nil, err
			}
			policy.Special[prefix] = pair
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

// ResolveUserIdentifierRateLimit returns the limit and a deterministic counter identity.
// A configured group suppresses legacy top-level rules. Valid special prefixes
// override the group default; the longest matching prefix wins. [0,0] explicitly
// disables this layer. Default counters use the complete identifier, while all
// identifiers matching the same special prefix share its counter.
func ResolveUserIdentifierRateLimit(group, identifier string) (limits [2]int, configured, active bool, counterIdentity string) {
	counterIdentity = "identifier:" + identifier
	phonePolicyStore.RLock()
	defer phonePolicyStore.RUnlock()
	policy, found := phonePolicyStore.data[group]
	if !found {
		return limits, false, false, counterIdentity
	}
	if policy.Default != nil {
		limits = *policy.Default
		active = limits != [2]int{}
	}
	bestPrefix := ""
	var bestLimit [2]int
	for prefix, pair := range policy.Special {
		// Defense in depth: old or injected prefixes shorter than eight
		// characters must never become active or match an identifier.
		if !ValidUserIdentifierPrefix(prefix) {
			continue
		}
		if pair != [2]int{} {
			active = true
		}
		if strings.HasPrefix(identifier, prefix) && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
			bestLimit = pair
		}
	}
	if bestPrefix != "" {
		limits = bestLimit
		counterIdentity = "prefix:" + bestPrefix
	}
	return limits, true, active, counterIdentity
}

// ResolvePhoneRateLimit is retained for existing callers. New gateway traffic
// must use ResolveUserIdentifierRateLimit to obtain the matching counter scope.
func ResolvePhoneRateLimit(group, identifier string) (limits [2]int, configured, active bool) {
	limits, configured, active, _ = ResolveUserIdentifierRateLimit(group, identifier)
	return limits, configured, active
}
