/**
 * SDK Entry Point Tests
 *
 * Tests for SDK exports and that all expected functions and types are available.
 */

import { describe, test, expect } from '@jest/globals';
import {
  // All 11 tool functions
  analyzeRepo,
  generateDockerfile,
  fixDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  generateK8sManifests,
  prepareCluster,
  verifyDeploy,
  ops,
  // Advanced
  tools,
  executeTool,
  Success,
  Failure,
} from '../../../src/sdk';

describe('SDK Exports', () => {
  describe('Function exports (all 11 tools)', () => {
    test('exports analyzeRepo function', () => {
      expect(typeof analyzeRepo).toBe('function');
    });

    test('exports generateDockerfile function', () => {
      expect(typeof generateDockerfile).toBe('function');
    });

    test('exports fixDockerfile function', () => {
      expect(typeof fixDockerfile).toBe('function');
    });

    test('exports buildImage function', () => {
      expect(typeof buildImage).toBe('function');
    });

    test('exports scanImage function', () => {
      expect(typeof scanImage).toBe('function');
    });

    test('exports tagImage function', () => {
      expect(typeof tagImage).toBe('function');
    });

    test('exports pushImage function', () => {
      expect(typeof pushImage).toBe('function');
    });

    test('exports generateK8sManifests function', () => {
      expect(typeof generateK8sManifests).toBe('function');
    });

    test('exports prepareCluster function', () => {
      expect(typeof prepareCluster).toBe('function');
    });

    test('exports verifyDeploy function', () => {
      expect(typeof verifyDeploy).toBe('function');
    });

    test('exports ops function', () => {
      expect(typeof ops).toBe('function');
    });

    test('exports executeTool function', () => {
      expect(typeof executeTool).toBe('function');
    });
  });

  describe('Tools object (all 11 tools)', () => {
    test('exports tools.analyzeRepo', () => {
      expect(tools.analyzeRepo).toBeDefined();
      expect(tools.analyzeRepo.name).toBe('analyze-repo');
    });

    test('exports tools.generateDockerfile', () => {
      expect(tools.generateDockerfile).toBeDefined();
      expect(tools.generateDockerfile.name).toBe('generate-dockerfile');
    });

    test('exports tools.fixDockerfile', () => {
      expect(tools.fixDockerfile).toBeDefined();
      expect(tools.fixDockerfile.name).toBe('fix-dockerfile');
    });

    test('exports tools.buildImage', () => {
      expect(tools.buildImage).toBeDefined();
      expect(tools.buildImage.name).toBe('build-image');
    });

    test('exports tools.scanImage', () => {
      expect(tools.scanImage).toBeDefined();
      expect(tools.scanImage.name).toBe('scan-image');
    });

    test('exports tools.tagImage', () => {
      expect(tools.tagImage).toBeDefined();
      expect(tools.tagImage.name).toBe('tag-image');
    });

    test('exports tools.pushImage', () => {
      expect(tools.pushImage).toBeDefined();
      expect(tools.pushImage.name).toBe('push-image');
    });

    test('exports tools.generateK8sManifests', () => {
      expect(tools.generateK8sManifests).toBeDefined();
      expect(tools.generateK8sManifests.name).toBe('generate-k8s-manifests');
    });

    test('exports tools.prepareCluster', () => {
      expect(tools.prepareCluster).toBeDefined();
      expect(tools.prepareCluster.name).toBe('prepare-cluster');
    });

    test('exports tools.verifyDeploy', () => {
      expect(tools.verifyDeploy).toBeDefined();
      expect(tools.verifyDeploy.name).toBe('verify-deploy');
    });

    test('exports tools.ops', () => {
      expect(tools.ops).toBeDefined();
      expect(tools.ops.name).toBe('ops');
    });

    test('exports exactly 11 tools', () => {
      expect(Object.keys(tools)).toHaveLength(11);
    });
  });

  describe('Result type constructors', () => {
    test('exports Success constructor', () => {
      const result = Success({ value: 'test' });
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ value: 'test' });
      }
    });

    test('exports Failure constructor', () => {
      const result = Failure('error message');
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('error message');
      }
    });

    test('Failure supports guidance', () => {
      const result = Failure('error', {
        message: 'Error occurred',
        hint: 'Try this',
        resolution: 'Do that',
      });
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.guidance?.hint).toBe('Try this');
        expect(result.guidance?.resolution).toBe('Do that');
      }
    });
  });

  describe('Tool metadata', () => {
    test('all tools have descriptions', () => {
      Object.values(tools).forEach((tool) => {
        expect(tool.description).toBeDefined();
        expect(typeof tool.description).toBe('string');
        expect(tool.description.length).toBeGreaterThan(0);
      });
    });

    test('all tools have schemas', () => {
      Object.values(tools).forEach((tool) => {
        expect(tool.schema).toBeDefined();
        expect(tool.inputSchema).toBeDefined();
      });
    });

    test('all tools have handlers', () => {
      Object.values(tools).forEach((tool) => {
        expect(tool.handler).toBeDefined();
        expect(typeof tool.handler).toBe('function');
      });
    });

    test('all tools have metadata', () => {
      Object.values(tools).forEach((tool) => {
        expect(tool.metadata).toBeDefined();
        expect(typeof tool.metadata.knowledgeEnhanced).toBe('boolean');
      });
    });
  });
});

describe('SDK Input Validation', () => {
  test('analyzeRepo validates required repositoryPath', async () => {
    // @ts-expect-error - intentionally passing invalid input
    const result = await analyzeRepo({});

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('Validation failed');
    }
  });

  test('ops validates required operation', async () => {
    // @ts-expect-error - intentionally passing invalid input
    const result = await ops({});

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('Validation failed');
    }
  });

  test('ops accepts valid ping operation', async () => {
    const result = await ops({ operation: 'ping' });

    // Should succeed (ping doesn't require external services)
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value).toHaveProperty('message');
    }
  });
});
