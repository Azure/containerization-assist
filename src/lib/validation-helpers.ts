/**
 * Validation Helper Functions
 *
 * Provides convenience wrappers and higher-level validation utilities
 * to reduce boilerplate across tools. These helpers build on the core
 * validation functions from @/lib/validation.
 */

import { Failure, Success, type Result } from '@/types';
import { validatePath, validateDockerTag, type PathValidationOptions } from './validation';

/**
 * Parsed components of a Docker image name
 *
 * @example
 * parseImageName('registry.io/org/app:v1.0')
 * // => { registry: 'registry.io', repository: 'org/app', tag: 'v1.0', fullName: '...' }
 *
 * parseImageName('node:20-alpine')
 * // => { repository: 'node', tag: '20-alpine', fullName: 'node:20-alpine' }
 */
export interface ParsedImageName {
  /** Registry hostname (e.g., 'registry.io', 'docker.io') */
  registry?: string;
  /** Repository path (e.g., 'library/node', 'org/app') */
  repository: string;
  /** Image tag (e.g., 'latest', 'v1.0.0', '20-alpine') */
  tag: string;
  /** Full image name as provided */
  fullName: string;
}

/**
 * Parse Docker image name into components
 *
 * Extracts registry, repository, and tag from a Docker image name.
 * Handles various formats:
 * - `image:tag` - Docker Hub library image
 * - `org/image:tag` - Docker Hub organization image
 * - `registry.io/org/image:tag` - Private registry image
 *
 * @param imageName - Full Docker image name
 * @returns Parsed components or error
 *
 * @example
 * ```typescript
 * const result = parseImageName('docker.io/library/node:20-alpine');
 * if (result.ok) {
 *   const { registry, repository, tag } = result.value;
 *   console.log(`Registry: ${registry}, Repo: ${repository}, Tag: ${tag}`);
 * }
 * ```
 */
export function parseImageName(imageName: string): Result<ParsedImageName> {
  if (!imageName?.trim()) {
    return Failure('Image name cannot be empty', {
      message: 'Image name is required',
      hint: 'Docker image names must contain at least a repository name',
      resolution: 'Provide a valid image name, e.g., "myapp:latest" or "docker.io/library/node:20"',
    });
  }

  // Split by colon to separate image path from tag
  const colonIndex = imageName.lastIndexOf(':');
  let imagePath: string;
  let tag: string;

  if (colonIndex > 0) {
    // Check if the part after colon looks like a port number (registry with port)
    // Port pattern: :5000/... or :8080/...
    const afterColon = imageName.substring(colonIndex + 1);
    const hasPathAfterColon = afterColon.includes('/');
    // Validate that the segment before the slash is actually numeric (port number)
    const isPort = /^\d+\//.test(afterColon);

    if (hasPathAfterColon && isPort) {
      // This is a port number, not a tag (e.g., registry.io:5000/image)
      imagePath = imageName;
      tag = 'latest';
    } else {
      // This is a tag
      imagePath = imageName.substring(0, colonIndex);
      tag = afterColon || 'latest';
    }
  } else {
    // No colon, default to 'latest' tag
    imagePath = imageName;
    tag = 'latest';
  }

  // Split path by slashes to identify registry and repository
  const parts = imagePath.split('/').filter((p) => p.length > 0);
  let registry: string | undefined;
  let repository: string;

  if (parts.length === 0) {
    return Failure('Invalid image name: repository is required', {
      message: 'Could not parse repository from image name',
      hint: 'Image name format should be [registry/][namespace/]repository[:tag]',
      resolution: 'Provide a valid image name with at least a repository component',
      details: { providedName: imageName },
    });
  } else if (parts.length === 1) {
    // Format: image (Docker Hub library)
    repository = parts[0] || '';
  } else if (parts.length === 2) {
    // Format: org/image (Docker Hub org) or registry.io/image
    // Check if first part contains a dot or colon (indicates registry)
    if (parts[0] && (parts[0].includes('.') || parts[0].includes(':'))) {
      registry = parts[0];
      repository = parts[1] || '';
    } else {
      // Docker Hub organization
      repository = imagePath;
    }
  } else {
    // Format: registry.io/org/image (3+ parts)
    registry = parts[0];
    repository = parts.slice(1).join('/');
  }

  // Validate extracted components
  if (!repository?.trim()) {
    return Failure('Invalid image name: repository is required', {
      message: 'Could not parse repository from image name',
      hint: 'Image name format should be [registry/][namespace/]repository[:tag]',
      resolution: 'Provide a valid image name with at least a repository component',
      details: { providedName: imageName },
    });
  }

  // Validate tag format using existing validator
  const tagValidation = validateDockerTag(tag);
  if (!tagValidation.ok) {
    return tagValidation;
  }

  return Success({
    ...(registry && { registry }),
    repository,
    tag,
    fullName: imageName,
  });
}

/**
 * Validate path and return Result - convenience wrapper
 *
 * This is a direct pass-through to validatePath from @/lib/validation,
 * provided for consistency and discoverability alongside other helpers.
 *
 * @param pathInput - Path to validate (relative or absolute)
 * @param options - Validation options
 * @returns Validated absolute path or error
 *
 * @example
 * ```typescript
 * const result = await validatePathOrFail('./src', {
 *   mustExist: true,
 *   mustBeDirectory: true,
 * });
 * if (!result.ok) return result;
 * const validPath = result.value;
 * ```
 */
export async function validatePathOrFail(
  pathInput: string,
  options: PathValidationOptions = {},
): Promise<Result<string>> {
  return validatePath(pathInput, options);
}

/**
 * Validate Docker image tag format - convenience alias
 *
 * Re-exported from @/lib/validation for consistency with other helpers.
 * Validates that a tag follows Docker naming conventions.
 *
 * @param tag - Docker tag to validate
 * @returns Validated tag or error
 *
 * @example
 * ```typescript
 * const result = validateImageTag('v1.0.0-alpha');
 * if (result.ok) {
 *   console.log('Valid tag:', result.value);
 * }
 * ```
 */
export function validateImageTag(tag: string): Result<string> {
  return validateDockerTag(tag);
}

/**
 * Create a reusable path validator with preset options
 *
 * Factory function for creating path validators with common configurations.
 * Useful for tools that validate multiple paths with the same requirements.
 *
 * @param options - Validation options to apply
 * @returns Validation function
 *
 * @example
 * ```typescript
 * const validateDirectory = createPathValidator({
 *   mustExist: true,
 *   mustBeDirectory: true,
 * });
 *
 * const repoResult = await validateDirectory(input.repositoryPath);
 * if (!repoResult.ok) return repoResult;
 *
 * const moduleResult = await validateDirectory(input.modulePath);
 * if (!moduleResult.ok) return moduleResult;
 * ```
 */
export function createPathValidator(
  options: PathValidationOptions,
): (path: string) => Promise<Result<string>> {
  return async (path: string): Promise<Result<string>> => {
    return validatePath(path, options);
  };
}
