import { describe, it, expect } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

describe('Docker Client', () => {
  describe('Module Structure', () => {
    it('should have docker client implementation file', () => {
      const clientPath = join(__dirname, '../../../../src/services/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('createDockerClient');
      expect(content).toContain('DockerClient');
      expect(content).toContain('buildImage');
      expect(content).toContain('getImage');
      expect(content).toContain('tagImage');
      expect(content).toContain('pushImage');
    });

    it('should define proper interface types', () => {
      const clientPath = join(__dirname, '../../../../src/services/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('DockerBuildOptions');
      expect(content).toContain('DockerBuildResult');
      expect(content).toContain('DockerPushResult');
      expect(content).toContain('DockerImageInfo');
    });

    it('should use Result pattern for error handling', () => {
      const clientPath = join(__dirname, '../../../../src/services/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('Result<');
      expect(content).toContain('Success');
      expect(content).toContain('Failure');
    });

    it('should integrate with dockerode library', () => {
      const clientPath = join(__dirname, '../../../../src/services/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('dockerode');
      expect(content).toContain('new Docker()');
    });
  });

  describe('Client Configuration', () => {
    it('should support build configuration options', () => {
      const clientPath = join(__dirname, '../../../../src/services/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('dockerfile');
      expect(content).toContain('buildargs');
      expect(content).toContain('context');
      expect(content).toContain('platform');
    });

    it('should support logging integration', () => {
      const clientPath = join(__dirname, '../../../../src/services/docker/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('Logger');
      expect(content).toContain('logger.debug');
      expect(content).toContain('logger.info');
      expect(content).toContain('logger.error');
    });
  });

  describe('Client Export', () => {
    it('should export createDockerClient function', async () => {
      const clientModule = await import('../../../../src/services/docker/client');
      expect(clientModule.createDockerClient).toBeDefined();
      expect(typeof clientModule.createDockerClient).toBe('function');
    });
  });

  describe('Enhanced Error Handling Implementation', () => {
    describe('Type Safety', () => {
      it('should import error handling functions from errors module', () => {
        const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
        const content = readFileSync(clientPath, 'utf-8');
        
        expect(content).toContain("import { extractDockerErrorMessage, sanitizeErrorDetails } from './errors'");
      });

      it('should have proper TypeScript interfaces in errors module', () => {
        const errorsPath = join(__dirname, '../../../../src/infrastructure/docker/errors.ts');
        const content = readFileSync(errorsPath, 'utf-8');
        
        expect(content).toContain('export interface DockerError extends Error');
        expect(content).toContain('statusCode?: number');
        expect(content).toContain('json?: Record<string, unknown>');
        expect(content).toContain('reason?: string');
        expect(content).toContain('code?: string');
      });

      it('should have type guard for Docker errors in errors module', () => {
        const errorsPath = join(__dirname, '../../../../src/infrastructure/docker/errors.ts');
        const content = readFileSync(errorsPath, 'utf-8');
        
        expect(content).toContain('export function isDockerError(error: unknown): error is DockerError');
        expect(content).toContain('return error instanceof Error');
      });

      it('should have error message extraction function in errors module', () => {
        const errorsPath = join(__dirname, '../../../../src/infrastructure/docker/errors.ts');
        const content = readFileSync(errorsPath, 'utf-8');
        
        expect(content).toContain('export function extractDockerErrorMessage');
        expect(content).toContain('{ message: string; details: Record<string, unknown> }');
      });
    });

    describe('Progress Error Handling', () => {
      it('should contain enhanced progress error handling for buildImage', () => {
        const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
        const content = readFileSync(clientPath, 'utf-8');
        
        // Verify enhanced followProgress callback is implemented
        expect(content).toContain('Docker build followProgress error');
        expect(content).toContain('errorDetails: sanitizeErrorDetails(details)');
        expect(content).toContain('Docker build error event received');
      });

      it('should contain enhanced progress error handling for pushImage', () => {
        const clientPath = join(__dirname, '../../../../src/infrastructure/docker/client.ts');
        const content = readFileSync(clientPath, 'utf-8');
        
        // Verify enhanced followProgress callback is implemented
        expect(content).toContain('Docker push followProgress error');
        expect(content).toContain('Docker push error event received');
      });
    });
  });
});
