/**
 * Type declarations for the validate-dockerfile npm package
 */
declare module 'validate-dockerfile' {
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
