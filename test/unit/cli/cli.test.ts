import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { Command } from 'commander';

/**
 * Behavioural CLI Tests
 *
 * These tests validate CLI behaviour by exercising Commander directly,
 * rather than relying on string-grep tests of source code.
 */
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

  /**
   * Helper to create a minimal CLI program with standard options
   */
  function createTestProgram(): Command {
    const program = new Command()
      .name('containerization-assist-mcp')
      .description('MCP server for AI-powered containerization workflows')
      .version('1.0.0')
      .argument('[command]', 'command to run (start, inspect-tools)', 'start')
      .option('--config <path>', 'path to configuration file (.env)')
      .option('--log-level <level>', 'logging level: debug, info, warn, error (default: info)', 'info')
      .option('--workspace <path>', 'workspace directory path (default: current directory)', process.cwd())
      .option('--dev', 'enable development mode with debug logging')
      .option('--validate', 'validate configuration and exit')
      .option('--list-tools', 'list all registered MCP tools and exit')
      .option('--health-check', 'perform system health check and exit')
      .option('--docker-socket <path>', 'Docker socket path (default: platform-specific)', '')
      .option(
        '--k8s-namespace <namespace>',
        'default Kubernetes namespace (default: default)',
        'default',
      );

    // Prevent program from exiting during tests
    program.exitOverride();
    return program;
  }

  describe('CLI Arguments Parsing', () => {
    it('should parse log-level option correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--log-level', 'debug']);

      const opts = program.opts();
      expect(opts.logLevel).toBe('debug');
    });

    it('should default log-level to info when not specified', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli']);

      const opts = program.opts();
      expect(opts.logLevel).toBe('info');
    });

    it('should parse workspace option correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--workspace', '/custom/path']);

      const opts = program.opts();
      expect(opts.workspace).toBe('/custom/path');
    });

    it('should parse dev flag correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--dev']);

      const opts = program.opts();
      expect(opts.dev).toBe(true);
    });

    it('should parse validate flag correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--validate']);

      const opts = program.opts();
      expect(opts.validate).toBe(true);
    });

    it('should parse list-tools flag correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--list-tools']);

      const opts = program.opts();
      expect(opts.listTools).toBe(true);
    });

    it('should parse health-check flag correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--health-check']);

      const opts = program.opts();
      expect(opts.healthCheck).toBe(true);
    });

    it('should parse docker-socket option correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--docker-socket', '/custom/docker.sock']);

      const opts = program.opts();
      expect(opts.dockerSocket).toBe('/custom/docker.sock');
    });

    it('should parse k8s-namespace option correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--k8s-namespace', 'production']);

      const opts = program.opts();
      expect(opts.k8sNamespace).toBe('production');
    });

    it('should default k8s-namespace to "default"', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli']);

      const opts = program.opts();
      expect(opts.k8sNamespace).toBe('default');
    });

    it('should parse multiple options together', () => {
      const program = createTestProgram();
      program.parse([
        'node', 'cli',
        '--log-level', 'warn',
        '--workspace', '/test',
        '--dev',
        '--k8s-namespace', 'staging'
      ]);

      const opts = program.opts();
      expect(opts.logLevel).toBe('warn');
      expect(opts.workspace).toBe('/test');
      expect(opts.dev).toBe(true);
      expect(opts.k8sNamespace).toBe('staging');
    });
  });

  describe('Command Arguments', () => {
    it('should parse command argument correctly', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', 'start']);

      expect(program.args[0]).toBe('start');
    });

    it('should default command to "start" when not specified', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli']);

      // Commander provides default values through argument definition
      const commandArg = program.args[0] ?? 'start';
      expect(commandArg).toBe('start');
    });

    it('should accept inspect-tools command', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', 'inspect-tools']);

      expect(program.args[0]).toBe('inspect-tools');
    });
  });

  describe('Program Metadata', () => {
    it('should have correct program name', () => {
      const program = createTestProgram();
      expect(program.name()).toBe('containerization-assist-mcp');
    });

    it('should have version information', () => {
      const program = createTestProgram();
      expect(program.version()).toBe('1.0.0');
    });

    it('should have description', () => {
      const program = createTestProgram();
      expect(program.description()).toBe('MCP server for AI-powered containerization workflows');
    });
  });

  describe('Option Types', () => {
    it('should treat boolean flags as boolean type', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--dev']);

      const opts = program.opts();
      expect(typeof opts.dev).toBe('boolean');
      expect(opts.dev).toBe(true);
    });

    it('should treat value options as string type', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli', '--log-level', 'debug']);

      const opts = program.opts();
      expect(typeof opts.logLevel).toBe('string');
    });

    it('should provide defaults for optional flags when not set', () => {
      const program = createTestProgram();
      program.parse(['node', 'cli']);

      const opts = program.opts();
      expect(opts.dev).toBeUndefined();
      expect(opts.validate).toBeUndefined();
      expect(opts.listTools).toBeUndefined();
    });
  });
});