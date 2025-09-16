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
 *   path: '/path/to/project',
 *   includeTests: true
 * }, logger);
 *
 * if (result.ok) {
 *   const { language, framework } = result.value;
 *   logger.info('Repository analyzed', { language, framework });
 * }
 * ```
 */

import { joinPaths, getExtension, safeNormalizePath, toNativePath } from '@lib/path-utils';
import { promises as fs, constants } from 'node:fs';
import * as path from 'node:path';
import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import type { Logger } from '@lib/logger';
import { createStandardProgress } from '@mcp/progress-helper';
import { aiGenerateWithSampling } from '@mcp/tool-ai-helpers';
import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
import { getBaseImageRecommendations } from '@lib/base-images';
import type { ToolContext } from '@mcp/context';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import { Success, Failure, type Result } from '@types';
import { analyzeRepoSchema, type AnalyzeRepoParams } from './schema';
import { z } from 'zod';
import { parsePackageJson, getAllDependencies } from '@lib/parsing-package-json';
import { DEFAULT_PORTS } from '@config/defaults';
import { extractErrorMessage } from '@lib/error-utils';

// Define the result schema for type safety
const AnalyzeRepoResultSchema = z.object({
  ok: z.boolean(),
  sessionId: z.string(),
  language: z.string(),
  languageVersion: z.string().optional(),
  framework: z.string().optional(),
  frameworkVersion: z.string().optional(),
  buildSystem: z
    .object({
      type: z.string(),
      file: z.string(),
      buildCommand: z.string(),
      testCommand: z.string().optional(),
    })
    .optional(),
  dependencies: z.array(
    z.object({
      name: z.string(),
      version: z.string().optional(),
      type: z.string(),
    }),
  ),
  ports: z.array(z.number()),
  hasDockerfile: z.boolean(),
  hasDockerCompose: z.boolean(),
  hasKubernetes: z.boolean(),
  recommendations: z.object({
    baseImage: z.string(),
    buildStrategy: z.enum(['multi-stage', 'single-stage']),
    securityNotes: z.array(z.string()),
  }),
  confidence: z.number(),
  detectionMethod: z.enum(['signature', 'extension', 'fallback', 'ai-enhanced']),
  detectionDetails: z.object({
    signatureMatches: z.number(),
    extensionMatches: z.number(),
    frameworkSignals: z.number(),
    buildSystemSignals: z.number(),
  }),
  metadata: z.object({
    path: z.string(),
    depth: z.number(),
    timestamp: z.number(),
    includeTests: z.boolean().optional(),
    aiInsights: z.unknown().optional(),
  }),
  modules: z.array(z.string()).optional(), // Detected module paths for multi-module projects
});

// Define the result type from the schema
export type AnalyzeRepoResult = z.infer<typeof AnalyzeRepoResultSchema>;

// Define tool IO for type-safe session operations
const io = defineToolIO(analyzeRepoSchema, AnalyzeRepoResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastAnalyzedAt: z.date().optional(),
  analysisDepth: z.number().optional(),
  detectedLanguages: z.array(z.string()).default([]),
});
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
    extensions: ['.cs', '.fs', '.vb', '.csproj', '.fsproj', '.vbproj', '.sln'],
    files: ['global.json', 'Directory.Build.props'],
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
  'blazor-server': { files: [], dependencies: ['Microsoft.AspNetCore.Components.Server'] },
  'blazor-webassembly': {
    files: [],
    dependencies: ['Microsoft.AspNetCore.Components.WebAssembly'],
  },
  'blazor-hybrid': { files: [], dependencies: ['Microsoft.AspNetCore.Components.WebView'] },
  'grpc-service': { files: [], dependencies: ['Grpc.AspNetCore'] },
  'worker-service': { files: [], dependencies: ['Microsoft.Extensions.Hosting'] },
  'minimal-api': { files: [], dependencies: ['Microsoft.AspNetCore.OpenApi'] },
  'aspnet-webforms': { files: ['Default.aspx', 'Site.Master'], dependencies: ['System.Web.UI'] },
  'wcf-service': { files: ['service.svc', 'App.config'], dependencies: ['System.ServiceModel'] },
  'windows-service': {
    files: ['Program.cs', 'app.config'],
    dependencies: ['System.ServiceProcess'],
  },
  'entity-framework-6': { files: ['App.config'], dependencies: ['EntityFramework'] },
  'aspnet-webapi': { files: ['Global.asax', 'Web.config'], dependencies: ['System.Web.Http'] },
  'aspnet-mvc': { files: ['Global.asax', 'Web.config'], dependencies: ['System.Web.Mvc'] },
  'aspnet-framework': { files: ['Global.asax', 'Web.config'], dependencies: [] },
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
 * Detection details for confidence calculation
 */
interface DetectionDetails {
  signatureMatches: number;
  extensionMatches: number;
  frameworkSignals: number;
  buildSystemSignals: number;
}

/**
 * Calculates detection confidence score using weighted signal analysis.
 *
 * Invariant: Signature files carry more weight than extensions for accuracy
 * Postcondition: Returns score 0-100 with detection method classification
 */
function calculateConfidence(
  language: string,
  framework: string | undefined,
  buildSystem: any,
  detectionDetails: DetectionDetails,
): { confidence: number; method: 'signature' | 'extension' | 'fallback' | 'ai-enhanced' } {
  if (language === 'unknown') {
    return { confidence: 0, method: 'fallback' };
  }

  let score = 0;
  let method: 'signature' | 'extension' | 'fallback' | 'ai-enhanced' = 'fallback';

  // Language confidence - signature files are stronger indicators
  if (detectionDetails.signatureMatches > 0) {
    score += 40;
    method = 'signature';
  } else if (detectionDetails.extensionMatches > 0) {
    score += 25;
    method = 'extension';
  }

  // Framework detection adds confidence
  if (framework && detectionDetails.frameworkSignals > 0) {
    score += 25;
  }

  // Build system detection adds confidence
  if (buildSystem && detectionDetails.buildSystemSignals > 0) {
    score += 20;
  }

  // Multiple signals boost confidence
  if (detectionDetails.signatureMatches > 1) {
    score += 10;
  }

  return { confidence: Math.min(score, 100), method };
}

/**
 * Validate repository path exists and is accessible
 */
async function validateRepositoryPath(
  repoPath: string,
): Promise<{ valid: boolean; error?: string }> {
  try {
    // Convert to native path format for file system operations
    const nativePath = toNativePath(repoPath);
    const stats = await fs.stat(nativePath);
    if (!stats.isDirectory()) {
      return { valid: false, error: 'Path is not a directory' };
    }
    await fs.access(nativePath, constants.R_OK);
    return { valid: true };
  } catch (error) {
    const errorMsg = extractErrorMessage(error);
    return { valid: false, error: `Cannot access repository: ${errorMsg}` };
  }
}

/**
 * Detects primary programming language using signature files and extensions.
 *
 * Trade-off: Prioritizes signature files over extensions for higher accuracy
 */
async function detectLanguage(repoPath: string): Promise<{
  language: string;
  version?: string;
  detectionDetails: DetectionDetails;
  allFiles?: string[];
}> {
  // Get all files recursively (up to 2 levels deep)
  const allFiles: string[] = [];
  const getAllFiles = async (dir: string, depth = 0): Promise<void> => {
    if (depth > 2) return; // Limit depth
    const files = await fs.readdir(toNativePath(dir));

    for (const file of files) {
      const filePath = joinPaths(dir, file);
      const stats = await fs.stat(toNativePath(filePath));
      const relativePath = filePath.replace(`${repoPath}/`, '');

      if (stats.isFile()) {
        allFiles.push(relativePath);
      } else if (stats.isDirectory() && !file.startsWith('.') && file !== 'node_modules') {
        await getAllFiles(filePath, depth + 1);
      }
    }
  };

  await getAllFiles(repoPath);

  const fileStats = await Promise.all(
    allFiles.map(async (file) => {
      const filePath = joinPaths(repoPath, file);
      const stats = await fs.stat(toNativePath(filePath));
      return { name: file, path: filePath, isFile: stats.isFile() };
    }),
  );

  const detectionDetails: DetectionDetails = {
    signatureMatches: 0,
    extensionMatches: 0,
    frameworkSignals: 0,
    buildSystemSignals: 0,
  };

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
    const matchedFiles =
      signature.files?.filter((f) =>
        allFiles.some((file) => file === f || file.endsWith(`/${f}`)),
      ) ?? [];
    if (matchedFiles.length > 0) {
      detectionDetails.signatureMatches = matchedFiles.length;
      return { language: lang, detectionDetails, allFiles };
    }

    // Check for extensions
    const matchedExtensions =
      signature.extensions?.filter((ext) => (extensionCounts[ext] ?? 0) > 0) ?? [];
    if (matchedExtensions.length > 0) {
      detectionDetails.extensionMatches = matchedExtensions.length;
      return { language: lang, detectionDetails, allFiles };
    }
  }

  return { language: 'unknown', detectionDetails, allFiles };
}

/**
 * Detects framework using dependency analysis and configuration files.
 *
 * Design rationale: .NET requires special handling due to complex framework ecosystem
 */
async function detectFramework(
  repoPath: string,
  language: string,
  allFiles?: string[],
): Promise<{ framework?: string; version?: string; frameworkSignals: number } | undefined> {
  // Use provided files or scan directory
  const files = allFiles || (await fs.readdir(toNativePath(repoPath)));
  let frameworkSignals = 0;

  // Check package.json for JS/TS frameworks
  if (language === 'javascript' || language === 'typescript') {
    try {
      const packageJson = await parsePackageJson(repoPath);
      const allDeps = getAllDependencies(packageJson);

      for (const [framework, signature] of Object.entries(FRAMEWORK_SIGNATURES)) {
        const matchingDeps = signature.dependencies?.filter((dep) => dep in allDeps) ?? [];
        if (matchingDeps.length > 0) {
          frameworkSignals = matchingDeps.length;
          return { framework, frameworkSignals };
        }
      }
    } catch {
      // Package.json not found or invalid
    }
  }

  // .NET specific framework detection
  if (language === 'dotnet') {
    try {
      // Find all .csproj files (they should already be in allFiles from detectLanguage)
      const csprojFiles = files.filter((f) => f.endsWith('.csproj'));
      for (const csprojFile of csprojFiles) {
        try {
          const csprojPath = joinPaths(repoPath, csprojFile);
          const csprojContent = await fs.readFile(toNativePath(csprojPath), 'utf-8');

          // Check for .NET Framework version
          const frameworkVersionMatch = csprojContent.match(
            /<TargetFrameworkVersion>v(\d+\.\d+)<\/TargetFrameworkVersion>/,
          );
          if (frameworkVersionMatch) {
            const version = frameworkVersionMatch[1];

            // Determine specific framework type based oni iences
            frameworkSignals = 1;
            if (csprojContent.includes('System.Web.Http')) {
              frameworkSignals = 2; // More specific detection
              return version
                ? { framework: 'aspnet-webapi', version, frameworkSignals }
                : { framework: 'aspnet-webapi', frameworkSignals };
            } else if (csprojContent.includes('System.Web.Mvc')) {
              frameworkSignals = 2; // More specific detection
              return version
                ? { framework: 'aspnet-mvc', version, frameworkSignals }
                : { framework: 'aspnet-mvc', frameworkSignals };
            } else if (csprojContent.includes('System.Web')) {
              frameworkSignals = 2; // More specific detection
              return version
                ? { framework: 'aspnet-framework', version, frameworkSignals }
                : { framework: 'aspnet-framework', frameworkSignals };
            }

            return version
              ? { framework: 'dotnet-framework', version, frameworkSignals }
              : { framework: 'dotnet-framework', frameworkSignals };
          }

          // Check for .NET Core/5+ (uses TargetFramework without 'v' prefix)
          const coreVersionMatch = csprojContent.match(
            /<TargetFramework>(net\d+\.\d+|netcoreapp\d+\.\d+)<\/TargetFramework>/,
          );
          if (coreVersionMatch) {
            const version = coreVersionMatch[1];
            frameworkSignals = 1;

            if (csprojContent.includes('Microsoft.AspNetCore')) {
              frameworkSignals = 2; // More specific detection
              return version
                ? { framework: 'aspnet-core', version, frameworkSignals }
                : { framework: 'aspnet-core', frameworkSignals };
            }

            return version
              ? { framework: 'dotnet-core', version, frameworkSignals }
              : { framework: 'dotnet-core', frameworkSignals };
          }
        } catch {
          // Continue to next file if reading fails
        }
      }
    } catch {
      // Fall through to generic detection
    }
  }

  // Check for framework-specific files
  for (const [framework, signature] of Object.entries(FRAMEWORK_SIGNATURES)) {
    const matchingFiles = signature.files?.filter((f) => files.includes(f)) ?? [];
    if (matchingFiles.length > 0) {
      frameworkSignals = matchingFiles.length;
      return { framework, frameworkSignals };
    }
  }

  return { frameworkSignals: 0 };
}

/**
 * Detects module roots in multi-module projects (Maven, Gradle, Node monorepos)
 */
async function detectModuleRoots(
  repoPath: string,
  language: string,
  logger: Logger,
): Promise<string[]> {
  // Skip module detection for languages that typically don't have multi-module structures
  if (
    language === 'dotnet' ||
    language === 'python' ||
    language === 'go' ||
    language === 'rust' ||
    language === 'php' ||
    language === 'ruby'
  ) {
    return [];
  }

  const moduleRoots: string[] = [];
  const buildFiles =
    language === 'java'
      ? ['pom.xml', 'build.gradle', 'build.gradle.kts']
      : language === 'javascript' || language === 'typescript'
        ? ['package.json']
        : [];

  if (buildFiles.length === 0) return [];

  async function scanDirectory(dirPath: string, depth: number = 0): Promise<void> {
    if (depth > 3) return;

    try {
      const entries = await fs.readdir(toNativePath(dirPath), { withFileTypes: true });

      for (const buildFile of buildFiles) {
        const buildFilePath = joinPaths(dirPath, buildFile);
        try {
          // Directly read the file to avoid TOCTOU race condition
          if (buildFile === 'pom.xml') {
            const pomContent = await fs.readFile(toNativePath(buildFilePath), 'utf-8');
            const isParentPom = pomContent.includes('<modules>') && pomContent.includes('<module>');
            const srcDir = joinPaths(dirPath, 'src');
            const hasSrc = await fs
              .stat(srcDir)
              .then((stats) => stats.isDirectory())
              .catch(() => false);

            if (!isParentPom || hasSrc) {
              const relativePath = path.relative(repoPath, dirPath) || '.';
              if (!moduleRoots.includes(relativePath)) {
                moduleRoots.push(relativePath);
                logger.debug({ moduleRoot: relativePath, buildFile }, 'Found module root');
              }
            }
          } else {
            // For non-pom.xml files, just check if we can stat the file
            await fs.stat(buildFilePath);
            const relativePath = path.relative(repoPath, dirPath) || '.';
            if (!moduleRoots.includes(relativePath)) {
              moduleRoots.push(relativePath);
              logger.debug({ moduleRoot: relativePath, buildFile }, 'Found module root');
            }
          }
          break;
        } catch {
          // Build file doesn't exist, continue
        }
      }

      for (const entry of entries) {
        if (
          entry.isDirectory() &&
          !entry.name.startsWith('.') &&
          !['node_modules', 'target', 'build', 'dist', 'out'].includes(entry.name)
        ) {
          await scanDirectory(joinPaths(dirPath, entry.name), depth + 1);
        }
      }
    } catch (error) {
      logger.debug({ dirPath, error }, 'Error scanning directory for modules');
    }
  }

  await scanDirectory(repoPath);

  // Only return if we found multiple modules
  return moduleRoots.length > 1 ? moduleRoots : [];
}

/**
 * Detects build system by scanning for configuration files.
 * Provides build and test commands for downstream tools.
 */
async function detectBuildSystem(repoPath: string): Promise<
  | {
      type: string;
      file: string;
      buildCmd: string;
      testCmd?: string;
      buildSystemSignals: number;
    }
  | undefined
> {
  const files = await fs.readdir(toNativePath(repoPath));

  // Simple check for .NET projects - let AI figure out the details
  const csprojFile = files.find((f) => f.endsWith('.csproj'));
  if (csprojFile) {
    return {
      type: 'dotnet',
      file: csprojFile,
      buildCmd: 'dotnet build',
      testCmd: 'dotnet test',
      buildSystemSignals: 1,
    };
  }

  const slnFile = files.find((f) => f.endsWith('.sln'));
  if (slnFile) {
    return {
      type: 'dotnet',
      file: slnFile,
      buildCmd: 'dotnet build',
      testCmd: 'dotnet test',
      buildSystemSignals: 1,
    };
  }

  // Check other build systems
  for (const [system, config] of Object.entries(BUILD_SYSTEMS)) {
    if (files.includes(config.file)) {
      return {
        type: system,
        file: config.file,
        buildCmd: config.buildCmd,
        testCmd: config.testCmd,
        buildSystemSignals: 1,
      };
    }
  }

  return undefined;
}

/**
 * Analyzes project dependencies by parsing package managers.
 * Currently supports Node.js ecosystem; extensible for other languages.
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
 * Detects application ports using language/framework defaults.
 * Trade-off: Static mapping over dynamic analysis for reliability.
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
 * Scans for existing containerization files.
 * Used to inform recommendation strategy and avoid conflicts.
 */
async function checkDockerFiles(repoPath: string): Promise<{
  hasDockerfile: boolean;
  hasDockerCompose: boolean;
  hasKubernetes: boolean;
}> {
  const files = await fs.readdir(toNativePath(repoPath));

  return {
    hasDockerfile: files.includes('Dockerfile') || files.includes('dockerfile'),
    hasDockerCompose: files.includes('docker-compose.yml') || files.includes('docker-compose.yaml'),
    hasKubernetes:
      files.includes('k8s') || files.includes('kubernetes') || files.includes('deployment.yaml'),
  };
}

/**
 * Generates security recommendations based on dependency analysis.
 *
 * Trade-off: Static analysis over runtime scanning for faster execution
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
 * Analyzes repository structure and generates containerization recommendations.
 * Combines static analysis with optional AI enhancement for comprehensive insights.
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
  const logger = getToolLogger(context, 'analyze-repo');
  const timer = createToolTimer(logger, 'analyze-repo');

  try {
    // When no path is specified, use process.cwd() and ensure it's properly normalized
    const defaultPath = process.cwd();
    const { path: rawPath = defaultPath, depth = 3, includeTests = false } = params;
    // Apply safeNormalizePath to handle Windows path variations
    let repoPath = safeNormalizePath(rawPath);

    // Use path.resolve to get the absolute path, but then normalize it again
    // to handle any duplication that might occur
    if (!path.isAbsolute(repoPath)) {
      repoPath = path.resolve(repoPath);
      repoPath = safeNormalizePath(repoPath);
    }

    logger.info({ repoPath, depth, includeTests }, 'Starting repository analysis');

    // Progress: Starting analysis
    if (progress) await progress('VALIDATING');

    // Validate repository path
    const validation = await validateRepositoryPath(repoPath);
    if (!validation.valid) {
      return Failure(validation.error ?? 'Invalid repository path');
    }

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: _session } = sessionResult.value;
    const slice = useSessionSlice('analyze-repo', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info({ sessionId, repoPath }, 'Starting repository analysis with session');

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    if (progress) await progress('EXECUTING');

    // AI enhancement available through context
    const hasAI =
      context.sampling &&
      context.getPrompt &&
      context.sampling !== null &&
      context.getPrompt !== null;

    // Perform analysis
    const languageInfo = await detectLanguage(repoPath);
    const frameworkInfo = await detectFramework(
      repoPath,
      languageInfo.language,
      languageInfo.allFiles,
    );
    const buildSystemRaw = await detectBuildSystem(repoPath);
    const dependencies = await analyzeDependencies(repoPath, languageInfo.language);
    const ports = await detectPorts(languageInfo.language);
    const dockerInfo = await checkDockerFiles(repoPath);

    // Detect multi-module structure
    const modules = await detectModuleRoots(repoPath, languageInfo.language, logger);
    if (modules.length > 0) {
      logger.info(
        { modules, language: languageInfo.language },
        'Detected multi-module project structure',
      );
    }

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

    // Build recommendations with framework context
    const baseImageOptions = {
      language: languageInfo.language,
      preference: 'balanced' as const,
      ...(frameworkInfo?.framework && { framework: frameworkInfo.framework }),
    };
    const baseImageRecommendations = getBaseImageRecommendations(baseImageOptions);
    const baseImage = baseImageRecommendations.primary;
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

    // Calculate confidence score and detection method
    const detectionDetails: DetectionDetails = {
      signatureMatches: languageInfo.detectionDetails.signatureMatches,
      extensionMatches: languageInfo.detectionDetails.extensionMatches,
      frameworkSignals: frameworkInfo?.frameworkSignals ?? 0,
      buildSystemSignals: buildSystemRaw?.buildSystemSignals ?? 0,
    };

    const { confidence, method } = calculateConfidence(
      languageInfo.language,
      frameworkInfo?.framework,
      buildSystem,
      detectionDetails,
    );

    const result: AnalyzeRepoResult = {
      ok: true,
      sessionId,
      language: languageInfo.language,
      confidence,
      detectionMethod: method,
      detectionDetails,
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
        path: repoPath,
        depth,
        includeTests,
        timestamp: Date.now(),
        ...(aiInsights !== undefined && { aiInsights }),
        ...(languageInfo.allFiles && {
          projectFiles: languageInfo.allFiles.filter(
            (f) =>
              f.endsWith('.csproj') ||
              f.endsWith('.sln') ||
              f === 'package.json' ||
              f === 'pom.xml' ||
              f === 'build.gradle' ||
              f === 'go.mod' ||
              f === 'Cargo.toml' ||
              f === 'requirements.txt',
          ),
        }),
      },
      ...(modules.length > 0 && { modules }), // Include detected modules if multi-module project
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastAnalyzedAt: new Date(),
        analysisDepth: params.depth || 3,
        detectedLanguages: frameworkInfo?.framework
          ? [languageInfo.language, frameworkInfo.framework]
          : [languageInfo.language],
      },
    });

    // Progress: Finalizing results
    if (progress) await progress('FINALIZING');

    timer.end({ language: languageInfo.language });
    logger.info({ language: languageInfo.language }, 'Repository analysis completed');

    // Progress: Complete
    if (progress) await progress('COMPLETE');

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Repository analysis failed');

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Main entry point for repository analysis tool.
 * Provides comprehensive project analysis for containerization planning.
 */
export const analyzeRepo = analyzeRepoImpl;
