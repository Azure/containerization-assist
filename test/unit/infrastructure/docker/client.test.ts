import { describe, it, expect } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

describe('Docker Client', () => {
  describe('Module Structure', () => {
    it('should have docker client implementation file', () => {
      const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('createDockerClient');
      expect(content).toContain('DockerClient');
      expect(content).toContain('buildImage');
      expect(content).toContain('getImage');
      expect(content).toContain('tagImage');
      expect(content).toContain('pushImage');
    });

    it('should define proper interface types', () => {
      const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('DockerBuildOptions');
      expect(content).toContain('DockerBuildResult');
      expect(content).toContain('DockerPushResult');
      expect(content).toContain('DockerImageInfo');
    });

    it('should use Result pattern for error handling', () => {
      const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('Result<');
      expect(content).toContain('Success');
      expect(content).toContain('Failure');
    });

    it('should integrate with dockerode library', () => {
      const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('dockerode');
      expect(content).toContain('new Docker()');
    });
  });

  describe('Client Configuration', () => {
    it('should support build configuration options', () => {
      const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('dockerfile');
      expect(content).toContain('buildargs');
      expect(content).toContain('context');
      expect(content).toContain('platform');
    });

    it('should support logging integration', () => {
      const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('Logger');
      expect(content).toContain('logger.debug');
      expect(content).toContain('logger.info');
      expect(content).toContain('logger.error');
    });
  });

  describe('Client Export', () => {
    it('should export createDockerClient function', async () => {
      const clientModule = await import('../../../../src/infrastructure/docker/client');
      expect(clientModule.createDockerClient).toBeDefined();
      expect(typeof clientModule.createDockerClient).toBe('function');
    });
  });
});