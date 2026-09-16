// Keep the classification and backoff rules independent of React and Axios
// so they can be tested without a browser or a running backend.
export function classifyApiError(error) {
  if (!error || (error.name !== 'AxiosError' && error.isAxiosError !== true)) {
    return null;
  }
  if (error.code === 'ERR_CANCELED' || error.code === 'ECONNABORTED' && error.message === 'canceled') {
    return null;
  }
  const status = error.response?.status;
  if (!error.response) {
    return error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT'
      ? 'timeout'
      : 'network';
  }
  if (status === 502 || status === 503 || status === 504) return 'gateway';
  if (status === 500) return 'server';
  return null;
}

export function retryDelayMs(attempt, jitter = 0) {
  const base = Math.min(30000, 5000 * 2 ** Math.min(Math.max(0, attempt), 3));
  return Math.round(base + base * 0.2 * Math.max(0, Math.min(1, jitter)));
}
