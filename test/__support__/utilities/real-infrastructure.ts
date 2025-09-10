/**
 * Real Infrastructure Helper
 * Provides real infrastructure connections for integration tests
 */

export function createRealInfrastructure(testEnvironment: any) {
  return {
    docker: testEnvironment.dockerClient,
    kubernetes: testEnvironment.kubernetesClient,
    cleanup: async () => {
      // Clean up test artifacts
      if (testEnvironment.cleanup) {
        await testEnvironment.cleanup();
      }
    },
  };
}

export {};