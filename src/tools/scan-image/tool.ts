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
import { type ScanImageParams } from './schema';
import { formatVulnerabilities, buildStatusSummary, pluralize } from '@/lib/summary-helpers';
import { scanImageToolDefinition } from './types';
import { resolveDockerContext } from '@/infra/docker/context';

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

/**
 * Actionable fix recommendation that groups related vulnerabilities
 */
export interface FixAction {
  type: 'UPGRADE_PACKAGE';
  action: string;
  current: string;
  recommended: string;
  package: string;
  vulnerabilitiesFixed: number;
  severityCounts: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    negligible: number;
    unknown: number;
  };
  vulnerabilityIds: string[];
}

export interface ScanImageResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the scan outcome, vulnerability counts, and recommendations.
   * @example "🔒 Security scan failed. Found 142 vulnerabilities (2 critical, 5 high, 12 medium). 1 remediation recommendation available."
   */
  summary?: string;
  success: boolean;
  scanner: string;
  /**
   * The Docker context used for this scan.
   * Only set when a specific context was requested.
   */
  context?: string;
  recommendedActions?: FixAction[];
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
  vulnerabilityDetails?: Array<{
    id: string;
    severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'NEGLIGIBLE' | 'UNKNOWN';
    package: string;
    version: string;
    description: string;
    fixedVersion?: string;
  }>;
  scanTime: string;
  passed: boolean;
}

/**
 * Analyze vulnerabilities and generate actionable fix recommendations
 * Groups vulnerabilities by package for cleaner output
 */
