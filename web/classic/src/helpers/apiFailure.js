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
    const base = new URL(configured || '/', window.location.origin);
    const url = new URL('/health/ready', base);
    const response = await fetch(url.toString(), {
      method: 'GET',
      cache: 'no-store',
      credentials: 'include',
      signal: controller.signal,
    });
    // Nginx SPA fallbacks can return index.html with HTTP 200; do not treat
    // that as a healthy backend or a recovered database.
    const readiness = response.ok ? await response.json() : null;
    if (!response.ok || readiness?.status !== 'ready') {
      throw new Error('Backend is not ready');
    }
    retryAttempt = 0;
    serverFailureCount = 0;
    // Do not silently discard unsaved forms by reloading automatically.
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
  // The interceptor and individual page catches may see the same error.
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

// Also suppress page-level stringification of the same Axios failure, e.g.
// catch (error) { showError(error.message) }. Keep business errors visible.
const NETWORK_MESSAGE = /Network Error|Failed to fetch|ERR_CONNECTION_(?:REFUSED|RESET|CLOSED)|网络错误|连接服务器失败/i;
export function showError(error) {
  if (reportApiFailure(error)) return;
  if (snapshot.status !== 'online') {
    const message = typeof error === 'string' ? error : error?.message;
    if (typeof message === 'string' && NETWORK_MESSAGE.test(message)) return;
  }
  legacyShowError(error);
}
