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
      expect(content).toContain('createApp');
      expect(content).toContain('app.startServer');
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
      expect(content).toContain('stdio');
    });

    it('should contain app setup', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('createApp');
      expect(content).toContain('app.startServer');
      expect(content).toContain('app.stop');
    });
  });

  describe('Signal Handlers', () => {
    it('should contain signal handler registration', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain("process.on('SIGINT'");
      expect(content).toContain("process.on('SIGTERM'");
    });

    it('should contain graceful shutdown logic', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('const shutdown = async');
      expect(content).toContain('Shutting down server');
      expect(content).toContain('app.stop()');
      expect(content).toContain('Server stopped successfully');
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

    it('should contain logger creation and error handling', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('const logger = createLogger');
      expect(content).toContain('console.error');
    });
  });

  describe('Module Entry Point', () => {
    it('should contain module execution guard', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      // Our new implementation just runs main() directly
      expect(content).toContain('void main()');
    });
  });

  describe('Process Lifecycle', () => {
    it('should contain stdio transport configuration', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      // Our new implementation uses stdio transport instead
      expect(content).toContain("transport: 'stdio'");
    });

    it('should contain server startup sequence', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('createApp');
      expect(content).toContain('await app.startServer');
      expect(content).toContain('Starting Containerization Assist MCP Server');
      expect(content).toContain('MCP Server started successfully');
    });

    it('should contain proper variable scoping', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('let app: ReturnType<typeof createApp>');
      expect(content).toContain('app = createApp');
    });
  });
});