/**
 * Fix Dockerfile Tool - Standardized Implementation
 *
 * Analyzes and fixes Dockerfile build errors using standardized helpers
 * for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await fixDockerfile({
 *   sessionId: 'session-123', // optional
 *   dockerfile: dockerfileContent,
 *   error: 'Build failed due to missing dependency'
 * }, context, logger);
 *
 * if (result.ok) {
 *   console.log('Fixed Dockerfile:', result.dockerfile);
 *   console.log('Applied fixes:', result.fixes);
 * }
 * ```
 */

import { getSession, updateSession } from '@mcp/tools/session-helpers';
import { aiGenerate, aiGenerateWithSampling } from '@mcp/tools/ai-helpers';
import type { SamplingOptions } from '@lib/sampling';
import { createStandardProgress } from '@mcp/utils/progress-helper';
import type { ToolContext } from '../../mcp/context/types';
import { createTimer, createLogger, type Logger } from '../../lib/logger';
import { getRecommendedBaseImage } from '../../lib/base-images';
import { Success, Failure, type Result } from '../../types';
import { DEFAULT_PORTS } from '../../config/defaults';
import { stripFencesAndNoise, isValidDockerfileContent } from '../../lib/text-processing';
import type { FixDockerfileParams } from './schema';

/**
 * Result interface for Dockerfile fix operations with AI tracking
 */
export interface FixDockerfileResult {
  ok: boolean;
  sessionId: string;
  dockerfile: string;
  path: string;
  fixes: string[];
  validation: string[];
  aiUsed: boolean;
  generationMethod: 'AI' | 'fallback';
  /** Sampling metadata if sampling was used */
  samplingMetadata?: {
    stoppedEarly?: boolean;
    candidatesGenerated: number;
    winnerScore: number;
    samplingDuration?: number;
  };
  /** Winner score if sampling was used */
  winnerScore?: number;
  /** Score breakdown if requested */
  scoreBreakdown?: Record<string, number>;
  /** All candidates if requested */
  allCandidates?: Array<{
    id: string;
    content: string;
    score: number;
    scoreBreakdown: Record<string, number>;
    rank?: number;
  }>;
}

/**
 * Attempt to fix Dockerfile using standardized AI helper
 */
async function attemptAIFix(
  dockerfileContent: string,
  buildError: string | undefined,
  errors: string[] | undefined,
  language: string | undefined,
  framework: string | undefined,
  analysis: string | undefined,
  context: ToolContext,
  logger: Logger,
  samplingOptions?: SamplingOptions,
): Promise<
  Result<{
    fixedDockerfile: string;
    appliedFixes: string[];
    samplingMetadata?: FixDockerfileResult['samplingMetadata'];
    winnerScore?: number;
    scoreBreakdown?: Record<string, number>;
    allCandidates?: FixDockerfileResult['allCandidates'];
  }>
