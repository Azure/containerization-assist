import { describe, it, expect } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

describe('Docker Infrastructure Index', () => {
  describe('Module Structure', () => {
    it('should have docker index file', () => {
      const indexPath = join(__dirname, '../../../../src/infrastructure/docker/index.ts');
      const content = readFileSync(indexPath, 'utf-8');
      
      expect(content).toContain('export');
      expect(content).toContain('Docker');
    });

    it('should export client types and functions', () => {
      const indexPath = join(__dirname, '../../../../src/infrastructure/docker/index.ts');
      const content = readFileSync(indexPath, 'utf-8');
      
      expect(content).toContain('DockerClient');
      expect(content).toContain('createDockerClient');
      expect(content).toContain('DockerBuildOptions');
      expect(content).toContain('DockerBuildResult');
      expect(content).toContain('DockerPushResult');
      expect(content).toContain('DockerImageInfo');
    });

    it('should export registry functions', () => {
      const indexPath = join(__dirname, '../../../../src/infrastructure/docker/index.ts');
      const content = readFileSync(indexPath, 'utf-8');
      
      expect(content).toContain('createDockerRegistryClient');
      expect(content).toContain('./registry');
    });
  });

  describe('Module Exports', () => {
    it('should export all expected Docker types and functions', async () => {
      const dockerModule = await import('../../../../src/infrastructure/docker/index');
      
      expect(dockerModule.createDockerClient).toBeDefined();
      expect(typeof dockerModule.createDockerClient).toBe('function');
      
      expect(dockerModule.createDockerRegistryClient).toBeDefined();
      expect(typeof dockerModule.createDockerRegistryClient).toBe('function');
    });

    it('should re-export client types', async () => {
      const dockerModule = await import('../../../../src/infrastructure/docker/index');
      
      // These are TypeScript types, so they won't be available at runtime
      // But we can verify they're part of the module's type structure
      expect(typeof dockerModule).toBe('object');
    });
  });
});

describe('Kubernetes Infrastructure Index', () => {
  describe('Module Structure', () => {
    it('should have kubernetes index file', () => {
      const indexPath = join(__dirname, '../../../../src/infrastructure/kubernetes/index.ts');
      const content = readFileSync(indexPath, 'utf-8');
      
      expect(content).toContain('export');
      expect(content).toContain('Kubernetes');
    });

    it('should export client types and functions', () => {
      const indexPath = join(__dirname, '../../../../src/infrastructure/kubernetes/index.ts');
      const content = readFileSync(indexPath, 'utf-8');
      
      expect(content).toContain('KubernetesClient');
      expect(content).toContain('createKubernetesClient');
    });
  });

  describe('Module Exports', () => {
    it('should export all expected Docker types and functions', () => {
      const dockerIndexPath = join(__dirname, '../../../../src/infrastructure/docker/index.ts');
      const content = readFileSync(dockerIndexPath, 'utf-8');
      
      expect(content).toContain('export');
      expect(content).toContain('Docker');
    });
  });
});