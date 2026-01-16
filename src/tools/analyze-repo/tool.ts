import path from 'node:path';
import { promises as fs } from 'node:fs';
import type { z } from 'zod';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/core/context';
import { tool } from '@/types/tool';
import { getToolLogger } from '@/lib/tool-helpers';
import { validatePathOrFail } from '@/lib/validation-helpers';
import { analyzeRepoSchema, type RepositoryAnalysis, type ModuleInfo } from './schema';
import { pluralize } from '@/lib/summary-helpers';
import {
  parsePackageJson,
  parseGradle,
  parsePomXml,
  parsePythonConfig,
  parseCargoToml,
  parseCsProj,
  parseGoMod,
  type ParsedConfig,
} from './parsers';

/**
 * Scan repository directory and gather file information
 */
async function gatherRepositoryInfo(repoPath: string): Promise<{
  configFiles: Record<string, string>;
  fileList: string[];
  directoryTree: string[];
}> {
  // Get file list (top 100 files)
  const files: string[] = [];
  const configFileContents: Record<string, string> = {};
  const dirTree: string[] = [];

  async function scanDirectory(
    dir: string,
    depth: number = 0,
    maxDepth: number = 3,
  ): Promise<void> {
    if (depth > maxDepth) return;

    try {
      const entries = await fs.readdir(dir, { withFileTypes: true });

      for (const entry of entries) {
        const fullPath = path.join(dir, entry.name);
        const relativePath = path.relative(repoPath, fullPath);

        // Skip node_modules, .git, and other common ignored directories
        if (entry.name.match(/^(node_modules|\.git|\.vscode|\.idea|dist|build|target|bin|obj)$/)) {
          continue;
        }

        if (entry.isDirectory()) {
          dirTree.push(`${'  '.repeat(depth)}${entry.name}/`);
          await scanDirectory(fullPath, depth + 1, maxDepth);
        } else {
          files.push(relativePath);

          // Read config files
          const configFilePattern = new RegExp(
            '^(package\\.json|pom\\.xml|build\\.gradle|build\\.gradle\\.kts|' +
              'requirements\\.txt|pyproject\\.toml|Cargo\\.toml|go\\.mod|' +
              'composer\\.json|Gemfile|.*\\.csproj|.*\\.fsproj|.*\\.vbproj|' +
              'Dockerfile|docker-compose\\.yml|application\\.properties|application\\.yml)$',
          );
          if (entry.name.match(configFilePattern)) {
            try {
              const content = await fs.readFile(fullPath, 'utf-8');
              // Limit content to 1000 characters to avoid token overload
              configFileContents[relativePath] =
                content.length > 1000 ? `${content.substring(0, 1000)}...[truncated]` : content;
            } catch {
              // Skip files that can't be read
            }
          }
        }

        // Limit total files scanned
        if (files.length > 100) break;
      }
    } catch {
      // Skip directories that can't be read
    }
  }

  await scanDirectory(repoPath);

  return {
    configFiles: configFileContents,
    fileList: files.slice(0, 50),
    directoryTree: dirTree.slice(0, 30),
  };
}

/**
 * Analyze repository deterministically by parsing config files
 */
async function analyzeRepositoryDeterministically(
  repoPath: string,
  repoInfo: { configFiles: Record<string, string>; fileList: string[]; directoryTree: string[] },
  ctx: ToolContext,
): Promise<ModuleInfo[]> {
  const logger = getToolLogger(ctx, 'analyze-repo');
  const configFilePaths = Object.keys(repoInfo.configFiles);

  const configsByDirectory = new Map<string, ParsedConfig[]>();

  for (const configPath of configFilePaths) {
    const fullPath = path.join(repoPath, configPath);
    const dirName = path.dirname(fullPath);
    const fileName = path.basename(configPath);

    logger.debug(
      `Processing config file: ${configPath}, fileName: ${fileName}, dirName: ${dirName}`,
    );

    let parsedConfig: ParsedConfig | null = null;

    try {
      // Node.js
      if (fileName === 'package.json') {
        parsedConfig = await parsePackageJson(fullPath);
      }
      // Java - Maven
      else if (fileName === 'pom.xml') {
        parsedConfig = await parsePomXml(fullPath);
      }
      // Java - Gradle
      else if (fileName.match(/^build\.gradle(\.kts)?$/)) {
        parsedConfig = await parseGradle(fullPath);
      }
      // Python
      else if (fileName === 'requirements.txt' || fileName === 'pyproject.toml') {
        parsedConfig = await parsePythonConfig(fullPath);
      }
      // Rust
      else if (fileName === 'Cargo.toml') {
        parsedConfig = await parseCargoToml(fullPath);
      }
      // .NET
      else if (fileName.match(/\.csproj$/)) {
        parsedConfig = await parseCsProj(fullPath);
      }
      // Go
      else if (fileName === 'go.mod') {
        parsedConfig = await parseGoMod(fullPath);
      } else {
        logger.debug(`Skipping unrecognized config file: ${configPath} (fileName: ${fileName})`);
      }

      if (parsedConfig) {
        const dirConfigs = configsByDirectory.get(dirName);
        if (dirConfigs) {
          dirConfigs.push(parsedConfig);
        } else {
          configsByDirectory.set(dirName, [parsedConfig]);
        }
      }
    } catch (error) {
      // Log but continue - don't fail entire analysis for one bad config
      logger.warn(
        { configPath, error: error instanceof Error ? error.message : String(error) },
        'Failed to parse config file',
      );
    }
  }

  const modules: ModuleInfo[] = [];
  for (const [dirName, configs] of configsByDirectory.entries()) {
    if (configs.length === 0) continue;

    const primaryConfig = configs[0];
    if (!primaryConfig) continue;

    const buildSystems = configs
      .filter(
        (c): c is typeof c & { buildSystem: NonNullable<typeof c.buildSystem> } =>
          c.buildSystem !== undefined,
      )
      .map((c) => ({
        type: c.buildSystem.type,
        languageVersion: c.languageVersion,
      }));

    modules.push({
      name: path.basename(dirName),
      modulePath: dirName,
      language: primaryConfig.language || 'other',
      frameworks: primaryConfig.framework
        ? [{ name: primaryConfig.framework, version: primaryConfig.frameworkVersion }]
        : undefined,
      buildSystems: buildSystems.length > 0 ? buildSystems : undefined,
      dependencies: primaryConfig.dependencies,
      ports: primaryConfig.ports,
      entryPoint: primaryConfig.entryPoint,
    });
  }

  return modules;
}

