"""One-time, fail-fast source transformation on a clean feature branch.

The workflow checks both builds/tests before publishing the transformed sources.
"""
from pathlib import Path
import re


def replace(path, old, new, expected=1):
    p = Path(path)
    source = p.read_text()
    count = source.count(old)
    if count != expected:
        raise RuntimeError(f"{path}: expected {expected} instances, got {count}: {old[:90]!r}")
    p.write_text(source.replace(old, new))


def add(path, content):
    p = Path(path)
    if p.exists():
        raise RuntimeError(f"Refusing to overwrite {path}")
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content)


add('setting/phone_rate_limit.go', r'''package setting

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
    Default *[2]int `json:"default,omitempty"`
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
    if err := json.Unmarshal(raw, &values); err != nil { return result, fmt.Errorf("%s: %w", scope, err) }
    if len(values) != 2 { return result, fmt.Errorf("%s: 限流配置必须恰好包含两个整数", scope) }
    for _, v := range values {
        if v < 0 || v > math.MaxInt32 { return result, fmt.Errorf("%s: 限流值必须在 0 至 2147483647 之间", scope) }
    }
    return [2]int{values[0], values[1]}, nil
}

// ParsePhoneRateLimitPolicies validates before any live configuration is changed.
func ParsePhoneRateLimitPolicies(value string) (PhoneRateLimitPolicies, error) {
    if strings.TrimSpace(value) == "" { return nil, fmt.Errorf("手机号策略不能为空") }
    var raw map[string]struct {
        Default json.RawMessage `json:"default"`
        Special map[string]json.RawMessage `json:"special"`
    }
    if err := json.Unmarshal([]byte(value), &raw); err != nil { return nil, err }
    if raw == nil { return nil, fmt.Errorf("手机号策略必须为 JSON 对象") }
    result := make(PhoneRateLimitPolicies, len(raw))
    for group, entry := range raw {
        if strings.TrimSpace(group) != group || group == "" { return nil, fmt.Errorf("无效的令牌分组名称: %q", group) }
        policy := PhoneRateLimitGroup{Special: map[string][2]int{}}
        if len(entry.Default) > 0 {
            pair, err := phonePair(entry.Default, group+"/default")
            if err != nil { return nil, err }
            policy.Default = &pair
        }
        for phone, pairRaw := range entry.Special {
            if !ValidPhoneNumber(phone) { return nil, fmt.Errorf("%s: 特殊手机号格式无效", group) }
            pair, err := phonePair(pairRaw, group+"/special")
            if err != nil { return nil, err }
            policy.Special[phone] = pair
        }
        result[group] = policy
    }
    return result, nil
}

func UpdatePhoneRateLimitPoliciesByJSONString(value string) error {
    parsed, err := ParsePhoneRateLimitPolicies(value)
    if err != nil { return err }
    phonePolicyStore.Lock()
    phonePolicyStore.data = parsed
    phonePolicyStore.Unlock()
    return nil
}

func PhoneRateLimitPolicies2JSONString() string {
    phonePolicyStore.RLock()
    defer phonePolicyStore.RUnlock()
    encoded, err := json.Marshal(phonePolicyStore.data)
    if err != nil { return "{}" }
    return string(encoded)
}

// ResolvePhoneRateLimit: a configured group suppresses the legacy global phone rule,
// even when its default is absent or [0,0]. A special [0,0] overrides its default.
func ResolvePhoneRateLimit(group, phone string) (limits [2]int, configured, active bool) {
    phonePolicyStore.RLock()
    defer phonePolicyStore.RUnlock()
    policy, found := phonePolicyStore.data[group]
    if !found { return limits, false, false }
    if policy.Default != nil {
        limits = *policy.Default
        active = limits != [2]int{}
    }
    for _, pair := range policy.Special {
        if pair != [2]int{} { active = true; break }
    }
    if pair, exists := policy.Special[phone]; exists { limits = pair }
    return limits, true, active
}
''')

