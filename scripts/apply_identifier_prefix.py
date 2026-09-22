"""One-time fail-fast source transformation; removed after verified commit."""
from pathlib import Path


def edit(name, old, new):
    path = Path(name)
    text = path.read_text(encoding='utf-8')
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f'{name}: expected exactly one match, found {count}: {old[:100]!r}')
    path.write_text(text.replace(old, new, 1), encoding='utf-8')


setting = 'setting/phone_rate_limit.go'
edit(setting, '\t"regexp"\n', '\t"unicode/utf8"\n')
edit(setting, 'var phoneNumberPattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)\n\nfunc ValidPhoneNumber(phone string) bool { return phoneNumberPattern.MatchString(phone) }', '''// Special rules use literal prefixes (not regexes or phone-number parsing).
// The minimum is eight Unicode characters, independent of UTF-8 byte length.
func ValidUserIdentifierPrefix(prefix string) bool { return utf8.RuneCountInString(prefix) >= 8 }''')
edit(setting, '手机号策略不能为空', '用户标识策略不能为空')
edit(setting, '手机号策略必须为 JSON 对象', '用户标识策略必须为 JSON 对象')
edit(setting, '''\t\tfor phone, pairRaw := range entry.Special {
\t\t\tif !ValidPhoneNumber(phone) {
\t\t\t\treturn nil, fmt.Errorf("%s: 特殊手机号格式无效", group)
\t\t\t}
\t\t\tpair, err := phonePair(pairRaw, group+"/special")
\t\t\tif err != nil {
\t\t\t\treturn nil, err
\t\t\t}
\t\t\tpolicy.Special[phone] = pair
\t\t}''', '''\t\tfor prefix, pairRaw := range entry.Special {
\t\t\tif !ValidUserIdentifierPrefix(prefix) {
\t\t\t\treturn nil, fmt.Errorf("%s: 特殊用户标识前缀至少需要 8 个字符", group)
\t\t\t}
\t\t\tpair, err := phonePair(pairRaw, group+"/special")
\t\t\tif err != nil {
\t\t\t\treturn nil, err
\t\t\t}
\t\t\tpolicy.Special[prefix] = pair
\t\t}''')
p = Path(setting)
text = p.read_text(encoding='utf-8')
mark = '// ResolvePhoneRateLimit: a configured group suppresses the legacy global phone rule,'
assert text.count(mark) == 1
text = text[:text.index(mark)] + '''// ResolveUserIdentifierRateLimit returns the limit and a deterministic counter identity.
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
'''
p.write_text(text, encoding='utf-8')

model_mw = 'middleware/model-rate-limit.go'
p = Path(model_mw)
text = p.read_text(encoding='utf-8')
start = text.index('\t\t// New group-specific phone policies override only legacy phone policy,')
end = text.index('\t\t// Old handler phone branches are superseded by withPhoneRateLimit.', start)
text = text[:start] + '''\t\t// Group-specific identifier policies override legacy top-level phone rules.
\t\t// Do not split the application identifier or impose a phone-number format.
\t\tidentifier := strings.TrimSpace(requestUser)
\t\tidentifierLimits, identifierConfigured, identifierActive, counterIdentity := setting.ResolveUserIdentifierRateLimit(group, identifier)
\t\tidentifierGroup := group
\t\tif identifierConfigured {
\t\t\tif identifierActive && identifier == "" {
\t\t\t\tabortWithOpenAiMessage(c, http.StatusBadRequest, "当前令牌分组要求提供用户标识")
\t\t\t\treturn
\t\t\t}
\t\t} else {
\t\t\t// Preserve old top-level number rules only when no new group policy exists.
\t\t\tlegacyID := subNumber(identifier)
\t\t\tif legacyID != "" {
\t\t\t\tif total, success, exists := setting.GetGroupRateLimit(legacyID); exists {
\t\t\t\t\tidentifierGroup = "legacy"
\t\t\t\t\tcounterIdentity = "identifier:" + legacyID
\t\t\t\t\tidentifierLimits = [2]int{total, success}
\t\t\t\t}
\t\t\t}
\t\t}
''' + text[end:]
old_call = 'withPhoneRateLimit(c, phoneScope, phoneID, phoneLimits, duration, next)'
assert text.count(old_call) == 1
text = text.replace(old_call, 'withPhoneRateLimit(c, identifierGroup, counterIdentity, identifierLimits, duration, next)', 1)
p.write_text(text, encoding='utf-8')

