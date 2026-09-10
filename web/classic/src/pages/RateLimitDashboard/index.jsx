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
  Input,
  DatePicker,
  RadioGroup,
  Radio,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { Gauge } from 'lucide-react';
import { API, showError } from '../../helpers';

const { Text } = Typography;

const CHART_CONFIG = { mode: 'desktop-browser' };
const REFRESH_INTERVAL_MS = 10000;

const COLOR_RATE_LIMITED = '#ef4444';
const COLOR_NORMAL = '#3b82f6';

const PERIOD_VALUES = [10, 20, 40, 80];
const REALTIME_PERIOD_OPTIONS = PERIOD_VALUES.map((value) => ({
  label: `最近 ${value} 周期`,
  value,
}));
const HISTORICAL_PERIOD_OPTIONS = PERIOD_VALUES.map((value) => ({
  label: `${value} 个周期`,
  value,
}));

// 刷新频率选项
const REFRESH_OPTIONS = [
  { label: '低 (1分)', value: 60 },
  { label: '中 (30秒)', value: 30 },
  { label: '高 (10秒)', value: 10 },
];
const DEFAULT_REFRESH = 30;
const DEFAULT_PERIODS = 10;

// 周期标签：实时模式显示“前 N 周期”，历史模式显示周期开始时间。
function periodLabel(periodIndex, startTimestamp, isHistorical) {
  if (isHistorical && startTimestamp) {
    const d = new Date(startTimestamp * 1000);
    const mmd = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    const hh = String(d.getHours()).padStart(2, '0');
    const mm = String(d.getMinutes()).padStart(2, '0');
    return `${mmd}-${dd} ${hh}:${mm}`;
  }
  if (periodIndex === 0) return '当前周期';
  return `前 ${periodIndex} 周期`;
}