add('middleware/phone_rate_limit.go', r'''package middleware

import (
    "context"
    "fmt"
    "net/http"
    "sync"
    "time"

    "github.com/QuantumNous/new-api/common"
    "github.com/QuantumNous/new-api/setting"
    "github.com/gin-gonic/gin"
    "github.com/go-redis/redis/v8"
)

// Admission reserves a completion slot until the request finishes; failed
// requests free that slot but still consume a request slot. Shared Redis
// reservations are atomic across gateway replicas.
var phoneAcquireScript = redis.NewScript(`
local maxTotal = tonumber(ARGV[1]); local maxSuccess = tonumber(ARGV[2]); local ttl = tonumber(ARGV[3]);
if maxTotal > 0 and tonumber(redis.call('GET', KEYS[1]) or '0') >= maxTotal then return 1 end
if maxSuccess > 0 and tonumber(redis.call('GET', KEYS[2]) or '0') >= maxSuccess then return 2 end
if maxTotal > 0 then
    if redis.call('INCR', KEYS[1]) == 1 then redis.call('EXPIRE', KEYS[1], ttl) end
end
if maxSuccess > 0 then
    if redis.call('INCR', KEYS[2]) == 1 then redis.call('EXPIRE', KEYS[2], ttl) end
end
return 0
`)
var phoneReleaseScript = redis.NewScript(`
if tonumber(redis.call('GET', KEYS[1]) or '0') > 0 then redis.call('DECR', KEYS[1]) end
return 1
`)

type phoneCounter struct { window int64; total int; reserved int }
var phoneMemory = struct {
    sync.Mutex
    counters map[string]phoneCounter
    operations uint64
}{counters: make(map[string]phoneCounter)}

func acquirePhoneMemory(scope string, window int64, pair [2]int) int {
    phoneMemory.Lock()
    defer phoneMemory.Unlock()
    counter := phoneMemory.counters[scope]
    if counter.window != window { counter = phoneCounter{window: window} }
    if pair[0] > 0 && counter.total >= pair[0] { return 1 }
    if pair[1] > 0 && counter.reserved >= pair[1] { return 2 }
    if pair[0] > 0 { counter.total++ }
    if pair[1] > 0 { counter.reserved++ }
    phoneMemory.counters[scope] = counter
    phoneMemory.operations++
    if len(phoneMemory.counters) > 10000 && phoneMemory.operations%1024 == 0 {
        for key, value := range phoneMemory.counters {
            if value.window < window-2 { delete(phoneMemory.counters, key) }
        }
    }
    return 0
}

func releasePhoneMemory(scope string, window int64) {
    phoneMemory.Lock()
    defer phoneMemory.Unlock()
    counter, ok := phoneMemory.counters[scope]
    if ok && counter.window == window && counter.reserved > 0 {
        counter.reserved--
        phoneMemory.counters[scope] = counter
    }
}

func withPhoneRateLimit(c *gin.Context, group, phone string, limits [2]int, duration int64, next gin.HandlerFunc) {
    if limits == [2]int{} || phone == "" { next(c); return }
    if duration <= 0 {
        abortWithOpenAiMessage(c, http.StatusInternalServerError, "invalid_phone_limit_duration")
        return
    }
    // No clear-text phone numbers in Redis or in-memory map keys.
    scope := common.GenerateHMAC("phone-rate-limit-v2\x00" + group + "\x00" + phone)
    window := time.Now().Unix() / duration
    var verdict int
    var release func()
    if common.RedisEnabled {
        totalKey := fmt.Sprintf("rateLimit:phone:v2:%s:%d:request", scope, window)
        successKey := fmt.Sprintf("rateLimit:phone:v2:%s:%d:success", scope, window)
        ctx := c.Request.Context()
        value, err := phoneAcquireScript.Run(ctx, common.RDB, []string{totalKey, successKey}, limits[0], limits[1], duration*2).Int()
        if err != nil {
            abortWithOpenAiMessage(c, http.StatusInternalServerError, "phone_rate_limit_check_failed")
            return
        }
        verdict = value
        release = func() {
            if limits[1] > 0 {
                // Use an independent context: request cancellation must not strand reservations.
                cleanCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
                defer cancel()
                if err := phoneReleaseScript.Run(cleanCtx, common.RDB, []string{successKey}).Err(); err != nil {
                    common.SysLog("failed to release phone completion reservation: " + err.Error())
                }
            }
        }
    } else {
        verdict = acquirePhoneMemory(scope, window, limits)
        release = func() { if limits[1] > 0 { releasePhoneMemory(scope, window) } }
    }
    if verdict != 0 {
        abortWithOpenAiMessage(c, http.StatusTooManyRequests, "手机号已达到当前令牌分组的限流上限")
        return
    }
    // Defer also runs after a panic, but a 2xx streaming header does not prove
    // the upstream response was semantically successful; match existing status semantics.
    defer func() {
        if c.Writer.Status() >= http.StatusBadRequest { release() }
    }()
    next(c)
}
''')