phone_mw = 'middleware/phone_rate_limit.go'
edit(phone_mw, 'func withPhoneRateLimit(c *gin.Context, group, phone string, limits [2]int, duration int64, next gin.HandlerFunc) {', 'func withPhoneRateLimit(c *gin.Context, group, counterIdentity string, limits [2]int, duration int64, next gin.HandlerFunc) {')
edit(phone_mw, 'if limits == [2]int{} || phone == "" {', 'if limits == [2]int{} || counterIdentity == "" {')
edit(phone_mw, 'common.GenerateHMAC("phone-rate-limit-v2\\x00" + group + "\\x00" + phone)', 'identifierCounterScope(group, counterIdentity)')
p = Path(phone_mw)
text = p.read_text(encoding='utf-8')
needle = 'func withPhoneRateLimit(c *gin.Context, group, counterIdentity string, limits [2]int, duration int64, next gin.HandlerFunc) {'
assert text.count(needle) == 1
text = text.replace(needle, '''// identifierCounterScope hashes both the token group and the selected scope.
// "prefix:" and "identifier:" prevent a full ID from sharing a prefix bucket.
func identifierCounterScope(group, counterIdentity string) string {
    return common.GenerateHMAC("user-identifier-rate-limit-v3\\x00" + group + "\\x00" + counterIdentity)
}

''' + needle, 1)
text = text.replace('rateLimit:phone:v2:', 'rateLimit:identifier:v3:')
text = text.replace('手机号已达到当前令牌分组的限流上限', '用户标识已达到当前令牌分组的限流上限')
text = text.replace('No clear-text phone numbers in Redis or in-memory map keys.', 'No clear-text user identifiers in Redis or in-memory map keys.')
p.write_text(text, encoding='utf-8')

controller = 'controller/phone_rate_limit.go'
p = Path(controller)
text = p.read_text(encoding='utf-8').replace('手机号限流策略必须是 JSON 对象', '用户标识限流策略必须是 JSON 对象')
p.write_text(text, encoding='utf-8')

ui = 'web/classic/src/pages/RateLimitManagement/PhoneRateLimit.jsx'
p = Path(ui)
text = p.read_text(encoding='utf-8')
assert text.count("const isPhone = (phone) => /^1[3-9][0-9]{9}$/.test(phone);") == 1
text = text.replace("const isPhone = (phone) => /^1[3-9][0-9]{9}$/.test(phone);", "import { isValidIdentifierPrefix } from './userIdentifierPolicy';")
text = text.replace("phone: ''", "prefix: ''")
text = text.replace('([phone, limits]) => ({ group, phone, limits })', '([prefix, limits]) => ({ group, prefix, limits })')
text = text.replace('row.phone', 'row.prefix')
text = text.replace('form.phone', 'form.prefix')
text = text.replace('phone: row.prefix', 'prefix: row.prefix')
text = text.replace("!isPhone(form.prefix)", "!isValidIdentifierPrefix(form.prefix)")
text = text.replace('请选择分组并填写有效的 11 位手机号', '请选择分组并填写至少 8 个字符的用户标识前缀')
text = text.replace('special[form.prefix]', 'special[form.prefix]')
text = text.replace('phone:v.trim()', 'prefix:v')
text = text.replace('phone: v.trim()', 'prefix: v')
text = text.replace('maxLength={11} ', '')
text = text.replace("dataIndex:'phone'", "dataIndex:'prefix'")
text = text.replace('手机号', '用户标识')
text = text.replace('特殊号码', '特殊前缀')
text = text.replace('特殊用户标识', '特殊用户标识前缀')
text = text.replace('前缀前缀', '前缀')
text = text.replace('每个用户标识独立计数', '通用规则按完整用户标识独立计数，特殊规则按匹配前缀共享计数')
text = text.replace('同一个用户标识在不同令牌分组分别计数', '不同令牌分组的计数互不影响')
text = text.replace("const [form, setForm] = useState({ group: '', prefix: '', total: 0, success: 0 });", "const [form, setForm] = useState({ group: '', prefix: '', total: 0, success: 0 });")
assert 'isPhone(' not in text and 'form.phone' not in text and 'row.phone' not in text
p.write_text(text, encoding='utf-8')

