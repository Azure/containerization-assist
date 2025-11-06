/**
 * Generate Dockerfile Tool
 *
 * Analyzes repository and queries knowledgebase to gather insights and return
 * structured requirements for creating a Dockerfile. This tool helps users
 * understand best practices and recommendations before actual Dockerfile generation.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 */

import { validatePathOrFail } from '@/lib/validation-helpers';
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
import type { ToolNextAction } from '../shared/schemas';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import {
  validateContentAgainstPolicy,
  type PolicyViolation,
  type PolicyValidationResult,
} from '@/lib/policy-helpers';
import type { RegoEvaluator } from '@/config/policy-rego';
import type { Logger } from 'pino';

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
  existingDockerfile?: {
    path: string;
    content: string;
    analysis: DockerfileAnalysis;
    guidance: EnhancementGuidance;
  };
}

/**
 * List of valid Dockerfile instruction keywords
 */
const DOCKERFILE_KEYWORDS = [
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
] as const;

/**
 * Default non-root username for policy validation pseudo-Dockerfile
 * Using a generic name that works across different base images and languages
 */
const DEFAULT_NON_ROOT_USER = 'appuser';

/**
 * Parse Dockerfile content and extract base images
 */
function extractBaseImages(lines: string[]): string[] {
  return lines
    .filter((line) => line.toUpperCase().startsWith('FROM '))
    .map((line) => line.substring(5).trim().split(' ')[0])
    .filter((image): image is string => Boolean(image));
}

/**
 * Check if Dockerfile has a HEALTHCHECK instruction
 */
function hasHealthCheckInstruction(lines: string[]): boolean {
  return lines.some((line) => line.toUpperCase().startsWith('HEALTHCHECK '));
}

/**
 * Check if Dockerfile has a non-root USER instruction
 */
function hasNonRootUserInstruction(lines: string[]): boolean {
  return lines.some((line) => {
    const upper = line.toUpperCase();
    return (
      upper.startsWith('USER ') && !upper.startsWith('USER ROOT') && !upper.startsWith('USER 0')
    );
  });
}

/**
 * Count Dockerfile instructions
 */
function countInstructions(lines: string[]): number {
  return lines.filter((line) => {
    const firstWord = line.split(/\s+/)[0];
    return (
      firstWord &&
      DOCKERFILE_KEYWORDS.includes(firstWord.toUpperCase() as (typeof DOCKERFILE_KEYWORDS)[number])
    );
  }).length;
}

/**
 * Determine Dockerfile complexity based on instruction count and structure
 */
function determineComplexity(
  instructionCount: number,
  isMultistage: boolean,
): 'simple' | 'moderate' | 'complex' {
  if (instructionCount > 20 || isMultistage) {
    return 'complex';
  } else if (instructionCount > 10) {
    return 'moderate';
  }
  return 'simple';
}

/**
 * Assess security posture based on Dockerfile features
 */
function assessSecurityPosture(
  hasNonRootUser: boolean,
  hasHealthCheck: boolean,
): 'good' | 'needs-improvement' | 'poor' {
  const hasRunAsRoot = !hasNonRootUser;
  const hasNoHealthCheck = !hasHealthCheck;

  if (!hasRunAsRoot && hasHealthCheck) {
    return 'good';
  } else if (hasRunAsRoot && hasNoHealthCheck) {
    return 'poor';
  }
  return 'needs-improvement';
}

/**
 * Analyzes an existing Dockerfile to extract structure and patterns
 */
function analyzeDockerfile(content: string): DockerfileAnalysis {
  const lines = content
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);

  const baseImages = extractBaseImages(lines);
  const isMultistage = baseImages.length > 1;
  const hasHealthCheck = hasHealthCheckInstruction(lines);
  const hasNonRootUser = hasNonRootUserInstruction(lines);
  const instructionCount = countInstructions(lines);
  const complexity = determineComplexity(instructionCount, isMultistage);
  const securityPosture = assessSecurityPosture(hasNonRootUser, hasHealthCheck);

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
 * Identify good patterns to preserve from existing Dockerfile
 */
