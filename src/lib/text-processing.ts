/**
 * Text Processing Utilities
 *
 * Utility functions for processing AI responses and text content,
 * particularly for cleaning up code generation responses.
 */

// Dynamic import to avoid webpack warnings with vscode-languageserver-types
let DockerfileParser: any;
import { parse as parseYaml } from 'yaml';

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
  // Build regex pattern based on language parameter
  let pattern: RegExp;

  if (language) {
    // Match specific language or its variations
    const langPattern = language.toLowerCase();
    if (langPattern === 'dockerfile' || langPattern === 'docker') {
      pattern = /```(?:docker|dockerfile|Dockerfile|DOCKERFILE)?\s*\n([\s\S]*?)```/;
    } else if (langPattern === 'yaml' || langPattern === 'yml') {
      pattern = /```(?:yaml|yml|YAML|YML)?\s*\n([\s\S]*?)```/;
    } else {
      // Generic pattern for other languages
      pattern = new RegExp(`\`\`\`(?:${langPattern})?\\s*\\n([\\s\\S]*?)\`\`\``);
    }
  } else {
    // Generic pattern that matches any code fence (including empty ones)
    pattern = /```[a-zA-Z0-9]*\s*\n?([\s\S]*?)```/;
  }

  const match = text.match(pattern);
  if (match) {
    // Found a code fence, return the content (which might be empty)
    return match[1] ? match[1].trim() : '';
  }

  // No code fence found, return trimmed original text
  // Callers should validate if the content is what they expect
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
export const isValidDockerfileContent = async (content: string): Promise<boolean> => {
  const cleaned = content.trim();

  if (!cleaned) {
    return false;
  }

  try {
    if (!DockerfileParser) {
      const { DockerfileParser: Parser } = await import('dockerfile-ast');
      DockerfileParser = Parser;
    }

    const dockerfile = DockerfileParser.parse(cleaned);
    const instructions = dockerfile.getInstructions();

    // Check if there's at least one FROM instruction
    return instructions.some((instruction: any) => instruction.getKeyword() === 'FROM');
  } catch {
    // Failed to parse as valid Dockerfile
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
export const extractBaseImage = async (dockerfileContent: string): Promise<string | null> => {
  try {
    if (!DockerfileParser) {
      const { DockerfileParser: Parser } = await import('dockerfile-ast');
      DockerfileParser = Parser;
    }

    const dockerfile = DockerfileParser.parse(dockerfileContent);
    const instructions = dockerfile.getInstructions();

    // Find the first FROM instruction
    const fromInstruction = instructions.find(
      (instruction: any) => instruction.getKeyword() === 'FROM',
    );

    if (fromInstruction) {
      // Get the image argument (first argument of FROM)
      const imageArg = fromInstruction.getArguments()[0];
      return imageArg?.getValue() ?? null;
    }

    return null;
  } catch {
    // Failed to parse, fallback to regex for backwards compatibility
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
    // Parse as YAML - parseYaml handles both single and multi-document YAML
    const docs = parseYaml(cleaned, { strict: false });

    // Handle both single document and array of documents
    const documents = Array.isArray(docs) ? docs : [docs];

    // Check if at least one document has apiVersion and kind
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
    // Failed to parse as valid YAML
    return false;
  }
};
