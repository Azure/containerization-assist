/**
 * Natural Language Formatters
 * Tool-specific narrative formatters for rich, human-friendly output
 *
 * @module mcp/formatters/natural-language-formatters
 *
 * @description
 * Provides tool-specific formatters that transform structured tool results
 * into rich, human-readable narratives with:
 * - Section headers and formatting
 * - Bullet points and structured lists
 * - Severity indicators and icons
 * - Context-aware next steps
 * - Proper handling of optional fields
 *
 * These formatters are used by the NATURAL_LANGUAGE output format to provide
 * superior user experience in chat interfaces and user-facing applications.
 */

import type { ScanImageResult } from '@/tools/scan-image/tool';
import type { DockerfilePlan } from '@/tools/generate-dockerfile/schema';
import type { BuildImageResult } from '@/tools/build-image-context/schema';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { VerifyDeploymentResult } from '@/tools/verify-deploy/tool';
import type { DockerfileFixPlan } from '@/tools/fix-dockerfile/schema';
import type { ManifestPlan } from '@/tools/generate-k8s-manifests/schema';
import type { PushImageResult } from '@/tools/push-image/tool';
import type { TagImageResult } from '@/tools/tag-image/tool';
import type { PrepareClusterResult } from '@/tools/prepare-cluster/tool';
import type { PingResult, ServerStatusResult } from '@/tools/ops/tool';
import { formatDuration, formatVulnerabilities, pluralize } from '@/lib/summary-helpers';
import { CHAINHINTSMODE, ChainHintsMode } from '@/app/orchestrator-types';

/**
 * Format scan-image result as natural language narrative
 *
 * @param result - Security scan result with vulnerability data
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with severity breakdown, remediation guidance, and next steps
 *
 * @description
 * Produces a detailed security scan report including:
 * - Pass/fail status with icon
 * - Vulnerability summary and severity breakdown (critical, high, medium, low)
 * - Remediation recommendations (up to 5, truncated with count)
 * - Scan metadata (timestamp)
 * - Context-aware next steps based on pass/fail status (when chainHintsMode is 'enabled')
 */
export function formatScanImageNarrative(
  result: ScanImageResult,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header
  const icon = result.passed ? '✅' : '❌';
  const status = result.passed ? 'PASSED' : 'FAILED';
  parts.push(`${icon} Security Scan ${status}\n`);
  const scanner = result.scanner ?? 'unknown scanner';
  parts.push(`Scanner: ${scanner}\n`);

  // Vulnerability summary
  const vulnText = formatVulnerabilities({
    critical: result.vulnerabilities.critical,
    high: result.vulnerabilities.high,
    medium: result.vulnerabilities.medium,
    low: result.vulnerabilities.low,
    total: result.vulnerabilities.total,
  });
  parts.push(`**Vulnerabilities:** ${vulnText}`);

  // Breakdown
  if (result.vulnerabilities.total > 0) {
    parts.push('\n**Severity Breakdown:**');
    if (result.vulnerabilities.critical > 0) {
      parts.push(`  🔴 Critical: ${result.vulnerabilities.critical}`);
    }
    if (result.vulnerabilities.high > 0) {
      parts.push(`  🟠 High: ${result.vulnerabilities.high}`);
    }
    if (result.vulnerabilities.medium > 0) {
      parts.push(`  🟡 Medium: ${result.vulnerabilities.medium}`);
    }
    if (result.vulnerabilities.low > 0) {
      parts.push(`  🟢 Low: ${result.vulnerabilities.low}`);
    }
  }

  // Recommended actions
  if (result.recommendedActions && result.recommendedActions.length > 0) {
    const totalFixed = result.recommendedActions.reduce(
      (sum, action) => sum + action.vulnerabilitiesFixed,
      0,
    );
    const actionLabel = pluralize(result.recommendedActions.length, 'action');
    const vulnerabilityLabel = pluralize(totalFixed, 'vulnerability', 'vulnerabilities');
    const actionVerb = result.recommendedActions.length === 1 ? 'fixes' : 'fix';
    parts.push(`\n**Recommended Actions:** (${actionLabel} ${actionVerb} ${vulnerabilityLabel})`);

    result.recommendedActions.forEach((action, idx) => {
      const severityOrder: Array<keyof typeof action.severityCounts> = [
        'critical',
        'high',
        'medium',
        'low',
        'negligible',
        'unknown',
      ];

      let severityText = 'no vulnerabilities with known severity';
      for (const severity of severityOrder) {
        const count = action.severityCounts[severity];
        if (count > 0) {
          severityText = `${count} ${severity.toLowerCase()}`;
          break;
        }
      }

      parts.push(
        `\n  ${idx + 1}. ${action.action} - Fixes ${action.vulnerabilitiesFixed} (${severityText})`,
      );
      parts.push(`     ${action.current}`);
      parts.push(`     → ${action.recommended}`);
    });
  }

  // Fixable vulnerabilities
  if (result.vulnerabilityDetails && result.vulnerabilityDetails.length > 0) {
    const fixableVulns = result.vulnerabilityDetails.filter((v) => v.fixedVersion);

    if (fixableVulns.length > 0) {
      const severityWeight: Record<string, number> = {
        CRITICAL: 4,
        HIGH: 3,
        MEDIUM: 2,
        LOW: 1,
        NEGLIGIBLE: 0,
        UNKNOWN: 0,
      };

      const sortedFixable = fixableVulns
        .sort((a, b) => (severityWeight[b.severity] ?? 0) - (severityWeight[a.severity] ?? 0))
        .slice(0, 10);

      parts.push(
        `\n**Fixable Vulnerabilities:** (${fixableVulns.length} of ${result.vulnerabilities.total})`,
      );
      sortedFixable.forEach((vuln, idx) => {
        parts.push(
          `  ${idx + 1}. [${vuln.severity}] ${vuln.package}: ${vuln.version} → ${vuln.fixedVersion}`,
        );
        if (vuln.id && vuln.id !== 'UNKNOWN') {
          parts.push(`     ID: ${vuln.id}`);
        }
      });

      if (fixableVulns.length > 10) {
        parts.push(`  ... and ${fixableVulns.length - 10} more fixable vulnerabilities`);
      }
    }
  }

  // Remediation guidance
  if (result.remediationGuidance && result.remediationGuidance.length > 0) {
    parts.push(
      `\n**Remediation Recommendations:** (${result.remediationGuidance.length} available)`,
    );
    result.remediationGuidance.slice(0, 5).forEach((guidance, idx) => {
      parts.push(`  ${idx + 1}. ${guidance.recommendation}`);
      if (guidance.example) {
        parts.push(`     Example: ${guidance.example}`);
      }
    });

    if (result.remediationGuidance.length > 5) {
      parts.push(`  ... and ${result.remediationGuidance.length - 5} more recommendations`);
    }
  }

  // Scan metadata
  parts.push(`\n**Scan Completed:** ${new Date(result.scanTime).toLocaleString()}`);

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push('\n**Next Steps:**');
    if (result.passed) {
      parts.push('  → Proceed with image tagging and registry push');
      parts.push('  → Consider deploying to staging environment');
    } else {
      parts.push('  → Review and address critical/high vulnerabilities');
      parts.push('  → Use fix-dockerfile to update base images');
      parts.push('  → Re-scan after applying fixes');
    }
  }

  return parts.join('\n');
}

