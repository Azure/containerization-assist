import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { validateOptions } from '@/cli/validation';
import type { CLIOptions, DockerSocketValidation } from '@/cli/validation';

describe('CLI Validation Module', () => {
  let testDir: string;
  let consoleErrorSpy: jest.SpiedFunction<typeof console.error>;

  beforeEach(() => {
    // Create a temporary test directory
    testDir = join(tmpdir(), `cli-validation-test-${Date.now()}`);
    mkdirSync(testDir, { recursive: true });

    // Spy on console.error
    consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    // Clean up temporary test directory
    try {
      rmSync(testDir, { recursive: true, force: true });
    } catch (error) {
      // Ignore cleanup errors
    }

    // Restore mocks
    jest.restoreAllMocks();
  });

  describe('validateOptions', () => {
    describe('log level validation', () => {
      it('should accept valid log levels', () => {
        const validLevels = ['debug', 'info', 'warn', 'error'];

        validLevels.forEach((level) => {
          const opts: CLIOptions = { logLevel: level };
          const result = validateOptions(opts);
          expect(result.valid).toBe(true);
          expect(result.errors).toHaveLength(0);
        });
      });

      it('should reject invalid log level', () => {
        const opts: CLIOptions = { logLevel: 'invalid' };
        const result = validateOptions(opts);

        expect(result.valid).toBe(false);
        expect(result.errors).toHaveLength(1);
        expect(result.errors[0]).toContain('Invalid log level: invalid');
        expect(result.errors[0]).toContain('debug, info, warn, error');
      });

      it('should accept undefined log level', () => {
        const opts: CLIOptions = {};
        const result = validateOptions(opts);

        expect(result.valid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });
    });

    describe('workspace validation', () => {
      it('should accept valid workspace directory', () => {
        const opts: CLIOptions = { workspace: testDir };
        const result = validateOptions(opts);

        expect(result.valid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });

      it('should reject non-existent workspace directory', () => {
        const opts: CLIOptions = { workspace: '/nonexistent/path' };
        const result = validateOptions(opts);

        expect(result.valid).toBe(false);
        expect(result.errors).toHaveLength(1);
        expect(result.errors[0]).toContain('Workspace directory does not exist');
        expect(result.errors[0]).toContain('/nonexistent/path');
      });

      it('should reject workspace path that is not a directory', () => {
        const filePath = join(testDir, 'not-a-dir.txt');
        writeFileSync(filePath, 'test');

        const opts: CLIOptions = { workspace: filePath };
        const result = validateOptions(opts);

        expect(result.valid).toBe(false);
        expect(result.errors).toHaveLength(1);
        expect(result.errors[0]).toContain('Workspace path is not a directory');
        expect(result.errors[0]).toContain(filePath);
      });

      it('should accept undefined workspace', () => {
        const opts: CLIOptions = {};
        const result = validateOptions(opts);

        expect(result.valid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });

      it('should handle permission errors gracefully', () => {
        // This test is tricky to implement without platform-specific behavior
        // We'll test that the error message contains the expected format
        const opts: CLIOptions = { workspace: '/root/no-permission' };
        const result = validateOptions(opts);

        expect(result.valid).toBe(false);
        expect(result.errors.length).toBeGreaterThan(0);
        // Should contain either ENOENT (doesn't exist) or EACCES (permission denied)
        expect(result.errors[0]).toMatch(/Workspace directory does not exist|Permission denied accessing workspace|Cannot access workspace directory/);
      });
    });

    describe('config file validation', () => {
      it('should accept valid config file', () => {
        const configPath = join(testDir, 'config.json');
        writeFileSync(configPath, '{}');

        const opts: CLIOptions = { config: configPath };
        const result = validateOptions(opts);

        expect(result.valid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });

      it('should reject non-existent config file', () => {
        const opts: CLIOptions = { config: '/nonexistent/config.json' };
        const result = validateOptions(opts);

        expect(result.valid).toBe(false);
        expect(result.errors).toHaveLength(1);
        expect(result.errors[0]).toContain('Configuration file not found');
        expect(result.errors[0]).toContain('/nonexistent/config.json');
      });

      it('should accept undefined config', () => {
        const opts: CLIOptions = {};
        const result = validateOptions(opts);

        expect(result.valid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });
    });

    describe('Docker socket validation integration', () => {
      it('should integrate Docker socket validation warnings', () => {
        const dockerValidation: DockerSocketValidation = {
          dockerSocket: '/var/run/docker.sock',
          warnings: ['Docker socket not accessible', 'No valid Docker socket'],
        };

        const opts: CLIOptions = {};
        const result = validateOptions(opts, dockerValidation);

        expect(result.valid).toBe(false);
        expect(result.errors).toContain('No valid Docker socket');
      });

      it('should update opts with validated Docker socket', () => {
        const dockerValidation: DockerSocketValidation = {
          dockerSocket: '/custom/docker.sock',
          warnings: [],
        };

        const opts: CLIOptions = {};
        validateOptions(opts, dockerValidation);

        expect(opts.dockerSocket).toBe('/custom/docker.sock');
      });

      it('should log non-fatal Docker warnings to console', () => {
        const dockerValidation: DockerSocketValidation = {
          dockerSocket: '/var/run/docker.sock',
          warnings: ['Docker socket permissions might be restricted'],
        };

        const opts: CLIOptions = {};
        const result = validateOptions(opts, dockerValidation);

        expect(result.valid).toBe(true);
        expect(consoleErrorSpy).toHaveBeenCalledWith(
          '⚠️  Docker socket permissions might be restricted'
        );
      });

      it('should not log warnings in MCP_MODE', () => {
        const originalMcpMode = process.env.MCP_MODE;
        process.env.MCP_MODE = 'true';

        const dockerValidation: DockerSocketValidation = {
          dockerSocket: '/var/run/docker.sock',
          warnings: ['Docker socket permissions might be restricted'],
        };

        const opts: CLIOptions = {};
        validateOptions(opts, dockerValidation);

        expect(consoleErrorSpy).not.toHaveBeenCalled();

        // Restore environment
        if (originalMcpMode === undefined) {
          delete process.env.MCP_MODE;
        } else {
          process.env.MCP_MODE = originalMcpMode;
        }
      });
    });

    describe('combined validation', () => {
      it('should report all validation errors', () => {
        const opts: CLIOptions = {
          logLevel: 'invalid',
          workspace: '/nonexistent/path',
          config: '/nonexistent/config.json',
        };

        const dockerValidation: DockerSocketValidation = {
          dockerSocket: '',
          warnings: ['No valid Docker socket'],
        };

        const result = validateOptions(opts, dockerValidation);

        expect(result.valid).toBe(false);
        expect(result.errors.length).toBeGreaterThanOrEqual(4);
        expect(result.errors).toEqual(
          expect.arrayContaining([
            expect.stringContaining('Invalid log level'),
            expect.stringContaining('Workspace directory does not exist'),
            expect.stringContaining('Configuration file not found'),
            expect.stringContaining('No valid Docker socket'),
          ])
        );
      });

      it('should pass with all valid options', () => {
        const configPath = join(testDir, 'config.json');
        writeFileSync(configPath, '{}');

        const opts: CLIOptions = {
          logLevel: 'info',
          workspace: testDir,
          config: configPath,
        };

        const dockerValidation: DockerSocketValidation = {
          dockerSocket: '/var/run/docker.sock',
          warnings: [],
        };

        const result = validateOptions(opts, dockerValidation);

        expect(result.valid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });

      it('should pass with minimal valid options', () => {
        const opts: CLIOptions = {};
        const result = validateOptions(opts);

        expect(result.valid).toBe(true);
        expect(result.errors).toHaveLength(0);
      });
    });
  });
});
