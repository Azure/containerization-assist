/**
 * Generate Dockerfile Tool
 *
 * Analyzes repository and queries knowledgebase to gather insights and return
 * structured requirements for creating a Dockerfile. This tool helps users
 * understand best practices and recommendations before actual Dockerfile generation.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 */

import { validatePath } from '@/lib/validation';
import { Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import {
  generateDockerfileSchema,
  type BaseImageRecommendation,
  type DockerfilePlan,
  type DockerfileRequirement,
  type GenerateDockerfileParams,
  type DockerfileAnalysis,
  type EnhancementGuidance,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';

const name = 'generate-dockerfile';
const description =
  'Gather insights from knowledgebase and return requirements for Dockerfile creation or enhancement. Automatically detects existing Dockerfiles and provides detailed analysis and guidance.';
const version = '2.0.0';

type DockerfileCategory = 'baseImages' | 'security' | 'optimization' | 'bestPractices';

/**
 * Extended input parameters that include optional existing Dockerfile data.
 * This is used internally to pass Dockerfile analysis results from the run function to buildPlan.
 */
interface ExtendedDockerfileParams extends GenerateDockerfileParams {
  _existingDockerfile?: {
    path: string;
    content: string;
    analysis: DockerfileAnalysis;
    guidance: EnhancementGuidance;
  };
}

/**
 * Analyzes an existing Dockerfile to extract structure and patterns
 */
function analyzeDockerfile(content: string): DockerfileAnalysis {
  const lines = content
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);

  // Extract base images (FROM instructions)
  const baseImages = lines
    .filter((line) => line.toUpperCase().startsWith('FROM '))
    .map((line) => line.substring(5).trim().split(' ')[0])
    .filter((image): image is string => Boolean(image));

  // Check for multi-stage build (multiple FROM statements)
  const isMultistage = baseImages.length > 1;

  // Check for HEALTHCHECK
  const hasHealthCheck = lines.some((line) => line.toUpperCase().startsWith('HEALTHCHECK '));

  // Check for non-root USER
  const hasNonRootUser = lines.some((line) => {
    const upper = line.toUpperCase();
    return (
      upper.startsWith('USER ') && !upper.startsWith('USER ROOT') && !upper.startsWith('USER 0')
    );
  });

  // Count total instructions (lines that start with Dockerfile keywords)
  const dockerfileKeywords = [
    'FROM',
    'RUN',
    'CMD',
    'LABEL',
    'EXPOSE',
    'ENV',
    'ADD',
    'COPY',
    'ENTRYPOINT',
    'VOLUME',
    'USER',
    'WORKDIR',
    'ARG',
    'ONBUILD',
    'STOPSIGNAL',
    'HEALTHCHECK',
    'SHELL',
  ];
  const instructionCount = lines.filter((line) => {
    const firstWord = line.split(/\s+/)[0];
    return firstWord && dockerfileKeywords.includes(firstWord.toUpperCase());
  }).length;

  // Determine complexity
  let complexity: 'simple' | 'moderate' | 'complex' = 'simple';
  if (instructionCount > 20 || isMultistage) {
    complexity = 'complex';
  } else if (instructionCount > 10) {
    complexity = 'moderate';
  }

  // Assess security posture
  let securityPosture: 'good' | 'needs-improvement' | 'poor' = 'needs-improvement';
  const hasRunAsRoot = !hasNonRootUser;
  const hasNoHealthCheck = !hasHealthCheck;

  if (!hasRunAsRoot && hasHealthCheck) {
    securityPosture = 'good';
  } else if (hasRunAsRoot && hasNoHealthCheck) {
    securityPosture = 'poor';
  }

  return {
    baseImages,
    isMultistage,
    hasHealthCheck,
    hasNonRootUser,
    instructionCount,
    complexity,
    securityPosture,
  };
}

/**
 * Generates enhancement guidance based on Dockerfile analysis and knowledge recommendations
 */
function generateEnhancementGuidance(
  analysis: DockerfileAnalysis,
  recommendations: {
    securityConsiderations: DockerfileRequirement[];
    optimizations: DockerfileRequirement[];
    bestPractices: DockerfileRequirement[];
  },
): EnhancementGuidance {
  const preserve: string[] = [];
  const improve: string[] = [];
  const addMissing: string[] = [];

  // Preserve good existing patterns
  if (analysis.isMultistage) {
    preserve.push('Multi-stage build structure');
  }
  if (analysis.hasHealthCheck) {
    preserve.push('HEALTHCHECK instruction');
  }
  if (analysis.hasNonRootUser) {
    preserve.push('Non-root USER configuration');
  }
  if (analysis.baseImages.length > 0) {
    preserve.push(`Existing base image selection (${analysis.baseImages.join(', ')})`);
  }

  // Identify improvements needed
  if (!analysis.hasNonRootUser) {
    improve.push('Add non-root USER for security');
    addMissing.push('Non-root user configuration');
  }
  if (!analysis.hasHealthCheck) {
    improve.push('Add HEALTHCHECK instruction');
    addMissing.push('Container health monitoring');
  }
  if (analysis.complexity === 'complex' && !analysis.isMultistage) {
    improve.push('Consider multi-stage build for optimization');
  }

  // Add security improvements from recommendations
  if (recommendations.securityConsiderations.length > 0 && analysis.securityPosture !== 'good') {
    improve.push('Apply security best practices from knowledge base');
  }

  // Add optimization opportunities
  if (recommendations.optimizations.length > 0) {
    improve.push('Apply layer caching and size optimization techniques');
  }

  // Determine strategy
  let strategy: 'minor-tweaks' | 'moderate-refactor' | 'major-overhaul' = 'minor-tweaks';
  const issueCount = improve.length + addMissing.length;

  if (analysis.securityPosture === 'poor' || issueCount > 5) {
    strategy = 'major-overhaul';
  } else if (analysis.securityPosture === 'needs-improvement' || issueCount > 2) {
    strategy = 'moderate-refactor';
  }

  // If nothing to improve, note that
  if (preserve.length > 0 && improve.length === 0 && addMissing.length === 0) {
    preserve.push('Well-structured Dockerfile - minimal changes needed');
  }

  return {
    preserve,
    improve,
    addMissing,
    strategy,
  };
}

interface DockerfileBuildRules {
  buildStrategy: {
    multistage: boolean;
    reason: string;
  };
}

/**
 * Regular expression to match Docker image names with optional registry/repository prefix and tag.
 * Matches format: [registry/][repository/]image:tag
 * Examples: node:20-alpine, gcr.io/distroless/nodejs, myregistry.com/my-app:1.0.0
 */
const DOCKER_IMAGE_NAME_REGEX = /\b([a-z0-9.-]+\/)?[a-z0-9.-]+:[a-z0-9._-]+\b/;

/**
 * Helper function to create base image recommendations from knowledge snippets
 */
function createBaseImageRecommendation(snippet: {
  id: string;
  text: string;
  category?: string;
  tags?: string[];
  weight: number;
}): BaseImageRecommendation {
  // Extract image name from the recommendation text
  const imageMatch = snippet.text.match(DOCKER_IMAGE_NAME_REGEX);
  const image = imageMatch ? imageMatch[0] : 'unknown';

  // Determine category based on tags and content
  let category: 'official' | 'distroless' | 'security' | 'size' = 'official';
  if (snippet.tags?.includes('distroless') || snippet.text.toLowerCase().includes('distroless')) {
    category = 'distroless';
  } else if (
    snippet.tags?.includes('security') ||
    snippet.tags?.includes('hardened') ||
    snippet.text.toLowerCase().includes('chainguard') ||
    snippet.text.toLowerCase().includes('wolfi')
  ) {
    category = 'security';
  } else if (
    snippet.tags?.includes('alpine') ||
    snippet.tags?.includes('slim') ||
    snippet.text.toLowerCase().includes('alpine') ||
    snippet.text.toLowerCase().includes('slim')
  ) {
    category = 'size';
  }

  // Extract size if mentioned (e.g., "50MB", "100 MB", "1GB")
  const sizeMatch = snippet.text.match(/(\d+)\s*(MB|GB|KB|B)/i);
  const size =
    sizeMatch?.[1] && sizeMatch[2] ? `${sizeMatch[1]}${sizeMatch[2].toUpperCase()}` : undefined;

  return {
    image,
    category,
    reason: snippet.text,
    size,
    tags: snippet.tags,
    matchScore: snippet.weight,
  };
}

const runPattern = createKnowledgeTool<
  ExtendedDockerfileParams,
  DockerfilePlan,
  DockerfileCategory,
  DockerfileBuildRules
>({
  name,
  query: {
    topic: TOPICS.DOCKERFILE,
    category: CATEGORY.DOCKERFILE,
    maxChars: 8000,
    maxSnippets: 20,
    extractFilters: (input) => ({
      environment: input.environment || 'production',
      language: input.language || 'auto-detect',
      framework: input.framework,
      detectedDependencies: input.detectedDependencies,
    }),
  },
  categorization: {
    categoryNames: ['baseImages', 'security', 'optimization', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<DockerfileCategory>({
      baseImages: (s) =>
        Boolean(
          s.tags?.includes('base-image') ||
            s.tags?.includes('registry') ||
            s.tags?.includes('official') ||
            s.tags?.includes('distroless') ||
            s.tags?.includes('alpine') ||
            s.tags?.includes('slim') ||
            s.text.toLowerCase().includes('from ') ||
            s.text.toLowerCase().includes('base image'),
        ),
      security: (s) => s.category === 'security' || Boolean(s.tags?.includes('security')),
      optimization: (s) =>
        Boolean(
          s.tags?.includes('optimization') ||
            s.tags?.includes('caching') ||
            s.tags?.includes('size'),
        ),
      bestPractices: () => true, // Catch remaining snippets as best practices
    }),
  },
  rules: {
    applyRules: (input) => {
      const language = input.language || 'auto-detect';
      const buildSystemType = undefined;

      const shouldUseMultistage =
        language === 'java' ||
        language === 'go' ||
        language === 'rust' ||
        language === 'dotnet' ||
        language === 'c#' ||
        (typeof buildSystemType === 'string' && ['maven', 'gradle'].includes(buildSystemType));

      return {
        buildStrategy: {
          multistage: shouldUseMultistage,
          reason: shouldUseMultistage
            ? 'Multi-stage build recommended to separate build tools from runtime, reducing image size by 70-90%'
            : 'Single-stage build sufficient for interpreted languages',
        },
      };
    },
  },
  plan: {
    buildPlan: (input, knowledge, rules, confidence) => {
      const path = input.repositoryPath || '';
      const modulePath = input.modulePath || path;
      const language = input.language || 'auto-detect';
      const framework = input.framework;

      // Access existing Dockerfile info from extended input (added in run function)
      // Type is already ExtendedDockerfileParams, so no assertion needed
      const existingDockerfile = input._existingDockerfile;

      const knowledgeMatches: DockerfileRequirement[] = knowledge.all.map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'generic',
        recommendation: snippet.text,
        ...(snippet.tags && { tags: snippet.tags }),
        matchScore: snippet.weight,
      }));

      // Extract base image recommendations from categorized knowledge
      const baseImageMatches: BaseImageRecommendation[] = (knowledge.categories.baseImages || [])
        .map((snippet) => createBaseImageRecommendation(snippet))
        .sort((a, b) => b.matchScore - a.matchScore); // Sort by match score descending

      const securityMatches: DockerfileRequirement[] = (knowledge.categories.security || []).map(
        (snippet) => ({
          id: snippet.id,
          category: snippet.category || 'security',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }),
      );

      const optimizationMatches: DockerfileRequirement[] = (
        knowledge.categories.optimization || []
      ).map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'optimization',
        recommendation: snippet.text,
        ...(snippet.tags && { tags: snippet.tags }),
        matchScore: snippet.weight,
      }));

      const bestPracticeMatches: DockerfileRequirement[] = (
        knowledge.categories.bestPractices || []
      )
        .filter((snippet) => {
          // Exclude snippets already in security or optimization
          const isInSecurity = (knowledge.categories.security || []).some(
            (s) => s.id === snippet.id,
          );
          const isInOptimization = (knowledge.categories.optimization || []).some(
            (s) => s.id === snippet.id,
          );
          return !isInSecurity && !isInOptimization;
        })
        .map((snippet) => ({
          id: snippet.id,
          category: snippet.category || 'generic',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }));

      // Build enhanced summary with existing Dockerfile info
      const summaryParts = [
        'Dockerfile Planning Summary:',
        `- Path: ${modulePath}${input.modulePath ? ' (module)' : ''}`,
        `- Language: ${language}${framework ? ` (${framework})` : ''}`,
        `- Environment: ${input.environment || 'production'}`,
        `- Build Strategy: ${rules.buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'}`,
      ];

      if (existingDockerfile) {
        const { analysis, guidance } = existingDockerfile;
        summaryParts.push(
          `- Mode: ENHANCE existing Dockerfile`,
          `- Existing Dockerfile: ${existingDockerfile.path}`,
          `- Analysis:`,
          `  - Complexity: ${analysis.complexity}`,
          `  - Security: ${analysis.securityPosture}`,
          `  - Multi-stage: ${analysis.isMultistage ? 'Yes' : 'No'}`,
          `  - Instructions: ${analysis.instructionCount}`,
          `- Enhancement Strategy: ${guidance.strategy}`,
          `- Preserve: ${guidance.preserve.length} items`,
          `- Improve: ${guidance.improve.length} items`,
          `- Add Missing: ${guidance.addMissing.length} items`,
        );
      } else {
        summaryParts.push('- Mode: CREATE new Dockerfile');
      }

      summaryParts.push(
        `- Knowledge Matches: ${knowledgeMatches.length} recommendations found`,
        `  - Base Images: ${baseImageMatches.length}`,
        `  - Security: ${securityMatches.length}`,
        `  - Optimizations: ${optimizationMatches.length}`,
        `  - Best Practices: ${bestPracticeMatches.length}`,
      );

      const summary = summaryParts.join('\n').trim();

      return {
        repositoryInfo: {
          name: modulePath.split('/').pop() || 'unknown',
          modulePath,
          ...(language &&
            language !== 'auto-detect' && {
              language: language === 'java' || language === 'dotnet' ? language : 'other',
            }),
          ...(framework &&
            framework !== 'auto-detect' && {
              frameworks: [{ name: framework }],
            }),
        },
        recommendations: {
          buildStrategy: rules.buildStrategy,
          baseImages: baseImageMatches,
          securityConsiderations: securityMatches,
          optimizations: optimizationMatches,
          bestPractices: bestPracticeMatches,
        },
        knowledgeMatches,
        confidence,
        summary,
        ...(existingDockerfile && {
          existingDockerfile: {
            path: existingDockerfile.path,
            content: existingDockerfile.content,
            analysis: existingDockerfile.analysis,
            guidance: existingDockerfile.guidance,
          },
        }),
      };
    },
  },
});

