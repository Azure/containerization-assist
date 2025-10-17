/**
 * Unified content extraction utilities for AI-generated responses
 *
 * This module consolidates duplicated content extraction logic from across the codebase,
 * providing standardized methods to extract code blocks, YAML content, and other structured
 * data from AI responses.
 */

import * as yaml from 'js-yaml';

/**
 * Options for content extraction
 */
export interface ExtractionOptions {
  /** Language hint for code block extraction */
  language?: string;
  /** Whether to fall back to raw content if no code blocks are found */
  fallbackToRaw?: boolean;
  /** Whether to strip comments from extracted content */
  stripComments?: boolean;
  /** Whether to validate the extracted content */
  validate?: boolean;
}

/**
 * Result of content extraction
 */
export interface ExtractionResult<T = string> {
  /** Whether extraction was successful */
  success: boolean;
  /** Extracted content (if successful) */
  content?: T;
  /** Error message (if unsuccessful) */
  error?: string;
  /** Source of the extracted content */
  source: 'codeblock' | 'json' | 'yaml' | 'raw' | 'signature';
}

/**
 * Extract content from markdown code blocks
 * Handles various code block formats: ```language, ```, and JSON-embedded content
 */
export function extractCodeBlock(text: string, options: ExtractionOptions = {}): string | null {
  const { language, fallbackToRaw = true } = options;

  // Try language-specific code blocks first
  if (language) {
    const languagePattern = new RegExp(`\`\`\`${language}\\s*\\n([\\s\\S]*?)\`\`\``, 'i');
    const match = text.match(languagePattern);
    if (match?.[1]) {
      return match[1].trim();
    }
  }

  // Try generic code blocks
  const genericPattern = /```(?:\w+)?\s*\n([\s\S]*?)```/;
  const genericMatch = text.match(genericPattern);
  if (genericMatch?.[1]) {
    return genericMatch[1].trim();
  }

  // Try single-line code blocks
  const inlinePattern = /`([^`]+)`/;
  const inlineMatch = text.match(inlinePattern);
  if (inlineMatch?.[1] && inlineMatch[1].length > 10) {
    return inlineMatch[1].trim();
  }

  // Fallback to raw content if requested
  return fallbackToRaw ? text.trim() : null;
}

/**
 * Extract JSON content from text
 * Handles both standalone JSON and JSON embedded in text
 */
export function extractJsonContent(text: string): object | null {
  try {
    // Try parsing the entire text as JSON first
    return JSON.parse(text.trim());
  } catch {
    // Look for JSON code blocks
    const jsonBlock = extractCodeBlock(text, { language: 'json', fallbackToRaw: false });
    if (jsonBlock) {
      try {
        return JSON.parse(jsonBlock);
      } catch {
        // Ignore parse errors
      }
    }

    // Look for JSON-like structures in text
    const jsonPattern = /\{[\s\S]*\}/;
    const match = text.match(jsonPattern);
    if (match?.[0]) {
      try {
        return JSON.parse(match[0]);
      } catch {
        // Ignore parse errors
      }
    }

    return null;
  }
}

/**
 * Extract YAML content from text
 * Handles YAML code blocks and raw YAML content
 */
export function extractYamlContent(text: string): object | null {
  // Return null for empty input
  if (!text?.trim()) {
    return null;
  }

  // Try YAML code blocks first
  const yamlBlock = extractCodeBlock(text, { language: 'yaml', fallbackToRaw: false });
  if (yamlBlock) {
    try {
      const result = yaml.load(yamlBlock) as object;
      return result === undefined ? null : result;
    } catch {
      // Ignore parse errors
    }
  }

  // Try parsing the entire text as YAML
  try {
    const result = yaml.load(text.trim()) as object;
    return result === undefined ? null : result;
  } catch {
    return null;
  }
}

/**
 * Extract multiple YAML documents from text
 * Handles YAML document separators (---) and validates each document
 */
export function extractYamlDocuments(text: string): object[] {
  let content = text;

  // Extract from code blocks if present
  const yamlBlock = extractCodeBlock(content, { language: 'yaml', fallbackToRaw: false });
  if (yamlBlock) {
    content = yamlBlock;
  } else if (content.includes('```')) {
    // Remove any remaining code block markers
    content = content.replace(/```/g, '').trim();
  }

  // Split on YAML document separators
  const docs = content.split(/^---$/m).filter((doc) => doc.trim());
  const validDocs: object[] = [];

  for (const doc of docs) {
    try {
      const parsed = yaml.load(doc.trim()) as object;
      if (parsed && typeof parsed === 'object') {
        validDocs.push(parsed);
      }
    } catch {
      // Skip invalid documents
    }
  }

  return validDocs;
}

/**
 * Extract Dockerfile content from AI response
 * Uses signature-based detection and code block extraction
 */
export function extractDockerfileContent(text: string): ExtractionResult<string> {
  // Try code block extraction first
  const codeBlock = extractCodeBlock(text, { language: 'dockerfile', fallbackToRaw: false });
  if (codeBlock?.includes('FROM ')) {
    return {
      success: true,
      content: codeBlock,
      source: 'codeblock',
    };
  }

  // Try JSON extraction
  const jsonContent = extractJsonContent(text);
  if (jsonContent && typeof jsonContent === 'object' && 'dockerfile' in jsonContent) {
    const dockerfile = (jsonContent as any).dockerfile;
    if (typeof dockerfile === 'string') {
      // Unescape JSON string content
      const unescaped = dockerfile.replace(/\\n/g, '\n').replace(/\\"/g, '"').trim();
      return {
        success: true,
        content: unescaped,
        source: 'json',
      };
    }
  }

  // Try signature-based detection (FROM keyword)
  const fromMatch = text.match(/(FROM\s+[\s\S]*)/);
  if (fromMatch?.[1]) {
    let dockerContent = fromMatch[1];

    // Try to find natural end of Dockerfile
    const endMatch = dockerContent.match(/([\s\S]*?(?:CMD|ENTRYPOINT|EXPOSE)[^\n]*)/);
    if (endMatch?.[1]) {
      dockerContent = endMatch[1];
    }

    return {
      success: true,
      content: dockerContent.trim(),
      source: 'signature',
    };
  }

  // Fallback to raw content if it looks like a Dockerfile
  if (text.includes('RUN ') || text.includes('COPY ') || text.includes('WORKDIR ')) {
    return {
      success: true,
      content: text.trim(),
      source: 'raw',
    };
  }

  return {
    success: false,
    error: 'No Dockerfile content found in response',
    source: 'raw',
  };
}

/**
 * Extract Kubernetes manifest content from AI response
 * Handles multiple manifests and YAML validation
 */
export function extractKubernetesContent(text: string): ExtractionResult<object[]> {
  try {
    const manifests = extractYamlDocuments(text);

    // Validate that extracted documents look like Kubernetes manifests
    const validManifests = manifests.filter((manifest) => {
      return (
        manifest && typeof manifest === 'object' && 'apiVersion' in manifest && 'kind' in manifest
      );
    });

    if (validManifests.length > 0) {
      return {
        success: true,
        content: validManifests,
        source: 'yaml',
      };
    }

    return {
      success: false,
      error: 'No valid Kubernetes manifests found in response',
      source: 'yaml',
    };
  } catch (error) {
    return {
      success: false,
      error: `YAML parsing error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      source: 'yaml',
    };
  }
}

