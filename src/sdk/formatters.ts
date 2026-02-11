/**
 * LLM Result Formatters for VS Code Extension Integration
 *
 * Converts SDK result types into formatted strings optimized for
 * LLM consumption in VS Code's LanguageModelToolResult.
 *
 * Design principles:
 * - Structured but readable output
 * - Include actionable next steps
 * - Highlight key information
 * - Keep within reasonable token limits
 *
 * @example
 * ```typescript
 * import { resultFormatters } from 'containerization-assist-mcp/sdk';
 *
 * const result = await analyzeRepo({ repositoryPath: '.' });
 * if (result.ok) {
 *   const formatted = resultFormatters.analyzeRepo(result.value);
 *   // Use formatted string in LanguageModelToolResult
 * }
 * ```
 */

import type {
  RepositoryAnalysis,
  DockerfilePlan,
  DockerfileFixPlan,
  BuildImageResult,
  ScanImageResult,
  TagImageResult,
  PushImageResult,
  ManifestPlan,
  PrepareClusterResult,
  VerifyDeploymentResult,
  OpsResult,
  PingResult,
  ServerStatusResult,
} from './types.js';

// ===== FORMATTER OPTIONS =====

/**
 * Options for result formatting.
 */
export interface FormatterOptions {
  /** Include suggested next tool in output (default: true) */
  includeSuggestedNext?: boolean;
  /** Maximum length for truncated fields (default: 1000) */
  maxFieldLength?: number;
  /** Format as JSON instead of prose (default: false) */
  asJson?: boolean;
}

const defaultOptions: Readonly<Required<FormatterOptions>> = {
  includeSuggestedNext: true,
  maxFieldLength: 1000,
  asJson: false,
} as const;

// ===== UTILITY FUNCTIONS =====

function truncate(str: string, maxLength: number): string {
  if (str.length <= maxLength) return str;
  return `${str.slice(0, maxLength - 3)}...`;
}

function formatSection(title: string, content: string): string {
  return `## ${title}\n${content}\n`;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = ((ms % 60000) / 1000).toFixed(0);
  return `${minutes}m ${seconds}s`;
}

// ===== HIGHER-ORDER FORMATTER FACTORY =====

/**
 * Wraps a prose formatter function to handle common options like JSON output.
 *
 * This eliminates the repetitive pattern of checking `opts.asJson` at the
 * start of every formatter function.
 *
 * @param proseFormatter - The formatting function that produces prose output
 * @returns Wrapped function that handles asJson option automatically
 */
function createFormatter<T>(
  proseFormatter: (result: T, opts: Required<FormatterOptions>) => string,
): (result: T, options?: FormatterOptions) => string {
  return (result, options = {}) => {
    const opts = { ...defaultOptions, ...options };

    if (opts.asJson) {
      return JSON.stringify(result, null, 2);
    }

    return proseFormatter(result, opts);
  };
}

// ===== INDIVIDUAL FORMATTERS =====

/**
 * Format analyze-repo result prose output.
 */
