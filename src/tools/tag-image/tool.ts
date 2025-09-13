/**
 * Tag Image Tool - Standardized Implementation
 *
 * Tags Docker images with version and registry information
 * Uses standardized helpers for consistency
 */

import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import type { ToolContext } from '../../mcp/context';
import { createDockerClient } from '../../lib/docker';
import { createTimer, createLogger } from '../../lib/logger';
import { Success, Failure, type Result } from '../../types';
import { tagImageSchema, type TagImageParams } from './schema';
import { z } from 'zod';
import {
  getSuccessProgression,
  getFailureProgression,
  formatFailureChainHint,
  type SessionContext,
} from '../../workflows/workflow-progression';
import { TOOL_NAMES } from '../../exports/tool-names.js';

// Define the result schema for type safety
const TagImageResultSchema = z.object({
  success: z.boolean(),
  sessionId: z.string(),
  tags: z.array(z.string()),
  imageId: z.string(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(tagImageSchema, TagImageResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastTaggedAt: z.date().optional(),
  tagsApplied: z.array(z.string()).default([]),
});

export interface TagImageResult {
  success: boolean;
  sessionId: string;
  tags: string[];
  imageId: string;
}

/**
 * Tag image implementation - direct execution without wrapper
 */
async function tagImageImpl(
  params: TagImageParams,
  context: ToolContext,
): Promise<Result<TagImageResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = context.logger || createLogger({ name: 'tag-image' });
  const timer = createTimer(logger, 'tag-image');

  try {
    const { tag } = params;

    if (!tag) {
      return Failure('Tag parameter is required');
    }

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('tag-image', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info({ sessionId, tag }, 'Starting image tagging');

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    const dockerClient = createDockerClient(logger);

    // Check for built image in session metadata or use provided imageId
    const buildResult = session.metadata?.build_result as
      | { imageId?: string; tags?: string[] }
      | undefined;
    const source = params.imageId || buildResult?.imageId;

    if (!source) {
      return Failure(
        'No image specified. Provide imageId parameter or ensure session has built image from build-image tool.',
      );
    }

    // Tag image using lib docker client
    // Parse repository and tag from the tag parameter
    const parts = tag.split(':');
    const repository = parts[0];
    const tagName = parts[1] || 'latest';

    if (!repository) {
      return Failure('Invalid tag format');
    }

    const tagResult = await dockerClient.tagImage(source, repository, tagName);
    if (!tagResult.ok) {
      return Failure(`Failed to tag image: ${tagResult.error ?? 'Unknown error'}`);
    }

    const tags = [tag];
    const result: TagImageResult = {
      success: true,
      sessionId,
      tags,
      imageId: source,
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastTaggedAt: new Date(),
        tagsApplied: tags,
      },
    });

    // Update session metadata for backward compatibility
    const sessionManager = context.sessionManager;
    if (sessionManager) {
      await sessionManager.update(sessionId, {
        metadata: {
          ...session.metadata,
          build_result: {
            ...(buildResult || {}),
            imageId: source,
            tags,
          },
        },
        completed_steps: [...(session.completed_steps || []), 'tag'],
      });
    }

    timer.end({ source, tag });
    logger.info({ source, tag }, 'Image tagging completed');

    // Prepare session context for dynamic chain hints
    const sessionContext: SessionContext = {
      completed_steps: session.completed_steps || [],
      ...((session as SessionContext).analysis_result && {
        analysis_result: (session as SessionContext).analysis_result,
      }),
    };

    return Success({
      ...result,
      NextStep: getSuccessProgression(TOOL_NAMES.TAG_IMAGE, sessionContext).summary,
    });
  } catch (error) {
    timer.end({ error });
    logger.error({ error }, 'Image tagging failed');

    // Add failure chain hint - use basic context since session may not be available
    const sessionContext = {
      completed_steps: [],
    };
    const errorMessage = error instanceof Error ? error.message : String(error);
    const progression = getFailureProgression(TOOL_NAMES.TAG_IMAGE, errorMessage, sessionContext);
    const chainHint = formatFailureChainHint(TOOL_NAMES.TAG_IMAGE, progression);

    return Failure(`${errorMessage}\n${chainHint}`);
  }
}

/**
 * Tag image tool
 */
export const tagImage = tagImageImpl;
