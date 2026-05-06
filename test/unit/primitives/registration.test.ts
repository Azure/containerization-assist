import { describe, it, expect } from '@jest/globals';
import { ALL_TOOLS } from '@/tools';

describe('primitive registration in ALL_TOOLS', () => {
  const PRIMITIVE_NAMES = ['query-knowledge', 'validate'];

  for (const name of PRIMITIVE_NAMES) {
    it(`registers ${name}`, () => {
      const found = ALL_TOOLS.find((t) => t.name === name);
      expect(found).toBeDefined();
      expect(found?.schema).toBeDefined();
      expect(typeof found?.handler).toBe('function');
      expect(typeof found?.parse).toBe('function');
    });
  }
});
