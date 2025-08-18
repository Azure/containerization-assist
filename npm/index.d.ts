// Type definitions for @thgamble/containerization-assist-mcp

export interface ToolResult {
  success: boolean;
  error?: string;
  data?: Record<string, any>;
  chain_hint?: {
    next_tool: string;
    reason: string;
  };
  raw?: string;
}

export interface AnalysisResult extends ToolResult {
  data?: {
    language?: string;
    framework?: string;
    dependencies?: string[];
    buildTool?: string;
    hasDockerfile?: boolean;
    hasKubernetesManifests?: boolean;
  };
}

export interface DockerfileResult extends ToolResult {
  data?: {
    dockerfilePath?: string;
    baseImage?: string;
    content?: string;
  };
}

export interface BuildResult extends ToolResult {
  data?: {
    imageId?: string;
    imageName?: string;
    tags?: string[];
    size?: number;
    buildTime?: number;
  };
}

export interface ScanResult extends ToolResult {
  data?: {
    vulnerabilities?: Array<{
      severity: string;
      cve: string;
      package?: string;
      version?: string;
      fixedVersion?: string;
      description?: string;
    }>;
    summary?: {
      critical: number;
      high: number;
      medium: number;
      low: number;
      total: number;
    };
    scanners?: string[];
  };
}

export interface PushResult extends ToolResult {
  data?: {
    registry?: string;
    repository?: string;
    tag?: string;
    digest?: string;
    pushedAt?: string;
  };
}

export interface ManifestResult extends ToolResult {
  data?: {
    manifests?: Array<{
      kind: string;
      name: string;
      path: string;
    }>;
    namespace?: string;
  };
}

export interface DeploymentResult extends ToolResult {
  data?: {
    deploymentName?: string;
    namespace?: string;
    replicas?: number;
    endpoints?: string[];
    status?: string;
  };
}

export interface ToolsListResult extends ToolResult {
  data?: {
    tools?: Array<{
      name: string;
      description: string;
      category: string;
    }>;
  };
}

// Common options
export interface BaseOptions {
  session_id?: string;
  [key: string]: any;
}

export interface BuildOptions extends BaseOptions {
  dockerfile?: string;
  context?: string;
  target?: string;
  buildArgs?: Record<string, string>;
  tags?: string[];
  nocache?: boolean;
}

export interface ScanOptions extends BaseOptions {
  scanners?: string[];
  severity?: string;
  ignoreUnfixed?: boolean;
}

export interface K8sOptions extends BaseOptions {
  namespace?: string;
  replicas?: number;
  port?: number;
  serviceType?: string;
}

// Tool functions
export function analyzeRepository(repoPath: string, options?: BaseOptions): Promise<AnalysisResult>;
export function generateDockerfile(options?: BaseOptions): Promise<DockerfileResult>;
export function buildImage(options?: BuildOptions): Promise<BuildResult>;
export function scanImage(options?: ScanOptions): Promise<ScanResult>;
export function tagImage(tag: string, options?: BaseOptions): Promise<ToolResult>;
export function pushImage(registry: string, options?: BaseOptions): Promise<PushResult>;
export function generateK8sManifests(options?: K8sOptions): Promise<ManifestResult>;
export function prepareCluster(options?: BaseOptions): Promise<ToolResult>;
export function deployApplication(options?: K8sOptions): Promise<DeploymentResult>;
export function verifyDeployment(options?: BaseOptions): Promise<DeploymentResult>;
export function listTools(): Promise<ToolsListResult>;
export function ping(): Promise<ToolResult>;
export function serverStatus(): Promise<ToolResult>;

// Helper function
export function createSession(): string;