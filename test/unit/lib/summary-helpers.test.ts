/**
 * Tests for summary generation utilities
 */

import {
  formatDuration,
  formatSize,
  pluralize,
  buildStatusSummary,
  buildDetailedSummary,
  truncate,
  formatVulnerabilities,
  formatTimestamp,
  summarizeList,
  type VulnerabilitySummary,
} from '@/lib/summary-helpers';

describe('summary-helpers', () => {
  describe('formatDuration', () => {
    it('should format seconds', () => {
      expect(formatDuration(0)).toBe('0s');
      expect(formatDuration(30)).toBe('30s');
      expect(formatDuration(45)).toBe('45s');
      expect(formatDuration(59)).toBe('59s');
    });

    it('should format minutes and seconds', () => {
      expect(formatDuration(60)).toBe('1m');
      expect(formatDuration(90)).toBe('1m 30s');
      expect(formatDuration(125)).toBe('2m 5s');
      expect(formatDuration(3599)).toBe('59m 59s');
    });

    it('should format hours and minutes', () => {
      expect(formatDuration(3600)).toBe('1h');
      expect(formatDuration(3660)).toBe('1h 1m');
      expect(formatDuration(7200)).toBe('2h');
      expect(formatDuration(5400)).toBe('1h 30m');
    });

    it('should handle negative values', () => {
      expect(formatDuration(-10)).toBe('0s');
      expect(formatDuration(-100)).toBe('0s');
    });

    it('should handle decimal values', () => {
      expect(formatDuration(45.7)).toBe('46s');
      expect(formatDuration(90.3)).toBe('1m 30s');
    });
  });

  describe('formatSize', () => {
    it('should format bytes', () => {
      expect(formatSize(0)).toBe('0B');
      expect(formatSize(500)).toBe('500B');
      expect(formatSize(1023)).toBe('1023B');
    });

    it('should format kilobytes', () => {
      expect(formatSize(1024)).toBe('1KB');
      expect(formatSize(1536)).toBe('2KB');
      expect(formatSize(10240)).toBe('10KB');
    });

    it('should format megabytes', () => {
      expect(formatSize(1048576)).toBe('1MB');
      expect(formatSize(1572864)).toBe('2MB');
      expect(formatSize(245678234)).toBe('234MB');
    });

    it('should format gigabytes', () => {
      expect(formatSize(1073741824)).toBe('1GB');
      expect(formatSize(2147483648)).toBe('2GB');
    });

    it('should format terabytes', () => {
      expect(formatSize(1099511627776)).toBe('1TB');
      expect(formatSize(2199023255552)).toBe('2TB');
    });

    it('should handle negative values', () => {
      expect(formatSize(-100)).toBe('0B');
    });

    it('should round to whole numbers', () => {
      expect(formatSize(1536)).toBe('2KB'); // 1.5 KB rounds to 2
      expect(formatSize(1200)).toBe('1KB'); // 1.17 KB rounds to 1
    });
  });

  describe('pluralize', () => {
    it('should handle singular correctly', () => {
      expect(pluralize(1, 'file')).toBe('1 file');
      expect(pluralize(1, 'vulnerability')).toBe('1 vulnerability');
    });

    it('should handle plural with default suffix', () => {
      expect(pluralize(0, 'file')).toBe('0 files');
      expect(pluralize(2, 'file')).toBe('2 files');
      expect(pluralize(10, 'replica')).toBe('10 replicas');
    });

    it('should handle plural with custom suffix', () => {
      expect(pluralize(2, 'vulnerability', 'vulnerabilities')).toBe('2 vulnerabilities');
      expect(pluralize(5, 'index', 'indices')).toBe('5 indices');
    });
  });

  describe('buildStatusSummary', () => {
    it('should build success summary with checkmark', () => {
      const result = buildStatusSummary(true, 'Operation succeeded', 'Operation failed');
      expect(result).toBe('✅ Operation succeeded');
      expect(result).toContain('✅');
    });

    it('should build failure summary with X mark', () => {
      const result = buildStatusSummary(false, 'Operation succeeded', 'Operation failed');
      expect(result).toBe('❌ Operation failed');
      expect(result).toContain('❌');
    });

    it('should work with different messages', () => {
      expect(buildStatusSummary(true, 'Build completed', 'Build failed')).toBe('✅ Build completed');
      expect(buildStatusSummary(false, 'Tests passed', 'Tests failed')).toBe('❌ Tests failed');
    });
  });

  describe('buildDetailedSummary', () => {
    it('should build summary with heading only', () => {
      const result = buildDetailedSummary('Test Heading', []);
      expect(result).toBe('Test Heading');
    });

    it('should build summary with heading and details', () => {
      const result = buildDetailedSummary('Deployment Complete', [
        '3 replicas running',
        'Service exposed on port 8080',
      ]);
      expect(result).toBe(
        'Deployment Complete\n  • 3 replicas running\n  • Service exposed on port 8080',
      );
    });

    it('should build summary with heading, details, and next steps', () => {
      const result = buildDetailedSummary(
        'Deployment Complete',
        ['3 replicas running', 'Service exposed on port 8080'],
        ['Verify health endpoints', 'Monitor logs'],
      );
      expect(result).toContain('Deployment Complete');
      expect(result).toContain('  • 3 replicas running');
      expect(result).toContain('Next steps:');
      expect(result).toContain('  → Verify health endpoints');
      expect(result).toContain('  → Monitor logs');
    });

    it('should handle empty next steps', () => {
      const result = buildDetailedSummary('Test', ['Detail 1'], []);
      expect(result).not.toContain('Next steps:');
      expect(result).toBe('Test\n  • Detail 1');
    });
  });

  describe('truncate', () => {
    it('should not truncate text shorter than max length', () => {
      expect(truncate('Short', 10)).toBe('Short');
      expect(truncate('Test', 10)).toBe('Test');
    });

    it('should truncate text longer than max length', () => {
      expect(truncate('This is a long message', 10)).toBe('This is...');
      expect(truncate('Very long text here', 12)).toBe('Very long...');
    });

    it('should handle edge cases', () => {
      expect(truncate('abc', 3)).toBe('abc');
      expect(truncate('abcd', 3)).toBe('...');
      expect(truncate('', 10)).toBe('');
    });
  });

  describe('formatVulnerabilities', () => {
    it('should format no significant vulnerabilities', () => {
      const vulns: VulnerabilitySummary = {
        critical: 0,
        high: 0,
        medium: 0,
        low: 5,
        total: 5,
      };
      expect(formatVulnerabilities(vulns)).toBe('No significant vulnerabilities');
    });

    it('should format critical vulnerabilities', () => {
      const vulns: VulnerabilitySummary = {
        critical: 2,
        high: 0,
        medium: 0,
        low: 0,
        total: 2,
      };
      expect(formatVulnerabilities(vulns)).toBe('2 vulnerabilities (2 critical)');
    });

    it('should format multiple severity levels', () => {
      const vulns: VulnerabilitySummary = {
        critical: 2,
        high: 5,
        medium: 12,
        low: 34,
        total: 53,
      };
      expect(formatVulnerabilities(vulns)).toBe(
        '53 vulnerabilities (2 critical, 5 high, 12 medium)',
      );
    });

    it('should handle high and medium only', () => {
      const vulns: VulnerabilitySummary = {
        critical: 0,
        high: 3,
        medium: 8,
        low: 10,
        total: 21,
      };
      expect(formatVulnerabilities(vulns)).toBe('21 vulnerabilities (3 high, 8 medium)');
    });

    it('should handle singular vulnerability', () => {
      const vulns: VulnerabilitySummary = {
        critical: 1,
        high: 0,
        medium: 0,
        low: 0,
        total: 1,
      };
      expect(formatVulnerabilities(vulns)).toBe('1 vulnerability (1 critical)');
    });
  });

  describe('formatTimestamp', () => {
    it('should format ISO timestamp string', () => {
      const result = formatTimestamp('2025-01-15T10:30:00.000Z');
      expect(result).toBe('2025-01-15 10:30:00');
    });

    it('should format Date object', () => {
      const date = new Date('2025-01-15T10:30:00.000Z');
      const result = formatTimestamp(date);
      expect(result).toBe('2025-01-15 10:30:00');
    });

    it('should handle invalid timestamp', () => {
      const result = formatTimestamp('invalid');
      expect(result).toBe('invalid');
    });
  });

  describe('summarizeList', () => {
    it('should handle empty array', () => {
      expect(summarizeList([])).toBe('none');
    });

    it('should list all items when under limit', () => {
      expect(summarizeList(['v1.0.0'])).toBe('v1.0.0');
      expect(summarizeList(['v1.0.0', 'latest'])).toBe('v1.0.0, latest');
      expect(summarizeList(['v1.0.0', 'latest', 'stable'])).toBe('v1.0.0, latest, stable');
    });

    it('should truncate when over limit', () => {
      expect(summarizeList(['tag1', 'tag2', 'tag3', 'tag4'])).toBe('tag1, tag2, tag3, and 1 more');
      expect(summarizeList(['a', 'b', 'c', 'd', 'e'])).toBe('a, b, c, and 2 more');
    });

    it('should respect custom maxItems', () => {
      expect(summarizeList(['a', 'b', 'c', 'd'], 2)).toBe('a, b, and 2 more');
      expect(summarizeList(['a', 'b', 'c', 'd', 'e'], 1)).toBe('a, and 4 more');
    });
  });
});
