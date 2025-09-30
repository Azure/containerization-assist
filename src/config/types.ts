/**
 * Configuration Types
 */

// Type aliases for better type safety
type NodeEnv = 'development' | 'production' | 'test';
type LogLevel = 'error' | 'warn' | 'info' | 'debug' | 'trace';
type WorkflowMode = 'interactive' | 'auto' | 'batch';
type StoreType = 'memory' | 'file' | 'redis';

// Extended configuration interface to match current usage
export interface ApplicationConfig {
  logLevel: LogLevel;
  workspaceDir: string;
  server: {
    nodeEnv: NodeEnv;
    logLevel: LogLevel;
    port: number;
    host: string;
  };
  session: {
    store: StoreType;
    persistencePath: string;
    persistenceInterval: number;
    cleanupInterval: number;
  };
  mcp: {
    name: string;
    version: string;
    storePath: string;
    enableMetrics: boolean;
    enableEvents: boolean;
  };
  docker: {
    socketPath: string;
    host: string;
    port: number;
    registry: string;
    timeout: number;
    buildArgs: Record<string, string>;
  };
  kubernetes: {
    namespace: string;
    kubeconfig: string;
    timeout: number;
  };
  workspace: {
    workspaceDir: string;
    tempDir: string;
    cleanupOnExit: boolean;
  };
  logging: {
    level: LogLevel;
    format: string;
  };
  workflow: {
    mode: WorkflowMode;
  };
}
