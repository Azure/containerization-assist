import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { spawn } from 'node:child_process';
import { join } from 'node:path';
import { readFileSync, statSync } from 'node:fs';

describe('CLI Interface', () => {
  let processExitSpy: jest.SpiedFunction<typeof process.exit>;
  let consoleErrorSpy: jest.SpiedFunction<typeof console.error>;

  beforeEach(() => {
    processExitSpy = jest.spyOn(process, 'exit').mockImplementation(() => {
      throw new Error('process.exit called');
    });
    consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('CLI Arguments Parsing', () => {
    it('should have executable CLI file', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      expect(() => statSync(cliPath)).not.toThrow();
      
      const content = readFileSync(cliPath, 'utf-8');
      expect(content).toContain('#!/usr/bin/env node');
      expect(content).toContain('.name(');
      expect(content).toContain('.version(');
      expect(content).toContain('.option(');
    });

    it('should define all required CLI options', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      // Check for required options
      expect(content).toContain('--log-level');
      expect(content).toContain('--workspace');
      expect(content).toContain('--dev');
      expect(content).toContain('--validate');
      expect(content).toContain('--list-tools');
      expect(content).toContain('--health-check');
      expect(content).toContain('--docker-socket');
      expect(content).toContain('--k8s-namespace');
      expect(content).toContain('--show-merged');
    });

    it('should NOT include --config option (removed as breaking change)', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      // Verify --config was removed
      expect(content).not.toContain('--config');
    });
  });

  describe('Option Validation', () => {
    it('should import validation module', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      expect(content).toContain('validateOptions');
      expect(content).toContain("from './validation'");
    });

    it('should call validateOptions with Docker validation', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      expect(content).toContain('validateDockerSocket');
      expect(content).toContain('validateOptions(options, dockerValidation)');
    });
  });

  describe('Transport Detection', () => {
    it('should use stdio transport only', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      expect(content).toContain('transportConfig');
      expect(content).toContain('stdio');
      expect(content).toContain("transport: 'stdio'");
    });
  });

  describe('Docker Socket Validation', () => {
    it('should import Docker socket validation from infra module', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      expect(content).toContain('validateDockerSocket');
      expect(content).toContain('@/infra/docker/socket-validation');
    });

    it('should use extracted validateDockerSocket function', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      // Verify it calls the imported function
      expect(content).toContain('validateDockerSocket');
      // Verify it's not defining the function locally
      expect(content).not.toContain('function validateDockerSocket');
    });
  });

  describe('Command Handling', () => {
    it('should contain command validation logic', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');
      
      expect(content).toContain('Unknown command');
      expect(content).toContain('Available commands: start');
    });

    it('should default to start command', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');
      
      expect(content).toContain("'start'");
      expect(content).toContain('command to run');
    });

    it('should contain main execution logic', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');
      
      expect(content).toContain('async function main');
      expect(content).toContain('void main()');
    });
  });

  describe('Environment Variable Setting', () => {
    it('should contain environment variable setting logic', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');
      
      expect(content).toContain('env.LOG_LEVEL');
      expect(content).toContain('env.WORKSPACE_DIR');
      expect(content).toContain('process.env.DOCKER_SOCKET');
      expect(content).toContain('process.env.K8S_NAMESPACE');
      expect(content).toContain('process.env.NODE_ENV');
    });

    it('should contain development mode setting', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');
      
      expect(content).toContain("'development'");
      expect(content).toContain('options.dev');
    });
  });

  describe('Package.json Loading', () => {
    it('should contain package.json loading logic', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');
      
      expect(content).toContain('package.json');
      expect(content).toContain('JSON.parse');
      expect(content).toContain('readFileSync');
      expect(content).toContain('packageJson.version');
    });

    it('should have proper path resolution for package.json', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');
      
      expect(content).toContain('packageJsonPath');
      expect(content).toContain('__dirname');
      expect(content).toContain('dist');
    });
  });

  describe('Error Handling', () => {
    it('should import contextual guidance module', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      expect(content).toContain('provideContextualGuidance');
      expect(content).toContain("from './guidance'");
    });

    it('should use guidance module in error handling', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      // Verify guidance is called with error and options
      expect(content).toContain('provideContextualGuidance(error, options)');
    });

    it('should install shutdown handlers via runtime-logging', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      // Verify that shutdown handlers are installed via runtime-logging module
      expect(content).toContain('installShutdownHandlers');
      expect(content).toContain('@/lib/runtime-logging');
    });

    it('should contain signal handlers for graceful shutdown', () => {
      const cliPath = join(__dirname, '../../../src/cli/cli.ts');
      const content = readFileSync(cliPath, 'utf-8');

      expect(content).toContain('installShutdownHandlers');
      expect(content).toContain('shutdown');
    });
  });
});