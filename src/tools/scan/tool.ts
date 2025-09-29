/**
 * Scan Image Tool - Standardized Implementation
 *
 * Scans Docker images for security vulnerabilities
 * Uses standardized helpers for consistency
 */

import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import type { ToolContext } from '@/mcp/context';

import { createSecurityScanner } from '@/lib/scanner';
import { Success, Failure, type Result } from '@/types';
import { getKnowledgeForCategory } from '@/knowledge/index';
import type { KnowledgeMatch } from '@/knowledge/types';
import { enhanceValidationWithAI } from '@/validation/ai-enhancement';
import { ValidationSeverity, type ValidationResult } from '@/validation/core-types';
import type { Tool } from '@/types/tool';
import { scanImageSchema, type ScanImageParams } from './schema';

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
  workflowHints?: {
    nextStep: string;
    message: string;
  };
  aiSuggestions?: string[];
  aiAnalysis?: {
    assessment: string;
    riskLevel: 'low' | 'medium' | 'high' | 'critical';
    priorities: Array<{
      area: string;
      severity: string;
      description: string;
      impact: string;
    }>;
    confidence: number;
  };
}

/**
 * Scan image implementation - direct execution without wrapper
 */
async function scanImageImpl(
  params: ScanImageParams,
  context: ToolContext,
): Promise<Result<ScanImageResult>> {
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const logger = getToolLogger(context, 'scan');
  const timer = createToolTimer(logger, 'scan');

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

    // Use session if available
    const sessionId = params.sessionId || context.session?.id || 'unknown';
    if (!context.session) {
      return Failure('Session not available');
    }

    logger.info(
      { sessionId, scanner, severityThreshold: finalSeverityThreshold },
      'Starting image security scan with session',
    );

    const securityScanner = createSecurityScanner(logger, scanner);

    // Check for built image in session results or use provided imageId
    const results = context.session?.get('results');
    const buildResult =
      results && typeof results === 'object' && 'build-image' in results
        ? ((results as any)['build-image'] as { imageId?: string } | undefined)
        : undefined;
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

    // AI Enhancement Integration
    let aiSuggestions: string[] | undefined;
    let aiAnalysis: ScanImageResult['aiAnalysis'];

    if (params.enableAISuggestions !== false && dockerScanResult.vulnerabilities) {
      try {
        logger.info('Starting AI-powered vulnerability analysis');

        // Convert scan results to validation results format
        const validationResults: ValidationResult[] = dockerScanResult.vulnerabilities.map(
          (vuln) => ({
            isValid: false,
            errors: [`${vuln.severity} vulnerability in ${vuln.package}:${vuln.version}`],
            warnings: vuln.description ? [vuln.description] : [],
            ...(vuln.id && { ruleId: vuln.id }),
            confidence: 0.9,
            metadata: {
              severity:
                vuln.severity.toLowerCase() === 'high' || vuln.severity.toLowerCase() === 'critical'
                  ? ValidationSeverity.ERROR
                  : vuln.severity.toLowerCase() === 'medium'
                    ? ValidationSeverity.WARNING
                    : ValidationSeverity.INFO,
              location: `${vuln.package}:${vuln.version}`,
              aiEnhanced: true,
            },
          }),
        );

        // Create a comprehensive content string for AI analysis
        const analysisContent = `
Docker Image: ${imageId}
Scanner: ${scanner}
Total Vulnerabilities: ${dockerScanResult.summary?.total || 0}

Critical: ${dockerScanResult.summary?.critical || 0}
High: ${dockerScanResult.summary?.high || 0}
Medium: ${dockerScanResult.summary?.medium || 0}
Low: ${dockerScanResult.summary?.low || 0}

Top Vulnerabilities:
${dockerScanResult.vulnerabilities
  .slice(0, 10)
  .map(
    (v) =>
      `- ${v.package}:${v.version} (${v.severity}): ${v.description || 'No description'}${
        v.fixedVersion ? ` [Fix: upgrade to ${v.fixedVersion}]` : ''
      }`,
  )
  .join('\n')}
        `.trim();

        const enhancementOptions = params.aiEnhancementOptions || {
          mode: 'suggestions',
          focus: 'security',
          confidence: 0.8,
          maxSuggestions: 5,
          includeExamples: true,
        };

        const enhancementResult = await enhanceValidationWithAI(
          analysisContent,
          validationResults,
          context,
          enhancementOptions,
        );

        if (enhancementResult.ok) {
          aiSuggestions = enhancementResult.value.suggestions;
          aiAnalysis = {
            assessment: enhancementResult.value.analysis.assessment,
            riskLevel: enhancementResult.value.analysis.riskLevel,
            priorities: enhancementResult.value.analysis.priorities,
            confidence: enhancementResult.value.confidence,
          };

          logger.info(
            {
              suggestionsCount: aiSuggestions.length,
              riskLevel: aiAnalysis.riskLevel,
              confidence: aiAnalysis.confidence,
            },
            'AI enhancement completed successfully',
          );
        } else {
          logger.warn(
            { error: enhancementResult.error },
            'AI enhancement failed, continuing without AI suggestions',
          );
        }
      } catch (error) {
        logger.debug({ error }, 'Failed to get AI enhancement, continuing without AI suggestions');
      }
    }

    // Determine context-dependent workflow hints
    let workflowHints: { nextStep: string; message: string } | undefined;

    if (!passed) {
      // Security issues found - suggest remediation
      if (scanResult.criticalCount > 0 || scanResult.highCount > 0) {
        workflowHints = {
          nextStep: 'fix-dockerfile',
          message: `Security scan found ${scanResult.criticalCount} critical and ${scanResult.highCount} high severity vulnerabilities. Use "fix-dockerfile" to address security issues in your base images and dependencies.`,
        };
      } else {
        workflowHints = {
          nextStep: 'generate-dockerfile',
          message: `Security scan found ${scanResult.totalVulnerabilities} vulnerabilities. Consider regenerating your Dockerfile with more secure base images using "generate-dockerfile".`,
        };
      }
    } else {
      // Scan passed - suggest next deployment steps
      workflowHints = {
        nextStep: 'push-image',
        message: `Security scan passed! Your image has no vulnerabilities above the ${finalSeverityThreshold} threshold. Use "push-image" with sessionId ${sessionId} to push to a registry, or proceed with deployment.`,
      };
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
      workflowHints,
      ...(aiSuggestions && aiSuggestions.length > 0 && { aiSuggestions }),
      ...(aiAnalysis && { aiAnalysis }),
    };

    // Store scan result in session
    if (context.session) {
      context.session.set('results', {
        scan: result,
      });
      context.session.pushStep('scan');
      context.session.set('current_step', 'scan');
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

// New Tool interface export
const tool: Tool<typeof scanImageSchema, ScanImageResult> = {
  name: 'scan',
  description: 'Scan Docker images for security vulnerabilities',
  version: '2.0.0',
  schema: scanImageSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'rerank',
    enhancementCapabilities: ['vulnerability-analysis', 'security-suggestions', 'risk-assessment'],
  },
  run: scanImageImpl,
};

export default tool;