add('controller/phone_rate_limit.go', r'''package controller

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/QuantumNous/new-api/common"
    "github.com/QuantumNous/new-api/model"
    "github.com/QuantumNous/new-api/setting"
    "github.com/QuantumNous/new-api/setting/ratio_setting"
    "github.com/gin-gonic/gin"
)

func GetPhoneRateLimitPolicies(c *gin.Context) {
    var policies setting.PhoneRateLimitPolicies
    if err := json.Unmarshal([]byte(setting.PhoneRateLimitPolicies2JSONString()), &policies); err != nil {
        common.ApiError(c, err); return
    }
    c.JSON(http.StatusOK, gin.H{"success": true, "data": policies})
}

func UpdatePhoneRateLimitPolicies(c *gin.Context) {
    var request struct { Policies json.RawMessage `json:"policies"` }
    if err := c.ShouldBindJSON(&request); err != nil || len(request.Policies) == 0 {
        common.ApiErrorMsg(c, "手机号限流策略必须是 JSON 对象"); return
    }
    policies, err := setting.ParsePhoneRateLimitPolicies(string(request.Policies))
    if err != nil { common.ApiError(c, err); return }
    groups := ratio_setting.GetGroupRatioCopy()
    for group := range policies {
        if group != "auto" {
            if _, exists := groups[group]; !exists {
                common.ApiErrorMsg(c, "不存在的令牌分组: "+group); return
            }
        }
        if strings.TrimSpace(group) == "" { common.ApiErrorMsg(c, "令牌分组不能为空"); return }
    }
    encoded, err := json.Marshal(policies)
    if err != nil { common.ApiError(c, err); return }
    if err = model.UpdateOptionsBulk(map[string]string{
        setting.PhoneRateLimitPoliciesOptionKey: string(encoded),
    }); err != nil { common.ApiError(c, err); return }
    common.ApiSuccess(c, nil)
}
''')

add('setting/phone_rate_limit_test.go', r'''package setting

import "testing"

func TestPhoneRateLimitPolicyResolution(t *testing.T) {
    original := PhoneRateLimitPolicies2JSONString()
    defer func() { if err := UpdatePhoneRateLimitPoliciesByJSONString(original); err != nil { t.Fatal(err) } }()
    raw := `{"group1":{"default":[10,5],"special":{"18946512326":[100,50],"13800138000":[0,0]}},"group2":{"default":[20,10],"special":{"18946512326":[5,2]}}}`
    if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err != nil { t.Fatal(err) }
    cases := []struct { group, phone string; expected [2]int; configured, active bool }{
        {"group1", "18946512326", [2]int{100,50}, true, true},
        {"group1", "13800138000", [2]int{}, true, true},
        {"group1", "13900139000", [2]int{10,5}, true, true},
        {"group2", "18946512326", [2]int{5,2}, true, true},
        {"group2", "13900139000", [2]int{20,10}, true, true},
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
    if err := UpdatePhoneRateLimitPoliciesByJSONString(`{"group1":{"default":[10,5]}}`); err != nil { t.Fatal(err) }
    for _, raw := range []string{
        `null`, `{"group1":{"default":[10]}}`, `{"group1":{"default":[-1,5]}}`,
        `{"group1":{"default":[2147483648,5]}}`, `{"group1":{"special":{"123":[5,2]}}}`,
    } {
        if err := UpdatePhoneRateLimitPoliciesByJSONString(raw); err == nil { t.Errorf("accepted %s",raw) }
        got, _, _ := ResolvePhoneRateLimit("group1", "13900139000")
        if got != [2]int{10,5} { t.Fatalf("failed update replaced active policy: %v",got) }
    }
}
''')

