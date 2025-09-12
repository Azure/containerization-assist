/**
 * Repository Analysis Tool
 *
 * Analyzes repository structure to detect programming languages, frameworks,
 * build systems, and generates containerization recommendations.
 *
 * @example
 * ```typescript
 * const result = await analyzeRepo({
 *   sessionId: 'session-123',
 *   repoPath: '/path/to/project',
 *   includeTests: true
 * }, logger);
 *
 * if (result.ok) {
 *   const { language, framework } = result.value;
 *   logger.info('Repository analyzed', { language, framework });
 * }
 * ```
 */

import { joinPaths, getExtension } from '@lib/path-utils';
import { promises as fs } from 'node:fs';
import { getSession, updateSession } from '@mcp/tool-session-helpers';
import { createStandardProgress } from '@mcp/progress-helper';
import { aiGenerateWithSampling } from '@mcp/tool-ai-helpers';
import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
import { getRecommendedBaseImage } from '../../lib/base-images';
import type { ToolContext } from '../../mcp/context';
import { createTimer, createLogger } from '../../lib/logger';
import { Success, Failure, type Result, type AnalyzeRepoResult } from '../../types';
import type { AnalyzeRepoParams } from './schema';
import { parsePackageJson, getAllDependencies } from '../../lib/parsing-package-json';
import { DEFAULT_PORTS } from '../../config/defaults';
import { getSuccessProgression } from '../../workflows/workflow-progression';
import { TOOL_NAMES } from '../../exports/tool-names.js';
import { extractErrorMessage } from '../../lib/error-utils';

export type { AnalyzeRepoResult } from '../../types';
const LANGUAGE_SIGNATURES: Record<string, { extensions: string[]; files: string[] }> = {
  javascript: {
    extensions: ['', '.mjs', '.cjs'],
    files: ['package.json', 'node_modules'],
  },
  typescript: {
    extensions: ['.ts', '.tsx'],
    files: ['tsconfig.json', 'package.json'],
  },
  python: {
    extensions: ['.py'],
    files: ['requirements.txt', 'setup.py', 'pyproject.toml', 'Pipfile'],
  },
  java: {
    extensions: ['.java'],
    files: ['pom.xml', 'build.gradle', 'build.gradle.kts'],
  },
  go: {
    extensions: ['.go'],
    files: ['go.mod', 'go.sum'],
  },
  rust: {
    extensions: ['.rs'],
    files: ['Cargo.toml', 'Cargo.lock'],
  },
  ruby: {
    extensions: ['.rb'],
    files: ['Gemfile', 'Gemfile.lock', 'Rakefile'],
  },
  php: {
    extensions: ['.php'],
    files: ['composer.json', 'composer.lock'],
  },
  dotnet: {
    extensions: ['.cs', '.fs', '.vb'],
    files: ['.csproj', '.fsproj', '.vbproj', '.sln', 'global.json', 'Directory.Build.props'],
  },
};

// Framework detection configuration
const FRAMEWORK_SIGNATURES: Record<string, { files: string[]; dependencies?: string[] }> = {
  express: { files: [], dependencies: ['express'] },
  nestjs: { files: ['nest-cli.json'], dependencies: ['@nestjs/core'] },
  nextjs: { files: ['next.config', 'next.config.mjs'], dependencies: ['next'] },
  react: { files: [], dependencies: ['react', 'react-dom'] },
  vue: { files: ['vue.config'], dependencies: ['vue'] },
  angular: { files: ['angular.json'], dependencies: ['@angular/core'] },
  django: { files: ['manage.py'], dependencies: ['django'] },
  flask: { files: [], dependencies: ['flask'] },
  fastapi: { files: [], dependencies: ['fastapi'] },
  spring: { files: ['pom.xml', 'build.gradle'], dependencies: [] },
  rails: { files: ['Gemfile'], dependencies: ['rails'] },
  laravel: { files: ['artisan'], dependencies: [] },
  'aspnet-core': { files: [], dependencies: ['Microsoft.AspNetCore'] },
  blazor: { files: [], dependencies: ['Microsoft.AspNetCore.Components'] },
  'minimal-api': { files: [], dependencies: ['Microsoft.AspNetCore.OpenApi'] },
};

