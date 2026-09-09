import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Card,
  Select,
  Spin,
  Typography,
  Table,
  Button,
  Tag,
  Modal,
  Empty,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { Database, RefreshCw, AlertTriangle } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

const CHART_CONFIG = { mode: 'desktop-browser' };

// 刷新频率选项
const REFRESH_OPTIONS = [
  { label: '低 (1分)', value: 60 },
  { label: '中 (30秒)', value: 30 },
  { label: '高 (10秒)', value: 10 },
  { label: '不刷新', value: 0 },
];
const DEFAULT_REFRESH = 30;

const formatBytes = (bytes) => {
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(2) + ' GB';
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(2) + ' MB';
  if (bytes >= 1024) return (bytes / 1024).toFixed(2) + ' KB';
  return bytes + ' B';
};

const formatNum = (n) => {
  if (n === undefined || n === null) return '-';
  return n.toLocaleString();
};

// 图表统一高度
const CHART_HEIGHT = 280;

const MySQLDashboard = () => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState(null);
  const [refreshInterval, setRefreshInterval] = useState(DEFAULT_REFRESH);
  const [chartHistory, setChartHistory] = useState([]); // 累积历史数据
  const [killModal, setKillModal] = useState({ visible: false, id: null });
  const refreshTimerRef = useRef(null);
  const prevCountersRef = useRef(null); // 上次累计值

  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  // 拉取数据
  const loadData = useCallback(async () => {
    try {
      const res = await API.get('/api/log/mysql_monitor');
      if (!res.data.success) {
        throw new Error(res.data.message || '获取 MySQL 监控数据失败');
      }
      const d = res.data.data;
      setData(d);

      // 计算速率并累积历史
      const now = d.timestamp;
      const prev = prevCountersRef.current;
      if (prev && now > prev.timestamp) {
        const dt = now - prev.timestamp; // 秒
        const qps = dt > 0 ? (d.questions - prev.questions) / dt : 0;
        const tps = dt > 0 ? ((d.com_commit + d.com_rollback) - (prev.com_commit + prev.com_rollback)) / dt : 0;
        const bytesIn = dt > 0 ? (d.bytes_received - prev.bytes_received) / dt : 0;
        const bytesOut = dt > 0 ? (d.bytes_sent - prev.bytes_sent) / dt : 0;

        setChartHistory((h) => {
          const entry = { ts: now, qps, tps, bytesIn, bytesOut };
          const next = [...h, entry];
          // 保留最近 60 个点
          if (next.length > 60) next.shift();
          return next;
        });
      }
      prevCountersRef.current = {
        timestamp: now,
        questions: d.questions,
        com_commit: d.com_commit,
        com_rollback: d.com_rollback,
        bytes_received: d.bytes_received,
        bytes_sent: d.bytes_sent,
      };
    } catch (e) {
      showError(e.message || '获取 MySQL 监控数据失败');
    } finally {
      setLoading(false);
    }
  }, []);

  // 页面打开时首次加载
  useEffect(() => {
    setLoading(true);
    loadData();
  }, [loadData]);

  // 定时刷新
  useEffect(() => {
    if (refreshTimerRef.current) {
      clearInterval(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }
    if (refreshInterval > 0) {
      refreshTimerRef.current = setInterval(() => {
        loadData();
      }, refreshInterval * 1000);
    }
    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
      }
    };
  }, [refreshInterval, loadData]);

  // KILL 操作
  const doKill = async () => {
    const { id } = killModal;
    if (!id) return;
    try {
      const res = await API.post(`/api/log/mysql_kill?id=${id}`);
      if (res.data.success) {
        showSuccess(res.data.message || 'KILL 成功');
        loadData();
      } else {
        showError(res.data.message || 'KILL 失败');
      }
    } catch (e) {
      showError('KILL 失败');
    } finally {
      setKillModal({ visible: false, id: null });
    }
  };

  // 连接使用率
  const connUsage = data && data.max_connections > 0
    ? ((data.threads_connected / data.max_connections) * 100).toFixed(1)
    : '-';

  const slowQueryLogStatus = data?.slow_query_log_status || 'UNKNOWN';
  const slowQueryLogEnabled = data?.slow_query_log_enabled === true;
  const longQueryTimeValue = Number(data?.long_query_time);
  const longQueryTime = Number.isFinite(longQueryTimeValue) ? longQueryTimeValue : 10;
  const longQueryTimeLabel = longQueryTime.toLocaleString(undefined, {
    maximumFractionDigits: 3,
  });

  // 实时折线图 spec - QPS & TPS
  const qpsTpsSpec = useMemo(() => {
    const rows = chartHistory.map((h) => ({
      time: new Date(h.ts * 1000).toLocaleTimeString('zh-CN', { hour12: false }),
      QPS: Math.round(h.qps * 10) / 10,
      TPS: Math.round(h.tps * 10) / 10,
    }));
    return {
      type: 'line',
      data: [{ id: 'data', values: rows }],
      xField: 'time',
      yField: ['QPS', 'TPS'],
      series: [
        { field: 'QPS', type: 'line', line: { style: { lineDash: [] } } },
        { field: 'TPS', type: 'line', line: { style: { lineDash: [4, 4] } } },
      ],
      axes: [
        {
          orient: 'bottom',
          title: { visible: true, text: '时间 (HH:mm:ss)', style: { fontSize: 11 } },
          label: { style: { fontSize: 10 } },
        },
        {
          orient: 'left',
          title: { visible: true, text: '速率 (次/秒)', style: { fontSize: 11 } },
          label: { style: { fontSize: 10 } },
        },
      ],
      legends: { visible: true, position: 'top' },
      tooltip: { visible: true },
      padding: { top: 8, bottom: 36, left: 64, right: 16 },
      height: CHART_HEIGHT,
    };
  }, [chartHistory]);

  // 实时折线图 spec - 网络流入/流出
  const netSpec = useMemo(() => {
    const rows = chartHistory.map((h) => ({
      time: new Date(h.ts * 1000).toLocaleTimeString('zh-CN', { hour12: false }),
      '流入(KB/s)': Math.round(h.bytesIn / 1024 * 10) / 10,
      '流出(KB/s)': Math.round(h.bytesOut / 1024 * 10) / 10,
    }));
    return {
      type: 'line',
      data: [{ id: 'data', values: rows }],
      xField: 'time',
      yField: ['流入(KB/s)', '流出(KB/s)'],
      series: [
        { field: '流入(KB/s)', type: 'line', line: { style: { lineDash: [] } } },
        { field: '流出(KB/s)', type: 'line', line: { style: { lineDash: [4, 4] } } },
      ],
      axes: [
        {
          orient: 'bottom',
          title: { visible: true, text: '时间 (HH:mm:ss)', style: { fontSize: 11 } },
          label: { style: { fontSize: 10 } },
        },
        {
          orient: 'left',
          title: { visible: true, text: '吞吐量 (KB/s)', style: { fontSize: 11 } },
          label: { style: { fontSize: 10 } },
        },
      ],
      legends: { visible: true, position: 'top' },
      tooltip: { visible: true },
      padding: { top: 8, bottom: 36, left: 72, right: 16 },
      height: CHART_HEIGHT,
    };
  }, [chartHistory]);

  // 慢查询表格列
  const slowQueryColumns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 90 },
    { title: 'User', dataIndex: 'user', key: 'user', width: 100 },
    { title: 'Host', dataIndex: 'host', key: 'host', width: 160 },
    { title: 'DB', dataIndex: 'db', key: 'db', width: 120 },
    { title: 'Command', dataIndex: 'command', key: 'command', width: 90,
      render: (v) => <Tag color='blue' size='small'>{v}</Tag> },
    { title: 'Time(s)', dataIndex: 'time', key: 'time', width: 80,
      render: (v) => v > 60 ? <Text type='danger' strong>{v}</Text> : <Text type='warning'>{v}</Text> },
    { title: 'State', dataIndex: 'state', key: 'state', width: 120 },
    { title: 'Info', dataIndex: 'info', key: 'info',
      render: (v) => v
        ? <Text style={{ fontFamily: 'monospace', fontSize: 12 }} ellipsis={{ showTooltip: true }}>{v.length > 200 ? v.slice(0, 200) + '...' : v}</Text>
        : '-' },
    { title: '操作', key: 'action', width: 80, fixed: 'right',
      render: (_, record) => (
        <Button
          size='small'
          type='danger'
          onClick={() => setKillModal({ visible: true, id: record.id })}
        >
          Kill
        </Button>
      ),
    },
  ];

  // 锁等待表格列
  const lockWaitColumns = [
    { title: '等待 PID', dataIndex: 'waiting_pid', key: 'waiting_pid', width: 90 },
    { title: '等待 SQL', dataIndex: 'waiting_query', key: 'waiting_query',
      render: (v) => v
        ? <Text style={{ fontFamily: 'monospace', fontSize: 12 }} ellipsis={{ showTooltip: true }}>{v.length > 200 ? v.slice(0, 200) + '...' : v}</Text>
        : '-' },
    { title: '等待模式', dataIndex: 'waiting_mode', key: 'waiting_mode', width: 100 },
    { title: '阻塞 PID', dataIndex: 'blocking_pid', key: 'blocking_pid', width: 90 },
    { title: '阻塞 SQL', dataIndex: 'blocking_query', key: 'blocking_query',
      render: (v) => v
        ? <Text style={{ fontFamily: 'monospace', fontSize: 12 }} ellipsis={{ showTooltip: true }}>{v.length > 200 ? v.slice(0, 200) + '...' : v}</Text>
        : '-' },
    { title: '阻塞模式', dataIndex: 'blocking_mode', key: 'blocking_mode', width: 100 },
    { title: '等待时间(s)', dataIndex: 'wait_time', key: 'wait_time', width: 100,
      render: (v) => v > 30 ? <Text type='danger' strong>{v}</Text> : v },
  ];

  // 指标卡片
  const MetricCard = ({ title, value, sub, color }) => (
    <Card style={{ flex: 1, minWidth: 180 }}>
      <div style={{ textAlign: 'center' }}>
        <Text type='tertiary' size='small'>{title}</Text>
        <div style={{ fontSize: 28, fontWeight: 700, color: color || '#1f2937', margin: '8px 0 4px' }}>
          {value}
        </div>
        {sub && <Text type='tertiary' size='small'>{sub}</Text>}
      </div>
    </Card>
  );

  return (
    <div className='mt-[60px] px-2'>
      <Spin spinning={loading}>
        {/* 标题栏 */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Database size={24} />
            <Title heading={4} style={{ margin: 0 }}>DB 看板</Title>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Select
              value={refreshInterval}
              onChange={(v) => setRefreshInterval(v)}
              optionList={REFRESH_OPTIONS}
              style={{ width: 140 }}
            />
            {refreshInterval === 0 && (
              <Button
                icon={<RefreshCw size={16} />}
                onClick={() => { setLoading(true); loadData(); }}
                loading={loading}
              >
                刷新
              </Button>
            )}
          </div>
        </div>

        {!data ? (
          <Empty description='暂无数据' />
        ) : (
          <>
            {/* 指标卡片 */}
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 20 }}>
              <MetricCard
                title='连接数'
                value={`${data.threads_connected} / ${data.max_connections}`}
                sub={`使用率 ${connUsage}%`}
                color='#3b82f6'
              />
              <MetricCard
                title='缓冲池命中率'
                value={data.buffer_pool_hit_rate.toFixed(2) + '%'}
                sub='InnoDB Buffer Pool'
                color='#10b981'
              />
              <MetricCard
                title='当前慢查询'
                value={slowQueryLogEnabled
                  ? formatNum(data.slow_queries)
                  : (slowQueryLogStatus === 'OFF' ? '未开启' : '状态未知')}
                sub={slowQueryLogEnabled
                  ? `阈值 ≥ ${longQueryTimeLabel} 秒`
                  : `slow_query_log=${slowQueryLogStatus}`}
                color={slowQueryLogEnabled && data.slow_queries > 0 ? '#ef4444' : '#10b981'}
              />
              <MetricCard
                title='死锁数'
                value={formatNum(data.innodb_deadlocks)}
                sub='Innodb Deadlocks'
                color={data.innodb_deadlocks > 0 ? '#ef4444' : '#10b981'}
              />
              <MetricCard
                title='临时表落盘'
                value={formatNum(data.created_tmp_tables_on_disk)}
                sub='Tmp Tables on Disk'
                color={data.created_tmp_tables_on_disk > 0 ? '#f59e0b' : '#10b981'}
              />
            </div>

            {/* 实时折线图 */}
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 20 }}>
              <Card style={{ flex: 1, minWidth: 400 }} title='QPS / TPS 实时趋势'>
                {chartHistory.length < 2 ? (
                  <div style={{ height: CHART_HEIGHT, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Text type='tertiary'>选择刷新频率后，至少 2 次采集才能形成趋势图</Text>
                  </div>
                ) : (
                  <VChart spec={qpsTpsSpec} option={CHART_CONFIG} />
                )}
              </Card>
              <Card style={{ flex: 1, minWidth: 400 }} title='网络流入/流出 实时趋势'>
                {chartHistory.length < 2 ? (
                  <div style={{ height: CHART_HEIGHT, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Text type='tertiary'>选择刷新频率后，至少 2 次采集才能形成趋势图</Text>
                  </div>
                ) : (
                  <VChart spec={netSpec} option={CHART_CONFIG} />
                )}
              </Card>
            </div>

            {/* 慢查询表格 */}
            <Card
              style={{ marginBottom: 20 }}
              title={slowQueryLogEnabled
                ? `当前慢查询（运行时间 ≥ ${longQueryTimeLabel} 秒）`
                : '当前慢查询'}
            >
              {slowQueryLogEnabled ? (
                <Table
                  columns={slowQueryColumns}
                  dataSource={data.slow_queries_list || []}
                  rowKey='id'
                  pagination={{ pageSize: 10, showSizeChanger: true }}
                  size='small'
                  scroll={{ x: 1100 }}
                />
              ) : (
                <Empty
                  description={slowQueryLogStatus === 'OFF'
                    ? 'MySQL slow_query_log 未开启，不展示慢查询列表'
                    : '无法确认 MySQL slow_query_log 状态，不展示慢查询列表'}
                />
              )}
            </Card>

            {/* 锁等待表格 */}
            <Card title='锁等待'>
              <Table
                columns={lockWaitColumns}
                dataSource={data.lock_waits || []}
                rowKey={(r) => r.waiting_pid + '_' + r.blocking_pid}
                pagination={{ pageSize: 10, showSizeChanger: true }}
                size='small'
                scroll={{ x: 900 }}
              />
            </Card>
          </>
        )}
      </Spin>

      {/* KILL 确认弹窗 */}
      <Modal
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <AlertTriangle size={18} color='#ef4444' />
            <span>确认 KILL 进程</span>
          </div>
        }
        visible={killModal.visible}
        onCancel={() => setKillModal({ visible: false, id: null })}
        onOk={doKill}
        okText='确认 Kill'
        cancelText='取消'
        okButtonProps={{ type: 'danger' }}
      >
        <Text>确认要 KILL 进程 ID <Text strong type='danger'>{killModal.id}</Text> 吗？</Text>
        <br />
        <Text type='tertiary' size='small'>这将强制终止该 MySQL 会话，可能导致未提交的事务回滚。</Text>
      </Modal>
    </div>
  );
};

export default MySQLDashboard;
