export interface TestRepository {
  name: string;
  type: string;
  path: string;
  language: string;
  framework?: string;
  hasDockerfile?: boolean;
  hasK8sManifests?: boolean;
  complexity: 'simple' | 'moderate' | 'complex';
  description: string;
  expectedFeatures?: string[];
  securityIssues?: string[];
}

export interface RepositoryAnalysisExpectation {
  language: string;
  framework?: string;
  buildTool?: string;
  packageManager?: string;
  entryPoints: string[];
  dependencies: string[];
  ports: number[];
  environment?: Record<string, string>;
}

export interface DockerfileExpectation {
  baseImage: string;
  workdir: string;
  exposedPorts: number[];
  hasMultiStage?: boolean;
  hasHealthCheck?: boolean;
  hasNonRootUser?: boolean;
}

export interface K8sManifestExpectation {
  hasDeployment: boolean;
  hasService: boolean;
  hasConfigMap?: boolean;
  hasSecret?: boolean;
  hasIngress?: boolean;
  replicas?: number;
}

export interface TestRepositoryExpectation {
  analysis: RepositoryAnalysisExpectation;
  dockerfile: DockerfileExpectation;
  k8sManifests: K8sManifestExpectation;
  buildShouldSucceed: boolean;
  estimatedBuildTimeMs?: number;
}

export interface TestRepositoryConfig {
  repository: TestRepository;
  expectation: TestRepositoryExpectation;
}