/**
 * VS Code Extension Integration Tests
 *
 * Verifies that all exports needed for VS Code extension integration
 * are available and correctly typed.
 */

import { describe, it, expect } from '@jest/globals';
import {
  // Functions
  analyzeRepo,
  executeTool,
  tools,

  // JSON Schemas
  jsonSchemas,

  // Metadata
  toolMetadata,
  standardWorkflow,
  getToolsByCategory,

  // Formatters
  resultFormatters,

  // VS Code utilities
  createAbortSignalFromToken,
  formatErrorForLLM,
  resolveWorkspacePath,
  validateRequiredFields,
  sanitizeForMarkdown,
} from '@/sdk';

import {
  createMockCancellationToken,
  createMockInvocationOptions,
} from '../../__support__/mocks/mock-factories.js';

describe('VS Code Extension Integration Exports', () => {
  describe('JSON Schema exports', () => {
    it('exports all 11 tool schemas', () => {
      expect(Object.keys(jsonSchemas)).toHaveLength(11);
    });

    it('exports schemas for all expected tools', () => {
      const expectedTools = [
        'analyzeRepo',
        'generateDockerfile',
        'fixDockerfile',
        'buildImage',
        'scanImage',
        'tagImage',
        'pushImage',
        'generateK8sManifests',
        'prepareCluster',
        'verifyDeploy',
        'ops',
      ];

      for (const tool of expectedTools) {
        expect(jsonSchemas).toHaveProperty(tool);
      }
    });

    it('schemas have required JSON Schema properties', () => {
      const schema = jsonSchemas.analyzeRepo;
      expect(schema).toHaveProperty('type');
      expect(schema).toHaveProperty('properties');
    });

    it('analyzeRepo schema has repositoryPath property', () => {
      const schema = jsonSchemas.analyzeRepo as Record<string, unknown>;
      const properties = schema.properties as Record<string, unknown>;
      expect(properties).toHaveProperty('repositoryPath');
    });

    it('buildImage schema has required properties', () => {
      const schema = jsonSchemas.buildImage as Record<string, unknown>;
      const properties = schema.properties as Record<string, unknown>;
      expect(properties).toHaveProperty('path');
      expect(properties).toHaveProperty('imageName');
    });
  });

  describe('Tool metadata exports', () => {
    it('exports all 11 tool metadata', () => {
      expect(Object.keys(toolMetadata)).toHaveLength(11);
    });

    it('exports metadata for all expected tools', () => {
      const expectedTools = [
        'analyzeRepo',
        'generateDockerfile',
        'fixDockerfile',
        'buildImage',
        'scanImage',
        'tagImage',
        'pushImage',
        'generateK8sManifests',
        'prepareCluster',
        'verifyDeploy',
        'ops',
      ];

      for (const tool of expectedTools) {
        expect(toolMetadata).toHaveProperty(tool);
      }
    });

    it('metadata includes required VS Code fields', () => {
      const meta = toolMetadata.analyzeRepo;
      expect(meta.name).toBe('analyze_repo');
      expect(meta.displayName).toBeDefined();
      expect(meta.modelDescription).toBeDefined();
      expect(meta.toolReferenceName).toBeDefined();
      expect(meta.icon).toBeDefined();
      expect(meta.canBeReferencedInPrompt).toBeDefined();
    });

    it('metadata has user-friendly display names', () => {
      expect(toolMetadata.analyzeRepo.displayName).toBe('Analyze Repository');
      expect(toolMetadata.buildImage.displayName).toBe('Build Docker Image');
      expect(toolMetadata.scanImage.displayName).toBe('Scan Docker Image');
    });

    it('metadata includes confirmation config', () => {
      const meta = toolMetadata.buildImage;
      expect(meta.confirmation).toBeDefined();
      expect(meta.confirmation.title).toBeDefined();
      expect(meta.confirmation.messageTemplate).toBeDefined();
      expect(meta.confirmation.isReadOnly).toBe(false);
      expect(meta.confirmation.warning).toBeDefined();
    });

    it('read-only operations are marked correctly', () => {
      expect(toolMetadata.analyzeRepo.confirmation.isReadOnly).toBe(true);
      expect(toolMetadata.scanImage.confirmation.isReadOnly).toBe(true);
      expect(toolMetadata.buildImage.confirmation.isReadOnly).toBe(false);
      expect(toolMetadata.pushImage.confirmation.isReadOnly).toBe(false);
    });

    it('metadata includes suggested next tools', () => {
      expect(toolMetadata.analyzeRepo.suggestedNextTools).toContain('generate_dockerfile');
      expect(toolMetadata.buildImage.suggestedNextTools).toContain('scan_image');
    });

    it('metadata includes category', () => {
      expect(toolMetadata.analyzeRepo.category).toBe('analysis');
      expect(toolMetadata.buildImage.category).toBe('image');
      expect(toolMetadata.generateK8sManifests.category).toBe('kubernetes');
    });

    it('metadata includes external dependencies info', () => {
      expect(toolMetadata.analyzeRepo.requiresExternalDeps).toEqual([]);
      // buildImage requires docker (structured format)
      expect(toolMetadata.buildImage.requiresExternalDeps).toContainEqual(
        expect.objectContaining({ id: 'docker' })
      );
    });

    it('standardWorkflow has correct order', () => {
      expect(standardWorkflow[0]).toBe('analyzeRepo');
      expect(standardWorkflow).toContain('buildImage');
      expect(standardWorkflow).toContain('generateK8sManifests');
      expect(standardWorkflow.indexOf('analyzeRepo')).toBeLessThan(
        standardWorkflow.indexOf('generateDockerfile')
      );
      expect(standardWorkflow.indexOf('buildImage')).toBeLessThan(
        standardWorkflow.indexOf('scanImage')
      );
    });

    it('getToolsByCategory returns correct tools', () => {
      const analysisTools = getToolsByCategory('analysis');
      expect(analysisTools.length).toBeGreaterThan(0);
      expect(analysisTools.every((t) => t.category === 'analysis')).toBe(true);

      const imageTools = getToolsByCategory('image');
      expect(imageTools.length).toBeGreaterThan(0);
      expect(imageTools.every((t) => t.category === 'image')).toBe(true);
    });
  });

  describe('Result formatter exports', () => {
    it('exports all 11 formatters', () => {
      expect(Object.keys(resultFormatters)).toHaveLength(11);
    });

    it('exports formatters for all expected tools', () => {
      const expectedTools = [
        'analyzeRepo',
        'generateDockerfile',
        'fixDockerfile',
        'buildImage',
        'scanImage',
        'tagImage',
        'pushImage',
        'generateK8sManifests',
        'prepareCluster',
        'verifyDeploy',
        'ops',
      ];

      for (const tool of expectedTools) {
        expect(resultFormatters).toHaveProperty(tool);
      }
    });

    it('formatters are functions', () => {
      expect(typeof resultFormatters.analyzeRepo).toBe('function');
      expect(typeof resultFormatters.buildImage).toBe('function');
      expect(typeof resultFormatters.scanImage).toBe('function');
    });
  });

  describe('VS Code utility exports', () => {
    describe('createAbortSignalFromToken', () => {
      it('is exported as a function', () => {
        expect(typeof createAbortSignalFromToken).toBe('function');
      });

      it('creates signal from mock token', () => {
        const mockToken = createMockCancellationToken();

        const { signal, dispose } = createAbortSignalFromToken(mockToken);
        expect(signal).toBeInstanceOf(AbortSignal);
        expect(signal.aborted).toBe(false);
        dispose();
      });

      it('creates aborted signal when token is already cancelled', () => {
        const mockToken = createMockCancellationToken({ cancelled: true });

        const { signal, dispose } = createAbortSignalFromToken(mockToken);
        expect(signal.aborted).toBe(true);
        dispose();
      });

      it('aborts signal when cancellation is requested', () => {
        const mockToken = createMockCancellationToken();

        const { signal, dispose } = createAbortSignalFromToken(mockToken);
        expect(signal.aborted).toBe(false);

        // Simulate cancellation using the factory's _trigger helper
        mockToken._trigger();
        expect(signal.aborted).toBe(true);

        dispose();
      });
    });

    describe('Mock factories', () => {
      it('createMockCancellationToken creates valid token', () => {
        const token = createMockCancellationToken();
        expect(token.isCancellationRequested).toBe(false);
        expect(typeof token.onCancellationRequested).toBe('function');
        expect(typeof token._trigger).toBe('function');
      });

      it('createMockCancellationToken respects cancelled option', () => {
        const token = createMockCancellationToken({ cancelled: true });
        expect(token.isCancellationRequested).toBe(true);
      });

      it('createMockInvocationOptions wraps input correctly', () => {
        const input = { repositoryPath: '/test' };
        const options = createMockInvocationOptions(input);
        expect(options.input).toBe(input);
      });
    });

    describe('formatErrorForLLM', () => {
      it('is exported as a function', () => {
        expect(typeof formatErrorForLLM).toBe('function');
      });

      it('formats error message only', () => {
        const formatted = formatErrorForLLM('Test error');
        expect(formatted).toBe('Test error');
      });

      it('includes hint when provided', () => {
        const formatted = formatErrorForLLM('Test error', {
          hint: 'Try again',
        });

        expect(formatted).toContain('Test error');
        expect(formatted).toContain('Hint: Try again');
      });

      it('includes resolution when provided', () => {
        const formatted = formatErrorForLLM('Test error', {
          resolution: 'Check input',
        });

        expect(formatted).toContain('Test error');
        expect(formatted).toContain('Resolution: Check input');
      });

      it('includes both hint and resolution when provided', () => {
        const formatted = formatErrorForLLM('Test error', {
          hint: 'Try again',
          resolution: 'Check input',
        });

        expect(formatted).toContain('Test error');
        expect(formatted).toContain('Hint: Try again');
        expect(formatted).toContain('Resolution: Check input');
      });
    });

    describe('resolveWorkspacePath', () => {
      it('is exported as a function', () => {
        expect(typeof resolveWorkspacePath).toBe('function');
      });

      it('resolves relative path with ./', () => {
        const result = resolveWorkspacePath('./src', '/workspace');
        expect(result).toBe('/workspace/src');
      });

      it('resolves relative path without ./', () => {
        const result = resolveWorkspacePath('src', '/workspace');
        expect(result).toBe('/workspace/src');
      });

      it('returns absolute Unix path unchanged', () => {
        expect(resolveWorkspacePath('/absolute/path', '/workspace')).toBe('/absolute/path');
      });

      it('returns absolute Windows path unchanged', () => {
        // Only test on Windows - on Unix, Windows paths aren't recognized as absolute
        if (process.platform === 'win32') {
          expect(resolveWorkspacePath('C:\\absolute\\path', '/workspace')).toBe('C:\\absolute\\path');
        }
      });

      it('handles workspace root with trailing slash', () => {
        const result = resolveWorkspacePath('src', '/workspace/');
        expect(result).toBe('/workspace/src');
      });

      it('handles parent directory references', () => {
        const result = resolveWorkspacePath('../sibling/file', '/workspace/project');
        expect(result).toBe('/workspace/sibling/file');
      });

      it('normalizes path separators', () => {
        const result = resolveWorkspacePath('src//nested///file', '/workspace');
        expect(result).not.toContain('//');
      });
    });

    describe('validateRequiredFields', () => {
      it('is exported as a function', () => {
        expect(typeof validateRequiredFields).toBe('function');
      });

      it('returns valid when all required fields present', () => {
        const result = validateRequiredFields({ name: 'test', value: 123 }, ['name', 'value']);
        expect(result.valid).toBe(true);
        expect(result.missing).toHaveLength(0);
      });

      it('returns invalid when fields are missing', () => {
        const result = validateRequiredFields({ name: 'test' }, ['name', 'value']);
        expect(result.valid).toBe(false);
        expect(result.missing).toContain('value');
      });

      it('treats empty string as missing', () => {
        const result = validateRequiredFields({ name: '' }, ['name']);
        expect(result.valid).toBe(false);
        expect(result.missing).toContain('name');
      });

      it('treats null as missing', () => {
        const result = validateRequiredFields({ name: null }, ['name']);
        expect(result.valid).toBe(false);
        expect(result.missing).toContain('name');
      });
    });

    describe('sanitizeForMarkdown', () => {
      it('is exported as a function', () => {
        expect(typeof sanitizeForMarkdown).toBe('function');
      });

      it('escapes backticks', () => {
        expect(sanitizeForMarkdown('code `here`')).toContain('\\`');
      });

      it('escapes asterisks', () => {
        expect(sanitizeForMarkdown('*bold*')).toContain('\\*');
      });

      it('escapes underscores', () => {
        expect(sanitizeForMarkdown('_italic_')).toContain('\\_');
      });

      it('escapes angle brackets', () => {
        const result = sanitizeForMarkdown('<script>');
        expect(result).toContain('&lt;');
        expect(result).toContain('&gt;');
      });
    });
  });

  describe('SDK function exports', () => {
    it('exports analyzeRepo function', () => {
      expect(typeof analyzeRepo).toBe('function');
    });

    it('exports executeTool function', () => {
      expect(typeof executeTool).toBe('function');
    });

    it('exports tools object with all 11 tools', () => {
      expect(Object.keys(tools)).toHaveLength(11);
    });

    it('tools object has expected tool names', () => {
      const expectedTools = [
        'analyzeRepo',
        'generateDockerfile',
        'fixDockerfile',
        'buildImage',
        'scanImage',
        'tagImage',
        'pushImage',
        'generateK8sManifests',
        'prepareCluster',
        'verifyDeploy',
        'ops',
      ];

      for (const tool of expectedTools) {
        expect(tools).toHaveProperty(tool);
      }
    });

    it('each tool has required properties', () => {
      for (const [_name, tool] of Object.entries(tools)) {
        expect(tool).toHaveProperty('name', expect.any(String));
        expect(tool).toHaveProperty('description', expect.any(String));
        expect(tool).toHaveProperty('schema');
        // Tools use 'handler' property for execution
        expect(tool).toHaveProperty('handler', expect.any(Function));
      }
    });
  });

  describe('Schema and metadata consistency', () => {
    it('jsonSchemas and toolMetadata have matching keys', () => {
      const schemaKeys = Object.keys(jsonSchemas).sort();
      const metadataKeys = Object.keys(toolMetadata).sort();
      expect(schemaKeys).toEqual(metadataKeys);
    });

    it('resultFormatters has matching keys', () => {
      const schemaKeys = Object.keys(jsonSchemas).sort();
      const formatterKeys = Object.keys(resultFormatters).sort();
      expect(schemaKeys).toEqual(formatterKeys);
    });

    it('tools object has matching keys', () => {
      const schemaKeys = Object.keys(jsonSchemas).sort();
      const toolKeys = Object.keys(tools).sort();
      expect(schemaKeys).toEqual(toolKeys);
    });
  });
});
