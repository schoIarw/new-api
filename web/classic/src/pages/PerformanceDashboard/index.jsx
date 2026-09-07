/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Card,
  Select,
  Spin,
  Tag,
  Typography,
  Empty,
  Tabs,
  DatePicker,
  RadioGroup,
  Radio,
  Row,
  Col,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { Gauge } from 'lucide-react';
import { API, showError } from '../../helpers';

const { Text } = Typography;

const CHART_CONFIG = { mode: 'desktop-browser' };
const BUCKET_MINUTES = 5;

const HOUR_OPTIONS = [1, 2, 4].map((h) => ({
  label: `最近 ${h} 小时`,
  value: h,
}));
const DEFAULT_HOURS = 1;

const REFRESH_OPTIONS = [
  { label: '低 (1分)', value: 60 },
  { label: '中 (30秒)', value: 30 },
  { label: '高 (10秒)', value: 10 },
];
const DEFAULT_REFRESH = 30;

// 顶部性能摘要。模型访问量/Token 已融合进同一接口，但只作为趋势页签展示，
// 避免把性能摘要区扩展成七张卡片。
const PERFORMANCE_METRICS = [
  {
    key: 'frt',
    label: '首字节时间',
    fields: { avg: 'avg_frt', min: 'min_frt', max: 'max_frt' },
    unit: 'ms',
    hasRange: true,
    format: (v) => Math.round(v),
  },
  {
    key: 'token_rate',
    label: 'Token 生成速率',
    fields: {
      avg: 'avg_token_rate',
      min: 'min_token_rate',
      max: 'max_token_rate',
    },
    unit: 'tok/s',
    hasRange: true,
    format: (v) => Number(v || 0).toFixed(1),
  },
  {
    key: 'rpm',
    label: 'RPM',
    single: 'rpm',
    unit: 'req/min',
    hasRange: false,
    aggregateByBucket: true,
    format: (v) => Number(v || 0).toFixed(1),
  },
  {
    key: 'tpm',
    label: 'TPM',
    single: 'tpm',
    unit: 'tok/min',
    hasRange: false,
    aggregateByBucket: true,
    format: (v) => Math.round(v || 0),
  },
];

// 原模型看板的三个核心指标直接复用 performance_dashboard 返回的
// count / prompt_tokens / completion_tokens，不再发起第二次模型看板查询。
const MODEL_METRICS = [
  {
    key: 'request_count',
    label: '访问次数',
    single: 'count',
    unit: '次/5分钟',
    hasRange: false,
    format: (v) => Math.round(v || 0),
  },
  {
    key: 'prompt_tokens',
    label: '输入Token',
    single: 'prompt_tokens',
    unit: 'Token/5分钟',
    hasRange: false,
    format: (v) => Math.round(v || 0),
  },
  {
    key: 'completion_tokens',
    label: '完成Token',
    single: 'completion_tokens',
    unit: 'Token/5分钟',
    hasRange: false,
    format: (v) => Math.round(v || 0),
  },
];

const ALL_METRICS = [...PERFORMANCE_METRICS, ...MODEL_METRICS];

