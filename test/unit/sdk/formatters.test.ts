/**
 * SDK Formatters Tests
 *
 * Tests for SDK result formatters, particularly for discriminated union types
 * that use type-safe narrowing via the `kind` field.
 */

import { describe, it, expect } from '@jest/globals';
import {
  resultFormatters,
  formatOpsResult,
  formatVerifyDeployResult,
} from '../../../src/sdk/formatters';
import type {
  PingResult,
  ServerStatusResult,
  OpsResult,
  VerifyDeploymentResult,
} from '../../../src/sdk/types';

describe('SDK Formatters', () => {
  describe('formatOpsResult', () => {
    describe('PingResult formatting', () => {
      it('should format PingResult with summary using kind field', () => {
        const result: PingResult = {
          kind: 'ping',
          success: true,
          message: 'pong: test',
          timestamp: '2025-01-15T10:30:00Z',
          summary: '✅ Server is responsive. Ping successful.',
          server: {
            name: 'containerization-assist-mcp',
            version: '2.0.0',
            uptime: 3600,
            pid: 12345,
          },
          capabilities: {
            tools: true,
            progress: true,
          },
        };

        const formatted = formatOpsResult(result);

        expect(formatted).toContain('## Ping Result');
        expect(formatted).toContain('Server is responsive');
      });

      it('should format PingResult without summary using kind field', () => {
        const result: PingResult = {
          kind: 'ping',
          success: true,
          message: 'pong: hello',
          timestamp: '2025-01-15T10:30:00Z',
          server: {
            name: 'containerization-assist-mcp',
            version: '2.0.0',
            uptime: 3600,
            pid: 12345,
          },
          capabilities: {
            tools: true,
            progress: true,
          },
        };

        const formatted = formatOpsResult(result);

        expect(formatted).toContain('## Ping Result');
        expect(formatted).toContain('pong: hello');
        expect(formatted).toContain('containerization-assist-mcp');
        expect(formatted).toContain('v2.0.0');
      });

      it('should format PingResult as JSON when asJson is true', () => {
        const result: PingResult = {
          kind: 'ping',
          success: true,
          message: 'pong: test',
          timestamp: '2025-01-15T10:30:00Z',
          server: {
            name: 'containerization-assist-mcp',
            version: '2.0.0',
            uptime: 3600,
            pid: 12345,
          },
          capabilities: {
            tools: true,
            progress: true,
          },
        };

        const formatted = formatOpsResult(result, { asJson: true });
        const parsed = JSON.parse(formatted);

        expect(parsed.kind).toBe('ping');
        expect(parsed.message).toBe('pong: test');
        expect(parsed.server.name).toBe('containerization-assist-mcp');
      });
    });

    describe('ServerStatusResult formatting', () => {
      it('should format ServerStatusResult with summary using kind field', () => {
        const result: ServerStatusResult = {
          kind: 'status',
          success: true,
          version: '2.0.0',
          uptime: 7200,
          summary: '✅ Server healthy. Running for 2h. Memory: 45% used.',
          memory: {
            used: 4500000000,
            total: 10000000000,
            free: 5500000000,
            percentage: 45,
          },
          cpu: {
            model: 'Intel Xeon',
            cores: 4,
            loadAverage: [1.5, 1.2, 1.0],
          },
          system: {
            platform: 'linux',
            release: '5.15.0',
            hostname: 'server-1',
          },
          tools: {
            count: 14,
            migrated: 12,
          },
        };

        const formatted = formatOpsResult(result);

        expect(formatted).toContain('## Server Status');
        expect(formatted).toContain('Server healthy');
      });

      it('should format ServerStatusResult without summary using kind field', () => {
        const result: ServerStatusResult = {
          kind: 'status',
          success: true,
          version: '2.0.0',
          uptime: 7200,
          memory: {
            used: 4500000000,
            total: 10000000000,
            free: 5500000000,
            percentage: 45,
          },
          cpu: {
            model: 'Intel Xeon',
            cores: 4,
            loadAverage: [1.5, 1.2, 1.0],
          },
          system: {
            platform: 'linux',
            release: '5.15.0',
            hostname: 'server-1',
          },
          tools: {
            count: 14,
            migrated: 12,
          },
        };

        const formatted = formatOpsResult(result);

        expect(formatted).toContain('## Server Status');
        expect(formatted).toContain('Server is healthy');
        expect(formatted).toContain('**Version**: 2.0.0');
        expect(formatted).toContain('**Uptime**:');
        expect(formatted).toContain('**Memory**: 45% used');
        expect(formatted).toContain('**CPU Cores**: 4');
      });

      it('should format ServerStatusResult as JSON when asJson is true', () => {
        const result: ServerStatusResult = {
          kind: 'status',
          success: true,
          version: '2.0.0',
          uptime: 7200,
          memory: {
            used: 4500000000,
            total: 10000000000,
            free: 5500000000,
            percentage: 45,
          },
          cpu: {
            model: 'Intel Xeon',
            cores: 4,
            loadAverage: [1.5, 1.2, 1.0],
          },
          system: {
            platform: 'linux',
            release: '5.15.0',
            hostname: 'server-1',
          },
          tools: {
            count: 14,
            migrated: 12,
          },
        };

        const formatted = formatOpsResult(result, { asJson: true });
        const parsed = JSON.parse(formatted);

        expect(parsed.kind).toBe('status');
        expect(parsed.version).toBe('2.0.0');
        expect(parsed.memory.percentage).toBe(45);
        expect(parsed.cpu.cores).toBe(4);
      });
    });

    describe('Type narrowing', () => {
      it('should correctly narrow PingResult type via kind field', () => {
        const result: OpsResult = {
          kind: 'ping',
          success: true,
          message: 'pong: test',
          timestamp: '2025-01-15T10:30:00Z',
          server: {
            name: 'containerization-assist-mcp',
            version: '2.0.0',
            uptime: 3600,
            pid: 12345,
          },
          capabilities: {
            tools: true,
            progress: true,
          },
        };

        // Type narrowing should work
        if (result.kind === 'ping') {
          expect(result.message).toBe('pong: test');
          expect(result.server.name).toBe('containerization-assist-mcp');
        }
      });

      it('should correctly narrow ServerStatusResult type via kind field', () => {
        const result: OpsResult = {
          kind: 'status',
          success: true,
          version: '2.0.0',
          uptime: 7200,
          memory: {
            used: 4500000000,
            total: 10000000000,
            free: 5500000000,
            percentage: 45,
          },
          cpu: {
            model: 'Intel Xeon',
            cores: 4,
            loadAverage: [1.5, 1.2, 1.0],
          },
          system: {
            platform: 'linux',
            release: '5.15.0',
            hostname: 'server-1',
          },
          tools: {
            count: 14,
            migrated: 12,
          },
        };

        // Type narrowing should work
        if (result.kind === 'status') {
          expect(result.version).toBe('2.0.0');
          expect(result.memory.percentage).toBe(45);
          expect(result.cpu.cores).toBe(4);
        }
      });
    });
  });

  describe('resultFormatters registry', () => {
    it('should have ops formatter registered', () => {
      expect(resultFormatters.ops).toBeDefined();
      expect(typeof resultFormatters.ops).toBe('function');
    });

    it('should format ops results via registry', () => {
      const result: PingResult = {
        kind: 'ping',
        success: true,
        message: 'pong: registry-test',
        timestamp: '2025-01-15T10:30:00Z',
        server: {
          name: 'containerization-assist-mcp',
          version: '2.0.0',
          uptime: 3600,
          pid: 12345,
        },
        capabilities: {
          tools: true,
          progress: true,
        },
      };

      const formatted = resultFormatters.ops(result);

      expect(formatted).toContain('## Ping Result');
      expect(formatted).toContain('pong: registry-test');
    });
  });

  describe('formatVerifyDeployResult', () => {
    const createVerifyDeployResult = (
      overrides: Partial<VerifyDeploymentResult> = {},
    ): VerifyDeploymentResult => ({
      success: true,
      namespace: 'default',
      deploymentName: 'my-app',
      serviceName: 'my-app-service',
      endpoints: [],
      ready: true,
      replicas: 3,
      status: {
        readyReplicas: 3,
        totalReplicas: 3,
        conditions: [],
      },
      ...overrides,
    });

    it('should format healthy deployment result', () => {
      const result = createVerifyDeployResult({ ready: true });

      const formatted = formatVerifyDeployResult(result);

      expect(formatted).toContain('Deployment Verification');
      expect(formatted).toContain('my-app');
      expect(formatted).toContain('Ready');
      expect(formatted).toContain('3/3 ready');
    });

    it('should format failing deployment result', () => {
      const result = createVerifyDeployResult({
        ready: false,
        status: {
          readyReplicas: 1,
          totalReplicas: 3,
          conditions: [
            {
              type: 'Available',
              status: 'False',
              message: 'Deployment does not have minimum availability',
            },
          ],
        },
      });

      const formatted = formatVerifyDeployResult(result);

      expect(formatted).toContain('Deployment Verification');
      expect(formatted).toContain('Not Ready');
      expect(formatted).toContain('1/3 ready');
      expect(formatted).toContain('Deployment does not have minimum availability');
    });

    it('should include suggested next step for failing deployment when enabled', () => {
      const result = createVerifyDeployResult({
        ready: false,
        status: {
          readyReplicas: 0,
          totalReplicas: 3,
          conditions: [],
        },
      });

      const formatted = formatVerifyDeployResult(result, { includeSuggestedNext: true });

      expect(formatted).toContain('Suggested Next Step');
      expect(formatted).toContain('kubectl logs');
      expect(formatted).toContain('kubectl describe pod');
    });

    it('should not include suggested next step for healthy deployment', () => {
      const result = createVerifyDeployResult({ ready: true });

      const formatted = formatVerifyDeployResult(result, { includeSuggestedNext: true });

      expect(formatted).not.toContain('Suggested Next Step');
    });

    it('should not include suggested next step when option is disabled', () => {
      const result = createVerifyDeployResult({
        ready: false,
        status: {
          readyReplicas: 0,
          totalReplicas: 3,
          conditions: [],
        },
      });

      const formatted = formatVerifyDeployResult(result, { includeSuggestedNext: false });

      expect(formatted).not.toContain('Suggested Next Step');
    });

    it('should format deployment with endpoints', () => {
      const result = createVerifyDeployResult({
        endpoints: [
          { type: 'internal', url: 'my-app-service', port: 8080, healthy: true },
          { type: 'external', url: '192.168.1.100', port: 80, healthy: false },
        ],
      });

      const formatted = formatVerifyDeployResult(result);

      expect(formatted).toContain('Endpoints');
      expect(formatted).toContain('my-app-service');
      expect(formatted).toContain('8080');
      expect(formatted).toContain('192.168.1.100');
    });

    it('should format deployment with health check', () => {
      const result = createVerifyDeployResult({
        healthCheck: {
          status: 'healthy',
          message: 'All health checks passing',
        },
      });

      const formatted = formatVerifyDeployResult(result);

      expect(formatted).toContain('Health Check');
      expect(formatted).toContain('healthy');
      expect(formatted).toContain('All health checks passing');
    });

    it('should format as JSON when asJson option is true', () => {
      const result = createVerifyDeployResult();

      const formatted = formatVerifyDeployResult(result, { asJson: true });
      const parsed = JSON.parse(formatted);

      expect(parsed.deploymentName).toBe('my-app');
      expect(parsed.namespace).toBe('default');
      expect(parsed.ready).toBe(true);
    });
  });
});
