import { showError as legacyShowError } from './utils';
import { classifyApiError, retryDelayMs } from './apiFailurePolicy';

// One availability state per browser tab; individual pages must not create
// competing health-check loops when the database or proxy is unavailable.
let snapshot = { status: 'online', reason: '', since: null };
const subscribers = new Set();
let retryTimer = null;
let retryAttempt = 0;
let probing = false;
let lastServerNotice = 0;
let serverFailureCount = 0;
let serverFailureWindow = 0;

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
  if (snapshot.status === 'online' || retryTimer !== null || probing) return;
  const delay = retryDelayMs(retryAttempt++, Math.random());
  retryTimer = setTimeout(() => {
    retryTimer = null;
    void retryApiAvailability();
  }, delay);
}

// The endpoint checks the primary DB with a deadline. /api/status alone is
// insufficient: it can return cached settings while MySQL is still offline.
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
    setAvailability('online');
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
  if (kind === 'server') {
    // A single 500 may be a business-specific error, not a global outage.
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

// Re-exported explicitly by helpers/index.js. Page-level catch blocks that
// call showError(error) no longer produce duplicate infrastructure toasts.
export function showError(error) {
  if (reportApiFailure(error)) return;
  legacyShowError(error);
}
