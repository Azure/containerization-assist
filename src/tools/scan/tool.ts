/**
 * Scan Image Tool - Standardized Implementation
 *
 * Scans Docker images for security vulnerabilities
 * Uses standardized helpers for consistency
 */

import { getSession, updateSession } from '@mcp/tool-session-helpers';
import type { ToolContext } from '../../mcp/context';
import { createTimer, createLogger } from '../../lib/logger';
import { createSecurityScanner } from '../../lib/scanner';
import { Success, Failure, type Result } from '../../types';
import { getKnowledgeForCategory } from '../../knowledge';
import type { ScanImageParams } from './schema';
import type { SessionData } from '../session-types';
import {
  getSuccessChainHint,
  getFailureHint,
  formatChainHint,
  type SessionContext,
} from '../../lib/chain-hints';
import { TOOL_NAMES } from '../../exports/tools.js';

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
  const logger = context.logger || createLogger({ name: 'scan' });
  const timer = createTimer(logger, 'scan-image');

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

    // Resolve session (now always optional)
    const sessionResult = await getSession(params.sessionId, context);

    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId, scanner }, 'Starting image security scan');

    const securityScanner = createSecurityScanner(logger, scanner);

    // Check for built image in session or use provided imageId
    const sessionData = session as SessionData;
    const buildResult = sessionData?.build_result;
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

    // Update session with scan results using standardized helper
    const updateResult = await updateSession(
      sessionId,
      {
        scan_result: {
          success: passed,
          vulnerabilities: dockerScanResult.vulnerabilities?.map((v) => ({
            id: v.id ?? 'unknown',
            severity: v.severity,
            package: v.package ?? 'unknown',
            version: v.version ?? 'unknown',
            description: v.description ?? '',
            ...(v.fixedVersion && { fixedVersion: v.fixedVersion }),
          })),
          summary: dockerScanResult.summary,
        },
        completed_steps: [...(session.completed_steps || []), 'scan'],
        metadata: {
          ...session.metadata,
          scanTime: dockerScanResult.scanTime ?? new Date().toISOString(),
          scanner,
          scanPassed: passed,
        },
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn({ error: updateResult.error }, 'Failed to update session, but scan succeeded');
    }

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

    // Prepare session context for dynamic chain hints
    const sessionContext: SessionContext = {
      completed_steps: session.completed_steps || [],
      ...((session as SessionContext).analysis_result && {
        analysis_result: (session as SessionContext).analysis_result,
      }),
    };

    return Success({
      success: true,
      sessionId,
      imageId,
      ...(remediationGuidance && remediationGuidance.length > 0 && { remediationGuidance }),
      vulnerabilities: {
        critical: dockerScanResult.summary?.critical ?? 0,
        high: dockerScanResult.summary?.high ?? 0,
        medium: dockerScanResult.summary?.medium ?? 0,
        low: dockerScanResult.summary?.low ?? 0,
        unknown: dockerScanResult.summary?.unknown ?? 0,
        total: dockerScanResult.summary?.total ?? 0,
      },
      scanTime: dockerScanResult.scanTime ?? new Date().toISOString(),
      passed,
      NextStep: getSuccessChainHint(TOOL_NAMES.SCAN_IMAGE, sessionContext),
    });
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Image scan failed');

    // Add failure chain hint - use basic context since session may not be available
    const sessionContext = {
      completed_steps: [],
    };
    const errorMessage = error instanceof Error ? error.message : String(error);
    const hint = getFailureHint(TOOL_NAMES.SCAN_IMAGE, errorMessage, sessionContext);
    const chainHint = formatChainHint(hint);

    return Failure(`${errorMessage}\n${chainHint}`);
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
