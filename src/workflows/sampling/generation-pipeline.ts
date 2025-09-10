/**
 * Generation Pipeline - Orchestrates Dockerfile sampling workflow
 */

import type { Logger } from 'pino';
import { Success, Failure, isFail, type Result } from '@types';
import { getDefaultPort } from '@config/defaults';
import type {
  SamplingConfig,
  SamplingResult,
  DockerfileContext,
  ScoringCriteria,
  DockerfileVariant,
  ScoredVariant,
} from './types';

// Type definitions for analysis data
interface PackageJsonData {
  scripts?: Record<string, string>;
  devDependencies?: Record<string, string>;
}

interface AnalysisFiles {
  'package.json'?: PackageJsonData;
  'package-lock.json'?: unknown;
  'yarn.lock'?: unknown;
  'pnpm-lock.yaml'?: unknown;
}

interface AnalysisData {
  language?: string;
  framework?: string;
  dependencies?: (string | { name: string })[];
  files?: AnalysisFiles;
}
import { executeMultipleSamplingStrategies } from './strategy-engine';
import { PromptRegistry } from '../../core/prompts/registry';
import { VariantScorer } from './scorer';
import { analyzeRepo } from '@tools/analyze-repo';
import type { ToolContext } from '../../mcp/context/types';
import { validateSamplingConfig, validateScoringCriteria } from './validation';
import {
  createMCPAIOrchestrator,
  type MCPAIOrchestrator,
} from '@workflows/intelligent-orchestration';

/**
 * Main generation pipeline for Dockerfile sampling with AI validation
 */
export class VariantGenerationPipeline {
  private scorer: VariantScorer;
  private aiOrchestrator: MCPAIOrchestrator;

  constructor(
    private logger: Logger,
    promptRegistry?: PromptRegistry,
    aiOrchestrator?: MCPAIOrchestrator,
  ) {
    this.scorer = new VariantScorer(logger);
    this.aiOrchestrator =
      aiOrchestrator ||
      createMCPAIOrchestrator(logger, promptRegistry ? { promptRegistry } : undefined);
  }

  /**
   * Execute complete sampling pipeline with AI validation
   */
  async generateSampledDockerfiles(config: SamplingConfig): Promise<Result<SamplingResult>> {
    const startTime = Date.now();

    try {
      // Step 0: AI-powered parameter validation
      const aiValidationResult = await this.aiOrchestrator.validateParameters(
        'dockerfile-sampling',
        config as unknown as Record<string, unknown>,
        {
          toolName: 'dockerfile-sampling',
          environment: config.environment || 'development',
          targetType: 'dockerfile',
        },
      );

      if (aiValidationResult.ok && !aiValidationResult.value.isValid) {
        const errors = aiValidationResult.value.errors;
        this.logger.warn(
          { errors, sessionId: config.sessionId },
          'AI validation failed for sampling configuration',
        );
        return Failure(`Configuration validation failed: ${errors.join(', ')}`);
      }

      // Enhanced configuration validation
      const configValidation = validateSamplingConfig(config);
      if (!configValidation.ok) {
        return Failure(`Invalid configuration: ${configValidation.error}`);
      }

      this.logger.info(
        {
          sessionId: config.sessionId,
          repoPath: config.repoPath,
          variantCount: config.variantCount || 5,
        },
        'Starting Dockerfile sampling pipeline',
      );

      // Step 1: Analyze repository
      const contextResult = await this.buildSamplingContext(config);
      if (!contextResult.ok) {
        return Failure(`Context building failed: ${contextResult.error}`);
      }

      const context = contextResult.value;
      this.logger.debug(
        {
          language: context.analysis.language,
          framework: context.analysis.framework,
          environment: context.constraints.targetEnvironment,
        },
        'Sampling context prepared',
      );

      // Step 2: Generate variants
      const generationStart = Date.now();
      const variantsResult = await executeMultipleSamplingStrategies(
        (config.strategies || ['balanced', 'security-first']) as (
          | 'balanced'
          | 'security-first'
          | 'performance-optimized'
          | 'size-optimized'
        )[],
        context,
        this.logger,
      );
      if (!variantsResult.ok) {
        return Failure(`Variant generation failed: ${variantsResult.error}`);
      }

      let variants = variantsResult.value;

      // Limit to requested count
      const requestedCount = config.variantCount || 5;
      if (variants.length > requestedCount) {
        variants = variants.slice(0, requestedCount);
      }

      this.logger.info(
        {
          generated: variants.length,
          strategies: [...new Set(variants.map((v) => v.strategy))],
        },
        'Dockerfile variants generated',
      );

      // Step 3: Score variants
      const scoringStart = Date.now();
      const criteria = await this.prepareScoringCriteria(config.criteria, context);

      const scoredVariantsResult = await this.scorer.scoreVariants(variants, criteria);
      if (isFail(scoredVariantsResult)) {
        return Failure(`Variant scoring failed: ${scoredVariantsResult.error}`);
      }

      const scoredVariants = scoredVariantsResult.value;
      const scoringEnd = Date.now();

      // Step 4: Select best variant
      const bestVariant = this.scorer.selectBestVariant(scoredVariants, config.constraints);
      if (!bestVariant) {
        return Failure('No suitable variant found matching selection criteria');
      }

      const endTime = Date.now();

      // Build final result
      const result: SamplingResult = {
        sessionId: config.sessionId,
        variants: scoredVariants,
        bestVariant,
        criteria,
        metadata: {
          totalVariants: scoredVariants.length,
          strategiesUsed: [...new Set(variants.map((v) => v.strategy))],
          samplingDuration: scoringStart - generationStart,
          scoringDuration: scoringEnd - scoringStart,
          context,
        },
        generated: new Date(),
      };

      this.logger.info(
        {
          sessionId: config.sessionId,
          totalVariants: result.variants.length,
          bestVariant: result.bestVariant?.id ?? 'none',
          bestScore: result.bestVariant?.score?.total ?? 0,
          totalDuration: endTime - startTime,
        },
        'Dockerfile sampling completed successfully',
      );

      return Success(result);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error(
        {
          error: message,
          sessionId: config.sessionId,
          duration: Date.now() - startTime,
        },
        'Sampling pipeline failed',
      );

      return Failure(`Sampling pipeline error: ${message}`);
    }
  }

