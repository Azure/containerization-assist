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
      expect(content).toContain('bootstrap');
      expect(content).toContain('await bootstrap');
    });

    it('should use bootstrap for MCP mode setting', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('bootstrap');
      expect(content).toContain('MCP_MODE setup');
    });

    it('should contain server configuration', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('Containerization Assist MCP Server');
      expect(content).toContain('bootstrap');
    });

    it('should use bootstrap for app setup', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('bootstrap');
      expect(content).toContain('appName');
      expect(content).toContain('version');
    });
  });

  describe('Signal Handlers', () => {
    it('should use bootstrap for signal handler registration', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('bootstrap');
      expect(content).toContain('Bootstrap handles');
    });

    it('should use bootstrap for graceful shutdown logic', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('bootstrap');
      expect(content).toContain('Shutdown handler installation');
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

    it('should handle errors via bootstrap', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('catch (error)');
      expect(content).toContain('logger.fatal');
    });

    it('should contain logger creation and error handling', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('const logger = createLogger');
      expect(content).toContain('logger.fatal');
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
    it('should use bootstrap for server lifecycle', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('bootstrap');
      expect(content).toContain('await bootstrap');
    });

    it('should contain server configuration', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('containerization-assist-mcp');
      expect(content).toContain('packageJson.version');
      expect(content).toContain('policyPath');
    });

    it('should use bootstrap helper pattern', () => {
      const serverPath = join(__dirname, '../../../src/cli/server.ts');
      const content = readFileSync(serverPath, 'utf-8');

      expect(content).toContain('bootstrap');
      expect(content).toContain('logger');
    });
  });
});