/**
 * AI-powered base image resolution tool
 */

import { createPromptBackedTool } from '@mcp/tools/prompt-backed-tool';

import { BaseImageOutputSchema, resolveBaseImagesSchema } from './schema';

/**
 * AI-powered base image resolution tool that leverages:
 * - base-image-resolution.yaml template for intelligent recommendations
 * - Language and framework-specific knowledge packs
 * - Security and performance best practices
 * - Strict output validation with JSON repair
 *
 * This tool moves all heuristic logic to the AI:
 * - Language/framework detection and version matching
 * - Security level assessment (distroless, alpine, slim, etc.)
 * - Performance optimization (size vs functionality trade-offs)
 * - Environment-specific recommendations (dev vs production)
 * - Multi-stage build base image selection
 * - Compatibility analysis
 * - License considerations
 */
const resolveBaseImagesAI = createPromptBackedTool({
  name: 'resolve-base-images',
  description: 'Recommend optimal Docker base images using AI and knowledge packs',
  inputSchema: resolveBaseImagesSchema,
  outputSchema: BaseImageOutputSchema,
  promptId: 'base-image-resolution',
  knowledge: {
    category: 'dockerfile',
    textSelector: (params) => {
      const parts = [];

      // Technology/language is the primary selector
      if (params.technology) {
        parts.push(params.technology);
      }

      // Requirements provide additional context
      if (params.requirements) {
        const req = params.requirements;
        if (req.language) parts.push(String(req.language));
        if (req.framework) parts.push(String(req.framework));
        if (req.environment) parts.push(`for ${String(req.environment)}`);
        if (req.security === 'high') parts.push('high-security');
        if (req.size === 'minimal') parts.push('minimal-size');
      }

      // Session context indicator
      if (params.sessionId) {
        parts.push('with session context');
      }

      return parts.join(' ') || 'general purpose application';
    },
    context: (params) => {
      const context: Record<string, string | undefined> = {
        technology: params.technology,
        targetEnvironment: params.targetEnvironment || 'production',
        sessionId: params.sessionId,
      };

      // Extract requirements if provided
      if (params.requirements) {
        const req = params.requirements;
        context.language = req.language ? String(req.language) : undefined;
        context.languageVersion = req.languageVersion ? String(req.languageVersion) : undefined;
        context.framework = req.framework ? String(req.framework) : undefined;
        context.frameworkVersion = req.frameworkVersion ? String(req.frameworkVersion) : undefined;
        context.environment = req.environment ? String(req.environment) : undefined;
        context.security = req.security ? String(req.security) : undefined;
        context.size = req.size ? String(req.size) : undefined;
        context.performance = req.performance ? String(req.performance) : undefined;
        context.compatibility = req.compatibility ? String(req.compatibility) : undefined;
        context.multiStage = req.multiStage ? String(req.multiStage) : undefined;
        context.specialRequirements = req.specialRequirements
          ? String(req.specialRequirements)
          : undefined;
      }

      return context;
    },
    limit: 10, // More entries for comprehensive base image recommendations
  },
  policy: {
    tool: 'resolve-base-images',
    extractor: (params) => ({
      technology: params.technology,
      targetEnvironment: params.targetEnvironment || 'production',
      language: params.requirements?.language ? String(params.requirements.language) : undefined,
      framework: params.requirements?.framework ? String(params.requirements.framework) : undefined,
      security: params.requirements?.security ? String(params.requirements.security) : undefined,
      size: params.requirements?.size ? String(params.requirements.size) : undefined,
      performance: params.requirements?.performance
        ? String(params.requirements.performance)
        : undefined,
      multiStage: params.requirements?.multiStage
        ? String(params.requirements.multiStage)
        : undefined,
    }),
  },
});

/**
 * Standard tool export for MCP server integration
 */
export const tool = {
  type: 'standard' as const,
  name: 'resolve-base-images',
  description: 'Recommend optimal Docker base images using AI and knowledge packs',
  inputSchema: resolveBaseImagesSchema,
  // Wrap execute to adapt the signature for router compatibility
  execute: async (params: unknown, context: any) => {
    // The prompt-backed tool expects (params, deps, context)
    // but router calls with (params, context), so we need to adjust
    const deps = { logger: context.logger };
    return resolveBaseImagesAI.execute(params, deps, context);
  },
};

// Export for backward compatibility
export const resolveBaseImages = resolveBaseImagesAI.execute;
