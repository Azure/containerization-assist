import { Result } from '@types';

export type ResourceCategory =
  | 'dockerfile'
  | 'k8s-manifest'
  | 'scan-result'
  | 'build-artifact'
  | 'deployment-status'
  | 'session-data'
  | 'sampling-result'
  | 'sampling-variant'
  | 'sampling-config';

export interface Resource {
  uri: string;
  content: unknown;
  mimeType: string;
  createdAt: Date;
  expiresAt?: Date;
  metadata?: {
    size?: number;
    scheme?: string;
    category?: ResourceCategory;
    [key: string]: unknown;
  };
  // Enhanced MCP-native fields
  name?: string;
  description?: string;
  annotations?: {
    audience?: string[];
    priority?: number;
    tags?: string[];
  };
}

export interface ResourceCache {
  set(key: string, value: unknown, ttl?: number): Promise<Result<void>>;
  get(key: string): Promise<Result<unknown>>;
  delete(key: string): Promise<Result<boolean>>;
  clear(): Promise<Result<void>>;
  has(key: string): Promise<Result<boolean>>;
  invalidate(pattern: string | { tags?: string[]; keyPattern?: string }): Promise<Result<number>>;
  keys(pattern?: string): string[];
}

export const URI_SCHEMES = {
  MCP: 'mcp',
  CACHE: 'cache',
  SESSION: 'session',
  TEMP: 'temp',
  SAMPLING: 'sampling',
} as const;

export type UriScheme = (typeof URI_SCHEMES)[keyof typeof URI_SCHEMES];

export interface ParsedUri {
  scheme: UriScheme;
  path: string;
  query?: Record<string, string>;
  fragment?: string;
}
