/**
 * Unit tests for generation config schema validation
 */
import { describe, it, expect } from '@jest/globals';
import {
  dockerfileGenerationConfigSchema,
  k8sGenerationConfigSchema,
  resourceDefaultsSchema,
  orgStandardsSchema,
  generationConfigSchema,
  featureTogglesSchema,
  type DockerfileGenerationConfig,
  type K8sGenerationConfig,
} from '@/config/policy-generation-config';

describe('Policy Generation Config Schemas', () => {
  describe('dockerfileGenerationConfigSchema', () => {
    it('should validate complete Dockerfile config', () => {
      const validConfig: DockerfileGenerationConfig = {
        buildStrategy: 'multi-stage',
        baseImageCategory: 'distroless',
        optimizationPriority: 'security',
        securityFeatures: {
          nonRootUser: true,
          readOnlyRootFS: true,
          noNewPrivileges: true,
          dropCapabilities: true,
        },
        buildFeatures: {
          buildCache: true,
          layerOptimization: true,
          healthcheck: true,
        },
      };

      const result = dockerfileGenerationConfigSchema.safeParse(validConfig);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data).toEqual(validConfig);
      }
    });

    it('should validate minimal Dockerfile config (all optional)', () => {
      const minimalConfig = {};

      const result = dockerfileGenerationConfigSchema.safeParse(minimalConfig);
      expect(result.success).toBe(true);
    });

    it('should accept valid buildStrategy values', () => {
      const validStrategies = ['multi-stage', 'single-stage', 'distroless'];

      validStrategies.forEach((strategy) => {
        const config = { buildStrategy: strategy };
        const result = dockerfileGenerationConfigSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });

    it('should reject invalid buildStrategy', () => {
      const invalidConfig = {
        buildStrategy: 'invalid-strategy',
      };

      const result = dockerfileGenerationConfigSchema.safeParse(invalidConfig);
      expect(result.success).toBe(false);
    });

    it('should accept valid baseImageCategory values', () => {
      const validCategories = ['official', 'distroless', 'alpine', 'minimal'];

      validCategories.forEach((category) => {
        const config = { baseImageCategory: category };
        const result = dockerfileGenerationConfigSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });

    it('should reject invalid baseImageCategory', () => {
      const invalidConfig = {
        baseImageCategory: 'invalid-category',
      };

      const result = dockerfileGenerationConfigSchema.safeParse(invalidConfig);
      expect(result.success).toBe(false);
    });

    it('should accept valid optimizationPriority values', () => {
      const validPriorities = ['security', 'size', 'speed', 'balanced'];

      validPriorities.forEach((priority) => {
        const config = { optimizationPriority: priority };
        const result = dockerfileGenerationConfigSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });

    it('should reject invalid optimizationPriority', () => {
      const invalidConfig = {
        optimizationPriority: 'invalid-priority',
      };

      const result = dockerfileGenerationConfigSchema.safeParse(invalidConfig);
      expect(result.success).toBe(false);
    });

    it('should validate securityFeatures object', () => {
      const config = {
        securityFeatures: {
          nonRootUser: true,
          readOnlyRootFS: false,
          noNewPrivileges: true,
          dropCapabilities: false,
        },
      };

      const result = dockerfileGenerationConfigSchema.safeParse(config);
      expect(result.success).toBe(true);
    });

    it('should validate buildFeatures object', () => {
      const config = {
        buildFeatures: {
          buildCache: true,
          layerOptimization: true,
          healthcheck: false,
        },
      };

      const result = dockerfileGenerationConfigSchema.safeParse(config);
      expect(result.success).toBe(true);
    });
  });

  describe('resourceDefaultsSchema', () => {
    it('should validate valid resource strings', () => {
      const validResources = {
        cpuRequest: '500m',
        cpuLimit: '2',
        memoryRequest: '512Mi',
        memoryLimit: '2Gi',
      };

      const result = resourceDefaultsSchema.safeParse(validResources);
      expect(result.success).toBe(true);
    });

    it('should validate partial resources', () => {
      const partialResources = {
        cpuRequest: '500m',
        memoryRequest: '512Mi',
      };

      const result = resourceDefaultsSchema.safeParse(partialResources);
      expect(result.success).toBe(true);
    });

    it('should validate empty resources object', () => {
      const emptyResources = {};

      const result = resourceDefaultsSchema.safeParse(emptyResources);
      expect(result.success).toBe(true);
    });

    it('should accept various CPU formats', () => {
      const cpuFormats = ['100m', '500m', '1', '2', '4'];

      cpuFormats.forEach((cpu) => {
        const config = { cpuRequest: cpu };
        const result = resourceDefaultsSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });

    it('should accept various memory formats', () => {
      const memoryFormats = ['128Mi', '256Mi', '512Mi', '1Gi', '2Gi', '4Gi'];

      memoryFormats.forEach((memory) => {
        const config = { memoryRequest: memory };
        const result = resourceDefaultsSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });
  });

  describe('orgStandardsSchema', () => {
    it('should validate complete org standards', () => {
      const validStandards = {
        requiredLabels: {
          team: 'platform',
          costCenter: 'engineering',
          'app.kubernetes.io/name': 'myapp',
        },
        namespace: 'myapp-prod',
        allowedRegistries: ['docker.io', 'gcr.io', 'mcr.microsoft.com'],
        serviceAccount: 'my-service-account',
        imagePullPolicy: 'Always',
      };

      const result = orgStandardsSchema.safeParse(validStandards);
      expect(result.success).toBe(true);
    });

    it('should validate empty labels', () => {
      const standardsWithEmptyLabels = {
        requiredLabels: {},
      };

      const result = orgStandardsSchema.safeParse(standardsWithEmptyLabels);
      expect(result.success).toBe(true);
    });

    it('should validate minimal org standards', () => {
      const minimalStandards = {
        namespace: 'default',
      };

      const result = orgStandardsSchema.safeParse(minimalStandards);
      expect(result.success).toBe(true);
    });

    it('should accept valid imagePullPolicy values', () => {
      const validPolicies = ['Always', 'IfNotPresent', 'Never'];

      validPolicies.forEach((policy) => {
        const config = { imagePullPolicy: policy };
        const result = orgStandardsSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });

    it('should reject invalid imagePullPolicy', () => {
      const invalidStandards = {
        imagePullPolicy: 'Sometimes',
      };

      const result = orgStandardsSchema.safeParse(invalidStandards);
      expect(result.success).toBe(false);
    });
  });

  describe('featureTogglesSchema', () => {
    it('should validate all features enabled', () => {
      const allEnabled = {
        healthChecks: true,
        autoscaling: true,
        resourceQuotas: true,
        networkPolicies: true,
        podSecurityPolicies: true,
        ingress: true,
      };

      const result = featureTogglesSchema.safeParse(allEnabled);
      expect(result.success).toBe(true);
    });

    it('should validate all features disabled', () => {
      const allDisabled = {
        healthChecks: false,
        autoscaling: false,
        resourceQuotas: false,
        networkPolicies: false,
        podSecurityPolicies: false,
        ingress: false,
      };

      const result = featureTogglesSchema.safeParse(allDisabled);
      expect(result.success).toBe(true);
    });

    it('should validate partial features', () => {
      const partial = {
        healthChecks: true,
        autoscaling: false,
      };

      const result = featureTogglesSchema.safeParse(partial);
      expect(result.success).toBe(true);
    });

    it('should validate empty features object', () => {
      const empty = {};

      const result = featureTogglesSchema.safeParse(empty);
      expect(result.success).toBe(true);
    });
  });

  describe('k8sGenerationConfigSchema', () => {
    it('should validate complete K8s config', () => {
      const validConfig: K8sGenerationConfig = {
        resourceDefaults: {
          cpuRequest: '500m',
          cpuLimit: '1000m',
          memoryRequest: '512Mi',
          memoryLimit: '1Gi',
        },
        orgStandards: {
          requiredLabels: {
            'app.kubernetes.io/name': 'myapp',
            'app.kubernetes.io/environment': 'production',
          },
          namespace: 'myapp-prod',
          allowedRegistries: ['docker.io', 'gcr.io'],
          serviceAccount: 'default',
          imagePullPolicy: 'Always',
        },
        features: {
          healthChecks: true,
          autoscaling: true,
          resourceQuotas: false,
        },
        replicas: 3,
        deploymentStrategy: 'RollingUpdate',
      };

      const result = k8sGenerationConfigSchema.safeParse(validConfig);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data).toEqual(validConfig);
      }
    });

    it('should validate minimal K8s config', () => {
      const minimalConfig = {};

      const result = k8sGenerationConfigSchema.safeParse(minimalConfig);
      expect(result.success).toBe(true);
    });

    it('should accept valid replica counts', () => {
      const validReplicas = [1, 2, 3, 5, 10];

      validReplicas.forEach((replicas) => {
        const config = { replicas };
        const result = k8sGenerationConfigSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });

    it('should reject non-positive replicas', () => {
      const invalidReplicaCounts = [0, -1, -10];

      invalidReplicaCounts.forEach((replicas) => {
        const config = { replicas };
        const result = k8sGenerationConfigSchema.safeParse(config);
        expect(result.success).toBe(false);
      });
    });

    it('should reject non-integer replicas', () => {
      const invalidConfig = {
        replicas: 1.5,
      };

      const result = k8sGenerationConfigSchema.safeParse(invalidConfig);
      expect(result.success).toBe(false);
    });

    it('should accept valid deploymentStrategy values', () => {
      const validStrategies = ['RollingUpdate', 'Recreate'];

      validStrategies.forEach((strategy) => {
        const config = { deploymentStrategy: strategy };
        const result = k8sGenerationConfigSchema.safeParse(config);
        expect(result.success).toBe(true);
      });
    });

    it('should reject invalid deploymentStrategy', () => {
      const invalidConfig = {
        deploymentStrategy: 'BlueGreen',
      };

      const result = k8sGenerationConfigSchema.safeParse(invalidConfig);
      expect(result.success).toBe(false);
    });
  });

  describe('generationConfigSchema (unified)', () => {
    it('should validate complete unified config', () => {
      const completeConfig = {
        dockerfile: {
          buildStrategy: 'multi-stage' as const,
          baseImageCategory: 'distroless' as const,
          optimizationPriority: 'security' as const,
        },
        kubernetes: {
          resourceDefaults: {
            cpuRequest: '500m',
            memoryRequest: '512Mi',
          },
          replicas: 3,
          orgStandards: {
            requiredLabels: { env: 'prod' },
            namespace: 'myapp-prod',
          },
        },
      };

      const result = generationConfigSchema.safeParse(completeConfig);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.dockerfile?.buildStrategy).toBe('multi-stage');
        expect(result.data.kubernetes?.replicas).toBe(3);
      }
    });

    it('should validate Dockerfile-only config', () => {
      const dockerfileOnly = {
        dockerfile: {
          buildStrategy: 'multi-stage' as const,
        },
      };

      const result = generationConfigSchema.safeParse(dockerfileOnly);
      expect(result.success).toBe(true);
    });

    it('should validate K8s-only config', () => {
      const k8sOnly = {
        kubernetes: {
          replicas: 3,
        },
      };

      const result = generationConfigSchema.safeParse(k8sOnly);
      expect(result.success).toBe(true);
    });

    it('should validate empty config', () => {
      const emptyConfig = {};

      const result = generationConfigSchema.safeParse(emptyConfig);
      expect(result.success).toBe(true);
    });

    it('should validate complex nested config', () => {
      const complexConfig = {
        dockerfile: {
          buildStrategy: 'distroless' as const,
          baseImageCategory: 'distroless' as const,
          optimizationPriority: 'balanced' as const,
          securityFeatures: {
            nonRootUser: true,
            readOnlyRootFS: true,
          },
          buildFeatures: {
            buildCache: true,
            layerOptimization: true,
          },
        },
        kubernetes: {
          resourceDefaults: {
            cpuRequest: '1',
            cpuLimit: '2',
            memoryRequest: '1Gi',
            memoryLimit: '2Gi',
          },
          orgStandards: {
            requiredLabels: {
              'app.kubernetes.io/managed-by': 'containerization-assist',
              'app.kubernetes.io/environment': 'production',
            },
            namespace: 'production',
            allowedRegistries: ['docker.io', 'gcr.io', 'mcr.microsoft.com'],
            serviceAccount: 'default',
            imagePullPolicy: 'Always' as const,
          },
          features: {
            healthChecks: true,
            autoscaling: true,
            networkPolicies: true,
          },
          replicas: 5,
          deploymentStrategy: 'RollingUpdate' as const,
        },
      };

      const result = generationConfigSchema.safeParse(complexConfig);
      expect(result.success).toBe(true);
    });

    it('should reject invalid nested values', () => {
      const invalidConfig = {
        dockerfile: {
          buildStrategy: 'invalid-strategy',
        },
        kubernetes: {
          replicas: -1,
        },
      };

      const result = generationConfigSchema.safeParse(invalidConfig);
      expect(result.success).toBe(false);
    });
  });
});
