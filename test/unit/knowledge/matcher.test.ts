import { findKnowledgeMatches } from '@/knowledge/matcher';
import type { LoadedEntry, KnowledgeQuery } from '@/knowledge/types';

describe('findKnowledgeMatches', () => {
  let sampleEntries: LoadedEntry[];

  beforeEach(() => {
    sampleEntries = [
      {
        id: 'node-alpine',
        category: 'dockerfile',
        pattern: 'node:(?!.*alpine)',
        recommendation: 'Use node:alpine for smaller images',
        severity: 'high',
        tags: ['node', 'alpine', 'optimization'],
      },
      {
        id: 'k8s-resources',
        category: 'kubernetes',
        pattern: 'kind:\\s*Deployment',
        recommendation: 'Set resource limits',
        severity: 'high',
        tags: ['resources', 'limits'],
      },
      {
        id: 'user-security',
        category: 'security',
        pattern: 'USER\\s+(root|0)',
        recommendation: 'Avoid running as root',
        severity: 'high',
        tags: ['security', 'user'],
      },
    ];
  });

  describe('finding matches', () => {
    test('should return matches sorted by score', () => {
      const query: KnowledgeQuery = {
        category: 'dockerfile',
        text: 'FROM node:16',
      };

      const matches = findKnowledgeMatches(sampleEntries, query);

      expect(matches.length).toBeGreaterThan(0);
      expect(matches[0].entry.id).toBe('node-alpine');
      expect(matches[0].score).toBeGreaterThan(0);
    });

    test('should filter by category', () => {
      const query: KnowledgeQuery = {
        category: 'kubernetes',
      };

      const matches = findKnowledgeMatches(sampleEntries, query);

      expect(matches).toHaveLength(1);
      expect(matches[0].entry.category).toBe('kubernetes');
    });

    test('should match patterns in text', () => {
      const query: KnowledgeQuery = {
        text: 'kind: Deployment',
      };

      const matches = findKnowledgeMatches(sampleEntries, query);

      const k8sMatch = matches.find((m) => m.entry.id === 'k8s-resources');
      expect(k8sMatch).toBeDefined();
      expect(k8sMatch!.score).toBeGreaterThan(0);
    });

    test('should respect limit parameter', () => {
      const query: KnowledgeQuery = {
        limit: 1,
      };

      const matches = findKnowledgeMatches(sampleEntries, query);

      expect(matches).toHaveLength(1);
    });

    test('should match by tags', () => {
      const query: KnowledgeQuery = {
        tags: ['security'],
      };

      const matches = findKnowledgeMatches(sampleEntries, query);

      expect(matches.length).toBeGreaterThan(0);
      expect(matches[0].entry.tags).toContain('security');
    });

    test('should provide reasons for matches', () => {
      const query: KnowledgeQuery = {
        category: 'dockerfile',
        text: 'FROM node:16',
        tags: ['optimization'],
      };

      const matches = findKnowledgeMatches(sampleEntries, query);

      expect(matches[0].reasons.length).toBeGreaterThan(0);
      expect(matches[0].reasons.some((r) => r.includes('Category'))).toBe(true);
    });

    test('should handle language context', () => {
      const query: KnowledgeQuery = {
        language: 'javascript',
        text: 'FROM node:16',
      };

      const matches = findKnowledgeMatches(sampleEntries, query);

      const nodeMatch = matches.find((m) => m.entry.tags?.includes('node'));
      expect(nodeMatch?.score).toBeGreaterThan(0);
    });

    test('should exclude entries with conflicting language tags', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'java-maven',
          category: 'dockerfile',
          pattern: 'mvn',
          recommendation: 'Use Maven for Java builds',
          tags: ['java', 'maven'],
        },
        {
          id: 'python-pip',
          category: 'dockerfile',
          pattern: 'pip',
          recommendation: 'Use pip for Python packages',
          tags: ['python', 'pip'],
        },
        {
          id: 'generic-security',
          category: 'dockerfile',
          pattern: 'USER',
          recommendation: 'Use non-root user',
          tags: ['security'],
        },
      ];

      const query: KnowledgeQuery = {
        language: 'java',
        category: 'dockerfile',
      };

      const matches = findKnowledgeMatches(entries, query);

      // Should include Java entry
      expect(matches.some((m) => m.entry.id === 'java-maven')).toBe(true);

      // Should exclude Python entry (conflicting language tag)
      expect(matches.some((m) => m.entry.id === 'python-pip')).toBe(false);

      // Should include generic entry (no language tag)
      expect(matches.some((m) => m.entry.id === 'generic-security')).toBe(true);
    });

    test('should handle empty query gracefully', () => {
      const query: KnowledgeQuery = {};

      const matches = findKnowledgeMatches(sampleEntries, query);

      // Should return all entries sorted by severity/score
      expect(matches.length).toBe(sampleEntries.length);
    });

    test('should handle invalid patterns gracefully', () => {
      const entriesWithInvalidPattern: KnowledgeEntry[] = [
        {
          id: 'invalid-pattern',
          category: 'dockerfile',
          pattern: '[unclosed', // Invalid regex
          recommendation: 'Test recommendation',
        },
      ];

      const query: KnowledgeQuery = {
        text: 'some text',
      };

      // Should not throw error
      expect(() => {
        findKnowledgeMatches(entriesWithInvalidPattern, query);
      }).not.toThrow();
    });
  });

  describe('context evaluation', () => {
    test('should boost score for environment match', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'prod-optimized',
          category: 'dockerfile',
          pattern: 'FROM',
          recommendation: 'Use production optimizations',
          tags: ['production', 'alpine'],
        },
      ];

      const query: KnowledgeQuery = {
        environment: 'production',
        text: 'FROM node:16',
      };

      const matches = findKnowledgeMatches(entries, query);

      expect(matches[0].score).toBeGreaterThan(0); // Should have environment boost
      expect(matches[0].reasons.some((r) => r.includes('Environment'))).toBe(true);
    });

    test('should boost score for framework match', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'express-optimized',
          category: 'dockerfile',
          pattern: 'FROM',
          recommendation: 'Express optimization',
          tags: ['express', 'node'],
        },
      ];

      const query: KnowledgeQuery = {
        framework: 'express',
        text: 'FROM node:16',
      };

      const matches = findKnowledgeMatches(entries, query);

      expect(matches[0].reasons.some((r) => r.includes('Framework'))).toBe(true);
    });
  });

  describe('tag normalization', () => {
    test('should normalize build tool aliases', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'maven-entry',
          category: 'dockerfile',
          pattern: 'pom.xml',
          recommendation: 'Use Maven wrapper',
          tags: ['maven', 'java'],
        },
        {
          id: 'gradle-entry',
          category: 'dockerfile',
          pattern: 'build.gradle',
          recommendation: 'Use Gradle wrapper',
          tags: ['gradle', 'java'],
        },
        {
          id: 'rust-entry',
          category: 'dockerfile',
          pattern: 'Cargo.toml',
          recommendation: 'Use Cargo for Rust builds',
          tags: ['rust', 'cargo-build'],
        },
      ];

      // Test mvn → maven normalization
      const mavenQuery: KnowledgeQuery = {
        tags: ['mvn'],
      };
      const mavenMatches = findKnowledgeMatches(entries, mavenQuery);
      expect(mavenMatches.length).toBeGreaterThan(0);
      expect(mavenMatches[0].entry.id).toBe('maven-entry');

      // Test gradlew → gradle normalization
      const gradleQuery: KnowledgeQuery = {
        tags: ['gradlew'],
      };
      const gradleMatches = findKnowledgeMatches(entries, gradleQuery);
      expect(gradleMatches.length).toBeGreaterThan(0);
      expect(gradleMatches[0].entry.id).toBe('gradle-entry');

      // Test cargo → rust normalization
      const cargoQuery: KnowledgeQuery = {
        tags: ['cargo'],
      };
      const cargoMatches = findKnowledgeMatches(entries, cargoQuery);
      expect(cargoMatches.length).toBeGreaterThan(0);
      expect(cargoMatches[0].entry.id).toBe('rust-entry');
    });

    test('should normalize vendor aliases', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'google-entry',
          category: 'dockerfile',
          pattern: 'distroless',
          recommendation: 'Use Google distroless images',
          tags: ['google', 'distroless', 'security'],
        },
        {
          id: 'aws-entry',
          category: 'kubernetes',
          pattern: 'EKS',
          recommendation: 'Configure for AWS EKS',
          tags: ['aws', 'eks', 'kubernetes'],
        },
        {
          id: 'azure-entry',
          category: 'kubernetes',
          pattern: 'AKS',
          recommendation: 'Configure for Azure AKS',
          tags: ['azure', 'aks', 'kubernetes'],
        },
      ];

      // Test gcp → google normalization
      const gcpQuery: KnowledgeQuery = {
        tags: ['gcp'],
      };
      const gcpMatches = findKnowledgeMatches(entries, gcpQuery);
      expect(gcpMatches.length).toBeGreaterThan(0);
      expect(gcpMatches[0].entry.id).toBe('google-entry');

      // Test gcr → google normalization
      const gcrQuery: KnowledgeQuery = {
        tags: ['gcr'],
      };
      const gcrMatches = findKnowledgeMatches(entries, gcrQuery);
      expect(gcrMatches.length).toBeGreaterThan(0);
      expect(gcrMatches[0].entry.id).toBe('google-entry');

      // Test eks → aws normalization
      const eksQuery: KnowledgeQuery = {
        tags: ['eks'],
      };
      const eksMatches = findKnowledgeMatches(entries, eksQuery);
      expect(eksMatches.length).toBeGreaterThan(0);
      expect(eksMatches[0].entry.id).toBe('aws-entry');

      // Test ecr → aws normalization
      const ecrQuery: KnowledgeQuery = {
        tags: ['ecr'],
      };
      const ecrMatches = findKnowledgeMatches(entries, ecrQuery);
      expect(ecrMatches.length).toBeGreaterThan(0);
      expect(ecrMatches[0].entry.id).toBe('aws-entry');

      // Test aks → azure normalization
      const aksQuery: KnowledgeQuery = {
        tags: ['aks'],
      };
      const aksMatches = findKnowledgeMatches(entries, aksQuery);
      expect(aksMatches.length).toBeGreaterThan(0);
      expect(aksMatches[0].entry.id).toBe('azure-entry');

      // Test acr → azure normalization
      const acrQuery: KnowledgeQuery = {
        tags: ['acr'],
      };
      const acrMatches = findKnowledgeMatches(entries, acrQuery);
      expect(acrMatches.length).toBeGreaterThan(0);
      expect(acrMatches[0].entry.id).toBe('azure-entry');
    });

    test('should normalize language aliases', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'node-entry',
          category: 'dockerfile',
          pattern: 'FROM node',
          recommendation: 'Use Node.js best practices',
          tags: ['node', 'javascript'],
        },
      ];

      // Test javascript → node normalization
      const jsQuery: KnowledgeQuery = {
        tags: ['javascript'],
      };
      const jsMatches = findKnowledgeMatches(entries, jsQuery);
      expect(jsMatches.length).toBeGreaterThan(0);
      expect(jsMatches[0].entry.id).toBe('node-entry');

      // Test typescript → node normalization
      const tsQuery: KnowledgeQuery = {
        tags: ['typescript'],
      };
      const tsMatches = findKnowledgeMatches(entries, tsQuery);
      expect(tsMatches.length).toBeGreaterThan(0);
      expect(tsMatches[0].entry.id).toBe('node-entry');

      // Test nodejs → node normalization
      const nodejsQuery: KnowledgeQuery = {
        tags: ['nodejs'],
      };
      const nodejsMatches = findKnowledgeMatches(entries, nodejsQuery);
      expect(nodejsMatches.length).toBeGreaterThan(0);
      expect(nodejsMatches[0].entry.id).toBe('node-entry');
    });
  });

  describe('tool-specific matching', () => {
    test('should prioritize tool-tagged entries', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'fix-dockerfile-security',
          category: 'security',
          pattern: 'USER\\s+(root|0)',
          recommendation: 'Avoid running as root',
          severity: 'high',
          tags: ['security', 'user', 'fix-dockerfile'],
        },
        {
          id: 'general-security',
          category: 'security',
          pattern: 'USER',
          recommendation: 'Set user correctly',
          severity: 'medium',
          tags: ['security', 'user'],
        },
      ];

      const query: KnowledgeQuery = {
        tool: 'fix-dockerfile',
        category: 'security',
        text: 'USER root',
      };

      const matches = findKnowledgeMatches(entries, query);

      expect(matches.length).toBeGreaterThan(0);
      // Tool-tagged entry should be prioritized due to higher tool score
      expect(matches[0].entry.id).toBe('fix-dockerfile-security');
      expect(matches[0].reasons.some((r) => r.includes('Tool: fix-dockerfile'))).toBe(true);
    });

    test('should match scan-image tool context', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'scan-vulnerability-fix',
          category: 'security',
          pattern: 'CVE|vulnerability',
          recommendation: 'Update vulnerable packages',
          severity: 'high',
          tags: ['security', 'vulnerability', 'scan-image'],
        },
        {
          id: 'general-update',
          category: 'security',
          pattern: 'update',
          recommendation: 'Keep packages updated',
          severity: 'medium',
          tags: ['security', 'maintenance'],
        },
      ];

      const query: KnowledgeQuery = {
        tool: 'scan-image',
        category: 'security',
      };

      const matches = findKnowledgeMatches(entries, query);

      expect(matches.length).toBeGreaterThan(0);
      expect(matches[0].entry.tags).toContain('scan-image');
      expect(matches[0].reasons.some((r) => r.includes('Tool: scan-image'))).toBe(true);
    });

    test('should match generate-dockerfile tool context', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'dockerfile-multistage',
          category: 'dockerfile',
          pattern: 'multistage|multi-stage',
          recommendation: 'Use multi-stage builds for optimization',
          severity: 'high',
          tags: ['optimization', 'multistage', 'generate-dockerfile'],
        },
        {
          id: 'dockerfile-basic',
          category: 'dockerfile',
          pattern: 'FROM',
          recommendation: 'Basic Dockerfile guidance',
          severity: 'low',
          tags: ['dockerfile'],
        },
      ];

      const query: KnowledgeQuery = {
        tool: 'generate-dockerfile',
        text: 'multi-stage build',
      };

      const matches = findKnowledgeMatches(entries, query);

      expect(matches.length).toBeGreaterThan(0);
      expect(matches[0].entry.id).toBe('dockerfile-multistage');
      expect(matches[0].reasons.some((r) => r.includes('Tool: generate-dockerfile'))).toBe(true);
    });

    test('should match deploy and verify-deploy tool contexts', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'deploy-health-check',
          category: 'kubernetes',
          pattern: 'readiness|liveness',
          recommendation: 'Configure health checks',
          severity: 'high',
          tags: ['kubernetes', 'health', 'deploy', 'verify-deploy'],
        },
        {
          id: 'k8s-general',
          category: 'kubernetes',
          pattern: 'Deployment',
          recommendation: 'Basic deployment config',
          severity: 'medium',
          tags: ['kubernetes'],
        },
      ];

      const deployQuery: KnowledgeQuery = {
        tool: 'deploy',
        category: 'kubernetes',
      };

      const deployMatches = findKnowledgeMatches(entries, deployQuery);
      expect(deployMatches.length).toBeGreaterThan(0);
      expect(deployMatches[0].entry.tags).toContain('deploy');

      const verifyQuery: KnowledgeQuery = {
        tool: 'verify-deploy',
        category: 'kubernetes',
      };

      const verifyMatches = findKnowledgeMatches(entries, verifyQuery);
      expect(verifyMatches.length).toBeGreaterThan(0);
      expect(verifyMatches[0].entry.tags).toContain('verify-deploy');
    });

    test('should combine tool context with other scoring factors', () => {
      const entries: LoadedEntry[] = [
        {
          id: 'node-fix-security',
          category: 'security',
          pattern: 'USER\\s+root',
          recommendation: 'Node.js specific security fix',
          severity: 'high',
          tags: ['node', 'security', 'fix-dockerfile'],
        },
        {
          id: 'python-fix-security',
          category: 'security',
          pattern: 'USER\\s+root',
          recommendation: 'Python specific security fix',
          severity: 'high',
          tags: ['python', 'security', 'fix-dockerfile'],
        },
        {
          id: 'generic-fix-security',
          category: 'security',
          pattern: 'USER\\s+root',
          recommendation: 'Generic security fix',
          severity: 'medium',
          tags: ['security', 'fix-dockerfile'],
        },
      ];

      const query: KnowledgeQuery = {
        tool: 'fix-dockerfile',
        language: 'javascript',
        category: 'security',
        text: 'USER root',
      };

      const matches = findKnowledgeMatches(entries, query);

      expect(matches.length).toBeGreaterThan(0);
      // Should prioritize node entry due to language match + tool match
      expect(matches[0].entry.id).toBe('node-fix-security');
      expect(matches[0].reasons.some((r) => r.includes('Tool'))).toBe(true);
      expect(matches[0].reasons.some((r) => r.includes('Language'))).toBe(true);
    });
  });
});
