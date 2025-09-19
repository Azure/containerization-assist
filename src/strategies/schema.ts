import { z } from 'zod';

/**
 * Strategy Schema - Defines AI model parameters and execution strategies
 * Strategies guide HOW tools execute (temperature, tokens, timeouts)
 */
export const StrategySchema = z.object({
  version: z.string(),
  id: z.string(),
  description: z.string().optional(),

  // Model parameters (no specific model, just parameters)
  parameters: z
    .object({
      temperature: z.number().min(0).max(2).optional(),
      topP: z.number().min(0).max(1).optional(),
      maxTokens: z.number().positive().optional(),
    })
    .optional(),

  // Selection rules for capabilities/features
  selectionRules: z
    .object({
      preferredCapabilities: z.record(z.string()).optional(),
      toolChain: z.array(z.string()).optional(),
    })
    .optional(),

  // Execution timeouts
  timeouts: z
    .object({
      stepMs: z.number().positive().optional(),
      totalMs: z.number().positive().optional(),
    })
    .optional(),

  // Cost constraints (soft limits)
  cost: z
    .object({
      maxUsd: z.number().positive().optional(),
    })
    .optional(),

  // Strategy templates for different contexts
  strategies: z.record(z.array(z.string())).optional(),

  // Selection conditions for strategies
  selection_rules: z
    .record(
      z.object({
        conditions: z
          .array(
            z.object({
              key: z.string(),
              value: z.union([z.string(), z.boolean(), z.number()]),
              strategy_index: z.number(),
            }),
          )
          .optional(),
        default_strategy_index: z.number(),
      }),
    )
    .optional(),
});

export type Strategy = z.infer<typeof StrategySchema>;

/**
 * Validate a strategy configuration
 */
export function validateStrategy(data: unknown): Strategy {
  return StrategySchema.parse(data);
}

/**
 * Safely validate a strategy configuration
 */
export function safeValidateStrategy(
  data: unknown,
): { success: true; data: Strategy } | { success: false; error: z.ZodError } {
  const result = StrategySchema.safeParse(data);
  if (result.success) {
    return { success: true, data: result.data };
  } else {
    return { success: false, error: result.error };
  }
}
