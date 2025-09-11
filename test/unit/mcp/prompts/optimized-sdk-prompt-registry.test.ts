/**
 * Optimized SDK Prompt Registry Tests
 */

import type { Logger } from 'pino';
import * as promptRegistry from '../../../../src/prompts/registry';
import { createMockLogger } from '../../../__support__/utilities/mock-factories';
import { join } from 'path';

describe('Prompt Registry Functions', () => {
  let mockLogger: Logger;

  beforeEach(async () => {
    mockLogger = createMockLogger();
    promptRegistry.clearPromptCache(); // Clear any previous state
    await promptRegistry.initializePrompts(
      join(process.cwd(), 'src', 'prompts'),
      mockLogger
    );
  });

  describe('initialization', () => {
    it('should initialize with default templates', async () => {
      const result = await promptRegistry.listPrompts();
      const promptNames = result.prompts.map(p => p.name);
      
      expect(promptNames).toContain('dockerfile-sampling');
      expect(promptNames).toContain('dockerfile-generation');
      expect(promptNames).toContain('k8s-manifest-generation');
      expect(promptNames).toContain('parameter-validation');
      expect(promptNames).toContain('parameter-suggestions');
      expect(promptNames).toContain('security-analysis');
    });

    it('should have at least 6 default templates', async () => {
      const result = await promptRegistry.listPrompts();
      expect(result.prompts.length).toBeGreaterThanOrEqual(6);
    });
  });

  describe('hasPrompt', () => {
    it('should return true for existing prompts', () => {
      expect(promptRegistry.hasPrompt('dockerfile-generation')).toBe(true);
      expect(promptRegistry.hasPrompt('k8s-manifest-generation')).toBe(true);
    });

    it('should return false for non-existing prompts', () => {
      expect(promptRegistry.hasPrompt('non-existent-prompt')).toBe(false);
      expect(promptRegistry.hasPrompt('')).toBe(false);
    });
  });

  describe('getPromptInfo', () => {
    it('should return prompt info for existing prompts', () => {
      const info = promptRegistry.getPromptInfo('dockerfile-generation');
      
      expect(info).toBeDefined();
      expect(info?.description).toContain('optimized Dockerfile');
      expect(info?.arguments).toBeInstanceOf(Array);
      expect(info?.arguments.length).toBeGreaterThan(0);
    });

    it('should return null for non-existing prompts', () => {
      const info = promptRegistry.getPromptInfo('non-existent-prompt');
      expect(info).toBeNull();
    });

    it('should include required argument information', () => {
      const info = promptRegistry.getPromptInfo('dockerfile-generation');
      
      expect(info?.arguments).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            name: 'language',
            description: expect.any(String),
            required: true
          })
        ])
      );
    });
  });

  describe('getPrompt', () => {
    it('should generate prompt with template rendering', async () => {
      const result = await promptRegistry.getPrompt('dockerfile-generation', {
        language: 'javascript',
        framework: 'express',
        optimization: 'performance'
      });

      expect(result.name).toBe('dockerfile-generation');
      expect(result.description).toContain('optimized Dockerfile');
      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].role).toBe('user');
      expect(result.messages[0].content.type).toBe('text');
      
      const text = result.messages[0].content.text;
      expect(text).toContain('javascript');
      expect(text).toContain('express');
      expect(text).toContain('performance');
    });

    it('should handle missing template variables gracefully', async () => {
      const result = await promptRegistry.getPrompt('dockerfile-generation', {
        language: 'python'
        // framework and optimization missing
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('python');
      // Should not contain unrendered template variables for missing args
      expect(text).not.toContain('{{framework}}');
      expect(text).not.toContain('{{optimization}}');
    });

    it('should throw error for non-existent prompts', async () => {
      await expect(
        promptRegistry.getPrompt('non-existent-prompt')
      ).rejects.toThrow('Prompt not found: non-existent-prompt');
    });

    it('should work with empty args', async () => {
      const result = await promptRegistry.getPrompt('dockerfile-generation', {});
      
      expect(result.name).toBe('dockerfile-generation');
      expect(result.messages).toHaveLength(1);
      
      const text = result.messages[0].content.text;
      expect(text).toContain('Generate an optimized Dockerfile'); // Should have base prompt text
    });
  });

  describe('dockerfile-sampling template', () => {
    it('should render dockerfile sampling prompt correctly', async () => {
      const result = await promptRegistry.getPrompt('dockerfile-sampling', {
        strategy: 'security',
        language: 'nodejs',
        context: 'web application with database'
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('nodejs');
      expect(text).toContain('web application with database');
    });
  });

  describe('k8s-manifest-generation template', () => {
    it('should render k8s manifest prompt correctly', async () => {
      const result = await promptRegistry.getPrompt('k8s-manifest-generation', {
        appName: 'my-app',
        environment: 'production',
        replicas: '3'
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('my-app');
      expect(text).toContain('Generate Kubernetes deployment manifests');
    });
  });

  describe('parameter-validation template', () => {
    it('should render validation prompt correctly', async () => {
      const result = await promptRegistry.getPrompt('parameter-validation', {
        toolName: 'generate-dockerfile',
        parameters: '{"language": "python", "optimization": "size"}',
        context: 'production deployment'
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('generate-dockerfile');
      expect(text).toContain('{"language": "python", "optimization": "size"}');
      expect(text).toContain('production deployment');
      expect(text).toContain('Required parameter presence');
    });
  });

  describe('parameter-suggestions template', () => {
    it('should render suggestions prompt correctly', async () => {
      const result = await promptRegistry.getPrompt('parameter-suggestions', {
        toolName: 'generate-k8s-manifests',
        partialParameters: '{"appName": "myapp"}',
        context: 'microservice deployment'
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('generate-k8s-manifests');
      expect(text).toContain('{"appName": "myapp"}');
      expect(text).toContain('microservice deployment');
      expect(text).toContain('Missing required parameters');
    });
  });

  describe('security-analysis template', () => {
    it('should render security analysis prompt correctly', async () => {
      const result = await promptRegistry.getPrompt('security-analysis', {
        configType: 'dockerfile',
        content: 'FROM ubuntu\nRUN apt-get update',
        complianceStandard: 'CIS'
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('dockerfile');
      expect(text).toContain('FROM ubuntu');
      expect(text).toContain('CIS');
      expect(text).toContain('Vulnerability assessment');
    });
  });

  describe('template rendering edge cases', () => {
    it('should handle special characters in arguments', async () => {
      const result = await promptRegistry.getPrompt('dockerfile-generation', {
        language: 'C++',
        framework: 'Qt/C++ Framework'
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('C++');
      expect(text).toContain('Qt/C++ Framework');
    });

    it('should handle empty string arguments', async () => {
      const result = await promptRegistry.getPrompt('dockerfile-generation', {
        language: '',
        framework: 'express'
      });

      const text = result.messages[0].content.text;
      expect(text).toContain('express');
      // Empty string should be preserved, not replaced with template variable
      expect(text).not.toContain('{{language}}');
    });
  });

  describe('performance and complexity reduction', () => {
    it('should be significantly simpler than original registry', async () => {
      // Test that the simplified registry has reduced complexity
      const prompts = promptRegistry.getPromptNames();
      
      // Should have core prompts but not overly complex structure
      expect(prompts.length).toBeLessThan(20); // Reasonable upper limit
      expect(prompts.length).toBeGreaterThan(5); // Should have essential prompts
      
      // All prompts should be accessible
      for (const promptName of prompts) {
        expect(promptRegistry.hasPrompt(promptName)).toBe(true);
        expect(promptRegistry.getPromptInfo(promptName)).toBeDefined();
      }
    });

    it('should handle concurrent getPrompt calls efficiently', async () => {
      const promises = Array.from({ length: 10 }, (_, i) =>
        promptRegistry.getPrompt('dockerfile-generation', {
          language: `lang${i}`,
          framework: `framework${i}`
        })
      );

      const results = await Promise.all(promises);
      
      expect(results).toHaveLength(10);
      results.forEach((result, i) => {
        expect(result.messages[0].content.text).toContain(`lang${i}`);
        expect(result.messages[0].content.text).toContain(`framework${i}`);
      });
    });
  });
});