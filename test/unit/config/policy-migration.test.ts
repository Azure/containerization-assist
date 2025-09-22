/**
 * Policy Migration Tests
 * Tests for migrating v1.0 policies to v2.0
 */

import { describe, it, expect } from '@jest/globals';
import {
  migrateV1ToV2,
  validatePolicy,
  type LegacyPolicy,
  type UnifiedPolicy,
} from '@/config/policy';

describe('Policy Migration', () => {
  describe('V1 to V2 Migration', () => {
    it('should migrate basic v1 policy to v2', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'test-rule',
            priority: 100,
            matchers: [
              {
                pattern: 'FROM.*alpine',
              },
            ],
            actions: {
              scan: true,
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.version).toBe('2.0');
      expect(migrated.metadata?.description).toContain('Migrated from v1.0');
      expect(migrated.rules).toHaveLength(1);
      expect(migrated.rules[0].id).toBe('test-rule');
      expect(migrated.rules[0].priority).toBe(100);
      expect(migrated.rules[0].conditions).toHaveLength(1);
      expect(migrated.rules[0].conditions[0].kind).toBe('regex');
      expect((migrated.rules[0].conditions[0] as any).pattern).toBe('FROM.*alpine');
      expect(migrated.rules[0].actions.scan).toBe(true);
    });

    it('should handle missing priority with default', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'no-priority-rule',
            matchers: [],
            actions: {},
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);
      expect(migrated.rules[0].priority).toBe(50);
    });

    it('should convert function matchers', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'function-rule',
            priority: 80,
            matchers: [
              {
                function: 'fileExists',
                args: ['package.json'],
              },
            ],
            actions: {
              validate: true,
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].conditions).toHaveLength(1);
      expect(migrated.rules[0].conditions[0].kind).toBe('function');
      expect((migrated.rules[0].conditions[0] as any).name).toBe('fileExists');
      expect((migrated.rules[0].conditions[0] as any).args).toEqual(['package.json']);
    });

    it('should handle multiple matchers', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'multi-matcher',
            priority: 90,
            matchers: [
              {
                pattern: 'FROM',
              },
              {
                pattern: 'alpine',
              },
              {
                function: 'largerThan',
                args: [1000],
              },
            ],
            actions: {
              complex: true,
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].conditions).toHaveLength(3);
      expect(migrated.rules[0].conditions[0].kind).toBe('regex');
      expect(migrated.rules[0].conditions[1].kind).toBe('regex');
      expect(migrated.rules[0].conditions[2].kind).toBe('function');
    });

    it('should handle rules without matchers', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'no-matchers',
            priority: 70,
            actions: {
              always: true,
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].conditions).toEqual([]);
      expect(migrated.rules[0].actions.always).toBe(true);
    });

    it('should handle empty actions', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'no-actions',
            priority: 60,
            matchers: [
              {
                pattern: 'test',
              },
            ],
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].actions).toEqual({});
    });

    it('should preserve metadata during migration', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'preserve-data',
            priority: 100,
            matchers: [
              {
                pattern: '^LABEL',
                function: undefined,
                args: undefined,
              },
            ],
            actions: {
              nested: {
                data: {
                  value: 123,
                  flag: true,
                },
              },
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].actions.nested).toEqual({
        data: {
          value: 123,
          flag: true,
        },
      });
    });

    it('migrated policy should pass validation', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'valid-rule',
            priority: 85,
            matchers: [
              {
                pattern: 'RUN npm',
              },
              {
                function: 'hasPattern',
                args: ['test', 'i'],
              },
            ],
            actions: {
              optimize: true,
              cache: false,
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);
      const validationResult = validatePolicy(migrated);

      expect(validationResult.ok).toBe(true);
      if (validationResult.ok) {
        expect(validationResult.value.version).toBe('2.0');
      }
    });
  });

  describe('Complex Migration Scenarios', () => {
    it('should migrate policy with multiple rules', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'security-rule',
            priority: 100,
            matchers: [
              {
                pattern: 'USER root',
              },
            ],
            actions: {
              block: true,
            },
          },
          {
            id: 'quality-rule',
            priority: 80,
            matchers: [
              {
                pattern: ':latest',
              },
            ],
            actions: {
              warn: true,
            },
          },
          {
            id: 'performance-rule',
            priority: 60,
            matchers: [
              {
                function: 'largerThan',
                args: [50000000],
              },
            ],
            actions: {
              optimize: true,
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules).toHaveLength(3);
      expect(migrated.rules[0].id).toBe('security-rule');
      expect(migrated.rules[1].id).toBe('quality-rule');
      expect(migrated.rules[2].id).toBe('performance-rule');

      // Validate each rule's migration
      expect(migrated.rules[0].conditions[0].kind).toBe('regex');
      expect(migrated.rules[1].conditions[0].kind).toBe('regex');
      expect(migrated.rules[2].conditions[0].kind).toBe('function');
    });

    it('should handle malformed matchers gracefully', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'malformed',
            matchers: [
              {
                // No pattern or function specified
              } as any,
              {
                pattern: 'valid-pattern',
              },
            ],
            actions: {},
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      // Should skip invalid matcher but keep valid ones
      expect(migrated.rules[0].conditions).toHaveLength(1);
      expect(migrated.rules[0].conditions[0].kind).toBe('regex');
      expect((migrated.rules[0].conditions[0] as any).pattern).toBe('valid-pattern');
    });

    it('should add timestamp to migrated policy metadata', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.metadata).toBeDefined();
      expect(migrated.metadata?.created).toBeDefined();

      // Check if created is a valid ISO date string
      const createdDate = new Date(migrated.metadata!.created!);
      expect(createdDate).toBeInstanceOf(Date);
      expect(!isNaN(createdDate.getTime())).toBe(true);
    });

    it('should handle deeply nested actions', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'nested-actions',
            priority: 75,
            matchers: [],
            actions: {
              level1: {
                level2: {
                  level3: {
                    value: 'deep',
                    array: [1, 2, 3],
                    object: {
                      key: 'value',
                    },
                  },
                },
              },
            },
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].actions.level1).toBeDefined();
      expect((migrated.rules[0].actions.level1 as any).level2.level3.value).toBe('deep');
      expect((migrated.rules[0].actions.level1 as any).level2.level3.array).toEqual([1, 2, 3]);
      expect((migrated.rules[0].actions.level1 as any).level2.level3.object.key).toBe('value');
    });
  });

  describe('Migration Edge Cases', () => {
    it('should handle empty v1 policy', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.version).toBe('2.0');
      expect(migrated.rules).toEqual([]);
      expect(migrated.metadata).toBeDefined();
    });

    it('should handle v1 policy with undefined fields', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'undefined-fields',
            priority: undefined as any,
            matchers: undefined as any,
            actions: undefined as any,
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].id).toBe('undefined-fields');
      expect(migrated.rules[0].priority).toBe(50); // Default priority
      expect(migrated.rules[0].conditions).toEqual([]);
      expect(migrated.rules[0].actions).toEqual({});
    });

    it('should handle function args as empty array when undefined', () => {
      const v1Policy: LegacyPolicy = {
        version: '1.0',
        rules: [
          {
            id: 'no-args',
            matchers: [
              {
                function: 'fileExists',
                // args is undefined
              },
            ],
            actions: {},
          },
        ],
      };

      const migrated = migrateV1ToV2(v1Policy);

      expect(migrated.rules[0].conditions[0].kind).toBe('function');
      expect((migrated.rules[0].conditions[0] as any).args).toEqual([]);
    });
  });
});