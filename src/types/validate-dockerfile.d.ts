declare module 'validate-dockerfile' {
  interface ValidationResult {
    valid: boolean;
    line?: number;
    message?: string;
    priority?: number;
  }

  function validateDockerfile(dockerfile: string): ValidationResult;

  export = validateDockerfile;
}