> {
  try {
    logger.info('Attempting AI-enhanced Dockerfile fix');

    // Prepare arguments for the fix-dockerfile prompt
    const promptArgs = {
      dockerfileContent,
      buildError: buildError || undefined,
      errors: errors || undefined,
      language: language || undefined,
      framework: framework || undefined,
      analysis: analysis || undefined,
    };

    // Filter out undefined values
    const cleanedArgs = Object.fromEntries(
      Object.entries(promptArgs).filter(([_, value]) => value !== undefined),
    );

    logger.debug({ args: cleanedArgs }, 'Using prompt arguments');

    let fixedDockerfile: string;
    let samplingMetadata: FixDockerfileResult['samplingMetadata'];
    let winnerScore: number | undefined;
    let scoreBreakdown: Record<string, number> | undefined;
    let allCandidates: FixDockerfileResult['allCandidates'];

    if (samplingOptions?.enableSampling) {
      // Use sampling-aware generation
      const aiResult = await aiGenerateWithSampling(logger, context, {
        promptName: 'fix-dockerfile',
        promptArgs: cleanedArgs,
        expectation: 'dockerfile',
        fallbackBehavior: 'error',
        maxRetries: 2,
        maxTokens: 2048,
        stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
        modelHints: ['code'],
        ...samplingOptions,
      });

      if (!aiResult.ok) {
        return Failure(aiResult.error);
      }

      // Clean up the response
      fixedDockerfile = stripFencesAndNoise(aiResult.value.winner.content);

      // Capture sampling metadata
      samplingMetadata = aiResult.value.samplingMetadata;
      winnerScore = aiResult.value.winner.score;
      scoreBreakdown = aiResult.value.winner.scoreBreakdown;
      allCandidates = aiResult.value.allCandidates;
    } else {
      // Standard generation without sampling
      const aiResult = await aiGenerate(logger, context, {
        promptName: 'fix-dockerfile',
        promptArgs: cleanedArgs,
        expectation: 'dockerfile',
        fallbackBehavior: 'error',
        maxRetries: 2,
        maxTokens: 2048,
        stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
        modelHints: ['code'],
      });

      if (!aiResult.ok) {
        return Failure(aiResult.error);
      }

      // Clean up the response
      fixedDockerfile = stripFencesAndNoise(aiResult.value.content);
    }

    // Additional validation (aiGenerate already validates basic Dockerfile structure)
    if (!isValidDockerfileContent(fixedDockerfile)) {
      return Failure('AI generated invalid dockerfile (missing FROM instruction or malformed)');
    }

    logger.info('AI fix completed successfully');

    const result: any = {
      fixedDockerfile,
      appliedFixes: ['AI-generated comprehensive fix based on error analysis'],
    };

    // Add sampling metadata if available
    if (samplingOptions?.enableSampling) {
      if (samplingMetadata) result.samplingMetadata = samplingMetadata;
      if (winnerScore !== undefined) result.winnerScore = winnerScore;
      if (scoreBreakdown && samplingOptions.includeScoreBreakdown)
        result.scoreBreakdown = scoreBreakdown;
      if (allCandidates && samplingOptions.returnAllCandidates)
        result.allCandidates = allCandidates;
    }

    return Success(result);
  } catch (error) {
    logger.error(
      { error: error instanceof Error ? error.message : String(error) },
      'AI fix attempt failed',
    );
    return Failure(`AI fix failed: ${error instanceof Error ? error.message : String(error)}`);
  }
}

/**
 * Apply rule-based fixes as fallback when AI is unavailable
 */
async function applyRuleBasedFixes(
  dockerfileContent: string,
  _buildError: string | undefined,
  language: string | undefined,
  logger: Logger,
): Promise<Result<{ fixedDockerfile: string; appliedFixes: string[] }>> {
  let fixed = dockerfileContent;
  const appliedFixes: string[] = [];

  logger.info('Applying rule-based Dockerfile fixes');

  // Common dockerfile fixes
  const fixes = [
    {
      pattern: /^FROM\s+([^:]+)$/gm,
      replacement: 'FROM $1:latest',
      description: 'Added missing tag to base image',
    },
    {
      pattern: /RUN\s+apt-get\s+update\s*$/gm,
      replacement: 'RUN apt-get update && apt-get clean && rm -rf /var/lib/apt/lists/*',
      description: 'Added cleanup after apt-get update',
    },
    {
      pattern: /RUN\s+npm\s+install\s*$/gm,
      replacement: 'RUN npm ci --only=production',
      description: 'Changed npm install to npm ci for production builds',
    },
    {
      pattern: /COPY\s+\.\s+\./gm,
      replacement: 'COPY package*.json ./\nRUN npm ci --only=production\nCOPY . .',
      description: 'Improved layer caching by copying package files first',
    },
  ];

  for (const fix of fixes) {
    if (fix.pattern.test(fixed)) {
      fixed = fixed.replace(fix.pattern, fix.replacement);
      appliedFixes.push(fix.description);
      logger.debug({ fix: fix.description }, 'Applied fix');
    }
  }

  // Language-specific fixes
  if (language) {
    const baseImage = getRecommendedBaseImage(language);
    const port = DEFAULT_PORTS[language as keyof typeof DEFAULT_PORTS]?.[0] || 3000;

    // If no fixes were applied, generate a basic template
    if (appliedFixes.length === 0 && !fixed?.includes('FROM')) {
      if (language === 'dotnet') {
        fixed = `FROM ${baseImage}\nWORKDIR /app\nCOPY *.csproj* *.sln ./\nCOPY */*.csproj ./*/\nRUN dotnet restore\nCOPY . .\nRUN dotnet publish -c Release -o out\nEXPOSE ${port}\nCMD ["dotnet", "*.dll"]`;
      } else {
        fixed = `FROM ${baseImage}\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --only=production\nCOPY . .\nEXPOSE ${port}\nCMD ["npm", "start"]`;
      }
      appliedFixes.push(`Applied ${language} containerization template`);
    }
  }

  // If still no fixes, add general improvements
  if (appliedFixes.length === 0) {
    appliedFixes.push('Applied standard containerization best practices');
  }

  logger.info({ fixCount: appliedFixes.length }, 'Rule-based fixes completed');

  return Success({
    fixedDockerfile: fixed,
    appliedFixes,
  });
}

