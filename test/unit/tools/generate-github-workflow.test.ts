/**
 * Unit Tests: Generate GitHub Workflow Tool
 * Tests the generate-github-workflow tool schema and handler
 */

import { jest } from '@jest/globals';
import type { ToolContext } from '@/mcp/context';

// Mock knowledge loader
const mockGetKnowledgeForCategory = jest.fn();
jest.mock('@/knowledge', () => ({
  getKnowledgeForCategory: mockGetKnowledgeForCategory,
}));

// Mock logger
jest.mock('@/lib/logger', () => ({
  createLogger: jest.fn(() => ({
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  })),
}));

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

function createMockToolContext(): ToolContext {
  return {
    logger: createMockLogger(),
  } as any;
}

// Import after mocks
import generateGithubWorkflowTool from '@/tools/generate-github-workflow/tool';
import { generateGithubWorkflowSchema } from '@/tools/generate-github-workflow/schema';

// ─── Knowledge mock helpers ───────────────────────────────────────────────────

const authSnippet = {
  id: 'github-oidc-permissions',
  text: 'Set id-token: write and contents: read at workflow level',
  category: 'cicd',
  tags: ['azure-oidc', 'permissions'],
  weight: 1.0,
};

const buildSnippet = {
  id: 'docker-build-push-acr',
  text: 'Use docker/build-push-action@v5 with cache-from/cache-to: type=gha',
  category: 'cicd',
  tags: ['docker-build', 'acr', 'registry'],
  weight: 0.9,
};

const deploySnippet = {
  id: 'k8s-deploy-action',
  text: 'Use azure/k8s-deploy@v5 to apply manifests to AKS',
  category: 'cicd',
  tags: ['aks', 'k8s-deploy'],
  weight: 0.9,
};

