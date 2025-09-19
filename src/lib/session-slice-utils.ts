/**
 * Session Slice Utilities
 *
 * Provides typed wrapper utilities for tool-specific session slices
 */

import type { ToolContext } from '@/mcp/context';
import type { SessionManager } from './session';
import { Result, Success, Failure } from '@/types';
import { createLogger } from './logger';
import {
  SessionSlice,
  SessionSliceOperations,
  ToolSliceMetadata,
  hasToolSlices,
  createSliceMetadata,
} from './session-types';
import { extractErrorMessage } from './error-utils';

/**
 * Get session manager from context
 */
function getSessionManager(context?: ToolContext): SessionManager | null {
  if (context && typeof context === 'object' && 'sessionManager' in context) {
    const manager = context.sessionManager;
    if (manager && typeof manager === 'object') {
      return manager;
    }
  }
  return null;
}

/**
 * Create session slice operations for a tool
 *
 * @param toolName - Name of the tool (used as namespace in session)
 * @param context - Tool context containing session manager
 * @returns Session slice operations or null if no session manager
 */
export function useSessionSlice(
  toolName: string,
  context?: ToolContext,
): SessionSliceOperations | null {
  const sessionManager = getSessionManager(context);
  if (!sessionManager) {
    return null;
  }

  return {
    async get(sessionId: string): Promise<SessionSlice | null> {
      try {
        const sessionResult = await sessionManager.get(sessionId);
        if (!sessionResult.ok || !sessionResult.value) {
          return null;
        }
        const session = sessionResult.value;

        const metadata = session.metadata || {};
        if (!hasToolSlices(metadata)) {
          return null;
        }

        const slice = metadata.toolSlices[toolName];
        if (!slice) {
          return null;
        }

        // Validate the slice structure
        if (typeof slice !== 'object') {
          return null;
        }

        // Type assertion after validation
        return slice as SessionSlice;
      } catch (error) {
        // Log error but return null for graceful degradation
        const logger = createLogger({ name: 'session-slice-utils' });
        logger.error({ error }, `Failed to get session slice for ${toolName}`);
        return null;
      }
    },

    async set(sessionId: string, slice: SessionSlice): Promise<void> {
      const sessionResult = await sessionManager.get(sessionId);
      if (!sessionResult.ok || !sessionResult.value) {
        // Create new session if it doesn't exist
        await sessionManager.create(sessionId);
        // Now update with the slice data
        await sessionManager.update(sessionId, {
          metadata: {
            ...createSliceMetadata(),
            toolSlices: {
              [toolName]: { ...slice, updatedAt: new Date() },
            },
          },
        });
        return;
      }
      const session = sessionResult.value;

      // Update existing session
      const metadata = session.metadata || {};
      const sliceMetadata = hasToolSlices(metadata) ? metadata : createSliceMetadata();

      await sessionManager.update(sessionId, {
        ...session,
        metadata: {
          ...metadata,
          toolSlices: {
            ...sliceMetadata.toolSlices,
            [toolName]: { ...slice, updatedAt: new Date() },
          },
          lastAccessed: {
            ...('lastAccessed' in sliceMetadata
              ? (sliceMetadata as ToolSliceMetadata).lastAccessed
              : {}),
            [toolName]: new Date(),
          },
        },
      });
    },

    async patch(sessionId: string, partial: Partial<SessionSlice>): Promise<void> {
      const existing = await this.get(sessionId);

      if (!existing) {
        // If no existing slice and no input provided, cannot create
        if (!partial.input) {
          throw new Error('Cannot create new slice without input');
        }
        // Create a minimal slice with the partial data
        const newSlice: SessionSlice = {
          input: partial.input,
          state: partial.state || {},
          updatedAt: new Date(),
        };
        // Only add output if it's defined
        if (partial.output !== undefined) {
          newSlice.output = partial.output;
        }
        await this.set(sessionId, newSlice);
        return;
      }

      // Merge with existing slice
      const updated: SessionSlice = {
        ...existing,
        ...partial,
        updatedAt: new Date(),
      };
      await this.set(sessionId, updated);
    },

    async clear(sessionId: string): Promise<void> {
      const sessionResult = await sessionManager.get(sessionId);
      if (!sessionResult.ok || !sessionResult.value) {
        return;
      }
      const session = sessionResult.value;

      const metadata = session.metadata || {};
      if (!hasToolSlices(metadata)) {
        return;
      }

      const { [toolName]: _, ...remainingSlices } = metadata.toolSlices;

      await sessionManager.update(sessionId, {
        ...session,
        metadata: {
          ...metadata,
          toolSlices: remainingSlices,
        },
      });
    },
  };
}

/**
 * Get session slice with Result wrapper
 *
 * @param toolName - Name of the tool
 * @param sessionId - Session ID
 * @param context - Tool context
 * @returns Result with session slice or error
 */
export async function getSessionSlice(
  toolName: string,
  sessionId: string,
  context?: ToolContext,
): Promise<Result<SessionSlice | null>> {
  try {
    const ops = useSessionSlice(toolName, context);
    if (!ops) {
      return Failure('Session manager not available');
    }
    const slice = await ops.get(sessionId);
    return Success(slice);
  } catch (error) {
    return Failure(`Failed to get session slice: ${extractErrorMessage(error)}`);
  }
}

/**
 * Update session slice with Result wrapper
 *
 * @param toolName - Name of the tool
 * @param sessionId - Session ID
 * @param slice - Partial slice to update
 * @param context - Tool context
 * @returns Result indicating success or failure
 */
export async function updateSessionSlice(
  toolName: string,
  sessionId: string,
  slice: Partial<SessionSlice>,
  context?: ToolContext,
): Promise<Result<void>> {
  try {
    const ops = useSessionSlice(toolName, context);
    if (!ops) {
      return Failure('Session manager not available');
    }
    await ops.patch(sessionId, slice);
    return Success(undefined);
  } catch (error) {
    return Failure(`Failed to update session slice: ${extractErrorMessage(error)}`);
  }
}
