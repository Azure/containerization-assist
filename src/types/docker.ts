/**
 * Docker API and container-related type definitions
 * Provides type safety for Docker daemon interactions and image operations
 */

// Basic Docker API types
export interface DockerAPIResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: string;
  statusCode?: number;
}

export interface DockerImageInfo {
  Id: string;
  ParentId: string;
  RepoTags: string[];
  RepoDigests: string[];
  Created: string;
  Size: number;
  VirtualSize: number;
  SharedSize: number;
  Labels: Record<string, string>;
  Containers: number;
  RootFS?: {
    Type: string;
    Layers: string[];
  };
  Config?: {
    Image?: string;
    Env?: string[];
    Cmd?: string[];
    Entrypoint?: string[];
    WorkingDir?: string;
    User?: string;
    ExposedPorts?: Record<string, Record<string, never>>;
    Labels?: Record<string, string>;
  };
}

export interface DockerImageManifest {
  schemaVersion: number;
  mediaType: string;
  config: {
    mediaType: string;
    size: number;
    digest: string;
  };
  layers: Array<{
    mediaType: string;
    size: number;
    digest: string;
  }>;
}

export interface DockerRegistryResponse {
  id?: string;
  repositories?: string[];
  tags?: string[];
  manifest?: DockerImageManifest;
  config?: DockerImageConfig;
  // Docker Hub API specific fields
  full_size?: number;
  size?: number;
  last_updated?: string;
  user?: string;
  namespace?: string;
  images?: Array<{
    architecture?: string;
    os?: string;
    created?: string;
    layers?: Array<{
      digest: string;
      size: number;
    }>;
  }>;
}

export interface DockerImageConfig {
  architecture: string;
  os: string;
  config?: {
    Env?: string[];
    Cmd?: string[];
    Entrypoint?: string[];
    WorkingDir?: string;
    User?: string;
    ExposedPorts?: Record<string, Record<string, never>>;
    Labels?: Record<string, string>;
  };
  rootfs?: {
    type: string;
    diff_ids: string[];
  };
  history?: Array<{
    created: string;
    created_by: string;
    empty_layer?: boolean;
  }>;
}

// Build-related types
export interface BuildProgress {
  stream?: string;
  status?: string;
  progress?: string;
  progressDetail?: {
    current: number;
    total: number;
  };
  id?: string;
  error?: string;
}

export interface BuildContext {
  dockerfile: string;
  context: string;
  buildArgs?: Record<string, string>;
  labels?: Record<string, string>;
  target?: string;
  platform?: string;
  nocache?: boolean;
}

export interface BuildResult {
  success: boolean;
  imageId?: string;
  imageTag: string;
  buildLog: string;
  error?: string;
  size?: number;
  warnings?: string[];
}

// Registry authentication
export interface RegistryAuth {
  username?: string;
  password?: string;
  email?: string;
  serveraddress?: string;
  auth?: string;
  identitytoken?: string;
  registrytoken?: string;
}

// Container inspection
export interface ContainerInspection {
  Id: string;
  Created: string;
  Path: string;
  Args: string[];
  State: {
    Status: string;
    Running: boolean;
    Paused: boolean;
    Restarting: boolean;
    OOMKilled: boolean;
    Dead: boolean;
    Pid: number;
    ExitCode: number;
    Error: string;
    StartedAt: string;
    FinishedAt: string;
  };
  Image: string;
  ResolvConfPath: string;
  HostnamePath: string;
  HostsPath: string;
  LogPath: string;
  Name: string;
  RestartCount: number;
  Driver: string;
  Platform: string;
  MountLabel: string;
  ProcessLabel: string;
  AppArmorProfile: string;
  Config: {
    Hostname: string;
    Domainname: string;
    User: string;
    AttachStdin: boolean;
    AttachStdout: boolean;
    AttachStderr: boolean;
    ExposedPorts: Record<string, Record<string, never>>;
    Tty: boolean;
    OpenStdin: boolean;
    StdinOnce: boolean;
    Env: string[];
    Cmd: string[];
    Image: string;
    Volumes: Record<string, Record<string, never>>;
    WorkingDir: string;
    Entrypoint: string[];
    Labels: Record<string, string>;
  };
  NetworkSettings: {
    Bridge: string;
    SandboxID: string;
    HairpinMode: boolean;
    LinkLocalIPv6Address: string;
    LinkLocalIPv6PrefixLen: number;
    Ports: Record<
      string,
      Array<{
        HostIp: string;
        HostPort: string;
      }>
    >;
    SandboxKey: string;
    SecondaryIPAddresses: unknown[];
    SecondaryIPv6Addresses: unknown[];
    EndpointID: string;
    Gateway: string;
    GlobalIPv6Address: string;
    GlobalIPv6PrefixLen: number;
    IPAddress: string;
    IPPrefixLen: number;
    IPv6Gateway: string;
    MacAddress: string;
    Networks: Record<
      string,
      {
        IPAMConfig?: {
          IPv4Address: string;
        };
        Links: string[];
        Aliases: string[];
        NetworkID: string;
        EndpointID: string;
        Gateway: string;
        IPAddress: string;
        IPPrefixLen: number;
        IPv6Gateway: string;
        GlobalIPv6Address: string;
        GlobalIPv6PrefixLen: number;
        MacAddress: string;
        DriverOpts: Record<string, string>;
      }
    >;
  };
}

// Image metadata (enhanced from existing)
export interface ImageMetadata {
  size: number;
  layers: number;
  architecture?: string;
  os?: string;
  created?: string;
  author?: string;
  config?: DockerImageConfig;
  labels?: Record<string, string>;
  digest?: string;
  mediaType?: string;
}

// Scanning and security
export interface SecurityScanResult {
  vulnerabilities: Array<{
    id: string;
    severity: 'critical' | 'high' | 'medium' | 'low' | 'negligible';
    title: string;
    description: string;
    fixedVersion?: string;
    installedVersion: string;
    package: string;
    link?: string;
  }>;
  summary: {
    total: number;
    critical: number;
    high: number;
    medium: number;
    low: number;
    negligible: number;
  };
  scanTime: string;
  scanner: string;
}

// Push operations
export interface PushProgress {
  status: string;
  progressDetail?: {
    current: number;
    total: number;
  };
  progress?: string;
  id?: string;
}

export interface PushResult {
  success: boolean;
  repository: string;
  tag: string;
  digest?: string;
  size?: number;
  error?: string;
  warnings?: string[];
}

// Tag operations
export interface TagResult {
  success: boolean;
  sourceImage: string;
  targetRepository: string;
  targetTag: string;
  error?: string;
}

// Docker daemon configuration
export interface DockerDaemonConfig {
  host?: string;
  port?: number;
  socketPath?: string;
  protocol?: 'http' | 'https' | 'unix';
  ca?: string;
  cert?: string;
  key?: string;
  timeout?: number;
}
