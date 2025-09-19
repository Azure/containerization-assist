import { z } from 'zod';

export const resolveBaseImagesSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  technology: z.string().optional().describe('Technology stack to resolve'),
  requirements: z.record(z.unknown()).optional().describe('Requirements for base image'),
  targetEnvironment: z
    .enum(['development', 'staging', 'production', 'testing'])
    .optional()
    .describe('Target environment'),
});

export type ResolveBaseImagesParams = z.infer<typeof resolveBaseImagesSchema>;

/**
 * Output schema for resolve-base-images tool (AI-first)
 */
export const BaseImageOutputSchema = z.object({
  recommendations: z
    .array(
      z.object({
        image: z.string().describe('Full image name with tag (e.g., node:20-alpine)'),
        reason: z.string().describe('Explanation of why this image is recommended'),
        pros: z.array(z.string()).describe('Advantages of this image'),
        cons: z.array(z.string()).describe('Disadvantages or trade-offs'),
        size: z.string().optional().describe('Approximate compressed image size'),
        securityLevel: z.enum(['high', 'medium', 'low']).describe('Security assessment'),
        performanceLevel: z.enum(['high', 'medium', 'low']).describe('Performance characteristics'),
        compatibility: z
          .enum(['excellent', 'good', 'fair', 'limited'])
          .describe('Compatibility rating'),
        bestFor: z.array(z.string()).optional().describe('Best use cases for this image'),
      }),
    )
    .describe('List of recommended base images ranked by preference'),

  primary: z
    .object({
      image: z.string().describe('Top recommended image'),
      rationale: z.string().describe('Detailed explanation for the primary recommendation'),
      alternates: z.array(z.string()).describe('Alternative images if primary is not suitable'),
    })
    .describe('Primary recommendation with alternatives'),

  considerations: z
    .object({
      security: z.array(z.string()).optional().describe('Security considerations'),
      performance: z.array(z.string()).optional().describe('Performance considerations'),
      compatibility: z.array(z.string()).optional().describe('Compatibility notes'),
      licensing: z.array(z.string()).optional().describe('Licensing considerations'),
    })
    .optional()
    .describe('Important considerations for image selection'),

  metadata: z
    .object({
      language: z.string().optional().describe('Detected or specified language'),
      framework: z.string().optional().describe('Detected or specified framework'),
      environment: z.string().optional().describe('Target deployment environment'),
      analysisSource: z
        .enum(['session', 'params', 'defaults'])
        .optional()
        .describe('Source of analysis data'),
    })
    .optional()
    .describe('Metadata about the resolution process'),

  warnings: z.array(z.string()).optional().describe('Important warnings or caveats'),
});

export type BaseImageOutput = z.infer<typeof BaseImageOutputSchema>;
