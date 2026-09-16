import { showError as legacyShowError } from './utils';
import { classifyApiError, retryDelayMs } from './apiFailurePolicy';

// One availability state per browser tab. A single recovery probe serves all
// pages rather than creating a separate retry loop for every failed request.
let snapshot = { status: 'online', reason: '', since: null };
const subscribers = new Set();
let retryTimer = null;
let retryAttempt = 0;
let probing = false;
let lastServerNotice = 0;
let serverFailureCount = 0;
let serverFailureWindow = 0;
const processedErrors = new WeakSet();

export const getApiAvailability = () => snapshot;
export function subscribeApiAvailability(listener) {
  subscribers.add(listener);
  return () => subscribers.delete(listener);
}

function setAvailability(status, reason = '') {
  if (snapshot.status === status && snapshot.reason === reason) return;
  snapshot = {
    status,
    reason,
    since: status === 'online' ? null : snapshot.since || Date.now(),
  };
  subscribers.forEach((listener) => listener());
}

function clearRetry() {
  if (retryTimer !== null) clearTimeout(retryTimer);
  retryTimer = null;
}

function scheduleRetry() {
  if (snapshot.status === 'online' || snapshot.status === 'recovered' || retryTimer !== null || probing) return;
  const delay = retryDelayMs(retryAttempt++, Math.random());
  retryTimer = setTimeout(() => {
    retryTimer = null;
    void retryApiAvailability();
  }, delay);
}

// A successful /api/status can be served from cached options while the DB is
// offline. The dedicated readiness endpoint checks DB and Redis with a deadline.
export async function retryApiAvailability() {
  if (probing) return;
  clearRetry();
  probing = true;
  setAvailability('recovering', snapshot.reason);
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 5000);
  try {
    const configured = import.meta.env.VITE_REACT_APP_SERVER_URL;
    const url = new URL('/health/ready', configured || window.location.origin);
    const response = await fetch(url.toString(), {
      method: 'GET',
      cache: 'no-store',
      credentials: 'include',
      signal: controller.signal,
    });
    if (!response.ok) throw new Error('Backend is not ready');
    retryAttempt = 0;
    serverFailureCount = 0;
    // Do not silently discard unsaved forms by reloading automatically.
    // Let the operator decide when to refresh existing page data.
    setAvailability('recovered', '服务连接已恢复');
  } catch (_error) {
    setAvailability('unavailable', snapshot.reason);
  } finally {
    clearTimeout(timeout);
    probing = false;
    scheduleRetry();
  }
}

export function reportApiFailure(error) {
  const kind = classifyApiError(error);
  if (!kind || error?.config?.skipAvailabilityTracking) return false;
  // The global interceptor and individual page catches may see the same error.
  if (processedErrors.has(error)) return true;
  processedErrors.add(error);
  if (kind === 'server') {
    // A single HTTP 500 can be specific to one operation, not a global outage.
    const now = Date.now();
    if (now - serverFailureWindow > 10000) {
      serverFailureCount = 0;
      serverFailureWindow = now;
    }
    serverFailureCount++;
    if (now - lastServerNotice > 20000) {
      legacyShowError('服务器内部错误，请稍后重试');
      lastServerNotice = now;
    }
    if (serverFailureCount < 3) return true;
  }
  const reason = kind === 'timeout'
    ? '服务响应超时'
    : kind === 'network'
      ? '无法连接服务器'
      : '服务暂时不可用';
  setAvailability('unavailable', reason);
  scheduleRetry();
  return true;
}

// Exported explicitly by helpers/index.js. Page-level catches sharing the
// Axios error cannot generate duplicate outage toasts.
export function showError(error) {
  if (reportApiFailure(error)) return;
  legacyShowError(error);
}
