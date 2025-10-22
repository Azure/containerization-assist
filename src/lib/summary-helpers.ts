/**
 * Summary generation utilities for natural language tool outputs
 *
 * Provides consistent formatting for human-readable summaries across all tools.
 * These utilities help maintain a consistent tone, style, and format.
 *
 * @module lib/summary-helpers
 */

/**
 * Format duration in human-readable form
 *
 * @param seconds - Duration in seconds
 * @returns Human-readable duration string
 *
 * @example
 * formatDuration(30) // "30s"
 * formatDuration(90) // "1m 30s"
 * formatDuration(3665) // "1h 1m"
 */
export function formatDuration(seconds: number): string {
  if (seconds < 0) return '0s';

  if (seconds < 60) {
    return `${Math.round(seconds)}s`;
  }

  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.round(seconds % 60);
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
  }

  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

/**
 * Format byte size in human-readable form
 *
 * @param bytes - Size in bytes
 * @returns Human-readable size string
 *
 * @example
 * formatSize(1024) // "1KB"
 * formatSize(1536) // "2KB"
 * formatSize(1048576) // "1MB"
 * formatSize(245678234) // "234MB"
 */
export function formatSize(bytes: number): string {
  if (bytes < 0) return '0B';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = bytes;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  // Round to whole number for cleaner display
  const rounded = Math.round(size);
  return `${rounded}${units[unitIndex]}`;
}

/**
 * Format plurals correctly
 *
 * @param count - Number of items
 * @param singular - Singular form of the word
 * @param plural - Optional plural form (defaults to singular + 's')
 * @returns Formatted string with count and properly pluralized word
 *
 * @example
 * pluralize(1, 'file') // "1 file"
 * pluralize(3, 'file') // "3 files"
 * pluralize(2, 'vulnerability', 'vulnerabilities') // "2 vulnerabilities"
 */
export function pluralize(count: number, singular: string, plural?: string): string {
  if (count === 1) {
    return `${count} ${singular}`;
  }
  return `${count} ${plural || `${singular}s`}`;
}

/**
 * Build summary with icon based on success status
 *
 * @param success - Whether the operation succeeded
 * @param successMessage - Message to display on success
 * @param failureMessage - Message to display on failure
 * @returns Formatted summary with status icon
 *
 * @example
 * buildStatusSummary(true, 'Build completed', 'Build failed')
 * // "✅ Build completed"
 *
 * buildStatusSummary(false, 'Tests passed', 'Tests failed')
 * // "❌ Tests failed"
 */
export function buildStatusSummary(
  success: boolean,
  successMessage: string,
  failureMessage: string,
): string {
  const icon = success ? '✅' : '❌';
  const message = success ? successMessage : failureMessage;
  return `${icon} ${message}`;
}

/**
 * Build multi-part summary with bullet points
 *
 * Creates a detailed summary with sections, useful for NATURAL_LANGUAGE format.
 *
 * @param heading - Main heading for the summary
 * @param details - Array of detail lines (will be bulleted)
 * @param nextSteps - Optional array of next step items (will be arrowed)
 * @returns Multi-line formatted summary
 *
 * @example
 * buildDetailedSummary(
 *   'Deployment Complete',
 *   ['3 replicas running', 'Service exposed on port 8080'],
 *   ['Verify health endpoints', 'Monitor logs']
 * )
 * // Returns:
 * // "Deployment Complete
 * //   • 3 replicas running
 * //   • Service exposed on port 8080
 * // Next steps:
 * //   → Verify health endpoints
 * //   → Monitor logs"
 */
export function buildDetailedSummary(
  heading: string,
  details: string[],
  nextSteps?: string[],
): string {
  const parts = [heading];

  if (details.length > 0) {
    parts.push(...details.map((d) => `  • ${d}`));
  }

  if (nextSteps && nextSteps.length > 0) {
    parts.push('Next steps:');
    parts.push(...nextSteps.map((s) => `  → ${s}`));
  }

  return parts.join('\n');
}

/**
 * Truncate text with ellipsis
 *
 * @param text - Text to truncate
 * @param maxLength - Maximum length including ellipsis
 * @returns Truncated text
 *
 * @example
 * truncate('This is a long message', 10) // "This is..."
 * truncate('Short', 10) // "Short"
 */
export function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.substring(0, maxLength - 3)}...`;
}

/**
 * Vulnerability summary for formatVulnerabilities
 */
export interface VulnerabilitySummary {
  critical: number;
  high: number;
  medium: number;
  low: number;
  total: number;
}

/**
 * Format vulnerability counts in human-readable form
 *
 * Focuses on critical, high, and medium vulnerabilities for concise summaries.
 *
 * @param vulns - Vulnerability counts by severity
 * @returns Formatted vulnerability summary
 *
 * @example
 * formatVulnerabilities({ critical: 2, high: 5, medium: 12, low: 34, total: 53 })
 * // "53 vulnerabilities (2 critical, 5 high, 12 medium)"
 *
 * formatVulnerabilities({ critical: 0, high: 0, medium: 0, low: 5, total: 5 })
 * // "No significant vulnerabilities"
 */
export function formatVulnerabilities(vulns: VulnerabilitySummary): string {
  const parts: string[] = [];

  if (vulns.critical > 0) {
    parts.push(`${vulns.critical} critical`);
  }
  if (vulns.high > 0) {
    parts.push(`${vulns.high} high`);
  }
  if (vulns.medium > 0) {
    parts.push(`${vulns.medium} medium`);
  }

  if (parts.length === 0) {
    return 'No significant vulnerabilities';
  }

  const vulnWord = vulns.total === 1 ? 'vulnerability' : 'vulnerabilities';
  return `${vulns.total} ${vulnWord} (${parts.join(', ')})`;
}

/**
 * Format timestamp in human-readable form
 *
 * @param timestamp - ISO timestamp string or Date object
 * @returns Human-readable timestamp
 *
 * @example
 * formatTimestamp('2025-01-15T10:30:00Z') // "2025-01-15 10:30:00"
 */
export function formatTimestamp(timestamp: string | Date): string {
  try {
    const date = typeof timestamp === 'string' ? new Date(timestamp) : timestamp;
    return date.toISOString().replace('T', ' ').replace(/\.\d+Z$/, '');
  } catch {
    return String(timestamp);
  }
}

/**
 * Build a concise list summary
 *
 * Useful for summarizing arrays with truncation for long lists.
 *
 * @param items - Array of items to summarize
 * @param maxItems - Maximum items to show before truncation (default: 3)
 * @returns Formatted list summary
 *
 * @example
 * summarizeList(['v1.0.0', 'latest', 'stable']) // "v1.0.0, latest, stable"
 * summarizeList(['tag1', 'tag2', 'tag3', 'tag4'], 2) // "tag1, tag2, and 2 more"
 */
export function summarizeList(items: string[], maxItems = 3): string {
  if (items.length === 0) {
    return 'none';
  }

  if (items.length <= maxItems) {
    return items.join(', ');
  }

  const shown = items.slice(0, maxItems);
  const remaining = items.length - maxItems;
  return `${shown.join(', ')}, and ${remaining} more`;
}