index = 'web/classic/src/pages/RateLimitManagement/index.jsx'
p = Path(index)
text = p.read_text(encoding='utf-8')
assert "tab='手机号限流管理'" in text
text = text.replace("tab='手机号限流管理'", "tab='用户标识限流管理'")
p.write_text(text, encoding='utf-8')

Path('web/classic/src/pages/RateLimitManagement/userIdentifierPolicy.js').write_text('''// Count Unicode characters, not UTF-8 bytes or a fixed phone-number format.
export const isValidIdentifierPrefix = (prefix) =>
  typeof prefix === 'string' && Array.from(prefix).length >= 8;
''', encoding='utf-8')
Path('web/classic/src/pages/RateLimitManagement/userIdentifierPolicy.test.js').write_text('''import { describe, expect, test } from 'bun:test';
import { isValidIdentifierPrefix } from './userIdentifierPolicy';

describe('special user identifier prefix validation', () => {
  test('rejects seven characters and accepts eight', () => {
    expect(isValidIdentifierPrefix('1234567')).toBe(false);
    expect(isValidIdentifierPrefix('12345678')).toBe(true);
  });
  test('accepts arbitrary identifiers and counts Unicode characters', () => {
    expect(isValidIdentifierPrefix('企业用户甲乙丙丁')).toBe(true);
    expect(isValidIdentifierPrefix('abcdefg|app')).toBe(true);
    expect(isValidIdentifierPrefix('')).toBe(false);
  });
});
''', encoding='utf-8')

setting_tests = 'setting/phone_rate_limit_test.go'
p = Path(setting_tests)
text = p.read_text(encoding='utf-8')
text += '''
func TestIdentifierPrefixLongestMatchAndIndependentGroups(t *testing.T) {
    original := PhoneRateLimitPolicies2JSONString()
    defer func() { _ = UpdatePhoneRateLimitPoliciesByJSONString(original) }()
    raw := `{"group1":{"default":[10,5],"special":{"13701010":[100,50],"13701010202":[20,10],"tenant001":[0,0]}},"group2":{"default":[4,2],"special":{"13701010":[3,1]}}}`
    if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err != nil { t.Fatal(err) }
    cases := []struct {
        group, identifier, scope string
        limits [2]int
        active bool
    }{
        {"group1", "13701010202|appid|ip", "prefix:13701010202", [2]int{20,10}, true},
        {"group1", "13701010999|other", "prefix:13701010", [2]int{100,50}, true},
        {"group1", "tenant001|app1", "prefix:tenant001", [2]int{}, true},
        {"group1", "short", "identifier:short", [2]int{10,5}, true},
        {"group1", "alice|app1", "identifier:alice|app1", [2]int{10,5}, true},
        {"group1", "alice|app2", "identifier:alice|app2", [2]int{10,5}, true},
        {"group2", "13701010202|appid|ip", "prefix:13701010", [2]int{3,1}, true},
        {"group2", "short", "identifier:short", [2]int{4,2}, true},
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
    if err := UpdatePhoneRateLimitPoliciesByJSONString(`{"group1":{"default":[10,5]}}`); err != nil { t.Fatal(err) }
    for _, raw := range []string{
        `{"group1":{"special":{"1234567":[50,20]}}}`,
        `{"group1":{"special":{"短前缀":[50,20]}}}`,
        `{"group1":{"special":{"abcdefgh":[-1,2]}}}`,
    } {
        if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err == nil { t.Errorf("accepted invalid config: %s", raw) }
        got, _, _, _ := ResolveUserIdentifierRateLimit("group1", "short")
        if got != [2]int{10,5} { t.Fatalf("invalid config replaced active rules: %v", got) }
    }
    phonePolicyStore.Lock()
    phonePolicyStore.data = PhoneRateLimitPolicies{"group1":{Special: map[string][2]int{"1234567":[1,1]}}}
    phonePolicyStore.Unlock()
    got, configured, active, scope := ResolveUserIdentifierRateLimit("group1", "123456789")
    if !configured || active || got != [2]int{} || scope != "identifier:123456789" {
        t.Fatalf("invalid in-memory rule became active: %v %v %v %s", got, configured, active, scope)
    }
}
'''
p.write_text(text, encoding='utf-8')