function analyzeFixActions(
  vulnerabilities: Array<{
    id: string;
    severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'NEGLIGIBLE' | 'UNKNOWN';
    package: string;
    version: string;
    fixedVersion?: string;
  }>,
): FixAction[] {
  const fixable = vulnerabilities.filter((v) => v.fixedVersion !== undefined);
  if (fixable.length === 0) return [];

  const normalizeVersion = (value: string | undefined): string => {
    if (value === undefined) return 'unknown';
    return value.trim() === '' ? 'unknown' : value;
  };

  const byPackageVersion = new Map<string, typeof fixable>();
  for (const vuln of fixable) {
    const currentVersion = normalizeVersion(vuln.version);
    const fixedVersion = normalizeVersion(vuln.fixedVersion);
    const key = `${vuln.package}::${currentVersion}::${fixedVersion}`;
    const grouped = byPackageVersion.get(key) || [];
    grouped.push(vuln);
    byPackageVersion.set(key, grouped);
  }

  const actions: FixAction[] = Array.from(byPackageVersion.entries()).map(([, vulns]) => {
    const severityCounts = {
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      negligible: 0,
      unknown: 0,
    };

    for (const vuln of vulns) {
      switch (vuln.severity) {
        case 'CRITICAL':
          severityCounts.critical += 1;
          break;
        case 'HIGH':
          severityCounts.high += 1;
          break;
        case 'MEDIUM':
          severityCounts.medium += 1;
          break;
        case 'LOW':
          severityCounts.low += 1;
          break;
        case 'NEGLIGIBLE':
          severityCounts.negligible += 1;
          break;
        case 'UNKNOWN':
          severityCounts.unknown += 1;
          break;
      }
    }

    const vulnerabilityIds = [...new Set(vulns.map((v) => v.id))].slice(0, 5);
    const packageName = vulns[0]?.package ?? 'unknown';
    const currentVersion = normalizeVersion(vulns[0]?.version);
    const fixedVersion = normalizeVersion(vulns[0]?.fixedVersion);

    return {
      type: 'UPGRADE_PACKAGE',
      action: `Upgrade ${packageName}`,
      current: `${packageName}: ${currentVersion}`,
      recommended: `${packageName}: ${fixedVersion}`,
      package: packageName,
      vulnerabilitiesFixed: vulns.length,
      severityCounts,
      vulnerabilityIds,
    };
  });

  const severityOrder: Array<keyof FixAction['severityCounts']> = [
    'critical',
    'high',
    'medium',
    'low',
    'negligible',
    'unknown',
  ];

  actions.sort((a, b) => {
    for (const severity of severityOrder) {
      if (b.severityCounts[severity] !== a.severityCounts[severity]) {
        return b.severityCounts[severity] - a.severityCounts[severity];
      }
    }
    return b.vulnerabilitiesFixed - a.vulnerabilitiesFixed;
  });

  return actions.slice(0, 5);
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

  const {
    scanner = 'osv',
    severity,
    scanType = 'vulnerability',
    enableAISuggestions = true,
    context: dockerContext,
  } = params;

  // Normalize scanType: "all" defaults to "vulnerability" until other scan types are supported
  const effectiveScanType = scanType === 'all' ? 'vulnerability' : scanType;

  if (effectiveScanType !== 'vulnerability') {
    return Failure(`Scan type '${scanType}' is not supported`, {
      message: 'Only vulnerability scans are currently supported',
      hint: 'Use scanType="vulnerability" for image vulnerability checks',
      resolution: 'Update scanType to "vulnerability" and retry',
    });
  }

  // Map severity parameter to threshold
  const finalSeverityThreshold = severity
    ? (severity.toLowerCase() as 'low' | 'medium' | 'high' | 'critical')
    : 'high';

  // Resolve Docker context to endpoint if a specific context is requested
  let dockerHost: string | undefined;
  if (dockerContext) {
    const resolved = await resolveDockerContext(dockerContext, logger);
    if (!resolved.ok) {
      return Failure(resolved.error, resolved.guidance);
    }
    dockerHost = resolved.value;
    logger.info({ context: dockerContext }, 'Using Docker context');
    logger.debug({ context: dockerContext, dockerHost }, 'Resolved Docker context endpoint');
  }

  try {
    logger.info(
      { scanner, severityThreshold: finalSeverityThreshold, scanType: effectiveScanType },
      'Starting image security scan',
    );

    const securityScanner = createSecurityScanner(logger, scanner, dockerHost);

    const imageId = params.imageId;

    if (!imageId) {
      return Failure('No image specified. Provide imageId parameter.', {
        message: 'Missing required parameter: imageId',
        hint: 'Image ID or name must be specified to scan',
        resolution: 'Add imageId parameter with the Docker image ID or name to scan',
      });
    }
    logger.info({ imageId, scanner, scanType: effectiveScanType }, 'Scanning image for vulnerabilities');

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
    if (
      enableAISuggestions &&
      dockerScanResult.vulnerabilities &&
      dockerScanResult.vulnerabilities.length > 0
    ) {
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

    const contextLabel = dockerContext ? ` [context: ${dockerContext}]` : '';
    const remediationText =
      remediationGuidance.length > 0
        ? ` ${pluralize(remediationGuidance.length, 'remediation')} available.`
        : '';

    const summary = buildStatusSummary(
      passed,
      `🔒 Security scan passed (${scanner})${contextLabel}. ${vulnSummary}.${remediationText}`,
      `🔒 Security scan failed (${scanner})${contextLabel}. ${vulnSummary}.${remediationText}`,
    );

    const vulnerabilityDetails =
      dockerScanResult.vulnerabilities && dockerScanResult.vulnerabilities.length > 0
        ? dockerScanResult.vulnerabilities.map((v) => ({
            id: v.id ?? 'UNKNOWN',
            severity: v.severity,
            package: v.package ?? 'unknown',
            version: v.version ?? 'unknown',
            description: v.description ?? 'No description available',
            ...(v.fixedVersion !== undefined && { fixedVersion: v.fixedVersion }),
          }))
        : undefined;

    const recommendedActions =
      vulnerabilityDetails && vulnerabilityDetails.length > 0
        ? analyzeFixActions(vulnerabilityDetails)
        : undefined;

    const result: ScanImageResult = {
      summary,
      success: true,
      scanner,
      ...(dockerContext && { context: dockerContext }),
      ...(recommendedActions && recommendedActions.length > 0 && { recommendedActions }),
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
      ...(vulnerabilityDetails && { vulnerabilityDetails }),
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
      resolution:
        'Verify that the scanner is installed and accessible, the image exists, and you have proper permissions',
    });
  }
}

/**
 * Scan image tool
 */
export const scanImage = handleScanImage;

import { tool } from '@/types/tool';

export default tool({
  ...scanImageToolDefinition,
  handler: handleScanImage,
});
