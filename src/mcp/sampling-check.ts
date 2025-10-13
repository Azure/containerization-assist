/**
 * Sampling Availability Check
 * Centralized utility for checking if MCP sampling is available
 */

import type { ToolContext } from './context';

/**
 * Result of sampling availability check
 */
export interface SamplingCheckResult {
  available: boolean;
  message: string;
}

/**
 * Check if sampling is available in the current context
 * This is a lightweight test that attempts a minimal sampling request
 *
 * @param ctx - Tool context with sampling capabilities
 * @returns Result indicating availability and appropriate message
 */
export async function checkSamplingAvailability(ctx: ToolContext): Promise<SamplingCheckResult> {
  try {
    await ctx.sampling.createMessage({
      messages: [
        {
          role: 'user',
          content: [
            {
              type: 'text',
              text: 'test',
            },
          ],
        },
      ],
      maxTokens: 1, // Minimal token count to keep the check fast
    });

    return {
      available: true,
      message: '',
    };
  } catch (e) {
    ctx.logger.debug({ error: e }, 'Sampling not available');
    return {
      available: false,
      message: 'Sampling is not enabled, turn it on to get the best output',
    };
  }
}
