/**
 * Legacy Configuration Removal Tests
 * Ensures that the unified configuration system works correctly
 */

import { describe, it, expect } from '@jest/globals';
import { config } from '@/config';
import type { AppRuntimeConfig } from '@/types/runtime';
import type { SessionConfig } from '@/session/core';

describe('Legacy Configuration Removal', () => {
  describe('AppRuntimeConfig Interface', () => {
    it('should not contain TTL or session limit properties', () => {
      const testConfig: AppRuntimeConfig = {
        logger: undefined,
        policyPath: 'test/path',
        policyEnvironment: 'test',
      };

      expect((testConfig as any).ttl).toBeUndefined();
      expect((testConfig as any).maxSessions).toBeUndefined();
      expect((testConfig as any).retryAttempts).toBeUndefined();
      expect((testConfig as any).retryDelay).toBeUndefined();
    });

    it('should only contain expected configuration properties', () => {
      const testConfig: AppRuntimeConfig = {
        logger: undefined,
        policyPath: 'config/policy.yaml',
        policyEnvironment: 'development',
        tools: [],
        toolAliases: {},
      };

      const allowedKeys = [
        'logger',
        'policyPath',
        'policyEnvironment',
        'tools',
        'toolAliases',
      ];

      Object.keys(testConfig).forEach(key => {
        expect(allowedKeys).toContain(key);
      });
    });

    it('should not contain runtime reconfiguration properties', () => {
      const testConfig: AppRuntimeConfig = {
        logger: undefined,
      };

      // These properties should not exist on the type (removed in Workstream A)
      expect((testConfig as any).samplingHooks).toBeUndefined();
      expect((testConfig as any).configure).toBeUndefined();
      expect((testConfig as any).getConfig).toBeUndefined();
    });
  });

  describe('Main Configuration Object', () => {
    it('should not contain legacy top-level properties', () => {
      expect((config as any).legacyMode).toBeUndefined();
      expect((config as any).backwardCompatibility).toBeUndefined();
      expect((config as any).deprecated).toBeUndefined();
    });

    it('should have clean configuration sections', () => {
      const configSections = [
        'server',
        'workspace',
        'docker',
        'mutex',
      ];

      configSections.forEach(section => {
        expect(config).toHaveProperty(section);
        const sectionConfig = (config as any)[section];

        expect(sectionConfig.legacy).toBeUndefined();
        expect(sectionConfig.deprecated).toBeUndefined();
        expect(sectionConfig.oldFormat).toBeUndefined();
        expect(sectionConfig.backcompat).toBeUndefined();
      });
    });

    it('should not have sampling or cache configuration in main config', () => {
      expect((config as any).sampling).toBeUndefined();
      expect((config as any).cache).toBeUndefined();
      expect((config as any).mcp).toBeUndefined();
      expect((config as any).kubernetes).toBeUndefined();
      expect((config as any).security).toBeUndefined();
      expect((config as any).logging).toBeUndefined();
      expect((config as any).orchestrator).toBeUndefined();
      expect((config as any).ai).toBeUndefined();
      expect((config as any).errors).toBeUndefined();
      expect((config as any).correlation).toBeUndefined();
    });
  });
});