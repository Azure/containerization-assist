import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { ToolContext } from '../../../src/mcp/context/types';
import type { DeploymentWorkflowParams } from '../../../src/workflows/types';

// Mock all the tool imports
jest.mock('@tools/prepare-cluster', () => ({
  prepareCluster: jest.fn(),
}));

jest.mock('@tools/generate-k8s-manifests', () => ({
  generateK8sManifests: jest.fn(),
}));

jest.mock('@tools/push-image', () => ({
  pushImage: jest.fn(),
}));

jest.mock('@tools/deploy', () => ({
  deployApplication: jest.fn(),
}));

jest.mock('@tools/verify-deployment', () => ({
  verifyDeployment: jest.fn(),
}));

jest.mock('../../../src/lib/session', () => ({
  createSessionManager: jest.fn(),
}));

describe('Deployment Workflow', () => {
  let mockToolContext: ToolContext;
  let mockSessionManager: any;

  beforeEach(() => {
    mockSessionManager = {
      get: jest.fn(),
      create: jest.fn(),
      update: jest.fn(),
    };

    mockToolContext = {
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
        debug: jest.fn(),
      },
      sessionManager: mockSessionManager,
      signal: undefined,
    } as any;

    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('runDeploymentWorkflow', () => {
    it('should exist and be a function', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      expect(typeof runDeploymentWorkflow).toBe('function');
    });

    it('should initialize workflow context correctly', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { prepareCluster } = await import('@tools/prepare-cluster');

      // Mock successful cluster preparation
      (prepareCluster as jest.Mock).mockResolvedValue({
        ok: true,
        value: {
          namespace: 'test-namespace',
          context: 'test-context',
        },
      });

      mockSessionManager.get.mockResolvedValue(null);
      mockSessionManager.create.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      // This will fail at manifest generation, but that's OK for testing initialization
      await runDeploymentWorkflow(params, mockToolContext);

      // Verify session operations
      expect(mockSessionManager.get).toHaveBeenCalledWith('test-session');
      expect(mockSessionManager.create).toHaveBeenCalledWith('test-session');
      expect(mockSessionManager.update).toHaveBeenCalledWith('test-session', {
        status: 'active',
        stage: 'prepare-cluster',
      });
    });

    it('should handle cluster preparation failures', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { prepareCluster } = await import('@tools/prepare-cluster');

      // Mock failed cluster preparation
      (prepareCluster as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Cluster preparation failed',
      });

      mockSessionManager.get.mockResolvedValue(null);
      mockSessionManager.create.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      const result = await runDeploymentWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Cluster preparation failed');
      expect(result.sessionId).toBe('test-session');
      expect(result.metadata).toBeDefined();
      expect(result.metadata.steps).toBeDefined();
    });

    it('should handle manifest generation failures', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { prepareCluster } = await import('@tools/prepare-cluster');
      const { generateK8sManifests } = await import('@tools/generate-k8s-manifests');

      // Mock successful cluster preparation
      (prepareCluster as jest.Mock).mockResolvedValue({
        ok: true,
        value: {
          namespace: 'test-namespace',
          context: 'test-context',
        },
      });

      // Mock failed manifest generation
      (generateK8sManifests as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Manifest generation failed',
      });

      mockSessionManager.get.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      const result = await runDeploymentWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Manifest generation failed');
    });

    it('should handle push image failures', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { prepareCluster } = await import('@tools/prepare-cluster');
      const { generateK8sManifests } = await import('@tools/generate-k8s-manifests');
      const { pushImage } = await import('@tools/push-image');

      // Mock successful cluster preparation and manifest generation
      (prepareCluster as jest.Mock).mockResolvedValue({
        ok: true,
        value: { namespace: 'test-namespace' },
      });

      (generateK8sManifests as jest.Mock).mockResolvedValue({
        ok: true,
        value: { manifests: ['deployment.yaml', 'service.yaml'] },
      });

      // Mock failed image push
      (pushImage as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Image push failed',
      });

      mockSessionManager.get.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      const result = await runDeploymentWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Image push failed');
    });

    it('should handle deployment failures', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { prepareCluster } = await import('@tools/prepare-cluster');
      const { generateK8sManifests } = await import('@tools/generate-k8s-manifests');
      const { pushImage } = await import('@tools/push-image');
      const { deployApplication } = await import('@tools/deploy');

      // Mock successful steps up to deployment
      (prepareCluster as jest.Mock).mockResolvedValue({
        ok: true,
        value: { namespace: 'test-namespace' },
      });

      (generateK8sManifests as jest.Mock).mockResolvedValue({
        ok: true,
        value: { manifests: ['deployment.yaml'] },
      });

      (pushImage as jest.Mock).mockResolvedValue({
        ok: true,
        value: { pushed: true },
      });

      // Mock failed deployment
      (deployApplication as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Deployment failed',
      });

      mockSessionManager.get.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      const result = await runDeploymentWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Deployment failed');
    });

    it('should treat verification failures as warnings, not hard failures', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { prepareCluster } = await import('@tools/prepare-cluster');
      const { generateK8sManifests } = await import('@tools/generate-k8s-manifests');
      const { pushImage } = await import('@tools/push-image');
      const { deployApplication } = await import('@tools/deploy');
      const { verifyDeployment } = await import('@tools/verify-deployment');

      // Mock successful steps up to verification
      (prepareCluster as jest.Mock).mockResolvedValue({
        ok: true,
        value: { namespace: 'test-namespace' },
      });

      (generateK8sManifests as jest.Mock).mockResolvedValue({
        ok: true,
        value: { manifests: ['deployment.yaml'] },
      });

      (pushImage as jest.Mock).mockResolvedValue({
        ok: true,
        value: { pushed: true },
      });

      (deployApplication as jest.Mock).mockResolvedValue({
        ok: true,
        value: { serviceName: 'test-service' },
      });

      // Mock failed verification
      (verifyDeployment as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Verification failed',
      });

      mockSessionManager.get.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      const result = await runDeploymentWorkflow(params, mockToolContext);

      // Workflow should still succeed despite verification failure
      expect(result.success).toBe(true);
      expect(mockToolContext.logger.warn).toHaveBeenCalledWith(
        'Deployment verification had issues'
      );
    });

    it('should contain all required workflow steps', () => {
      const workflowPath = join(__dirname, '../../../src/workflows/deployment.ts');
      const content = readFileSync(workflowPath, 'utf-8');
      
      // Verify all steps are defined
      expect(content).toContain('prepare-cluster');
      expect(content).toContain('generate-manifests');
      expect(content).toContain('push-image');
      expect(content).toContain('deploy-application');
      expect(content).toContain('verify-deployment');
    });

    it('should handle general exceptions', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { prepareCluster } = await import('@tools/prepare-cluster');

      // Mock exception
      (prepareCluster as jest.Mock).mockRejectedValue(new Error('Unexpected error'));

      mockSessionManager.get.mockResolvedValue(null);
      mockSessionManager.create.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      const result = await runDeploymentWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toBe('Unexpected error');
    });

    it('should use fallback session manager when not provided', async () => {
      const { runDeploymentWorkflow } = await import('../../../src/workflows/deployment');
      const { createSessionManager } = await import('../../../src/lib/session');
      const { prepareCluster } = await import('@tools/prepare-cluster');

      const fallbackSessionManager = {
        get: jest.fn(),
        create: jest.fn(),
        update: jest.fn(),
      };

      (createSessionManager as jest.Mock).mockReturnValue(fallbackSessionManager);
      
      // Mock successful cluster preparation
      (prepareCluster as jest.Mock).mockResolvedValue({
        ok: true,
        value: { namespace: 'test-namespace' },
      });

      fallbackSessionManager.get.mockResolvedValue(null);
      fallbackSessionManager.create.mockResolvedValue({ id: 'test-session' });
      fallbackSessionManager.update.mockResolvedValue(undefined);

      const contextWithoutSessionManager = {
        ...mockToolContext,
        sessionManager: undefined,
      };

      const params: DeploymentWorkflowParams = {
        sessionId: 'test-session',
        imageId: 'test-image:latest',
        clusterConfig: {
          namespace: 'test-namespace',
        },
        deploymentOptions: {
          name: 'test-app',
          registry: 'docker.io',
        },
      };

      // This will fail at manifest generation, but should use fallback session manager
      await runDeploymentWorkflow(params, contextWithoutSessionManager);

      expect(createSessionManager).toHaveBeenCalledWith(mockToolContext.logger);
      expect(fallbackSessionManager.get).toHaveBeenCalledWith('test-session');
    });
  });

  describe('deploymentWorkflow export', () => {
    it('should export workflow configuration', async () => {
      const { deploymentWorkflow } = await import('../../../src/workflows/deployment');
      
      expect(deploymentWorkflow).toBeDefined();
      expect(deploymentWorkflow.name).toBe('deployment-workflow');
      expect(deploymentWorkflow.description).toContain('Complete deployment pipeline');
      expect(typeof deploymentWorkflow.execute).toBe('function');
      expect(deploymentWorkflow.schema).toBeDefined();
      expect(deploymentWorkflow.schema.type).toBe('object');
      expect(deploymentWorkflow.schema.required).toContain('sessionId');
      expect(deploymentWorkflow.schema.required).toContain('imageId');
      expect(deploymentWorkflow.schema.required).toContain('deploymentOptions');
    });

    it('should have proper schema properties', async () => {
      const { deploymentWorkflow } = await import('../../../src/workflows/deployment');
      
      const schema = deploymentWorkflow.schema;
      expect(schema.properties).toBeDefined();
      expect(schema.properties.sessionId).toBeDefined();
      expect(schema.properties.imageId).toBeDefined();
      expect(schema.properties.clusterConfig).toBeDefined();
      expect(schema.properties.deploymentOptions).toBeDefined();

      // Check clusterConfig structure
      expect(schema.properties.clusterConfig.type).toBe('object');
      expect(schema.properties.clusterConfig.properties).toBeDefined();
      
      // Check deploymentOptions structure
      expect(schema.properties.deploymentOptions.type).toBe('object');
      expect(schema.properties.deploymentOptions.properties).toBeDefined();
      expect(schema.properties.deploymentOptions.required).toContain('name');
      expect(schema.properties.deploymentOptions.required).toContain('registry');
    });

    it('should have valid serviceType enum', async () => {
      const { deploymentWorkflow } = await import('../../../src/workflows/deployment');
      
      const serviceTypeProperty = 
        deploymentWorkflow.schema.properties.deploymentOptions.properties.serviceType;
      
      expect(serviceTypeProperty).toBeDefined();
      expect(serviceTypeProperty.enum).toContain('ClusterIP');
      expect(serviceTypeProperty.enum).toContain('NodePort');
      expect(serviceTypeProperty.enum).toContain('LoadBalancer');
    });

    it('should have valid imagePullPolicy enum', async () => {
      const { deploymentWorkflow } = await import('../../../src/workflows/deployment');
      
      const imagePullPolicyProperty = 
        deploymentWorkflow.schema.properties.deploymentOptions.properties.imagePullPolicy;
      
      expect(imagePullPolicyProperty).toBeDefined();
      expect(imagePullPolicyProperty.enum).toContain('Always');
      expect(imagePullPolicyProperty.enum).toContain('IfNotPresent');
      expect(imagePullPolicyProperty.enum).toContain('Never');
    });
  });
});