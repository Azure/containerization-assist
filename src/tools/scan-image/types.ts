import { scanImageSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const scanImageToolDefinition = {
  name: TOOL_NAME.SCAN_IMAGE,
  description:
    'Scan Docker images for security vulnerabilities with knowledge-based remediation guidance',
  category: 'security' as const,
  version: '2.0.0',
  schema: scanImageSchema,
  metadata: {
    knowledgeEnhanced: true,
  },
  chainHints: {
    success:
      'Security scan passed! Proceed with push-image to push to a registry, or continue with deployment preparation.',
    failure:
      'Security scan found vulnerabilities. Use fix-dockerfile to address security issues in your base images and dependencies.',
  },
} satisfies IToolDefinition;
