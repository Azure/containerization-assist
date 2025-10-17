/**
 * Standardized validation helpers for input validation across all tools.
 * Provides consistent error messages and validation logic.
 */

import fs from 'node:fs/promises';
import path from 'node:path';

import { Failure, Success, type Result } from '@/types';

/**
 * Options for path validation
 */
export interface PathValidationOptions {
  /** Path must exist on filesystem */
  mustExist?: boolean;
  /** Path must be a directory */
  mustBeDirectory?: boolean;
  /** Path must be a file */
  mustBeFile?: boolean;
}

/**
 * Validate and resolve file path
 *
 * Ensures paths are absolute and optionally validates existence and type.
 * Always returns absolute paths for consistent tool behavior.
 *
 * @param pathStr - Path to validate (relative or absolute)
 * @param options - Validation options
 * @returns Validated absolute path or error
 *
 * @example
 * ```typescript
 * const result = await validatePath('./src', { mustExist: true, mustBeDirectory: true });
 * if (result.ok) {
 *   console.log('Valid directory:', result.value);
 * }
 * ```
 */
export async function validatePath(
  pathStr: string,
  options: PathValidationOptions = {},
): Promise<Result<string>> {
  // Resolve to absolute path
  const absolutePath = path.isAbsolute(pathStr) ? pathStr : path.resolve(process.cwd(), pathStr);

  // Check existence and type if required
  if (options.mustExist) {
    try {
      const stats = await fs.stat(absolutePath);

      if (options.mustBeDirectory && !stats.isDirectory()) {
        return Failure(`Path is not a directory: ${absolutePath}`, {
          message: `Path is not a directory: ${absolutePath}`,
          hint: 'The specified path exists but is a file, not a directory',
          resolution: 'Provide a path to a directory, not a file',
        });
      }

      if (options.mustBeFile && !stats.isFile()) {
        return Failure(`Path is not a file: ${absolutePath}`, {
          message: `Path is not a file: ${absolutePath}`,
          hint: 'The specified path exists but is a directory, not a file',
          resolution: 'Provide a path to a file, not a directory',
        });
      }
    } catch (error) {
      const errorMsg = error instanceof Error ? error.message : String(error);
      return Failure(`Path does not exist: ${absolutePath}`, {
        message: `Path does not exist: ${absolutePath}`,
        hint: 'The specified path could not be found on the filesystem',
        resolution: 'Verify the path is correct and the file/directory exists',
        details: { originalError: errorMsg },
      });
    }
  }

  return Success(absolutePath);
}

/**
 * Validate Docker image name format
 *
 * Validates image names according to Docker naming conventions.
 * Supports: [registry/][namespace/]repository:tag
 *
 * @param imageName - Docker image name to validate
 * @returns Validated image name or error
 *
 * @example
 * ```typescript
 * validateImageName('myapp:latest') // Valid
 * validateImageName('docker.io/library/node:18') // Valid
 * validateImageName('INVALID NAME') // Invalid
 * ```
 */
export function validateImageName(imageName: string): Result<string> {
  // Docker image name pattern:
  // - Optional registry (hostname[:port]/)
  // - Optional namespace (name/)
  // - Repository name (required)
  // - Optional tag (:tag)
  const imagePattern = new RegExp(
    '^(?:(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?' +
      '(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?)*(?::[0-9]+)?\\/)?' +
      '(?:[a-z0-9]+(?:[._-][a-z0-9]+)*\\/)*)?[a-z0-9]+(?:[._-][a-z0-9]+)*' +
      '(?::[a-zA-Z0-9][a-zA-Z0-9._-]*)?$',
  );

  if (!imageName?.trim()) {
    return Failure('Image name cannot be empty', {
      message: 'Image name cannot be empty',
      hint: 'Docker image names must contain at least a repository name',
      resolution: 'Provide a valid image name, e.g., "myapp:latest"',
    });
  }

  if (!imagePattern.test(imageName)) {
    return Failure('Invalid image name format. Expected: [registry/]repository[:tag]', {
      message: 'Invalid image name format',
      hint: 'Docker image names must follow the pattern: [registry/][namespace/]repository[:tag]',
      resolution:
        'Use lowercase alphanumeric characters, hyphens, underscores, and dots. Examples: "myapp:latest", "docker.io/library/node:18"',
      details: { providedName: imageName },
    });
  }

  // Additional validation: check length
  if (imageName.length > 255) {
    return Failure('Image name too long (max 255 characters)', {
      message: 'Image name exceeds maximum length',
      hint: 'Docker image names cannot exceed 255 characters',
      resolution: 'Shorten the image name or use a shorter registry/namespace',
      details: { length: imageName.length, maxLength: 255 },
    });
  }

  return Success(imageName);
}

