/**
 * Zod schema for the create-policy prompt arguments.
 *
 * No required arguments — the prompt itself instructs the LLM to ask the
 * user where to store the policy (project vs global) during the conversation.
 */

export const createPolicySchema = {} as const;

export type CreatePolicyArgs = Record<string, never>;
