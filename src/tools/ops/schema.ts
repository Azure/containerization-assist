/**
 * Schema definition for ops tool
 */

import { z } from 'zod';

export const opsToolSchema = z.object({
  operation: z.enum(['ping', 'status']).describe('Operation to perform'),
  message: z.string().optional().describe('Message for ping operation'),
  details: z.boolean().optional().describe('Include detailed information in status'),
});

export type OpsToolParams = z.infer<typeof opsToolSchema>;
