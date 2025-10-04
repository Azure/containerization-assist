/**
 * Unit tests for Docker socket validation module
 */

import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';

describe('Docker Socket Validation', () => {
  let originalPlatform: string;
  let originalEnv: NodeJS.ProcessEnv;

  beforeEach(() => {
    originalPlatform = process.platform;
    originalEnv = { ...process.env };
    jest.resetModules();
  });

  afterEach(() => {
    Object.defineProperty(process, 'platform', {
      value: originalPlatform,
      writable: true,
      configurable: true,
    });
    process.env = originalEnv;
  });

  describe('Module Structure', () => {
    it('should export validateDockerSocket function', async () => {
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');
      expect(typeof validateDockerSocket).toBe('function');
    });

    it('should export autoDetectDockerSocket function', async () => {
      const { autoDetectDockerSocket } = await import('@/infra/docker/socket-validation');
      expect(typeof autoDetectDockerSocket).toBe('function');
    });

    it('should export SocketValidationResult type', async () => {
      const module = await import('@/infra/docker/socket-validation');
      expect(module).toBeDefined();
    });
  });

  describe('autoDetectDockerSocket', () => {
    it('should return Windows named pipe on win32 platform', async () => {
      Object.defineProperty(process, 'platform', {
        value: 'win32',
        writable: true,
        configurable: true,
      });

      // Re-import to get the function with the new platform
      jest.resetModules();
      const { autoDetectDockerSocket } = await import('@/infra/docker/socket-validation');

      const socket = autoDetectDockerSocket();
      expect(socket).toBe('npipe://./pipe/docker_engine');
    });

    it('should return Unix socket path on non-Windows platforms', async () => {
      Object.defineProperty(process, 'platform', {
        value: 'linux',
        writable: true,
        configurable: true,
      });

      jest.resetModules();
      const { autoDetectDockerSocket } = await import('@/infra/docker/socket-validation');

      const socket = autoDetectDockerSocket();
      // Should return either detected or default socket path
      expect(typeof socket).toBe('string');
      expect(socket.length).toBeGreaterThan(0);
    });

    it('should fall back to default socket when none available', async () => {
      Object.defineProperty(process, 'platform', {
        value: 'linux',
        writable: true,
        configurable: true,
      });

      jest.resetModules();
      const { autoDetectDockerSocket } = await import('@/infra/docker/socket-validation');

      const socket = autoDetectDockerSocket();
      // Should always return a string (fallback to default)
      expect(typeof socket).toBe('string');
      expect(socket).toContain('docker.sock');
    });
  });

  describe('validateDockerSocket', () => {
    it('should validate Windows named pipe without file check', async () => {
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket(
        { dockerSocket: 'npipe://./pipe/docker_engine' },
        true,
      );

      expect(result.dockerSocket).toBe('npipe://./pipe/docker_engine');
      expect(result.warnings).toEqual([]);
    });

    it('should use CLI option over environment variable', async () => {
      process.env.DOCKER_SOCKET = '/env/docker.sock';

      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      // Test with a pipe path which doesn't require fs access
      const result = validateDockerSocket({ dockerSocket: 'npipe://test' }, true);

      expect(result.dockerSocket).toBe('npipe://test');
    });

    it('should use environment variable when no CLI option provided', async () => {
      process.env.DOCKER_SOCKET = 'npipe://env-pipe';

      jest.resetModules();
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket({}, true);

      expect(result.dockerSocket).toBe('npipe://env-pipe');
    });

    it('should return SocketValidationResult with correct structure', async () => {
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket({ dockerSocket: 'npipe://test' }, true);

      expect(result).toHaveProperty('dockerSocket');
      expect(result).toHaveProperty('warnings');
      expect(Array.isArray(result.warnings)).toBe(true);
    });

    it('should handle quiet mode flag', async () => {
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      validateDockerSocket({ dockerSocket: 'npipe://test' }, true);

      expect(consoleErrorSpy).not.toHaveBeenCalled();

      consoleErrorSpy.mockRestore();
    });

    it('should respect MCP_MODE environment variable', async () => {
      process.env.MCP_MODE = 'true';

      jest.resetModules();
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      validateDockerSocket({ dockerSocket: 'npipe://test' }, false);

      expect(consoleErrorSpy).not.toHaveBeenCalled();

      consoleErrorSpy.mockRestore();
    });

    it('should respect MCP_QUIET environment variable', async () => {
      process.env.MCP_QUIET = 'true';

      jest.resetModules();
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      validateDockerSocket({ dockerSocket: 'npipe://test' }, false);

      expect(consoleErrorSpy).not.toHaveBeenCalled();

      consoleErrorSpy.mockRestore();
    });
  });

  describe('Integration with Docker Client', () => {
    it('should be importable from docker client module', async () => {
      const clientModule = await import('@/infra/docker/client');
      expect(clientModule).toBeDefined();
      expect(typeof clientModule.createDockerClient).toBe('function');
    });

    it('should be importable from config modules', async () => {
      const configModule = await import('@/config/index');
      expect(configModule.config).toBeDefined();
      expect(configModule.config.docker).toBeDefined();
    });
  });
});