add('middleware/phone_rate_limit_test.go', r'''package middleware

import (
    "sync"
    "sync/atomic"
    "testing"
)

func TestPhoneMemoryIndependentScopeAndReservation(t *testing.T) {
    const workers = 50
    var wg sync.WaitGroup
    var granted atomic.Int32
    for i:=0;i<workers;i++ {
        wg.Add(1)
        go func(){ defer wg.Done(); if acquirePhoneMemory("phone-group1-unique", 123456, [2]int{10,5}) == 0 { granted.Add(1) } }()
    }
    wg.Wait()
    if got:=granted.Load(); got!=5 { t.Fatalf("granted %d, want 5 reserved completion slots",got) }
    if verdict:=acquirePhoneMemory("phone-group2-unique", 123456, [2]int{10,5}); verdict!=0 {
        t.Fatalf("a different token group shared counters: %d",verdict)
    }
    releasePhoneMemory("phone-group1-unique",123456)
    if verdict:=acquirePhoneMemory("phone-group1-unique",123456,[2]int{10,5}); verdict!=0 {
        t.Fatalf("failed request must free completion slot: %d",verdict)
    }
    if verdict:=acquirePhoneMemory("phone-group1-unique",123457,[2]int{10,5}); verdict!=0 {
        t.Fatalf("new window must reset counters: %d",verdict)
    }
}

func TestPhoneMemoryRequestOnly(t *testing.T) {
    for i:=0;i<3;i++ { if got:=acquirePhoneMemory("phone-request-only", 923456, [2]int{3,0}); got!=0 { t.Fatal(got) } }
    if got:=acquirePhoneMemory("phone-request-only",923456,[2]int{3,0}); got!=1 { t.Fatalf("request-only limit was bypassed: %d",got) }
}
''')

# Keep the existing model policy untouched; add a separate, hot-reloaded option.
replace('model/option.go', '\tcommon.OptionMap["ModelRequestRateLimitGroup"] = setting.ModelRequestRateLimitGroup2JSONString()\n', '\tcommon.OptionMap["ModelRequestRateLimitGroup"] = setting.ModelRequestRateLimitGroup2JSONString()\n\tcommon.OptionMap[setting.PhoneRateLimitPoliciesOptionKey] = setting.PhoneRateLimitPolicies2JSONString()\n')
replace('model/option.go', '\tcase "ModelRequestRateLimitGroup":\n\t\terr = setting.UpdateModelRequestRateLimitGroupByJSONString(value)\n', '\tcase "ModelRequestRateLimitGroup":\n\t\terr = setting.UpdateModelRequestRateLimitGroupByJSONString(value)\n\tcase setting.PhoneRateLimitPoliciesOptionKey:\n\t\terr = setting.UpdatePhoneRateLimitPoliciesByJSONString(value)\n')
replace('router/rate-limit-management.go', '\t\troute.POST("/rebuild", controller.RebuildRateLimitManagement)\n', '\t\troute.POST("/rebuild", controller.RebuildRateLimitManagement)\n\t\troute.GET("/phone-policies", controller.GetPhoneRateLimitPolicies)\n\t\troute.PUT("/phone-policies", controller.UpdatePhoneRateLimitPolicies)\n')

