/**
 * Simple Kubernetes client mock for LLM integration tests
 */

export const KubeConfig = class {
  loadFromDefault() {}
  makeApiClient() {
    return {};
  }
};

export const CoreV1Api = class {};
export const AppsV1Api = class {};

export default {
  KubeConfig,
  CoreV1Api,
  AppsV1Api,
};