/**
 * Mock Kubernetes client for LLM integration tests
 * Provides minimal stubs to avoid import issues
 */

// Mock the main kubernetes client exports
export const KubeConfig = class {
  loadFromDefault() {}
  makeApiClient() {
    return {};
  }
};

export const AppsV1Api = class {
  constructor() {}
};

export const CoreV1Api = class {
  constructor() {}
};

export const NetworkingV1Api = class {
  constructor() {}
};

// Mock common kubernetes utilities
export const loadYaml = () => ({});
export const dumpYaml = () => '';

// Default export for compatibility
export default {
  KubeConfig,
  AppsV1Api,
  CoreV1Api,
  NetworkingV1Api,
  loadYaml,
  dumpYaml,
};

// Mock any other exports that might be used
export * from './kubernetes-mock';