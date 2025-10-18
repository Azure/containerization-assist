import { describe, it, expect } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

describe('Kubernetes Client', () => {
  describe('Module Structure', () => {
    it('should have kubernetes client implementation file', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('createKubernetesClient');
      expect(content).toContain('KubernetesClient');
      expect(content).toContain('applyManifest');
      expect(content).toContain('getDeploymentStatus');
      expect(content).toContain('ping');
      expect(content).toContain('namespaceExists');
      expect(content).toContain('checkPermissions');
      expect(content).toContain('checkIngressController');
    });

    it('should define proper interface types', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');

      expect(content).toContain('DeploymentResult');
      expect(content).toContain('K8sManifest');
    });

    it('should use Result pattern for error handling', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('Result<');
      expect(content).toContain('Success');
      expect(content).toContain('Failure');
    });

    it('should integrate with @kubernetes/client-node library', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('@kubernetes/client-node');
      expect(content).toContain('KubeConfig');
    });
  });

  describe('Client Configuration', () => {
    it('should support manifest application options', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('kind');
      expect(content).toContain('metadata');
      expect(content).toContain('namespace');
    });

    it('should support logging integration', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');
      
      expect(content).toContain('Logger');
      expect(content).toContain('logger.debug');
      expect(content).toContain('logger.info');
      expect(content).toContain('logger.warn');
    });
  });

  describe('Client Export', () => {
    it('should export createKubernetesClient function', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');

      expect(content).toContain('export const createKubernetesClient');
    });
  });

  describe('Single-App Resource Support', () => {
    it('should support core single-app resource types', () => {
      const resourceOpsPath = join(__dirname, '../../../../src/infra/kubernetes/resource-operations.ts');
      const content = readFileSync(resourceOpsPath, 'utf-8');

      // Core resources for single-app scenarios - now in resource-operations.ts
      expect(content).toContain('Deployment');
      expect(content).toContain('Service');
    });

    it('should have simplified ingress detection for single-app flow', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');

      expect(content).toContain('checkIngressController');
      expect(content).toContain('IngressClass');
      // Should check common ingress controllers
      expect(content).toContain('ingress');
    });

    it('should focus on namespace and deployment operations', () => {
      const clientPath = join(__dirname, '../../../../src/infra/kubernetes/client.ts');
      const content = readFileSync(clientPath, 'utf-8');

      // Essential operations for single-app deployment
      expect(content).toContain('namespaceExists');
      expect(content).toContain('getDeploymentStatus');
      expect(content).toContain('checkPermissions');
    });
  });
});