// Build system detection
const BUILD_SYSTEMS = {
  npm: { file: 'package.json', buildCmd: 'npm run build', testCmd: 'npm test' },
  yarn: { file: 'yarn.lock', buildCmd: 'yarn build', testCmd: 'yarn test' },
  pnpm: { file: 'pnpm-lock.yaml', buildCmd: 'pnpm build', testCmd: 'pnpm test' },
  maven: { file: 'pom.xml', buildCmd: 'mvn package', testCmd: 'mvn test' },
  gradle: { file: 'build.gradle', buildCmd: 'gradle build', testCmd: 'gradle test' },
  cargo: { file: 'Cargo.toml', buildCmd: 'cargo build --release', testCmd: 'cargo test' },
  go: { file: 'go.mod', buildCmd: 'go build', testCmd: 'go test ./...' },
  pip: { file: 'requirements.txt', buildCmd: 'python setup.py build', testCmd: 'pytest' },
  poetry: { file: 'pyproject.toml', buildCmd: 'poetry build', testCmd: 'poetry run pytest' },
  composer: { file: 'composer.json', buildCmd: 'composer install', testCmd: 'phpunit' },
  bundler: { file: 'Gemfile', buildCmd: 'bundle install', testCmd: 'bundle exec rspec' },
  dotnet: { file: '.csproj', buildCmd: 'dotnet build', testCmd: 'dotnet test' },
  'dotnet-sln': { file: '.sln', buildCmd: 'dotnet build', testCmd: 'dotnet test' },
};

/**
 * Validate repository path exists and is accessible
 */
async function validateRepositoryPath(
  repoPath: string,
): Promise<{ valid: boolean; error?: string }> {
  try {
    const stats = await fs.stat(repoPath);
    if (!stats.isDirectory()) {
      return { valid: false, error: 'Path is not a directory' };
    }
    await fs.access(repoPath, fs.constants.R_OK);
    return { valid: true };
  } catch (error) {
    const errorMsg = extractErrorMessage(error);
    return { valid: false, error: `Cannot access repository: ${errorMsg}` };
  }
}

/**
 * Detect primary programming language
 */
async function detectLanguage(repoPath: string): Promise<{ language: string; version?: string }> {
  const files = await fs.readdir(repoPath);
  const fileStats = await Promise.all(
    files.map(async (file) => {
      const filePath = joinPaths(repoPath, file);
      const stats = await fs.stat(filePath);
      return { name: file, path: filePath, isFile: stats.isFile() };
    }),
  );

  // Count file extensions
  const extensionCounts: Record<string, number> = {};
  for (const file of fileStats.filter((f) => f.isFile)) {
    const ext = getExtension(file.name);
    if (ext) {
      extensionCounts[ext] = (extensionCounts[ext] ?? 0) + 1;
    }
  }

  // Check for language signatures
  for (const [lang, signature] of Object.entries(LANGUAGE_SIGNATURES)) {
    // Check for specific files
    const hasFiles = signature.files?.some((f) => files.includes(f)) ?? false;
    if (hasFiles) {
      return { language: lang };
    }

    // Check for extensions
    const hasExtensions =
      signature.extensions?.some((ext) => (extensionCounts[ext] ?? 0) > 0) ?? false;
    if (hasExtensions) {
      return { language: lang };
    }
  }

  return { language: 'unknown' };
}

/**
 * Detect framework
 */
