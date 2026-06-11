import type { z } from 'zod';
import type { ToolCategory } from '@/types/categories';
import type { ToolMetadata } from '@/types/tool-metadata';
import type { ChainHints } from '@/types/tool';

export const TOOL_NAME = {
  ANALYZE_REPO: 'analyze-repo',
  BUILD_IMAGE_CONTEXT: 'build-image-context',
  FIX_DOCKERFILE: 'fix-dockerfile',
  GENERATE_DOCKERFILE: 'generate-dockerfile',
  GENERATE_GITHUB_WORKFLOW: 'generate-github-workflow',
  GENERATE_K8S_MANIFESTS: 'generate-k8s-manifests',
  OPS: 'ops',
  PREPARE_CLUSTER: 'prepare-cluster',
  PUSH_IMAGE: 'push-image',
  SCAN_IMAGE: 'scan-image',
  TAG_IMAGE: 'tag-image',
  VERIFY_DEPLOY: 'verify-deploy',
} as const;

/** Names for workflow tools registered directly on the MCP server (not orchestrated). */
export const WORKFLOW_TOOL_NAME = {
  CREATE_POLICY: 'create-containerization-policy',
  KIND_LOOP: 'kind-loop',
  AKS_LOOP: 'aks-loop',
} as const;

export type ToolName = (typeof TOOL_NAME)[keyof typeof TOOL_NAME];

/**
 * Tool definition containing metadata without the handler implementation.
 * This is a lightweight interface for importing tool definitions without
 * pulling in the heavy handler implementations and their dependencies.
 */
export interface IToolDefinition {
  /** Unique tool identifier */
  name: ToolName;
  /** Human-readable description */
  description: string;
  /** Tool category for organization and grouping */
  category?: ToolCategory;
  /** Optional semantic version */
  version?: string;
  /** Zod schema for validation */
  schema: z.ZodTypeAny;
  /** Tool metadata for AI enhancement tracking */
  metadata: ToolMetadata;
  /** Optional workflow guidance hints for tool chaining */
  chainHints?: ChainHints;
}