/**
 * Validate Kubernetes resource name
 *
 * Validates resource names according to Kubernetes naming conventions (DNS-1123).
 * Names must be lowercase alphanumeric with hyphens, max 253 characters.
 *
 * @param name - Kubernetes resource name to validate
 * @returns Validated name or error
 *
 * @example
 * ```typescript
 * validateK8sName('my-app') // Valid
 * validateK8sName('my-app-123') // Valid
 * validateK8sName('MyApp') // Invalid (uppercase)
 * validateK8sName('my_app') // Invalid (underscore)
 * ```
 */
export function validateK8sName(name: string): Result<string> {
  // Kubernetes DNS-1123 label pattern:
  // - Lowercase alphanumeric or hyphen
  // - Must start and end with alphanumeric
  // - Max 253 characters
  const namePattern = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

  if (!name?.trim()) {
    return Failure('Kubernetes name cannot be empty', {
      message: 'Kubernetes name cannot be empty',
      hint: 'Kubernetes resource names must contain at least one character',
      resolution: 'Provide a valid name using lowercase letters, numbers, and hyphens',
    });
  }

  if (name.length > 253) {
    return Failure('Kubernetes name too long (max 253 characters)', {
      message: 'Kubernetes name exceeds maximum length',
      hint: 'Kubernetes resource names cannot exceed 253 characters',
      resolution: 'Shorten the resource name',
      details: { length: name.length, maxLength: 253 },
    });
  }

  if (!namePattern.test(name)) {
    return Failure('Invalid Kubernetes name. Must be lowercase alphanumeric with hyphens', {
      message: 'Invalid Kubernetes name format',
      hint: 'Kubernetes names must be lowercase alphanumeric characters or hyphens',
      resolution:
        'Use only lowercase letters (a-z), numbers (0-9), and hyphens (-). Must start and end with alphanumeric. Example: "my-app-123"',
      details: { providedName: name },
    });
  }

  return Success(name);
}

/**
 * Validate environment variable name
 *
 * Validates environment variable names according to POSIX conventions.
 * Names must be uppercase letters, numbers, and underscores, starting with a letter or underscore.
 *
 * @param name - Environment variable name to validate
 * @returns Validated name or error
 *
 * @example
 * ```typescript
 * validateEnvName('MY_VAR') // Valid
 * validateEnvName('API_KEY_123') // Valid
 * validateEnvName('my-var') // Invalid (lowercase, hyphen)
 * validateEnvName('123_VAR') // Invalid (starts with number)
 * ```
 */
export function validateEnvName(name: string): Result<string> {
  // POSIX environment variable pattern:
  // - Uppercase letters, digits, and underscores
  // - Must start with letter or underscore
  const envPattern = /^[A-Z_][A-Z0-9_]*$/;

  if (!name?.trim()) {
    return Failure('Environment variable name cannot be empty', {
      message: 'Environment variable name cannot be empty',
      hint: 'Environment variable names must contain at least one character',
      resolution: 'Provide a valid name using uppercase letters, numbers, and underscores',
    });
  }

  if (!envPattern.test(name)) {
    return Failure('Invalid environment variable name. Must be uppercase with underscores', {
      message: 'Invalid environment variable name format',
      hint: 'Environment variable names must use uppercase letters, numbers, and underscores only',
      resolution:
        'Use only uppercase letters (A-Z), numbers (0-9), and underscores (_). Must start with a letter or underscore. Example: "API_KEY_123"',
      details: { providedName: name },
    });
  }

  return Success(name);
}

/**
 * Validate port number
 *
 * Validates that a port is within valid range (1-65535).
 *
 * @param port - Port number to validate
 * @returns Validated port number or error
 *
 * @example
 * ```typescript
 * validatePort(8080) // Valid
 * validatePort(443) // Valid
 * validatePort(0) // Invalid (too low)
 * validatePort(70000) // Invalid (too high)
 * ```
 */