/**
 * Analyze repository structure and detect technologies deterministically
 */
async function handleAnalyzeRepo(
  input: z.infer<typeof analyzeRepoSchema>,
  ctx: ToolContext,
): Promise<Result<RepositoryAnalysis>> {
  const logger = getToolLogger(ctx, 'analyze-repo');

  // Validate and resolve repository path
  const pathResult = await validatePathOrFail(input.repositoryPath, {
    mustExist: true,
    mustBeDirectory: true,
  });
  if (!pathResult.ok) return pathResult;

  const repoPath = pathResult.value;

  try {
    // If modules are provided by user, use them
    if (input.modules && input.modules.length > 0) {
      const numberOfModules = input.modules.length;
      const isMonorepo = numberOfModules > 1;

      logger.info({ moduleCount: numberOfModules }, 'Using pre-provided modules');

      return Success({
        modules: input.modules,
        isMonorepo,
        analyzedPath: repoPath,
      });
    }

    // No modules provided - perform deterministic analysis
    logger.info({ repoPath }, 'Starting deterministic repository analysis');

    // Gather repository information
    const repoInfo = await gatherRepositoryInfo(repoPath);

    // Analyze deterministically by parsing config files
    const modules = await analyzeRepositoryDeterministically(repoPath, repoInfo, ctx);

    if (modules.length === 0) {
      return Failure('No modules detected in repository', {
        message: 'No buildable projects found',
        hint: 'Could not identify any recognizable project files',
        resolution:
          'Ensure the repository contains project files like package.json, pom.xml, requirements.txt, etc.',
      });
    }

    const isMonorepo = modules.length > 1;

    logger.info({ moduleCount: modules.length, isMonorepo }, 'Repository analysis complete');

    // Generate summary
    const modulesText =
      modules.length === 1
        ? `${modules[0]?.language || 'unknown'} project`
        : `${pluralize(modules.length, 'module')} (${modules.map((m) => m.language).join(', ')})`;

    const summary = `✅ Analyzed repository at ${repoPath}. Detected ${modulesText}.${isMonorepo ? ' Monorepo structure identified.' : ''} Ready for Dockerfile generation.`;

    return Success({
      summary,
      modules,
      isMonorepo,
      analyzedPath: repoPath,
    });
  } catch (e) {
    const error = e as Error;
    logger.error({ error: error.message }, 'Repository analysis failed');
    return Failure(`Repository analysis failed: ${error.message}`, {
      message: `Repository analysis failed: ${error.message}`,
      hint: 'Failed to analyze repository',
      resolution: 'Verify the path exists and contains a valid project structure',
    });
  }
}

export default tool({
  name: 'analyze-repo',
  description:
    'Analyze repository structure and detect technologies by parsing config files to prepare for containerization and generating container artifacts like Dockerfiles and Kubernetes manifests',
  category: 'analysis',
  version: '4.0.0',
  schema: analyzeRepoSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  chainHints: {
    success:
      'Repository analysis completed successfully. Continue by calling the generate-dockerfile or fix-dockerfile tools to create or fix your Dockerfile.',
    failure: 'Repository analysis failed. Please check the logs for details.',
  },
  handler: handleAnalyzeRepo,
});
