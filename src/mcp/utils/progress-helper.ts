import type { ProgressReporter } from '@mcp/context/types';

/**
 * Standardized 4-stage progress pattern for all tools
 */
export const STANDARD_STAGES = {
  VALIDATING: { message: 'Validating', percentage: 10 },
  EXECUTING: { message: 'Executing', percentage: 50 },
  FINALIZING: { message: 'Finalizing', percentage: 90 },
  COMPLETE: { message: 'Complete', percentage: 100 },
} as const;

/**
 * Helper function to report progress with optional reporter
 * Works with or without a reporter instance (null-safe)
 */
export async function reportProgress(
  reporter: ProgressReporter | undefined,
  message: string,
  percentage: number,
): Promise<void> {
  if (reporter) {
    await reporter(message, percentage);
  }
}

/**
 * Creates a standardized progress handler with 4-stage pattern
 * Returns a function that accepts stage names and reports appropriate progress
 */
export function createStandardProgress(reporter?: ProgressReporter) {
  return async (stage: keyof typeof STANDARD_STAGES): Promise<void> => {
    const { message, percentage } = STANDARD_STAGES[stage];
    await reportProgress(reporter, message, percentage);
  };
}