function formatBucket(ts, isHistorical) {
  const d = new Date(ts * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  if (isHistorical) {
    const mmd = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    return `${mmd}-${dd} ${hh}:${mm}`;
  }
  return `${hh}:${mm}`;
}

function summarizeRange(items, metric) {
  const validAvg = items.filter((it) => Number(it[metric.fields.avg]) > 0);
  // 按桶请求量加权，避免“1000 个请求的桶”和“1 个请求的桶”等权平均。
  // 后端目前没有单独返回 FRT/token-rate 样本数，count 是现有数据下最稳定的权重。
  const totalWeight = validAvg.reduce((sum, it) => sum + Math.max(Number(it.count) || 0, 1), 0);
  const weightedSum = validAvg.reduce(
    (sum, it) => sum + Number(it[metric.fields.avg]) * Math.max(Number(it.count) || 0, 1),
    0,
  );
  const minValues = items
    .map((it) => Number(it[metric.fields.min]) || 0)
    .filter((v) => v > 0);
  const maxValues = items
    .map((it) => Number(it[metric.fields.max]) || 0)
    .filter((v) => v > 0);

  return {
    avg: totalWeight > 0 ? weightedSum / totalWeight : 0,
    min: minValues.length ? Math.min(...minValues) : 0,
    max: maxValues.length ? Math.max(...maxValues) : 0,
  };
}

function summarizeRateByBucket(items, field) {
  const bucketTotals = new Map();
  items.forEach((it) => {
    const bucket = Number(it.bucket) || 0;
    bucketTotals.set(bucket, (bucketTotals.get(bucket) || 0) + (Number(it[field]) || 0));
  });
  const values = [...bucketTotals.values()];
  if (!values.length) return { avg: 0, min: 0, max: 0 };
  return {
    avg: values.reduce((a, b) => a + b, 0) / values.length,
    min: Math.min(...values),
    max: Math.max(...values),
  };
}

const PerformanceDashboard = () => {
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState('realtime');
  const [hours, setHours] = useState(DEFAULT_HOURS);
  const [dateRange, setDateRange] = useState([]);
  const [rawItems, setRawItems] = useState([]);
  const [metric, setMetric] = useState('frt');
  const [filterKey, setFilterKey] = useState('__ignore__');
  const [filterModel, setFilterModel] = useState('');
  const [refreshInterval, setRefreshInterval] = useState(DEFAULT_REFRESH);
  const [lastUpdated, setLastUpdated] = useState(null);
  const refreshTimerRef = useRef(null);
  const requestSeqRef = useRef(0);

  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  const isHistorical = mode === 'historical';
  const isIgnoreKey = filterKey === '__ignore__';

  const loadData = useCallback(async () => {
    const requestSeq = ++requestSeqRef.current;
    setLoading(true);
    try {
      let params;
      if (isHistorical && dateRange && dateRange.length === 2) {
        const startTs = Math.floor(dateRange[0].getTime() / 1000);
        const endTs = Math.floor(dateRange[1].getTime() / 1000);
        if (endTs - startTs > 48 * 3600) {
          showError('历史查询时间范围不能超过48小时');
          return;
        }
        params = { start_timestamp: startTs, end_timestamp: endTs };
      } else {
        // 实时模式每次刷新只传 hours，由后端用当前 time.Now() 重建滑动窗口。
        // 不能把首次加载的 start/end 固定下来，否则卡片和曲线都会逐渐“假实时”。
        params = { hours };
      }
      if (isIgnoreKey) params.ignore_key = 'true';

      const res = await API.get('/api/log/performance_dashboard', { params });
      if (!res.data.success) {
        throw new Error(res.data.message || '获取性能看板数据失败');
      }
      if (requestSeq !== requestSeqRef.current) return;

      setRawItems(res.data.data.items || []);
      setLastUpdated(new Date());
    } catch (e) {
      if (requestSeq === requestSeqRef.current) {
        showError(e.message || '获取性能看板数据失败');
      }
    } finally {
      if (requestSeq === requestSeqRef.current) setLoading(false);
    }
  }, [hours, isHistorical, dateRange, isIgnoreKey]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  useEffect(() => {
    if (refreshTimerRef.current) clearInterval(refreshTimerRef.current);
    refreshTimerRef.current = null;

    if (!isHistorical) {
      refreshTimerRef.current = setInterval(loadData, refreshInterval * 1000);
    }
    return () => {
      if (refreshTimerRef.current) clearInterval(refreshTimerRef.current);
    };
  }, [isHistorical, loadData, refreshInterval]);

  const enrichedItems = useMemo(
    () =>
      rawItems.map((it) => ({
        ...it,
        rpm: (Number(it.count) || 0) / BUCKET_MINUTES,
        tpm:
          ((Number(it.prompt_tokens) || 0) + (Number(it.completion_tokens) || 0)) /
          BUCKET_MINUTES,
      })),
    [rawItems],
  );

  const filteredItems = useMemo(
    () =>
      enrichedItems.filter((it) => {
        if (!isIgnoreKey && filterKey && it.token_name !== filterKey) return false;
        if (filterModel && it.model_name !== filterModel) return false;
        return true;
      }),
    [enrichedItems, filterKey, filterModel, isIgnoreKey],
  );

  const keyOptions = useMemo(() => {
    const set = new Set();
    rawItems.forEach((it) => it.token_name && set.add(it.token_name));
    const opts = [
      { label: '忽略 Key', value: '__ignore__' },
      { label: '全部 Key', value: '' },
    ];
    [...set].sort().forEach((k) => opts.push({ label: k, value: k }));
    return opts;
  }, [rawItems]);

  const modelOptions = useMemo(() => {
    const set = new Set();
    rawItems.forEach((it) => it.model_name && set.add(it.model_name));
    const opts = [{ label: '全部模型', value: '' }];
    [...set].sort().forEach((m) => opts.push({ label: m, value: m }));
    return opts;
  }, [rawItems]);

  const metricMeta = ALL_METRICS.find((m) => m.key === metric) || ALL_METRICS[0];

  // 摘要依赖 filteredItems；每次实时请求 setRawItems 后都会重新计算，
  // 因而 FRT、Token速率、RPM、TPM 卡片与曲线使用同一批最新数据刷新。
  const summary = useMemo(() => {
    const result = {};
    PERFORMANCE_METRICS.forEach((m) => {
      if (m.hasRange) {
        result[m.key] = summarizeRange(filteredItems, m);
      } else if (m.aggregateByBucket) {
        result[m.key] = summarizeRateByBucket(filteredItems, m.single);
      }
    });
    return result;
  }, [filteredItems]);

  const chartData = useMemo(() => {
    const buckets = [...new Set(filteredItems.map((it) => it.bucket))].sort((a, b) => a - b);
    const seriesSet = new Set();
    filteredItems.forEach((it) => seriesSet.add(`${it.token_name}||${it.model_name}`));
    const seriesKeys = [...seriesSet];

    const valueMap = {};
    filteredItems.forEach((it) => {
      const sKey = `${it.token_name}||${it.model_name}`;
      if (!valueMap[sKey]) valueMap[sKey] = {};
      valueMap[sKey][it.bucket] = it;
    });

    const seriesLabel = (tokenName, modelName) =>
      isIgnoreKey || !tokenName ? modelName : `${tokenName} / ${modelName}`;

    const rows = [];
    buckets.forEach((bucket) => {
      const label = formatBucket(bucket, isHistorical);
      seriesKeys.forEach((sKey) => {
        const [tokenName, modelName] = sKey.split('||');
        const it = valueMap[sKey]?.[bucket];
        const base = { bucket: label, bucketTs: bucket, tokenName, modelName };
        const sName = seriesLabel(tokenName, modelName);

        if (metricMeta.hasRange) {
          rows.push({
            ...base,
            name: `${sName} · 平均`,
            type: '平均',
            value: it?.[metricMeta.fields.avg] || 0,
          });
          rows.push({
            ...base,
            name: `${sName} · 最大`,
            type: '最大',
            value: it?.[metricMeta.fields.max] || 0,
          });
          rows.push({
            ...base,
            name: `${sName} · 最小`,
            type: '最小',
            value: it?.[metricMeta.fields.min] || 0,
          });
        } else {
          rows.push({
            ...base,
            name: sName,
            type: '值',
            value: it?.[metricMeta.single] || 0,
          });
        }
      });
    });
    return rows;
  }, [filteredItems, metricMeta, isHistorical, isIgnoreKey]);

  const filteredSeriesCount = useMemo(
    () => new Set(filteredItems.map((it) => `${it.token_name}||${it.model_name}`)).size,
    [filteredItems],
  );

  const spec = useMemo(() => {
    const subtext = isHistorical
      ? '历史数据 ｜ 粒度：5 分钟'
      : `最近 ${hours} 小时 ｜ 粒度：5 分钟 ｜ 实时滑动窗口`;

    return {
      type: 'line',
      data: [{ id: 'perfData', values: chartData }],
      xField: 'bucket',
      yField: 'value',
      seriesField: 'name',
      stack: false,
      smooth: true,
      legends: { visible: true, selectMode: 'multiple', position: 'bottom' },
      title: { visible: true, text: `${metricMeta.label} 趋势`, subtext },
      line: { style: { lineWidth: 2 } },
      point: { visible: true, size: 3 },
      axes: [
        {
          orient: 'left',
          title: { visible: true, text: `${metricMeta.label} (${metricMeta.unit})` },
        },
        { orient: 'bottom', title: { visible: true, text: '时间' } },
      ],
      tooltip: {
        mark: {
          content: [
            { key: 'Key', value: (datum) => datum?.tokenName || '-' },
            { key: '模型', value: (datum) => datum?.modelName || '-' },
            { key: '类型', value: (datum) => datum?.type || '-' },
            {
              key: metricMeta.label,
              value: (datum) =>
                `${metricMeta.format(datum?.value ?? 0)} ${metricMeta.unit}`,
            },
          ],
        },
      },
    };
  }, [chartData, metricMeta, hours, isHistorical]);

  return (
    <div className='mt-[60px] px-2'>
      <Card
        className='!rounded-2xl'
        title={
          <div className='flex flex-col w-full gap-3'>
            <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between w-full gap-3'>
              <div className='flex items-center gap-2'>
                <Gauge size={16} />
                <span>性能看板</span>
                <Tag color={isHistorical ? 'grey' : 'blue'} size='small'>
                  {isHistorical ? '历史数据' : '自动刷新中'}
                </Tag>
                {!isHistorical && lastUpdated && (
                  <Text type='tertiary' size='small'>
                    更新 {lastUpdated.toLocaleTimeString('zh-CN', { hour12: false })}
                  </Text>
                )}
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <RadioGroup type='button' value={mode} onChange={(e) => setMode(e.target.value)}>
                  <Radio value='realtime'>实时</Radio>
                  <Radio value='historical'>历史</Radio>
                </RadioGroup>
                {isHistorical ? (
                  <DatePicker
                    type='dateTimeRange'
                    value={dateRange}
                    onChange={(v) => setDateRange(v || [])}
                    placeholder={['开始时间', '结束时间']}
                    style={{ width: 320 }}
                  />
                ) : (
                  <>
                    <Select
                      value={hours}
                      onChange={(v) => setHours(v)}
                      optionList={HOUR_OPTIONS}
                      style={{ width: 140 }}
                    />
                    <Select
                      value={refreshInterval}
                      onChange={(v) => setRefreshInterval(v)}
                      optionList={REFRESH_OPTIONS}
                      style={{ width: 130 }}
                    />
                  </>
                )}
              </div>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <Text type='tertiary' size='small'>
                筛选
              </Text>
              <Select
                value={filterKey}
                onChange={(v) => setFilterKey(v ?? '')}
                optionList={keyOptions}
                placeholder='选择 Key'
                style={{ width: 180 }}
              />
              <Select
                value={filterModel}
                onChange={(v) => setFilterModel(v || '')}
                optionList={modelOptions}
                placeholder='选择模型'
                style={{ width: 200 }}
              />
              {(filterKey !== '__ignore__' && filterKey) || filterModel ? (
                <Tag color='light-blue' size='small'>命中 {filteredSeriesCount} 组</Tag>
              ) : isIgnoreKey ? (
                <Tag color='violet' size='small'>按模型分组</Tag>
              ) : null}
            </div>
          </div>
        }
        bodyStyle={{ padding: 0 }}
      >
        <Spin spinning={loading}>
          <div className='px-2 pt-3'>
            <Row gutter={[8, 8]}>
              {PERFORMANCE_METRICS.map((m) => {
                const s = summary[m.key] || { avg: 0, min: 0, max: 0 };
                return (
                  <Col key={m.key} xs={12} sm={6}>
                    <Card className='!rounded-xl' bodyStyle={{ padding: 12 }}>
                      <div className='text-xs text-slate-500 mb-1'>{m.label}</div>
                      <div className='flex justify-between text-center'>
                        <div>
                          <div className='text-[11px] text-slate-400'>最小</div>
                          <div className='text-sm font-medium' style={{ color: '#10b981' }}>
                            {m.format(s.min)}
                          </div>
                        </div>
                        <div>
                          <div className='text-[11px] text-slate-400'>平均</div>
                          <div className='text-sm font-medium' style={{ color: '#3b82f6' }}>
                            {m.format(s.avg)}
                          </div>
                        </div>
                        <div>
                          <div className='text-[11px] text-slate-400'>最大</div>
                          <div className='text-sm font-medium' style={{ color: '#ef4444' }}>
                            {m.format(s.max)}
                          </div>
                        </div>
                      </div>
                      <div className='text-[10px] text-slate-400 mt-1 text-center'>{m.unit}</div>
                    </Card>
                  </Col>
                );
              })}
            </Row>
          </div>

          <div className='px-2 pt-2'>
            <Tabs type='card' activeKey={metric} onChange={setMetric}>
              {ALL_METRICS.map((m) => (
                <Tabs.TabPane key={m.key} tab={m.label} itemKey={m.key} />
              ))}
            </Tabs>
          </div>

          <div className='h-[480px] p-2'>
            {chartData.length > 0 ? (
              <VChart spec={spec} option={CHART_CONFIG} />
            ) : rawItems.length > 0 ? (
              <Empty
                title='无匹配数据'
                description='当前筛选条件下没有命中的曲线'
                style={{ padding: 80 }}
              />
            ) : (
              <Empty
                title='暂无数据'
                description='所选时间范围内没有消费日志'
                style={{ padding: 80 }}
              />
            )}
          </div>
        </Spin>
      </Card>
    </div>
  );
};

export default PerformanceDashboard;
