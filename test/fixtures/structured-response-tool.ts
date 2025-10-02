/**
 * Fixture tool for testing structured response formatting
 */

import { z } from 'zod';
import type { Tool } from '@/types/tool';
import { Success } from '@/types';

// Schema with options for different response types
const structuredResponseSchema = z.object({
  responseType: z
    .enum(['summary-with-data', 'summary-only', 'data-only', 'primitive'])
    .default('summary-with-data'),
});

type StructuredResponseInput = z.infer<typeof structuredResponseSchema>;

/**
 * Fixture tool that returns different structured response formats
 * for testing MCP content block formatting.
 */
const tool: Tool<typeof structuredResponseSchema, unknown> = {
  name: 'structured-response-fixture',
  description: 'Test fixture for structured response formatting',
  version: '1.0.0',
  schema: structuredResponseSchema,

  async run(input: StructuredResponseInput) {
    switch (input.responseType) {
      case 'summary-with-data':
        // Returns both summary and data - should emit 2 text blocks
        return Success({
          summary: 'Processed 3 items successfully',
          data: {
            items: ['item1', 'item2', 'item3'],
            metrics: { processed: 3, failed: 0, duration: 125 },
            timestamp: '2025-10-01T12:00:00Z',
          },
        });

      case 'summary-only':
        // Returns only summary - should emit 1 text block
        return Success({
          summary: 'Operation completed successfully',
        });

      case 'data-only':
        // Returns object without summary - should emit 1 JSON text block
        return Success({
          items: ['item1', 'item2', 'item3'],
          count: 3,
        });

      case 'primitive':
        // Returns primitive value - should emit 1 text block
        return Success('Simple string result');

      default:
        return Success({ summary: 'Default response' });
    }
  },
};

export default tool;
