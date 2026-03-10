/**
 * Zod schema for the kind-loop prompt arguments.
 *
 * MCP prompt arguments are always strings at the protocol level.
 * Optional fields default to sensible values in the prompt builder.
 */

import { sharedLoopSchema } from '../shared/schema';

export const localKindDevLoopSchema = {
  ...sharedLoopSchema,
} as const;

export type LocalKindDevLoopArgs = {
  namespace?: string | undefined;
  imageName?: string | undefined;
};
