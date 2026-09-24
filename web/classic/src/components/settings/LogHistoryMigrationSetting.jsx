import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Col,
  InputNumber,
  Row,
  Spin,
  Switch,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

const formatTimestamp = (value) => {
  if (!value) return '-';
  return new Date(Number(value) * 1000).toLocaleString('zh-CN', { hour12: false });
};

const taskStatus = {
  pending: ['orange', '等待执行'],
  running: ['blue', '执行中'],
  succeeded: ['green', '成功'],
  failed: ['red', '失败'],
};

export default function LogHistoryMigrationSetting() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState(false);
  const [data, setData] = useState(null);
  const [config, setConfig] = useState({
    enabled: false,
    retention_days: 31,
    interval_minutes: 1440,
    batch_size: 1000,
  });

  const loadStatus = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/log/history-migration/status');
      if (!res.data.success) {
        throw new Error(res.data.message || t('获取日志迁移状态失败'));
      }
      const next = res.data.data || {};
      setData(next);
      if (next.config) setConfig(next.config);
    } catch (error) {
      showError(error.message || t('获取日志迁移状态失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadStatus();
  }, [loadStatus]);

  const save = async () => {
    if (config.retention_days < 1 || config.interval_minutes < 1) {
      showError(t('保留天数和执行间隔必须大于0'));
      return;
    }
    if (config.batch_size < 1 || config.batch_size > 10000) {
      showError(t('单批数量必须在1到10000之间'));
      return;
    }
    setSaving(true);
    try {
      const responses = await Promise.all(
        Object.entries(config).map(([key, value]) =>
          API.put('/api/option/', {
            key: `log_history_setting.${key}`,
            value: String(value),
          }),
        ),
      );
      if (responses.some((response) => !response?.data?.success)) {
        throw new Error(t('保存日志迁移设置失败'));
      }
      showSuccess(t('日志迁移设置已保存'));
      await loadStatus();
    } catch (error) {
      showError(error.message || t('保存日志迁移设置失败'));
    } finally {
      setSaving(false);
    }
  };

  const runNow = async () => {
    setRunning(true);
    try {
      const res = await API.post('/api/log/history-migration/run');
      if (!res.data.success) {
        throw new Error(res.data.message || t('提交日志迁移任务失败'));
      }
      showSuccess(res.data.message || t('日志迁移任务已提交'));
      await loadStatus();
    } catch (error) {
      showError(error.message || t('提交日志迁移任务失败'));
    } finally {
      setRunning(false);
    }
  };

  const stats = data?.stats || {};
  const latestTask = data?.latest_task;
  const statusMeta = taskStatus[latestTask?.status] || ['grey', '尚未执行'];
  const migratedCount = latestTask?.result?.migrated_count ?? 0;

  return (
    <Spin spinning={loading} size='large'>
      <Card style={{ marginTop: 10 }}>
        <div className='flex items-center justify-between flex-wrap gap-3'>
          <div>
            <Title heading={5}>{t('logs 定时迁移至 logs_history')}</Title>
            <Text type='tertiary'>
              {t('超过保留期的日志将按批次事务迁移；默认关闭，迁移后现有日志查询仍只读取 logs。')}
            </Text>
          </div>
          <Tag color={stats.supported === false ? 'red' : config.enabled ? 'green' : 'grey'}>
            {stats.supported === false
              ? t('当前日志库不支持')
              : config.enabled
                ? t('定时迁移已开启')
                : t('定时迁移已关闭')}
          </Tag>
        </div>

        {stats.unsupported_reason ? (
          <div className='mt-4'>
            <Text type='danger'>{stats.unsupported_reason}</Text>
          </div>
        ) : null}

        <Row gutter={16} style={{ marginTop: 20 }}>
          <Col span={6}>
            <Text>{t('启用定时迁移')}</Text>
            <div className='mt-2'>
              <Switch
                checked={config.enabled}
                disabled={stats.supported === false}
                onChange={(enabled) => setConfig((old) => ({ ...old, enabled }))}
              />
            </div>
          </Col>
          <Col span={6}>
            <Text>{t('logs 保留天数')}</Text>
            <InputNumber
              className='mt-2 w-full'
              min={1}
              value={config.retention_days}
              onChange={(value) => setConfig((old) => ({ ...old, retention_days: Number(value) }))}
            />
          </Col>
          <Col span={6}>
            <Text>{t('执行间隔（分钟）')}</Text>
            <InputNumber
              className='mt-2 w-full'
              min={1}
              value={config.interval_minutes}
              onChange={(value) => setConfig((old) => ({ ...old, interval_minutes: Number(value) }))}
            />
          </Col>
          <Col span={6}>
            <Text>{t('单批迁移条数')}</Text>
            <InputNumber
              className='mt-2 w-full'
              min={1}
              max={10000}
              value={config.batch_size}
              onChange={(value) => setConfig((old) => ({ ...old, batch_size: Number(value) }))}
            />
          </Col>
        </Row>

        <div className='flex gap-2 mt-5'>
          <Button theme='solid' loading={saving} onClick={save}>
            {t('保存设置')}
          </Button>
          <Button
            loading={running}
            disabled={stats.supported === false}
            onClick={runNow}
          >
            {t('立即迁移一次')}
          </Button>
          <Button onClick={loadStatus}>{t('刷新状态')}</Button>
        </div>
      </Card>

      <Row gutter={16} style={{ marginTop: 10 }}>
        <Col span={6}><Card title={t('logs 当前记录')}>{Number(stats.logs_count || 0).toLocaleString()}</Card></Col>
        <Col span={6}><Card title={t('待迁移记录')}>{Number(stats.eligible_count || 0).toLocaleString()}</Card></Col>
        <Col span={6}><Card title={t('history 已迁移')}>{Number(stats.history_count || 0).toLocaleString()}</Card></Col>
        <Col span={6}>
          <Card title={t('最近任务')}>
            <Tag color={statusMeta[0]}>{t(statusMeta[1])}</Tag>
            <Text style={{ marginLeft: 8 }}>{t('迁移')} {Number(migratedCount).toLocaleString()} {t('条')}</Text>
          </Card>
        </Col>
      </Row>

      <Card title={t('迁移情况')} style={{ marginTop: 10 }}>
        <Row gutter={16}>
          <Col span={8}><Text type='tertiary'>{t('logs 最早时间')}</Text><div>{formatTimestamp(stats.oldest_log_timestamp)}</div></Col>
          <Col span={8}><Text type='tertiary'>{t('history 最新时间')}</Text><div>{formatTimestamp(stats.newest_history_timestamp)}</div></Col>
          <Col span={8}><Text type='tertiary'>{t('最近任务时间')}</Text><div>{formatTimestamp(latestTask?.updated_at)}</div></Col>
        </Row>
        {latestTask?.error ? <Text type='danger'>{latestTask.error}</Text> : null}
      </Card>
    </Spin>
  );
}
