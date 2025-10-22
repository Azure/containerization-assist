/**
 * Schema definition for ops tool
 *
 * Defines parameters for MCP server diagnostic operations.
 */

import { z } from 'zod';

export const opsToolSchema = z.object({
  operation: z.enum(['ping', 'status']).describe(
    'Diagnostic operation: "ping" tests server connectivity and responsiveness, "status" shows detailed resource metrics and health information',
  ),
  message: z.string().optional().describe(
    'Optional message for ping operation (will be echoed back in the response for testing)',
  ),
  details: z.boolean().optional().describe(
    'Include detailed system information in status operation (additional metrics and metadata)',
  ),
});

export type OpsToolParams = z.infer<typeof opsToolSchema>;
