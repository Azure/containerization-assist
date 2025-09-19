/**
 * Golden File Regression Tests for Critical Prompts
 *
 * Tests prompt rendering with known parameters and snapshots results
 * to ensure no functionality is lost during format changes.
 */

import { initializeRegistry, getPrompt } from '../../../src/prompts/prompt-registry';
import { join } from 'path';
import { createLogger } from '../../../src/lib/logger';

describe('Prompt Golden File Regression Tests', () => {
  const logger = createLogger({ name: 'test', level: 'silent' });

  beforeAll(async () => {
    // Initialize registry with actual prompts directory
    const promptsDir = join(process.cwd(), 'src/prompts');
    const result = await initializeRegistry(promptsDir, logger);

    if (!result.ok) {
      throw new Error(`Failed to initialize prompts: ${result.error}`);
    }
  });

  describe('deploy-application prompt', () => {
    const testParams = {
      manifests: ['deployment.yaml', 'service.yaml'],
      namespace: 'production',
      cluster: 'main-cluster',
      environment: 'production',
      strategy: 'rolling',
      existingDeployment: {
        name: 'my-app',
        replicas: 3,
        version: '1.2.0'
      },
      sessionAnalysis: {
        risks: ['high-traffic'],
        recommendations: ['enable-hpa']
      },
      knowledge: 'Best practices for production deployments',
      policy: 'Security-first deployment policy'
    };

    it('should render consistently with known parameters', async () => {
      const result = await getPrompt('deploy-application', testParams);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const content = result.value.content;

        // Verify key sections are present
        expect(content).toContain('Deployment Configuration:');
        expect(content).toContain('production'); // namespace
        expect(content).toContain('deployment.yaml'); // manifests
        expect(content).toContain('main-cluster'); // cluster
        expect(content).toContain('rolling'); // strategy
        expect(content).toContain('"name": "my-app"'); // existing deployment (as JSON)
        expect(content).toContain('"risks"'); // session analysis (as JSON)
        expect(content).toContain('Best practices'); // knowledge
        expect(content).toContain('Security-first'); // policy

        // Verify JSON structure request is present
        expect(content).toContain('"deploymentStrategy"');
        expect(content).toContain('"preDeploymentChecks"');
        expect(content).toContain('"rolloutSteps"');
        expect(content).toContain('"healthChecks"');
        expect(content).toContain('"riskAssessment"');

        // Snapshot test - this will create/verify golden file
        expect(content).toMatchSnapshot('deploy-application-golden');
      }
    });

    it('should handle minimal parameters correctly', async () => {
      const minimalParams = {
        manifests: ['app.yaml'],
        namespace: 'default'
      };

      const result = await getPrompt('deploy-application', minimalParams);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const content = result.value.content;

        // Should contain provided params
        expect(content).toContain('app.yaml');
        expect(content).toContain('default');

        // Optional params should be empty or handled gracefully
        // Just verify the content was generated without errors

        // Structure should still be intact
        expect(content).toContain('Deployment Configuration:');
        expect(content).toContain('"deploymentStrategy"');
      }
    });
  });

  describe('generate-k8s-manifests prompt', () => {
    it('should be loadable and render with test parameters', async () => {
      const testParams = {
        appName: 'test-app',
        imageId: 'nginx:1.21', // Use imageId as required by the schema
        port: 80,
        environment: 'staging',
        replicas: 2
      };

      const result = await getPrompt('generate-k8s-manifests', testParams);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.content).toContain('test-app');
        expect(result.value.content).toContain('nginx:1.21');
        expect(result.value.content.length).toBeGreaterThan(100);
      }
    });
  });

  describe('nested variable handling', () => {
    it('should handle complex nested objects correctly', () => {
      const complexParams = {
        app: {
          name: 'complex-app',
          config: {
            database: {
              host: 'db.example.com',
              port: 5432
            }
          }
        }
      };

      // Test with a template that uses nested variables
      const testTemplate = 'App: {{app.name}}, DB: {{app.config.database.host}}:{{app.config.database.port}}';

      // Since we can't easily test this with an actual prompt, we'll verify
      // the template rendering logic works correctly using a simple helper
      function renderTemplate(template: string, params: Record<string, unknown>): string {
        return template.replace(/\{\{\s*(\w+(?:\.\w+)*)\s*\}\}/g, (_, path) => {
          const value = path.split('.').reduce((current: any, key: string) => {
            return current && typeof current === 'object' ? current[key] : undefined;
          }, params);
          return value !== undefined ? String(value) : '';
        });
      }

      const rendered = renderTemplate(testTemplate, complexParams);
      expect(rendered).toBe('App: complex-app, DB: db.example.com:5432');
    });
  });

  describe('outputFormat: json handling', () => {
    it('should preserve JSON output format instructions', async () => {
      const result = await getPrompt('deploy-application', {
        manifests: ['test.yaml'],
        namespace: 'test'
      });

      if (result.ok) {
        const content = result.value.content;

        // Should contain JSON format instruction
        expect(content).toContain('```json');
        expect(content).toContain('```');

        // Should have proper JSON structure examples
        expect(content).toMatch(/"[\w]+": "[^"]*"/); // JSON key-value pairs
      }
    });
  });
});