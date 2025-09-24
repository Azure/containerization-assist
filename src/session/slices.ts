/**
 * Session Slice Utilities
 *
 * Provides typed wrapper utilities for tool-specific session slices
 */

import { z, type ZodTypeAny } from 'zod';
import type { ToolContext } from '@/mcp/context';
import type { SessionManager } from './core';
import { Result, Success, Failure } from '@/types';
import {
  SessionSlice,
  SessionSliceOperations,
  SessionSliceStore,
  ToolIO,
  ToolSliceMetadata,
  hasToolSlices,
  createSliceMetadata,
} from './types';
import { extractErrorMessage } from '@/lib/error-utils';

/**
 * Create a SessionSliceStore adapter from SessionManager
 */
function createSessionStore(manager: SessionManager): SessionSliceStore {
  return {
    async get<T>(sessionId: string, key: string): Promise<T | null> {
      const result = await manager.get(sessionId);
      if (!result.ok || !result.value) return null;
      const value = result.value[key];
      return value as T;
    },

    async set<T>(sessionId: string, key: string, value: T): Promise<void> {
      await manager.update(sessionId, { [key]: value });
    },

    async delete(sessionId: string, key: string): Promise<void> {
      const result = await manager.get(sessionId);
      if (result.ok && result.value) {
        const { [key]: _, ...rest } = result.value;
        await manager.update(sessionId, rest);
      }
    },

    async has(sessionId: string): Promise<boolean> {
      const result = await manager.get(sessionId);
      return result.ok && result.value !== null;
    },
  };
}

/**
 * Get session store from context
 */
function getSessionStore(context?: ToolContext): SessionSliceStore | null {
  if (context && typeof context === 'object' && 'sessionManager' in context) {
    const manager = context.sessionManager;
    if (manager && typeof manager === 'object') {
      return createSessionStore(manager);
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
export function useSessionSlice<
  TInputSchema extends ZodTypeAny,
  TOutputSchema extends ZodTypeAny,
  TStateSchema extends ZodTypeAny = z.ZodObject<Record<string, never>>,
>(
  toolName: string,
  io: ToolIO<TInputSchema, TOutputSchema>,
  context?: ToolContext,
  stateSchema?: TStateSchema,
): SessionSliceOperations<
  z.infer<TInputSchema>,
  z.infer<TOutputSchema>,
  z.infer<TStateSchema>
> | null {
  const store = getSessionStore(context);
  if (!store) {
    return null;
  }

  type In = z.infer<TInputSchema>;
  type Out = z.infer<TOutputSchema>;
  type State = z.infer<TStateSchema>;

  return {
    async get(sessionId: string): Promise<SessionSlice<In, Out, State> | null> {
      try {
        const metadata = await store.get<Record<string, unknown>>(sessionId, 'metadata');
        if (!metadata || !hasToolSlices(metadata)) {
          return null;
        }

        const rawSlice = metadata.toolSlices[toolName];
        if (!rawSlice || typeof rawSlice !== 'object') {
          return null;
        }

        // Cast to any for property access, then validate
        const slice = rawSlice as Record<string, unknown>;
        const validatedSlice: Partial<SessionSlice<In, Out, State>> = {};

        // Validate with schemas if provided
        if ('input' in slice && slice.input !== undefined) {
          const inputResult = io.input.safeParse(slice.input);
          if (!inputResult.success) return null;
          validatedSlice.input = inputResult.data;
        }

        if ('output' in slice && slice.output !== undefined) {
          const outputResult = io.output.safeParse(slice.output);
          if (!outputResult.success) return null;
          validatedSlice.output = outputResult.data;
        }

        if ('state' in slice) {
          if (stateSchema && slice.state !== undefined) {
            const stateResult = stateSchema.safeParse(slice.state);
            if (!stateResult.success) return null;
            validatedSlice.state = stateResult.data;
          } else {
            validatedSlice.state = slice.state || {};
          }
        } else {
          validatedSlice.state = {};
        }

        if ('updatedAt' in slice) {
          const rawUpdatedAt = slice.updatedAt;
          if (typeof rawUpdatedAt === 'string' || typeof rawUpdatedAt === 'number') {
            const date = new Date(rawUpdatedAt);
            if (!isNaN(date.getTime())) {
              validatedSlice.updatedAt = date;
            }
          } else if (rawUpdatedAt instanceof Date) {
            validatedSlice.updatedAt = rawUpdatedAt;
          }
        }

        return validatedSlice as SessionSlice<In, Out, State>;
      } catch (error) {
        console.error(`Failed to get session slice for ${toolName}:`, error);
        return null;
      }
    },

    async set(sessionId: string, slice: SessionSlice<In, Out, State>): Promise<void> {
      // Validate slice data with schemas
      const validatedSlice = { ...slice };

      if (slice.input !== undefined) {
        validatedSlice.input = io.input.parse(slice.input);
      }

      if (slice.output !== undefined) {
        validatedSlice.output = io.output.parse(slice.output);
      }

      if (slice.state !== undefined && stateSchema) {
        validatedSlice.state = stateSchema.parse(slice.state);
      }

      const metadata = (await store.get<Record<string, unknown>>(sessionId, 'metadata')) || {};
      const sliceMetadata = hasToolSlices(metadata) ? metadata : createSliceMetadata();

      await store.set(sessionId, 'metadata', {
        ...metadata,
        toolSlices: {
          ...sliceMetadata.toolSlices,
          [toolName]: { ...validatedSlice, updatedAt: new Date() },
        },
        lastAccessed: {
          ...('lastAccessed' in sliceMetadata
            ? (sliceMetadata as ToolSliceMetadata).lastAccessed
            : {}),
          [toolName]: new Date(),
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
      const metadata = await store.get<Record<string, unknown>>(sessionId, 'metadata');
      if (!metadata || !hasToolSlices(metadata)) {
        return;
      }

      const { [toolName]: _, ...remainingSlices } = metadata.toolSlices;

      await store.set(sessionId, 'metadata', {
        ...metadata,
        toolSlices: remainingSlices,
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
 * @param stateSchema - Optional state schema
 * @returns Result with session slice or error
 */
export async function getSessionSlice<
  TInputSchema extends ZodTypeAny,
  TOutputSchema extends ZodTypeAny,
  TStateSchema extends ZodTypeAny = z.ZodObject<Record<string, never>>,
>(
  toolName: string,
  sessionId: string,
  io: ToolIO<TInputSchema, TOutputSchema>,
  context?: ToolContext,
  stateSchema?: TStateSchema,
): Promise<
  Result<SessionSlice<z.infer<TInputSchema>, z.infer<TOutputSchema>, z.infer<TStateSchema>> | null>
> {
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
 * @param stateSchema - Optional state schema
 * @returns Result indicating success or failure
 */
export async function updateSessionSlice<
  TInputSchema extends ZodTypeAny,
  TOutputSchema extends ZodTypeAny,
  TStateSchema extends ZodTypeAny = z.ZodObject<Record<string, never>>,
>(
  toolName: string,
  sessionId: string,
  slice: Partial<
    SessionSlice<z.infer<TInputSchema>, z.infer<TOutputSchema>, z.infer<TStateSchema>>
  >,
  io: ToolIO<TInputSchema, TOutputSchema>,
  context?: ToolContext,
  stateSchema?: TStateSchema,
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