/**
 * Format generate-dockerfile result as natural language narrative
 *
 * @param plan - Dockerfile generation plan with recommendations
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with project info, base images, security, and optimizations
 *
 * @description
 * Produces a comprehensive Dockerfile planning report including:
 * - Project information (language, version, framework)
 * - Build strategy (single-stage vs multi-stage)
 * - Recommended base image (primary + 1 alternative if available)
 * - Security considerations (top 5 most relevant)
 * - Optimization recommendations (top 5 most relevant)
 * - Existing Dockerfile analysis (if applicable)
 * - Policy validation results (if applicable)
 * - Actionable next steps (when chainHintsMode is 'enabled')
 */
export function formatDockerfilePlanNarrative(
  plan: DockerfilePlan,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Action-oriented header
  const actionIcon = plan.nextAction.action === 'create-files' ? '✨' : '🔧';
  const actionVerb = plan.nextAction.action === 'create-files' ? 'CREATE' : 'UPDATE';
  parts.push(`${actionIcon} ${actionVerb} DOCKERFILE\n`);

  // Clear instruction
  parts.push(`**Action:** ${plan.nextAction.instruction}\n`);

  // Files to create/update
  parts.push(`**Files:**`);
  plan.nextAction.files.forEach((f) => {
    parts.push(`  📄 ${f.path} - ${f.purpose}`);
  });
  parts.push('');

  // Project info
  const { repositoryInfo, recommendations } = plan;
  const framework = repositoryInfo.frameworks?.[0]?.name
    ? ` (${repositoryInfo.frameworks[0].name})`
    : '';
  parts.push(`**Project:** ${repositoryInfo.name}`);
  parts.push(`**Language:** ${repositoryInfo.language}${framework}`);

  if (repositoryInfo.buildSystems && repositoryInfo.buildSystems.length > 0) {
    repositoryInfo.buildSystems.forEach((bs) => {
      const version = bs.languageVersion
        ? ` (${repositoryInfo.language} ${bs.languageVersion})`
        : '';
      parts.push(`**Build System:** ${bs.type}${version}`);
    });
  }

  parts.push(
    `**Strategy:** ${recommendations.buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'} build`,
  );
  if (recommendations.buildStrategy.reason) {
    parts.push(`  ${recommendations.buildStrategy.reason}`);
  }

  // Platform and tag recommendations
  if (recommendations.platform) {
    parts.push(`**Platform:** ${recommendations.platform}`);
  }
  if (recommendations.defaultTag) {
    parts.push(`**Default Tag:** ${recommendations.defaultTag}`);
  }

  // Base images - opinionated recommendation (top 1-2 only)
  if (recommendations.baseImages.length > 0) {
    const primaryImage = recommendations.baseImages[0];
    if (primaryImage) {
      const sizeText = primaryImage.size ? ` (${primaryImage.size})` : '';
      parts.push(`\n**Recommended Base Image:**`);
      parts.push(`  **${primaryImage.image}**${sizeText}`);
      parts.push(`  ${primaryImage.reason}`);
    }

    // Show alternative if available
    if (recommendations.baseImages.length > 1) {
      const altImage = recommendations.baseImages[1];
      if (altImage) {
        const altSizeText = altImage.size ? ` (${altImage.size})` : '';
        parts.push(`\n**Alternative Option:**`);
        parts.push(`  **${altImage.image}**${altSizeText}`);
        parts.push(`  ${altImage.reason}`);
      }
    }
  }

  // Security
  if (recommendations.securityConsiderations.length > 0) {
    parts.push(
      `\n**Security Considerations:** (${recommendations.securityConsiderations.length} items)`,
    );
    recommendations.securityConsiderations.slice(0, 5).forEach((rec) => {
      const severity = rec.severity ? ` [${rec.severity}]` : '';
      parts.push(`  • ${rec.recommendation}${severity}`);
    });
  }

  // Optimizations
  if (recommendations.optimizations.length > 0) {
    parts.push(`\n**Optimizations:** (${recommendations.optimizations.length} recommendations)`);
    recommendations.optimizations.slice(0, 5).forEach((rec) => {
      parts.push(`  • ${rec.recommendation}`);
    });
  }

  // Existing Dockerfile analysis
  if (plan.existingDockerfile) {
    const { analysis, guidance } = plan.existingDockerfile;
    parts.push('\n**Existing Dockerfile Analysis:**');
    parts.push(`  Path: ${plan.existingDockerfile.path}`);
    parts.push(`  Complexity: ${analysis.complexity}`);
    parts.push(`  Security: ${analysis.securityPosture}`);
    parts.push(`  Enhancement Strategy: ${guidance.strategy}`);

    if (guidance.preserve.length > 0) {
      parts.push(`\n  **Preserve:** (${guidance.preserve.length} items)`);
      guidance.preserve.forEach((item) => parts.push(`    ✓ ${item}`));
    }

    if (guidance.improve.length > 0) {
      parts.push(`\n  **Improve:** (${guidance.improve.length} items)`);
      guidance.improve.forEach((item) => parts.push(`    → ${item}`));
    }
  }

  // Policy validation
  if (plan.policyValidation) {
    const { passed, violations, warnings } = plan.policyValidation;
    parts.push(`\n**Policy Validation:** ${passed ? '✅ Passed' : '❌ Failed'}`);
    if (violations.length > 0) {
      parts.push(`  Violations: ${violations.length}`);
      violations.slice(0, 3).forEach((v) => parts.push(`    • ${v.message}`));
    }
    if (warnings.length > 0) {
      parts.push(`  Warnings: ${warnings.length}`);
    }
  }

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push('\n**Next Steps:**');
    if (plan.nextAction.action === 'create-files') {
      parts.push('  1. Create Dockerfile using the base images and recommendations above');
      parts.push('  2. Build image with build-image-context tool');
      parts.push('  3. Scan for vulnerabilities with scan-image');
    } else {
      parts.push('  1. Update Dockerfile preserving good patterns and applying improvements');
      parts.push('  2. Rebuild image with build-image-context tool');
      parts.push('  3. Re-scan with scan-image to verify fixes');
    }
  }

  return parts.join('\n');
}

