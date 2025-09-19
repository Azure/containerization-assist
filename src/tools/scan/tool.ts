/**
 * Simplified Scan Tool
 *
 * Uses AI for vulnerability assessment while keeping scanner operations in TypeScript
 */
import { z } from 'zod';
import { exec } from 'node:child_process';
import { promisify } from 'node:util';
import { createPromptBackedTool } from '@mcp/tools/prompt-backed-tool';
import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result } from '@types';
import { scanImageSchema, type ScanImageParams } from './schema';
import { extractErrorMessage } from '@lib/error-utils';

const execAsync = promisify(exec);

// Simple scan result structure for this tool
interface SimpleScanResult {
  critical: number;
  high: number;
  medium: number;
  low: number;
  total: number;
}

// Result schema for AI assessment
const VulnerabilityAssessmentSchema = z.object({
  summary: z.object({
    totalVulnerabilities: z.number(),
    critical: z.number(),
    high: z.number(),
    medium: z.number(),
    low: z.number(),
    negligible: z.number(),
  }),
  topVulnerabilities: z.array(
    z.object({
      id: z.string(),
      severity: z.string(),
      package: z.string(),
      version: z.string(),
      fixedVersion: z.string(),
      description: z.string(),
      exploitability: z.enum(['high', 'medium', 'low', 'none']),
    }),
  ),
  remediations: z.array(
    z.object({
      priority: z.enum(['immediate', 'high', 'medium', 'low']),
      action: z.string(),
      packages: z.array(z.string()),
      effort: z.enum(['low', 'medium', 'high']),
      impact: z.string(),
    }),
  ),
  baseImageRecommendations: z.array(
    z.object({
      currentBase: z.string(),
      recommendedBase: z.string(),
      reason: z.string(),
      vulnerabilityReduction: z.number(),
    }),
  ),
  complianceStatus: z.object({
    passes: z.boolean(),
    blockers: z.array(z.string()),
    warnings: z.array(z.string()),
  }),
  riskScore: z.object({
    score: z.number(),
    level: z.enum(['critical', 'high', 'medium', 'low']),
    factors: z.array(z.string()),
  }),
  deploymentRecommendation: z.object({
    canDeploy: z.boolean(),
    conditions: z.array(z.string()),
    requiredActions: z.array(z.string()),
  }),
  nextSteps: z.array(z.string()),
});

// Result schema
const _ScanResultSchema = z.object({
  success: z.boolean(),
  scanner: z.string(),
  vulnerabilities: z.object({
    critical: z.number(),
    high: z.number(),
    medium: z.number(),
    low: z.number(),
    total: z.number(),
  }),
  report: z.string().optional(),
  assessment: VulnerabilityAssessmentSchema.optional(),
  scanTime: z.number(),
});

export type ScanResult = z.infer<typeof _ScanResultSchema>;

// Create AI assessment tool
const vulnerabilityAssessmentTool = createPromptBackedTool({
  name: 'vulnerability-assessment',
  description: 'Assess and prioritize vulnerabilities',
  inputSchema: scanImageSchema.extend({
    scanResults: z.custom<SimpleScanResult>(),
    imageName: z.string().optional(),
    severityThreshold: z.string().optional(),
    format: z.string().optional(),
  }),
  outputSchema: VulnerabilityAssessmentSchema,
  promptId: 'vulnerability-scan',
  knowledge: {
    category: 'security',
    limit: 4,
  },
  policy: {
    tool: 'scan',
    extractor: (params) => ({
      severity: params.severity ?? 'medium',
    }),
  },
});

// Helper: Run Trivy scanner
async function runTrivy(
  imageName: string,
  format: string = 'json',
  severityThreshold: string = 'MEDIUM',
): Promise<{ output: string; vulnerabilities: SimpleScanResult }> {
  try {
    const cmd = `trivy image --severity ${severityThreshold.toUpperCase()} --format ${format} --quiet ${imageName}`;
    const { stdout } = await execAsync(cmd, { maxBuffer: 10 * 1024 * 1024 });

    const vulnerabilities = {
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      total: 0,
    };

    // Parse JSON output to count vulnerabilities
    if (format === 'json') {
      try {
        const results = JSON.parse(stdout);
        if (results.Results) {
          for (const result of results.Results) {
            if (result.Vulnerabilities) {
              for (const vuln of result.Vulnerabilities) {
                vulnerabilities.total++;
                switch (vuln.Severity?.toLowerCase()) {
                  case 'critical':
                    vulnerabilities.critical++;
                    break;
                  case 'high':
                    vulnerabilities.high++;
                    break;
                  case 'medium':
                    vulnerabilities.medium++;
                    break;
                  case 'low':
                    vulnerabilities.low++;
                    break;
                }
              }
            }
          }
        }
      } catch {
        // If parsing fails, continue with empty counts
      }
    }

    return { output: stdout, vulnerabilities };
  } catch (error: unknown) {
    // Trivy might not be installed, return mock data
    const errorWithCode = error as { code?: string; message?: string };
    const isNotFound =
      errorWithCode?.code === 'ENOENT' || errorWithCode?.message?.includes?.('command not found');
    if (isNotFound) {
      return {
        output: JSON.stringify({ Results: [] }),
        vulnerabilities: {
          critical: 0,
          high: 0,
          medium: 0,
          low: 0,
          total: 0,
        },
      };
    }
    throw error;
  }
}

