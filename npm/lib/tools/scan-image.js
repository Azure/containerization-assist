import { createTool, z } from './_tool-factory.js';

export default createTool({
  name: 'scan_image',
  title: 'Scan Docker Image',
  description: 'Scan the Docker image for security vulnerabilities',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    scanners: z.array(z.enum(['trivy', 'grype'])).optional()
      .describe('Security scanners to use'),
    severity: z.string().optional()
      .describe('Minimum severity level to report (e.g., "HIGH,CRITICAL")'),
    ignore_unfixed: z.boolean().optional()
      .describe('Ignore vulnerabilities without fixes')
  }
});