/**
 * Extract Helm chart content from AI response
 * Handles Chart.yaml, values.yaml, and template files
 */
export function extractHelmContent(text: string): ExtractionResult<Record<string, string>> {
  const files: Record<string, string> = {};

  try {
    // Try to extract as structured JSON first
    const jsonContent = extractJsonContent(text);
    if (jsonContent && typeof jsonContent === 'object') {
      // Handle structured Helm chart response
      for (const [key, value] of Object.entries(jsonContent)) {
        if (typeof value === 'string' && key.includes('.yaml')) {
          files[key] = value;
        }
      }

      if (Object.keys(files).length > 0) {
        return {
          success: true,
          content: files,
          source: 'json',
        };
      }
    }

    // Try YAML code block extraction
    const yamlContent = extractCodeBlock(text, { language: 'yaml', fallbackToRaw: false });
    if (yamlContent) {
      // Determine file type based on content
      const parsed = yaml.load(yamlContent) as any;
      if (parsed && typeof parsed === 'object') {
        if ('apiVersion' in parsed && 'name' in parsed && 'version' in parsed) {
          files['Chart.yaml'] = yamlContent;
        } else if ('replicaCount' in parsed || 'image' in parsed) {
          files['values.yaml'] = yamlContent;
        } else {
          files['template.yaml'] = yamlContent;
        }

        return {
          success: true,
          content: files,
          source: 'codeblock',
        };
      }
    }

    // Fallback to raw YAML parsing
    const yamlDocs = extractYamlDocuments(text);
    if (yamlDocs.length > 0) {
      yamlDocs.forEach((doc, index) => {
        const yamlString = yaml.dump(doc);
        files[`chart-${index + 1}.yaml`] = yamlString;
      });

      return {
        success: true,
        content: files,
        source: 'yaml',
      };
    }

    return {
      success: false,
      error: 'No Helm chart content found in response',
      source: 'raw',
    };
  } catch (error) {
    return {
      success: false,
      error: `Helm chart extraction error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      source: 'raw',
    };
  }
}

/**
 * Universal content extraction function that detects content type and applies appropriate extractor
 * @param text Text to extract content from
 * @param contentType Optional hint about the content type
 * @returns Extraction result
 */
export function extractContent(
  text: string,
  contentType?: string,
): ExtractionResult<string | object | object[] | Record<string, string>> {
  if (!text?.trim()) {
    return {
      success: false,
      error: 'Empty input text',
      source: 'raw',
    };
  }

  // Use hint if provided
  if (contentType) {
    switch (contentType.toLowerCase()) {
      case 'dockerfile':
        return extractDockerfileContent(text);
      case 'kubernetes':
      case 'k8s':
        return extractKubernetesContent(text);
      case 'helm':
        return extractHelmContent(text);
      case 'yaml': {
        const yamlResult = extractYamlContent(text);
        if (yamlResult !== null) {
          return {
            success: true,
            content: yamlResult,
            source: 'yaml',
          };
        } else {
          return {
            success: false,
            error: 'Invalid YAML content',
            source: 'yaml',
          };
        }
      }
      case 'json': {
        const jsonResult = extractJsonContent(text);
        if (jsonResult !== null) {
          return {
            success: true,
            content: jsonResult,
            source: 'json',
          };
        } else {
          return {
            success: false,
            error: 'Invalid JSON content',
            source: 'json',
          };
        }
      }
    }
  }

  // Auto-detect content type
  if (text.includes('FROM ') && (text.includes('RUN ') || text.includes('COPY '))) {
    return extractDockerfileContent(text);
  }

  if (text.includes('apiVersion:') && text.includes('kind:')) {
    return extractKubernetesContent(text);
  }

  if (
    (text.includes('{{') && text.includes('}}')) ||
    text.includes('Chart.yaml') ||
    text.includes('values.yaml')
  ) {
    return extractHelmContent(text);
  }

  // Default to code block extraction
  const codeBlock = extractCodeBlock(text);
  if (codeBlock) {
    return {
      success: true,
      content: codeBlock,
      source: 'codeblock',
    };
  }

  return {
    success: true,
    content: text.trim(),
    source: 'raw',
  };
}
