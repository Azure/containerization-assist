/**
 * Type definitions for validate-dockerfile package
 * @packageDocumentation
 */

declare module 'validate-dockerfile' {
  /**
   * Validates a Dockerfile string
   * @param dockerfile - The Dockerfile content as a string
   * @returns Validation result object
   */
  function validateDockerfile(dockerfile: string): {
    valid: boolean;
    message?: string;
    error?: Error;
  };

  export = validateDockerfile;
}
