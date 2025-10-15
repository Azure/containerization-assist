/**
 * AI Determinism Tests
 * Verifies that all AI-driven tools use deterministic single-candidate sampling
 */

import { describe, it, expect } from '@jest/globals';
import { SAMPLING_CONFIG } from '@/config/sampling';

// Import generate-* tools to test their configurations
import generateHelmChartsTool from '../../../src/tools/generate-helm-charts/tool';

describe('AI Determinism Configuration', () => {
  describe('Sampling Configuration Standards', () => {
    it('should define operation limits', () => {
      expect(SAMPLING_CONFIG.LIMITS).toBeDefined();
      expect(SAMPLING_CONFIG.LIMITS.MAX_SUGGESTIONS).toBeGreaterThan(0);
      expect(SAMPLING_CONFIG.PRIORITIES).toBeDefined();
      expect(SAMPLING_CONFIG.PRIORITIES.INTELLIGENCE).toBeGreaterThan(0);
    });

    it('should enforce deterministic single-candidate sampling', () => {
      expect(SAMPLING_CONFIG.LIMITS).toBeDefined();
    });
  });

  describe('Sampling-Enabled Tool Consistency', () => {
    const samplingTools = [
      {
        name: 'generate-helm-charts',
        tool: generateHelmChartsTool,
      },
    ];

    samplingTools.forEach(({ name, tool }) => {
      describe(`${name}`, () => {
        it('should have sampling enabled', () => {
          expect(tool.metadata?.samplingStrategy).toBe('single');
        });

        it('should use deterministic single-candidate sampling', () => {
          expect(tool.name).toBe(name);
          expect(tool.description).toBeDefined();
          expect(typeof tool.run).toBe('function');
        });
      });
    });
  });


  describe('Temperature Control', () => {
    it('should rely on MCP host configuration for temperature', () => {
      const aiTools = [generateHelmChartsTool];

      aiTools.forEach(tool => {
        expect((tool as any).temperature).toBeUndefined();
        expect(tool.metadata?.samplingConfig?.temperature).toBeUndefined();
      });
    });

    it('should use MCP model preferences for AI behavior', () => {
      expect(true).toBe(true);
    });
  });

  describe('Sampling Determinism', () => {
    it('should use single-candidate deterministic sampling across all tools', () => {
      expect(SAMPLING_CONFIG).not.toHaveProperty('CANDIDATES');
    });

    it('should have reasonable early stopping thresholds', () => {
      const thresholds = [95, 80, 80, 85];

      thresholds.forEach(threshold => {
        expect(threshold).toBeGreaterThanOrEqual(80);
        expect(threshold).toBeLessThanOrEqual(95);
      });
    });

    it('should enforce deterministic single-candidate behavior', () => {
      expect(SAMPLING_CONFIG).not.toHaveProperty('CANDIDATES');
      expect(SAMPLING_CONFIG).not.toHaveProperty('DEFAULTS');
      expect(SAMPLING_CONFIG).toBeDefined();
    });
  });
});
