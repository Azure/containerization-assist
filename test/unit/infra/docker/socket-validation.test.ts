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

    it('should export parseDockerHost function', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      expect(typeof parseDockerHost).toBe('function');
    });

    it('should export SocketValidationResult type', async () => {
      const module = await import('@/infra/docker/socket-validation');
      expect(module).toBeDefined();
    });
  });

  describe('parseDockerHost', () => {
    it('should parse unix:// scheme', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('unix:///var/run/docker.sock');
      expect(result).toEqual({ type: 'unix', value: '/var/run/docker.sock' });
    });

    it('should parse unix:// with home path', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('unix:///home/user/.rd/docker.sock');
      expect(result).toEqual({ type: 'unix', value: '/home/user/.rd/docker.sock' });
    });

    it('should parse tcp:// scheme', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('tcp://192.168.1.100:2375');
      expect(result).toEqual({
        type: 'tcp',
        value: 'tcp://192.168.1.100:2375',
        host: '192.168.1.100',
        port: 2375,
      });
    });

    it('should parse tcp:// without port (defaults to 2375)', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('tcp://myhost');
      expect(result).toEqual({ type: 'tcp', value: 'tcp://myhost', host: 'myhost', port: 2375 });
    });

    it('should parse http:// scheme as tcp', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('http://localhost:2375');
      expect(result).toEqual({
        type: 'tcp',
        value: 'http://localhost:2375',
        host: 'localhost',
        port: 2375,
      });
    });

    it('should parse https:// scheme as tcp', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('https://docker.example.com:2376');
      expect(result).toEqual({
        type: 'tcp',
        value: 'https://docker.example.com:2376',
        host: 'docker.example.com',
        port: 2376,
      });
    });

    it('should parse https:// without port (defaults to 2376)', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('https://docker.example.com');
      expect(result).toEqual({
        type: 'tcp',
        value: 'https://docker.example.com',
        host: 'docker.example.com',
        port: 2376,
      });
    });

    it('should parse npipe:// scheme', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('npipe:////./pipe/docker_engine');
      expect(result).toEqual({ type: 'npipe', value: '//./pipe/docker_engine' });
    });

    it('should parse raw absolute path as unix', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('/var/run/docker.sock');
      expect(result).toEqual({ type: 'unix', value: '/var/run/docker.sock' });
    });

    it('should parse raw tilde path as unix', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('~/.rd/docker.sock');
      expect(result).toEqual({ type: 'unix', value: '~/.rd/docker.sock' });
    });

    it('should parse raw Windows pipe path', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('//./pipe/docker_engine');
      expect(result).toEqual({ type: 'npipe', value: '//./pipe/docker_engine' });
    });

    it('should throw on empty string', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      expect(() => parseDockerHost('')).toThrow('DOCKER_HOST is empty');
    });

    it('should throw on whitespace-only string', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      expect(() => parseDockerHost('   ')).toThrow('DOCKER_HOST is empty');
    });

    it('should throw on unsupported fd:// scheme', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      expect(() => parseDockerHost('fd://something')).toThrow('scheme not supported');
    });

    it('should throw on unsupported ssh:// scheme', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      expect(() => parseDockerHost('ssh://user@host')).toThrow('scheme not supported');
    });

    it('should throw on unix:// with no path', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      expect(() => parseDockerHost('unix://')).toThrow('has no path');
    });

    it('should throw on unrecognized value', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      expect(() => parseDockerHost('just-a-hostname')).toThrow('not recognized');
    });

    it('should trim whitespace from input', async () => {
      const { parseDockerHost } = await import('@/infra/docker/socket-validation');
      const result = parseDockerHost('  unix:///var/run/docker.sock  ');
      expect(result).toEqual({ type: 'unix', value: '/var/run/docker.sock' });
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
      expect(socket).toBe('//./pipe/docker_engine');
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

      const result = validateDockerSocket({ dockerSocket: '//./pipe/docker_engine' }, true);

      expect(result.dockerSocket).toBe('//./pipe/docker_engine');
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

    it('should not match http://pipeline:2375 as a Windows pipe', async () => {
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      // http://pipeline:2375 contains 'pipe' but is a TCP address, not a Windows pipe
      const result = validateDockerSocket({ dockerSocket: 'http://pipeline:2375' }, true);

      // Should be handled as TCP, not as a pipe
      expect(result.dockerSocket).toBe('http://pipeline:2375');
      expect(result.warnings).toEqual([]);
    });

    it('should use DOCKER_HOST when DOCKER_SOCKET is not set', async () => {
      delete process.env.DOCKER_SOCKET;
      process.env.DOCKER_HOST = 'tcp://192.168.1.100:2375';

      jest.resetModules();
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket({}, true);

      expect(result.dockerSocket).toBe('tcp://192.168.1.100:2375');
      expect(result.warnings).toEqual([]);
    });

    it('should prefer DOCKER_SOCKET over DOCKER_HOST', async () => {
      process.env.DOCKER_SOCKET = 'npipe://from-socket-env';
      process.env.DOCKER_HOST = 'tcp://from-docker-host:2375';

      jest.resetModules();
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket({}, true);

      expect(result.dockerSocket).toBe('npipe://from-socket-env');
    });

    it('should prefer CLI option over DOCKER_HOST', async () => {
      process.env.DOCKER_HOST = 'tcp://from-docker-host:2375';

      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket({ dockerSocket: 'npipe://from-cli' }, true);

      expect(result.dockerSocket).toBe('npipe://from-cli');
    });

    it('should handle invalid DOCKER_HOST with warning and fallback', async () => {
      delete process.env.DOCKER_SOCKET;
      process.env.DOCKER_HOST = 'fd://unsupported';

      jest.resetModules();
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket({}, true);

      // Should fall back to auto-detect and include a warning about invalid DOCKER_HOST
      expect(result.warnings.some((w) => w.includes('Invalid DOCKER_HOST'))).toBe(true);
    });

    it('should validate tcp:// DOCKER_HOST without file system check', async () => {
      delete process.env.DOCKER_SOCKET;
      process.env.DOCKER_HOST = 'tcp://remote-docker:2376';

      jest.resetModules();
      const { validateDockerSocket } = await import('@/infra/docker/socket-validation');

      const result = validateDockerSocket({}, true);

      expect(result.dockerSocket).toBe('tcp://remote-docker:2376');
      expect(result.warnings).toEqual([]);
    });
  });

  describe('dockerHostToOptions', () => {
    it('should convert tcp:// to host/port options', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      const result = dockerHostToOptions('tcp://192.168.1.100:2375');
      expect(result).toEqual({ host: '192.168.1.100', port: 2375 });
    });

    it('should convert https:// to host/port/protocol options', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      const result = dockerHostToOptions('https://docker.example.com:2376');
      expect(result).toEqual({ host: 'docker.example.com', port: 2376, protocol: 'https' });
    });

    it('should convert unix:// to socketPath', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      const result = dockerHostToOptions('unix:///var/run/docker.sock');
      expect(result).toEqual({ socketPath: '/var/run/docker.sock' });
    });

    it('should convert raw path to socketPath', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      const result = dockerHostToOptions('/var/run/docker.sock');
      expect(result).toEqual({ socketPath: '/var/run/docker.sock' });
    });

    it('should convert npipe:// to socketPath', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      const result = dockerHostToOptions('npipe:////./pipe/docker_engine');
      expect(result).toEqual({ socketPath: '//./pipe/docker_engine' });
    });

    it('should convert http:// to host/port/protocol options', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      const result = dockerHostToOptions('http://localhost:2375');
      expect(result).toEqual({ host: 'localhost', port: 2375, protocol: 'http' });
    });

    it('should throw on ssh:// endpoints', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      expect(() => dockerHostToOptions('ssh://user@remote-host')).toThrow(
        'Unsupported Docker host scheme',
      );
    });

    it('should throw on fd:// endpoints', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      expect(() => dockerHostToOptions('fd://something')).toThrow('Unsupported Docker host scheme');
    });

    it('should throw on invalid tcp:// URL', async () => {
      const { dockerHostToOptions } = await import('@/infra/docker/socket-validation');
      expect(() => dockerHostToOptions('tcp://')).toThrow('invalid URL');
    });
  });

  describe('toDockerHostURI', () => {
    it('should prepend unix:// to raw Unix paths', async () => {
      const { toDockerHostURI } = await import('@/infra/docker/socket-validation');
      expect(toDockerHostURI('/var/run/docker.sock')).toBe('unix:///var/run/docker.sock');
    });

    it('should prepend unix:// to home-relative paths', async () => {
      const { toDockerHostURI } = await import('@/infra/docker/socket-validation');
      expect(toDockerHostURI('/home/user/.docker/desktop/docker.sock')).toBe(
        'unix:///home/user/.docker/desktop/docker.sock',
      );
    });

    it('should prepend npipe:// to raw Windows pipe paths', async () => {
      const { toDockerHostURI } = await import('@/infra/docker/socket-validation');
      expect(toDockerHostURI('//./pipe/docker_engine')).toBe('npipe:////./pipe/docker_engine');
    });

    it('should normalize backslash Windows pipe paths to forward-slash npipe:// URIs', async () => {
      const { toDockerHostURI } = await import('@/infra/docker/socket-validation');
      expect(toDockerHostURI('\\\\.\\pipe\\docker_engine')).toBe('npipe:////./pipe/docker_engine');
    });

    it('should pass through tcp:// URIs unchanged', async () => {
      const { toDockerHostURI } = await import('@/infra/docker/socket-validation');
      expect(toDockerHostURI('tcp://192.168.1.100:2375')).toBe('tcp://192.168.1.100:2375');
    });

    it('should pass through unix:// URIs unchanged', async () => {
      const { toDockerHostURI } = await import('@/infra/docker/socket-validation');
      expect(toDockerHostURI('unix:///var/run/docker.sock')).toBe('unix:///var/run/docker.sock');
    });

    it('should pass through https:// URIs unchanged', async () => {
      const { toDockerHostURI } = await import('@/infra/docker/socket-validation');
      expect(toDockerHostURI('https://docker.example.com:2376')).toBe(
        'https://docker.example.com:2376',
      );
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
