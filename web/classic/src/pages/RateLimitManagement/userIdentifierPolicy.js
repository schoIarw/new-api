// Count Unicode characters, not UTF-8 bytes or a fixed phone-number format.
export const isValidIdentifierPrefix = (prefix) =>
  typeof prefix === 'string' && Array.from(prefix).length >= 8;
