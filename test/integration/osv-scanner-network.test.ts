/**
 * Integration tests for OSV Scanner with cached fixtures
 *
 * These tests exercise the full OSV scanning pipeline without network or Docker:
 *   - extractPackagesFromImage is mocked to return fixture data
 *   - global fetch is mocked to return cached OSV API responses
 *
 * This eliminates flakiness from Docker image builds, OSV API availability,
 * and network timeouts while still validating the scan-result assembly logic.
 */

import { describe, it, expect, beforeAll, afterAll, jest } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import type { Logger } from 'pino';
import type { ExtractedPackage } from '@/infra/security/osv-scanner/maven/types';
import type { OSVBatchResponse, OSVVulnerability } from '@/infra/security/osv-scanner/osv-api';

// ---------------------------------------------------------------------------
// Fixtures — cached responses representing a log4j-core 2.14.1 scan
// ---------------------------------------------------------------------------

const FIXTURE_PACKAGES: ExtractedPackage[] = [
  {
    name: 'org.apache.logging.log4j:log4j-core',
    version: '2.14.1',
    ecosystem: 'Maven',
    path: '/app/pom.xml',
  },
];

/** Batch query response: maps the single package to two known vuln IDs */
const FIXTURE_BATCH_RESPONSE: OSVBatchResponse = {
  results: [
    {
      vulns: [
        { id: 'GHSA-jfh8-c2jp-5v3q' }, // Log4Shell
        { id: 'GHSA-7rjr-3q55-vv33' }, // Log4j RCE (related)
      ],
    },
  ],
};

/** Full vulnerability detail for GHSA-jfh8-c2jp-5v3q (Log4Shell) */
const FIXTURE_VULN_LOG4SHELL: OSVVulnerability = {
  id: 'GHSA-jfh8-c2jp-5v3q',
  summary: 'Remote code injection in Apache Log4j (Log4Shell)',
  details:
    'Apache Log4j2 <=2.14.1 JNDI features used in configuration, log messages, ' +
    'and parameters do not protect against attacker controlled LDAP and other JNDI related endpoints.',
  aliases: ['CVE-2021-44228'],
  severity: [
    {
      type: 'CVSS_V3',
      score: 'CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H',
    },
  ],
  affected: [
    {
      package: {
        name: 'org.apache.logging.log4j:log4j-core',
        ecosystem: 'Maven',
      },
      ranges: [
        {
          type: 'ECOSYSTEM',
          events: [{ introduced: '2.0' }, { fixed: '2.15.0' }],
        },
      ],
    },
  ],
};

/** Full vulnerability detail for GHSA-7rjr-3q55-vv33 */
const FIXTURE_VULN_LOG4J_RCE: OSVVulnerability = {
  id: 'GHSA-7rjr-3q55-vv33',
  summary: 'Incomplete fix for Apache Log4j vulnerability',
  details:
    'Apache Log4j2 versions 2.0-beta7 through 2.17.0 (excluding security fix releases 2.3.1, 2.12.3, and 2.17.0) ' +
    'are vulnerable to a remote code execution attack.',
  aliases: ['CVE-2021-44832'],
  severity: [
    {
      type: 'CVSS_V3',
      score: 'CVSS:3.1/AV:N/AC:H/PR:H/UI:N/S:U/C:H/I:H/A:H',
    },
  ],
  affected: [
    {
      package: {
        name: 'org.apache.logging.log4j:log4j-core',
        ecosystem: 'Maven',
      },
      ranges: [
        {
          type: 'ECOSYSTEM',
          events: [{ introduced: '2.0' }, { fixed: '2.17.1' }],
        },
      ],
    },
  ],
};

const FIXTURE_VULNS: Record<string, OSVVulnerability> = {
  'GHSA-jfh8-c2jp-5v3q': FIXTURE_VULN_LOG4SHELL,
  'GHSA-7rjr-3q55-vv33': FIXTURE_VULN_LOG4J_RCE,
};

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// Mock package extractor — avoids Docker entirely
jest.mock('@/infra/security/osv-scanner/package-extractor', () => ({
  extractPackagesFromImage: jest
    .fn<() => Promise<ExtractedPackage[]>>()
    .mockResolvedValue(FIXTURE_PACKAGES),
}));

