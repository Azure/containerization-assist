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
import type { DeployApplicationResult } from '@/tools/deploy/tool';
import type { BuildImageResult } from '@/tools/build-image/tool';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import {
  formatSize,
  formatDuration,
  formatVulnerabilities,
} from '@/lib/summary-helpers';

/**
 * Format scan-image result as natural language narrative
 *
 * @param result - Security scan result with vulnerability data
 * @returns Formatted narrative with severity breakdown, remediation guidance, and next steps
 *
 * @description
 * Produces a detailed security scan report including:
 * - Pass/fail status with icon
 * - Vulnerability summary and severity breakdown (critical, high, medium, low)
 * - Remediation recommendations (up to 5, truncated with count)
 * - Scan metadata (timestamp)
 * - Context-aware next steps based on pass/fail status
 */
export function formatScanImageNarrative(result: ScanImageResult): string {
  const parts: string[] = [];

  // Header
  const icon = result.passed ? '✅' : '❌';
  const status = result.passed ? 'PASSED' : 'FAILED';
  parts.push(`${icon} Security Scan ${status}\n`);

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

  // Remediation guidance
  if (result.remediationGuidance && result.remediationGuidance.length > 0) {
    parts.push(`\n**Remediation Recommendations:** (${result.remediationGuidance.length} available)`);
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

  // Next steps
  parts.push('\n**Next Steps:**');
  if (result.passed) {
    parts.push('  → Proceed with image tagging and registry push');
    parts.push('  → Consider deploying to staging environment');
  } else {
    parts.push('  → Review and address critical/high vulnerabilities');
    parts.push('  → Use fix-dockerfile to update base images');
    parts.push('  → Re-scan after applying fixes');
  }

  return parts.join('\n');
}

/**
 * Format generate-dockerfile result as natural language narrative
 *
 * @param plan - Dockerfile generation plan with recommendations
 * @returns Formatted narrative with project info, base images, security, and optimizations
 *
 * @description
 * Produces a comprehensive Dockerfile planning report including:
 * - Project information (language, version, framework)
 * - Build strategy (single-stage vs multi-stage)
 * - Base image recommendations (top 3 with scores and reasoning)
 * - Security considerations (up to 5)
 * - Optimization recommendations (up to 5)
 * - Existing Dockerfile analysis (if applicable)
 * - Policy validation results (if applicable)
 * - Actionable next steps
 */
export function formatDockerfilePlanNarrative(plan: DockerfilePlan): string {
  const parts: string[] = [];

  // Header
  parts.push('📝 Dockerfile Planning Complete\n');

  // Project info
  const { repositoryInfo, recommendations } = plan;
  const langVersion = repositoryInfo.languageVersion ? ` ${repositoryInfo.languageVersion}` : '';
  const framework = repositoryInfo.frameworks?.[0]?.name ? ` (${repositoryInfo.frameworks[0].name})` : '';
  parts.push(`**Project:** ${repositoryInfo.name}`);
  parts.push(`**Language:** ${repositoryInfo.language}${langVersion}${framework}`);
  parts.push(`**Strategy:** ${recommendations.buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'} build`);
  if (recommendations.buildStrategy.reason) {
    parts.push(`  ${recommendations.buildStrategy.reason}`);
  }

  // Base images
  if (recommendations.baseImages.length > 0) {
    parts.push(`\n**Base Image Recommendations:** (${recommendations.baseImages.length} options)`);
    recommendations.baseImages.slice(0, 3).forEach((img, idx) => {
      const sizeText = img.size ? `, ${img.size}` : '';
      const scoreText = img.matchScore ? ` [score: ${Math.round(img.matchScore)}]` : '';
      parts.push(`  ${idx + 1}. **${img.image}** (${img.category}${sizeText})${scoreText}`);
      parts.push(`     ${img.reason}`);
    });
  }

  // Security
  if (recommendations.securityConsiderations.length > 0) {
    parts.push(`\n**Security Considerations:** (${recommendations.securityConsiderations.length} items)`);
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
      guidance.preserve.forEach(item => parts.push(`    ✓ ${item}`));
    }

    if (guidance.improve.length > 0) {
      parts.push(`\n  **Improve:** (${guidance.improve.length} items)`);
      guidance.improve.forEach(item => parts.push(`    → ${item}`));
    }
  }

  // Policy validation
  if (plan.policyValidation) {
    const { passed, violations, warnings } = plan.policyValidation;
    parts.push(`\n**Policy Validation:** ${passed ? '✅ Passed' : '❌ Failed'}`);
    if (violations.length > 0) {
      parts.push(`  Violations: ${violations.length}`);
      violations.slice(0, 3).forEach(v => parts.push(`    • ${v.message}`));
    }
    if (warnings.length > 0) {
      parts.push(`  Warnings: ${warnings.length}`);
    }
  }

  // Next steps
  parts.push('\n**Next Steps:**');
  parts.push('  → Review base image recommendations');
  parts.push('  → Use fix-dockerfile to create or update Dockerfile');
  parts.push('  → Build image with build-image tool');

  return parts.join('\n');
}

