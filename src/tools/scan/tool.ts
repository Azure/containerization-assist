/**
 * Scan Image Tool - Standardized Implementation
 *
 * Scans Docker images for security vulnerabilities
 * Uses standardized helpers for consistency
 */

import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import type { ToolContext } from '../../mcp/context';

import { createSecurityScanner } from '../../lib/scanner';
import { Success, Failure, type Result } from '../../types';
import { getKnowledgeForCategory } from '../../knowledge';
import { scanImageSchema, type ScanImageParams } from './schema';
import { z } from 'zod';

interface DockerScanResult {
  vulnerabilities?: Array<{
    id?: string;
    severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
    package?: string;
    version?: string;
    description?: string;
    fixedVersion?: string;
  }>;
  summary?: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    unknown?: number;
    total: number;
  };
  scanTime?: string;
  metadata?: {
    image: string;
  };
}

// Define the result schema for type safety
const ScanImageResultSchema = z.object({
  success: z.boolean(),
  sessionId: z.string(),
  remediationGuidance: z
    .array(
      z.object({
        vulnerability: z.string(),
        recommendation: z.string(),
        severity: z.string().optional(),
        example: z.string().optional(),
      }),
    )
    .optional(),
  vulnerabilities: z.object({
    critical: z.number(),
    high: z.number(),
    medium: z.number(),
    low: z.number(),
    unknown: z.number(),
    total: z.number(),
  }),
  scanTime: z.string(),
  passed: z.boolean(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(scanImageSchema, ScanImageResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastScannedAt: z.date().optional(),
  lastScannedImage: z.string().optional(),
  vulnerabilityCount: z.number().optional(),
  scannerUsed: z.string().optional(),
});

export interface ScanImageResult {
  success: boolean;
  sessionId: string;
  remediationGuidance?: Array<{
    vulnerability: string;
    recommendation: string;
    severity?: string;
    example?: string;
  }>;
  vulnerabilities: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    unknown: number;
    total: number;
  };
  scanTime: string;
  passed: boolean;
}

/**
 * Scan image implementation - direct execution without wrapper
 */
async function scanImageImpl(
  params: ScanImageParams,
  context: ToolContext,
): Promise<Result<ScanImageResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = getToolLogger(context, 'scan');
  const timer = createToolTimer(logger, 'scan-image');

  try {
    const { scanner = 'trivy', severity } = params;

    // Map severity parameter to threshold
    const finalSeverityThreshold = severity
      ? (severity.toLowerCase() as 'low' | 'medium' | 'high' | 'critical')
      : 'high';

    logger.info(
      { scanner, severityThreshold: finalSeverityThreshold },
      'Starting image security scan',
    );

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('scan', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info(
      { sessionId, scanner, severityThreshold: finalSeverityThreshold },
      'Starting image security scan with session',
    );

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    const securityScanner = createSecurityScanner(logger, scanner);

    // Check for built image in session or use provided imageId
    const buildResult = session?.results?.['build-image'] as { imageId?: string } | undefined;
    const imageId = params.imageId || buildResult?.imageId;

    if (!imageId) {
      return Failure(
        'No image specified. Provide imageId parameter or ensure session has built image from build-image tool.',
      );
    }
    logger.info({ imageId, scanner }, 'Scanning image for vulnerabilities');

    // Scan image using security scanner
    const scanResultWrapper = await securityScanner.scanImage(imageId);

    if (!scanResultWrapper.ok) {
      return Failure(`Failed to scan image: ${scanResultWrapper.error ?? 'Unknown error'}`);
    }

    const scanResult = scanResultWrapper.value;

    // Convert BasicScanResult to DockerScanResult
    const dockerScanResult: DockerScanResult = {
      vulnerabilities: scanResult.vulnerabilities.map((v) => ({
        id: v.id,
        severity: v.severity,
        package: v.package,
        version: v.version,
        description: v.description,
        ...(v.fixedVersion !== undefined && { fixedVersion: v.fixedVersion }),
      })),
      summary: {
        critical: scanResult.criticalCount,
        high: scanResult.highCount,
        medium: scanResult.mediumCount,
        low: scanResult.lowCount,
        total: scanResult.totalVulnerabilities,
      },
      scanTime: scanResult.scanDate.toISOString(),
      metadata: {
        image: imageId,
      },
    };

    // Determine if scan passed based on threshold
    const thresholdMap = {
      critical: ['critical'],
      high: ['critical', 'high'],
      medium: ['critical', 'high', 'medium'],
      low: ['critical', 'high', 'medium', 'low'],
    };

    const failingSeverities = thresholdMap[finalSeverityThreshold] || thresholdMap['high'];
    let vulnerabilityCount = 0;

    for (const severity of failingSeverities) {
      if (severity === 'critical') {
        vulnerabilityCount += scanResult.criticalCount;
      } else if (severity === 'high') {
        vulnerabilityCount += scanResult.highCount;
      } else if (severity === 'medium') {
        vulnerabilityCount += scanResult.mediumCount;
      } else if (severity === 'low') {
        vulnerabilityCount += scanResult.lowCount;
      }
    }

    const passed = vulnerabilityCount === 0;

    // Get knowledge-based remediation guidance for vulnerabilities
    let remediationGuidance: ScanImageResult['remediationGuidance'] = [];
    if (dockerScanResult.vulnerabilities && dockerScanResult.vulnerabilities.length > 0) {
      try {
        // Create a summary of vulnerabilities for knowledge query
        const vulnSummary = dockerScanResult.vulnerabilities
          .slice(0, 10) // Limit to top 10 for performance
          .map((v) => `${v.package}:${v.version} (${v.severity})`)
          .join(', ');

        const securityKnowledge = await getKnowledgeForCategory('security', vulnSummary);

        // Add general security recommendations
        const generalKnowledge = await getKnowledgeForCategory('security', undefined);

        remediationGuidance = [
          ...securityKnowledge.map((match) => ({
            vulnerability: 'General',
            recommendation: match.entry.recommendation,
            ...(match.entry.severity && { severity: match.entry.severity }),
            ...(match.entry.example && { example: match.entry.example }),
          })),
          ...generalKnowledge.map((match) => ({
            vulnerability: 'Best Practice',
            recommendation: match.entry.recommendation,
            ...(match.entry.severity && { severity: match.entry.severity }),
            ...(match.entry.example && { example: match.entry.example }),
          })),
        ];

        logger.info(
          { guidanceCount: remediationGuidance.length },
          'Added knowledge-based remediation guidance',
        );
      } catch (error) {
        logger.debug({ error }, 'Failed to get remediation guidance, continuing without');
      }
    }

    // Prepare the result
    const result: ScanImageResult = {
      success: true,
      sessionId,
      ...(remediationGuidance.length > 0 && { remediationGuidance }),
      vulnerabilities: {
        critical: scanResult.criticalCount,
        high: scanResult.highCount,
        medium: scanResult.mediumCount,
        low: scanResult.lowCount,
        unknown: 0, // BasicScanResult doesn't track unknown severity vulnerabilities
        total: scanResult.totalVulnerabilities,
      },
      scanTime: dockerScanResult.scanTime ?? new Date().toISOString(),
      passed,
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastScannedAt: new Date(),
        lastScannedImage: imageId,
        vulnerabilityCount: scanResult.totalVulnerabilities,
        scannerUsed: scanner,
      },
    });

    timer.end({
      vulnerabilities: scanResult.totalVulnerabilities,
      critical: scanResult.criticalCount,
      high: scanResult.highCount,
      passed,
    });

    logger.info(
      {
        imageId,
        vulnerabilities: scanResult.totalVulnerabilities,
        passed,
      },
      'Image scan completed',
    );

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Image scan failed');

    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(errorMessage);
  }
}

/**
 * Type alias for test compatibility
 */
export type ScanImageConfig = ScanImageParams;

/**
 * Scan image tool
 */
export const scanImage = scanImageImpl;