  /**
   * Build sampling context from repository analysis
   */
  private async buildSamplingContext(config: SamplingConfig): Promise<Result<DockerfileContext>> {
    try {
      // Analyze repository
      const toolContext: ToolContext = {
        logger: this.logger,
        sampling: {
          createMessage: async () => ({
            role: 'assistant' as const,
            content: [{ type: 'text', text: '' }],
          }),
        },
        getPrompt: async () => ({
          messages: [],
          name: '',
          description: '',
        }),
        progress: undefined,
      };
      const analysisResult = await analyzeRepo(
        {
          sessionId: config.sessionId,
          repoPath: config.repoPath,
          depth: 2,
          includeTests: false,
        },
        toolContext,
      );

      if (!analysisResult.ok) {
        return Failure(`Repository analysis failed: ${analysisResult.error}`);
      }

      const analysis = analysisResult.value;

      // Extract relevant information for sampling
      const samplingContext: DockerfileContext = {
        sessionId: config.sessionId,
        repoPath: config.repoPath,
        analysis: {
          language: analysis.language || 'javascript',
          ...(analysis.framework && { framework: analysis.framework }),
          packageManager: this.detectPackageManager(analysis),
          dependencies: Array.isArray(analysis.dependencies)
            ? analysis.dependencies.map((dep) =>
                typeof dep === 'string' ? dep : (dep as { name: string }).name || String(dep),
              )
            : [],
          buildTools: this.extractBuildTools(analysis),
          ...(analysis.framework && { testFramework: analysis.framework }),
          hasDatabase: this.detectDatabaseUsage(analysis),
          ports: this.extractPorts(analysis),
          environmentVars: this.extractEnvironmentVars(analysis),
        },
        constraints: {
          targetEnvironment: this.determineEnvironment(config),
          securityLevel: this.determineSecurityLevel(config),
          ...(config.timeout && { buildTimeLimit: config.timeout }),
        },
      };

      this.logger.debug({ context: samplingContext }, 'Sampling context built');
      return Success(samplingContext);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      return Failure(`Context building error: ${message}`);
    }
  }

  /**
   * Prepare scoring criteria based on config and context
   */
  private async prepareScoringCriteria(
    customCriteria?: Partial<ScoringCriteria>,
    context?: DockerfileContext,
  ): Promise<ScoringCriteria> {
    if (customCriteria) {
      const validation = validateScoringCriteria(customCriteria);
      if (validation.ok) {
        return validation.value;
      }
    }

    // Use environment-based preset if no custom criteria
    const environment = context?.constraints.targetEnvironment || 'production';
    return this.scorer.getScoringPreset(environment);
  }

