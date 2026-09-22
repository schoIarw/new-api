import { describe, expect, test } from 'bun:test';
import { isValidIdentifierPrefix } from './userIdentifierPolicy';

describe('special user identifier prefix validation', () => {
  test('rejects seven characters and accepts eight', () => {
    expect(isValidIdentifierPrefix('1234567')).toBe(false);
    expect(isValidIdentifierPrefix('12345678')).toBe(true);
  });
  test('accepts arbitrary identifiers and counts Unicode characters', () => {
    expect(isValidIdentifierPrefix('企业用户甲乙丙丁')).toBe(true);
    expect(isValidIdentifierPrefix('abcdefg|app')).toBe(true);
    expect(isValidIdentifierPrefix('')).toBe(false);
  });
});
