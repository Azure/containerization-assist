/**
 * Docker-related parsing utilities for consistent parsing across the codebase
 */

import { isEmptyString } from '../string-validators';

/**
 * Parsed image tag components
 */
export interface ParsedImageTag {
  repository: string;
  tag: string;
  registry?: string | undefined;
}

/**
 * Parses a Docker image string into its components
 * Handles formats:
 * - image:tag
 * - registry/image:tag
 * - registry:port/namespace/image:tag
 *
 * @param imageString - Full image string to parse
 * @returns Parsed components
 */
export function parseImageTag(imageString: string): ParsedImageTag {
  if (isEmptyString(imageString)) {
    throw new Error('Image string cannot be empty');
  }

  // Split by colon to separate tag (last part after last colon that doesn't have slashes after it)
  const tagMatch = imageString.match(/^(.+?)(?::([^/]+))?$/);

  if (!tagMatch) {
    throw new Error(`Invalid image format: ${imageString}`);
  }

  const repositoryPart = tagMatch[1];
  const tag = tagMatch[2] || 'latest';

  if (!repositoryPart) {
    throw new Error(`Invalid image format: ${imageString}`);
  }

  // Check if repository contains registry
  let registry: string | undefined;
  let repository: string = repositoryPart;

  // If contains slash, might have registry
  if (repositoryPart.includes('/')) {
    const parts = repositoryPart.split('/');

    // Check if first part looks like registry (contains dot or colon, or is localhost)
    const firstPart = parts[0];
    if (
      firstPart &&
      (firstPart.includes('.') || firstPart.includes(':') || firstPart === 'localhost')
    ) {
      registry = firstPart;
      repository = parts.slice(1).join('/');
    }
  }

  const result: ParsedImageTag = {
    repository,
    tag,
  };

  if (registry !== undefined) {
    result.registry = registry;
  }

  return result;
}

/**
 * Constructs a full image string from components
 */
export function buildImageString(components: ParsedImageTag): string {
  const { repository, tag, registry } = components;

  let imageString = repository;

  if (registry) {
    imageString = `${registry}/${repository}`;
  }

  if (tag && tag !== 'latest') {
    imageString = `${imageString}:${tag}`;
  }

  return imageString;
}

/**
 * Extracts registry from an image string
 * Returns undefined if no explicit registry (Docker Hub implied)
 */
export function extractRegistry(imageString: string): string | undefined {
  const parsed = parseImageTag(imageString);
  return parsed.registry;
}

/**
 * Normalizes an image tag to always include explicit tag
 */
export function normalizeImageTag(imageString: string): string {
  const parsed = parseImageTag(imageString);
  return buildImageString({ ...parsed, tag: parsed.tag || 'latest' });
}

/**
 * Checks if an image string refers to an official Docker Hub image
 */
export function isOfficialImage(imageString: string): boolean {
  const parsed = parseImageTag(imageString);

  // Official images have no registry and no namespace (no slash in repository)
  return !parsed.registry && !parsed.repository.includes('/');
}

/**
 * Validates Docker build argument format (ARG=value)
 */
export function validateBuildArg(arg: string): boolean {
  return /^[A-Z_][A-Z0-9_]*=.*$/.test(arg);
}

/**
 * Parses Docker build arguments into key-value pairs
 */
export function parseBuildArgs(args: string[]): Record<string, string> {
  const parsed: Record<string, string> = {};

  for (const arg of args) {
    if (!validateBuildArg(arg)) {
      throw new Error(`Invalid build argument format: ${arg}`);
    }

    const [key, ...valueParts] = arg.split('=');
    if (key) {
      parsed[key] = valueParts.join('=');
    }
  }

  return parsed;
}
