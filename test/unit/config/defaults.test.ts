import { describe, it, expect } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

describe('Configuration Defaults', () => {
  describe('Module Structure', () => {
    it('should have constants configuration file with defaults', () => {
      const constantsPath = join(__dirname, '../../../src/config/constants.ts');
      const content = readFileSync(constantsPath, 'utf-8');

      expect(content).toContain('export');
      expect(content).toContain('DEFAULT');
    });

    it('should contain network defaults', () => {
      const constantsPath = join(__dirname, '../../../src/config/constants.ts');
      const content = readFileSync(constantsPath, 'utf-8');

      expect(content).toContain('DEFAULT_NETWORK');
      expect(content).toContain('host');
    });

    it('should contain timeout defaults', () => {
      const constantsPath = join(__dirname, '../../../src/config/constants.ts');
      const content = readFileSync(constantsPath, 'utf-8');

      expect(content).toContain('DEFAULT_TIMEOUTS');
      expect(content).toContain('timeout');
    });

    it('should contain port configuration', () => {
      const constantsPath = join(__dirname, '../../../src/config/constants.ts');
      const content = readFileSync(constantsPath, 'utf-8');

      expect(content).toContain('getDefaultPort');
      expect(content).toContain('Port');
    });
  });

  describe('Defaults Export', () => {
    it('should export defaults configuration', async () => {
      const constantsModule = await import('../../../src/config/constants');
      expect(typeof constantsModule).toBe('object');
    });

    it('should export DEFAULT_NETWORK', async () => {
      const { DEFAULT_NETWORK } = await import('../../../src/config/constants');
      expect(DEFAULT_NETWORK).toBeDefined();
      expect(typeof DEFAULT_NETWORK).toBe('object');
    });

    it('should export DEFAULT_TIMEOUTS', async () => {
      const { DEFAULT_TIMEOUTS } = await import('../../../src/config/constants');
      expect(DEFAULT_TIMEOUTS).toBeDefined();
      expect(typeof DEFAULT_TIMEOUTS).toBe('object');
    });

    it('should export getDefaultPort', async () => {
      const { getDefaultPort } = await import('../../../src/config/constants');
      expect(getDefaultPort).toBeDefined();
      expect(typeof getDefaultPort).toBe('function');
    });
  });

  describe('Port Configuration', () => {
    it('should handle port calculation for different languages', async () => {
      const { getDefaultPort } = await import('../../../src/config/constants');

      // Test common language types
      const jsPort = getDefaultPort('javascript');
      const pyPort = getDefaultPort('python');
      const javaPort = getDefaultPort('java');

      expect(typeof jsPort).toBe('number');
      expect(jsPort).toBeGreaterThan(0);
      expect(jsPort).toBeLessThan(65536);

      expect(typeof pyPort).toBe('number');
      expect(pyPort).toBeGreaterThan(0);
      expect(pyPort).toBeLessThan(65536);

      expect(typeof javaPort).toBe('number');
      expect(javaPort).toBeGreaterThan(0);
      expect(javaPort).toBeLessThan(65536);
    });
  });
});