middleware_tests = 'middleware/phone_rate_limit_test.go'
p = Path(middleware_tests)
text = p.read_text(encoding='utf-8')
text += '''
func TestUserIdentifierCounterScopeIsolation(t *testing.T) {
    shared := identifierCounterScope("group1", "prefix:13701010")
    if shared != identifierCounterScope("group1", "prefix:13701010") { t.Fatal("same prefix must share counter") }
    if shared == identifierCounterScope("group2", "prefix:13701010") { t.Fatal("group counters collided") }
    if shared == identifierCounterScope("group1", "identifier:13701010") { t.Fatal("default and special counters collided") }
    if identifierCounterScope("group1", "identifier:alice|app1") == identifierCounterScope("group1", "identifier:alice|app2") { t.Fatal("distinct full identifiers collided") }
    if identifierCounterScope("group1", "identifier:13701010202|appid|ip") == shared { t.Fatal("full identifier unexpectedly shared prefix counter") }
}
'''
p.write_text(text, encoding='utf-8')

Path('docs/phone-rate-limit.md').write_text('''# 分组用户标识限流（2.1）

模型限流继续使用 `ModelRequestRateLimitGroup`。用户标识策略独立存放于历史兼容配置键 `PhoneRateLimitPolicies`，**配置键保留旧名称，不再限定手机号格式**，无需新增数据表。

```json
{
  "group1": {"default": [10, 5], "special": {"13701010": [100, 50], "13701010202": [20, 10], "tenant001": [0, 0]}},
  "group2": {"default": [20, 10], "special": {"13701010": [5, 2]}}
}
```

- `user` 请求体字段优先于 `X-User-Id` 请求头；标识允许任意字符串，不拆分 `|`、不截取手机号、不限制为数字。建议业务系统传递稳定、可信的用户标识；任意更换标识可绕过通用限流。
- **通用规则**：每个分组中按**完整用户标识**单独计数，因此 `alice|app1` 和 `alice|app2` 默认使用不同额度。
- **特殊规则**：以字面前缀匹配（不是正则），前缀至少 **8 个 Unicode 字符**，多个匹配时**最长前缀优先**；同组内命中同一前缀的所有完整标识**共享**该特殊额度。建议选择足以区分真实用户的前缀，否则多个用户可能共享额度。
- 特殊规则覆盖本分组通用规则；`[0,0]` 取消该层限制，不绕过现有组/模型限流。只有分组完全不存在于新配置时才回退旧 `ModelRequestRateLimitGroup` 顶层号码规则；原有模型限流 JSON 格式保持不变。
- 保存时拒绝长度小于 8 字符的特殊前缀；运行时也忽略异常遗留的短前缀。完整用户标识本身可短于 8 字符，此时仅适用通用规则。对启用了有效用户标识限流的分组，缺失标识则拒绝请求。
- Redis 采用固定时间窗口和 Lua 原子准入计数；请求次数包含失败，完成次数预留在失败时退还。内存计数只能服务单进程；多节点生产环境必须共享 Redis。
- Redis/内存 Key 使用令牌分组及匹配标识的 HMAC，不保存明文。新计数版本 `identifier:v3` 与旧手机号 `phone:v2` 不互通，升级后周期内计数从零开始；保持各节点相同 `CRYPTO_SECRET`。
- 流式 HTTP 头已写出 2xx 后发生流内错误无法可靠识别，完成计数仍遵循原系统 HTTP 状态码口径。全局开关 `ModelRequestRateLimitEnabled` 与全局周期 `ModelRequestRateLimitDurationMinutes` 沿用原值。
''', encoding='utf-8')
print('identifier prefix source transformation completed')
