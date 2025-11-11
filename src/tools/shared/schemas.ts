/**
 * Shared Zod schemas for tool parameters
 * Common building blocks to reduce duplication across tools
 */

import { z } from 'zod';
import { environmentSchema } from '@/config/constants';

/**
 * Supported Docker platforms for multi-architecture builds
 * See: https://docs.docker.com/build/building/multi-platform/
 */
export const DOCKER_PLATFORMS = [
  'linux/amd64',
  'linux/arm64',
  'linux/arm/v7',
  'linux/arm/v6',
  'linux/386',
  'linux/ppc64le',
  'linux/s390x',
  'linux/riscv64',
  'windows/amd64',
] as const;

export type DockerPlatform = (typeof DOCKER_PLATFORMS)[number];

// Paths
export const repositoryPath = z
  .string()
  .min(1, 'Repository path cannot be empty')
  .describe(
    'Absolute path to the repository. Paths are automatically normalized to forward slashes on all platforms (e.g., /path/to/repo or C:/path/to/repo)',
  );

export const namespaceOptional = z.string().optional().describe('Kubernetes namespace');

// Environment schema
export const environment = environmentSchema.optional();

// Docker image fields
export const imageName = z.string().describe('Name for the Docker image');
export const tags = z.array(z.string()).describe('Tags to apply to the image');
export const buildArgs = z.record(z.string(), z.string()).describe('Build arguments');

// Application basics
export const replicas = z.number().optional().describe('Number of replicas');
export const port = z.number().optional().describe('Application port');

// Service types

// Ingress

// Health checks

// Autoscaling

// Analysis options
export const analysisOptions = {
  depth: z.number().optional().describe('Analysis depth (1-5)'),
  includeTests: z.boolean().optional().describe('Include test files in analysis'),
  securityFocus: z.boolean().optional().describe('Focus on security aspects'),
  performanceFocus: z.boolean().optional().describe('Focus on performance aspects'),
};

// Platform - Required for consistent builds across environments
export const platform = z
  .enum(DOCKER_PLATFORMS)
  .default('linux/amd64')
  .describe(
    'Target platform for the Docker image (e.g., "linux/amd64", "linux/arm64"). Defaults to linux/amd64 for maximum compatibility. Required to ensure consistent builds across environments and cross-platform scenarios (e.g., ARM Mac targeting AMD64 servers).',
  );

// Multi-module/monorepo support

/**
 * File action descriptor for tool next actions
 */
export interface ToolNextActionFile {
  /** Relative path where file should be created/updated (e.g., "./Dockerfile" or "./k8s/deployment.yaml") */
  path: string;
  /** Human-readable purpose of this file (e.g., "Container build configuration") */
  purpose: string;
}

/**
 * Next action directive for tools that generate file content
 * Provides explicit guidance to AI about what files to create/update
 */
export interface ToolNextAction {
  /** Type of action to perform */
  action: 'create-files' | 'update-files' | 'review-and-decide';
  /** Clear instruction for the AI on what to do with the recommendations */
  instruction: string;
  /** Files to create or update */
  files: ToolNextActionFile[];
}
