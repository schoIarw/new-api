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
  Input,
  InputNumber,
  Modal,
  Pagination,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Gauge, Tags } from 'lucide-react';
import { API, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;
const CATEGORY_KEYS = ['fast', 'flagship', 'dedicated'];

const normalizePair = (value) => {
  if (!Array.isArray(value) || value.length !== 2) return [0, 0];
  return [Number(value[0]) || 0, Number(value[1]) || 0];
};

const RateLimitManagement = () => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState(null);
  const [defaults, setDefaults] = useState({
    enabled: false,
    duration_minutes: 1,
    total_count: 0,
    success_count: 1000,
  });
  const [categoryLimits, setCategoryLimits] = useState({});
  const [specialLimits, setSpecialLimits] = useState({});
  const [modelKeyword, setModelKeyword] = useState('');
  const [modelPage, setModelPage] = useState(1);
  const [modelPageSize, setModelPageSize] = useState(20);
  const [specialModalVisible, setSpecialModalVisible] = useState(false);
  const [specialEditing, setSpecialEditing] = useState(null);
  const [specialForm, setSpecialForm] = useState({
    group: '',
    model_name: '',
    total: 0,
    success: 0,
  });

  const loadData = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/rate-limit-management/');
      if (!res.data.success) {
        throw new Error(res.data.message || '获取限流管理配置失败');
      }
      const next = res.data.data || {};
      setData(next);
      setDefaults({
        enabled: Boolean(next.enabled),
        duration_minutes: Number(next.duration_minutes) || 1,
        total_count: Number(next.total_count) || 0,
        success_count: Number(next.success_count) || 0,
      });
      const nextCategoryLimits = {};
      CATEGORY_KEYS.forEach((key) => {
        nextCategoryLimits[key] = normalizePair(next.category_limits?.[key]);
      });
      setCategoryLimits(nextCategoryLimits);
      setSpecialLimits(next.special_limits || {});
    } catch (error) {
      showError(error.message || '获取限流管理配置失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const categories = data?.categories || [
    { key: 'fast', name: '快速' },
    { key: 'flagship', name: '旗舰' },
    { key: 'dedicated', name: '专用' },
  ];
  const categoryNameMap = useMemo(
    () => Object.fromEntries(categories.map((item) => [item.key, item.name])),
    [categories],
  );

  const models = data?.models || [];
  const filteredModels = useMemo(() => {
    const keyword = modelKeyword.trim().toLowerCase();
    if (!keyword) return models;
    return models.filter((item) =>
      String(item.model_name || '').toLowerCase().includes(keyword),
    );
  }, [models, modelKeyword]);
  const pagedModels = useMemo(() => {
    const start = (modelPage - 1) * modelPageSize;
    return filteredModels.slice(start, start + modelPageSize);
  }, [filteredModels, modelPage, modelPageSize]);

  const modelsByCategory = useMemo(() => {
    const result = {};
    CATEGORY_KEYS.forEach((key) => {
      result[key] = [];
    });
    models.forEach((item) => {
      const category = item.category;
      const modelName = String(item.model_name || '').trim();
      if (!category || !modelName) return;
      if (!result[category]) result[category] = [];
      result[category].push(modelName);
    });
    Object.keys(result).forEach((key) => {
      result[key].sort((a, b) => a.localeCompare(b));
    });
    return result;
  }, [models]);

  const modelCountByCategory = useMemo(() => {
    const result = {};
    Object.entries(modelsByCategory).forEach(([category, categoryModels]) => {
      result[category] = categoryModels.length;
    });
    return result;
  }, [modelsByCategory]);

  const saveDefaults = async () => {
    setLoading(true);
    try {
      const res = await API.put('/api/rate-limit-management/defaults', defaults);
      if (!res.data.success) throw new Error(res.data.message || '保存基础限流失败');
      showSuccess('基础限流配置已保存');
      await loadData();
    } catch (error) {
      showError(error.message || '保存基础限流失败');
      setLoading(false);
    }
  };

  const saveCategoryLimits = async () => {
    setLoading(true);
    try {
      const policies = {};
      CATEGORY_KEYS.forEach((key) => {
        policies[key] = normalizePair(categoryLimits[key]);
      });
      const res = await API.put('/api/rate-limit-management/category-policies', {
        policies,
      });
      if (!res.data.success) throw new Error(res.data.message || '保存通用限流失败');
      showSuccess('通用限流已保存并自动生成运行配置');
      await loadData();
    } catch (error) {
      showError(error.message || '保存通用限流失败');
      setLoading(false);
    }
  };

  const changeModelCategory = async (modelName, category) => {
    setLoading(true);
    try {
      let res;
      if (category) {
        res = await API.put('/api/rate-limit-management/model-category', {
          model_name: modelName,
          category,
        });
      } else {
        res = await API.delete('/api/rate-limit-management/model-category', {
          data: { model_name: modelName },
        });
      }
      if (!res.data.success) throw new Error(res.data.message || '更新模型分类失败');
      showSuccess('模型分类已更新，限流配置已自动同步');
      await loadData();
    } catch (error) {
      showError(error.message || '更新模型分类失败');
      setLoading(false);
    }
  };

  const specialRows = useMemo(() => {
    const rows = [];
    Object.entries(specialLimits || {}).forEach(([group, rules]) => {
      Object.entries(rules || {}).forEach(([modelName, pair]) => {
        const value = normalizePair(pair);
        rows.push({
          key: `${group}||${modelName}`,
          group,
          model_name: modelName,
          total: value[0],
          success: value[1],
        });
      });
    });
    return rows.sort(
      (a, b) =>
        a.group.localeCompare(b.group) || a.model_name.localeCompare(b.model_name),
    );
  }, [specialLimits]);

  const persistSpecialLimits = async (nextLimits) => {
    setLoading(true);
    try {
      const res = await API.put('/api/rate-limit-management/special-policies', {
        policies: nextLimits,
      });
      if (!res.data.success) throw new Error(res.data.message || '保存特殊限流失败');
      showSuccess('特殊限流已保存并覆盖通用规则');
      setSpecialModalVisible(false);
      setSpecialEditing(null);
      await loadData();
    } catch (error) {
      showError(error.message || '保存特殊限流失败');
      setLoading(false);
    }
  };

  const openSpecialModal = (row = null) => {
    if (row) {
      setSpecialEditing(row.key);
      setSpecialForm({
        group: row.group,
        model_name: row.model_name,
        total: row.total,
        success: row.success,
      });
    } else {
      setSpecialEditing(null);
      setSpecialForm({ group: '', model_name: '', total: 0, success: 0 });
    }
    setSpecialModalVisible(true);
  };

  const saveSpecialRow = async () => {
    if (!specialForm.group || !specialForm.model_name) {
      showError('请选择分组和模型');
      return;
    }
    const next = JSON.parse(JSON.stringify(specialLimits || {}));
    if (specialEditing) {
      const [oldGroup, oldModel] = specialEditing.split('||');
      if (next[oldGroup]) {
        delete next[oldGroup][oldModel];
        if (!Object.keys(next[oldGroup]).length) delete next[oldGroup];
      }
    }
    if (!next[specialForm.group]) next[specialForm.group] = {};
    next[specialForm.group][specialForm.model_name] = [
      Number(specialForm.total) || 0,
      Number(specialForm.success) || 0,
    ];
    await persistSpecialLimits(next);
  };

  const deleteSpecialRow = async (row) => {
    const next = JSON.parse(JSON.stringify(specialLimits || {}));
    if (next[row.group]) {
      delete next[row.group][row.model_name];
      if (!Object.keys(next[row.group]).length) delete next[row.group];
    }
    await persistSpecialLimits(next);
  };

  const categoryColumns = [
    { title: '模型分类', dataIndex: 'name', width: 120 },
    {
      title: '模型数',
      render: (_, row) => (
        <Tag color='blue'>{modelCountByCategory[row.key] || 0}</Tag>
      ),
      width: 90,
    },
    {
      title: '每周期最多请求数',
      render: (_, row) => (
        <InputNumber
          min={0}
          value={normalizePair(categoryLimits[row.key])[0]}
          onChange={(value) =>
            setCategoryLimits((prev) => ({
              ...prev,
              [row.key]: [Number(value) || 0, normalizePair(prev[row.key])[1]],
            }))
          }
          style={{ width: 160 }}
        />
      ),
      width: 190,
    },
    {
      title: '每周期最多完成数',
      render: (_, row) => (
        <InputNumber
          min={0}
          value={normalizePair(categoryLimits[row.key])[1]}
          onChange={(value) =>
            setCategoryLimits((prev) => ({
              ...prev,
              [row.key]: [normalizePair(prev[row.key])[0], Number(value) || 0],
            }))
          }
          style={{ width: 160 }}
        />
      ),
      width: 190,
    },
    {
      title: '归属模型',
      render: (_, row) => {
        const categoryModels = modelsByCategory[row.key] || [];
        if (!categoryModels.length) {
          return <Text type='tertiary'>暂无模型</Text>;
        }
        return (
          <div className='flex flex-wrap gap-1 py-1'>
            {categoryModels.map((modelName) => (
              <Tag key={`${row.key}-${modelName}`} color='grey'>
                {modelName}
              </Tag>
            ))}
          </div>
        );
      },
      width: 360,
    },
  ];

  const modelColumns = [
    { title: '模型名称', dataIndex: 'model_name' },
    {
      title: '当前分类',
      dataIndex: 'category',
      width: 220,
      render: (category, row) => (
        <Select
          value={category || ''}
          onChange={(value) =>
            changeModelCategory(row.model_name, value || '')
          }
          optionList={[
            { label: '未分类', value: '' },
            ...categories.map((item) => ({ label: item.name, value: item.key })),
          ]}
          style={{ width: 180 }}
        />
      ),
    },
  ];

  const specialColumns = [
    { title: '分组', dataIndex: 'group', width: 180 },
    { title: '模型', dataIndex: 'model_name' },
    { title: '最多请求数', dataIndex: 'total', width: 130 },
    { title: '最多完成数', dataIndex: 'success', width: 130 },
    {
      title: '操作',
      width: 180,
      render: (_, row) => (
        <Space>
          <Button size='small' onClick={() => openSpecialModal(row)}>
            编辑
          </Button>
          <Button size='small' type='danger' onClick={() => deleteSpecialRow(row)}>
            删除
          </Button>
        </Space>
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
              <Gauge size={18} />
              <span>限流管理</span>
              {data?.managed_enabled ? (
                <Tag color='green'>界面化策略已启用</Tag>
              ) : (
                <Tag color='grey'>尚未启用界面化策略</Tag>
              )}
            </div>
          }
        >
          <Tabs type='card' defaultActiveKey='general'>
            <Tabs.TabPane tab='通用限流管理' itemKey='general'>
              <Card className='!rounded-xl mb-3' title='基础限流参数'>
                <div className='flex flex-wrap items-end gap-4'>
                  <div>
                    <Text type='tertiary'>启用用户模型请求速率限制</Text>
                    <div className='mt-2'>
                      <Switch
                        checked={defaults.enabled}
                        onChange={(value) =>
                          setDefaults((prev) => ({ ...prev, enabled: value }))
                        }
                      />
                    </div>
                  </div>
                  <div>
                    <Text type='tertiary'>限流周期（分钟）</Text>
                    <div className='mt-2'>
                      <InputNumber
                        min={1}
                        value={defaults.duration_minutes}
                        onChange={(value) =>
                          setDefaults((prev) => ({
                            ...prev,
                            duration_minutes: Number(value) || 1,
                          }))
                        }
                        style={{ width: 150 }}
                      />
                    </div>
                  </div>
                  <div>
                    <Text type='tertiary'>每周期最多请求数</Text>
                    <div className='mt-2'>
                      <InputNumber
                        min={0}
                        value={defaults.total_count}
                        onChange={(value) =>
                          setDefaults((prev) => ({
                            ...prev,
                            total_count: Number(value) || 0,
                          }))
                        }
                        style={{ width: 170 }}
                      />
                    </div>
                  </div>
                  <div>
                    <Text type='tertiary'>每周期最多完成数</Text>
                    <div className='mt-2'>
                      <InputNumber
                        min={0}
                        value={defaults.success_count}
                        onChange={(value) =>
                          setDefaults((prev) => ({
                            ...prev,
                            success_count: Number(value) || 0,
                          }))
                        }
                        style={{ width: 170 }}
                      />
                    </div>
                  </div>
                  <Button type='primary' onClick={saveDefaults}>
                    保存基础设置
                  </Button>
                </div>
              </Card>

              <Card className='!rounded-xl mb-3' title='模型分类'>
                <div className='flex items-center justify-between mb-3 gap-3'>
                  <Text type='tertiary'>
                    一个模型最多属于快速 / 旗舰 / 专用中的一个分类；分类变更会自动重新生成限流 JSON。
                  </Text>
                  <Input
                    value={modelKeyword}
                    onChange={(value) => {
                      setModelKeyword(value);
                      setModelPage(1);
                    }}
                    showClear
                    placeholder='搜索模型名称'
                    style={{ width: 240 }}
                  />
                </div>
                <Table
                  columns={modelColumns}
                  dataSource={pagedModels}
                  rowKey='model_name'
                  pagination={false}
                />
                <div className='flex justify-end mt-3'>
                  <Pagination
                    currentPage={modelPage}
                    pageSize={modelPageSize}
                    total={filteredModels.length}
                    showSizeChanger
                    pageSizeOpts={[10, 20, 50, 100]}
                    onPageChange={setModelPage}
                    onPageSizeChange={(value) => {
                      setModelPage(1);
                      setModelPageSize(value);
                    }}
                  />
                </div>
              </Card>

              <Card className='!rounded-xl' title='按模型分类设置通用限流'>
                <Text type='tertiary'>
                  分类规则会展开到所有现有分组；[0,0] 表示该分类不设置基础限制。特殊配置可覆盖指定分组/模型。
                </Text>
                <div className='mt-3'>
                  <Table
                    columns={categoryColumns}
                    dataSource={categories}
                    rowKey='key'
                    pagination={false}
                  />
                </div>
                <div className='flex justify-end mt-3'>
                  <Button type='primary' onClick={saveCategoryLimits}>
                    保存并生成限流配置
                  </Button>
                </div>
              </Card>
            </Tabs.TabPane>

            <Tabs.TabPane tab='特殊限流配置' itemKey='special'>
              <div className='flex items-center justify-between mb-3'>
                <Text type='tertiary'>
                  针对指定“分组 + 模型”覆盖通用分类限流；设置 [0,0] 可显式取消该模型在该分组的基础限制。
                </Text>
                <Button
                  type='primary'
                  icon={<Tags size={16} />}
                  onClick={() => openSpecialModal()}
                >
                  新增特殊配置
                </Button>
              </div>
              <Table
                columns={specialColumns}
                dataSource={specialRows}
                rowKey='key'
                pagination={false}
              />
            </Tabs.TabPane>
          </Tabs>
        </Card>

        <Card className='!rounded-2xl mt-3' title='当前兼容限流 JSON'>
          <Text type='tertiary'>
            该 JSON 为运行时最终配置，继续兼容现有 ModelRequestRateLimitGroup 规范；手机号/Account 顶层数组配置会保留。
          </Text>
          <TextArea
            className='mt-3'
            value={data?.generated_json || '{}'}
            autosize={{ minRows: 6, maxRows: 18 }}
            readonly
          />
        </Card>
      </Spin>

      <Modal
        title={specialEditing ? '编辑特殊限流配置' : '新增特殊限流配置'}
        visible={specialModalVisible}
        onCancel={() => setSpecialModalVisible(false)}
        onOk={saveSpecialRow}
      >
        <div className='grid gap-4'>
          <div>
            <Text type='tertiary'>分组</Text>
            <Select
              value={specialForm.group}
              onChange={(value) =>
                setSpecialForm((prev) => ({ ...prev, group: value }))
              }
              optionList={(data?.groups || []).map((group) => ({
                label: group,
                value: group,
              }))}
              filter
              style={{ width: '100%', marginTop: 8 }}
              disabled={Boolean(specialEditing)}
            />
          </div>
          <div>
            <Text type='tertiary'>模型</Text>
            <Select
              value={specialForm.model_name}
              onChange={(value) =>
                setSpecialForm((prev) => ({ ...prev, model_name: value }))
              }
              optionList={models.map((item) => ({
                label: item.model_name,
                value: item.model_name,
              }))}
              filter
              style={{ width: '100%', marginTop: 8 }}
              disabled={Boolean(specialEditing)}
            />
          </div>
          <div className='flex gap-4'>
            <div className='flex-1'>
              <Text type='tertiary'>每周期最多请求数</Text>
              <InputNumber
                min={0}
                value={specialForm.total}
                onChange={(value) =>
                  setSpecialForm((prev) => ({
                    ...prev,
                    total: Number(value) || 0,
                  }))
                }
                style={{ width: '100%', marginTop: 8 }}
              />
            </div>
            <div className='flex-1'>
              <Text type='tertiary'>每周期最多完成数</Text>
              <InputNumber
                min={0}
                value={specialForm.success}
                onChange={(value) =>
                  setSpecialForm((prev) => ({
                    ...prev,
                    success: Number(value) || 0,
                  }))
                }
                style={{ width: '100%', marginTop: 8 }}
              />
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default RateLimitManagement;