  // Helper methods for context building
  private detectPackageManager(analysis: AnalysisData): string {
    if (analysis.files?.['package-lock.json']) return 'npm';
    if (analysis.files?.['yarn.lock']) return 'yarn';
    if (analysis.files?.['pnpm-lock.yaml']) return 'pnpm';
    return 'npm';
  }

  private extractBuildTools(analysis: AnalysisData): string[] {
    const tools: string[] = [];
    const packageJson = analysis.files?.['package.json'];

    if (packageJson && typeof packageJson === 'object') {
      const scripts = packageJson.scripts || {};
      if (scripts.build) tools.push('build-script');
      if (scripts.test) tools.push('test-runner');
      if (scripts.lint) tools.push('linter');

      const devDeps = packageJson.devDependencies || {};
      if (devDeps.webpack) tools.push('webpack');
      if (devDeps.vite) tools.push('vite');
      if (devDeps.typescript) tools.push('typescript');
    }

    return tools;
  }

  private detectDatabaseUsage(analysis: AnalysisData): boolean {
    const dependencies = analysis.dependencies || [];
    const dbKeywords = [
      'mongodb',
      'mysql',
      'postgres',
      'redis',
      'sqlite',
      'prisma',
      'sequelize',
      'typeorm',
    ];
    return dependencies.some((dep) => {
      const depName = typeof dep === 'string' ? dep : (dep as { name: string }).name;
      return dbKeywords.some((keyword) => depName.toLowerCase().includes(keyword));
    });
  }

  private extractPorts(analysis: AnalysisData): number[] {
    const ports: number[] = [];

    // Check common port patterns in code
    const files = analysis.files || {};
    const content = Object.values(files)
      .filter((f): f is string => typeof f === 'string')
      .join(' ')
      .toLowerCase();

    // Look for common port patterns
    const portMatches = content.match(/port[:\s=]+(\d+)/g);
    if (portMatches) {
      portMatches.forEach((match) => {
        const port = parseInt(match.replace(/[^\d]/g, ''));
        if (port > 1000 && port < 65535 && !ports.includes(port)) {
          ports.push(port);
        }
      });
    }

    // Default ports based on language/framework
    if (ports.length === 0) {
      const language = analysis.language || 'javascript';
      const defaultPort = getDefaultPort(language);
      ports.push(defaultPort);
    }

    return ports.slice(0, 3); // Limit to first 3 ports
  }

  private extractEnvironmentVars(analysis: AnalysisData): Record<string, string> {
    const envVars: Record<string, string> = {};

    // Look for common environment variables
    const files = analysis.files || {};
    for (const [_filename, content] of Object.entries(files)) {
      if (typeof content === 'string') {
        // Look for process.env usage
        const envMatches = content.match(/process\.env\.([A-Z_][A-Z0-9_]*)/g);
        if (envMatches) {
          envMatches.forEach((match) => {
            const varName = match.replace('process.env.', '');
            if (!envVars[varName]) {
              envVars[varName] = `\${${varName}}`;
            }
          });
        }
      }
    }

    // Add common defaults
    if (Object.keys(envVars).length === 0) {
      envVars['NODE_ENV'] = 'production';
      const defaultPort = getDefaultPort(analysis.language || 'javascript');
      envVars['PORT'] = defaultPort.toString();
    }

    return envVars;
  }

  private determineEnvironment(_config: SamplingConfig): 'development' | 'staging' | 'production' {
    // Could be extended to detect from repo structure or config
    return 'production'; // Default to production for sampling
  }

  private determineSecurityLevel(_config: SamplingConfig): 'basic' | 'standard' | 'strict' {
    // Could be extended based on detected security requirements
    return 'standard'; // Default to standard security
  }

  /**
   * Get available sampling strategies
   */
  getAvailableStrategies(): string[] {
    return ['security-first', 'performance-optimized', 'size-optimized', 'balanced'];
  }

  /**
   * Score variants using the internal scorer
   */
  async scoreVariants(
    variants: DockerfileVariant[],
    criteria: ScoringCriteria,
  ): Promise<Result<ScoredVariant[]>> {
    return this.scorer.scoreVariants(variants, criteria);
  }
}