const RateLimitDashboard = () => {
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState('realtime');
  const [periods, setPeriods] = useState(DEFAULT_PERIODS);
  const [historyStart, setHistoryStart] = useState(null);
  const [historyPeriods, setHistoryPeriods] = useState(DEFAULT_PERIODS);
  const [chartData, setChartData] = useState([]);
  const [durationMinutes, setDurationMinutes] = useState(10);
  const [summary, setSummary] = useState({ total: 0, limited: 0 });
  const [filterKey, setFilterKey] = useState('');
  const [filterAccount, setFilterAccount] = useState('');
  const [refreshInterval, setRefreshInterval] = useState(DEFAULT_REFRESH);
  const refreshTimerRef = useRef(null);

  const isHistorical = mode === 'historical';
  const selectedPeriodCount = isHistorical ? historyPeriods : periods;

  const keyOptions = useMemo(() => {
    const set = new Set();
    chartData.forEach((r) => {
      if (r.tokenName) set.add(r.tokenName);
    });
    const opts = [{ label: '全部 Key', value: '' }];
    [...set].sort().forEach((k) => opts.push({ label: k, value: k }));
    return opts;
  }, [chartData]);

  const accountFilterKeyword = filterAccount.trim().toLowerCase();
  const accountFilterTooShort =
    accountFilterKeyword.length > 0 && accountFilterKeyword.length < 7;

  const filteredChartData = useMemo(() => {
    const kw = filterAccount.trim().toLowerCase();
    if (kw.length > 0 && kw.length < 7) {
      return [];
    }
    return chartData.filter((r) => {
      if (filterKey && r.tokenName !== filterKey) return false;
      if (kw.length >= 7 && !(r.account || '').toLowerCase().includes(kw)) return false;
      return true;
    });
  }, [chartData, filterKey, filterAccount]);

  const filteredSeriesCount = useMemo(
    () => new Set(filteredChartData.map((r) => r.key)).size,
    [filteredChartData],
  );

  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  const handleModeChange = useCallback(
    (e) => {
      const nextMode = e.target.value;
      if (nextMode === 'historical' && !historyStart) {
        // 第一次进入历史模式时给一个合理默认开始时间：当前时间向前 N 个限流周期。
        // 用户仍可直接修改开始时间；结束时间始终由服务端按周期数推导。
        setHistoryStart(
          new Date(
            Date.now() -
              historyPeriods * Math.max(1, durationMinutes) * 60 * 1000,
          ),
        );
      }
      setMode(nextMode);
    },
    [historyStart, historyPeriods, durationMinutes],
  );

  // 拉取数据。实时模式只传 periods；历史模式只传 start_timestamp + periods。
  const loadData = useCallback(async () => {
    if (isHistorical && !historyStart) {
      setChartData([]);
      setSummary({ total: 0, limited: 0 });
      return;
    }

    setLoading(true);
    try {
      const params = isHistorical
        ? {
            start_timestamp: Math.floor(historyStart.getTime() / 1000),
            periods: historyPeriods,
          }
        : { periods };

      const res = await API.get('/api/log/rate_limit_dashboard', { params });
      if (!res.data.success) {
        throw new Error(res.data.message || '获取限流看板数据失败');
      }

      const data = res.data.data;
      // 服务端实时模式为了保留 period_index=0 表示“当前周期”，返回顺序是新到旧；
      // 历史模式则从开始时间向后。这里统一按开始时间排序，保证图表左旧右新。
      const periodList = [...(data.periods || [])].sort(
        (a, b) => (a.start_timestamp || 0) - (b.start_timestamp || 0),
      );
      const dm = data.duration_minutes || 10;
      setDurationMinutes(dm);

      const rows = [];
      const limitedSet = new Set();
      const keys = new Set();

      periodList.forEach((p) => {
        (p.items || []).forEach((item) => {
          const key = `${item.token_name}||${item.account}`;
          keys.add(key);
          if (item.rate_limited) limitedSet.add(key);
        });
      });

      periodList.forEach((p) => {
        const label = periodLabel(p.period_index, p.start_timestamp, isHistorical);
        const map = new Map();
        (p.items || []).forEach((item) => {
          map.set(`${item.token_name}||${item.account}`, item);
        });
        keys.forEach((key) => {
          const item = map.get(key);
          const [tokenName, account] = key.split('||');
          const displayToken = tokenName || '(未命名Token)';
          const displayAccount = account || '(空账号)';
          rows.push({
            period: label,
            periodIndex: p.period_index,
            key,
            name: `${displayToken} / ${displayAccount}`,
            tokenName,
            account,
            count: item ? item.count : 0,
            successLimit: item ? item.success_limit : 0,
            rateLimited: item ? item.rate_limited : false,
          });
        });
      });

      setChartData(rows);
      setSummary({ total: keys.size, limited: limitedSet.size });
    } catch (e) {
      showError(e.message || '获取限流看板数据失败');
    } finally {
      setLoading(false);
    }
  }, [periods, isHistorical, historyStart, historyPeriods]);

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
      }, refreshInterval * 1000);
    }
    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
      }
    };
  }, [isHistorical, loadData, refreshInterval]);

  const spec = useMemo(() => {
    const colorMap = {};
    filteredChartData.forEach((row) => {
      if (row.rateLimited) {
        colorMap[row.name] = COLOR_RATE_LIMITED;
      } else if (!(row.name in colorMap)) {
        colorMap[row.name] = COLOR_NORMAL;
      }
    });

    return {
      type: 'line',
      data: [{ id: 'rateLimitData', values: filteredChartData }],
      xField: 'period',
      yField: 'count',
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
        text: 'Token名 / Account 请求数趋势',
        subtext: `限流周期：${durationMinutes} 分钟 ｜ 查询：${selectedPeriodCount} 个周期${isHistorical ? ' ｜ 历史数据' : ''}`,
      },
      line: { style: { lineWidth: 2 } },
      point: {
        visible: true,
        size: (datum) => {
          const d = datum && typeof datum === 'object' ? datum : {};
          return d.rateLimited ? 7 : 4;
        },
        style: {
          fill: (datum) => {
            const d = datum && typeof datum === 'object' ? datum : {};
            return d.rateLimited ? COLOR_RATE_LIMITED : COLOR_NORMAL;
          },
          stroke: '#fff',
          lineWidth: 1,
        },
      },
      label: {
        visible: true,
        position: 'top',
        style: {
          fontSize: 11,
          fill: (text, datum) => {
            const d = datum && typeof datum === 'object' ? datum : {};
            return d.rateLimited ? COLOR_RATE_LIMITED : '#666';
          },
          fontWeight: (text, datum) => {
            const d = datum && typeof datum === 'object' ? datum : {};
            return d.rateLimited ? 'bold' : 'normal';
          },
        },
        formatMethod: (text, datum) => {
          const d = datum && typeof datum === 'object' ? datum : {};
          const count = d.count ?? (typeof text === 'number' ? text : 0);
          const limit = d.successLimit ?? 0;
          const rateLimited = d.rateLimited ?? false;
          if (count <= 0 && !rateLimited) return '';
          if (limit > 0) return `${count}/${limit}${rateLimited ? ' ⚠' : ''}`;
          return `${count}`;
        },
      },
      axes: [
        { orient: 'left', title: { visible: true, text: '完成请求数' } },
        {
          orient: 'bottom',
          title: { visible: true, text: isHistorical ? '周期时间' : '周期' },
        },
      ],
      tooltip: {
        mark: {
          content: [
            { key: 'Token名', value: (datum) => datum?.tokenName || '-' },
            { key: 'Account', value: (datum) => datum?.account || '-' },
            { key: '请求数', value: (datum) => `${datum?.count ?? 0}` },
            {
              key: '限流上限',
              value: (datum) =>
                datum?.successLimit > 0 ? `${datum.successLimit}` : '未配置',
            },
            {
              key: '状态',
              value: (datum) => (datum?.rateLimited ? '已限流 ⚠' : '正常'),
            },
          ],
        },
      },
      color: { specified: colorMap },
    };
  }, [filteredChartData, durationMinutes, isHistorical, selectedPeriodCount]);

  return (
    <div className='mt-[60px] px-2'>
      <Card
        className='!rounded-2xl'
        title={
          <div className='flex flex-col w-full gap-3'>
            <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between w-full gap-3'>
              <div className='flex items-center gap-2'>
                <Gauge size={16} />
                <span>限流看板</span>
                <Tag color={summary.limited > 0 ? 'red' : 'green'} size='small'>
                  被限流 {summary.limited} / 共 {summary.total}
                </Tag>
              </div>
              <div className='flex flex-wrap items-center gap-2'>
                <RadioGroup type='button' value={mode} onChange={handleModeChange}>
                  <Radio value='realtime'>实时</Radio>
                  <Radio value='historical'>历史</Radio>
                </RadioGroup>
                {isHistorical ? (
                  <>
                    <DatePicker
                      type='dateTime'
                      value={historyStart}
                      onChange={(v) => setHistoryStart(v || null)}
                      placeholder='开始时间'
                      style={{ width: 200 }}
                    />
                    <Select
                      value={historyPeriods}
                      onChange={(v) => setHistoryPeriods(v)}
                      optionList={HISTORICAL_PERIOD_OPTIONS}
                      style={{ width: 130 }}
                    />
                  </>
                ) : (
                  <>
                    <Select
                      value={periods}
                      onChange={(v) => setPeriods(v)}
                      optionList={REALTIME_PERIOD_OPTIONS}
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
                {isHistorical ? (
                  <Tag color='grey' size='small'>
                    从开始时间连续 {historyPeriods} 周期
                  </Tag>
                ) : (
                  <Tag color='blue' size='small'>自动刷新中</Tag>
                )}
              </div>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <Text type='tertiary' size='small'>筛选</Text>
              <Select
                value={filterKey}
                onChange={(v) => setFilterKey(v || '')}
                optionList={keyOptions}
                placeholder='选择 Key'
                style={{ width: 180 }}
              />
              <Input
                value={filterAccount}
                onChange={(v) => setFilterAccount(v)}
                placeholder='Account 模糊匹配（至少7位）'
                showClear
                style={{ width: 220 }}
              />
              {accountFilterTooShort ? (
                <Tag color='orange' size='small'>
                  至少输入 7 位 Account 后才执行筛选
                </Tag>
              ) : filterKey || filterAccount ? (
                <Tag color='light-blue' size='small'>
                  命中 {filteredSeriesCount} 条曲线
                </Tag>
              ) : null}
            </div>
          </div>
        }
        bodyStyle={{ padding: 0 }}
      >
        <Spin spinning={loading}>
          <div className='h-[480px] p-2'>
            {accountFilterTooShort ? (
              <Empty
                title='请输入至少 7 位 Account'
                description='输入不足 7 位时不会执行 Account 筛选'
                style={{ padding: 80 }}
              />
            ) : filteredChartData.length > 0 ? (
              <VChart spec={spec} option={CHART_CONFIG} />
            ) : chartData.length > 0 ? (
              <Empty
                title='无匹配数据'
                description='当前筛选条件下没有命中的曲线'
                style={{ padding: 80 }}
              />
            ) : (
              <Empty
                title='暂无数据'
                description='当前周期内没有消费日志'
                style={{ padding: 80 }}
              />
            )}
          </div>
        </Spin>
      </Card>
    </div>
  );
};

export default RateLimitDashboard;
