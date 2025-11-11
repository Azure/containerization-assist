/**
 * Unit tests for template merger utility
 */

import { describe, it, expect } from '@jest/globals';
import {
  dockerfileTemplatesToRecommendations,
  k8sTemplatesToRecommendations,
  mergeTemplateRecommendations,
  mergeTemplatesIntoPlan,
} from '@/lib/template-merger';
import type {
  DockerfileTemplateSnippet,
  K8sTemplateComponent,
} from '@/config/policy-generation-config';

describe('Template Merger', () => {
  describe('dockerfileTemplatesToRecommendations', () => {
    it('converts Dockerfile templates to recommendations', () => {
      const templates: DockerfileTemplateSnippet[] = [
        {
          id: 'ca-certs',
          section: 'security',
          content: 'RUN update-ca-certificates',
          description: 'Install CA certificates',
          priority: 100,
        },
      ];

      const recommendations = dockerfileTemplatesToRecommendations(templates, {
        language: 'java',
        environment: 'production',
      });

      expect(recommendations).toHaveLength(1);
      expect(recommendations[0]).toMatchObject({
        id: 'ca-certs',
        category: 'dockerfile-template-security',
        policyDriven: true,
        matchScore: 100,
      });
      expect(recommendations[0].recommendation).toContain('Install CA certificates');
      expect(recommendations[0].recommendation).toContain('RUN update-ca-certificates');
    });

    it('filters templates by language condition', () => {
      const templates: DockerfileTemplateSnippet[] = [
        {
          id: 'java-agent',
          section: 'observability',
          content: 'ADD newrelic.jar /opt/',
          description: 'Install New Relic',
          conditions: {
            languages: ['java'],
          },
        },
        {
          id: 'ca-certs',
          section: 'security',
          content: 'RUN update-ca-certificates',
          description: 'Install CA certificates',
        },
      ];

      // Java context - should get both
      const javaRecs = dockerfileTemplatesToRecommendations(templates, {
        language: 'java',
        environment: 'production',
      });
      expect(javaRecs).toHaveLength(2);

      // Python context - should only get CA certs (no language condition)
      const pythonRecs = dockerfileTemplatesToRecommendations(templates, {
        language: 'python',
        environment: 'production',
      });
      expect(pythonRecs).toHaveLength(1);
      expect(pythonRecs[0].id).toBe('ca-certs');
    });

    it('filters templates by environment condition', () => {
      const templates: DockerfileTemplateSnippet[] = [
        {
          id: 'prod-hardening',
          section: 'security',
          content: 'USER appuser',
          description: 'Security hardening',
          conditions: {
            environments: ['production'],
          },
        },
      ];

      // Production - should get template
      const prodRecs = dockerfileTemplatesToRecommendations(templates, {
        language: 'java',
        environment: 'production',
      });
      expect(prodRecs).toHaveLength(1);

      // Development - should not get template
      const devRecs = dockerfileTemplatesToRecommendations(templates, {
        language: 'java',
        environment: 'development',
      });
      expect(devRecs).toHaveLength(0);
    });

    it('sorts templates by priority', () => {
      const templates: DockerfileTemplateSnippet[] = [
        {
          id: 'low-priority',
          section: 'security',
          content: 'RUN echo low',
          description: 'Low priority',
          priority: 10,
        },
        {
          id: 'high-priority',
          section: 'security',
          content: 'RUN echo high',
          description: 'High priority',
          priority: 100,
        },
        {
          id: 'medium-priority',
          section: 'security',
          content: 'RUN echo medium',
          description: 'Medium priority',
          priority: 50,
        },
      ];

      const recommendations = dockerfileTemplatesToRecommendations(templates, {
        language: 'java',
      });

      expect(recommendations).toHaveLength(3);
      expect(recommendations[0].id).toBe('high-priority');
      expect(recommendations[1].id).toBe('medium-priority');
      expect(recommendations[2].id).toBe('low-priority');
    });
  });

  describe('k8sTemplatesToRecommendations', () => {
    it('converts K8s templates to recommendations', () => {
      const templates: K8sTemplateComponent[] = [
        {
          id: 'log-sidecar',
          type: 'sidecar',
          description: 'Fluentd log forwarder',
          spec: {
            name: 'fluentd',
            image: 'fluent/fluentd:latest',
          },
          priority: 100,
        },
      ];

      const recommendations = k8sTemplatesToRecommendations(templates, {
        language: 'java',
        environment: 'production',
      });

      expect(recommendations).toHaveLength(1);
      expect(recommendations[0]).toMatchObject({
        id: 'log-sidecar',
        category: 'k8s-template-sidecar',
        policyDriven: true,
        matchScore: 100,
      });
      expect(recommendations[0].recommendation).toContain('Fluentd log forwarder');
      expect(recommendations[0].recommendation).toContain('sidecar');
    });

    it('filters K8s templates by conditions', () => {
      const templates: K8sTemplateComponent[] = [
        {
          id: 'db-init',
          type: 'initContainer',
          description: 'Database migration',
          spec: { name: 'flyway' },
          conditions: {
            languages: ['java'],
            environments: ['production'],
          },
        },
        {
          id: 'secrets',
          type: 'volume',
          description: 'Org secrets',
          spec: { name: 'secrets' },
        },
      ];

      // Java + production - should get both
      const javaProdRecs = k8sTemplatesToRecommendations(templates, {
        language: 'java',
        environment: 'production',
      });
      expect(javaProdRecs).toHaveLength(2);

      // Python + production - should only get secrets (no language condition)
      const pythonProdRecs = k8sTemplatesToRecommendations(templates, {
        language: 'python',
        environment: 'production',
      });
      expect(pythonProdRecs).toHaveLength(1);
      expect(pythonProdRecs[0].id).toBe('secrets');

      // Java + dev - should only get secrets (no environment condition)
      const javaDevRecs = k8sTemplatesToRecommendations(templates, {
        language: 'java',
        environment: 'development',
      });
      expect(javaDevRecs).toHaveLength(1);
      expect(javaDevRecs[0].id).toBe('secrets');
    });
  });

  describe('mergeTemplateRecommendations', () => {
    it('merges templates at the beginning of existing recommendations', () => {
      const existing = [
        { id: 'existing-1', policyDriven: false },
        { id: 'existing-2', policyDriven: false },
      ];

      const templates = [
        {
          id: 'template-1',
          category: 'security',
          recommendation: 'Template recommendation',
          tags: ['policy-template'],
          matchScore: 100,
          policyDriven: true,
        },
      ];

      const merged = mergeTemplateRecommendations(existing, templates);

      expect(merged).toHaveLength(3);
      expect(merged[0].id).toBe('template-1');
      expect(merged[1].id).toBe('existing-1');
      expect(merged[2].id).toBe('existing-2');
    });

    it('marks existing recommendations as not policy-driven', () => {
      const existing = [{ id: 'existing-1' }];
      const templates = [
        {
          id: 'template-1',
          category: 'security',
          recommendation: 'Template',
          tags: [],
          matchScore: 100,
          policyDriven: true,
        },
      ];

      const merged = mergeTemplateRecommendations(existing, templates);

      expect(merged[1].policyDriven).toBe(false);
    });
  });

  describe('mergeTemplatesIntoPlan', () => {
    it('merges Dockerfile templates into plan recommendations', () => {
      const plan = {
        recommendations: {
          securityConsiderations: [
            { id: 'existing-sec', policyDriven: false },
          ] as unknown[],
          bestPractices: [{ id: 'existing-bp', policyDriven: false }] as unknown[],
        },
      };

      const templates = {
        dockerfile: [
          {
            id: 'security-template',
            section: 'security' as const,
            content: 'RUN hardening',
            description: 'Security hardening',
          },
          {
            id: 'runtime-template',
            section: 'runtime' as const,
            content: 'USER appuser',
            description: 'Non-root user',
          },
        ],
      };

      const merged = mergeTemplatesIntoPlan(plan, templates, {
        language: 'java',
        environment: 'production',
      });

      expect(merged.recommendations.securityConsiderations.length).toBeGreaterThan(1);
      expect(merged.recommendations.bestPractices.length).toBeGreaterThan(1);

      // Security template should be in securityConsiderations
      const secTemplates = merged.recommendations.securityConsiderations.filter(
        (r: { policyDriven?: boolean }) => r.policyDriven === true,
      );
      expect(secTemplates.length).toBeGreaterThan(0);
    });

    it('merges K8s templates into plan recommendations', () => {
      const plan = {
        recommendations: {
          securityConsiderations: [
            { id: 'existing-sec', policyDriven: false },
          ] as unknown[],
          bestPractices: [{ id: 'existing-bp', policyDriven: false }] as unknown[],
        },
      };

      const templates = {
        kubernetes: [
          {
            id: 'security-sidecar',
            type: 'sidecar' as const,
            description: 'Security scanning sidecar',
            spec: { name: 'scanner' },
          },
          {
            id: 'init-container',
            type: 'initContainer' as const,
            description: 'Init container for setup',
            spec: { name: 'init' },
          },
        ],
      };

      const merged = mergeTemplatesIntoPlan(plan, templates, {
        language: 'java',
        environment: 'production',
      });

      expect(merged.recommendations.bestPractices.length).toBeGreaterThan(1);

      // K8s templates should be in bestPractices
      const k8sTemplates = merged.recommendations.bestPractices.filter(
        (r: { policyDriven?: boolean }) => r.policyDriven === true,
      );
      expect(k8sTemplates.length).toBeGreaterThan(0);
    });

    it('returns plan unchanged when no templates provided', () => {
      const plan = {
        recommendations: {
          securityConsiderations: [{ id: 'existing' }] as unknown[],
        },
      };

      const merged = mergeTemplatesIntoPlan(plan, null, {
        language: 'java',
      });

      expect(merged).toBe(plan);
    });
  });
});
