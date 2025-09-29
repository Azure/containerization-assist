/**
 * Token Configuration
 *
 * Token limits for different operation types and contexts.
 */

export const TOKEN_CONFIG = {
  /** Standard token limit - 4096 */
  STANDARD: 4096,
  /** Extended token limit - 6144 */
  EXTENDED: 6144,
  /** Repair operation token limit - 256 */
  REPAIR: 256,
  /** Large operation token limit - 8192 */
  LARGE: 8192,
} as const;

export type TokenConfigKey = keyof typeof TOKEN_CONFIG;