/**
 * Format build-image-context result as natural language narrative
 *
 * @param result - Build context preparation result with analysis and command
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with context analysis, security warnings, and build command
 *
 * @description
 * Produces a concise build preparation report including:
 * - Summary of build context analysis
 * - Dockerfile analysis (base images, ports, layers)
 * - Security warnings with severity
 * - BuildKit feature recommendations
 * - Ready-to-execute build command
 * - Next steps for executing the build (when chainHintsMode is 'enabled')
 */
export function formatBuildImageNarrative(
  result: BuildImageResult,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header
  parts.push('📦 Build Context Ready\n');

  // Summary
  parts.push(`**Summary:** ${result.summary}`);

  // Build configuration
  if (result.buildConfig.finalTags.length > 0) {
    parts.push(`**Tags:** ${result.buildConfig.finalTags.join(', ')}`);
  }
  parts.push(`**Platform:** ${result.buildConfig.platform}`);

  // Dockerfile analysis
  const analysis = result.dockerfileAnalysis;
  parts.push(`\n**Dockerfile Analysis:**`);
  parts.push(`  Base Images: ${analysis.baseImages.join(', ') || 'None'}`);
  if (analysis.exposedPorts.length > 0) {
    parts.push(`  Exposed Ports: ${analysis.exposedPorts.join(', ')}`);
  }
  parts.push(`  Estimated Layers: ${analysis.layerCount}`);
  if (analysis.finalUser) {
    parts.push(`  Final USER: ${analysis.finalUser}`);
  }
  parts.push(`  HEALTHCHECK: ${analysis.hasHealthcheck ? 'Yes' : 'No'}`);

  // Security warnings
  if (result.securityAnalysis.warnings.length > 0) {
    parts.push(`\n**Security Warnings:**`);
    result.securityAnalysis.warnings.forEach((w) => {
      parts.push(`  ⚠️ [${w.severity.toUpperCase()}] ${w.message}`);
    });
    parts.push(`  Risk Level: ${result.securityAnalysis.riskLevel}`);
  }

  // BuildKit recommendations
  if (result.buildKitAnalysis.recommended) {
    parts.push(`\n**BuildKit:** Recommended for this Dockerfile`);
  }

  // Build command
  parts.push(`\n**Build Command:**`);
  parts.push(`\`\`\`bash\n${result.nextAction.buildCommand.command}\n\`\`\``);

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push('\n**Next Steps:**');
    result.nextAction.preChecks.forEach((check) => {
      parts.push(`  □ ${check}`);
    });
    parts.push('  → Execute the build command above');
    parts.push('  → Scan built image for vulnerabilities with scan-image');
    parts.push('  → Tag image for registry with tag-image');
  }

  return parts.join('\n');
}

