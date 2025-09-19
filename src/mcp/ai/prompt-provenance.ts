import crypto from 'node:crypto';

export interface PromptProvenance {
  id: string;
  version: string | undefined;
  hash: string; // sha256 of resolved prompt
  createdAt: string; // ISO timestamp
}

/**
 * Create provenance metadata for a prompt execution
 * @param id - The prompt ID
 * @param resolvedPrompt - The fully-rendered prompt with variables substituted
 * @param version - Optional version string
 * @returns PromptProvenance object for tracking and reproducibility
 */
export function makeProvenance(
  id: string,
  resolvedPrompt: string,
  version?: string,
): PromptProvenance {
  // Hash the fully-rendered prompt for reproducibility
  const hash = crypto.createHash('sha256').update(resolvedPrompt, 'utf8').digest('hex');

  return {
    id,
    version,
    hash,
    createdAt: new Date().toISOString(),
  };
}

/**
 * Check if provenance should be included in outputs
 * @returns true if INCLUDE_PROVENANCE_IN_OUTPUT env var is set to 'true'
 */
export function shouldIncludeProvenance(): boolean {
  return process.env.INCLUDE_PROVENANCE_IN_OUTPUT === 'true';
}
