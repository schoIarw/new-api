import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';

const { Text } = Typography;
const isPhone = (phone) => /^1[3-9][0-9]{9}$/.test(phone);
const pair = (values) => Array.isArray(values) && values.length === 2 ? values : [0, 0];

export default function PhoneRateLimit({ groups = [] }) {
  const [policies, setPolicies] = useState({});
  const [busy, setBusy] = useState(false);
  const [modal, setModal] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState({ group: '', phone: '', total: 0, success: 0 });
  const load = async () => {
    setBusy(true);
    try {
      const response = await API.get('/api/rate-limit-management/phone-policies');
      if (!response.data.success) throw new Error(response.data.message || '加载手机号策略失败');
      setPolicies(response.data.data || {});
    } catch (error) { showError(error.message || '加载手机号策略失败'); }
    finally { setBusy(false); }
  };
  useEffect(() => { void load(); }, []);
  const save = async (next) => {
    setBusy(true);
    try {
      const response = await API.put('/api/rate-limit-management/phone-policies', { policies: next });
      if (!response.data.success) throw new Error(response.data.message || '保存手机号策略失败');
      setPolicies(next);
      setModal(false);
      showSuccess('手机号限流策略已保存并生效');
    } catch (error) { showError(error.message || '保存手机号策略失败'); }
    finally { setBusy(false); }
  };
  const groupRows = useMemo(() => groups.map((group) => ({
    group, total: pair(policies[group]?.default)[0], success: pair(policies[group]?.default)[1],
    specialCount: Object.keys(policies[group]?.special || {}).length,
  })), [groups, policies]);
  const specialRows = useMemo(() => Object.entries(policies).flatMap(([group, policy]) =>
    Object.entries(policy?.special || {}).map(([phone, limits]) => ({ group, phone, limits }))
  ), [policies]);
  const editDefault = (group, index, value) => {
    setPolicies((previous) => {
      const next = structuredClone(previous);
      if (!next[group]) next[group] = { default: [0, 0], special: {} };
      const limits = [...pair(next[group].default)];
      limits[index] = Number(value) || 0;
      next[group].default = limits;
      return next;
    });
  };
  const open = (row) => {
    setEditing(row ? `${row.group}:${row.phone}` : null);
    setForm(row ? {group: row.group, phone: row.phone, total: pair(row.limits)[0], success: pair(row.limits)[1]}
      : { group: '', phone: '', total: 0, success: 0 });
    setModal(true);
  };
  const saveSpecial = () => {
    if (!form.group || !isPhone(form.phone)) { showError('请选择分组并填写有效的 11 位手机号'); return; }
    const next = structuredClone(policies);
    if (!next[form.group]) next[form.group] = { default: [0,0], special: {} };
    if (!next[form.group].special) next[form.group].special = {};
    next[form.group].special[form.phone] = [Number(form.total)||0, Number(form.success)||0];
    void save(next);
  };
  const remove = (row) => {
    const next = structuredClone(policies);
    delete next[row.group]?.special?.[row.phone];
    void save(next);
  };
  return <div>
    <Card className='!rounded-xl mb-3' title='每个令牌分组的手机号通用限流'>
      <Text type='tertiary'>每个手机号独立计数；同一个手机号在不同令牌分组分别计数。[0,0] 表示不限制。保存后策略立即生效。</Text>
      <Table rowKey='group' dataSource={groupRows} pagination={false} columns={[
        { title:'令牌分组', dataIndex:'group' },
        { title:'每周期最多请求数', render:(_,row)=><InputNumber min={0} value={row.total} onChange={v=>editDefault(row.group,0,v)}/> },
        { title:'每周期最多完成数', render:(_,row)=><InputNumber min={0} value={row.success} onChange={v=>editDefault(row.group,1,v)}/> },
        { title:'特殊号码数', render:(_,row)=><Tag color='blue'>{row.specialCount}</Tag> },
      ]}/>
      <div className='flex justify-end mt-3'><Button loading={busy} type='primary' onClick={()=>void save(policies)}>保存通用限流</Button></div>
    </Card>
    <Card className='!rounded-xl' title='分组特殊手机号限流'>
      <div className='flex justify-between mb-3'><Text type='tertiary'>特殊规则只覆盖本分组的手机号通用规则，不覆盖模型限流。</Text>
        <Button type='primary' onClick={()=>open(null)}>新增特殊号码</Button></div>
      <Table rowKey={row=>`${row.group}:${row.phone}`} dataSource={specialRows} pagination={false} columns={[
        { title:'令牌分组', dataIndex:'group' }, { title:'手机号', dataIndex:'phone' },
        { title:'最多请求数', render:(_,row)=>pair(row.limits)[0] },
        { title:'最多完成数', render:(_,row)=>pair(row.limits)[1] },
        { title:'操作', render:(_,row)=><Space><Button size='small' onClick={()=>open(row)}>编辑</Button><Button size='small' type='danger' onClick={()=>remove(row)}>删除</Button></Space> },
      ]}/>
    </Card>
    <Card className='!rounded-xl mt-3' title='独立手机号策略 JSON（PhoneRateLimitPolicies）'>
      <pre style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{JSON.stringify(policies,null,2)}</pre>
      <Text type='tertiary'>未配置新策略的令牌分组，继续兼容旧版顶层手机号数组规则。</Text>
    </Card>
    <Modal title={editing?'编辑特殊手机号':'新增特殊手机号'} visible={modal} onCancel={()=>setModal(false)} onOk={saveSpecial} okButtonProps={{loading:busy}}>
      <div className='grid gap-3'>
        <div><Text type='tertiary'>令牌分组</Text><Select style={{width:'100%',marginTop:6}} disabled={Boolean(editing)} value={form.group} optionList={groups.map(g=>({label:g,value:g}))} onChange={v=>setForm(p=>({...p,group:v}))}/></div>
        <div><Text type='tertiary'>手机号</Text><Input style={{marginTop:6}} disabled={Boolean(editing)} value={form.phone} maxLength={11} onChange={v=>setForm(p=>({...p,phone:v.trim()}))}/></div>
        <div><Text type='tertiary'>每周期最多请求数</Text><InputNumber min={0} style={{width:'100%',marginTop:6}} value={form.total} onChange={v=>setForm(p=>({...p,total:Number(v)||0}))}/></div>
        <div><Text type='tertiary'>每周期最多完成数</Text><InputNumber min={0} style={{width:'100%',marginTop:6}} value={form.success} onChange={v=>setForm(p=>({...p,success:Number(v)||0}))}/></div>
      </div>
    </Modal>
  </div>;
}