/**
 * Format analyze-repo result as natural language narrative
 *
 * @param result - Repository analysis with module detection
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with repository structure, modules, and next steps
 *
 * @description
 * Produces a comprehensive repository analysis report including:
 * - Analysis completion status
 * - Repository path and type (monorepo vs single-module)
 * - Module count and detailed information for each module:
 *   - Language and version
 *   - Detected frameworks
 *   - Build system
 *   - Entry point
 *   - Exposed ports
 * - Graceful handling of empty or undefined modules
 * - Context-aware next steps (with monorepo-specific guidance, when chainHintsMode is 'enabled')
 */
export function formatAnalyzeRepoNarrative(
  result: RepositoryAnalysis,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header
  parts.push('✅ Repository Analysis Complete\n');

  // Path
  parts.push(`**Path:** ${result.analyzedPath}`);
  parts.push(`**Type:** ${result.isMonorepo ? 'Monorepo' : 'Single-module project'}`);

  // Modules
  if (result.modules && result.modules.length > 0) {
    parts.push(`\n**Modules Found:** ${result.modules.length}`);
    result.modules.forEach((module, idx) => {
      parts.push(`\n  ${idx + 1}. **${module.name}**`);
      parts.push(`     Language: ${module.language}`);
      if (module.frameworks && module.frameworks.length > 0) {
        const frameworks = module.frameworks.map((f) => f.name).join(', ');
        parts.push(`     Frameworks: ${frameworks}`);
      }
      if (module.buildSystems && module.buildSystems.length > 0) {
        module.buildSystems.forEach((bs) => {
          const version = bs.languageVersion ? ` (${module.language} ${bs.languageVersion})` : '';
          parts.push(`     Build System: ${bs.type}${version}`);
        });
      }
      if (module.entryPoint) {
        parts.push(`     Entry Point: ${module.entryPoint}`);
      }
      if (module.ports && module.ports.length > 0) {
        parts.push(`     Ports: ${module.ports.join(', ')}`);
      }
    });
  } else {
    parts.push('\n**Modules Found:** 0');
    parts.push('  No modules detected in repository.');
  }

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push('\n**Next Steps:**');
    parts.push('  → Use generate-dockerfile to create container configuration');
    if (result.isMonorepo) {
      parts.push('  → Consider creating separate Dockerfiles for each module');
    }
  }

  return parts.join('\n');
}

/**
 * Format verify-deploy result as natural language narrative
 *
 * @param result - Deployment verification result with health and pod details
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with health status, pod breakdown, and next steps
 *
 * @description
 * Produces a detailed deployment verification report including:
 * - Deployment health status with icon
 * - Pod breakdown (running, pending, failed)
 * - Individual pod details (name, status, restarts)
 * - Health check results (pass/fail)
 * - Conditions and issues
 * - Context-aware next steps based on health status (when chainHintsMode is 'enabled')
 */
export function formatVerifyDeployNarrative(
  result: VerifyDeploymentResult,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header
  const icon = result.ready ? '✅' : '❌';
  const status = result.ready ? 'HEALTHY' : 'UNHEALTHY';
  parts.push(`${icon} Deployment Verification ${status}\n`);

  // Deployment info
  parts.push(`**Deployment:** ${result.deploymentName}`);
  parts.push(`**Namespace:** ${result.namespace}`);

  // Replica status
  if (result.status) {
    const { readyReplicas, totalReplicas } = result.status;
    const replicaIcon = readyReplicas === totalReplicas ? '✓' : '⚠';
    parts.push(`**Replicas:** ${replicaIcon} ${readyReplicas}/${totalReplicas} ready`);
  }

  // Pod breakdown
  if (result.pods && result.pods.length > 0) {
    const runningPods = result.pods.filter((p) => p.status === 'Running').length;
    const pendingPods = result.pods.filter((p) => p.status === 'Pending').length;
    const failedPods = result.pods.filter((p) => p.status === 'Failed').length;

    parts.push(`\n**Pod Status:**`);
    if (runningPods > 0) parts.push(`  ✅ Running: ${runningPods}`);
    if (pendingPods > 0) parts.push(`  ⏳ Pending: ${pendingPods}`);
    if (failedPods > 0) parts.push(`  ❌ Failed: ${failedPods}`);

    // Show individual pod details (up to 5)
    parts.push(`\n**Pod Details:**`);
    result.pods.slice(0, 5).forEach((pod) => {
      const statusIcon = pod.ready ? '✓' : '✗';
      const healthIcon = pod.healthy ? '💚' : '💔';
      const restartWarning = pod.restarts > 0 ? ` (${pod.restarts} restarts)` : '';
      parts.push(`  ${statusIcon} ${pod.name}`);
      parts.push(`     Status: ${pod.status}${restartWarning}`);
      parts.push(`     Health: ${healthIcon} ${pod.healthy ? 'Healthy' : 'Unhealthy'}`);
      parts.push(`     Age: ${pod.age}`);
    });

    if (result.pods.length > 5) {
      parts.push(`  ... and ${result.pods.length - 5} more pods`);
    }
  }

  // Health checks
  if (result.healthCheck) {
    const hcIcon = result.healthCheck.status === 'healthy' ? '💚' : '💔';
    parts.push(`\n**Health Check:** ${hcIcon} ${result.healthCheck.status.toUpperCase()}`);
    parts.push(`  ${result.healthCheck.message}`);

    if (result.healthCheck.checks && result.healthCheck.checks.length > 0) {
      parts.push(`\n  **Check Details:**`);
      result.healthCheck.checks.forEach((check) => {
        const checkIcon = check.status === 'pass' ? '✓' : '✗';
        parts.push(`    ${checkIcon} ${check.name}: ${check.message || check.status}`);
      });
    }
  }

  // Conditions
  if (result.status?.conditions && result.status.conditions.length > 0) {
    parts.push(`\n**Conditions:**`);
    result.status.conditions.forEach((cond) => {
      const condIcon = cond.status === 'True' ? '✓' : '✗';
      parts.push(`  ${condIcon} ${cond.type}: ${cond.message}`);
    });
  }

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push(`\n**Next Steps:**`);
    if (result.ready) {
      parts.push('  → Deployment is healthy and serving traffic');
      parts.push('  → Monitor pod logs for application errors');
      parts.push('  → Set up alerting for production monitoring');
    } else {
      parts.push('  → Review pod logs for error messages');
      parts.push('  → Check deployment events with kubectl describe');
      parts.push('  → Verify resource limits and constraints');
      const failedPods = result.pods?.filter((p) => p.status === 'Failed').length || 0;
      if (failedPods > 0) {
        parts.push('  → Investigate failed pods with kubectl logs');
      }
    }
  }

  return parts.join('\n');
}

