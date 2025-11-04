/**
 * Unit tests for telemetry sanitization utilities
 */

import { describe, expect, it } from '@jest/globals';

import {
  createSafeTelemetryEvent,
  extractSafeTelemetryMetrics,
  hashValue,
  sanitizePath,
  sanitizeToolInput,
} from '@/lib/telemetry-utils';
import type { ToolName } from '@/tools';

describe('telemetry-utils', () => {
  describe('hashValue', () => {
    it('should produce consistent hashes', () => {
      const hash1 = hashValue('test-value');
      const hash2 = hashValue('test-value');
      expect(hash1).toBe(hash2);
      expect(hash1).toHaveLength(16);
    });

    it('should produce different hashes for different values', () => {
      expect(hashValue('value1')).not.toBe(hashValue('value2'));
    });

    it('should produce deterministic hashes', () => {
      const value = '/home/user/secret-project';
      const hash = hashValue(value);
      // Verify it's a hex string
      expect(hash).toMatch(/^[0-9a-f]{16}$/);
      // Verify it's always the same for the same input
      expect(hashValue(value)).toBe(hash);
    });
  });

  describe('sanitizePath', () => {
    it('should hash sensitive path segments and preserve last 2 for long paths', () => {
      const result = sanitizePath('/home/user/myproject/src/index.ts');
      expect(result).not.toContain('user');
      expect(result).not.toContain('myproject');
      expect(result).toContain('src/index.ts'); // Last 2 segments preserved (5 segments total)
    });

    it('should preserve last 2 path segments for context in long paths', () => {
      const result = sanitizePath('/home/john.doe/acme-corp/backend-api/src/server.ts');
      expect(result).toContain('src/server.ts');
      expect(result).not.toContain('john.doe');
      expect(result).not.toContain('acme-corp');
      expect(result).not.toContain('backend-api');
    });

    it('should handle relative paths', () => {
      const result = sanitizePath('./src/index.ts');
      expect(result).toBe('./src/index.ts');
    });

    it('should handle parent directory references', () => {
      const result = sanitizePath('../src/index.ts');
      expect(result).toBe('../src/index.ts');
    });

    it('should hash all segments for short paths (4 or fewer)', () => {
      // 2 segments - hash all
      const result1 = sanitizePath('/etc/config');
      expect(result1).toBe('/<' + hashValue('etc') + '>/<' + hashValue('config') + '>');
      expect(result1).not.toContain('etc');
      expect(result1).not.toContain('config');

      // 2 segments - hash all
      const result2 = sanitizePath('/home/user');
      expect(result2).not.toContain('home');
      expect(result2).not.toContain('user');

      // 3 segments - hash all
      const result3 = sanitizePath('/home/user/myapp');
      expect(result3).not.toContain('home');
      expect(result3).not.toContain('user');
      expect(result3).not.toContain('myapp');

      // 4 segments - hash all (security-first: don't preserve potentially sensitive context)
      const result4 = sanitizePath('/home/user/myapp/src');
      expect(result4).not.toContain('home');
      expect(result4).not.toContain('user');
      expect(result4).not.toContain('myapp');
      expect(result4).not.toContain('src');
    });

    it('should preserve last 2 segments only for 5+ segment paths', () => {
      // 5 segments - preserve last 2
      const result5 = sanitizePath('/home/user/myapp/src/index.ts');
      expect(result5).toContain('src/index.ts');
      expect(result5).not.toContain('home');
      expect(result5).not.toContain('user');
      expect(result5).not.toContain('myapp');

      // 6 segments - preserve last 2
      const result6 = sanitizePath('/home/user/company/project/src/index.ts');
      expect(result6).toContain('src/index.ts');
      expect(result6).not.toContain('home');
      expect(result6).not.toContain('user');
      expect(result6).not.toContain('company');
      expect(result6).not.toContain('project');
    });
  });

  describe('sanitizeToolInput', () => {
    it('should hash path fields', () => {
      const result = sanitizeToolInput({
        path: '/home/user/myapp',
        imageName: 'my-company/my-app:latest',
      });

      // Path has 3 segments, so all should be hashed
      expect(result.path).not.toContain('home');
      expect(result.path).not.toContain('user');
      expect(result.path).not.toContain('myapp');
      expect(typeof result.imageName).toBe('string');
      expect((result.imageName as string).length).toBe(16); // Hashed
    });

    it('should preserve safe enum values', () => {
      const result = sanitizeToolInput({
        imageName: 'test-image',
        skipVendors: true,
        severity: 'high',
      });

      expect(result.skipVendors).toBe(true);
      expect(result.severity).toBe('high'); // Known safe enum
      expect(typeof result.imageName).toBe('string');
      expect((result.imageName as string).length).toBe(16); // Hashed (sensitive)
    });

    it('should hash dockerfile paths', () => {
      const result = sanitizeToolInput({
        dockerfile: '/home/user/project/Dockerfile',
      });

      // Path has 4 segments, so all should be hashed (security-first)
      expect(result.dockerfile).not.toContain('home');
      expect(result.dockerfile).not.toContain('user');
      expect(result.dockerfile).not.toContain('project');
      expect(result.dockerfile).not.toContain('Dockerfile');
    });

    it('should hash manifest paths', () => {
      const result = sanitizeToolInput({
        manifestPath: '/home/user/k8s/deployment.yaml',
      });

      // Path has 4 segments, so all should be hashed (security-first)
      expect(result.manifestPath).not.toContain('home');
      expect(result.manifestPath).not.toContain('user');
      expect(result.manifestPath).not.toContain('k8s');
      expect(result.manifestPath).not.toContain('deployment.yaml');
    });

    it('should hash registry URLs', () => {
      const result = sanitizeToolInput({
        imageName: 'myapp:latest',
        registryUrl: 'myregistry.azurecr.io',
      });

      expect(typeof result.registryUrl).toBe('string');
      expect((result.registryUrl as string).length).toBe(16); // Hashed
      expect(result.registryUrl).not.toContain('azurecr');
    });

    it('should omit complex objects', () => {
      const result = sanitizeToolInput({
        path: '/home/user/repo',
        options: { deep: true, include: ['*.ts'] },
      });

      expect(result.options).toBe('<omitted>');
    });

    it('should preserve numbers and booleans', () => {
      const result = sanitizeToolInput({
        replicas: 3,
        wait: true,
        timeout: 300,
      });

      expect(result.replicas).toBe(3);
      expect(result.wait).toBe(true);
      expect(result.timeout).toBe(300);
    });

    it('should hash deployment names and namespaces', () => {
      const result = sanitizeToolInput({
        deploymentName: 'my-app-deployment',
        namespace: 'production',
      });

      expect(typeof result.deploymentName).toBe('string');
      expect((result.deploymentName as string).length).toBe(16); // Hashed
      expect(result.deploymentName).not.toContain('deployment');

      expect(typeof result.namespace).toBe('string');
      expect((result.namespace as string).length).toBe(16); // Hashed
      expect(result.namespace).not.toContain('production');
    });

    it('should hash unknown string fields by default', () => {
      const result = sanitizeToolInput({
        unknownField: 'some-sensitive-value',
        platform: 'linux/amd64', // Known safe field
      });

      // Unknown string fields are hashed for safety
      expect(typeof result.unknownField).toBe('string');
      expect((result.unknownField as string).length).toBe(16); // Hashed
      expect(result.unknownField).not.toContain('sensitive');

      // Known safe fields are preserved
      expect(result.platform).toBe('linux/amd64');
    });
  });

  describe('extractSafeTelemetryMetrics', () => {
    it('should extract safe metrics from analyze-repo results', () => {
      const metrics = extractSafeTelemetryMetrics('analyze-repo' as ToolName, {
        framework: 'node',
        language: 'typescript',
        moduleCount: 42,
        path: '/home/user/repo', // Should not be extracted
      });

      expect(metrics.framework).toBe('node');
      expect(metrics.language).toBe('typescript');
      expect(metrics.moduleCount).toBe(42);
      expect(metrics).not.toHaveProperty('path');
    });

    it('should extract safe metrics from build-image results', () => {
      const metrics = extractSafeTelemetryMetrics('build-image' as ToolName, {
        imageId: 'sha256:abc123',
        size: 245678234,
        buildTime: 45000,
        tags: ['myapp:latest'],
      });

      expect(metrics.imageSizeBytes).toBe(245678234);
      expect(metrics.buildTimeMs).toBe(45000);
      expect(metrics).not.toHaveProperty('imageId'); // Sensitive
      expect(metrics).not.toHaveProperty('tags'); // Sensitive
    });

    it('should extract safe metrics from scan-image results', () => {
      const metrics = extractSafeTelemetryMetrics('scan-image' as ToolName, {
        summary: { critical: 2, high: 5, medium: 10, low: 20 },
        vulnerabilities: [
          /* ... */
        ],
      });

      expect(metrics.criticalVulns).toBe(2);
      expect(metrics.highVulns).toBe(5);
      expect(metrics.mediumVulns).toBe(10);
      expect(metrics.lowVulns).toBe(20);
      expect(metrics).not.toHaveProperty('vulnerabilities');
    });

    it('should handle missing vulnerability counts', () => {
      const metrics = extractSafeTelemetryMetrics('scan-image' as ToolName, {
        summary: { critical: 0 },
      });

      expect(metrics.criticalVulns).toBe(0);
      expect(metrics.highVulns).toBe(0);
      expect(metrics.mediumVulns).toBe(0);
      expect(metrics.lowVulns).toBe(0);
    });

    it('should extract safe metrics from verify-deploy results', () => {
      const metrics = extractSafeTelemetryMetrics('verify-deploy' as ToolName, {
        namespace: 'production',
        deploymentName: 'myapp',
        replicas: 3,
        readyReplicas: 3,
        status: 'ready',
      });

      expect(metrics.replicas).toBe(3);
      expect(metrics.readyReplicas).toBe(3);
      expect(metrics.status).toBe('ready');
      expect(metrics).not.toHaveProperty('namespace'); // Sensitive
      expect(metrics).not.toHaveProperty('deploymentName'); // Sensitive
    });

    it('should handle unknown tools gracefully', () => {
      const metrics = extractSafeTelemetryMetrics('ops' as ToolName, {
        someResult: 'value',
      });

      expect(metrics.hasResult).toBe(true);
      expect(metrics).not.toHaveProperty('someResult');
    });
  });

  describe('createSafeTelemetryEvent', () => {
    it('should create complete safe telemetry event for success', () => {
      const event = createSafeTelemetryEvent(
        'build-image' as ToolName,
        { path: '/home/user/app', imageName: 'myapp' },
        { ok: true, value: { imageId: 'abc', size: 1000, buildTime: 5000, tags: [] } },
        5000,
      );

      expect(event.toolName).toBe('build-image');
      expect(event.success).toBe(true);
      expect(event.durationMs).toBe(5000);
      // Path has 3 segments, so all should be hashed
      expect(event.sanitizedInput.path).not.toContain('home');
      expect(event.sanitizedInput.path).not.toContain('user');
      expect(event.sanitizedInput.path).not.toContain('app');
      expect(event.metrics.imageSizeBytes).toBe(1000);
      expect(event.metrics.buildTimeMs).toBe(5000);
      expect(event.metrics).not.toHaveProperty('imageId');
      expect(event.errorType).toBeUndefined();
    });

    it('should create complete safe telemetry event for error', () => {
      const event = createSafeTelemetryEvent(
        'build-image' as ToolName,
        { path: '/home/user/app', imageName: 'myapp' },
        { ok: false, error: 'Build failed: file not found' },
        2000,
      );

      expect(event.toolName).toBe('build-image');
      expect(event.success).toBe(false);
      expect(event.durationMs).toBe(2000);
      // Path has 3 segments, so all should be hashed
      expect(event.sanitizedInput.path).not.toContain('home');
      expect(event.sanitizedInput.path).not.toContain('user');
      expect(event.sanitizedInput.path).not.toContain('app');
      expect(event.metrics).toEqual({});
      expect(event.errorType).toBe('Error');
    });

    it('should handle missing duration', () => {
      const event = createSafeTelemetryEvent(
        'scan-image' as ToolName,
        { imageName: 'test:latest' },
        { ok: true, value: { summary: { critical: 0 } } },
      );

      expect(event.durationMs).toBeUndefined();
      expect(event.success).toBe(true);
    });

    it('should sanitize complex tool inputs', () => {
      const event = createSafeTelemetryEvent(
        'verify-deploy' as ToolName,
        {
          deploymentName: 'my-deployment',
          namespace: 'production',
          replicas: 5,
          wait: true,
        },
        { ok: true, value: { replicas: 5, readyReplicas: 5, status: 'ready' } },
      );

      // Sensitive fields should be hashed
      expect(typeof event.sanitizedInput.deploymentName).toBe('string');
      expect(typeof event.sanitizedInput.namespace).toBe('string');

      // Safe primitives should be preserved
      expect(event.sanitizedInput.replicas).toBe(5);
      expect(event.sanitizedInput.wait).toBe(true);

      // Metrics should only include safe data
      expect(event.metrics.replicas).toBe(5);
      expect(event.metrics.readyReplicas).toBe(5);
      expect(event.metrics.status).toBe('ready');
    });
  });

  describe('security verification', () => {
    it('should never expose customer paths in sanitized input', () => {
      const sensitiveInput = {
        path: '/home/john.doe/acme-corp/secret-project/src',
        dockerfile: '/home/john.doe/acme-corp/secret-project/Dockerfile',
        contextPath: '/home/john.doe/acme-corp/secret-project',
      };

      const result = sanitizeToolInput(sensitiveInput);

      // Verify no sensitive strings appear anywhere
      const resultStr = JSON.stringify(result);
      expect(resultStr).not.toContain('john.doe');
      expect(resultStr).not.toContain('acme-corp');
      // Note: For 5-segment paths, last 2 segments (secret-project/src) are preserved
      // For 4-segment paths (contextPath), all are hashed including secret-project
    });

    it('should never expose customer identifiers in metrics', () => {
      const sensitiveResult = {
        imageId: 'sha256:abc123def456',
        tags: ['acme-corp/secret-app:v1.2.3', 'acme-corp/secret-app:latest'],
        namespace: 'acme-production',
        deploymentName: 'secret-api-v2',
        size: 123456789,
        buildTime: 45000,
      };

      const metrics = extractSafeTelemetryMetrics('build-image' as ToolName, sensitiveResult);

      // Verify only safe metrics are included
      const metricsStr = JSON.stringify(metrics);
      expect(metricsStr).not.toContain('sha256');
      expect(metricsStr).not.toContain('acme-corp');
      expect(metricsStr).not.toContain('secret-app');
      expect(metricsStr).not.toContain('secret-api');

      // Verify safe metrics are present
      expect(metrics.imageSizeBytes).toBe(123456789);
      expect(metrics.buildTimeMs).toBe(45000);
    });

    it('should create telemetry events with no customer data leakage', () => {
      const event = createSafeTelemetryEvent(
        'build-image' as ToolName,
        {
          path: '/home/john.doe/acme-corp/payment-service',
          imageName: 'acme-corp/payment-api:v2.1.0',
        },
        {
          ok: true,
          value: {
            imageId: 'sha256:payment123',
            tags: ['acme-corp/payment-api:v2.1.0'],
            size: 250000000,
            buildTime: 55000,
          },
        },
      );

      // Verify the entire event contains no customer identifiers
      const eventStr = JSON.stringify(event);
      expect(eventStr).not.toContain('john.doe');
      expect(eventStr).not.toContain('acme-corp');
      // Path has 4 segments, so all should be hashed (including payment-service)
      expect(eventStr).not.toContain('payment-service');
      expect(eventStr).not.toContain('sha256');

      // Verify safe metrics are present
      expect(event.metrics.imageSizeBytes).toBe(250000000);
      expect(event.metrics.buildTimeMs).toBe(55000);

      // Verify imageName is hashed (not exposed in metrics)
      expect(event.sanitizedInput.imageName).not.toContain('acme-corp');
      expect(event.sanitizedInput.imageName).not.toContain('payment');
    });
  });
});
