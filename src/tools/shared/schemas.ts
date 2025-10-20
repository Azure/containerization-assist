/**
 * Shared Zod schemas for tool parameters
 * Common building blocks to reduce duplication across tools
 */

import { z } from 'zod';
import { environmentSchema } from '@/config/constants';

// Paths
export const repositoryPath = z
  .string()
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

// Platform
export const platform = z.string().optional().describe('Target platform (e.g., linux/amd64)');

// Multi-module/monorepo support