/**
 * Format fix-dockerfile result as natural language narrative
 *
 * @param result - Dockerfile validation and fix plan
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with issues, recommendations, and validation score
 *
 * @description
 * Produces a comprehensive Dockerfile validation report including:
 * - Validation score/grade prominently displayed
 * - Issues categorized by type (security, performance, best practices)
 * - Fix recommendations with priority
 * - Policy validation results
 * - Estimated impact of fixes
 * - Context-aware next steps for implementation (when chainHintsMode is 'enabled')
 */
export function formatFixDockerfileNarrative(
  result: DockerfileFixPlan,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header with validation grade
  const gradeIcon: Record<string, string> = {
    A: '🌟',
    B: '✅',
    C: '⚠️',
    D: '⚠️',
    F: '❌',
  };

  parts.push(
    `${gradeIcon[result.validationGrade] || '❓'} Dockerfile Validation: Grade ${result.validationGrade} (Score: ${result.validationScore}/100)\n`,
  );

  // Priority indicator
  const priorityIcon: Record<string, string> = {
    high: '🔴',
    medium: '🟡',
    low: '🟢',
  };
  parts.push(
    `**Priority:** ${priorityIcon[result.priority] || '⚪'} ${result.priority.toUpperCase()}`,
  );
  parts.push(`**Estimated Impact:** ${result.estimatedImpact}`);
  parts.push(`**Confidence:** ${Math.round(result.confidence * 100)}%`);

  // Current issues by category
  const totalIssues =
    result.currentIssues.security.length +
    result.currentIssues.performance.length +
    result.currentIssues.bestPractices.length;

  if (totalIssues > 0) {
    parts.push(`\n**Current Issues:** (${totalIssues} total)`);

    if (result.currentIssues.security.length > 0) {
      parts.push(`\n  🔒 **Security Issues:** (${result.currentIssues.security.length})`);
      result.currentIssues.security.slice(0, 5).forEach((issue, idx) => {
        const priorityLabel = issue.priority ? ` [${issue.priority}]` : '';
        const message = issue.message || 'Issue detected';
        parts.push(`    ${idx + 1}. ${message}${priorityLabel}`);
      });
      if (result.currentIssues.security.length > 5) {
        parts.push(`    ... and ${result.currentIssues.security.length - 5} more`);
      }
    }

    if (result.currentIssues.performance.length > 0) {
      parts.push(`\n  ⚡ **Performance Issues:** (${result.currentIssues.performance.length})`);
      result.currentIssues.performance.slice(0, 3).forEach((issue, idx) => {
        const message = issue.message || 'Issue detected';
        parts.push(`    ${idx + 1}. ${message}`);
      });
      if (result.currentIssues.performance.length > 3) {
        parts.push(`    ... and ${result.currentIssues.performance.length - 3} more`);
      }
    }

    if (result.currentIssues.bestPractices.length > 0) {
      parts.push(`\n  📋 **Best Practice Issues:** (${result.currentIssues.bestPractices.length})`);
      result.currentIssues.bestPractices.slice(0, 3).forEach((issue, idx) => {
        const message = issue.message || 'Issue detected';
        parts.push(`    ${idx + 1}. ${message}`);
      });
      if (result.currentIssues.bestPractices.length > 3) {
        parts.push(`    ... and ${result.currentIssues.bestPractices.length - 3} more`);
      }
    }
  }

  // Fix recommendations
  const totalFixes =
    result.fixes.security.length +
    result.fixes.performance.length +
    result.fixes.bestPractices.length;

  if (totalFixes > 0) {
    parts.push(`\n**Fix Recommendations:** (${totalFixes} available)`);

    if (result.fixes.security.length > 0) {
      parts.push(`\n  🔒 **Security Fixes:** (${result.fixes.security.length})`);
      result.fixes.security.slice(0, 3).forEach((fix, idx) => {
        const effortLabel = fix.effort ? ` [${fix.effort} effort]` : '';
        parts.push(`    ${idx + 1}. ${fix.title}${effortLabel}`);
        parts.push(`       ${fix.description}`);
        if (fix.example) {
          const truncatedExample =
            fix.example.length > 80 ? `${fix.example.substring(0, 77)}...` : fix.example;
          parts.push(`       Example: ${truncatedExample}`);
        }
      });
    }

    if (result.fixes.performance.length > 0) {
      parts.push(`\n  ⚡ **Performance Fixes:** (${result.fixes.performance.length})`);
      result.fixes.performance.slice(0, 2).forEach((fix, idx) => {
        parts.push(`    ${idx + 1}. ${fix.title}`);
        parts.push(`       ${fix.description}`);
      });
    }
  }

  // Policy validation
  if (result.policyValidation) {
    const policyIcon = result.policyValidation.passed ? '✅' : '❌';
    parts.push(
      `\n**Policy Validation:** ${policyIcon} ${result.policyValidation.passed ? 'PASSED' : 'FAILED'}`,
    );
    if (result.policyValidation.violations.length > 0) {
      parts.push(`  Violations: ${result.policyValidation.violations.length}`);
      result.policyValidation.violations.slice(0, 2).forEach((v) => {
        parts.push(`    • ${v.message}`);
      });
    }
  }

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push(`\n**Next Steps:**`);
    if (result.validationGrade === 'A' || result.validationGrade === 'B') {
      parts.push('  → Dockerfile is in good shape with minor improvements available');
      parts.push('  → Review fix recommendations for optimization');
      parts.push('  → Proceed with build-image-context');
    } else {
      parts.push('  → Address high-priority security issues first');
      parts.push('  → Apply recommended fixes to improve validation score');
      parts.push('  → Re-run fix-dockerfile to verify improvements');
      if (result.policyValidation && !result.policyValidation.passed) {
        parts.push('  → Resolve policy violations before deployment');
      }
    }
  }

  return parts.join('\n');
}

