/**
 * Text Processing Utilities
 *
 * Utility functions for processing AI responses and text content,
 * particularly for cleaning up code generation responses.
 */

import * as dockerParser from 'docker-file-parser';
import validateDockerfile from 'validate-dockerfile';
import { parse as parseYaml } from 'yaml';
import { DOCKERFILE_FENCE, YAML_FENCE, GENERIC_FENCE, AS_CLAUSE } from './regex-patterns';

/**
 * Extracts code content from markdown code fences
 *
 * This function extracts content from within markdown code fences,
 * removing the fence markers and any surrounding text.
 * It specifically handles code blocks with language specifiers.
 *
 * @param text - The text content containing code fences
 * @param language - Optional language specifier to match (e.g., 'dockerfile', 'yaml')
 * @returns Extracted code content, or original text if no fences found
 *
 * @example
 * ```typescript
 * const response = "Here's a Dockerfile:\n```dockerfile\nFROM node:18\nWORKDIR /app\n```\nThis is optimized.";
 * const cleaned = stripFencesAndNoise(response);
 * // Result: "FROM node:18\nWORKDIR /app"
 * ```
 */
export const stripFencesAndNoise = (text: string, language?: string): string => {
  let pattern: RegExp;

  if (language) {
    const langPattern = language.toLowerCase();
    if (langPattern === 'dockerfile' || langPattern === 'docker') {
      pattern = DOCKERFILE_FENCE;
    } else if (langPattern === 'yaml' || langPattern === 'yml') {
      pattern = YAML_FENCE;
    } else {
      pattern = new RegExp(`\`\`\`(?:${langPattern})?\\s*\\n([\\s\\S]*?)\`\`\``);
    }
  } else {
    pattern = GENERIC_FENCE;
  }

  const match = text.match(pattern);
  if (match) {
    return match[1] ? match[1].trim() : '';
  }

  return text.trim();
};

/**
 * Validates that text content looks like a Dockerfile
 *
 * Uses the dockerfile-ast parser to properly validate Dockerfile content:
 * - Must parse successfully as a Dockerfile
 * - Must contain at least one FROM instruction
 * - Provides proper syntax validation
 *
 * @param content - The content to validate
 * @returns True if content appears to be a valid Dockerfile
 *
 * @example
 * ```typescript
 * const dockerfile = "FROM node:18\nWORKDIR /app\nCOPY . .";
 * const isValid = isValidDockerfileContent(dockerfile);
 * // Result: true
 * ```
 */
export const isValidDockerfileContent = (content: string): boolean => {
  const cleaned = content.trim();

  if (!cleaned) {
    return false;
  }

  try {
    const commands = dockerParser.parse(cleaned);
    const hasFROM = commands.some((cmd: any) => cmd.name === 'FROM');

    try {
      const validationResult = validateDockerfile(cleaned);
      if (!validationResult.valid && validationResult.priority === 0) {
        return false;
      }
    } catch {
      // Continue with parse result
    }

    return hasFROM;
  } catch {
    return false;
  }
};

/**
 * Extracts the base image from Dockerfile content
 *
 * Uses the dockerfile-ast parser to properly extract base images.
 * Handles multi-stage builds by returning the first FROM instruction.
 *
 * @param dockerfileContent - The Dockerfile content to analyze
 * @returns The base image string, or null if no FROM found
 *
 * @example
 * ```typescript
 * const dockerfile = "FROM node:18-alpine\nWORKDIR /app";
 * const baseImage = extractBaseImage(dockerfile);
 * // Result: "node:18-alpine"
 * ```
 */
export const extractBaseImage = (dockerfileContent: string): string | null => {
  try {
    const commands = dockerParser.parse(dockerfileContent);
    const fromCommand = commands.find((cmd: any) => cmd.name === 'FROM');

    if (fromCommand?.args) {
      let baseImage: string | null = null;
      if (typeof fromCommand.args === 'string') {
        baseImage = fromCommand.args.trim();
      } else if (Array.isArray(fromCommand.args) && fromCommand.args.length > 0) {
        const firstArg = fromCommand.args[0];
        baseImage = typeof firstArg === 'string' ? firstArg.trim() : String(firstArg).trim();
      }

      if (baseImage) {
        const parts = baseImage.split(AS_CLAUSE);
        return parts[0]?.trim() || baseImage;
      }
    }

    return null;
  } catch {
    const fromMatch = dockerfileContent.match(/^\s*FROM\s+(\S+)/im);
    return fromMatch?.[1] ?? null;
  }
};

/**
 * Validates that text content looks like valid Kubernetes manifest(s)
 *
 * Uses proper YAML parsing to validate Kubernetes manifests:
 * - Must be valid YAML syntax
 * - Must contain apiVersion and kind fields
 * - Handles both single documents and multi-document YAML
 *
 * @param content - The content to validate
 * @returns True if content appears to be valid Kubernetes YAML
 *
 * @example
 * ```typescript
 * const manifest = `
 * apiVersion: apps/v1
 * kind: Deployment
 * metadata:
 *   name: my-app
 * `;
 * const isValid = isValidKubernetesContent(manifest);
 * // Result: true
 * ```
 */
export const isValidKubernetesContent = (content: string): boolean => {
  const cleaned = content.trim();

  if (!cleaned) {
    return false;
  }

  try {
    const docs = parseYaml(cleaned, { strict: false });
    const documents = Array.isArray(docs) ? docs : [docs];

    return documents.some(
      (doc) =>
        doc &&
        typeof doc === 'object' &&
        'apiVersion' in doc &&
        'kind' in doc &&
        doc.apiVersion &&
        doc.kind,
    );
  } catch {
    return false;
  }
};
