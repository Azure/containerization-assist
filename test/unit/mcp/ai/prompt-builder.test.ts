/**
 * Tests for AI Prompt Builder
 */

import { describe, it, expect } from '@jest/globals';
import { AIPromptBuilder } from '@mcp/ai/prompt-builder';
import type { AIParamRequest } from '@mcp/ai/host-ai-assist';

describe('AIPromptBuilder', () => {
  describe('basic building', () => {
    it('should build simple prompt', () => {
      const prompt = new AIPromptBuilder()
        .addSection('Tool', 'analyze-repo')
        .addSection('Missing', 'path')
        .build();

      expect(prompt).toContain('Tool: analyze-repo');
      expect(prompt).toContain('Missing: path');
    });

    it('should format JSON objects', () => {
      const prompt = new AIPromptBuilder()
        .addSection('Params', { foo: 'bar', nested: { value: 123 } })
        .build();

      expect(prompt).toContain('"foo": "bar"');
      expect(prompt).toContain('"nested": {');
      expect(prompt).toContain('"value": 123');
    });

    it('should skip undefined sections', () => {
      const prompt = new AIPromptBuilder()
        .addSection('Valid', 'value')
        .addSection('Invalid', undefined)
        .addSection('Null', null)
        .addSection('Another', 'included')
        .build();

      expect(prompt).toContain('Valid: value');
      expect(prompt).not.toContain('Invalid');
      expect(prompt).not.toContain('Null');
      expect(prompt).toContain('Another: included');
    });

    it('should add instructions', () => {
      const prompt = new AIPromptBuilder()
        .addInstruction('First instruction')
        .addInstruction('Second instruction')
        .build();

      expect(prompt).toBe('First instruction\nSecond instruction');
    });

    it('should add separators', () => {
      const prompt = new AIPromptBuilder()
        .addSection('Before', 'separator')
        .addSeparator()
        .addSection('After', 'separator')
        .build();

      const lines = prompt.split('\n');
      expect(lines).toEqual(['Before: separator', '', 'After: separator']);
    });
  });

  describe('forParameterSuggestion', () => {
    it('should create complete parameter suggestion prompt', () => {
      const request: AIParamRequest = {
        toolName: 'build-image',
        currentParams: { path: '/app' },
        requiredParams: ['path', 'imageId'],
        missingParams: ['imageId'],
        sessionContext: { appName: 'testapp' },
        schema: { imageId: { type: 'string' } },
      };

      const prompt = createParameterSuggestionPrompt(request);

      expect(prompt).toContain('Tool: build-image');
      expect(prompt).toContain('Current: {');
      expect(prompt).toContain('"path": "/app"');
      expect(prompt).toContain('Missing: imageId');
      expect(prompt).toContain('Schema: {');
      expect(prompt).toContain('Context: {');
      expect(prompt).toContain('"appName": "testapp"');
      expect(prompt).toContain('Return JSON object with suggested parameter values.');
      expect(prompt).toContain('Example: {"path": ".", "imageId": "app:latest"}');
    });

    it('should handle request with minimal fields', () => {
      const request: AIParamRequest = {
        toolName: 'simple-tool',
        currentParams: {},
        requiredParams: ['param1'],
        missingParams: ['param1'],
      };

      const prompt = createParameterSuggestionPrompt(request);

      expect(prompt).toContain('Tool: simple-tool');
      expect(prompt).toContain('Current: {}');
      expect(prompt).toContain('Missing: param1');
      expect(prompt).not.toContain('Schema:');
      expect(prompt).not.toContain('Context:');
      expect(prompt).toContain('Return JSON object');
    });

    it('should handle multiple missing parameters', () => {
      const request: AIParamRequest = {
        toolName: 'multi-param',
        currentParams: { existing: 'value' },
        requiredParams: ['existing', 'param1', 'param2', 'param3'],
        missingParams: ['param1', 'param2', 'param3'],
      };

      const prompt = createParameterSuggestionPrompt(request);

      expect(prompt).toContain('Missing: param1, param2, param3');
    });
  });

  describe('forContextAnalysis', () => {
    it('should create context analysis prompt', () => {
      const context = {
        language: 'typescript',
        framework: 'express',
        hasDocker: false,
      };
      const objective = 'Determine containerization strategy';

      const prompt = createContextAnalysisPrompt(context, objective);

      expect(prompt).toContain('Objective: Determine containerization strategy');
      expect(prompt).toContain('Context: {');
      expect(prompt).toContain('"language": "typescript"');
      expect(prompt).toContain('"framework": "express"');
      expect(prompt).toContain('"hasDocker": false');
      expect(prompt).toContain('Analyze the context and provide insights.');
    });
  });

  describe('chaining', () => {
    it('should support method chaining', () => {
      const builder = new AIPromptBuilder();
      const result = builder
        .addSection('First', 'value1')
        .addSection('Second', 'value2')
        .addSeparator()
        .addInstruction('Do something')
        .addInstruction('Do something else');

      expect(result).toBe(builder);

      const prompt = result.build();
      expect(prompt.split('\n')).toHaveLength(5);
    });
  });

  describe('edge cases', () => {
    it('should handle empty builder', () => {
      const prompt = new AIPromptBuilder().build();
      expect(prompt).toBe('');
    });

    it('should handle special characters in content', () => {
      const prompt = new AIPromptBuilder()
        .addSection('Special', 'Line\nbreak\ttab"quote')
        .build();

      expect(prompt).toContain('Special: Line\nbreak\ttab"quote');
    });

    it('should handle arrays as content', () => {
      const prompt = new AIPromptBuilder()
        .addSection('Array', ['item1', 'item2', 'item3'])
        .build();

      expect(prompt).toContain('Array: [');
      expect(prompt).toContain('"item1"');
      expect(prompt).toContain('"item2"');
      expect(prompt).toContain('"item3"');
    });

    it('should handle deeply nested objects', () => {
      const deepObject = {
        level1: {
          level2: {
            level3: {
              value: 'deep',
            },
          },
        },
      };

      const prompt = new AIPromptBuilder().addSection('Deep', deepObject).build();

      expect(prompt).toContain('"level1": {');
      expect(prompt).toContain('"level2": {');
      expect(prompt).toContain('"level3": {');
      expect(prompt).toContain('"value": "deep"');
    });
  });
});