/**
 * Format generate-k8s-manifests result as natural language narrative
 *
 * @param result - Kubernetes manifest generation result
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with manifest details and resource breakdown
 *
 * @description
 * Produces a comprehensive manifest generation report including:
 * - Manifest type and format
 * - Resources and requirements
 * - Security considerations
 * - Best practices recommendations
 * - Context-aware next steps (when chainHintsMode is 'enabled')
 */
export function formatGenerateK8sManifestsNarrative(
  result: ManifestPlan,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Action-oriented header
  parts.push('✨ CREATE KUBERNETES MANIFESTS\n');

  // Clear instruction
  parts.push(`**Action:** ${result.nextAction.instruction}\n`);

  // Files to create
  parts.push(`**Files:**`);
  result.nextAction.files.forEach((f) => {
    parts.push(`  📄 ${f.path} - ${f.purpose}`);
  });
  parts.push('');

  // Manifest type
  parts.push(`**Manifest Type:** ${result.manifestType}`);

  // Repository info if available
  if (result.repositoryInfo) {
    const { name, language, languageVersion, frameworks } = result.repositoryInfo;
    const langVersion = languageVersion ? ` ${languageVersion}` : '';
    const framework = frameworks?.[0]?.name ? ` (${frameworks[0].name})` : '';
    parts.push(`**Application:** ${name}`);
    parts.push(`**Language:** ${language}${langVersion}${framework}`);
  }

  // ACA analysis if available
  if (result.acaAnalysis) {
    parts.push(`\n**Container Apps:** ${result.acaAnalysis.containerApps.length}`);
    result.acaAnalysis.containerApps.forEach((app, idx) => {
      parts.push(`  ${idx + 1}. ${app.name}`);
      parts.push(`     Containers: ${app.containers}`);
      if (app.hasIngress) parts.push(`     ✓ Ingress configured`);
      if (app.hasScaling) parts.push(`     ✓ Scaling enabled`);
      if (app.hasSecrets) parts.push(`     ✓ Secrets configured`);
    });

    if (result.acaAnalysis.warnings.length > 0) {
      parts.push(`\n  **Warnings:**`);
      result.acaAnalysis.warnings.forEach((w) => parts.push(`    ⚠ ${w}`));
    }
  }

  // Recommendations
  const { recommendations } = result;

  // Security considerations
  if (recommendations.securityConsiderations.length > 0) {
    parts.push(
      `\n**Security Considerations:** (${recommendations.securityConsiderations.length} items)`,
    );
    recommendations.securityConsiderations.slice(0, 5).forEach((rec) => {
      const severity = rec.severity ? ` [${rec.severity}]` : '';
      parts.push(`  • ${rec.recommendation}${severity}`);
    });
  }

  // Resource management
  if (recommendations.resourceManagement && recommendations.resourceManagement.length > 0) {
    parts.push(
      `\n**Resource Management:** (${recommendations.resourceManagement.length} recommendations)`,
    );
    recommendations.resourceManagement.slice(0, 3).forEach((rec) => {
      parts.push(`  • ${rec.recommendation}`);
    });
  }

  // Best practices
  if (recommendations.bestPractices.length > 0) {
    parts.push(`\n**Best Practices:** (${recommendations.bestPractices.length} recommendations)`);
    recommendations.bestPractices.slice(0, 5).forEach((rec) => {
      parts.push(`  • ${rec.recommendation}`);
    });
  }

  // Policy validation
  if (result.policyValidation) {
    const { passed, violations, warnings } = result.policyValidation;
    parts.push(`\n**Policy Validation:** ${passed ? '✅ Passed' : '❌ Failed'}`);
    if (violations.length > 0) {
      parts.push(`  Violations: ${violations.length}`);
      violations.slice(0, 3).forEach((v) => parts.push(`    • ${v.message}`));
    }
    if (warnings.length > 0) {
      parts.push(`  Warnings: ${warnings.length}`);
    }
  }

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push(`\n**Next Steps:**`);
    parts.push('  1. Create manifest files using the recommendations above');
    parts.push('  2. Use prepare-cluster to setup namespace and prerequisites');
    parts.push('  3. Use kubectl apply to deploy manifests to cluster');
    parts.push('  4. Verify deployment with verify-deploy');
  }

  return parts.join('\n');
}