# Legacy top-level phone limits now use the same correctly accounted phone
# limiter as the new group-scoped limits; no duplicate old-phone counters.
p = Path('middleware/model-rate-limit.go')
s = p.read_text()
old = '''\t\t// 手机号/个人标识仍使用顶层数组配置，逻辑保持独立。
\t\txUserId := subNumber(requestUser)
\t\txUserGroupTotalCount, xUserGroupGroupSuccessCount, found := setting.GetGroupRateLimit(xUserId)
\t\tif !found {
\t\t\txUserGroupTotalCount = -1
\t\t\txUserGroupGroupSuccessCount = -1
\t\t}

'''
if s.count(old)!=1: raise RuntimeError('missing legacy phone selection')
s=s.replace(old,'')
old='''\t\tgroupTotalCount, groupSuccessCount := 0, 0
'''
new='''\t\t// New group-specific phone policies override only legacy phone policy,
\t\t// not the group's all/model limits. Old top-level phone limits are a
\t\t// global fallback when this token group has no new phone policy.
\t\tphoneID := strings.TrimSpace(requestUser)
\t\tphoneLimits, phoneConfigured, phoneActive := setting.ResolvePhoneRateLimit(group, phoneID)
\t\tphoneScope := group
\t\tif phoneConfigured {
\t\t\tif phoneActive && !setting.ValidPhoneNumber(phoneID) {
\t\t\t\tabortWithOpenAiMessage(c, http.StatusBadRequest, "当前令牌分组要求提供有效的 11 位手机号")
\t\t\t\treturn
\t\t\t}
\t\t} else {
\t\t\tlegacyID := subNumber(phoneID)
\t\t\tif legacyID != "" {
\t\t\t\tif total, success, exists := setting.GetGroupRateLimit(legacyID); exists {
\t\t\t\t\tphoneID = legacyID
\t\t\t\t\tphoneScope = "legacy"
\t\t\t\t\tphoneLimits = [2]int{total, success}
\t\t\t\t}
\t\t\t}
\t\t}
\t\t// Old handler phone branches are superseded by withPhoneRateLimit.
\t\txUserId, xUserGroupTotalCount, xUserGroupGroupSuccessCount := "", 0, 0

\t\tgroupTotalCount, groupSuccessCount := 0, 0
'''
if s.count(old)!=1: raise RuntimeError('missing group limit anchor')
s=s.replace(old,new)
start=s.index('\t\t// 根据存储类型选择并执行限流处理器')
end=s.index('\n\t}\n}\n\nfunc subNumber',start)
s=s[:start]+'''\t\t// The phone limiter wraps the original model handler. Both layers must
\t\t// allow a request; a failed request releases its reserved completion slot.
\t\tvar next gin.HandlerFunc
\t\tif common.RedisEnabled {
\t\t\tnext = redisRateLimitHandler(duration, totalMaxCount, successMaxCount,
\t\t\t\trequestModel, modelTotalCount, modelSuccessCount,
\t\t\t\txUserId, xUserGroupTotalCount, xUserGroupGroupSuccessCount)
\t\t} else {
\t\t\tnext = memoryRateLimitHandler(duration, totalMaxCount, successMaxCount,
\t\t\t\trequestModel, modelTotalCount, modelSuccessCount,
\t\t\t\txUserId, xUserGroupTotalCount, xUserGroupGroupSuccessCount)
\t\t}
\t\twithPhoneRateLimit(c, phoneScope, phoneID, phoneLimits, duration, next)'''+s[end:]
p.write_text(s)

