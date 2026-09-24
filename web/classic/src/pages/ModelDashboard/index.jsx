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
  DatePicker,
  Empty,
  Radio,
  RadioGroup,
  Select,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { Activity } from 'lucide-react';
import { API, showError } from '../../helpers';

const { Text, Title } = Typography;
const CHART_CONFIG = { mode: 'desktop-browser' };
const CHART_HEIGHT = 360;

const RANGE_OPTIONS = [
  { label: '最近 30 分钟', value: 30 },
  { label: '最近 10 分钟', value: 10 },
  { label: '最近 1 小时', value: 60 },
  { label: '最近 2 小时', value: 120 },
  { label: '最近 4 小时', value: 240 },
  { label: '最近 8 小时', value: 480 },
];

const REFRESH_OPTIONS = [
  { label: '慢 (1分钟)', value: 60 },
  { label: '中 (30秒)', value: 30 },
  { label: '快 (10秒)', value: 10 },
];

const METRICS = [
  { key: 'running', label: '正在运行任务数', axis: '任务数', unit: '个', decimals: 0 },
  { key: 'waiting', label: '排队任务数', axis: '任务数', unit: '个', decimals: 0 },
  { key: 'queue_avg_seconds', label: '平均排队耗时', axis: '秒', unit: 's', decimals: 3 },
  { key: 'queue_p95_seconds', label: '排队耗时 P95', axis: '秒', unit: 's', decimals: 3 },
  { key: 'prefill_tokens_per_sec', label: 'Prefill 吞吐', axis: 'Token/秒', unit: 'tok/s', decimals: 1 },
  { key: 'decode_tokens_per_sec', label: 'Decode 吞吐', axis: 'Token/秒', unit: 'tok/s', decimals: 1 },
  { key: 'prefix_cache_hit_rate', label: 'Prefix Cache 命中率', axis: '百分比', unit: '%', decimals: 1 },
  { key: 'kv_cache_usage', label: 'KV Cache 使用率', axis: '百分比', unit: '%', decimals: 1 },
  { key: 'ttft_p95_seconds', label: 'TTFT P95', axis: '秒', unit: 's', decimals: 3 },
  { key: 'e2e_p95_seconds', label: 'E2E 时延 P95', axis: '秒', unit: 's', decimals: 3 },
  { key: 'request_per_sec', label: '完成请求速率', axis: '请求/秒', unit: 'req/s', decimals: 2 },
  { key: 'error_rate', label: '错误率', axis: '百分比', unit: '%', decimals: 2 },
  { key: 'preemptions_per_min', label: '抢占速率', axis: '次/分钟', unit: '次/min', decimals: 2 },
];

const DEFAULT_RANGE_MINUTES = 30;
const DEFAULT_REFRESH = 30;
const DEFAULT_METRIC = 'running';

