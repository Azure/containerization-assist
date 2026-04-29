import type { z } from 'zod';

import { Success, Failure, type Result } from '@types';
import type { ToolContext } from '@/core/context';
import { applyPolicy } from '@/config/policy-eval';
import { tool, type Tool } from '@/types/tool';
import type { IToolDefinition } from '@/tools/shared/toolDefinition';

import { toFinding, type ValidatePolicyOut } from './types';

/**
 * Build a policy-validator primitive from a tool definition.
 *
 * All three policy validators (validate-dockerfile, validate-k8s-manifest,
 * validate-compose) share identical handler logic — only the error prefix
 * and the tool-definition wrapper differ. This factory captures that pattern.
 *
 * The handler:
 *   1. Returns a passing empty envelope when ctx.policy is undefined.
 *   2. Calls applyPolicy(ctx.policy, input.content) otherwise.
 *   3. Maps the result via toFinding into a uniform ValidatePolicyOut envelope.
 *   4. Returns Failure with hint context on evaluator throws.
 *
 * The schema's inferred output is required to have `content: string` —
 * this is enforced at the runtime level by the handler reading
 * `input.content`. Each caller's schema must satisfy this contract.
 */
export function createPolicyValidatorPrimitive<TSchema extends z.ZodTypeAny>(
  toolDefinition: IToolDefinition,
  errorPrefix: string,
): Tool<TSchema, ValidatePolicyOut> {
  async function handler(
    input: z.infer<TSchema>,
    ctx: ToolContext,
  ): Promise<Result<ValidatePolicyOut>> {
    // Schema contract: every caller's schema produces { content: string, ... }.
    const content = (input as { content: string }).content;

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
    schema: toolDefinition.schema as TSchema,
    handler,
  });
}
