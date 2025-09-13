/**
 * Session Slice Utilities
 *
 * Provides typed wrapper utilities for tool-specific session slices
 */

import { z } from 'zod';
import type { ToolContext } from '@mcp/context';
import type { SessionManager } from './session';
import { Result, Success, Failure } from '../types';
import {
  SessionSlice,
  SessionSliceOperations,
  ToolIO,
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
 * Create typed session slice operations for a tool
 *
 * @param toolName - Name of the tool (used as namespace in session)
 * @param io - Tool input/output schemas
 * @param stateSchema - Optional schema for tool-specific state
 * @param context - Tool context containing session manager
 * @returns Session slice operations or null if no session manager
 */
export function useSessionSlice<In, Out, State = Record<string, unknown>>(
  toolName: string,
  _io: ToolIO<In, Out>,
  context?: ToolContext,
  _stateSchema?: z.ZodType<State>,
): SessionSliceOperations<In, Out, State> | null {
  const sessionManager = getSessionManager(context);
  if (!sessionManager) {
    return null;
  }

  return {
    async get(sessionId: string): Promise<SessionSlice<In, Out, State> | null> {
      try {
        const session = await sessionManager.get(sessionId);
        if (!session) {
          return null;
        }

        const metadata = session.metadata || {};
        if (!hasToolSlices(metadata)) {
          return null;
        }

        const slice = metadata.toolSlices[toolName];
        if (!slice) {
          return null;
        }

        // Validate the slice structure
        if (typeof slice !== 'object' || slice === null) {
          return null;
        }

        // Type assertion after validation
        return slice as SessionSlice<In, Out, State>;
      } catch (error) {
        // Log error but return null for graceful degradation
        console.error(`Failed to get session slice for ${toolName}:`, error);
        return null;
      }
    },

    async set(sessionId: string, slice: SessionSlice<In, Out, State>): Promise<void> {
      const session = await sessionManager.get(sessionId);
      if (!session) {
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

    async patch(sessionId: string, partial: Partial<SessionSlice<In, Out, State>>): Promise<void> {
      const existing = await this.get(sessionId);

      if (!existing) {
        // If no existing slice and no input provided, cannot create
        if (!partial.input) {
          throw new Error('Cannot create new slice without input');
        }
        // Create a minimal slice with the partial data
        const newSlice: SessionSlice<In, Out, State> = {
          input: partial.input,
          state: (partial.state || {}) as State,
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
      const updated: SessionSlice<In, Out, State> = {
        ...existing,
        ...partial,
        updatedAt: new Date(),
      };
      await this.set(sessionId, updated);
    },

    async clear(sessionId: string): Promise<void> {
      const session = await sessionManager.get(sessionId);
      if (!session) {
        return;
      }

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
 * Get typed session slice with Result wrapper
 *
 * @param toolName - Name of the tool
 * @param sessionId - Session ID
 * @param io - Tool input/output schemas
 * @param context - Tool context
 * @returns Result with session slice or error
 */
export async function getSessionSlice<In, Out, State = Record<string, unknown>>(
  toolName: string,
  sessionId: string,
  io: ToolIO<In, Out>,
  context?: ToolContext,
  stateSchema?: z.ZodType<State>,
): Promise<Result<SessionSlice<In, Out, State> | null>> {
  try {
    const ops = useSessionSlice(toolName, io, context, stateSchema);
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
 * Update typed session slice with Result wrapper
 *
 * @param toolName - Name of the tool
 * @param sessionId - Session ID
 * @param slice - Partial slice to update
 * @param io - Tool input/output schemas
 * @param context - Tool context
 * @returns Result indicating success or failure
 */
export async function updateSessionSlice<In, Out, State = Record<string, unknown>>(
  toolName: string,
  sessionId: string,
  slice: Partial<SessionSlice<In, Out, State>>,
  io: ToolIO<In, Out>,
  context?: ToolContext,
  stateSchema?: z.ZodType<State>,
): Promise<Result<void>> {
  try {
    const ops = useSessionSlice(toolName, io, context, stateSchema);
    if (!ops) {
      return Failure('Session manager not available');
    }
    await ops.patch(sessionId, slice);
    return Success(undefined);
  } catch (error) {
    return Failure(`Failed to update session slice: ${extractErrorMessage(error)}`);
  }
}
