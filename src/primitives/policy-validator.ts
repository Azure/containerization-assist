import type { z } from 'zod';

import { Success, Failure, type Result } from '@types';
import type { ToolContext } from '@/core/context';
import { applyPolicy } from '@/config/policy-eval';
import { tool, type Tool } from '@/types/tool';
import type { IToolDefinition } from '@/tools/shared/toolDefinition';

import { toFinding, type ValidatePolicyOut } from './types';

/**
 * Build a policy-validator primitive from a tool definition and its Zod schema.
 *
 * Today only the `validate` primitive uses this factory; it survives as a
 * named helper because the handler logic is non-trivial (no-policy fast path,
 * mapping, error envelope) and worth keeping separate from the schema/wiring.
 *
 * The TSchema generic is constrained to a Zod type whose inferred output
 * includes `content: string`, so the handler can read `input.content` with
 * full static safety — no runtime cast required. Calling this factory with a
 * schema that doesn't include `content: string` is a compile-time error.
 *
 * The handler:
 *   1. Returns a passing empty envelope when ctx.policy is undefined.
 *   2. Calls applyPolicy(ctx.policy, input.content) otherwise.
 *   3. Maps the result via toFinding into a uniform ValidatePolicyOut envelope.
 *   4. Returns Failure with hint context on evaluator throws.
 */
export function createPolicyValidatorPrimitive<TSchema extends z.ZodType<{ content: string }>>(
  toolDefinition: IToolDefinition,
  schema: TSchema,
  errorPrefix: string,
): Tool<TSchema, ValidatePolicyOut> {
  async function handler(
    input: z.infer<TSchema>,
    ctx: ToolContext,
  ): Promise<Result<ValidatePolicyOut>> {
    const { content } = input;

    if (!ctx.policy) {
      return Success<ValidatePolicyOut>({
        passed: true,
        violations: [],
        warnings: [],
        suggestions: [],
      });
    }

    try {
      const result = await applyPolicy(ctx.policy, content);
      const violations = (result.violations ?? []).map((r) => toFinding(r, 'block'));
      const warnings = (result.warnings ?? []).map((r) => toFinding(r, 'warn'));
      const suggestions = (result.suggestions ?? []).map((r) => toFinding(r, 'suggest'));

      return Success<ValidatePolicyOut>({
        passed: violations.length === 0,
        violations,
        warnings,
        suggestions,
      });
    } catch (err) {
      const cause = err instanceof Error ? err.message : String(err);
      return Failure(`${errorPrefix} while evaluating policy: ${cause}`, {
        message: cause,
        hint: `Content length: ${content.length} chars. Check the policy file syntax and that the evaluator initialized correctly.`,
      });
    }
  }

  return tool<TSchema, ValidatePolicyOut>({
    ...toolDefinition,
    schema,
    handler,
  });
}