# A self-contained UI section reuses the existing admin page and backend API.
add('web/classic/src/pages/RateLimitManagement/PhoneRateLimit.jsx', r'''import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';

const { Text } = Typography;
const isPhone = (phone) => /^1[3-9][0-9]{9}$/.test(phone);
const pair = (values) => Array.isArray(values) && values.length === 2 ? values : [0, 0];

export default function PhoneRateLimit({ groups = [] }) {
  const [policies, setPolicies] = useState({});
  const [busy, setBusy] = useState(false);
  const [modal, setModal] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState({ group: '', phone: '', total: 0, success: 0 });
  const load = async () => {
    setBusy(true);
    try {
      const response = await API.get('/api/rate-limit-management/phone-policies');
      if (!response.data.success) throw new Error(response.data.message || '加载手机号策略失败');
      setPolicies(response.data.data || {});
    } catch (error) { showError(error.message || '加载手机号策略失败'); }
    finally { setBusy(false); }
  };
  useEffect(() => { void load(); }, []);
  const save = async (next) => {
    setBusy(true);
    try {
      const response = await API.put('/api/rate-limit-management/phone-policies', { policies: next });
      if (!response.data.success) throw new Error(response.data.message || '保存手机号策略失败');
      setPolicies(next);
      setModal(false);
      showSuccess('手机号限流策略已保存并生效');
    } catch (error) { showError(error.message || '保存手机号策略失败'); }
    finally { setBusy(false); }
  };
  const groupRows = useMemo(() => groups.map((group) => ({
    group, total: pair(policies[group]?.default)[0], success: pair(policies[group]?.default)[1],
    specialCount: Object.keys(policies[group]?.special || {}).length,
  })), [groups, policies]);
  const specialRows = useMemo(() => Object.entries(policies).flatMap(([group, policy]) =>
    Object.entries(policy?.special || {}).map(([phone, limits]) => ({ group, phone, limits }))
  ), [policies]);
  const editDefault = (group, index, value) => {
    setPolicies((previous) => {
      const next = structuredClone(previous);
      if (!next[group]) next[group] = { default: [0, 0], special: {} };
      const limits = [...pair(next[group].default)];
      limits[index] = Number(value) || 0;
      next[group].default = limits;
      return next;
    });
  };
  const open = (row) => {
    setEditing(row ? `${row.group}:${row.phone}` : null);
    setForm(row ? {group: row.group, phone: row.phone, total: pair(row.limits)[0], success: pair(row.limits)[1]}
      : { group: '', phone: '', total: 0, success: 0 });
    setModal(true);
  };
  const saveSpecial = () => {
    if (!form.group || !isPhone(form.phone)) { showError('请选择分组并填写有效的 11 位手机号'); return; }
    const next = structuredClone(policies);
    if (!next[form.group]) next[form.group] = { default: [0,0], special: {} };
    if (!next[form.group].special) next[form.group].special = {};
    next[form.group].special[form.phone] = [Number(form.total)||0, Number(form.success)||0];
    void save(next);
  };
  const remove = (row) => {
    const next = structuredClone(policies);
    delete next[row.group]?.special?.[row.phone];
    void save(next);
  };
  return <div>
    <Card className='!rounded-xl mb-3' title='每个令牌分组的手机号通用限流'>
      <Text type='tertiary'>每个手机号独立计数；同一个手机号在不同令牌分组分别计数。[0,0] 表示不限制。保存后策略立即生效。</Text>
      <Table rowKey='group' dataSource={groupRows} pagination={false} columns={[
        { title:'令牌分组', dataIndex:'group' },
        { title:'每周期最多请求数', render:(_,row)=><InputNumber min={0} value={row.total} onChange={v=>editDefault(row.group,0,v)}/> },
        { title:'每周期最多完成数', render:(_,row)=><InputNumber min={0} value={row.success} onChange={v=>editDefault(row.group,1,v)}/> },
        { title:'特殊号码数', render:(_,row)=><Tag color='blue'>{row.specialCount}</Tag> },
      ]}/>
      <div className='flex justify-end mt-3'><Button loading={busy} type='primary' onClick={()=>void save(policies)}>保存通用限流</Button></div>
    </Card>
    <Card className='!rounded-xl' title='分组特殊手机号限流'>
      <div className='flex justify-between mb-3'><Text type='tertiary'>特殊规则只覆盖本分组的手机号通用规则，不覆盖模型限流。</Text>
        <Button type='primary' onClick={()=>open(null)}>新增特殊号码</Button></div>
      <Table rowKey={row=>`${row.group}:${row.phone}`} dataSource={specialRows} pagination={false} columns={[
        { title:'令牌分组', dataIndex:'group' }, { title:'手机号', dataIndex:'phone' },
        { title:'最多请求数', render:(_,row)=>pair(row.limits)[0] },
        { title:'最多完成数', render:(_,row)=>pair(row.limits)[1] },
        { title:'操作', render:(_,row)=><Space><Button size='small' onClick={()=>open(row)}>编辑</Button><Button size='small' type='danger' onClick={()=>remove(row)}>删除</Button></Space> },
      ]}/>
    </Card>
    <Card className='!rounded-xl mt-3' title='独立手机号策略 JSON（PhoneRateLimitPolicies）'>
      <pre style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{JSON.stringify(policies,null,2)}</pre>
      <Text type='tertiary'>未配置新策略的令牌分组，继续兼容旧版顶层手机号数组规则。</Text>
    </Card>
    <Modal title={editing?'编辑特殊手机号':'新增特殊手机号'} visible={modal} onCancel={()=>setModal(false)} onOk={saveSpecial} okButtonProps={{loading:busy}}>
      <div className='grid gap-3'>
        <div><Text type='tertiary'>令牌分组</Text><Select style={{width:'100%',marginTop:6}} disabled={Boolean(editing)} value={form.group} optionList={groups.map(g=>({label:g,value:g}))} onChange={v=>setForm(p=>({...p,group:v}))}/></div>
        <div><Text type='tertiary'>手机号</Text><Input style={{marginTop:6}} disabled={Boolean(editing)} value={form.phone} maxLength={11} onChange={v=>setForm(p=>({...p,phone:v.trim()}))}/></div>
        <div><Text type='tertiary'>每周期最多请求数</Text><InputNumber min={0} style={{width:'100%',marginTop:6}} value={form.total} onChange={v=>setForm(p=>({...p,total:Number(v)||0}))}/></div>
        <div><Text type='tertiary'>每周期最多完成数</Text><InputNumber min={0} style={{width:'100%',marginTop:6}} value={form.success} onChange={v=>setForm(p=>({...p,success:Number(v)||0}))}/></div>
      </div>
    </Modal>
  </div>;
}
''')
replace('web/classic/src/pages/RateLimitManagement/index.jsx', "import { API, showError, showSuccess } from '../../helpers';\n", "import { API, showError, showSuccess } from '../../helpers';\nimport PhoneRateLimit from './PhoneRateLimit';\n")
replace('web/classic/src/pages/RateLimitManagement/index.jsx', "            </Tabs.TabPane>\n          </Tabs>\n        </Card>\n\n        <Card className='!rounded-2xl mt-3' title='当前兼容限流 JSON'>", "            </Tabs.TabPane>\n            <Tabs.TabPane tab='手机号限流管理' itemKey='phone'>\n              <PhoneRateLimit groups={data?.groups || []} />\n            </Tabs.TabPane>\n          </Tabs>\n        </Card>\n\n        <Card className='!rounded-2xl mt-3' title='当前兼容限流 JSON'>")
replace('web/classic/src/pages/RateLimitManagement/index.jsx', '该 JSON 为运行时最终配置，继续兼容现有 ModelRequestRateLimitGroup 规范；手机号/Account 顶层数组配置会保留。', '该 JSON 仅用于用户和模型限流；手机号新策略独立保存为 PhoneRateLimitPolicies，旧顶层手机号数组仅用于未配置新策略的分组。')

