/**
 * Unit Tests: Resolve Base Images Tool (Simplified)
 * Tests for the simplified AI-driven base image resolution
 */

import { jest } from '@jest/globals';
import { resolveBaseImages } from '../../../src/tools/resolve-base-images/tool';
import type { ResolveBaseImagesParams } from '../../../src/tools/resolve-base-images/schema';
import type { ToolContext } from '../../../src/mcp/context';

describe('resolveBaseImagesTool (Simplified)', () => {
  // Helper function to create a mock context with sampling
  function createMockContext(overrides?: Partial<ToolContext>): ToolContext {
    return {
      sampling: {
        createMessage: jest.fn().mockResolvedValue({
          content: [{ text: JSON.stringify({ recommendations: 'node:18-alpine for production use' }) }],
        }),
      },
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
        debug: jest.fn(),
        trace: jest.fn(),
        fatal: jest.fn(),
        child: jest.fn().mockReturnThis(),
      },
      ...overrides,
    } as unknown as ToolContext;
  }

  describe('successful base image resolution', () => {
    it('should resolve base images for JavaScript application', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'javascript',
      };

      const mockContext = createMockContext();
      const result = await resolveBaseImages(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('recommendations');
        expect(typeof result.value.recommendations).toBe('string');
      }

      // Verify AI was called
      expect(mockContext.sampling.createMessage).toHaveBeenCalled();
    });

    it('should handle Python applications', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'python',
      };

      const mockContext = createMockContext();
      const result = await resolveBaseImages(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('recommendations');
      }
    });

    it('should use default values when technology not provided', async () => {
      const config: ResolveBaseImagesParams = {};

      const mockContext = createMockContext();
      const result = await resolveBaseImages(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('recommendations');
      }
    });
  });

  describe('error handling', () => {
    it('should handle AI response errors gracefully', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'javascript',
      };

      const mockContext = createMockContext({
        sampling: {
          createMessage: jest.fn().mockResolvedValue({
            content: [{ text: 'invalid json' }],
          }),
        },
      } as unknown as ToolContext);

      const result = await resolveBaseImages(config, mockContext);

      // With invalid JSON, tool returns the raw text as recommendations
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('recommendations', 'invalid json');
      }
    });

    it('should handle missing AI response', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'javascript',
      };

      const mockContext = createMockContext({
        sampling: {
          createMessage: jest.fn().mockResolvedValue({
            content: [],
          }),
        },
      } as unknown as ToolContext);

      const result = await resolveBaseImages(config, mockContext);

      // Should return with empty response
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('recommendations', '');
      }
    });
  });
});