/**
 * Format push-image result as natural language narrative
 *
 * @param result - Image push result with registry and digest
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with push details and next steps
 *
 * @description
 * Produces a concise push report including:
 * - Push success status
 * - Registry and tag information
 * - Image digest (truncated for readability)
 * - Full image reference
 * - Standard next steps (when chainHintsMode is 'enabled')
 */
export function formatPushImageNarrative(
  result: PushImageResult,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header
  parts.push('✅ Image Pushed to Registry\n');

  // Push details
  parts.push(`**Registry:** ${result.registry}`);
  parts.push(`**Tag:** ${result.pushedTag}`);

  // Digest (truncated for readability)
  const shortDigest = result.digest.startsWith('sha256:')
    ? `${result.digest.substring(0, 19)}...`
    : result.digest;
  parts.push(`**Digest:** ${shortDigest}`);

  // Full image reference
  // Check if pushedTag already includes the registry to avoid duplication
  const fullImage = result.pushedTag.startsWith(result.registry)
    ? `${result.pushedTag}@${result.digest}`
    : `${result.registry}/${result.pushedTag}@${result.digest}`;
  parts.push(`\n**Full Reference:**`);
  parts.push(`  ${fullImage}`);

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push(`\n**Next Steps:**`);
    parts.push('  → Image is now available in the registry');
    parts.push('  → Update Kubernetes manifests with this image');
    parts.push('  → Use deploy to deploy to cluster');
    parts.push('  → Consider setting up automated deployments');
  }

  return parts.join('\n');
}

/**
 * Format tag-image result as natural language narrative
 *
 * @param result - Image tagging result
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with tags applied
 *
 * @description
 * Produces a simple tagging report including:
 * - Success status
 * - Image identifier
 * - Tags applied (list)
 * - Standard next steps with versioning guidance (when chainHintsMode is 'enabled')
 */
export function formatTagImageNarrative(
  result: TagImageResult,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header
  parts.push('✅ Image Tagged\n');

  // Image info
  const shortImageId = result.imageId.startsWith('sha256:')
    ? `${result.imageId.substring(0, 19)}...`
    : result.imageId;
  parts.push(`**Image ID:** ${shortImageId}`);

  // Tags applied
  parts.push(`\n**Tags Applied:** (${result.tags.length})`);
  result.tags.forEach((tag) => {
    parts.push(`  • ${tag}`);
  });

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push(`\n**Next Steps:**`);
    parts.push('  → Use push-image to push tagged image to registry');
    parts.push('  → Tags can be used in Kubernetes manifests');
    if (result.tags.some((t) => t.includes('latest'))) {
      parts.push('  → Consider using semantic versioning instead of "latest"');
    }
  }

  return parts.join('\n');
}

/**
 * Format prepare-cluster result as natural language narrative
 *
 * @param result - Cluster preparation result
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with setup details
 *
 * @description
 * Produces a cluster preparation report including:
 * - Cluster preparation status
 * - Namespace and connectivity checks
 * - Resources and checks performed
 * - Warnings if any
 * - Context-aware next steps (when chainHintsMode is 'enabled')
 */
export function formatPrepareClusterNarrative(
  result: PrepareClusterResult,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  const parts: string[] = [];

  // Header
  const icon = result.success ? '✅' : '❌';
  parts.push(`${icon} Cluster Preparation ${result.success ? 'Complete' : 'Failed'}\n`);

  // Cluster and namespace info
  parts.push(`**Cluster:** ${result.cluster}`);
  parts.push(`**Namespace:** ${result.namespace}`);
  parts.push(`**Ready:** ${result.clusterReady ? 'Yes' : 'No'}`);

  // Checks performed
  parts.push(`\n**Checks Performed:**`);
  const checks = result.checks;
  Object.entries(checks).forEach(([check, passed]) => {
    if (typeof passed === 'boolean') {
      const checkIcon = passed ? '✅' : '❌';
      const checkName = check
        .replace(/([A-Z])/g, ' $1')
        .replace(/^./, (str) => str.toUpperCase())
        .trim();
      parts.push(`  ${checkIcon} ${checkName}`);
    }
  });

  // Warnings if any
  if (result.warnings && result.warnings.length > 0) {
    parts.push(`\n**Warnings:** (${result.warnings.length})`);
    result.warnings.slice(0, 3).forEach((warning) => {
      parts.push(`  ⚠ ${warning}`);
    });
    if (result.warnings.length > 3) {
      parts.push(`  ... and ${result.warnings.length - 3} more`);
    }
  }

  // Local registry if created
  if (result.localRegistryUrl) {
    parts.push(`\n**Local Registry:** ${result.localRegistryUrl}`);
  }

  // Detailed registry information if available
  if (result.localRegistry) {
    parts.push(`\n**Local Registry Details:**`);
    const healthIcon = result.localRegistry.healthy ? '✅' : '⚠️';
    const reachableIcon = result.localRegistry.reachableFromCluster ? '✅' : '⚠️';
    parts.push(`  External URL: ${result.localRegistry.externalUrl}`);
    parts.push(`  Internal Endpoint: ${result.localRegistry.internalEndpoint}`);
    parts.push(`  Container Name: ${result.localRegistry.containerName}`);
    parts.push(
      `  Health Status: ${healthIcon} ${result.localRegistry.healthy ? 'Healthy' : 'Unhealthy'}`,
    );
    parts.push(
      `  Reachable from Cluster: ${reachableIcon} ${result.localRegistry.reachableFromCluster ? 'Yes' : 'No'}`,
    );
  }

  // Next steps (only if chainHintsMode is enabled)
  if (chainHintsMode === CHAINHINTSMODE.ENABLED) {
    parts.push(`\n**Next Steps:**`);
    if (result.success && result.clusterReady) {
      parts.push('  → Cluster is ready for deployment');
      parts.push('  → Use kubectl apply to deploy your application');
      parts.push('  → Resources will be deployed to the prepared namespace');
    } else {
      parts.push('  → Check cluster connectivity');
      parts.push('  → Verify RBAC permissions');
      parts.push('  → Review error logs for details');
    }
  }

  return parts.join('\n');
}

