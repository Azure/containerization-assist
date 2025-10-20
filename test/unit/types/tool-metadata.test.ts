/**
 * Tests for tool metadata validation
 */

import { validateAllToolMetadata, type ValidatableTool } from '@/types/tool-metadata';

describe('Tool Metadata Validation', () => {
  describe('validateAllToolMetadata', () => {
    it('should validate tools with correct metadata', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'test-tool-1',
          metadata: { knowledgeEnhanced: true },
        },
        {
          name: 'test-tool-2',
          metadata: { knowledgeEnhanced: false },
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.totalTools).toBe(2);
        expect(result.value.summary.validTools).toBe(2);
        expect(result.value.summary.invalidTools).toBe(0);
        expect(result.value.summary.compliancePercentage).toBe(100);
        expect(result.value.validTools).toEqual(['test-tool-1', 'test-tool-2']);
        expect(result.value.invalidTools).toEqual([]);
        expect(result.value.metadataErrors).toEqual([]);
      }
    });

    it('should detect missing knowledgeEnhanced field', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'invalid-tool',
          metadata: {} as any, // Missing knowledgeEnhanced
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.totalTools).toBe(1);
        expect(result.value.summary.validTools).toBe(0);
        expect(result.value.summary.invalidTools).toBe(1);
        expect(result.value.summary.compliancePercentage).toBe(0);
        expect(result.value.metadataErrors).toHaveLength(1);
        expect(result.value.metadataErrors[0].name).toBe('invalid-tool');
        expect(result.value.metadataErrors[0].error).toContain('knowledgeEnhanced');
        expect(result.value.invalidTools[0].issues).toContain('Invalid metadata schema');
        expect(result.value.invalidTools[0].suggestions).toContain('Fix metadata schema validation errors');
      }
    });

    it('should detect invalid knowledgeEnhanced type', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'invalid-tool',
          metadata: { knowledgeEnhanced: 'yes' } as any, // Wrong type
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.invalidTools).toBe(1);
        expect(result.value.metadataErrors).toHaveLength(1);
        expect(result.value.metadataErrors[0].error).toContain('boolean');
      }
    });

    it('should handle mixed valid and invalid tools', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'valid-tool',
          metadata: { knowledgeEnhanced: true },
        },
        {
          name: 'invalid-tool',
          metadata: {} as any,
        },
        {
          name: 'another-valid-tool',
          metadata: { knowledgeEnhanced: false },
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.totalTools).toBe(3);
        expect(result.value.summary.validTools).toBe(2);
        expect(result.value.summary.invalidTools).toBe(1);
        expect(result.value.summary.compliancePercentage).toBe(67); // Rounded
        expect(result.value.validTools).toEqual(['valid-tool', 'another-valid-tool']);
        expect(result.value.invalidTools).toHaveLength(1);
        expect(result.value.invalidTools[0].name).toBe('invalid-tool');
      }
    });

    it('should handle empty tool list', async () => {
      const tools: ValidatableTool[] = [];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.totalTools).toBe(0);
        expect(result.value.summary.validTools).toBe(0);
        expect(result.value.summary.invalidTools).toBe(0);
        expect(result.value.validTools).toEqual([]);
        expect(result.value.invalidTools).toEqual([]);
      }
    });

    it('should calculate compliance percentage correctly', async () => {
      const tools: ValidatableTool[] = [
        { name: 'tool-1', metadata: { knowledgeEnhanced: true } },
        { name: 'tool-2', metadata: { knowledgeEnhanced: false } },
        { name: 'tool-3', metadata: {} as any }, // Invalid
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // 2 out of 3 valid = 66.67% ≈ 67%
        expect(result.value.summary.compliancePercentage).toBe(67);
      }
    });

    it('should provide suggestions for metadata errors', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'invalid-tool',
          metadata: { knowledgeEnhanced: null } as any,
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.invalidTools[0].suggestions).toContain(
          'Fix metadata schema validation errors'
        );
      }
    });

    it('should handle extra metadata fields gracefully', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'tool-with-extras',
          metadata: {
            knowledgeEnhanced: true,
            extraField: 'extra-value',
          } as any,
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Zod by default strips unknown fields, so this should still be valid
        expect(result.value.summary.validTools).toBe(1);
        expect(result.value.validTools).toContain('tool-with-extras');
      }
    });

    it('should handle null metadata', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'null-metadata-tool',
          metadata: null as any,
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.invalidTools).toBe(1);
        expect(result.value.metadataErrors).toHaveLength(1);
      }
    });

    it('should handle undefined metadata', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'undefined-metadata-tool',
          metadata: undefined as any,
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.invalidTools).toBe(1);
        expect(result.value.metadataErrors).toHaveLength(1);
      }
    });

    it('should handle string metadata', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'string-metadata-tool',
          metadata: 'not-an-object' as any,
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.invalidTools).toBe(1);
        expect(result.value.metadataErrors).toHaveLength(1);
      }
    });

    it('should return error metadata for each invalid tool', async () => {
      const tools: ValidatableTool[] = [
        { name: 'invalid-1', metadata: {} as any },
        { name: 'valid', metadata: { knowledgeEnhanced: true } },
        { name: 'invalid-2', metadata: { knowledgeEnhanced: 'yes' } as any },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.metadataErrors).toHaveLength(2);
        expect(result.value.metadataErrors[0].name).toBe('invalid-1');
        expect(result.value.metadataErrors[1].name).toBe('invalid-2');
        expect(result.value.invalidTools).toHaveLength(2);
      }
    });

    it('should include both issues and suggestions for invalid tools', async () => {
      const tools: ValidatableTool[] = [
        {
          name: 'problematic-tool',
          metadata: { knowledgeEnhanced: 123 } as any,
        },
      ];

      const result = await validateAllToolMetadata(tools);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const invalidTool = result.value.invalidTools[0];
        expect(invalidTool.issues).toHaveLength(1);
        expect(invalidTool.suggestions).toHaveLength(1);
        expect(invalidTool.issues[0]).toBe('Invalid metadata schema');
        expect(invalidTool.suggestions[0]).toBe('Fix metadata schema validation errors');
      }
    });
  });
});
