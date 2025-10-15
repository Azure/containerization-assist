/**
 * Resolve Base Images Tool
 *
 * Queries the knowledge base to provide structured recommendations for optimal
 * Docker base images based on technology stack. Returns categorized recommendations
 * (official, distroless, security-hardened, size-optimized) without making AI calls.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 *
 * @category docker
 * @version 2.0.0
 * @knowledgeEnhanced true
 * @samplingStrategy none
 */

import { Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import {
  resolveBaseImagesSchema,
  type BaseImagePlan,
  type BaseImageRecommendation,
  type BaseImageRequirement,
  type ResolveBaseImagesParams,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';

const name = 'resolve-base-images';
const description =
  'Query knowledge base and return structured recommendations for Docker base images';
const version = '2.0.0';

// Define category types for better type safety
type BaseImageCategory = 'official' | 'distroless' | 'security' | 'size';

// Define rule results interface
interface BaseImageRules {
  recommendAlpine: boolean;
  recommendSlim: boolean;
  recommendDistroless: boolean;
  preferredImages: string[];
}

/**
 * Regular expression to match Docker image names with optional registry/repository prefix and tag.
 * Matches format: [registry/][repository/]image:tag
 * Examples: node:20-alpine, gcr.io/distroless/nodejs, myregistry.com/my-app:1.0.0
 */
const DOCKER_IMAGE_NAME_REGEX = /\b([a-z0-9.-]+\/)?[a-z0-9.-]+:[a-z0-9._-]+\b/;

// Helper function to create base image recommendations from knowledge snippets
function createRecommendation(
  snippet: { id: string; text: string; category?: string; tags?: string[]; weight: number },
  category: BaseImageCategory,
): BaseImageRecommendation {
  // Extract image name from the recommendation text (assumes format like "node:20-alpine" or mentions image)
  const imageMatch = snippet.text.match(DOCKER_IMAGE_NAME_REGEX);
  const image = imageMatch ? imageMatch[0] : 'unknown';

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

// Create the tool runner using the shared pattern
const runPattern = createKnowledgeTool<
  ResolveBaseImagesParams,
  BaseImagePlan,
  BaseImageCategory,
  BaseImageRules
>({
  name,
  query: {
    topic: TOPICS.RESOLVE_BASE_IMAGES,
    category: CATEGORY.DOCKERFILE,
    maxChars: 4000,
    maxSnippets: 15,
    extractFilters: (input) => ({
      environment: input.environment || 'production',
      language: input.technology,
      framework: input.framework,
    }),
  },
  categorization: {
    categoryNames: ['official', 'distroless', 'security', 'size'] as const,
    categorize: createSimpleCategorizer<BaseImageCategory>({
      official: (s) =>
        Boolean(
          s.tags?.includes('official') || s.category === 'official' || s.text.includes('official'),
        ),
      distroless: (s) =>
        Boolean(s.tags?.includes('distroless') || s.text.toLowerCase().includes('distroless')),
      security: (s) =>
        Boolean(
          s.tags?.includes('security') ||
            s.tags?.includes('hardened') ||
            s.text.toLowerCase().includes('chainguard') ||
            s.text.toLowerCase().includes('wolfi'),
        ),
      size: (s) =>
        Boolean(
          s.tags?.includes('alpine') ||
            s.tags?.includes('slim') ||
            s.text.toLowerCase().includes('alpine') ||
            s.text.toLowerCase().includes('slim'),
        ),
    }),
  },
  rules: {
    applyRules: (input) => {
      const language = input.technology.toLowerCase();

      // Deterministic rules based on language/technology
      const rules: BaseImageRules = {
        recommendAlpine: ['node', 'python', 'ruby', 'php'].includes(language),
        recommendSlim: ['python', 'node'].includes(language),
        recommendDistroless: ['java', 'go', 'rust'].includes(language),
        preferredImages: [],
      };

      // Build preferred images list based on language
      switch (language) {
        case 'node':
        case 'nodejs':
          rules.preferredImages = [
            `node:${input.languageVersion || '20'}-alpine`,
            `node:${input.languageVersion || '20'}-slim`,
            'gcr.io/distroless/nodejs',
          ];
          break;
        case 'python':
          rules.preferredImages = [
            `python:${input.languageVersion || '3.11'}-slim`,
            `python:${input.languageVersion || '3.11'}-alpine`,
            'gcr.io/distroless/python3',
          ];
          break;
        case 'java':
          rules.preferredImages = [
            `eclipse-temurin:${input.languageVersion || '21'}-jre-alpine`,
            `openjdk:${input.languageVersion || '21'}-jre-slim`,
            'gcr.io/distroless/java',
          ];
          break;
        case 'go':
        case 'golang':
          rules.preferredImages = [
            `golang:${input.languageVersion || '1.21'}-alpine`,
            'gcr.io/distroless/static',
            'gcr.io/distroless/base',
          ];
          break;
        case 'rust':
          rules.preferredImages = [
            `rust:${input.languageVersion || '1.74'}-alpine`,
            'gcr.io/distroless/cc',
            'gcr.io/distroless/static',
          ];
          break;
        case 'dotnet':
        case 'c#':
          rules.preferredImages = [
            `mcr.microsoft.com/dotnet/aspnet:${input.languageVersion || '8.0'}-alpine`,
            `mcr.microsoft.com/dotnet/runtime:${input.languageVersion || '8.0'}-alpine`,
          ];
          break;
        default:
          // Generic recommendations
          rules.preferredImages = ['alpine:latest', 'ubuntu:22.04'];
      }

      return rules;
    },
  },
  plan: {
    buildPlan: (input, knowledge, rules, confidence) => {
      // Map knowledge snippets to requirements
      const knowledgeMatches: BaseImageRequirement[] = knowledge.all.map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'generic',
        recommendation: snippet.text,
        tags: snippet.tags,
        matchScore: snippet.weight,
      }));

      // Create categorized recommendations
      const officialImages: BaseImageRecommendation[] = (knowledge.categories.official || []).map(
        (snippet) => createRecommendation(snippet, 'official'),
      );

      const distrolessOptions: BaseImageRecommendation[] = (
        knowledge.categories.distroless || []
      ).map((snippet) => createRecommendation(snippet, 'distroless'));

      const securityHardened: BaseImageRecommendation[] = (knowledge.categories.security || []).map(
        (snippet) => createRecommendation(snippet, 'security'),
      );

      const sizeOptimized: BaseImageRecommendation[] = (knowledge.categories.size || []).map(
        (snippet) => createRecommendation(snippet, 'size'),
      );

      // Add rule-based preferred images as recommendations if not already present
      for (const image of rules.preferredImages) {
        const category: BaseImageCategory = image.includes('alpine')
          ? 'size'
          : image.includes('distroless')
            ? 'distroless'
            : 'official';

        const recommendation: BaseImageRecommendation = {
          image,
          category,
          reason: `Recommended ${category} image for ${input.technology}`,
          matchScore: 1.0,
        };

        // Add to appropriate category if not duplicate
        switch (category) {
          case 'official':
            if (!officialImages.some((r) => r.image === image)) {
              officialImages.unshift(recommendation);
            }
            break;
          case 'distroless':
            if (!distrolessOptions.some((r) => r.image === image)) {
              distrolessOptions.unshift(recommendation);
            }
            break;
          case 'size':
            if (!sizeOptimized.some((r) => r.image === image)) {
              sizeOptimized.unshift(recommendation);
            }
            break;
        }
      }

      const summary = `
Base Image Planning Summary:
- Technology: ${input.technology}${input.languageVersion ? ` ${input.languageVersion}` : ''}${input.framework ? ` (${input.framework})` : ''}
- Environment: ${input.environment || 'production'}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Official Images: ${officialImages.length}
  - Distroless Options: ${distrolessOptions.length}
  - Security-Hardened: ${securityHardened.length}
  - Size-Optimized: ${sizeOptimized.length}
- Build Rules:
  - Recommend Alpine: ${rules.recommendAlpine ? 'Yes' : 'No'}
  - Recommend Slim: ${rules.recommendSlim ? 'Yes' : 'No'}
  - Recommend Distroless: ${rules.recommendDistroless ? 'Yes' : 'No'}
      `.trim();

      return {
        repositoryInfo: {
          language: input.technology,
          languageVersion: input.languageVersion,
          framework: input.framework,
          buildSystem: input.buildSystem,
        },
        recommendations: {
          officialImages,
          distrolessOptions,
          securityHardened,
          sizeOptimized,
        },
        knowledgeMatches,
        confidence,
        summary,
      };
    },
  },
});

// Wrapper function to add validation
async function run(
  input: z.infer<typeof resolveBaseImagesSchema>,
  ctx: ToolContext,
): Promise<Result<BaseImagePlan>> {
  if (!input.technology) {
    return Failure(
      'Technology is required. Provide a technology parameter (e.g., "node", "python").',
    );
  }

  return runPattern(input, ctx);
}

const tool: MCPTool<typeof resolveBaseImagesSchema, BaseImagePlan> = {
  name,
  description,
  category: 'docker',
  version,
  schema: resolveBaseImagesSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
