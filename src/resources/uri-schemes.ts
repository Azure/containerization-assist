import { Result, Success, Failure } from '@types';
import { URI_SCHEMES, type ParsedUri, type UriScheme } from './types';
import { extractErrorMessage } from '@lib/error-utils';

/**
 * URI parser and builder for MCP resource schemes
 *
 * Functional module providing utilities for parsing, building, and validating
 * MCP resource URIs with support for pattern matching and unique ID generation.
 *
 * @example
 * ```typescript
 * // Parse a URI
 * const result = parseUri('mcp://dockerfile/app.dockerfile');
 * if (result.success) {
 *   const { scheme, path } = result.value;
 *   // scheme: 'mcp', path: '/dockerfile/app.dockerfile'
 * }
 *
 * // Build a URI
 * const uri = buildUri('cache', '/results/scan-123', { type: 'json' });
 * // Returns: 'cache:///results/scan-123?type=json'
 *
 * // Generate unique URI
 * const unique = generateUniqueUri('session', 'workflows');
 * // Returns: 'session:///workflows/1234567890-abc123def'
 * ```
 */

/**
 * Parse a URI into its components
 *
 * @param uri - The URI string to parse
 * @returns Result containing parsed URI components or error message
 */
export const parseUri = (uri: string): Result<ParsedUri> => {
  try {
    const url = new URL(uri);

    if (!isValidScheme(url.protocol.slice(0, -1))) {
      return Failure(`Invalid URI scheme: ${url.protocol}`);
    }

    const query: Record<string, string> = {};
    url.searchParams.forEach((value, key) => {
      query[key] = value;
    });

    const result: ParsedUri = {
      scheme: url.protocol.slice(0, -1) as UriScheme,
      path: url.pathname,
    };

    if (Object.keys(query).length > 0) {
      result.query = query;
    }

    if (url.hash) {
      result.fragment = url.hash.slice(1);
    }

    return Success(result);
  } catch (error) {
    return Failure(`Failed to parse URI: ${extractErrorMessage(error)}`);
  }
};

/**
 * Build a URI from components
 *
 * @param scheme - The URI scheme (e.g., 'mcp', 'cache')
 * @param path - The path component
 * @param query - Optional query parameters as key-value pairs
 * @param fragment - Optional fragment identifier
 * @returns Complete URI string
 */
export const buildUri = (
  scheme: UriScheme,
  path: string,
  query?: Record<string, string>,
  fragment?: string,
): string => {
  let uri = `${scheme}://${path}`;

  if (query && Object.keys(query).length > 0) {
    const searchParams = new URLSearchParams(query);
    uri += `?${searchParams.toString()}`;
  }

  if (fragment) {
    uri += `#${fragment}`;
  }

  return uri;
};

/**
 * Generate a unique URI for a given scheme and base path
 *
 * Creates a URI with timestamp and random suffix for uniqueness.
 *
 * @param scheme - The URI scheme to use
 * @param basePath - Optional base path (defaults to empty)
 * @returns Unique URI string with timestamp-random suffix
 */
export const generateUniqueUri = (scheme: UriScheme, basePath: string = ''): string => {
  const timestamp = Date.now();
  const random = Math.random().toString(36).substr(2, 9);
  const path = basePath ? `${basePath}/${timestamp}-${random}` : `${timestamp}-${random}`;
  return buildUri(scheme, path);
};

/**
 * Check if a string matches a URI pattern (supports wildcards)
 *
 * Supports glob-style patterns:
 * - `*` matches any sequence of characters
 * - `?` matches any single character
 * - `*` pattern matches all URIs
 *
 * @param uri - The URI string to test
 * @param pattern - The pattern to match against (supports glob wildcards)
 * @returns True if the URI matches the pattern
 */
export const matchesUriPattern = (uri: string, pattern: string): boolean => {
  if (pattern === '*') return true;

  // First escape all RegExp special characters except glob wildcards
  const escapedPattern = pattern.replace(/[.+^${}()|\\]/g, '\\$&');

  // Then convert glob-style pattern to regex
  const regexPattern = escapedPattern
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.')
    .replace(/\\\[/g, '[') // Unescape [ that was escaped above
    .replace(/\\\]/g, ']'); // Unescape ] that was escaped above

  return new RegExp(`^${regexPattern}$`).test(uri);
};

/**
 * Type guard to validate URI schemes
 *
 * @param scheme - String to validate as a URI scheme
 * @returns True if the scheme is valid
 */
export const isValidScheme = (scheme: string): scheme is UriScheme => {
  return Object.values(URI_SCHEMES).includes(scheme as UriScheme);
};