async function detectFramework(
  repoPath: string,
  language: string,
): Promise<{ framework?: string; version?: string } | undefined> {
  const files = await fs.readdir(repoPath);

  // Check package.json for JS/TS frameworks
  if (language === 'javascript' || language === 'typescript') {
    try {
      const packageJson = await parsePackageJson(repoPath);
      const allDeps = getAllDependencies(packageJson);

      for (const [framework, signature] of Object.entries(FRAMEWORK_SIGNATURES)) {
        if (signature.dependencies?.some((dep) => dep in allDeps)) {
          return { framework };
        }
      }
    } catch {
      // Package.json not found or invalid
    }
  }

  // Check for framework-specific files
  for (const [framework, signature] of Object.entries(FRAMEWORK_SIGNATURES)) {
    if (signature.files?.some((f) => files.includes(f))) {
      return { framework };
    }
  }

  return undefined;
}

/**
 * Detect build system
 */
async function detectBuildSystem(repoPath: string): Promise<
  | {
      type: string;
      file: string;
      buildCmd: string;
      testCmd?: string;
    }
  | undefined
> {
  const files = await fs.readdir(repoPath);

  for (const [system, config] of Object.entries(BUILD_SYSTEMS)) {
    if (files.includes(config.file)) {
      return {
        type: system,
        file: config.file,
        buildCmd: config.buildCmd,
        testCmd: config.testCmd,
      };
    }
  }

  return undefined;
}

/**
 * Analyze dependencies
 */
async function analyzeDependencies(
  repoPath: string,
  language: string,
): Promise<Array<{ name: string; version?: string; type: string }>> {
  const dependencies: Array<{ name: string; version?: string; type: string }> = [];

  if (language === 'javascript' || language === 'typescript') {
    try {
      const packageJson = await parsePackageJson(repoPath);

      // Production dependencies
      for (const [name, version] of Object.entries(packageJson.dependencies ?? {})) {
        dependencies.push({ name, version: String(version), type: 'production' });
      }

      // Dev dependencies
      for (const [name, version] of Object.entries(packageJson.devDependencies ?? {})) {
        dependencies.push({ name, version: String(version), type: 'development' });
      }
    } catch {
      // Package.json not found or invalid
    }
  }

  return dependencies;
}

/**
 * Detect exposed ports
 */
async function detectPorts(language: string): Promise<number[]> {
  const ports: Set<number> = new Set();

  // Use centralized default ports by language/framework
  const languageKey = language as keyof typeof DEFAULT_PORTS;
  const languagePorts = DEFAULT_PORTS[languageKey] || DEFAULT_PORTS.default;

  if (languagePorts) {
    languagePorts.forEach((port) => ports.add(port));
  }

  return Array.from(ports);
}

/**
 * Check for Docker files
 */
async function checkDockerFiles(repoPath: string): Promise<{
  hasDockerfile: boolean;
  hasDockerCompose: boolean;
  hasKubernetes: boolean;
}> {
  const files = await fs.readdir(repoPath);

  return {
    hasDockerfile: files.includes('Dockerfile') || files.includes('dockerfile'),
    hasDockerCompose: files.includes('docker-compose.yml') || files.includes('docker-compose.yaml'),
    hasKubernetes:
      files.includes('k8s') || files.includes('kubernetes') || files.includes('deployment.yaml'),
  };
}

/**
 * Get security recommendations
 */
function getSecurityRecommendations(
  dependencies: Array<{ name: string; version?: string; type: string }>,
): string[] {
  const recommendations: string[] = [];

  // Check for known vulnerable packages
  const vulnerablePackages = ['lodash', 'moment', 'request'];
  const hasVulnerable = dependencies.some((dep) => vulnerablePackages.includes(dep.name));

  if (hasVulnerable) {
    recommendations.push('Consider updating or replacing deprecated/vulnerable packages');
  }

  if (dependencies.length > 50) {
    recommendations.push(
      'Large number of dependencies detected - consider reducing for smaller attack surface',
    );
  }

  recommendations.push('Use multi-stage builds to minimize final image size');
  recommendations.push('Run containers as non-root user');
  recommendations.push('Scan images regularly for vulnerabilities');

  return recommendations;
}

/**
 * Repository analysis implementation - direct execution with selective progress
 */