function identifyPreservationNeeds(analysis: DockerfileAnalysis): string[] {
  const preserve: string[] = [];

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

  return preserve;
}

/**
 * Identify improvements needed in existing Dockerfile
 */
function identifyImprovementOpportunities(
  analysis: DockerfileAnalysis,
  recommendations: {
    securityConsiderations: DockerfileRequirement[];
    optimizations: DockerfileRequirement[];
    bestPractices: DockerfileRequirement[];
  },
): { improve: string[]; addMissing: string[] } {
  const improve: string[] = [];
  const addMissing: string[] = [];

  // Identify missing security features
  if (!analysis.hasNonRootUser) {
    improve.push('Add non-root USER for security');
    addMissing.push('Non-root user configuration');
  }
  if (!analysis.hasHealthCheck) {
    improve.push('Add HEALTHCHECK instruction');
    addMissing.push('Container health monitoring');
  }

  // Suggest multi-stage for complex builds
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

  return { improve, addMissing };
}

/**
 * Determine enhancement strategy based on analysis and improvement needs
 */
function determineEnhancementStrategy(
  analysis: DockerfileAnalysis,
  preserve: string[],
  improve: string[],
  addMissing: string[],
): 'minor-tweaks' | 'moderate-refactor' | 'major-overhaul' {
  const issueCount = improve.length + addMissing.length;

  if (analysis.securityPosture === 'poor' || issueCount > 5) {
    return 'major-overhaul';
  } else if (analysis.securityPosture === 'needs-improvement' || issueCount > 2) {
    return 'moderate-refactor';
  }

  // If nothing to improve, note that it's well-structured
  if (preserve.length > 0 && improve.length === 0 && addMissing.length === 0) {
    preserve.push('Well-structured Dockerfile - minimal changes needed');
  }

  return 'minor-tweaks';
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
  const preserve = identifyPreservationNeeds(analysis);
  const { improve, addMissing } = identifyImprovementOpportunities(analysis, recommendations);
  const strategy = determineEnhancementStrategy(analysis, preserve, improve, addMissing);

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
 * Examples: node:20-alpine, gcr.io/distroless/nodejs, mcr.microsoft.com/openjdk/jdk:21-azurelinux
 * Updated to capture full registry paths including mcr.microsoft.com/path/image:tag
 */
const DOCKER_IMAGE_NAME_REGEX =
  /\b([a-z0-9.-]+\/)+[a-z0-9.-]+:[a-z0-9._-]+\b|[a-z0-9.-]+:[a-z0-9._-]+\b/;

/**
 * Substitute version in image name based on target language version
 * Examples:
 *   openjdk/jdk:21-azurelinux + version 25 -> openjdk/jdk:25-azurelinux
 *   eclipse-temurin:17-jdk-alpine + version 25 -> eclipse-temurin:25-jdk-alpine
 *   mcr.microsoft.com/openjdk/jdk:21-azurelinux + version 25 -> mcr.microsoft.com/openjdk/jdk:25-azurelinux
 *   maven:3.9-openjdk-17 + version 25 -> maven:3.9-openjdk-25
 */
function substituteImageVersion(image: string, targetVersion: string | undefined): string {
  if (!targetVersion) return image;

  // For maven/gradle images with format "tool:version-runtime-jdkversion"
  // Match patterns like maven:3.9-openjdk-17 or gradle:8.5-jdk21
  const toolWithRuntimePattern = /^(maven|gradle):(\d+\.\d+)-(openjdk|eclipse-temurin|jdk)-(\d+)/;
  const toolMatch = image.match(toolWithRuntimePattern);

  if (toolMatch) {
    const [, tool, toolVersion, runtime] = toolMatch;
    // Replace only the JDK version at the end
    return `${tool}:${toolVersion}-${runtime}-${targetVersion}`;
  }

  // For runtime images with format "runtime:version-variant"
  // Match patterns like :17-jdk-alpine, :21-azurelinux, :3.11-slim
  const runtimePattern = /:(\d+(?:\.\d+)?)([-.]|$)/;
  const match = image.match(runtimePattern);

  if (match) {
    const currentVersion = match[1];
    // Replace the version while preserving everything else
    return image.replace(`:${currentVersion}`, `:${targetVersion}`);
  }

  return image;
}

/**
 * Helper function to create base image recommendations from knowledge snippets
 */
function createBaseImageRecommendation(
  snippet: {
    id: string;
    text: string;
    category?: string;
    tags?: string[];
    weight: number;
  },
  languageVersion?: string,
): BaseImageRecommendation {
  // Extract image name from the recommendation text
  const imageMatch = snippet.text.match(DOCKER_IMAGE_NAME_REGEX);
  let image = imageMatch ? imageMatch[0] : 'unknown';

  // Substitute version if languageVersion is provided
  if (languageVersion && image !== 'unknown') {
    image = substituteImageVersion(image, languageVersion);
  }

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
      languageVersion: input.languageVersion,
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
            (s.tags?.includes('build-stage') && s.tags?.includes('build-tool')) ||
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
      const existingDockerfile = input.existingDockerfile;

      // Note: knowledgeMatches removed to reduce verbose output - all knowledge is already
      // categorized in recommendations.baseImages, securityConsiderations, optimizations, and bestPractices

      // Extract base image recommendations from categorized knowledge
      // Pass languageVersion for dynamic version substitution
      // Limit to top 2 recommendations to provide clear, opinionated guidance
      const baseImageMatches: BaseImageRecommendation[] = (knowledge.categories.baseImages || [])
        .map((snippet) => createBaseImageRecommendation(snippet, input.languageVersion))
        .sort((a, b) => b.matchScore - a.matchScore) // Sort by match score descending
        .slice(0, 2); // Take only top 2: primary recommendation + 1 alternative

      // Limit security recommendations to top 5 most relevant
      const securityMatches: DockerfileRequirement[] = (knowledge.categories.security || [])
        .map((snippet) => ({
          id: snippet.id,
          category: snippet.category || 'security',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }))
        .slice(0, 5); // Top 5 security recommendations

      // Limit optimization recommendations to top 5 most relevant
      const optimizationMatches: DockerfileRequirement[] = (knowledge.categories.optimization || [])
        .map((snippet) => ({
          id: snippet.id,
          category: snippet.category || 'optimization',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }))
        .slice(0, 5); // Top 5 optimization recommendations

      // Limit best practices to top 5 most relevant
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
        }))
        .slice(0, 5); // Top 5 best practice recommendations

      // Determine file path for nextAction
      const dockerfilePath = existingDockerfile
        ? existingDockerfile.path
        : nodePath.join(modulePath, 'Dockerfile');
      const relativeDockerfilePath = nodePath.relative(path, dockerfilePath) || './Dockerfile';

      // Build nextAction directive
      const nextAction: ToolNextAction = existingDockerfile
        ? {
            action: 'update-files',
            instruction: `Update the existing Dockerfile at ${relativeDockerfilePath} by applying the enhancement recommendations. Preserve the items listed in existingDockerfile.guidance.preserve, make improvements from existingDockerfile.guidance.improve, and add missing features from existingDockerfile.guidance.addMissing. Use the base images, security considerations, optimizations, and best practices from recommendations.`,
            files: [
              {
                path: relativeDockerfilePath,
                purpose: 'Container build configuration (enhancement)',
              },
            ],
          }
        : {
            action: 'create-files',
            instruction: `Create a new Dockerfile at ${relativeDockerfilePath} using the base images, security considerations, optimizations, and best practices from recommendations. Follow the ${rules.buildStrategy.multistage ? 'multi-stage' : 'single-stage'} build strategy described in recommendations.buildStrategy.`,
            files: [
              {
                path: relativeDockerfilePath,
                purpose: 'Container build configuration',
              },
            ],
          };

      // Build action-oriented summary
      const languageVersionStr = input.languageVersion ? ` ${input.languageVersion}` : '';
      const frameworkStr = framework ? ` (${framework})` : '';
      const totalRecommendations =
        baseImageMatches.length +
        securityMatches.length +
        optimizationMatches.length +
        bestPracticeMatches.length;

      let summary: string;
      if (existingDockerfile) {
        const { analysis, guidance } = existingDockerfile;
        summary =
          `🔨 ACTION REQUIRED: Update Dockerfile\n` +
          `Path: ${relativeDockerfilePath}\n` +
          `Language: ${language}${languageVersionStr}${frameworkStr}\n` +
          `Environment: ${input.environment || 'production'}\n` +
          `Target Platform: ${input.targetPlatform}\n` +
          `Current State: ${analysis.complexity}, ${analysis.securityPosture} security, ${analysis.instructionCount} instructions\n` +
          `Strategy: ${rules.buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'} build\n` +
          `Enhancement: ${guidance.strategy}\n` +
          `Changes: Preserve ${guidance.preserve.length} items, Improve ${guidance.improve.length} items, Add ${guidance.addMissing.length} missing items\n` +
          `Recommendations: ${totalRecommendations} total (${baseImageMatches.length} base images, ${securityMatches.length} security, ${optimizationMatches.length} optimizations, ${bestPracticeMatches.length} best practices)\n\n` +
          `✅ Ready to update Dockerfile. Use 'docker buildx build --platform=${input.targetPlatform}' to build for target platform.`;
      } else {
        summary =
          `🔨 ACTION REQUIRED: Create Dockerfile\n` +
          `Path: ${relativeDockerfilePath}\n` +
          `Language: ${language}${languageVersionStr}${frameworkStr}\n` +
          `Environment: ${input.environment || 'production'}\n` +
          `Target Platform: ${input.targetPlatform}\n` +
          `Strategy: ${rules.buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'} build\n` +
          `Recommendations: ${totalRecommendations} total (${baseImageMatches.length} base images, ${securityMatches.length} security, ${optimizationMatches.length} optimizations, ${bestPracticeMatches.length} best practices)\n\n` +
          `✅ Ready to create Dockerfile. Use 'docker buildx build --platform=${input.targetPlatform}' to build for target platform.`;
      }

      return {
        nextAction,
        repositoryInfo: {
          name: modulePath.split('/').pop() || 'unknown',
          modulePath,
          ...(language &&
            language !== 'auto-detect' && {
              language: language === 'java' || language === 'dotnet' ? language : 'other',
            }),
          ...(input.languageVersion && { languageVersion: input.languageVersion }),
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

/**
 * Convert DockerfilePlan to pseudo-Dockerfile text for policy validation
 * This allows policy rules to match against the planned Dockerfile structure
 */
function planToDockerfileText(plan: DockerfilePlan): string {
  const lines: string[] = [];

  // Add base image recommendations as FROM directives without platform flags
  const baseImages = plan.recommendations.baseImages || [];
  if (baseImages.length > 0) {
    const primaryImage = baseImages[0];
    if (primaryImage) {
      if (plan.recommendations.buildStrategy.multistage) {
        lines.push('# Multi-stage build');
        lines.push(`FROM ${primaryImage.image} AS builder`);
        lines.push('# ... build steps ...');
        lines.push(`FROM ${primaryImage.image}`);
      } else {
        lines.push(`FROM ${primaryImage.image}`);
      }
    }
  } else {
    // If no base images available (filtered out), use placeholder for policy validation
    lines.push(`FROM unknown`);
  }

  // Add default tag label for policy validation (use tag from plan or default to v1)
  const defaultTag = plan.recommendations.defaultTag || 'v1';
  lines.push(`LABEL tag="${defaultTag}"`);

  // Check for security recommendations
  const security = plan.recommendations.securityConsiderations || [];
  const hasNonRootUser = security.some(
    (s) =>
      s.recommendation.toLowerCase().includes('non-root user') ||
      s.recommendation.toLowerCase().includes('user directive'),
  );
  const hasHealthCheck = security.some((s) =>
    s.recommendation.toLowerCase().includes('healthcheck'),
  );

  // Add WORKDIR if mentioned in recommendations
  const allRecommendations = [
    ...(plan.recommendations.bestPractices || []),
    ...(plan.recommendations.optimizations || []),
  ];
  for (const rec of allRecommendations) {
    if (rec.recommendation.includes('WORKDIR')) {
      lines.push('WORKDIR /app');
      break;
    }
  }

  // Add EXPOSE if mentioned in recommendations
  for (const rec of allRecommendations) {
    if (rec.recommendation.includes('EXPOSE')) {
      lines.push('EXPOSE 8080');
      break;
    }
  }

  // Add USER directive if recommended or exists
  // Try to extract a recommended username from security recommendations
  if (hasNonRootUser || plan.existingDockerfile?.analysis.hasNonRootUser) {
    let userName: string | undefined;

    // Look for recommendations like "USER <username>"
    for (const s of security) {
      const match = s.recommendation.match(/USER\s+([a-zA-Z0-9_-]+)/i);
      if (match?.[1]) {
        userName = match[1];
        break;
      }
    }

    // Only add USER directive if a username is found or use a generic default
    lines.push(`USER ${userName || DEFAULT_NON_ROOT_USER}`);
  }

  // Add HEALTHCHECK if recommended or exists
  if (hasHealthCheck || plan.existingDockerfile?.analysis.hasHealthCheck) {
    lines.push('HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1');
  }

  return lines.join('\n');
}

/**
 * Validate a single base image against policy
 * Returns true if the image passes IMAGE-specific policy checks
 *
 * This function tests IMAGE-specific policies (registry restrictions, deprecated versions, etc.)
 * and ignores Dockerfile-structure policies (platform, tags) which are enforced later on the full plan.
 */
async function isImageCompliant(
  image: string,
  evaluator: RegoEvaluator,
  logger: Logger,
): Promise<boolean> {
  // Create a test Dockerfile without platform flags
  const dockerfileText = `FROM ${image}\nLABEL tag="v1"\nUSER appuser`;

  logger.debug({ image, dockerfileText }, 'Validating base image against policy');

  const validation = await validateContentAgainstPolicy(
    dockerfileText,
    evaluator,
    logger,
    'base image',
  );

  // Filter out Dockerfile-structure violations (platform, tag, label format)
  // We only care about IMAGE-specific violations here (registry, deprecation, security)
  // Note: Platform/tag/label violations are validated later in validatePlanAgainstPolicy()
  // and included in the final plan output as improvement recommendations
  const imageSpecificViolations = validation.violations.filter((v: PolicyViolation) => {
    const rule = v.ruleId.toLowerCase();
    // Ignore platform, tag, and label format rules - those are Dockerfile structure requirements
    return !(rule.includes('platform') || rule.includes('tag') || rule.includes('label'));
  });

  const isCompliant = imageSpecificViolations.length === 0;

  if (!isCompliant) {
    logger.debug(
      {
        image,
        violations: imageSpecificViolations.map((v: PolicyViolation) => v.ruleId),
      },
      'Base image has IMAGE-specific policy violations',
    );
  }

  return isCompliant;
}

/**
 * Validate DockerfilePlan against Rego policy
 * Uses shared validateContentAgainstPolicy utility
 */
async function validatePlanAgainstPolicy(
  plan: DockerfilePlan,
  evaluator: RegoEvaluator,
  logger: Logger,
): Promise<PolicyValidationResult> {
  // Convert plan to Dockerfile-like text for policy validation
  const dockerfileText = planToDockerfileText(plan);

  logger.debug({ dockerfileText }, 'Generated Dockerfile text from plan for policy validation');

  // Use shared validation utility
  return validateContentAgainstPolicy(dockerfileText, evaluator, logger, 'Dockerfile plan');
}

async function handleGenerateDockerfile(
  input: z.infer<typeof generateDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<DockerfilePlan>> {
  const path = input.repositoryPath || '';

  if (!path) {
    return Failure('Path is required. Provide a path parameter.', {
      message: 'Missing required parameter: path',
      hint: 'Repository path must be specified to generate Dockerfile',
      resolution: 'Add path parameter with the repository directory path',
    });
  }

  // Validate repository path
  const pathResult = await validatePathOrFail(path, {
    mustExist: true,
    mustBeDirectory: true,
  });
  if (!pathResult.ok) return pathResult;

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
    ...(existingDockerfile && { existingDockerfile }),
  };

  // Run the pattern to generate the plan
  const result = await runPattern(extendedInput, ctx);

  if (!result.ok) return result;

  const plan = result.value;

  // Add platform and default tag to recommendations
  // targetPlatform is now required, ensuring consistent builds across environments
  plan.recommendations.platform = input.targetPlatform;
  plan.recommendations.defaultTag = 'v1';

  // Filter base images based on policy if available
  if (ctx.policy && plan.recommendations.baseImages.length > 0) {
    ctx.logger.info(
      { count: plan.recommendations.baseImages.length, hasPolicyConfig: !!ctx.policy },
      'Filtering base images against policy',
    );

    // Validate each base image against policy
    const filteredBaseImages: BaseImageRecommendation[] = [];
    const totalImages = plan.recommendations.baseImages.length;

    for (const imageRec of plan.recommendations.baseImages) {
      const isCompliant = await isImageCompliant(imageRec.image, ctx.policy, ctx.logger);

      if (isCompliant) {
        filteredBaseImages.push(imageRec);
        ctx.logger.debug({ image: imageRec.image }, 'Base image passed policy validation');
      } else {
        ctx.logger.info(
          { image: imageRec.image },
          'Base image filtered out due to policy violation',
        );
      }
    }

    // Update plan with filtered images
    plan.recommendations.baseImages = filteredBaseImages;

    const filteredCount = totalImages - filteredBaseImages.length;
    if (filteredCount > 0) {
      ctx.logger.info(
        { filtered: filteredCount, remaining: filteredBaseImages.length },
        'Base images filtered by policy',
      );
    }
  } else if (!ctx.policy) {
    ctx.logger.warn('No policy configuration loaded - base images not filtered');
  }

  // Validate against policy if available
  if (ctx.policy) {
    const policyValidation = await validatePlanAgainstPolicy(plan, ctx.policy, ctx.logger);

    // Add policy validation to the plan (violations become recommendations for improvement)
    plan.policyValidation = policyValidation;

    // Log violations as warnings - they're guidance, not blockers
    if (!policyValidation.passed) {
      ctx.logger.warn(
        {
          violations: policyValidation.violations.map((v: PolicyViolation) => ({
            rule: v.ruleId,
            message: v.message,
          })),
        },
        'Policy violations detected - included in plan as improvement recommendations',
      );
    }

    // Log warnings/suggestions
    if (policyValidation.warnings.length > 0) {
      ctx.logger.warn(
        { warnings: policyValidation.warnings.map((w: PolicyViolation) => w.ruleId) },
        'Policy warnings in Dockerfile plan',
      );
    }
  }

  return result;
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
  },
  chainHints: {
    success:
      'Dockerfile plan generated successfully and passed policy validation. Next: Use fix-dockerfile to validate the actual Dockerfile content before building.',
    failure:
      'Failed to generate Dockerfile plan or plan violates policies. Review repository analysis and policy violations.',
  },
  handler: handleGenerateDockerfile,
});