/**
 * Format deploy result as natural language narrative
 *
 * @param result - Deployment result with status and endpoint information
 * @returns Formatted narrative with deployment status, endpoints, conditions, and next steps
 *
 * @description
 * Produces a detailed deployment report including:
 * - Deployment status (DEPLOYED or IN PROGRESS with icon)
 * - Application, namespace, and service information
 * - Replica readiness status
 * - Endpoints (external and internal with type indicators)
 * - Deployment conditions with status icons
 * - Context-aware next steps based on ready status
 */
export function formatDeployNarrative(result: DeployApplicationResult): string {
  const parts: string[] = [];

  // Header
  const icon = result.ready ? '✅' : '⏳';
  const status = result.ready ? 'DEPLOYED' : 'IN PROGRESS';
  parts.push(`${icon} Deployment ${status}\n`);

  // Deployment info
  parts.push(`**Application:** ${result.deploymentName}`);
  parts.push(`**Namespace:** ${result.namespace}`);
  parts.push(`**Service:** ${result.serviceName}`);

  // Status
  if (result.status) {
    const { readyReplicas, totalReplicas } = result.status;
    parts.push(`**Status:** ${readyReplicas}/${totalReplicas} replicas ready`);
  }

  // Endpoints
  if (result.endpoints.length > 0) {
    parts.push(`\n**Endpoints:** (${result.endpoints.length} available)`);
    result.endpoints.forEach((ep) => {
      const type = ep.type === 'external' ? '🌐 External' : '🔒 Internal';
      parts.push(`  ${type}: ${ep.url}:${ep.port}`);
    });
  }

  // Conditions
  if (result.status?.conditions) {
    parts.push('\n**Conditions:**');
    result.status.conditions.forEach((cond) => {
      const statusIcon = cond.status === 'True' ? '✓' : '✗';
      parts.push(`  ${statusIcon} ${cond.type}: ${cond.message}`);
    });
  }

  // Next steps
  parts.push('\n**Next Steps:**');
  if (result.ready) {
    parts.push('  → Use verify-deploy to check deployment health');
    parts.push('  → Test application endpoints');
    parts.push('  → Monitor pod logs for issues');
  } else {
    parts.push('  → Wait for all replicas to become ready');
    parts.push('  → Check pod status with kubectl get pods');
    parts.push('  → Review pod logs if deployment stalls');
  }

  return parts.join('\n');
}

/**
 * Format build-image result as natural language narrative
 *
 * @param result - Build result with image details and metrics
 * @returns Formatted narrative with image details, metrics, and next steps
 *
 * @description
 * Produces a concise build report including:
 * - Success status with icon
 * - Image ID and applied tags
 * - Image size (formatted in MB/GB)
 * - Build time (formatted in seconds/minutes)
 * - Layer count (if available)
 * - Standard next steps for containerization workflow
 */
export function formatBuildImageNarrative(result: BuildImageResult): string {
  const parts: string[] = [];

  // Header
  parts.push('✅ Image Built Successfully\n');

  // Build info
  parts.push(`**Image:** ${result.imageId}`);
  if (result.tags && result.tags.length > 0) {
    parts.push(`**Tags:** ${result.tags.join(', ')}`);
  }
  if (result.size) {
    parts.push(`**Size:** ${formatSize(result.size)}`);
  }
  if (result.buildTime) {
    parts.push(`**Build Time:** ${formatDuration(Math.round(result.buildTime / 1000))}`);
  }

  // Layer information
  if (result.layers) {
    parts.push(`**Layers:** ${result.layers}`);
  }

  // Next steps
  parts.push('\n**Next Steps:**');
  parts.push('  → Scan image for vulnerabilities with scan-image');
  parts.push('  → Tag image for registry with tag-image');
  parts.push('  → Push to registry with push-image');

  return parts.join('\n');
}

/**
 * Format analyze-repo result as natural language narrative
 *
 * @param result - Repository analysis with module detection
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
 * - Context-aware next steps (with monorepo-specific guidance)
 */
export function formatAnalyzeRepoNarrative(result: RepositoryAnalysis): string {
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
      parts.push(`     Language: ${module.language}${module.languageVersion ? ` ${module.languageVersion}` : ''}`);
      if (module.frameworks && module.frameworks.length > 0) {
        const frameworks = module.frameworks.map(f => f.name).join(', ');
        parts.push(`     Frameworks: ${frameworks}`);
      }
      if (module.buildSystem) {
        parts.push(`     Build System: ${module.buildSystem.type || 'Unknown'}`);
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

  // Next steps
  parts.push('\n**Next Steps:**');
  parts.push('  → Use generate-dockerfile to create container configuration');
  if (result.isMonorepo) {
    parts.push('  → Consider creating separate Dockerfiles for each module');
  }

  return parts.join('\n');
}