// Mock Docker socket detection — avoids accessing real Docker socket
jest.mock('@/infra/docker/socket-validation', () => ({
  autoDetectDockerSocket: jest.fn().mockReturnValue('/var/run/docker.sock'),
}));

// Mock dockerode — avoids real Docker connection
jest.mock('dockerode', () => {
  return jest.fn().mockImplementation(() => ({}));
});

// Save and restore real fetch
const realFetch = globalThis.fetch;

beforeAll(() => {
  // Replace global fetch with a mock that returns cached OSV responses
  globalThis.fetch = jest.fn<typeof fetch>().mockImplementation(async (input, _init?) => {
    const url = typeof input === 'string' ? input : (input as Request).url;

    // OSV availability check (POST to /v1/query)
    if (url === 'https://api.osv.dev/v1/query') {
      return new Response(JSON.stringify({ vulns: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    // Batch query (POST to /v1/querybatch)
    if (url === 'https://api.osv.dev/v1/querybatch') {
      return new Response(JSON.stringify(FIXTURE_BATCH_RESPONSE), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    // Individual vulnerability details (GET /v1/vulns/{id})
    const vulnMatch = url.match(/^https:\/\/api\.osv\.dev\/v1\/vulns\/(.+)$/);
    if (vulnMatch) {
      const vulnId = decodeURIComponent(vulnMatch[1]);
      const vuln = FIXTURE_VULNS[vulnId];
      if (vuln) {
        return new Response(JSON.stringify(vuln), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response('Not Found', { status: 404 });
    }

    // Unexpected URL — fail loudly so we notice
    throw new Error(`Unexpected fetch URL in OSV test: ${url}`);
  });
});

afterAll(() => {
  globalThis.fetch = realFetch;
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('OSV Scanner Integration — Cached Fixtures', () => {
  let logger: Logger;

  beforeAll(() => {
    logger = createLogger({ level: 'warn' });
  });

  describe('API Availability', () => {
    it('should confirm OSV API available (cached)', async () => {
      // Dynamic import so mocks are applied first
      const { checkOSVAvailability } = await import('@/infra/security/osv-scanner/index');
      const result = await checkOSVAvailability(logger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('osv-api-available');
      }
    });

    it('should return severity data for Log4Shell from cached response', () => {
      const vuln = FIXTURE_VULN_LOG4SHELL;

      expect(vuln.id).toBe('GHSA-jfh8-c2jp-5v3q');
      expect(vuln.summary).toContain('Log4');

      // Verify CVSS vector is present
      expect(vuln.severity).toBeDefined();
      expect(vuln.severity!.length).toBeGreaterThan(0);

      const cvssV3 = vuln.severity!.find((s) => s.type === 'CVSS_V3');
      expect(cvssV3).toBeDefined();
      expect(cvssV3!.score).toMatch(/^CVSS:/);
    });
  });

  describe('Full Scan Pipeline', () => {
    it('should detect vulnerabilities in log4j-core 2.14.1 via cached fixtures', async () => {
      const { createSecurityScanner } = await import('@/infra/security/scanner');

      const scanner = createSecurityScanner(logger, 'osv');
      const result = await scanner.scanImage('osv-test:log4j-vuln');

      expect(result.ok).toBe(true);
      if (!result.ok) {
        console.error('Scan failed:', result.error);
        return;
      }

      const { vulnerabilities, totalVulnerabilities, criticalCount } = result.value;

      // Should find both Log4Shell vulnerabilities
      expect(totalVulnerabilities).toBeGreaterThanOrEqual(2);

      // Check that we found log4j-core vulnerability
      const log4jVuln = vulnerabilities.find((v) => v.package.includes('log4j'));
      expect(log4jVuln).toBeDefined();

      // Log4Shell should be detected as critical (CVSS 10.0)
      expect(criticalCount).toBeGreaterThanOrEqual(1);

      // Verify fixed version is extracted
      const log4shell = vulnerabilities.find((v) => v.id === 'GHSA-jfh8-c2jp-5v3q');
      expect(log4shell).toBeDefined();
      expect(log4shell!.fixedVersion).toBe('2.15.0');
      expect(log4shell!.severity).toBe('CRITICAL');
    }, 15000);
  });
});