export function validatePort(port: number): Result<number> {
  if (!Number.isInteger(port)) {
    return Failure('Port must be an integer', {
      message: 'Invalid port number',
      hint: 'Port numbers must be whole numbers',
      resolution: 'Provide an integer port number between 1 and 65535',
      details: { providedPort: port },
    });
  }

  if (port < 1 || port > 65535) {
    return Failure('Port must be between 1 and 65535', {
      message: 'Port number out of valid range',
      hint: 'Valid port numbers range from 1 to 65535',
      resolution: 'Provide a port number within the valid range (1-65535)',
      details: { providedPort: port, validRange: '1-65535' },
    });
  }

  return Success(port);
}

/**
 * Validate namespace (relaxed Kubernetes name for namespaces)
 *
 * Similar to validateK8sName but specifically for namespaces.
 * Namespaces have the same rules as general K8s names but max 63 characters.
 *
 * @param namespace - Kubernetes namespace to validate
 * @returns Validated namespace or error
 *
 * @example
 * ```typescript
 * validateNamespace('production') // Valid
 * validateNamespace('my-app-prod') // Valid
 * validateNamespace('Prod') // Invalid (uppercase)
 * ```
 */
export function validateNamespace(namespace: string): Result<string> {
  const namePattern = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

  if (!namespace?.trim()) {
    return Failure('Namespace cannot be empty', {
      message: 'Namespace cannot be empty',
      hint: 'Kubernetes namespaces must contain at least one character',
      resolution: 'Provide a valid namespace using lowercase letters, numbers, and hyphens',
    });
  }

  if (namespace.length > 63) {
    return Failure('Namespace too long (max 63 characters)', {
      message: 'Namespace exceeds maximum length',
      hint: 'Kubernetes namespaces cannot exceed 63 characters',
      resolution: 'Shorten the namespace name',
      details: { length: namespace.length, maxLength: 63 },
    });
  }

  if (!namePattern.test(namespace)) {
    return Failure('Invalid namespace. Must be lowercase alphanumeric with hyphens', {
      message: 'Invalid namespace format',
      hint: 'Namespaces must be lowercase alphanumeric characters or hyphens',
      resolution:
        'Use only lowercase letters (a-z), numbers (0-9), and hyphens (-). Must start and end with alphanumeric. Example: "production"',
      details: { providedNamespace: namespace },
    });
  }

  return Success(namespace);
}

/**
 * Validate Docker tag
 *
 * Validates Docker image tags according to Docker conventions.
 * Tags can contain alphanumeric, periods, hyphens, and underscores, max 128 characters.
 *
 * @param tag - Docker tag to validate
 * @returns Validated tag or error
 *
 * @example
 * ```typescript
 * validateDockerTag('latest') // Valid
 * validateDockerTag('v1.0.0') // Valid
 * validateDockerTag('1.0.0-alpha.1') // Valid
 * validateDockerTag('UPPERCASE') // Invalid (uppercase not allowed)
 * ```
 */
export function validateDockerTag(tag: string): Result<string> {
  // Docker tag pattern:
  // - Alphanumeric, periods, hyphens, underscores
  // - Max 128 characters
  // - Cannot start with period or hyphen
  const tagPattern = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;

  if (!tag?.trim()) {
    return Failure('Docker tag cannot be empty', {
      message: 'Docker tag cannot be empty',
      hint: 'Docker tags must contain at least one character',
      resolution: 'Provide a valid tag like "latest", "v1.0.0", or "1.0.0-alpha"',
    });
  }

  if (tag.length > 128) {
    return Failure('Docker tag too long (max 128 characters)', {
      message: 'Docker tag exceeds maximum length',
      hint: 'Docker tags cannot exceed 128 characters',
      resolution: 'Shorten the tag name',
      details: { length: tag.length, maxLength: 128 },
    });
  }

  if (!tagPattern.test(tag)) {
    return Failure('Invalid Docker tag format', {
      message: 'Invalid Docker tag',
      hint: 'Docker tags must contain alphanumeric characters, periods, hyphens, and underscores',
      resolution:
        'Use only letters, numbers, periods (.), hyphens (-), and underscores (_). Cannot start with period or hyphen. Examples: "latest", "v1.0.0", "1.0.0-alpha"',
      details: { providedTag: tag },
    });
  }

  return Success(tag);
}
