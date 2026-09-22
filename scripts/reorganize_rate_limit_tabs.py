"""One-time fail-fast UI layout migration; removed before merging."""
from pathlib import Path

page = Path('web/classic/src/pages/RateLimitManagement/index.jsx')
s = page.read_text(encoding='utf-8')


def once(old, new, label):
    global s
    count = s.count(old)
    if count != 1:
        raise SystemExit(f'{label}: expected exactly one anchor, got {count}')
    s = s.replace(old, new, 1)


model_start = "              <Card className='!rounded-xl mb-3' title='模型分类'>"
model_end = "              </Card>\n\n              <Card className='!rounded-xl' title='按模型分类设置通用限流'>"
if s.count(model_start) != 1 or s.count(model_end) != 1:
    raise SystemExit('model category card boundaries not unique')
start = s.index(model_start)
end = s.index(model_end, start)
model_card = s[start:end] + '              </Card>\n'
s = s[:start] + s[end + len("              </Card>\n\n"):]  # Keep category-limit card in its original group position.

once(
    "          <Tabs type='card' defaultActiveKey='general'>\n            <Tabs.TabPane tab='通用限流管理' itemKey='general'>",
    "          <Tabs type='card' defaultActiveKey='model-category'>\n"
    "            <Tabs.TabPane tab='模型分类' itemKey='model-category'>\n"
    + model_card +
    "            </Tabs.TabPane>\n\n"
    "            <Tabs.TabPane tab='分组限流' itemKey='group-limits'>",
    'top-level tab structure',
)
once(
    "<Card className='!rounded-xl' title='按模型分类设置通用限流'>",
    "<Card className='!rounded-xl' title='分组通用限流（按模型分类）'>",
    'group default card label',
)
once(
    "              </Card>\n            </Tabs.TabPane>\n\n            <Tabs.TabPane tab='特殊限流配置' itemKey='special'>\n              <div className='flex items-center justify-between mb-3'>",
    "              </Card>\n\n              <Card className='!rounded-xl mt-3' title='分组特殊限流'>\n                <div className='flex items-center justify-between mb-3'>",
    'combine general and special panes',
)
once(
    "                pagination={false}\n              />\n            </Tabs.TabPane>\n            <Tabs.TabPane tab='用户标识限流管理' itemKey='phone'>",
    "                pagination={false}\n              />\n              </Card>\n            </Tabs.TabPane>\n\n            <Tabs.TabPane tab='用户标识限流' itemKey='identifier-limits'>",
    'close combined group pane and rename identifier tab',
)
once(
    '该 JSON 仅用于用户和模型限流；手机号新策略独立保存为 PhoneRateLimitPolicies，旧顶层手机号数组仅用于未配置新策略的分组。',
    '该 JSON 仅用于用户和模型限流；用户标识策略独立保存为 PhoneRateLimitPolicies，旧版顶层号码数组仅用于未配置新策略的分组。',
    'JSON explanation',
)
page.write_text(s, encoding='utf-8')

ci = Path('.github/workflows/ci.yml')
text = ci.read_text(encoding='utf-8')
old = 'run: bun test src/helpers/apiFailurePolicy.test.js'
assert text.count(old) == 1, 'CI test command must be unique'
text = text.replace(old, 'run: bun test src/helpers/apiFailurePolicy.test.js src/pages/RateLimitManagement/tabLayout.test.js', 1)
ci.write_text(text, encoding='utf-8')
print('Rate limit tab layout transformed; backend and policy logic untouched.')
