/**
 * Tests for Default Suggestions and SuggestionRegistry
 */

import { describe, it, expect, beforeEach } from '@jest/globals';
import {
  createSuggestionRegistry,
  DEFAULT_SUGGESTION_GENERATORS,
  registerSuggestion,
  unregisterSuggestion,
  hasSuggestion,
  generateSuggestion,
  generateAllSuggestions,
  getRegisteredParams,
  clearSuggestions,
  resetSuggestions,
  setRegistryLogger,
  type SuggestionGenerator,
} from '@/mcp/ai/default-suggestions';
import { createMockLogger } from '../../../__support__/utilities/mock-factories';
import type { Logger } from 'pino';

describe('SuggestionRegistry', () => {
  let registry: ReturnType<typeof createSuggestionRegistry>;
  let mockLogger: Logger;

  beforeEach(() => {
    mockLogger = createMockLogger();
    resetSuggestions(); // Reset to defaults
    setRegistryLogger(mockLogger);
    registry = createSuggestionRegistry();
  });

  describe('basic functionality', () => {
    it('should generate path default', () => {
      const value = registry.generate('path', {});
      expect(value).toBe('.');
    });

    it('should generate imageId from appName', () => {
      const value = registry.generate('imageId', { appName: 'myapp' });
      expect(value).toBe('myapp:latest');
    });

    it('should generate imageId from name when appName is missing', () => {
      const value = registry.generate('imageId', { name: 'myservice' });
      expect(value).toBe('myservice:latest');
    });

    it('should generate imageId with custom tag', () => {
      const value = registry.generate('imageId', { appName: 'myapp', tag: 'v1.0.0' });
      expect(value).toBe('myapp:v1.0.0');
    });

    it('should use context for registry', () => {
      const value = registry.generate('registry', {}, { registry: 'docker.io' });
      expect(value).toBe('docker.io');
    });

    it('should use defaultRegistry from context if registry is missing', () => {
      const value = registry.generate('registry', {}, { defaultRegistry: 'gcr.io' });
      expect(value).toBe('gcr.io');
    });

    it('should generate namespace default', () => {
      const value = registry.generate('namespace', {});
      expect(value).toBe('default');
    });

    it('should use existing namespace if provided', () => {
      const value = registry.generate('namespace', { namespace: 'production' });
      expect(value).toBe('production');
    });

    it('should generate replicas default', () => {
      const value = registry.generate('replicas', {});
      expect(value).toBe(1);
    });

    it('should generate port default', () => {
      const value = registry.generate('port', {});
      expect(value).toBe(8080);
    });

    it('should use existing port if provided', () => {
      const value = registry.generate('port', { port: 3000 });
      expect(value).toBe(3000);
    });

    it('should generate dockerfile path', () => {
      const value = registry.generate('dockerfile', { path: '/app' });
      expect(value).toBe('/app/Dockerfile');
    });

    it('should generate labels', () => {
      const value = registry.generate('labels', { appName: 'myapp', version: 'v2.0' });
      expect(value).toEqual({
        app: 'myapp',
        version: 'v2.0',
      });
    });
  });

  describe('custom generators', () => {
    it('should allow registering custom generators', () => {
      const customGenerator: SuggestionGenerator = () => 'custom-value';
      registry.register('customParam', customGenerator);

      const value = registry.generate('customParam', {});
      expect(value).toBe('custom-value');
    });

    it('should override default generators', () => {
      const customPath: SuggestionGenerator = () => '/custom/path';
      registry.register('path', customPath);

      const value = registry.generate('path', {});
      expect(value).toBe('/custom/path');
    });

    it('should unregister generators', () => {
      registry.unregister('path');
      const value = registry.generate('path', {});
      expect(value).toBeUndefined();
    });

    it('should check if generator exists', () => {
      expect(registry.has('path')).toBe(true);
      expect(registry.has('nonexistent')).toBe(false);

      registry.register('custom', () => 'value');
      expect(registry.has('custom')).toBe(true);
    });
  });

  describe('generateAll', () => {
    it('should generate multiple suggestions', () => {
      const suggestions = registry.generateAll(['path', 'namespace', 'replicas'], {}, {});

      expect(suggestions).toEqual({
        path: '.',
        namespace: 'default',
        replicas: 1,
      });
    });

    it('should skip parameters that already have values', () => {
      const suggestions = registry.generateAll(
        ['path', 'namespace', 'replicas'],
        { path: '/existing/path', replicas: 3 },
        {},
      );

      expect(suggestions).toEqual({
        namespace: 'default',
      });
    });

    it('should handle missing generators gracefully', () => {
      const suggestions = registry.generateAll(['path', 'unknownParam', 'namespace'], {}, {});

      expect(suggestions).toEqual({
        path: '.',
        namespace: 'default',
      });
    });

    it('should use context in batch generation', () => {
      const suggestions = registry.generateAll(
        ['registry', 'cluster'],
        {},
        { registry: 'ecr.aws', cluster: 'prod-cluster' },
      );

      expect(suggestions).toEqual({
        registry: 'ecr.aws',
        cluster: 'prod-cluster',
      });
    });
  });

  describe('error handling', () => {
    it('should handle generator errors gracefully', () => {
      const errorGenerator: SuggestionGenerator = () => {
        throw new Error('Generator failed');
      };
      registry.register('errorParam', errorGenerator);

      const value = registry.generate('errorParam', {});
      expect(value).toBeUndefined();
    });

    it('should continue generating after error', () => {
      const errorGenerator: SuggestionGenerator = () => {
        throw new Error('Generator failed');
      };
      registry.register('errorParam', errorGenerator);

      const suggestions = registry.generateAll(['path', 'errorParam', 'namespace'], {}, {});

      expect(suggestions).toEqual({
        path: '.',
        namespace: 'default',
      });
    });
  });

  describe('registry management', () => {
    it('should get all registered parameter names', () => {
      const params = registry.getRegisteredParams();
      expect(params).toContain('path');
      expect(params).toContain('imageId');
      expect(params).toContain('namespace');
      expect(params).toContain('replicas');
    });

    it('should clear all generators', () => {
      registry.clear();
      const params = registry.getRegisteredParams();
      expect(params).toHaveLength(0);

      const value = registry.generate('path', {});
      expect(value).toBeUndefined();
    });

    it('should reset to default generators', () => {
      registry.register('custom', () => 'custom');
      registry.clear();
      registry.reset();

      const params = registry.getRegisteredParams();
      expect(params).toContain('path');
      expect(params).not.toContain('custom');

      const value = registry.generate('path', {});
      expect(value).toBe('.');
    });

    it('should extend registry with additional generators', () => {
      // Save current state
      const originalParams = getRegisteredParams();

      // Add new generators
      registerSuggestion('customParam', () => 'extended-value');
      registerSuggestion('path', () => '/extended/path');

      // Test new functionality
      expect(generateSuggestion('customParam', {})).toBe('extended-value');
      expect(generateSuggestion('path', {})).toBe('/extended/path');
      expect(generateSuggestion('namespace', {})).toBe('default');

      // Reset for next test
      resetSuggestions();
    });
  });

  describe('createSuggestionRegistry factory', () => {
    it('should create registry with defaults', () => {
      const registry = createSuggestionRegistry();
      const value = registry.generate('path', {});
      expect(value).toBe('.');
    });

    it('should create registry with custom generators', () => {
      const registry = createSuggestionRegistry({
        customParam: () => 'factory-custom',
        path: () => '/factory/path',
      });

      expect(registry.generate('customParam', {})).toBe('factory-custom');
      expect(registry.generate('path', {})).toBe('/factory/path');
      expect(registry.generate('namespace', {})).toBe('default');
    });

    it('should pass logger to registry', () => {
      const logger = createMockLogger();
      const registry = createSuggestionRegistry({}, logger);

      // Register a custom generator and check if logging occurs
      registry.register('test', () => 'value');
      expect(logger.debug).toHaveBeenCalledWith(
        { param: 'test' },
        'Registered custom suggestion generator',
      );
    });
  });

  describe('complex parameter generation', () => {
    it('should generate buildArgs as empty object', () => {
      const value = registry.generate('buildArgs', {});
      expect(value).toEqual({});
    });

    it('should generate volumeMounts as empty array', () => {
      const value = registry.generate('volumeMounts', {});
      expect(value).toEqual([]);
    });

    it('should generate health check paths', () => {
      expect(registry.generate('healthCheckPath', {})).toBe('/health');
      expect(registry.generate('readinessPath', {})).toBe('/ready');
      expect(registry.generate('livenessPath', {})).toBe('/health');
    });

    it('should generate resource limits', () => {
      expect(registry.generate('memory', {})).toBe('512Mi');
      expect(registry.generate('cpu', {})).toBe('500m');
    });

    it('should generate network configuration', () => {
      expect(registry.generate('serviceType', {})).toBe('ClusterIP');
      expect(registry.generate('protocol', {})).toBe('TCP');
      expect(registry.generate('targetPort', { port: 3000 })).toBe(3000);
    });
  });

  describe('performance', () => {
    it('should generate suggestions quickly', () => {
      const start = Date.now();

      for (let i = 0; i < 1000; i++) {
        registry.generateAll(
          ['path', 'imageId', 'namespace', 'replicas', 'port', 'labels'],
          { appName: `app${i}` },
          { registry: 'docker.io' },
        );
      }

      const elapsed = Date.now() - start;
      expect(elapsed).toBeLessThan(100); // Should complete in < 100ms
    });
  });
});