function formatAnalyzeRepoResultProse(
  result: RepositoryAnalysis,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  // Summary
  if (result.summary) {
    sections.push(formatSection('Analysis Summary', result.summary));
  }

  // Modules
  if (result.modules && result.modules.length > 0) {
    const moduleList = result.modules
      .map((m) => {
        const parts = [`- **${m.name || 'root'}**`];
        if (m.language) parts.push(`Language: ${m.language}`);
        if (m.frameworks && m.frameworks.length > 0) {
          parts.push(`Framework: ${m.frameworks.map((f) => f.name).join(', ')}`);
        }
        if (m.buildSystems && m.buildSystems.length > 0) {
          parts.push(`Build: ${m.buildSystems.map((b) => b.type).join(', ')}`);
        }
        return parts.join(' | ');
      })
      .join('\n');
    sections.push(formatSection('Detected Modules', moduleList));
  } else if (result.language) {
    // Fallback to top-level fields if modules not populated
    const details: string[] = [];
    if (result.language) details.push(`- **Language**: ${result.language}${result.languageVersion ? ` ${result.languageVersion}` : ''}`);
    if (result.framework) details.push(`- **Framework**: ${result.framework}${result.frameworkVersion ? ` ${result.frameworkVersion}` : ''}`);
    if (result.buildSystem?.type) details.push(`- **Build System**: ${result.buildSystem.type}`);
    if (result.entryPoint) details.push(`- **Entry Point**: ${result.entryPoint}`);
    if (details.length > 0) {
      sections.push(formatSection('Detected Configuration', details.join('\n')));
    }
  }

  // Dependencies
  if (result.dependencies && result.dependencies.length > 0) {
    const depList = result.dependencies.slice(0, 10).join(', ');
    const moreText = result.dependencies.length > 10 ? ` (+${result.dependencies.length - 10} more)` : '';
    sections.push(formatSection('Key Dependencies', depList + moreText));
  }

  // Next steps
  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Use `generate_dockerfile` to create a Dockerfile based on this analysis.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format analyze-repo result for LLM consumption.
 */
export const formatAnalyzeRepoResult = createFormatter(formatAnalyzeRepoResultProse);

/**
 * Format generate-dockerfile result prose output.
 */
function formatGenerateDockerfileResultProse(
  result: DockerfilePlan,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  sections.push(formatSection('Dockerfile Plan Summary', result.summary));

  // Build strategy
  if (result.recommendations?.buildStrategy) {
    const strategy = result.recommendations.buildStrategy;
    sections.push(
      formatSection(
        'Build Strategy',
        `- **Multi-stage**: ${strategy.multistage ? 'Yes' : 'No'}\n- **Rationale**: ${strategy.reason}`,
      ),
    );
  }

  // Base images
  const baseImages = result.recommendations?.baseImages;
  if (baseImages && baseImages.length > 0) {
    const imageList = baseImages
      .map((img) => `- \`${img.image}\` (${img.category}): ${img.reason}`)
      .join('\n');
    sections.push(formatSection('Recommended Base Images', imageList));
  }

  // Security considerations
  const securityConsiderations = result.recommendations?.securityConsiderations;
  if (securityConsiderations && securityConsiderations.length > 0) {
    const secList = securityConsiderations
      .slice(0, 5)
      .map((sec) => `- [${sec.severity?.toUpperCase() || 'INFO'}] ${sec.recommendation}`)
      .join('\n');
    sections.push(formatSection('Security Considerations', truncate(secList, opts.maxFieldLength)));
  }

  // Optimizations
  const optimizations = result.recommendations?.optimizations;
  if (optimizations && optimizations.length > 0) {
    const optList = optimizations
      .slice(0, 5)
      .map((opt) => `- ${opt.recommendation}`)
      .join('\n');
    sections.push(formatSection('Optimizations', truncate(optList, opts.maxFieldLength)));
  }

  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Use this plan to write the Dockerfile, then use `build_image` to build it.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format generate-dockerfile result for LLM consumption.
 */
export const formatGenerateDockerfileResult = createFormatter(formatGenerateDockerfileResultProse);

/**
 * Format fix-dockerfile result prose output.
 */
function formatFixDockerfileResultProse(
  result: DockerfileFixPlan,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  sections.push(formatSection('Fix Analysis Summary', result.summary));

  // Validation score
  sections.push(
    formatSection(
      'Validation Score',
      `**Grade**: ${result.validationGrade} (${result.validationScore}/100)\n**Priority**: ${result.priority}`,
    ),
  );

  // Current issues
  const allIssues = [
    ...result.currentIssues.security.map((i) => ({ ...i, category: 'security' })),
    ...result.currentIssues.performance.map((i) => ({ ...i, category: 'performance' })),
    ...result.currentIssues.bestPractices.map((i) => ({ ...i, category: 'best-practices' })),
  ];

  if (allIssues.length > 0) {
    const issueList = allIssues
      .slice(0, 10)
      .map((issue) => {
        const severity = issue.priority?.toUpperCase() || issue.category?.toUpperCase() || 'INFO';
        return `- [${severity}] ${issue.message || issue.errors?.join(', ') || 'Issue detected'}`;
      })
      .join('\n');
    sections.push(formatSection('Issues Found', truncate(issueList, opts.maxFieldLength)));
  }

  // Fix recommendations
  const allFixes = [
    ...result.fixes.security,
    ...result.fixes.performance,
    ...result.fixes.bestPractices,
  ];

  if (allFixes.length > 0) {
    const recList = allFixes
      .slice(0, 5)
      .map((rec) => `- **${rec.title}**: ${rec.description}`)
      .join('\n');
    sections.push(formatSection('Recommendations', truncate(recList, opts.maxFieldLength)));
  }

  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Apply the fixes to the Dockerfile, then use `build_image` to test.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format fix-dockerfile result for LLM consumption.
 */
export const formatFixDockerfileResult = createFormatter(formatFixDockerfileResultProse);

/**
 * Format build-image result prose output.
 */
function formatBuildImageResultProse(
  result: BuildImageResult,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  if (result.summary) {
    sections.push(formatSection('Build Result', result.summary));
  } else {
    sections.push(
      formatSection('Build Result', result.success ? '✅ Build successful' : '❌ Build failed'),
    );
  }

  // Details
  const details: string[] = [];
  if (result.imageId) details.push(`- **Image ID**: \`${result.imageId.slice(0, 19)}...\``);
  if (result.createdTags?.length) {
    details.push(`- **Tags**: ${result.createdTags.map((t) => `\`${t}\``).join(', ')}`);
  }
  if (result.size) details.push(`- **Size**: ${formatBytes(result.size)}`);
  if (result.buildTime) details.push(`- **Build Time**: ${formatDuration(result.buildTime)}`);
  if (result.layers) details.push(`- **Layers**: ${result.layers}`);

  if (details.length > 0) {
    sections.push(formatSection('Build Details', details.join('\n')));
  }

  // Security warnings
  if (result.securityWarnings && result.securityWarnings.length > 0) {
    const warnings = result.securityWarnings.map((w) => `- ⚠️ ${w}`).join('\n');
    sections.push(formatSection('Security Warnings', warnings));
  }

  // Failed tags
  if (result.failedTags && result.failedTags.length > 0) {
    sections.push(
      formatSection('Failed Tags', result.failedTags.map((t) => `- \`${t}\``).join('\n')),
    );
  }

  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Use `scan_image` to check for vulnerabilities, or `tag_image` to add version tags.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format build-image result for LLM consumption.
 */
