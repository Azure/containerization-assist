/**
 * Unit Tests: Docker Context Resolution
 * Tests the Docker context resolution utility
 */

import { jest, describe, it, expect, beforeEach } from '@jest/globals';

// Mock child_process before importing the module
const mockExecFile = jest.fn<() => Promise<{ stdout: string; stderr: string }>>();

jest.mock('node:child_process', () => ({
  execFile: mockExecFile,
}));

jest.mock('node:util', () => ({
  promisify: () => mockExecFile,
}));

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

import { resolveDockerContext } from '../../../../src/infra/docker/context';

describe('Docker Context Resolution', () => {
  let logger: any;

  beforeEach(() => {
    jest.clearAllMocks();
    logger = createMockLogger();
  });

  describe('resolveDockerContext', () => {
    it('should resolve a context name to its endpoint', async () => {
      mockExecFile.mockResolvedValue({
        stdout: 'unix:///var/run/docker.sock\n',
        stderr: '',
      });

      const result = await resolveDockerContext('default', logger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('unix:///var/run/docker.sock');
      }
    });

    it('should resolve a TCP endpoint', async () => {
      mockExecFile.mockResolvedValue({
        stdout: 'tcp://192.168.1.10:2375\n',
        stderr: '',
      });

      const result = await resolveDockerContext('remote-host', logger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('tcp://192.168.1.10:2375');
      }
    });

    it('should fail when context has no endpoint', async () => {
      mockExecFile.mockResolvedValue({ stdout: '\n', stderr: '' });

      const result = await resolveDockerContext('empty-context', logger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('has no endpoint');
      }
    });

    it('should fail when context does not exist', async () => {
      mockExecFile.mockRejectedValue(new Error('context "nonexistent" does not exist'));

      const result = await resolveDockerContext('nonexistent', logger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain("Failed to resolve Docker context 'nonexistent'");
        expect(result.guidance).toBeDefined();
        expect(result.guidance?.resolution).toContain('docker context ls');
      }
    });

    it('should fail when Docker CLI is not available', async () => {
      const error = new Error('ENOENT') as NodeJS.ErrnoException;
      error.code = 'ENOENT';
      mockExecFile.mockRejectedValue(error);

      const result = await resolveDockerContext('default', logger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to resolve Docker context');
      }
    });
  });
});
