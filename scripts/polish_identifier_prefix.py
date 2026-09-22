"""Fail-fast follow-up refinements before merging; removed after verification."""
from pathlib import Path


def replace(path, old, new):
    p = Path(path)
    content = p.read_text(encoding='utf-8')
    assert content.count(old) == 1, f'{path}: expected single match for {old!r}, got {content.count(old)}'
    p.write_text(content.replace(old, new, 1), encoding='utf-8')


model = 'middleware/model-rate-limit.go'
replace(model, 'userId = strings.TrimSpace(meta.User)', 'userId = meta.User')
replace(model, 'identifier := strings.TrimSpace(requestUser)', 'identifier := requestUser')
replace(model, 'if identifierActive && identifier == "" {', 'if identifierActive && strings.TrimSpace(identifier) == "" {')

ui = 'web/classic/src/pages/RateLimitManagement/PhoneRateLimit.jsx'
replace(ui, "import { API, showError, showSuccess } from '../../helpers';", "import { API, showError, showSuccess } from '../../helpers';\nimport { isValidIdentifierPrefix } from './userIdentifierPolicy';")
replace(ui, "const { Text } = Typography;\nimport { isValidIdentifierPrefix } from './userIdentifierPolicy';", "const { Text } = Typography;")
replace(ui, 'export default function PhoneRateLimit(', 'export default function UserIdentifierRateLimit(')
replace(ui, "{ title:'用户标识', dataIndex:'prefix' }", "{ title:'用户标识前缀', dataIndex:'prefix' }")
replace(ui, "<Text type='tertiary'>用户标识</Text><Input", "<Text type='tertiary'>用户标识前缀（至少 8 个字符）</Text><Input")
replace(ui, '继续兼容旧版顶层用户标识数组规则', '继续兼容旧版顶层号码数组规则')
replace(ui, 'next[form.group].special[form.prefix] = [Number(form.total)||0, Number(form.success)||0];', 'next[form.group].special = { ...next[form.group].special, [form.prefix]: [Number(form.total)||0, Number(form.success)||0] };')
replace(ui, '`${row.group}:${row.prefix}`', 'JSON.stringify([row.group, row.prefix])')
replace(ui, '`${row.group}:${row.prefix}`', 'JSON.stringify([row.group, row.prefix])')
index = 'web/classic/src/pages/RateLimitManagement/index.jsx'
replace(index, "import PhoneRateLimit from './PhoneRateLimit';", "import UserIdentifierRateLimit from './PhoneRateLimit';")
replace(index, '<PhoneRateLimit groups={data?.groups || []} />', '<UserIdentifierRateLimit groups={data?.groups || []} />')

middleware_test = 'middleware/phone_rate_limit_test.go'
replace(middleware_test, '\t"sync"\n', '\t"io"\n\t"net/http"\n\t"net/http/httptest"\n\t"strings"\n\t"sync"\n')
replace(middleware_test, '\t"testing"\n)', '\t"testing"\n\n\t"github.com/gin-gonic/gin"\n)')
p = Path(middleware_test)
p.write_text(p.read_text(encoding='utf-8') + '''
func TestIdentifierRequestBodyPreservesArbitraryString(t *testing.T) {
    c, _ := gin.CreateTestContext(httptest.NewRecorder())
    body := `{"user":"  tenant001|appid|ip  ","model":"demo"}`
    c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
    identifier, model := getRateLimitRequestMeta(c)
    if identifier != "  tenant001|appid|ip  " || model != "demo" {
        t.Fatalf("identifier was normalized or model lost: %q %q", identifier, model)
    }
    restored, err := io.ReadAll(c.Request.Body)
    if err != nil || string(restored) != body {
        t.Fatalf("request body was not preserved: %q %v", restored, err)
    }
}
''', encoding='utf-8')

p = Path('docs/phone-rate-limit.md')
text = p.read_text(encoding='utf-8')
text = text.replace('标识允许任意字符串，不拆分 `|`、不截取手机号、不限制为数字。', '标识允许任意字符串，保留 JSON 请求体 `user` 内前后空格，不拆分 `|`、不截取手机号、不限制为数字。')
p.write_text(text, encoding='utf-8')
print('identifier follow-up refinement complete')