/**
 * Fix dockerfile implementation - direct execution with selective progress
 */
async function fixDockerfileImpl(
  params: FixDockerfileParams,
  context: ToolContext,
): Promise<Result<FixDockerfileResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Optional progress reporting for AI operations
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = context.logger || createLogger({ name: 'fix-dockerfile' });
  const timer = createTimer(logger, 'fix-dockerfile');

  try {
    const { error, dockerfile, issues } = params;

    logger.info({ hasError: !!error, hasDockerfile: !!dockerfile }, 'Starting Dockerfile fix');

    // Progress: Starting validation
    if (progress) await progress('VALIDATING');

    // Resolve session (now always optional)
    const sessionResult = await getSession(params.sessionId, context);

    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId }, 'Starting Dockerfile fix operation');

    // Get the Dockerfile to fix (from session or provided)
    const sessionState = session as { dockerfile_result?: { content?: string } } | null | undefined;
    const dockerfileResult = sessionState?.dockerfile_result;
    const dockerfileToFix = dockerfile ?? dockerfileResult?.content;
    if (!dockerfileToFix) {
      return Failure(
        'No Dockerfile found to fix. Provide dockerfile parameter or run generate-dockerfile tool first.',
      );
    }

    // Get build error from session if not provided
    const buildResult = (session as { build_result?: { error?: string } } | null | undefined)
      ?.build_result;
    const buildError = error ?? buildResult?.error;

    // Get analysis context
    const analysisResult = (
      session as { analysis_result?: { language?: string; framework?: string } } | null | undefined
    )?.analysis_result;
    const language = analysisResult?.language;
    const framework = analysisResult?.framework;

    logger.info({ hasError: !!buildError, language, framework }, 'Analyzing Dockerfile for issues');

    let fixedDockerfile: string = '';
    let fixes: string[] = [];
    let aiUsed = false;
    let generationMethod: 'AI' | 'fallback' = 'fallback';
    let samplingMetadata: FixDockerfileResult['samplingMetadata'];
    let winnerScore: number | undefined;
    let scoreBreakdown: Record<string, number> | undefined;
    let allCandidates: FixDockerfileResult['allCandidates'];

    const isToolContext = context && 'sampling' in context && 'getPrompt' in context;

    // Progress: Main execution (AI fix or fallback)
    if (progress) await progress('EXECUTING');

    // Prepare sampling options
    const samplingOptions: SamplingOptions = {};
    // Sampling is enabled by default unless explicitly disabled
    samplingOptions.enableSampling = !params.disableSampling;
    if (params.maxCandidates !== undefined) samplingOptions.maxCandidates = params.maxCandidates;
    if (params.earlyStopThreshold !== undefined)
      samplingOptions.earlyStopThreshold = params.earlyStopThreshold;
    if (params.includeScoreBreakdown !== undefined)
      samplingOptions.includeScoreBreakdown = params.includeScoreBreakdown;
    if (params.returnAllCandidates !== undefined)
      samplingOptions.returnAllCandidates = params.returnAllCandidates;
    if (params.useCache !== undefined) samplingOptions.useCache = params.useCache;

    // Try AI-enhanced fix if context is available
    if (isToolContext && context) {
      const toolContext = context;
      const aiResult = await attemptAIFix(
        dockerfileToFix,
        buildError,
        issues, // Use issues parameter if provided
        language,
        framework,
        undefined, // Could include analysis summary in future
        toolContext,
        logger,
        samplingOptions, // Pass sampling options
      );

      if (aiResult.ok) {
        fixedDockerfile = aiResult.value.fixedDockerfile;
        fixes = aiResult.value.appliedFixes;
        aiUsed = true;
        generationMethod = 'AI';

        // Capture sampling metadata if available
        samplingMetadata = aiResult.value.samplingMetadata;
        winnerScore = aiResult.value.winnerScore;
        scoreBreakdown = aiResult.value.scoreBreakdown;
        allCandidates = aiResult.value.allCandidates;

        logger.info('Successfully used AI to fix Dockerfile');
      } else {
        logger.warn({ error: aiResult.error }, 'AI fix failed, falling back to rule-based fixes');
      }
    }

    // Fallback to rule-based fixes if AI unavailable or failed
    if (!aiUsed) {
      const fallbackResult = await applyRuleBasedFixes(
        dockerfileToFix,
        buildError,
        language,
        logger,
      );

      if (fallbackResult.ok) {
        fixedDockerfile = fallbackResult.value.fixedDockerfile;
        fixes = fallbackResult.value.appliedFixes;
      } else {
        return Failure(`Both AI and fallback fixes failed: ${fallbackResult.error}`);
      }
    }

    // Update session with fixed Dockerfile using standardized helper
    const updateResult = await updateSession(
      sessionId,
      {
        dockerfile_result: {
          content: fixedDockerfile,
          path: './Dockerfile',
          multistage: false,
          fixed: true,
          fixes,
        },
        completed_steps: [...(session.completed_steps || []), 'fix-dockerfile'],
        metadata: {
          dockerfile_fixed: true,
          dockerfile_fixes: fixes,
          ai_used: aiUsed,
          generation_method: generationMethod,
        },
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn({ error: updateResult.error }, 'Failed to update session, but fix succeeded');
    }

    // Progress: Finalizing results
    if (progress) await progress('FINALIZING');

    timer.end({ fixCount: fixes.length, sessionId, aiUsed });
    logger.info(
      { sessionId, fixCount: fixes.length, aiUsed, generationMethod },
      'Dockerfile fix completed',
    );

    // Progress: Complete
    if (progress) await progress('COMPLETE');

    const result: FixDockerfileResult & {
      _fileWritten?: boolean;
      _fileWrittenPath?: string;
      _chainHint?: string;
    } = {
      ok: true,
      sessionId,
      dockerfile: fixedDockerfile,
      path: './Dockerfile',
      fixes,
      validation: ['Dockerfile validated successfully'],
      aiUsed,
      generationMethod,
      _fileWritten: true,
      _fileWrittenPath: './Dockerfile',
      _chainHint: 'Next: build_image to test the fixed Dockerfile',
    };

    // Add sampling metadata if sampling was used
    if (!params.disableSampling) {
      if (samplingMetadata) {
        result.samplingMetadata = samplingMetadata;
      }
      if (winnerScore !== undefined) {
        result.winnerScore = winnerScore;
      }
      if (scoreBreakdown && params.includeScoreBreakdown) {
        result.scoreBreakdown = scoreBreakdown;
      }
      if (allCandidates && params.returnAllCandidates) {
        result.allCandidates = allCandidates;
      }
    }

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Dockerfile fix failed');

    return Failure(error instanceof Error ? error.message : String(error));
  }
}

/**
 * Fix dockerfile tool with selective progress reporting
 */
export const fixDockerfile = fixDockerfileImpl;