const formatNumber = (value, digits = 1) => {
  const n = Number(value || 0);
  return n.toLocaleString(undefined, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
};

const formatDuration = (seconds) => {
  const value = Number(seconds || 0);
  if (value > 0 && value < 1) return `${Math.round(value * 1000)} ms`;
  return `${formatNumber(value, 2)} s`;
};

// 与 formatDuration 同义，但把数值与单位拆开，便于卡片用小字号渲染单位。
const formatDurationParts = (seconds) => {
  const value = Number(seconds || 0);
  if (value > 0 && value < 1) return [Math.round(value * 1000), 'ms'];
  return [formatNumber(value, 2), 's'];
};

const formatStep = (seconds) => {
  const value = Number(seconds || 0);
  if (value < 60) return `${value} 秒`;
  if (value < 3600) return `${Math.round(value / 60)} 分钟`;
  if (value < 86400) return `${Number(value / 3600).toFixed(value % 3600 === 0 ? 0 : 1)} 小时`;
  return `${Number(value / 86400).toFixed(value % 86400 === 0 ? 0 : 1)} 天`;
};

const formatTimestamp = (timestamp, historical, rangeSeconds) => {
  const d = new Date(Number(timestamp) * 1000);
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  if (!historical) return `${hh}:${mm}:${ss}`;

  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  if (rangeSeconds > 90 * 86400) {
    return `${d.getFullYear()}-${month}-${day}`;
  }
  return `${month}-${day} ${hh}:${mm}`;
};

const MetricCard = ({ title, value, unit, sub, grow = 1, minWidth = 165 }) => (
  <Card style={{ flex: `${grow} 1 0`, minWidth }}>
    <div style={{ textAlign: 'center' }}>
      <Text type='tertiary' size='small'>{title}</Text>
      <div
        style={{
          fontSize: 26,
          fontWeight: 700,
          margin: '8px 0 4px',
          whiteSpace: 'nowrap',
        }}
      >
        {value}
        {unit ? (
          <span
            style={{
              fontSize: 13,
              fontWeight: 400,
              marginLeft: unit === '%' ? 0 : 3,
              color: 'var(--semi-color-text-2)',
            }}
          >
            {unit}
          </span>
        ) : null}
      </div>
      <Text type='tertiary' size='small'>{sub}</Text>
    </div>
  </Card>
);

const ModelDashboard = () => {
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState('realtime');
  const [rangeMinutes, setRangeMinutes] = useState(DEFAULT_RANGE_MINUTES);
  const [dateRange, setDateRange] = useState([]);
  const [refreshInterval, setRefreshInterval] = useState(DEFAULT_REFRESH);
  const [metricKey, setMetricKey] = useState(DEFAULT_METRIC);
  const [modelName, setModelName] = useState('');
  const [data, setData] = useState(null);
  const [lastUpdated, setLastUpdated] = useState(null);
  const refreshTimerRef = useRef(null);
  const requestSeqRef = useRef(0);

  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  const historical = mode === 'historical';

  const loadData = useCallback(async () => {
    if (historical && (!dateRange || dateRange.length !== 2)) {
      setData(null);
      return;
    }

    const requestSeq = ++requestSeqRef.current;
    setLoading(true);
    try {
      let params;
      if (historical) {
        params = {
          start_timestamp: Math.floor(dateRange[0].getTime() / 1000),
          end_timestamp: Math.floor(dateRange[1].getTime() / 1000),
        };
      } else {
        params = { minutes: rangeMinutes };
      }

      const res = await API.get('/api/model-dashboard/metrics', { params });
      if (!res.data.success) {
        throw new Error(res.data.message || '获取模型运行指标失败');
      }
      if (requestSeq !== requestSeqRef.current) return;

      setData(res.data.data || null);
      setLastUpdated(new Date());
    } catch (error) {
      if (requestSeq === requestSeqRef.current) {
        showError(error.message || '获取模型运行指标失败');
      }
    } finally {
      if (requestSeq === requestSeqRef.current) setLoading(false);
    }
  }, [historical, dateRange, rangeMinutes]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  useEffect(() => {
    if (refreshTimerRef.current) {
      clearInterval(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }
    if (!historical) {
      refreshTimerRef.current = setInterval(loadData, refreshInterval * 1000);
    }
    return () => {
      if (refreshTimerRef.current) clearInterval(refreshTimerRef.current);
    };
  }, [historical, refreshInterval, loadData]);

  const summary = data?.summary || [];
  const modelOptions = useMemo(() => {
    const values = data?.model_names || [];
    return [
      { label: '全部模型', value: '' },
      ...values.map((name) => ({ label: name, value: name })),
    ];
  }, [data]);

  useEffect(() => {
    if (modelName && !(data?.model_names || []).includes(modelName)) {
      setModelName('');
    }
  }, [data, modelName]);

  const overview = useMemo(() => {
    const rows = summary || [];
    const total = (field) => rows.reduce((sum, item) => sum + Number(item[field] || 0), 0);
    const max = (field) => rows.reduce((value, item) => Math.max(value, Number(item[field] || 0)), 0);
    const average = (field) => {
      if (!rows.length) return 0;
      return total(field) / rows.length;
    };
    const queueP95 = formatDurationParts(max('queue_p95_seconds'));
    return {
      models: rows.length,
      running: total('running'),
      waiting: total('waiting'),
      prefill: total('prefill_tokens_per_sec'),
      decode: total('decode_tokens_per_sec'),
      queueP95Value: queueP95[0],
      queueP95Unit: queueP95[1],
      prefixAverage: average('prefix_cache_hit_rate'),
      kvMax: max('kv_cache_usage'),
    };
  }, [summary]);

  const summaryColumns = useMemo(() => [
    {
      title: '模型',
      dataIndex: 'model_name',
      key: 'model_name',
      width: 190,
      fixed: 'left',
      render: (value) => <Tag color='blue'>{value}</Tag>,
    },
    { title: '运行中', dataIndex: 'running', key: 'running', width: 80, render: (v) => Math.round(Number(v || 0)) },
    { title: '排队', dataIndex: 'waiting', key: 'waiting', width: 70, render: (v) => Math.round(Number(v || 0)) },
    { title: 'Queue Avg', dataIndex: 'queue_avg_seconds', key: 'queue_avg_seconds', width: 105, render: formatDuration },
    { title: 'Queue P95', dataIndex: 'queue_p95_seconds', key: 'queue_p95_seconds', width: 105, render: formatDuration },
    { title: 'Prefill', dataIndex: 'prefill_tokens_per_sec', key: 'prefill_tokens_per_sec', width: 115, render: (v) => `${formatNumber(v, 1)} tok/s` },
    { title: 'Decode', dataIndex: 'decode_tokens_per_sec', key: 'decode_tokens_per_sec', width: 115, render: (v) => `${formatNumber(v, 1)} tok/s` },
    { title: 'Prefix命中率', dataIndex: 'prefix_cache_hit_rate', key: 'prefix_cache_hit_rate', width: 110, render: (v) => `${formatNumber(v, 1)}%` },
    { title: 'KV Cache', dataIndex: 'kv_cache_usage', key: 'kv_cache_usage', width: 100, render: (v) => `${formatNumber(v, 1)}%` },
    { title: 'TTFT P95', dataIndex: 'ttft_p95_seconds', key: 'ttft_p95_seconds', width: 100, render: formatDuration },
    { title: 'E2E P95', dataIndex: 'e2e_p95_seconds', key: 'e2e_p95_seconds', width: 100, render: formatDuration },
    { title: '请求速率', dataIndex: 'request_per_sec', key: 'request_per_sec', width: 105, render: (v) => `${formatNumber(v, 2)} req/s` },
    { title: '错误率', dataIndex: 'error_rate', key: 'error_rate', width: 90, render: (v) => `${formatNumber(v, 2)}%` },
    { title: '抢占', dataIndex: 'preemptions_per_min', key: 'preemptions_per_min', width: 95, render: (v) => `${formatNumber(v, 2)}/min` },
  ], []);

  const metricMeta = METRICS.find((item) => item.key === metricKey) || METRICS[0];
  const rangeSeconds = data ? Number(data.end_timestamp || 0) - Number(data.start_timestamp || 0) : 0;

  const chartData = useMemo(() => {
    const metricSeries = data?.metrics?.[metricKey] || [];
    const rows = [];
    metricSeries.forEach((series) => {
      if (modelName && series.model_name !== modelName) return;
      (series.points || []).forEach((point) => {
        rows.push({
          time: formatTimestamp(point.timestamp, historical, rangeSeconds),
          timestamp: point.timestamp,
          model: series.model_name,
          value: Number(point.value || 0),
        });
      });
    });
    rows.sort((a, b) => Number(a.timestamp) - Number(b.timestamp));
    return rows;
  }, [data, metricKey, modelName, historical, rangeSeconds]);

  const chartSpec = useMemo(() => ({
    type: 'line',
    data: [{ id: 'modelRuntime', values: chartData }],
    xField: 'time',
    yField: 'value',
    seriesField: 'model',
    stack: false,
    smooth: false,
    point: { visible: chartData.length <= 80 },
    axes: [
      {
        orient: 'bottom',
        title: { visible: true, text: historical ? '时间' : '时间 (HH:mm:ss)' },
        label: { style: { fontSize: 10 } },
      },
      {
        orient: 'left',
        title: { visible: true, text: `${metricMeta.axis}${metricMeta.unit ? ` (${metricMeta.unit})` : ''}` },
        label: { style: { fontSize: 10 } },
      },
    ],
    legends: { visible: true, position: 'top' },
    tooltip: { visible: true },
    padding: { top: 10, bottom: 38, left: 72, right: 20 },
    height: CHART_HEIGHT,
  }), [chartData, historical, metricMeta]);

  return (
    <div className='mt-[60px] px-2'>
      <Spin spinning={loading}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Activity size={24} />
            <Title heading={4} style={{ margin: 0 }}>模型看板</Title>
            <Tag color='green'>Prometheus / vLLM</Tag>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <RadioGroup type='button' value={mode} onChange={(e) => setMode(e.target.value)}>
              <Radio value='realtime'>实时</Radio>
              <Radio value='historical'>历史</Radio>
            </RadioGroup>

            {historical ? (
              <DatePicker
                type='dateTimeRange'
                value={dateRange}
                onChange={(value) => setDateRange(value || [])}
                placeholder={['开始时间', '结束时间']}
                style={{ width: 330 }}
              />
            ) : (
              <Select
                value={rangeMinutes}
                onChange={setRangeMinutes}
                optionList={RANGE_OPTIONS}
                style={{ width: 130 }}
              />
            )}

            <Select
              value={modelName}
              onChange={setModelName}
              optionList={modelOptions}
              filter
              style={{ width: 210 }}
              placeholder='选择模型'
            />

            {!historical && (
              <Select
                value={refreshInterval}
                onChange={setRefreshInterval}
                optionList={REFRESH_OPTIONS}
                style={{ width: 130 }}
              />
            )}
          </div>
        </div>

        {lastUpdated && (
          <div style={{ marginBottom: 12 }}>
            <Text type='tertiary' size='small'>
              最近更新：{lastUpdated.toLocaleTimeString('zh-CN', { hour12: false })}
              {data?.step_seconds ? ` ｜ 图表采样：${formatStep(data.step_seconds)}` : ''}
              {data?.rate_window_seconds ? ` ｜ Rate窗口：${formatStep(data.rate_window_seconds)}` : ''}
            </Text>
          </div>
        )}

        {!data ? (
          <Empty description={historical ? '请选择历史查询时间范围' : '暂无模型运行数据'} />
        ) : (
          <>
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 16 }}>
              <MetricCard title='模型数' value={overview.models} sub='当前有指标的模型' />
              <MetricCard title='运行任务' value={Math.round(overview.running)} sub='全部模型合计' />
              <MetricCard title='排队任务' value={Math.round(overview.waiting)} sub='全部模型合计' />
              <MetricCard
                title='Prefill 吞吐'
                value={formatNumber(overview.prefill, 1)}
                unit='tok/s'
                grow={1.5}
                minWidth={215}
                sub='全部模型合计'
              />
              <MetricCard
                title='Decode 吞吐'
                value={formatNumber(overview.decode, 1)}
                unit='tok/s'
                grow={1.5}
                minWidth={215}
                sub='全部模型合计'
              />
              <MetricCard
                title='最大 Queue P95'
                value={overview.queueP95Value}
                unit={overview.queueP95Unit}
                sub='当前最慢模型'
              />
              <MetricCard
                title='平均 Prefix 命中率'
                value={formatNumber(overview.prefixAverage, 1)}
                unit='%'
                sub='模型简单平均'
              />
              <MetricCard
                title='最高 KV Cache'
                value={formatNumber(overview.kvMax, 1)}
                unit='%'
                sub='当前最高模型'
              />
            </div>

            <Card style={{ marginBottom: 16 }} title='模型当前运行状态'>
              <Table
                columns={summaryColumns}
                dataSource={summary}
                rowKey='model_name'
                pagination={false}
                size='small'
                scroll={{ x: 1550 }}
              />
            </Card>

            <Card
              title={
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                  <span>模型运行趋势</span>
                  <Select
                    value={metricKey}
                    onChange={setMetricKey}
                    optionList={METRICS.map((item) => ({ label: item.label, value: item.key }))}
                    style={{ width: 210 }}
                  />
                  <Tag color='blue'>{metricMeta.unit}</Tag>
                  {modelName && <Tag color='cyan'>{modelName}</Tag>}
                </div>
              }
            >
              {chartData.length ? (
                <VChart spec={chartSpec} option={CHART_CONFIG} />
              ) : (
                <div style={{ height: CHART_HEIGHT, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Empty description='当前时间范围没有该指标数据' />
                </div>
              )}
            </Card>
          </>
        )}
      </Spin>
    </div>
  );
};

export default ModelDashboard;