const concurrencySnippet = {
  id: 'workflow-concurrency',
  text: 'Add concurrency group with cancel-in-progress: true',
  category: 'cicd',
  tags: ['concurrency', 'best-practice'],
  weight: 0.7,
};

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('generate-github-workflow', () => {
  let mockContext: ToolContext;

  beforeEach(() => {
    mockContext = createMockToolContext();
    jest.clearAllMocks();

    mockGetKnowledgeForCategory.mockReturnValue([
      authSnippet,
      buildSnippet,
      deploySnippet,
      concurrencySnippet,
    ]);
  });

  // ── Schema validation ──────────────────────────────────────────────────────

  describe('Schema', () => {
    it('should parse a minimal valid input', () => {
      const result = generateGithubWorkflowSchema.safeParse({
        repositoryPath: '/home/user/myapp',
        registry: 'myregistry.azurecr.io',
        clusterName: 'my-aks',
        resourceGroup: 'my-rg',
      });

      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.namespace).toBe('default');
        expect(result.data.environment).toBe('production');
        expect(result.data.manifestFormat).toBe('k8s');
        expect(result.data.branches).toEqual(['main']);
      }
    });

    it('should parse a full input with all optional fields', () => {
      const result = generateGithubWorkflowSchema.safeParse({
        repositoryPath: '/home/user/myapp',
        registry: 'myregistry.azurecr.io',
        clusterName: 'my-aks',
        resourceGroup: 'my-rg',
        imageName: 'myapp',
        namespace: 'production',
        environment: 'staging',
        manifestFormat: 'helm',
        manifestPath: 'charts/',
        branches: ['main', 'release'],
        language: 'java',
        framework: 'spring-boot',
      });

      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.manifestFormat).toBe('helm');
        expect(result.data.namespace).toBe('production');
        expect(result.data.branches).toEqual(['main', 'release']);
      }
    });

    it('should reject input missing required registry field', () => {
      const result = generateGithubWorkflowSchema.safeParse({
        repositoryPath: '/home/user/myapp',
        clusterName: 'my-aks',
        resourceGroup: 'my-rg',
      });

      expect(result.success).toBe(false);
    });

    it('should reject input missing required clusterName field', () => {
      const result = generateGithubWorkflowSchema.safeParse({
        repositoryPath: '/home/user/myapp',
        registry: 'myregistry.azurecr.io',
        resourceGroup: 'my-rg',
      });

      expect(result.success).toBe(false);
    });

    it('should reject input missing required resourceGroup field', () => {
      const result = generateGithubWorkflowSchema.safeParse({
        repositoryPath: '/home/user/myapp',
        registry: 'myregistry.azurecr.io',
        clusterName: 'my-aks',
      });

      expect(result.success).toBe(false);
    });

    it('should reject an invalid manifestFormat value', () => {
      const result = generateGithubWorkflowSchema.safeParse({
        repositoryPath: '/home/user/myapp',
        registry: 'myregistry.azurecr.io',
        clusterName: 'my-aks',
        resourceGroup: 'my-rg',
        manifestFormat: 'terraform',
      });

      expect(result.success).toBe(false);
    });

    it('should reject an invalid environment value', () => {
      const result = generateGithubWorkflowSchema.safeParse({
        repositoryPath: '/home/user/myapp',
        registry: 'myregistry.azurecr.io',
        clusterName: 'my-aks',
        resourceGroup: 'my-rg',
        environment: 'prod',
      });

      expect(result.success).toBe(false);
    });
  });

  // ── Happy path ─────────────────────────────────────────────────────────────

  describe('Happy Path', () => {
    it('should return a successful plan for minimal k8s manifest format', async () => {
      const result = await generateGithubWorkflowTool.handler(
        {
          repositoryPath: '/home/user/myapp',
          registry: 'myregistry.azurecr.io',
          clusterName: 'my-aks',
          resourceGroup: 'my-rg',
          namespace: 'default',
          environment: 'production',
          manifestFormat: 'k8s',
          branches: ['main'],
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;

        // nextAction
        expect(plan.nextAction).toBeDefined();
        expect(plan.nextAction.action).toBe('create-files');
        expect(plan.nextAction.files).toHaveLength(1);
        expect(plan.nextAction.files[0].path).toBe('.github/workflows/deploy.yml');

        // instruction contains key identifiers
        expect(plan.nextAction.instruction).toContain('myregistry.azurecr.io');
        expect(plan.nextAction.instruction).toContain('my-aks');
        expect(plan.nextAction.instruction).toContain('my-rg');
        expect(plan.nextAction.instruction).toContain('AZURE_CLIENT_ID');

        // jobs
        expect(plan.workflowJobs).toHaveLength(2);
        expect(plan.workflowJobs[0].name).toBe('build-and-push');
        expect(plan.workflowJobs[1].name).toBe('deploy');
        expect(plan.workflowJobs[1].environment).toBe('production');

        // secrets
        expect(plan.secretsRequired).toContain('AZURE_CLIENT_ID');
        expect(plan.secretsRequired).toContain('AZURE_TENANT_ID');
        expect(plan.secretsRequired).toContain('AZURE_SUBSCRIPTION_ID');

        // attribution
        expect(
          plan.attributionLabels.annotations['com.azure.containerizationassist/workflow-generator'],
        ).toBeDefined();

        // summary
        expect(plan.summary).toContain('myregistry.azurecr.io');
      }
    });

    it('should include bake step details in instruction when manifestFormat is helm', async () => {
      const result = await generateGithubWorkflowTool.handler(
        {
          repositoryPath: '/home/user/myapp',
          registry: 'myregistry.azurecr.io',
          clusterName: 'my-aks',
          resourceGroup: 'my-rg',
          namespace: 'default',
          environment: 'staging',
          manifestFormat: 'helm',
          manifestPath: 'charts/',
          branches: ['main'],
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;
        expect(plan.nextAction.instruction).toContain('k8s-bake');
        expect(plan.nextAction.instruction).toContain('helm');
        // deploy job should include bake step in steps list
        const deployJob = plan.workflowJobs.find((j) => j.name === 'deploy');
        expect(deployJob?.steps.some((s) => s.includes('k8s-bake'))).toBe(true);
      }
    });

    it('should include bake step details when manifestFormat is kustomize', async () => {
      const result = await generateGithubWorkflowTool.handler(
        {
          repositoryPath: '/home/user/myapp',
          registry: 'myregistry.azurecr.io',
          clusterName: 'my-aks',
          resourceGroup: 'my-rg',
          manifestFormat: 'kustomize',
          branches: ['main'],
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;
        expect(plan.nextAction.instruction).toContain('kustomize');
      }
    });

    it('should derive image name from repository path when imageName is not provided', async () => {
      const result = await generateGithubWorkflowTool.handler(
        {
          repositoryPath: '/home/user/spring-petclinic',
          registry: 'myregistry.azurecr.io',
          clusterName: 'my-aks',
          resourceGroup: 'my-rg',
          branches: ['main'],
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;
        expect(plan.summary).toContain('spring-petclinic');
      }
    });

    it('should use provided imageName when given', async () => {
      const result = await generateGithubWorkflowTool.handler(
        {
          repositoryPath: '/home/user/myrepo',
          registry: 'myregistry.azurecr.io',
          clusterName: 'my-aks',
          resourceGroup: 'my-rg',
          imageName: 'custom-image',
          branches: ['main'],
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const plan = result.value;
        expect(plan.summary).toContain('custom-image');
        expect(plan.nextAction.instruction).toContain('custom-image');
      }
    });

    it('should list multiple trigger branches in instruction', async () => {
      const result = await generateGithubWorkflowTool.handler(
        {
          repositoryPath: '/home/user/myapp',
          registry: 'myregistry.azurecr.io',
          clusterName: 'my-aks',
          resourceGroup: 'my-rg',
          branches: ['main', 'release/v2'],
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.nextAction.instruction).toContain('main');
        expect(result.value.nextAction.instruction).toContain('release/v2');
      }
    });
  });

  // ── Tool metadata ──────────────────────────────────────────────────────────

  describe('Tool metadata', () => {
    it('should have the correct tool name', () => {
      expect(generateGithubWorkflowTool.name).toBe('generate-github-workflow');
    });

    it('should have knowledgeEnhanced metadata', () => {
      expect(generateGithubWorkflowTool.metadata.knowledgeEnhanced).toBe(true);
    });

    it('should have chainHints', () => {
      expect(generateGithubWorkflowTool.chainHints).toBeDefined();
      expect(generateGithubWorkflowTool.chainHints?.success).toBeDefined();
      expect(generateGithubWorkflowTool.chainHints?.failure).toBeDefined();
    });

    it('should expose an inputSchema', () => {
      expect(generateGithubWorkflowTool.inputSchema).toBeDefined();
      expect(typeof generateGithubWorkflowTool.inputSchema).toBe('object');
    });
  });
});
