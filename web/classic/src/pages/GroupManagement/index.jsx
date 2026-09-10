/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Modal,
  Pagination,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Layers3 } from 'lucide-react';
import GroupRatioSettings from '../Setting/Ratio/GroupRatioSettings';
import { API, showError, showSuccess, toBoolean } from '../../helpers';

const { Text } = Typography;

const GroupManagement = () => {
  const [loading, setLoading] = useState(false);
  const [options, setOptions] = useState({});
  const [channels, setChannels] = useState([]);
  const [channelTotal, setChannelTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [editChannel, setEditChannel] = useState(null);
  const [editGroups, setEditGroups] = useState([]);
  const [savingMapping, setSavingMapping] = useState(false);

  const loadOptions = async () => {
    const res = await API.get('/api/option/');
    if (!res.data.success) {
      throw new Error(res.data.message || '获取分组配置失败');
    }
    const next = {};
    (res.data.data || []).forEach((item) => {
      let value = item.value;
      if (typeof value === 'string' && (value.startsWith('{') || value.startsWith('['))) {
        try {
          value = JSON.stringify(JSON.parse(value), null, 2);
        } catch (_) {
          // Keep the original value when it is not valid JSON.
        }
      }
      if (['DefaultUseAutoGroup', 'ExposeRatioEnabled'].includes(item.key)) {
        next[item.key] = toBoolean(value);
      } else {
        next[item.key] = value;
      }
    });
    setOptions(next);
  };

  const loadChannels = async (targetPage = page, targetPageSize = pageSize) => {
    const res = await API.get('/api/channel/', {
      params: { p: targetPage, page_size: targetPageSize, id_sort: true },
    });
    if (!res.data.success) {
      throw new Error(res.data.message || '获取渠道列表失败');
    }
    const data = res.data.data || {};
    setChannels(data.items || []);
    setChannelTotal(data.total || 0);
  };

  const loadAll = async () => {
    setLoading(true);
    try {
      await Promise.all([loadOptions(), loadChannels()]);
    } catch (error) {
      showError(error.message || '加载分组管理失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAll();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    loadChannels(page, pageSize).catch((error) =>
      showError(error.message || '获取渠道列表失败'),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, pageSize]);

  const groupNames = useMemo(() => {
    try {
      const parsed = JSON.parse(options.GroupRatio || '{}');
      return Object.keys(parsed).sort();
    } catch (_) {
      return [];
    }
  }, [options.GroupRatio]);

  const groupOptions = useMemo(
    () => groupNames.map((group) => ({ label: group, value: group })),
    [groupNames],
  );

  const refreshGroupSettings = async () => {
    setLoading(true);
    try {
      await loadOptions();
      // When GUI-managed rate limiting has been enabled, adding/removing a
      // group must immediately regenerate the compatible runtime JSON.
      const rebuild = await API.post('/api/rate-limit-management/rebuild');
      if (!rebuild.data.success) {
        throw new Error(rebuild.data.message || '自动更新限流配置失败');
      }
      showSuccess('分组配置已刷新');
    } catch (error) {
      showError(error.message || '刷新失败');
    } finally {
      setLoading(false);
    }
  };

  const openMappingEditor = (channel) => {
    const groups = String(channel.group || '')
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
    setEditChannel(channel);
    setEditGroups(groups);
  };

  const saveChannelGroups = async () => {
    if (!editChannel) return;
    if (!editGroups.length) {
      showError('渠道至少需要关联一个分组');
      return;
    }
    setSavingMapping(true);
    try {
      const res = await API.put(`/api/channel/${editChannel.id}/groups`, {
        groups: editGroups,
      });
      if (!res.data.success) {
        throw new Error(res.data.message || '保存渠道分组失败');
      }
      showSuccess('渠道分组映射已更新');
      setEditChannel(null);
      await loadChannels(page, pageSize);
    } catch (error) {
      showError(error.message || '保存渠道分组失败');
    } finally {
      setSavingMapping(false);
    }
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 90 },
    {
      title: '渠道',
      dataIndex: 'name',
      render: (name, record) => (
        <div>
          <div className='font-medium'>{name || '-'}</div>
          <Text type='tertiary' size='small'>渠道配置请在“渠道管理”中维护</Text>
        </div>
      ),
    },
    {
      title: '对应分组',
      dataIndex: 'group',
      render: (value) => {
        const groups = String(value || '')
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean);
        return (
          <Space wrap>
            {groups.length ? groups.map((group) => (
              <Tag key={group} color={groupNames.includes(group) ? 'blue' : 'orange'}>
                {group}
              </Tag>
            )) : <Text type='tertiary'>未配置</Text>}
          </Space>
        );
      },
    },
    {
      title: '操作',
      width: 120,
      render: (_, record) => (
        <Button size='small' onClick={() => openMappingEditor(record)}>
          管理分组
        </Button>
      ),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <Spin spinning={loading} size='large'>
        <Card
          className='!rounded-2xl'
          title={
            <div className='flex items-center gap-2'>
              <Layers3 size={18} />
              <span>分组管理</span>
            </div>
          }
        >
          <GroupRatioSettings options={options} refresh={refreshGroupSettings} />
        </Card>

        <Card
          className='!rounded-2xl mt-3'
          title='渠道与分组映射'
          headerExtraContent={
            <Text type='tertiary' size='small'>
              仅管理渠道对应分组，不在此修改渠道本身
            </Text>
          }
        >
          <Table
            columns={columns}
            dataSource={channels}
            rowKey='id'
            pagination={false}
            empty='暂无渠道'
          />
          <div className='flex justify-end mt-4'>
            <Pagination
              currentPage={page}
              pageSize={pageSize}
              total={channelTotal}
              showSizeChanger
              pageSizeOpts={[10, 20, 50, 100]}
              onPageChange={(value) => setPage(value)}
              onPageSizeChange={(value) => {
                setPage(1);
                setPageSize(value);
              }}
            />
          </div>
        </Card>
      </Spin>

      <Modal
        title={`管理渠道分组：${editChannel?.name || ''}`}
        visible={Boolean(editChannel)}
        confirmLoading={savingMapping}
        onOk={saveChannelGroups}
        onCancel={() => setEditChannel(null)}
        closeOnEsc={!savingMapping}
      >
        <div className='mb-2'>
          <Text type='tertiary'>渠道只能在“渠道管理”页面创建、删除和修改；此处只维护分组映射。</Text>
        </div>
        <Select
          multiple
          filter
          value={editGroups}
          optionList={groupOptions}
          onChange={(value) => setEditGroups(value || [])}
          placeholder='选择一个或多个分组'
          style={{ width: '100%' }}
        />
      </Modal>
    </div>
  );
};

export default GroupManagement;
