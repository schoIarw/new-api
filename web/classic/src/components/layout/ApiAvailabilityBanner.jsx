import React, { useSyncExternalStore } from 'react';
import { Button } from '@douyinfe/semi-ui';
import {
  getApiAvailability,
  retryApiAvailability,
  subscribeApiAvailability,
} from '../../helpers/apiFailure';

// Render once in the shared page layout, not once per failed request/page.
export default function ApiAvailabilityBanner() {
  const availability = useSyncExternalStore(
    subscribeApiAvailability,
    getApiAvailability,
    getApiAvailability,
  );
  if (availability.status === 'online') return null;

  const checking = availability.status === 'recovering';
  return (
    <div
      role='status'
      aria-live='polite'
      style={{
        position: 'fixed',
        top: 72,
        right: 16,
        zIndex: 1100,
        maxWidth: 'min(480px, calc(100vw - 32px))',
        padding: '12px 16px',
        border: '1px solid var(--semi-color-warning-light-default)',
        borderRadius: 10,
        background: 'var(--semi-color-bg-0)',
        color: 'var(--semi-color-text-0)',
        boxShadow: '0 4px 18px rgba(0, 0, 0, 0.12)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div style={{ flex: 1 }}>
          <strong>{checking ? '正在检查服务恢复情况' : availability.reason || '服务暂时不可用'}</strong>
          <div style={{ fontSize: 12, marginTop: 4 }}>
            {checking
              ? '正在检查数据库及服务连接，请稍候。'
              : '部分数据可能不是最新数据。系统将自动重试，恢复前请勿重复提交操作。'}
          </div>
        </div>
        <Button size='small' loading={checking} disabled={checking} onClick={() => void retryApiAvailability()}>
          重试
        </Button>
      </div>
    </div>
  );
}
