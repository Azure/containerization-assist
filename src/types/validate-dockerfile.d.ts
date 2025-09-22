/**
 * Type declarations for the validate-dockerfile npm package
 * Use ValidationResult from @/types/result-types instead of the duplicate interface
 */
declare module 'validate-dockerfile' {
  // Use ValidationResult type instead
  // import type { CoreValidationResult } from '@/types/result-types.js';

  function validateDockerfile(dockerfile: string): {
    valid: boolean;
    line?: number;
    message?: string;
    priority?: number;
    errors?: Array<{
      message: string;
      line?: number;
      priority: number;
    }>;
  };
  export = validateDockerfile;
}
