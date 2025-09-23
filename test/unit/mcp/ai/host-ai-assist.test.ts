import { z } from 'zod';
import {
  suggestMissingParams,
  validateWithSchema,
  createHostAIAssistant,
  buildParameterPrompt,
  extractSchemaInfo,
} from '@/mcp/ai/host-ai-assist';

describe('Host AI Assistant', () => {
  describe('suggestMissingParams', () => {
    it('should return empty object for no missing params', async () => {
      const mockHostCall = jest.fn();
      const result = await suggestMissingParams('test-tool', [], mockHostCall);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({});
      }
      expect(mockHostCall).not.toHaveBeenCalled();
    });

    it('should call host AI and parse JSON response', async () => {
      const mockHostCall = jest.fn().mockResolvedValue('{"param1": "value1", "param2": 123}');
      const result = await suggestMissingParams(
        'test-tool',
        ['param1', 'param2'],
        mockHostCall,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ param1: 'value1', param2: 123 });
      }
      expect(mockHostCall).toHaveBeenCalledWith(expect.stringContaining('test-tool'));
      expect(mockHostCall).toHaveBeenCalledWith(expect.stringContaining('param1, param2'));
    });

    it('should handle JSON in code blocks', async () => {
      const mockHostCall = jest.fn().mockResolvedValue(
        '```json\n{"param1": "value1"}\n```',
      );
      const result = await suggestMissingParams('test-tool', ['param1'], mockHostCall);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ param1: 'value1' });
      }
    });

    it('should include context in prompt', async () => {
      const mockHostCall = jest.fn().mockResolvedValue('{"param1": "value1"}');
      const context = { existingParam: 'value' };

      await suggestMissingParams('test-tool', ['param1'], mockHostCall, context);

      expect(mockHostCall).toHaveBeenCalledWith(
        expect.stringContaining('existingParam'),
      );
    });

    it('should handle errors gracefully', async () => {
      const mockHostCall = jest.fn().mockRejectedValue(new Error('API error'));
      const result = await suggestMissingParams('test-tool', ['param1'], mockHostCall);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to get parameter suggestions');
      }
    });
  });

  describe('validateWithSchema', () => {
    it('should validate valid parameters', () => {
      const schema = z.object({
        name: z.string(),
        age: z.number(),
      });

      const result = validateWithSchema({ name: 'John', age: 30 }, schema);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ name: 'John', age: 30 });
      }
    });

    it('should return errors for invalid parameters', () => {
      const schema = z.object({
        name: z.string(),
        age: z.number(),
      });

      const result = validateWithSchema({ name: 'John', age: 'thirty' }, schema);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Validation failed');
        expect(result.error).toContain('age');
      }
    });

    it('should handle missing required fields', () => {
      const schema = z.object({
        required: z.string(),
        optional: z.string().optional(),
      });

      const result = validateWithSchema({ optional: 'value' }, schema);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('required');
      }
    });
  });

  describe('createHostAIAssistant', () => {
    it('should create an assistant with default config', () => {
      const mockHostCall = jest.fn();
      const assistant = createHostAIAssistant(mockHostCall);

      expect(assistant.isAvailable()).toBe(true);
    });

    it('should respect disabled config', () => {
      const mockHostCall = jest.fn();
      const assistant = createHostAIAssistant(mockHostCall, { enabled: false });

      expect(assistant.isAvailable()).toBe(false);
    });

    it('should suggest parameters successfully', async () => {
      const mockHostCall = jest.fn().mockResolvedValue('{"param1": "value1"}');
      const assistant = createHostAIAssistant(mockHostCall);

      const result = await assistant.suggestParameters({
        toolName: 'test-tool',
        currentParams: {},
        requiredParams: ['param1'],
        missingParams: ['param1'],
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.suggestions).toEqual({ param1: 'value1' });
        expect(result.value.confidence).toBe(0.7);
        expect(result.value.reasoning).toContain('param1');
      }
    });

    it('should return error when disabled', async () => {
      const mockHostCall = jest.fn();
      const assistant = createHostAIAssistant(mockHostCall, { enabled: false });

      const result = await assistant.suggestParameters({
        toolName: 'test-tool',
        currentParams: {},
        requiredParams: ['param1'],
        missingParams: ['param1'],
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('disabled');
      }
      expect(mockHostCall).not.toHaveBeenCalled();
    });

    it('should validate suggestions', () => {
      const mockHostCall = jest.fn();
      const assistant = createHostAIAssistant(mockHostCall);
      const schema = z.object({ param1: z.string() });

      const result = assistant.validateSuggestions({ param1: 'value1' }, schema);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ param1: 'value1' });
      }
    });
  });

  describe('buildParameterPrompt', () => {
    it('should build basic prompt', () => {
      const prompt = buildParameterPrompt('test-tool', ['param1', 'param2']);

      expect(prompt).toContain('test-tool');
      expect(prompt).toContain('- param1');
      expect(prompt).toContain('- param2');
      expect(prompt).toContain('JSON object');
    });

    it('should include context when provided', () => {
      const context = { existing: 'value' };
      const prompt = buildParameterPrompt('test-tool', ['param1'], context);

      expect(prompt).toContain('Context:');
      expect(prompt).toContain('existing');
      expect(prompt).toContain('value');
    });
  });

  describe('extractSchemaInfo', () => {
    it('should extract descriptions from Zod schema', () => {
      const schema = z.object({
        name: z.string().describe('User name'),
        age: z.number().describe('User age'),
        email: z.string(), // No description
      });

      const descriptions = extractSchemaInfo(schema);

      expect(descriptions).toEqual({
        name: 'User name',
        age: 'User age',
      });
    });

    it('should return empty object for non-object schema', () => {
      const schema = z.string();
      const descriptions = extractSchemaInfo(schema);

      expect(descriptions).toEqual({});
    });
  });
});