add('docs/phone-rate-limit.md', '''# 分组手机号限流\n\n模型限流仍使用 `ModelRequestRateLimitGroup`，手机号策略独立保存于 `PhoneRateLimitPolicies`，无需新增数据表。\n\n```json\n{\n  "group1": {"default": [10, 5], "special": {"18946512326": [100, 50]}},\n  "group2": {"default": [20, 10], "special": {"18946512326": [5, 2]}}\n}\n```\n\n- 每组每手机号独立计数。特殊号覆盖同组默认值；[0,0] 表示该层不限制。组/模型原有限制依旧同时生效。\n- 未配置新手机号策略的分组沿用旧 `ModelRequestRateLimitGroup` 顶层号码数组；一旦配置新策略，旧全局号码规则不再作用于此组。\n- 新策略要求从请求体 `user` 或请求头 `X-User-Id` 获取完整有效的 11 位中国大陆手机号；请求体优先。**必须由可信业务后端提供身份，不能信任终端自行填写的手机号。**\n- Redis 使用固定周期和 Lua 原子准入计数：总请求含失败请求，成功请求保留名额直到请求完成，失败退还完成名额；内存模式仅单进程计数，多副本必须共用 Redis。\n- 手机号仅以 HMAC 标识出现在 Redis Key；变更 CRYPTO_SECRET 会导致新计数空间。新旧限流 Key 不共用，部署切换时新策略计数从零开始。\n- 对流式响应，HTTP 状态码已经提交为 2xx 后发生的流内错误无法可靠地识别为失败，计数遵循原系统 HTTP 状态码口径。\n- 全局开关 `ModelRequestRateLimitEnabled` 以及全局周期 `ModelRequestRateLimitDurationMinutes` 仍决定本功能是否运行及周期长度。\n''')
print('phone policy source transformation completed')
