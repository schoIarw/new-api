import { describe, expect, test } from 'bun:test';
import { classifyApiError, retryDelayMs } from './apiFailurePolicy';

const axiosError = (status, code) => ({
  name: 'AxiosError',
  isAxiosError: true,
  code,
  ...(status === undefined ? {} : { response: { status } }),
});

describe('infrastructure error classification', () => {
  test('network and timeouts are infrastructure failures', () => {
    expect(classifyApiError(axiosError())).toBe('network');
    expect(classifyApiError(axiosError(undefined, 'ETIMEDOUT'))).toBe('timeout');
    expect(classifyApiError(axiosError(undefined, 'ECONNABORTED'))).toBe('timeout');
  });

  test('gateway and server failures are handled, normal business errors are not', () => {
    for (const status of [502, 503, 504]) {
      expect(classifyApiError(axiosError(status))).toBe('gateway');
    }
    expect(classifyApiError(axiosError(500))).toBe('server');
    for (const status of [400, 401, 403, 404, 429]) {
      expect(classifyApiError(axiosError(status))).toBeNull();
    }
  });

  test('user cancellations and unrelated errors do not trigger availability alarms', () => {
    expect(classifyApiError(axiosError(undefined, 'ERR_CANCELED'))).toBeNull();
    expect(classifyApiError(new Error('Not an Axios request'))).toBeNull();
  });
});

describe('bounded health probe retry delay', () => {
  test('backs off from 5 to 30 seconds, with bounded jitter', () => {
    expect([0, 1, 2, 3, 30].map((n) => retryDelayMs(n, 0))).toEqual([
      5000, 10000, 20000, 30000, 30000,
    ]);
    expect(retryDelayMs(30, 1)).toBe(36000);
    expect(retryDelayMs(0, -1)).toBe(5000);
  });
});
