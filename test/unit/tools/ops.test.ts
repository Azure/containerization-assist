/**
 * Unit Tests: Ops Tool
 * Tests the operations tool functionality with ping and server status operations
 */

import { jest } from '@jest/globals';
import opsToolNew from '@/tools/ops/tool';
import type { OpsToolParams } from '@/tools/ops/schema';
import { createMockLogger } from '../../__support__/utilities/mock-factories';

// Mock timer functionality
const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

jest.mock('@/lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
}));

jest.mock('@/lib/tool-helpers', () => ({
  createToolTimer: jest.fn(() => mockTimer),
  getToolLogger: jest.fn((context: any) => context.logger || {
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  }),
}));

describe('opsTool', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;

  beforeEach(() => {
    mockLogger = createMockLogger();
    jest.clearAllMocks();
  });

  describe('ping operation', () => {
    it('should return pong response with server details', async () => {
      const config: OpsToolParams = {
        operation: 'ping',
        message: 'test-ping',
      };

      const result = await opsToolNew.handler(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      if (result.ok) {
        const data = result.value as any;
        expect(data.kind).toBe('ping');
        expect(data.success).toBe(true);
        expect(data.message).toBe('pong: test-ping');
        expect(data.timestamp).toBeDefined();
        expect(data.server).toEqual({
          name: 'containerization-assist-mcp',
          version: '2.0.0',
          uptime: expect.any(Number),
          pid: expect.any(Number),
        });
        expect(data.capabilities).toEqual({
          tools: true,
          progress: true,
        });
      }
    });

    it('should use default message when none provided', async () => {
      const config: OpsToolParams = {
        operation: 'ping',
      };

      const result = await opsToolNew.handler(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      if (result.ok) {
        const data = result.value as any;
        expect(data.message).toBe('pong: ping');
      }
    });

    it('should log ping request', async () => {
      const config: OpsToolParams = {
        operation: 'ping',
        message: 'test-message',
      };

      await opsToolNew.handler(config, { logger: mockLogger });

      expect(mockLogger.info).toHaveBeenCalledWith(
        { message: 'test-message' },
        'Processing ping request',
      );
    });
  });

  describe('status operation', () => {
    it('should return comprehensive server status', async () => {
      const config: OpsToolParams = {
        operation: 'status',
        details: true,
      };

      const result = await opsToolNew.handler(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      if (result.ok) {
        const data = result.value as any;
        expect(data.kind).toBe('status');
        expect(data.success).toBe(true);
        expect(data.version).toBe('2.0.0');
        expect(data.uptime).toBeGreaterThanOrEqual(0);
        expect(data.memory).toEqual({
          used: expect.any(Number),
          total: expect.any(Number),
          free: expect.any(Number),
          percentage: expect.any(Number),
        });
        expect(data.cpu).toMatchObject({
          model: expect.any(String),
          cores: expect.any(Number),
          loadAverage: expect.arrayContaining([expect.any(Number)]),
        });
        expect(data.system).toEqual({
          platform: expect.any(String),
          release: expect.any(String),
          hostname: expect.any(String),
        });
        expect(data.tools).toEqual({
          count: 14,
          migrated: 12,
        });
      }
    });

    it('should handle status request without details', async () => {
      const config: OpsToolParams = {
        operation: 'status',
        details: false,
      };

      const result = await opsToolNew.handler(config, { logger: mockLogger });

      expect(result.ok).toBe(true);
      expect(mockLogger.info).toHaveBeenCalledWith({ details: false }, 'Server status requested');
    });

    it('should log server status compilation', async () => {
      const config: OpsToolParams = {
        operation: 'status',
      };

      await opsToolNew.handler(config, { logger: mockLogger });

      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          uptime: expect.any(Number),
          memoryUsed: expect.any(Number),
          memoryPercentage: expect.any(Number),
          toolsMigrated: 12,
        }),
        'Server status compiled',
      );
    });
  });

  describe('invalid operation', () => {
    it('should return failure for unknown operation', async () => {
      const config = {
        operation: 'invalid',
      } as any;

      const result = await opsToolNew.handler(config, { logger: mockLogger });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Unknown operation: invalid');
      }
    });
  });

  describe('timer usage', () => {
    it('should end timer on successful ping', async () => {
      const config: OpsToolParams = {
        operation: 'ping',
      };

      await opsToolNew.handler(config, { logger: mockLogger });

      expect(mockTimer.end).toHaveBeenCalled();
    });

    it('should end timer on successful status', async () => {
      const config: OpsToolParams = {
        operation: 'status',
      };

      await opsToolNew.handler(config, { logger: mockLogger });

      expect(mockTimer.end).toHaveBeenCalled();
    });
  });
});
