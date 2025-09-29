import { MCPClient, setupMCPTestEnvironment, cleanupMCPTestEnvironment } from '../../helpers/mcp-environment';
import { TestRepository } from '../../fixtures/types';
import { Result, Success, Failure } from "../../src/types"
import { Logger } from 'pino';
import path from 'path';
import fs from 'fs/promises';

export interface E2ETestContext {
  mcpClient: MCPClient;
  testRepositories: TestRepository[];
  logger: Logger;
  tempDir: string;
  cleanup: () => Promise<void>;
}

export interface E2ETestConfig {
  timeout?: number;
  useRealInfrastructure?: boolean;
  enablePersistence?: boolean;
  repositoryTypes?: string[];
}

export class E2ETestBase {
  private context: E2ETestContext | null = null;
  private config: E2ETestConfig;

  constructor(config: E2ETestConfig = {}) {
    this.config = {
      timeout: 300000, // 5 minutes default
      useRealInfrastructure: process.env.E2E_REAL_INFRA === 'true',
      enablePersistence: false,
      repositoryTypes: ['node-express-basic', 'python-flask', 'java-springboot'],
      ...config
    };
  }

  async setup(): Promise<Result<E2ETestContext>> {
    try {
      const tempDir = path.join(process.cwd(), 'temp', `e2e-test-${Date.now()}`);
      await fs.mkdir(tempDir, { recursive: true });

      const mcpClient = await setupMCPTestEnvironment();

      // Create mock repositories for testing
      const testRepositories: TestRepository[] = [
        {
          name: 'node-express-basic',
          type: 'web-api',
          path: path.join(tempDir, 'node-express-basic'),
          language: 'javascript',
          framework: 'express',
          complexity: 'simple',
          description: 'Basic Node.js Express application'
        },
        {
          name: 'python-flask',
          type: 'web-api', 
          path: path.join(tempDir, 'python-flask'),
          language: 'python',
          framework: 'flask',
          complexity: 'simple',
          description: 'Basic Python Flask application'
        },
        {
          name: 'java-springboot',
          type: 'web-api',
          path: path.join(tempDir, 'java-springboot'),
          language: 'java',
          framework: 'spring-boot',
          complexity: 'moderate',
          description: 'Java Spring Boot application'
        }
      ];

      // Create mock logger
      const logger = {
        info: (msg: string) => console.log(`[INFO] ${msg}`),
        warn: (msg: string) => console.log(`[WARN] ${msg}`),
        error: (msg: string) => console.log(`[ERROR] ${msg}`),
        debug: (msg: string) => console.log(`[DEBUG] ${msg}`)
      } as Logger;

      this.context = {
        mcpClient,
        testRepositories,
        logger,
        tempDir,
        cleanup: async () => {
          if (!this.config.enablePersistence) {
            await fs.rm(tempDir, { recursive: true, force: true });
          }
        }
      };

      return Success(this.context);
    } catch (error) {
      return Failure(`Failed to setup E2E test environment: ${error.message}`);
    }
  }

  async teardown(): Promise<Result<void>> {
    if (!this.context) {
      return Success(undefined);
    }

    try {
      await this.context.cleanup();
      this.context = null;
      return Success(undefined);
    } catch (error) {
      return Failure(`Failed to teardown E2E test environment: ${error.message}`);
    }
  }

  getContext(): E2ETestContext | null {
    return this.context;
  }

  async runCompleteWorkflow(repositoryPath: string): Promise<Result<CompleteWorkflowResult>> {
    if (!this.context) {
      return Failure('E2E test context not initialized');
    }

    try {
      const { mcpClient, logger } = this.context;

      // Step 1: Analyze repository
      logger.info('Starting repository analysis...');
      const analyzeResult = await mcpClient.callTool('analyze-repo', { 
        path: repositoryPath 
      });
      
      if (!analyzeResult.ok) {
        return Failure(`Repository analysis failed: ${analyzeResult.error}`);
      }

      // Step 2: Generate Dockerfile
      logger.info('Generating Dockerfile...');
      const dockerfileResult = await mcpClient.callTool('generate-dockerfile', {
        repositoryPath,
        analysis: analyzeResult.value
      });

      if (!dockerfileResult.ok) {
        return Failure(`Dockerfile generation failed: ${dockerfileResult.error}`);
      }

      // Step 3: Build image (if real infrastructure enabled)
      let buildResult = null;
      if (this.config.useRealInfrastructure) {
        logger.info('Building Docker image...');
        buildResult = await mcpClient.callTool('build-image', {
          dockerfilePath: path.join(repositoryPath, 'Dockerfile'),
          imageName: `test-app-${Date.now()}`,
          context: repositoryPath
        });

        if (!buildResult.ok) {
          return Failure(`Image build failed: ${buildResult.error}`);
        }
      }

      // Step 4: Generate K8s manifests
      logger.info('Generating Kubernetes manifests...');
      const k8sResult = await mcpClient.callTool('generate-k8s-manifests', {
        repositoryPath,
        analysis: analyzeResult.value,
        imageName: buildResult ? buildResult.value.imageName : 'placeholder-image'
      });

      if (!k8sResult.ok) {
        return Failure(`K8s manifest generation failed: ${k8sResult.error}`);
      }

      return Success({
        analysis: analyzeResult.value,
        dockerfile: dockerfileResult.value,
        buildOutput: buildResult?.value || null,
        k8sManifests: k8sResult.value,
        duration: Date.now(),
        repositoryPath
      });

    } catch (error) {
      return Failure(`Complete workflow failed: ${error.message}`);
    }
  }
}

export interface CompleteWorkflowResult {
  analysis: any;
  dockerfile: any;
  buildOutput: any | null;
  k8sManifests: any;
  duration: number;
  repositoryPath: string;
}

export interface WorkflowValidation {
  dockerfileExists: boolean;
  k8sManifestsGenerated: boolean;
  imageBuilt: boolean;
  allFilesValid: boolean;
  errors: string[];
}

export async function validateWorkflowOutput(
  result: CompleteWorkflowResult, 
  context: E2ETestContext
): Promise<Result<WorkflowValidation>> {
  try {
    const validation: WorkflowValidation = {
      dockerfileExists: false,
      k8sManifestsGenerated: false,
      imageBuilt: false,
      allFilesValid: true,
      errors: []
    };

    // Check if Dockerfile was created
    try {
      await fs.access(path.join(result.repositoryPath, 'Dockerfile'));
      validation.dockerfileExists = true;
    } catch {
      validation.errors.push('Dockerfile not found');
      validation.allFilesValid = false;
    }

    // Check if K8s manifests were created
    try {
      const k8sDir = path.join(result.repositoryPath, 'k8s');
      const files = await fs.readdir(k8sDir);
      validation.k8sManifestsGenerated = files.length > 0;
      if (!validation.k8sManifestsGenerated) {
        validation.errors.push('No K8s manifests generated');
        validation.allFilesValid = false;
      }
    } catch {
      validation.errors.push('K8s directory not found');
      validation.allFilesValid = false;
    }

    // Check if image was built (only if real infrastructure)
    if (result.buildOutput) {
      validation.imageBuilt = result.buildOutput.ok === true;
      if (!validation.imageBuilt) {
        validation.errors.push('Image build failed');
        validation.allFilesValid = false;
      }
    }

    return Success(validation);
  } catch (error) {
    return Failure(`Workflow validation failed: ${error.message}`);
  }
}