export const formatBuildImageResult = createFormatter(formatBuildImageResultProse);

/**
 * Format scan-image result prose output.
 */
function formatScanImageResultProse(
  result: ScanImageResult,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  if (result.summary) {
    sections.push(formatSection('Security Scan Summary', result.summary));
  } else {
    sections.push(
      formatSection(
        'Security Scan Summary',
        result.passed ? '✅ Security scan passed' : '❌ Security scan found issues',
      ),
    );
  }

  // Vulnerability counts
  if (result.vulnerabilities) {
    const counts = result.vulnerabilities;
    const countItems: string[] = [];
    if (counts.critical) countItems.push(`**Critical**: ${counts.critical}`);
    if (counts.high) countItems.push(`**High**: ${counts.high}`);
    if (counts.medium) countItems.push(`**Medium**: ${counts.medium}`);
    if (counts.low) countItems.push(`**Low**: ${counts.low}`);

    if (countItems.length > 0) {
      sections.push(formatSection('Vulnerability Counts', countItems.join(' | ')));
    } else {
      sections.push(formatSection('Vulnerability Counts', 'No vulnerabilities detected'));
    }
  }

  // Remediation guidance
  if (result.remediationGuidance && result.remediationGuidance.length > 0) {
    const guidance = result.remediationGuidance
      .slice(0, 5)
      .map((g) => `- **${g.vulnerability}**: ${g.recommendation}`)
      .join('\n');
    sections.push(formatSection('Remediation Guidance', truncate(guidance, opts.maxFieldLength)));
  }

  if (opts.includeSuggestedNext) {
    const hasIssues = result.vulnerabilities?.critical || result.vulnerabilities?.high;
    sections.push(
      formatSection(
        'Suggested Next Step',
        hasIssues
          ? 'Address critical/high vulnerabilities using `fix_dockerfile`, then rebuild.'
          : 'Image looks good! Use `tag_image` and `push_image` to publish.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format scan-image result for LLM consumption.
 */
export const formatScanImageResult = createFormatter(formatScanImageResultProse);

/**
 * Format tag-image result prose output.
 */
function formatTagImageResultProse(
  result: TagImageResult,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  if (result.summary) {
    sections.push(formatSection('Tag Result', result.summary));
  } else {
    sections.push(
      formatSection(
        'Tag Result',
        result.success
          ? `✅ Tagged image \`${result.imageId}\` with: ${result.tags.map((t) => `\`${t}\``).join(', ')}`
          : '❌ Tagging failed',
      ),
    );
  }

  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Use `push_image` to upload the tagged image to a registry.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format tag-image result for LLM consumption.
 */
export const formatTagImageResult = createFormatter(formatTagImageResultProse);

/**
 * Format push-image result prose output.
 */
function formatPushImageResultProse(
  result: PushImageResult,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  if (result.summary) {
    sections.push(formatSection('Push Result', result.summary));
  } else {
    sections.push(
      formatSection(
        'Push Result',
        `✅ Pushed to \`${result.registry}\`\n\n**Tag**: \`${result.pushedTag}\``,
      ),
    );
  }

  if (result.digest) {
    // Truncate digest for readability
    const digestShort =
      result.digest.length > 20 ? `${result.digest.slice(0, 20)}...` : result.digest;
    sections.push(`**Digest**: \`${digestShort}\``);
  }

  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Use `generate_k8s_manifests` to create Kubernetes deployment configs for this image.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format push-image result for LLM consumption.
 */
export const formatPushImageResult = createFormatter(formatPushImageResultProse);

/**
 * Format generate-k8s-manifests result prose output.
 */
function formatGenerateK8sManifestsResultProse(
  result: ManifestPlan,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  sections.push(formatSection('Kubernetes Manifest Plan', result.summary));

  // Manifest type
  sections.push(`**Manifest Type**: ${result.manifestType}`);

  // Repository info
  if (result.repositoryInfo) {
    const info = result.repositoryInfo;
    const details: string[] = [];
    if (info.name) details.push(`- **Name**: ${info.name}`);
    if (info.language) details.push(`- **Language**: ${info.language}`);
    if (info.ports && info.ports.length > 0) details.push(`- **Ports**: ${info.ports.join(', ')}`);
    if (details.length > 0) {
      sections.push(formatSection('Application Info', details.join('\n')));
    }
  }

  // Security considerations
  const k8sSecurityConsiderations = result.recommendations?.securityConsiderations;
  if (k8sSecurityConsiderations && k8sSecurityConsiderations.length > 0) {
    const secList = k8sSecurityConsiderations
      .slice(0, 5)
      .map((sec) => `- ${sec.recommendation}`)
      .join('\n');
    sections.push(formatSection('Security Considerations', truncate(secList, opts.maxFieldLength)));
  }

  // Best practices
  const bestPractices = result.recommendations?.bestPractices;
  if (bestPractices && bestPractices.length > 0) {
    const bpList = bestPractices
      .slice(0, 5)
      .map((bp) => `- ${bp.recommendation}`)
      .join('\n');
    sections.push(formatSection('Best Practices', truncate(bpList, opts.maxFieldLength)));
  }

  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Use this plan to write the YAML manifests, then `prepare_cluster` and apply them.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format generate-k8s-manifests result for LLM consumption.
 */
export const formatGenerateK8sManifestsResult = createFormatter(formatGenerateK8sManifestsResultProse);

/**
 * Format prepare-cluster result prose output.
 */
function formatPrepareClusterResultProse(
  result: PrepareClusterResult,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  if (result.summary) {
    sections.push(formatSection('Cluster Preparation', result.summary));
  } else {
    sections.push(
      formatSection(
        'Cluster Preparation',
        result.clusterReady ? '✅ Cluster is ready' : '❌ Cluster preparation incomplete',
      ),
    );
  }

  // Cluster details
  const details: string[] = [];
  details.push(`- **Cluster**: ${result.cluster}`);
  details.push(`- **Namespace**: ${result.namespace}`);
  if (result.localRegistryUrl) {
    details.push(`- **Local Registry**: ${result.localRegistryUrl}`);
  }
  sections.push(formatSection('Configuration', details.join('\n')));

  // Checks status
  const checks = result.checks;
  const checkItems: string[] = [];
  checkItems.push(`- Connectivity: ${checks.connectivity ? '✓' : '✗'}`);
  checkItems.push(`- Permissions: ${checks.permissions ? '✓' : '✗'}`);
  checkItems.push(`- Namespace: ${checks.namespaceExists ? '✓' : '✗'}`);
  if (checks.kindClusterCreated !== undefined) {
    checkItems.push(`- Kind Cluster: ${checks.kindClusterCreated ? '✓' : '✗'}`);
  }
  if (checks.localRegistryCreated !== undefined) {
    checkItems.push(`- Local Registry: ${checks.localRegistryCreated ? '✓' : '✗'}`);
  }
  sections.push(formatSection('Status Checks', checkItems.join('\n')));

  // Warnings
  if (result.warnings && result.warnings.length > 0) {
    const warnings = result.warnings.map((w) => `- ⚠️ ${w}`).join('\n');
    sections.push(formatSection('Warnings', warnings));
  }

  if (opts.includeSuggestedNext) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Apply your Kubernetes manifests using kubectl, then use `verify_deploy` to check status.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format prepare-cluster result for LLM consumption.
 */
export const formatPrepareClusterResult = createFormatter(formatPrepareClusterResultProse);

/**
 * Format verify-deploy result prose output.
 */
function formatVerifyDeployResultProse(
  result: VerifyDeploymentResult,
  opts: Required<FormatterOptions>,
): string {
  const sections: string[] = [];

  if (result.summary) {
    sections.push(formatSection('Deployment Verification', result.summary));
  } else {
    sections.push(
      formatSection(
        'Deployment Verification',
        result.ready ? '✅ Deployment is healthy' : '❌ Deployment has issues',
      ),
    );
  }

  // Status details
  const details: string[] = [];
  details.push(`- **Deployment**: ${result.deploymentName}`);
  details.push(`- **Namespace**: ${result.namespace}`);
  details.push(`- **Status**: ${result.ready ? 'Ready' : 'Not Ready'}`);
  details.push(`- **Replicas**: ${result.status.readyReplicas}/${result.status.totalReplicas} ready`);
  sections.push(formatSection('Status', details.join('\n')));

  // Endpoints
  if (result.endpoints && result.endpoints.length > 0) {
    const endpoints = result.endpoints
      .map((e) => `- ${e.type}: \`${e.url}:${e.port}\` ${e.healthy ? '✓' : e.healthy === false ? '✗' : ''}`)
      .join('\n');
    sections.push(formatSection('Endpoints', endpoints));
  }

  // Health check
  if (result.healthCheck) {
    sections.push(
      formatSection(
        'Health Check',
        `**Status**: ${result.healthCheck.status}\n${result.healthCheck.message}`,
      ),
    );
  }

  // Conditions
  if (result.status.conditions && result.status.conditions.length > 0) {
    const conditions = result.status.conditions
      .map((c) => `- **${c.type}**: ${c.status} - ${c.message}`)
      .join('\n');
    sections.push(formatSection('Conditions', conditions));
  }

  // Suggested next step for failing deployments
  if (opts.includeSuggestedNext && !result.ready) {
    sections.push(
      formatSection(
        'Suggested Next Step',
        'Check pod logs and events for error details. Use `kubectl logs` and `kubectl describe pod`.',
      ),
    );
  }

  return sections.join('\n');
}

/**
 * Format verify-deploy result for LLM consumption.
 */
export const formatVerifyDeployResult = createFormatter(formatVerifyDeployResultProse);

/**
 * Format a PingResult to prose output.
 */
function formatPingResultProse(result: PingResult): string {
  if (result.summary) {
    return `## Ping Result\n${result.summary}`;
  }
  return `## Ping Result\n${result.message}\n\n**Server**: ${result.server.name} v${result.server.version}`;
}

/**
 * Format a ServerStatusResult to prose output.
 */
function formatServerStatusResultProse(result: ServerStatusResult): string {
  const sections: string[] = [];

  if (result.summary) {
    sections.push(formatSection('Server Status', result.summary));
  } else {
    sections.push(formatSection('Server Status', '✅ Server is healthy'));
  }

  sections.push(`- **Version**: ${result.version}`);
  sections.push(`- **Uptime**: ${formatDuration(result.uptime * 1000)}`);
  sections.push(`- **Memory**: ${result.memory.percentage}% used`);
  sections.push(`- **CPU Cores**: ${result.cpu.cores}`);

  return sections.join('\n');
}

/**
 * Format ops result prose output.
 *
 * Uses discriminated union pattern with `kind` field for type-safe narrowing.
 * No type guards or unsafe casts needed.
 */
function formatOpsResultProse(
  result: OpsResult,
  _opts: Required<FormatterOptions>,
): string {
  switch (result.kind) {
    case 'ping':
      return formatPingResultProse(result);
    case 'status':
      return formatServerStatusResultProse(result);
    default: {
      // Exhaustiveness check - TypeScript will error if we miss a case
      // The void cast avoids unused variable warning while maintaining the type check
      void (result satisfies never);
      // Fallback for runtime safety (should never be reached)
      const genericResult = result as { summary?: string };
      return `## Operations Result\n${genericResult.summary ?? 'Operation completed successfully.'}`;
    }
  }
}

/**
 * Format ops result for LLM consumption.
 */
export const formatOpsResult = createFormatter(formatOpsResultProse);

// ===== FORMATTER REGISTRY TYPE =====

/**
 * Type for a formatter function.
 */
type FormatterFunction<T> = (result: T, options?: FormatterOptions) => string;

/**
 * Registry of all available formatters with their expected result types.
 *
 * This interface ensures type safety when adding or modifying formatters.
 * TypeScript will error if a formatter is missing or has the wrong signature.
 */
interface FormatterRegistry {
  analyzeRepo: FormatterFunction<RepositoryAnalysis>;
  generateDockerfile: FormatterFunction<DockerfilePlan>;
  fixDockerfile: FormatterFunction<DockerfileFixPlan>;
  buildImage: FormatterFunction<BuildImageResult>;
  scanImage: FormatterFunction<ScanImageResult>;
  tagImage: FormatterFunction<TagImageResult>;
  pushImage: FormatterFunction<PushImageResult>;
  generateK8sManifests: FormatterFunction<ManifestPlan>;
  prepareCluster: FormatterFunction<PrepareClusterResult>;
  verifyDeploy: FormatterFunction<VerifyDeploymentResult>;
  ops: FormatterFunction<OpsResult>;
}

// ===== AGGREGATED EXPORT =====

/**
 * All result formatters indexed by tool name.
 *
 * Uses `satisfies` to ensure type safety:
 * - TypeScript will error if a formatter is missing
 * - TypeScript will error if a formatter has the wrong signature
 * - TypeScript will error if a key is misspelled
 *
 * @example
 * ```typescript
 * import { resultFormatters } from 'containerization-assist-mcp/sdk';
 *
 * const result = await analyzeRepo({ repositoryPath: '.' });
 * if (result.ok) {
 *   const formatted = resultFormatters.analyzeRepo(result.value);
 * }
 * ```
 */
export const resultFormatters = {
  analyzeRepo: formatAnalyzeRepoResult,
  generateDockerfile: formatGenerateDockerfileResult,
  fixDockerfile: formatFixDockerfileResult,
  buildImage: formatBuildImageResult,
  scanImage: formatScanImageResult,
  tagImage: formatTagImageResult,
  pushImage: formatPushImageResult,
  generateK8sManifests: formatGenerateK8sManifestsResult,
  prepareCluster: formatPrepareClusterResult,
  verifyDeploy: formatVerifyDeployResult,
  ops: formatOpsResult,
} as const satisfies FormatterRegistry;

/**
 * Type for resultFormatters object keys.
 */
export type FormatterName = keyof typeof resultFormatters;

/**
 * Export the registry type for consumers who need to extend or type-check formatters.
 */
export type { FormatterRegistry };
