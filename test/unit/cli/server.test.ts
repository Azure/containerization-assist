import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

describe('Server Entry Point', () => {
  let originalEnv: Record<string, string | undefined>;

  beforeEach(() => {
    originalEnv = { ...process.env };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('Server Module Structure', () => {
    it('should have server entry point file', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      expect(() => statSync(serverPath)).not.toThrow();
      
      const content = readFileSync(serverPath, 'utf-8');
      expect(content).toContain('async function main');
      expect(content).toContain('MCPServer');
      expect(content).toContain('createDependencies');
    });

    it('should contain MCP mode setting', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain("process.env.MCP_MODE = 'true'");
    });

    it('should contain server configuration', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('Containerization Assist MCP Server');
      expect(content).toContain('SDK-Native');
    });

    it('should contain dependency injection setup', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('createDependencies');
      expect(content).toContain('shutdownDependencies');
      expect(content).toContain('deps');
    });
  });

  describe('Signal Handlers', () => {
    it('should contain signal handler registration', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain("process.on('SIGINT'");
      expect(content).toContain("process.on('SIGTERM'");
      expect(content).toContain("process.on('SIGQUIT'");
    });

    it('should contain graceful shutdown logic', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('const shutdown = async');
      expect(content).toContain('Shutting down server');
      expect(content).toContain('server.stop()');
      expect(content).toContain('shutdownDependencies');
    });
  });

  describe('Error Handling', () => {
    it('should contain error handling for startup failures', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('catch (error)');
      expect(content).toContain('Failed to start server');
      expect(content).toContain('process.exit(1)');
    });

    it('should contain error handling for shutdown failures', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('Error during shutdown');
      expect(content).toContain('logger.error');
    });

    it('should contain logger fallback for when deps is unavailable', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('deps?.logger ?? console');
      expect(content).toContain('console.error');
    });
  });

  describe('Module Entry Point', () => {
    it('should contain module execution guard', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('import.meta.url');
      expect(content).toContain('process.argv[1]');
      expect(content).toContain('main().catch');
    });
  });

  describe('Process Lifecycle', () => {
    it('should contain stdin resume for keeping process alive', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('process.stdin.resume()');
    });

    it('should contain server startup sequence', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('new MCPServer');
      expect(content).toContain('await server.start()');
      expect(content).toContain('Starting SDK-Native MCP Server');
      expect(content).toContain('MCP Server started successfully');
    });

    it('should contain proper variable scoping', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');
      
      expect(content).toContain('let deps: Dependencies | undefined');
      expect(content).toContain('let server: MCPServer | undefined');
    });
  });
});