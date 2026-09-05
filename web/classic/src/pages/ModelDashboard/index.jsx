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
import { Card, Select, Spin, Tag, Typography, Empty, Tabs, DatePicker, RadioGroup, Radio } from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { BarChart3 } from 'lucide-react';
import { API, showError } from '../../helpers';

const { Text } = Typography;

const CHART_CONFIG = { mode: 'desktop-browser' };
const REFRESH_INTERVAL_MS = 10000; // 自动刷新间隔

// 实时模式时间范围下拉选项（小时）
const HOUR_OPTIONS = [1, 2, 4, 8, 24].map((h) => ({
  label: `最近 ${h} 小时`,
  value: h,
}));
const DEFAULT_HOURS = 1;

// 指标类型
const METRICS = [
  { key: 'count', label: '访问次数', color: '#3b82f6' },
  { key: 'prompt_tokens', label: '输入 Token', color: '#10b981' },
  { key: 'completion_tokens', label: '完成 Token', color: '#f59e0b' },
];

// 5 分钟桶时间戳格式化为 HH:mm（历史模式跨天时显示 MM-DD HH:mm）
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

const ModelDashboard = () => {
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState('realtime'); // 'realtime' | 'historical'
  const [hours, setHours] = useState(DEFAULT_HOURS);
  const [dateRange, setDateRange] = useState([]); // [startDate, endDate] Date objects
  const [rawItems, setRawItems] = useState([]);
  const [metric, setMetric] = useState('count');
  const [filterKey, setFilterKey] = useState('__ignore__'); // '__ignore__' = 忽略Key
  const [filterModel, setFilterModel] = useState('');
  const refreshTimerRef = useRef(null);

  // 初始化 Semi 主题
  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  const isHistorical = mode === 'historical';
  const isIgnoreKey = filterKey === '__ignore__';

  // 拉取数据
  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      let params = {};
      if (isHistorical && dateRange && dateRange.length === 2) {
        params = {
          start_timestamp: Math.floor(dateRange[0].getTime() / 1000),
          end_timestamp: Math.floor(dateRange[1].getTime() / 1000),
        };
      } else {
        params = { hours };
      }
      if (isIgnoreKey) {
        params.ignore_key = 'true';
      }
      const res = await API.get('/api/log/model_dashboard', { params });
      if (!res.data.success) {
        throw new Error(res.data.message || '获取模型看板数据失败');
      }
      setRawItems(res.data.data.items || []);
    } catch (e) {
      showError(e.message || '获取模型看板数据失败');
    } finally {
      setLoading(false);
    }
  }, [hours, isHistorical, dateRange, isIgnoreKey]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // 仅实时模式自动刷新
  useEffect(() => {
    if (refreshTimerRef.current) {
      clearInterval(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }
    if (!isHistorical) {
      refreshTimerRef.current = setInterval(() => {
        loadData();
      }, REFRESH_INTERVAL_MS);
    }
    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
      }
    };
  }, [isHistorical, loadData]);

  // 筛选后的数据
  const filteredItems = useMemo(() => {
    return rawItems.filter((it) => {
      if (isIgnoreKey) {
        // 忽略Key 模式下不按 token_name 筛选
      } else if (filterKey && it.token_name !== filterKey) {
        return false;
      }
      if (filterModel && it.model_name !== filterModel) return false;
      return true;
    });
  }, [rawItems, filterKey, filterModel, isIgnoreKey]);

  // 所有 token_name 和 model_name 选项
  const keyOptions = useMemo(() => {
    const set = new Set();
    rawItems.forEach((it) => {
      if (it.token_name) set.add(it.token_name);
    });
    const opts = [
      { label: '忽略 Key', value: '__ignore__' },
      { label: '全部 Key', value: '' },
    ];
    [...set].sort().forEach((k) => opts.push({ label: k, value: k }));
    return opts;
  }, [rawItems]);

  const modelOptions = useMemo(() => {
    const set = new Set();
    rawItems.forEach((it) => {
      if (it.model_name) set.add(it.model_name);
    });
    const opts = [{ label: '全部模型', value: '' }];
    [...set].sort().forEach((m) => opts.push({ label: m, value: m }));
    return opts;
  }, [rawItems]);

  // 构建图表数据
  const chartData = useMemo(() => {
    const buckets = [...new Set(filteredItems.map((it) => it.bucket))].sort(
      (a, b) => a - b,
    );
    const seriesSet = new Set();
    filteredItems.forEach((it) => {
      seriesSet.add(`${it.token_name}||${it.model_name}`);
    });
    const seriesKeys = [...seriesSet];

    const valueMap = {};
    filteredItems.forEach((it) => {
      const sKey = `${it.token_name}||${it.model_name}`;
      if (!valueMap[sKey]) valueMap[sKey] = {};
      valueMap[sKey][it.bucket] = it[metric] || 0;
    });

    const rows = [];
    buckets.forEach((bucket) => {
      const label = formatBucket(bucket, isHistorical);
      seriesKeys.forEach((sKey) => {
        const [tokenName, modelName] = sKey.split('||');
        const sName = isIgnoreKey || !tokenName ? modelName : `${tokenName} / ${modelName}`;
        rows.push({
          bucket: label,
          bucketTs: bucket,
          name: sName,
          tokenName,
          modelName,
          value: valueMap[sKey]?.[bucket] || 0,
        });
      });
    });
    return rows;
  }, [filteredItems, metric, isHistorical, isIgnoreKey]);

  const filteredSeriesCount = useMemo(
    () => new Set(filteredItems.map((it) => `${it.token_name}||${it.model_name}`)).size,
    [filteredItems],
  );

  const metricMeta = METRICS.find((m) => m.key === metric) || METRICS[0];

  const spec = useMemo(() => {
    const colorMap = {};
    chartData.forEach((row) => {
      if (!(row.name in colorMap)) {
        colorMap[row.name] = metricMeta.color;
      }
    });

    const subtext = isHistorical
      ? '历史数据 ｜ 粒度：5 分钟'
      : `最近 ${hours} 小时 ｜ 粒度：5 分钟`;

    return {
      type: 'line',
      data: [{ id: 'modelData', values: chartData }],
      xField: 'bucket',
      yField: 'value',
      seriesField: 'name',
      stack: false,
      smooth: true,
      legends: {
        visible: true,
        selectMode: 'multiple',
        position: 'bottom',
      },
      title: {
        visible: true,
        text: `模型 / Key 趋势 — ${metricMeta.label}`,
        subtext,
      },
      line: { style: { lineWidth: 2 } },
      point: { visible: true, size: 3 },
      axes: [
        { orient: 'left', title: { visible: true, text: metricMeta.label } },
        { orient: 'bottom', title: { visible: true, text: '时间' } },
      ],
      tooltip: {
        mark: {
          content: [
            { key: 'Key', value: (datum) => datum?.tokenName || '-' },
            { key: '模型', value: (datum) => datum?.modelName || '-' },
            {
              key: metricMeta.label,
              value: (datum) => `${datum?.value ?? 0}`,
            },
          ],
        },
      },
      color: { specified: colorMap },
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
                <BarChart3 size={16} />
                <span>模型看板</span>
                <Tag color={isHistorical ? 'grey' : 'blue'} size='small'>
                  {isHistorical ? '历史数据' : '自动刷新中'}
                </Tag>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <RadioGroup
                  type='button'
                  value={mode}
                  onChange={(e) => setMode(e.target.value)}
                >
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
                  <Select
                    value={hours}
                    onChange={(v) => setHours(v)}
                    optionList={HOUR_OPTIONS}
                    style={{ width: 140 }}
                  />
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
                <Tag color='light-blue' size='small'>
                  命中 {filteredSeriesCount} 条曲线
                </Tag>
              ) : isIgnoreKey ? (
                <Tag color='violet' size='small'>
                  按模型分组
                </Tag>
              ) : null}
            </div>
          </div>
        }
        bodyStyle={{ padding: 0 }}
      >
        <Spin spinning={loading}>
          <div className='px-2 pt-2'>
            <Tabs
              type='card'
              activeKey={metric}
              onChange={(key) => setMetric(key)}
            >
              {METRICS.map((m) => (
                <Tabs.TabPane tab={m.label} itemKey={m.key} />
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

export default ModelDashboard;
