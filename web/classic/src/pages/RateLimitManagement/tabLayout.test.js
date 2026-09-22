import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./index.jsx', import.meta.url), 'utf8');
const tabsStart = source.indexOf("<Tabs type='card'");
const tabsEnd = source.indexOf('</Tabs>', tabsStart);
const tabs = source.slice(tabsStart, tabsEnd);
const matches = [...tabs.matchAll(/<Tabs\.TabPane tab='([^']+)' itemKey='([^']+)'>/g)];
const section = (index) => tabs.slice(matches[index].index, matches[index + 1]?.index ?? tabs.length);

describe('rate limit management tab layout', () => {
  test('has exactly three tabs in the requested order', () => {
    expect(tabsStart).toBeGreaterThan(-1);
    expect(tabsEnd).toBeGreaterThan(tabsStart);
    expect(matches.map((match) => match[1])).toEqual(['模型分类', '分组限流', '用户标识限流']);
    expect(matches.map((match) => match[2])).toEqual(['model-category', 'group-limits', 'identifier-limits']);
    expect(tabs).toContain("defaultActiveKey='model-category'");
  });

  test('model category tab owns model assignment but not limit editors', () => {
    expect(section(0)).toContain("title='模型分类'");
    expect(section(0)).toContain('columns={modelColumns}');
    expect(section(0)).toContain('dataSource={pagedModels}');
    expect(section(0)).not.toContain('saveDefaults');
    expect(section(0)).not.toContain('saveCategoryLimits');
    expect(section(0)).not.toContain('specialColumns');
  });

  test('group tab includes basic, general and special limits with their existing handlers', () => {
    const group = section(1);
    expect(group).toContain("title='基础限流参数'");
    expect(group).toContain('onClick={saveDefaults}');
    expect(group).toContain("title='分组通用限流（按模型分类）'");
    expect(group).toContain('columns={categoryColumns}');
    expect(group).toContain('onClick={saveCategoryLimits}');
    expect(group).toContain("title='分组特殊限流'");
    expect(group).toContain('columns={specialColumns}');
    expect(group).toContain('openSpecialModal()');
    expect(group).not.toContain('columns={modelColumns}');
  });

  test('identifier tab preserves the independent editor and compatible JSON description', () => {
    expect(section(2)).toContain('<UserIdentifierRateLimit groups={data?.groups || []} />');
    expect(source).toContain('用户标识策略独立保存为 PhoneRateLimitPolicies');
  });
});
