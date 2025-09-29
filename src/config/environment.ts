/**
 * Unified environment module - single source of truth for environment handling
 * Eliminates duplication between environmentFull and environmentBasic
 */

import { z } from 'zod';

/**
 * Standard environment types supported across all tools
 */
export type Environment = 'development' | 'staging' | 'production' | 'testing';

/**
 * Zod schema for environment validation
 */
export const environmentSchema = z
  .enum(['development', 'staging', 'production', 'testing'])
  .describe('Target environment');
