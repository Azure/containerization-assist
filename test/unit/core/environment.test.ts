import { describe, it, expect } from '@jest/globals';

describe('Test Infrastructure', () => {
  it('should run basic test', () => {
    expect(1 + 1).toBe(2);
  });
  
  it('should have test environment set', () => {
    expect(process.env.NODE_ENV).toBe('test');
  });
});