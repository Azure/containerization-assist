/**
 * Scan Image Tool - Standardized Implementation
 *
 * Scans Docker images for security vulnerabilities
 * Uses standardized helpers for consistency
 */

import { setupToolContext } from '@/lib/tool-context-helpers';
import type { ToolContext } from '@/core/context';

import { createSecurityScanner } from '@/infra/security/scanner';
import { Success, Failure, type Result } from '@/types';
import { getKnowledgeForCategory } from '@/knowledge/index';
import type { KnowledgeMatch } from '@/knowledge/types';
import { scanImageSchema, type ScanImageParams } from './schema';
import { formatVulnerabilities, buildStatusSummary, pluralize } from '@/lib/summary-helpers';

interface DockerScanResult {
  vulnerabilities?: Array<{
    id?: string;
    severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'NEGLIGIBLE' | 'UNKNOWN';
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
    negligible: number;
    unknown: number;
    total: number;
  };
  scanTime?: string;
  metadata?: {
    image: string;
  };
}

export interface ScanImageResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the scan outcome, vulnerability counts, and recommendations.
   * @example "🔒 Security scan failed. Found 142 vulnerabilities (2 critical, 5 high, 12 medium). 1 remediation recommendation available."
   */
  summary?: string;
  success: boolean;
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
    negligible: number;
    unknown: number;
    total: number;
  };
  scanTime: string;
  passed: boolean;
}

/**
 * Scan image handler - direct execution without wrapper
 */
async function handleScanImage(
  params: ScanImageParams,
  context: ToolContext,
): Promise<Result<ScanImageResult>> {
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided', {
      message: 'Parameters must be a valid object',
      hint: 'Tool received invalid or missing parameters',
      resolution: 'Ensure parameters are provided as a JSON object',
    });
  }
  const { logger, timer } = setupToolContext(context, 'scan-image');

  const { scanner = 'trivy', severity } = params;

  // Map severity parameter to threshold
  const finalSeverityThreshold = severity
    ? (severity.toLowerCase() as 'low' | 'medium' | 'high' | 'critical')
    : 'high';

  try {
    logger.info(
      { scanner, severityThreshold: finalSeverityThreshold },
      'Starting image security scan',
    );

    const securityScanner = createSecurityScanner(logger, scanner);

    const imageId = params.imageId;

    if (!imageId) {
      return Failure('No image specified. Provide imageId parameter.', {
        message: 'Missing required parameter: imageId',
        hint: 'Image ID or name must be specified to scan',
        resolution: 'Add imageId parameter with the Docker image ID or name to scan',
      });
    }
    logger.info({ imageId, scanner }, 'Scanning image for vulnerabilities');

    // Scan image using security scanner
    const scanResultWrapper = await securityScanner.scanImage(imageId);

    if (!scanResultWrapper.ok) {
      return Failure(
        `Failed to scan image: ${scanResultWrapper.error ?? 'Unknown error'}`,
        scanResultWrapper.guidance,
      );
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
        negligible: scanResult.negligibleCount,
        unknown: scanResult.unknownCount,
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
          ...securityKnowledge.map((match: KnowledgeMatch) => ({
            vulnerability: 'General',
            recommendation: match.entry.recommendation,
            ...(match.entry.severity && { severity: match.entry.severity }),
            ...(match.entry.example && { example: match.entry.example }),
          })),
          ...generalKnowledge.map((match: KnowledgeMatch) => ({
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

    // Generate summary
    const vulnSummary = formatVulnerabilities({
      critical: scanResult.criticalCount,
      high: scanResult.highCount,
      medium: scanResult.mediumCount,
      low: scanResult.lowCount,
      total: scanResult.totalVulnerabilities,
    });

    const remediationText =
      remediationGuidance.length > 0
        ? ` ${pluralize(remediationGuidance.length, 'remediation')} available.`
        : '';

    const summary = buildStatusSummary(
      passed,
      `🔒 Security scan passed. ${vulnSummary}.${remediationText}`,
      `🔒 Security scan failed. ${vulnSummary}.${remediationText}`,
    );

    // Prepare the result
    const result: ScanImageResult = {
      summary,
      success: true,
      ...(remediationGuidance.length > 0 && { remediationGuidance }),
      vulnerabilities: {
        critical: scanResult.criticalCount,
        high: scanResult.highCount,
        medium: scanResult.mediumCount,
        low: scanResult.lowCount,
        negligible: scanResult.negligibleCount,
        unknown: scanResult.unknownCount,
        total: scanResult.totalVulnerabilities,
      },
      scanTime: dockerScanResult.scanTime ?? new Date().toISOString(),
      passed,
    };

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
    return Failure(errorMessage, {
      message: errorMessage,
      hint: 'An unexpected error occurred during the security scan',
      resolution: 'Verify that the scanner (Trivy) is installed and accessible, the image exists, and you have proper permissions',
    });
  }
}

/**
 * Scan image tool
 */
export const scanImage = handleScanImage;

import { tool } from '@/types/tool';

export default tool({
  name: 'scan-image',
  description:
    'Scan Docker images for security vulnerabilities with knowledge-based remediation guidance',
  category: 'security',
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
  handler: handleScanImage,
});
