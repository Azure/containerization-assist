/**
 * Schema definition for build-image-context tool
 * Provides context and recommendations for Docker image builds
 */

import { z } from 'zod';
import { imageName, tags, buildArgs, platform } from '../shared/schemas';

/**
 * Input parameters for build context preparation
 */
export const buildImageSchema = z.object({
  path: z
    .string()
    .optional()
    .describe('Build context path (use forward slashes: /path/to/context)'),
  dockerfile: z.string().optional().describe('Dockerfile name (relative to context)'),
  dockerfilePath: z
    .string()
    .optional()
    .describe('Path to Dockerfile (use forward slashes: /path/to/Dockerfile)'),
  imageName: imageName.optional(),
  tags: tags.optional(),
  buildArgs: buildArgs.optional(),
  platform,
});

export type BuildImageParams = z.infer<typeof buildImageSchema>;

/**
 * Security warning with severity and remediation
 */
export interface SecurityWarning {
  /** Warning identifier */
  id: string;
  /** Severity level */
  severity: 'low' | 'medium' | 'high' | 'critical';
  /** Human-readable message */
  message: string;
  /** Line number in Dockerfile (if applicable) */
  line?: number;
  /** Recommended fix */
  remediation: string;
}

/**
 * BuildKit feature detection
 */
export interface BuildKitFeatures {
  /** Dockerfile uses --mount=type=cache */
  cacheMount: boolean;
  /** Dockerfile uses --mount=type=secret */
  secretMount: boolean;
  /** Dockerfile uses --mount=type=ssh */
  sshMount: boolean;
  /** Multi-stage build detected */
  multiStage: boolean;
  /** Number of build stages */
  stageCount: number;
  /** Uses COPY --from for multi-stage */
  copyFrom: boolean;
  /** Uses heredoc syntax (requires BuildKit) */
  heredoc: boolean;
}

/**
 * Structured command for agent execution
 */
export interface BuildCommand {
  /** Full command string ready to execute */
  command: string;
  /** Structured parts for programmatic use */
  parts: {
    executable: 'docker';
    subcommand: 'build' | 'buildx build';
    flags: string[];
    context: string;
  };
  /** Environment variables to set */
  environment: Record<string, string>;
}

/**
 * Result of build context preparation
 */
export interface BuildImageResult {
  /** Natural language summary */
  summary: string;

  /** Validated paths */
  context: {
    /** Absolute path to build context directory */
    buildContextPath: string;
    /** Absolute path to Dockerfile */
    dockerfilePath: string;
    /** Relative path for -f flag */
    dockerfileRelative: string;
    /** Whether .dockerignore exists */
    hasDockerignore: boolean;
  };

  /** Security analysis results */
  securityAnalysis: {
    /** Structured warnings */
    warnings: SecurityWarning[];
    /** Overall risk assessment */
    riskLevel: 'low' | 'medium' | 'high';
    /** Actionable recommendations */
    recommendations: string[];
  };

  /** Computed build configuration */
  buildConfig: {
    /** Final tags to apply (computed from imageName + tags) */
    finalTags: string[];
    /** Merged build arguments (defaults + user provided) */
    buildArgs: Record<string, string>;
    /** Target platform */
    platform: string;
  };

  /** BuildKit feature analysis */
  buildKitAnalysis: {
    /** Detected features */
    features: BuildKitFeatures;
    /** Whether BuildKit is recommended */
    recommended: boolean;
    /** Optimization recommendations */
    recommendations: string[];
  };

  /** Dockerfile content analysis */
  dockerfileAnalysis: {
    /** Base images used */
    baseImages: string[];
    /** Exposed ports */
    exposedPorts: number[];
    /** Final USER directive (if any) */
    finalUser?: string;
    /** Has HEALTHCHECK */
    hasHealthcheck: boolean;
    /** Estimated layer count */
    layerCount: number;
  };

  /** Agent execution instructions */
  nextAction: {
    action: 'execute-build';
    /** Pre-execution checklist */
    preChecks: string[];
    /** Primary build command */
    buildCommand: BuildCommand;
    /** Alternative command if primary fails */
    fallbackCommand?: BuildCommand;
    /** Post-build suggestions */
    postBuildSteps: string[];
  };
}