async function analyzeRepoImpl(
  params: AnalyzeRepoParams,
  context: ToolContext,
): Promise<Result<AnalyzeRepoResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Optional progress reporting for complex operations
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = context.logger || createLogger({ name: 'analyze-repo' });
  const timer = createTimer(logger, 'analyze-repo');

  try {
    const { repoPath = process.cwd(), depth = 3, includeTests = false } = params;

    logger.info({ repoPath, depth, includeTests }, 'Starting repository analysis');

    // Progress: Starting analysis
    if (progress) await progress('VALIDATING');

    // Validate repository path
    const validation = await validateRepositoryPath(repoPath);
    if (!validation.valid) {
      return Failure(validation.error ?? 'Invalid repository path');
    }

    // Get or create session
    const sessionResult = await getSession(params.sessionId, context);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId, repoPath }, 'Starting repository analysis with session');

    // Repository analysis discovers language, framework, and configuration patterns
    if (progress) await progress('EXECUTING');

    // AI enhancement available through context
    const hasAI =
      context.sampling &&
      context.getPrompt &&
      context.sampling !== null &&
      context.getPrompt !== null;

    // Perform analysis
    const languageInfo = await detectLanguage(repoPath);
    const frameworkInfo = await detectFramework(repoPath, languageInfo.language);
    const buildSystemRaw = await detectBuildSystem(repoPath);
    const dependencies = await analyzeDependencies(repoPath, languageInfo.language);
    const ports = await detectPorts(languageInfo.language);
    const dockerInfo = await checkDockerFiles(repoPath);

    // Get AI insights using standardized helper if available
    let aiInsights: string | undefined;
    if (hasAI) {
      try {
        logger.debug('Using AI to enhance repository analysis');

        // Prepare prompt arguments
        let promptArgs = {
          language: languageInfo.language,
          framework: frameworkInfo?.framework,
          buildSystem: buildSystemRaw?.type,
          dependencies: dependencies
            .slice(0, 10)
            .map((dep) => dep.name)
            .join(', '), // Limit for prompt length
          hasTests: dependencies.some(
            (dep) =>
              dep.name.includes('test') || dep.name.includes('jest') || dep.name.includes('mocha'),
          ),
          hasDocker: dockerInfo.hasDockerfile,
          ports: ports.join(', '),
          fileCount: dependencies.length, // Rough estimate
          repoStructure: `${languageInfo.language} project with ${frameworkInfo?.framework || 'standard'} structure`,
        };

        // Enhance with knowledge context
        try {
          const enhancedArgs = await enhancePromptWithKnowledge(promptArgs, {
            operation: 'analyze_repository',
            ...(languageInfo.language && { language: languageInfo.language }),
            ...(frameworkInfo?.framework && { framework: frameworkInfo.framework }),
            environment: 'production',
            tags: [
              'analysis',
              'repository',
              languageInfo.language,
              frameworkInfo?.framework,
            ].filter(Boolean) as string[],
          });
          // Only use enhanced args if they contain the original fields
          if (enhancedArgs.language && enhancedArgs.dependencies) {
            promptArgs = enhancedArgs as typeof promptArgs;
            logger.info('Enhanced repository analysis with knowledge');
          }
        } catch (error) {
          logger.debug({ error }, 'Knowledge enhancement failed, using base prompt');
        }

        const aiResult = await aiGenerateWithSampling(logger, context, {
          promptName: 'enhance-repo-analysis',
          promptArgs,
          expectation: 'text',
          fallbackBehavior: 'error',
          maxRetries: 2,
          maxTokens: 1500,
          modelHints: ['analysis'],
          maxCandidates: 1,
          enableSampling: false,
        });

        if (aiResult.ok && aiResult.value.winner.content) {
          aiInsights = aiResult.value.winner.content;
          logger.info('AI analysis enhancement completed successfully');
        } else {
          logger.error(
            {
              tool: 'analyze-repo',
              operation: 'enhance-repo-analysis',
              error: aiResult.ok ? 'Empty response' : aiResult.error,
            },
            'AI repository analysis failed',
          );
          logger.debug(
            { error: aiResult.ok ? 'Empty response' : aiResult.error },
            'AI analysis enhancement failed, continuing with basic analysis',
          );
        }
      } catch (error) {
        logger.debug(
          { error: extractErrorMessage(error) },
          'AI analysis enhancement failed, continuing with basic analysis',
        );
      }
    } else {
      logger.debug('No AI context available, using basic analysis');
    }

    // Build recommendations
    const baseImage = getRecommendedBaseImage(languageInfo.language);
    const securityNotes = getSecurityRecommendations(dependencies);

    // Transform build system
    const buildSystem = buildSystemRaw
      ? {
          type: buildSystemRaw.type,
          file: buildSystemRaw.file,
          buildCommand: buildSystemRaw.buildCmd,
          ...(buildSystemRaw.testCmd !== undefined && { testCommand: buildSystemRaw.testCmd }),
        }
      : undefined;

    const result: AnalyzeRepoResult = {
      ok: true,
      sessionId,
      language: languageInfo.language,
      ...(languageInfo.version !== undefined && { languageVersion: languageInfo.version }),
      ...(frameworkInfo?.framework !== undefined && { framework: frameworkInfo.framework }),
      ...(frameworkInfo?.version !== undefined && { frameworkVersion: frameworkInfo.version }),
      ...(buildSystem !== undefined && { buildSystem }),
      dependencies,
      ports,
      hasDockerfile: dockerInfo.hasDockerfile,
      hasDockerCompose: dockerInfo.hasDockerCompose,
      hasKubernetes: dockerInfo.hasKubernetes,
      recommendations: {
        baseImage,
        buildStrategy: buildSystem ? 'multi-stage' : 'single-stage',
        securityNotes,
      },
      metadata: {
        repoPath,
        depth,
        includeTests,
        timestamp: Date.now(),
        ...(aiInsights !== undefined && { aiInsights }),
      },
    };

    // Update session with analysis result using simplified helper
    const updateResult = await updateSession(
      sessionId,
      {
        repo_path: repoPath,
        analysis_result: {
          language: languageInfo.language,
          ...(languageInfo.version && { language_version: languageInfo.version }),
          ...(frameworkInfo?.framework && { framework: frameworkInfo.framework }),
          ...(frameworkInfo?.version && { framework_version: frameworkInfo.version }),
          ...(buildSystem && {
            build_system: {
              type: buildSystem.type,
              build_file: buildSystem.file,
              ...(buildSystem.buildCommand && { build_command: buildSystem.buildCommand }),
            },
          }),
          dependencies: dependencies.map((d) => ({
            name: d.name,
            ...(d.version && { version: d.version }),
            type:
              d.type === 'production'
                ? ('runtime' as const)
                : d.type === 'development'
                  ? ('dev' as const)
                  : ('test' as const),
          })),
          has_tests: dependencies.some((dep) => dep.type === 'test'),
          ports,
          docker_compose_exists: dockerInfo.hasDockerCompose,
          recommendations: {
            baseImage,
            buildStrategy: buildSystem ? 'multi-stage' : 'single-stage',
            securityNotes,
          },
        },
        completed_steps: [...(session?.completed_steps ?? []), 'analyze-repo'],
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn({ error: updateResult.error }, 'Failed to update session with analysis result');
    }

    // Progress: Finalizing results
    if (progress) await progress('FINALIZING');

    timer.end({ language: languageInfo.language });
    logger.info({ language: languageInfo.language }, 'Repository analysis completed');

    // Progress: Complete
    if (progress) await progress('COMPLETE');

    const sessionContext = {
      completed_steps: session.completed_steps || [],
      analysis_result: result,
    };
    const enrichedResult = {
      ...result,
      NextStep: getSuccessProgression(TOOL_NAMES.ANALYZE_REPO, sessionContext).summary,
    };

    return Success(enrichedResult);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Repository analysis failed');

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Analyze repository tool with selective progress reporting
 */
export const analyzeRepo = analyzeRepoImpl;