async function handleGenerateDockerfile(
  input: z.infer<typeof generateDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<DockerfilePlan>> {
  const path = input.repositoryPath || '';

  if (!path) {
    return Failure('Path is required. Provide a path parameter.');
  }

  // Validate repository path
  const pathResult = await validatePath(path, {
    mustExist: true,
    mustBeDirectory: true,
  });
  if (!pathResult.ok) {
    return pathResult;
  }

  // Check for existing Dockerfile in the repository path or module path
  const targetPath = input.modulePath || path;
  const dockerfilePath = nodePath.join(targetPath, 'Dockerfile');

  let existingDockerfile:
    | {
        path: string;
        content: string;
        analysis: DockerfileAnalysis;
        guidance: EnhancementGuidance;
      }
    | undefined;

  try {
    // Try to read the Dockerfile directly (no race condition with separate stat check)
    const content = await fs.readFile(dockerfilePath, 'utf-8');

    // Analyze the existing Dockerfile
    const analysis = analyzeDockerfile(content);

    // Generate preliminary guidance (will be refined with knowledge in buildPlan)
    const guidance = generateEnhancementGuidance(analysis, {
      securityConsiderations: [],
      optimizations: [],
      bestPractices: [],
    });

    existingDockerfile = {
      path: dockerfilePath,
      content,
      analysis,
      guidance,
    };

    ctx.logger.info(
      {
        path: dockerfilePath,
        size: content.length,
        complexity: analysis.complexity,
        security: analysis.securityPosture,
        strategy: guidance.strategy,
      },
      'Found existing Dockerfile - will enhance rather than create from scratch',
    );
  } catch (error) {
    // Dockerfile doesn't exist or can't be read - that's fine, we'll create a new one
    ctx.logger.info(
      { error, path: dockerfilePath },
      'No existing Dockerfile found - will create new one',
    );
  }

  // Add existing Dockerfile to input if found
  const extendedInput = {
    ...input,
    ...(existingDockerfile && { _existingDockerfile: existingDockerfile }),
  };

  return runPattern(extendedInput, ctx);
}

import { tool } from '@/types/tool';

export default tool({
  name,
  description,
  category: 'docker',
  version,
  schema: generateDockerfileSchema,
  metadata: {
    knowledgeEnhanced: true,
    enhancementCapabilities: ['recommendations'],
  },
  handler: handleGenerateDockerfile,
});