/**
 * Format ops ping result as natural language narrative
 *
 * @param result - Server ping result
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with server status
 *
 * @description
 * Produces a simple server ping report including:
 * - Response status
 * - Timestamp
 * - Server information (version, uptime, PID)
 * - Capabilities available
 * - Health indicators
 * Note: This formatter doesn't include Next Steps as it's a simple status check
 */
export function formatOpsPingNarrative(result: PingResult): string {
  const parts: string[] = [];

  // Header
  parts.push('✅ Server Ping Successful\n');

  // Response
  parts.push(`**Response:** ${result.message}`);
  parts.push(`**Timestamp:** ${new Date(result.timestamp).toLocaleString()}`);

  // Server info
  parts.push(`\n**Server Information:**`);
  parts.push(`  Name: ${result.server.name}`);
  parts.push(`  Version: ${result.server.version}`);
  parts.push(`  Uptime: ${formatDuration(Math.round(result.server.uptime))}`);
  parts.push(`  Process ID: ${result.server.pid}`);

  // Capabilities
  parts.push(`\n**Capabilities:**`);
  if (result.capabilities.tools) parts.push('  ✅ Tools available');
  if (result.capabilities.progress) parts.push('  ✅ Progress tracking enabled');

  // Status
  parts.push(`\n**Status:** Server is responsive and healthy`);

  return parts.join('\n');
}

/**
 * Format ops status result as natural language narrative
 *
 * @param result - Server status result with detailed metrics
 * @param chainHintsMode - Whether to include "Next Steps" section (default: 'enabled')
 * @returns Formatted narrative with system health and metrics
 *
 * @description
 * Produces a detailed server status report including:
 * - Health status with icon
 * - Version and uptime
 * - Memory usage with health indicators
 * - CPU information and load
 * - Tool availability
 * - Health summary
 * Note: This formatter doesn't include Next Steps as it's a status report
 */
export function formatOpsStatusNarrative(result: ServerStatusResult): string {
  const parts: string[] = [];

  // Header
  const icon = result.success ? '💚' : '❌';
  const status = result.success ? 'HEALTHY' : 'UNHEALTHY';
  parts.push(`${icon} Server Status: ${status}\n`);

  // Version and uptime
  parts.push(`**Version:** ${result.version}`);
  parts.push(`**Uptime:** ${formatDuration(Math.round(result.uptime))}`);

  // Memory usage
  if (result.memory) {
    const memUsedMB = Math.round(result.memory.used / 1024 / 1024);
    const memTotalMB = Math.round(result.memory.total / 1024 / 1024);
    const memPercent = Math.round((result.memory.used / result.memory.total) * 100);

    parts.push(`\n**Memory Usage:**`);
    parts.push(`  Used: ${memUsedMB}MB / ${memTotalMB}MB (${memPercent}%)`);

    // Memory health indicator
    if (memPercent > 90) {
      parts.push(`  ⚠️ Memory usage is high`);
    } else if (memPercent > 75) {
      parts.push(`  ⚡ Memory usage is moderate`);
    } else {
      parts.push(`  ✅ Memory usage is healthy`);
    }
  }

  // CPU info
  if (result.cpu) {
    parts.push(`\n**CPU:**`);
    parts.push(`  Cores: ${result.cpu.cores}`);
    if (result.cpu.loadAverage && result.cpu.loadAverage.length > 0) {
      const load = Array.isArray(result.cpu.loadAverage)
        ? result.cpu.loadAverage[0]
        : result.cpu.loadAverage;
      if (load !== undefined) {
        parts.push(`  Load: ${load.toFixed(2)}`);
      }
    }
  }

  // Tools
  if (result.tools) {
    parts.push(`\n**Tools Available:** ${result.tools.count}`);
  }

  // Policies
  if (result.policies) {
    parts.push(`\n**Policies Loaded:** ${result.policies.total}`);
    if (result.policies.files.length > 0) {
      for (const file of result.policies.files) {
        parts.push(`  - \`${file.path}\` (${file.source})`);
      }
    }
    if (result.policies.searchPaths && result.policies.searchPaths.length > 0) {
      parts.push(`\n**Policy Search Paths:**`);
      for (const sp of result.policies.searchPaths) {
        const status = sp.exists ? '✅' : '⚠️ not found';
        parts.push(`  - \`${sp.path}\` (${sp.source}) ${status}`);
      }
    }
  }

  // Health summary
  parts.push(`\n**Health Summary:**`);
  if (result.success) {
    parts.push('  ✅ All systems operational');
    parts.push('  ✅ Server ready to handle requests');
  } else {
    parts.push('  ⚠️ Server experiencing issues');
    parts.push('  → Check memory and CPU usage');
    parts.push('  → Review recent error logs');
  }

  return parts.join('\n');
}