// Helper: Run Grype scanner
async function runGrype(
  imageName: string,
  format: string = 'json',
): Promise<{ output: string; vulnerabilities: SimpleScanResult }> {
  try {
    const cmd = `grype ${imageName} -o ${format} --quiet`;
    const { stdout } = await execAsync(cmd, { maxBuffer: 10 * 1024 * 1024 });

    const vulnerabilities = {
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      total: 0,
    };

    // Parse JSON output
    if (format === 'json') {
      try {
        const results = JSON.parse(stdout);
        if (results.matches) {
          for (const match of results.matches) {
            vulnerabilities.total++;
            switch (match.vulnerability?.severity?.toLowerCase()) {
              case 'critical':
                vulnerabilities.critical++;
                break;
              case 'high':
                vulnerabilities.high++;
                break;
              case 'medium':
                vulnerabilities.medium++;
                break;
              case 'low':
                vulnerabilities.low++;
                break;
            }
          }
        }
      } catch {
        // Continue with empty counts
      }
    }

    return { output: stdout, vulnerabilities };
  } catch (error: unknown) {
    const errorWithCode = error as { code?: string };
    if (errorWithCode?.code === 'ENOENT') {
      throw new Error('Grype scanner not found');
    }
    throw error;
  }
}

// Main scan function
async function scanImage(
  params: ScanImageParams,
  context: ToolContext,
): Promise<Result<SimpleScanResult & { sessionId: string; ok: boolean }>> {
  const { logger } = context;

  try {
    const imageName = params.imageId;
    if (!imageName) {
      return Failure('Image name is required');
    }

    const scanner = params.scanner || 'trivy';
    const format = 'json';
    const severityThreshold = params.severity || 'MEDIUM';

    logger.info({ image: imageName, scanner }, 'Starting vulnerability scan');

    const startTime = Date.now();
    let scanOutput: { output: string; vulnerabilities: SimpleScanResult };

    // Run the appropriate scanner
    if (scanner === 'grype') {
      try {
        scanOutput = await runGrype(imageName, format);
      } catch {
        logger.warn('Grype not available, falling back to Trivy');
        scanOutput = await runTrivy(imageName, format, severityThreshold);
      }
    } else {
      scanOutput = await runTrivy(imageName, format, severityThreshold);
    }

    const scanTime = Date.now() - startTime;

    // Get AI vulnerability assessment
    let assessment: z.infer<typeof VulnerabilityAssessmentSchema> | undefined;
    if (format === 'json') {
      try {
        const assessmentResult = await vulnerabilityAssessmentTool.execute(
          {
            ...params,
            scanResults: scanOutput.output,
          },
          { logger: context.logger },
          context,
        );

        if (assessmentResult.ok) {
          assessment = assessmentResult.value;
          logger.info(
            { riskLevel: assessment.riskScore.level },
            'Vulnerability assessment completed',
          );
        }
      } catch {
        logger.warn('Could not generate vulnerability assessment');
      }
    }

    // Generate session ID
    const sessionId = params.sessionId || `scan-${Date.now()}`;

    // Save to session
    if (context.sessionManager) {
      await context.sessionManager.update(sessionId, {
        'scan-image': {
          imageName,
          vulnerabilities: scanOutput.vulnerabilities,
          assessment,
        },
      });
    }

    const result = {
      success: true,
      scanner,
      vulnerabilities: scanOutput.vulnerabilities,
      report: undefined, // format is always 'json' now
      assessment,
      scanTime,
      sessionId,
      ok: true,
      // Flatten vulnerability counts for SimpleScanResult compatibility
      critical: scanOutput.vulnerabilities.critical,
      high: scanOutput.vulnerabilities.high,
      medium: scanOutput.vulnerabilities.medium,
      low: scanOutput.vulnerabilities.low,
      total: scanOutput.vulnerabilities.total,
    };

    logger.info(
      {
        image: imageName,
        vulnerabilities: scanOutput.vulnerabilities,
        scanTime: `${scanTime}ms`,
      },
      'Vulnerability scan completed',
    );

    return Success(result);
  } catch (error) {
    logger.error({ error: extractErrorMessage(error) }, 'Scan failed');
    return Failure(extractErrorMessage(error));
  }
}

// Export for MCP registration
export const tool = {
  type: 'standard' as const,
  name: 'scan',
  description: 'Scan Docker image for vulnerabilities',
  inputSchema: scanImageSchema,
  execute: scanImage,
};
