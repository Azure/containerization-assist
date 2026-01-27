/**
 * OSV API Client
 *
 * Rate-limited HTTP client for OSV vulnerability database.
 * Token bucket: 10 req/sec sustained, 10 burst capacity.
 */

import type { Logger } from 'pino';

import { extractErrorMessage } from '@/lib/errors';
import type { StandardSeverity } from '../scanner-common';
import type { ExtractedPackage } from './maven/types';

/**
 * Token bucket rate limiter
 * Prevents API abuse with 10 req/sec steady state, 10 burst capacity
 */
class RateLimiter {
  private tokens: number;
  private lastRefill: number;
  private readonly maxTokens: number;
  private readonly refillRate: number;
  private queue: Array<() => void> = [];
  private refillTimer: NodeJS.Timeout | null = null;

  constructor(maxTokens: number = 10, refillRate: number = 10) {
    this.maxTokens = maxTokens;
    this.refillRate = refillRate;
    this.tokens = maxTokens;
    this.lastRefill = Date.now();
  }

  private refill(): void {
    const now = Date.now();
    const timePassed = (now - this.lastRefill) / 1000;
    const tokensToAdd = timePassed * this.refillRate;

    this.tokens = Math.min(this.maxTokens, this.tokens + tokensToAdd);
    this.lastRefill = now;

    // Process queue if we have tokens
    this.processQueue();
  }

  private processQueue(): void {
    while (this.queue.length > 0 && this.tokens >= 1) {
      const resolve = this.queue.shift();
      this.tokens -= 1;
      if (resolve) resolve();
    }

    // Schedule next refill if queue has waiting requests
    if (this.queue.length > 0 && !this.refillTimer) {
      const nextRefill = (1 / this.refillRate) * 1000;
      this.refillTimer = setTimeout(() => {
        this.refillTimer = null;
        this.refill();
      }, nextRefill);
    }
  }

  async acquire(tokens: number = 1): Promise<void> {
    if (tokens !== 1) {
      throw new Error('Only single token acquisition is supported');
    }

    this.refill();

    if (this.tokens >= 1) {
      this.tokens -= 1;
      return;
    }

    // Wait in queue
    return new Promise<void>((resolve) => {
      this.queue.push(resolve);
      this.processQueue();
    });
  }

  reset(): void {
    this.tokens = this.maxTokens;
    this.lastRefill = Date.now();
    if (this.refillTimer) {
      clearTimeout(this.refillTimer);
      this.refillTimer = null;
    }
    // Resolve all pending promises to prevent memory leaks
    while (this.queue.length > 0) {
      const resolve = this.queue.shift();
      if (resolve) resolve();
    }
  }
}

export const osvRateLimiter = new RateLimiter(10, 10);

// OSV API types
export interface OSVPackage {
  name: string;
  ecosystem: string;
  purl?: string;
}

export interface OSVQueryBatch {
  queries: Array<{
    package: OSVPackage;
    version?: string;
  }>;
}

export interface OSVSeverity {
  type: string;
  score: string;
}

export interface OSVAffected {
  package: OSVPackage;
  ranges?: Array<{
    type: string;
    events: Array<{
      introduced?: string;
      fixed?: string;
    }>;
  }>;
  versions?: string[];
  database_specific?: {
    severity?: string;
  };
  ecosystem_specific?: Record<string, unknown>;
}

export interface OSVVulnerability {
  id: string;
  summary?: string;
  details?: string;
  aliases?: string[];
  modified?: string;
  published?: string;
  database_specific?: {
    severity?: string;
    cwe_ids?: string[];
  };
  severity?: OSVSeverity[];
  affected?: OSVAffected[];
  references?: Array<{
    type: string;
    url: string;
  }>;
}

export interface OSVQueryResponse {
  vulns?: OSVVulnerability[];
}

export interface OSVBatchResponse {
  results: OSVQueryResponse[];
}

/** Map CVSS score to severity (9.0+ CRITICAL, 7.0+ HIGH, 4.0+ MEDIUM, >0 LOW) */
function mapCVSSToSeverity(score: number): StandardSeverity {
  if (score >= 9.0) return 'CRITICAL';
  if (score >= 7.0) return 'HIGH';
  if (score >= 4.0) return 'MEDIUM';
  if (score > 0.0) return 'LOW';
  return 'UNKNOWN';
}

/** Extract severity from vulnerability (CVSS > database_specific > affected) */
export function extractSeverity(vuln: OSVVulnerability): StandardSeverity {
  // Try CVSS scores first
  if (vuln.severity?.length) {
    for (const sev of vuln.severity) {
      if (sev.type === 'CVSS_V3' || sev.type === 'CVSS_V2') {
        const score = parseFloat(sev.score);
        if (!isNaN(score)) return mapCVSSToSeverity(score);
      }
    }
  }

  // Try database-specific severity
  const dbSev = vuln.database_specific?.severity?.toUpperCase();
  if (dbSev && ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'].includes(dbSev)) {
    return dbSev as StandardSeverity;
  }

  // Try affected packages severity
  for (const affected of vuln.affected || []) {
    const affSev = affected.database_specific?.severity?.toUpperCase();
    if (affSev && ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'].includes(affSev)) {
      return affSev as StandardSeverity;
    }
  }

  return 'UNKNOWN';
}

/** Extract first fixed version from affected ranges */
export function extractFixedVersion(vuln: OSVVulnerability): string | undefined {
  for (const affected of vuln.affected || []) {
    for (const range of affected.ranges || []) {
      for (const event of range.events || []) {
        if (event.fixed) return event.fixed;
      }
    }
  }
  return undefined;
}

/**
 * Fetch full vulnerability details by ID
 * The batch API only returns ID + modified timestamp, so we need to fetch full details
 */
async function fetchVulnerabilityDetails(
  vulnId: string,
  logger: Logger,
): Promise<OSVVulnerability | null> {
  const OSV_VULN_URL = `https://api.osv.dev/v1/vulns/${vulnId}`;
  const maxRetries = 3;

  for (let retryCount = 0; retryCount <= maxRetries; retryCount++) {
    try {
      await osvRateLimiter.acquire();

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 30000); // 30s timeout

      try {
        const response = await fetch(OSV_VULN_URL, {
          method: 'GET',
          headers: { 'Content-Type': 'application/json' },
          signal: controller.signal,
        });

        if (!response.ok) {
          if (response.status === 429 && retryCount < maxRetries) {
            const retryAfter = response.headers.get('Retry-After');
            const waitTime = retryAfter ? parseInt(retryAfter, 10) * 1000 : 2000;
            logger.warn(
              { vulnId, waitTime, retryCount: retryCount + 1 },
              'Rate limited on vuln fetch',
            );
            await new Promise((resolve) => setTimeout(resolve, waitTime));
            continue;
          }

          logger.warn({ vulnId, status: response.status }, 'Failed to fetch vulnerability details');
          return null;
        }

        const data = (await response.json()) as OSVVulnerability;
        return data;
      } finally {
        clearTimeout(timeoutId);
      }
    } catch (error) {
      logger.warn(
        { vulnId, error: extractErrorMessage(error) },
        'Error fetching vulnerability details',
      );
      if (retryCount < maxRetries) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        continue;
      }
      return null;
    }
  }

  return null;
}

/** Query OSV API in batches (max 1000 packages/batch) */
export async function queryOSVBatch(
  packages: ExtractedPackage[],
  logger: Logger,
): Promise<Map<string, OSVVulnerability[]>> {
  const OSV_API_URL = 'https://api.osv.dev/v1/querybatch';
  const vulnerabilityMap = new Map<string, OSVVulnerability[]>();

  if (packages.length === 0) return vulnerabilityMap;

  const batchSize = 1000;
  const maxRetries = 3;

  // Step 1: Batch query to get vulnerability IDs
  const vulnIdsByPackage = new Map<string, string[]>();

  for (let i = 0; i < packages.length; i += batchSize) {
    const batch = packages.slice(i, i + batchSize);
    const query: OSVQueryBatch = {
      queries: batch.map((pkg) => ({
        package: { name: pkg.name, ecosystem: pkg.ecosystem },
        version: pkg.version,
      })),
    };

    // Track retries per batch to prevent retry count reset issues
    let retryCount = 0;
    let batchSuccess = false;

    // Retry loop for this specific batch
    while (retryCount <= maxRetries && !batchSuccess) {
      try {
        await osvRateLimiter.acquire();

        logger.debug(
          { queryCount: batch.length, batchNumber: Math.floor(i / batchSize) + 1, retryCount },
          'Querying OSV API for vulnerability IDs',
        );

        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 60000); // 60s timeout

        try {
          const response = await fetch(OSV_API_URL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(query),
            signal: controller.signal,
          });

          if (!response.ok) {
            logger.warn({ status: response.status }, 'OSV API returned error status');

            if (response.status === 429 && retryCount < maxRetries) {
              const retryAfter = response.headers.get('Retry-After');
              const waitTime = retryAfter ? parseInt(retryAfter, 10) * 1000 : 5000;
              logger.warn(
                { waitTime, retryCount: retryCount + 1, maxRetries },
                'Rate limited, retrying batch',
              );
              await new Promise((resolve) => setTimeout(resolve, waitTime));
              retryCount++;
              continue; // Retry this batch
            }

            // Non-429 error or retries exhausted - skip this batch
            logger.warn({ status: response.status }, 'Skipping batch due to error');
            break;
          }

          const data = (await response.json()) as OSVBatchResponse;

          // Collect vulnerability IDs (batch API only returns ID + modified timestamp)
          for (let j = 0; j < data.results.length; j++) {
            const result = data.results[j];
            if (result?.vulns?.length) {
              const pkg = batch[j];
              if (pkg) {
                const pkgKey = `${pkg.name}@${pkg.version}`;
                const vulnIds = result.vulns.map((v) => v.id);
                vulnIdsByPackage.set(pkgKey, vulnIds);
              }
            }
          }

          // Mark batch as successful
          batchSuccess = true;
        } finally {
          clearTimeout(timeoutId);
        }
      } catch (error) {
        logger.error(
          { error: extractErrorMessage(error), retryCount, maxRetries },
          'Failed to query OSV API',
        );

        // Retry on exceptions (network errors, timeouts, etc.)
        if (retryCount < maxRetries) {
          retryCount++;
          await new Promise((resolve) => setTimeout(resolve, 2000)); // Wait 2s before retry
          continue;
        }

        // Max retries exceeded - skip this batch
        logger.warn({ error: extractErrorMessage(error) }, 'Skipping batch after max retries');
        break;
      }
    }
  }

  // Step 2: Fetch full vulnerability details for each ID
  const uniqueVulnIds = new Set<string>();
  for (const vulnIds of vulnIdsByPackage.values()) {
    vulnIds.forEach((id) => uniqueVulnIds.add(id));
  }

  logger.debug(
    { totalVulnIds: uniqueVulnIds.size, totalPackages: vulnIdsByPackage.size },
    'Fetching full vulnerability details',
  );

  const vulnDetailsCache = new Map<string, OSVVulnerability>();
  for (const vulnId of uniqueVulnIds) {
    const details = await fetchVulnerabilityDetails(vulnId, logger);
    if (details) {
      vulnDetailsCache.set(vulnId, details);
    }
  }

  // Step 3: Map vulnerability details back to packages
  for (const [pkgKey, vulnIds] of vulnIdsByPackage.entries()) {
    const vulns: OSVVulnerability[] = [];
    for (const vulnId of vulnIds) {
      const details = vulnDetailsCache.get(vulnId);
      if (details) {
        vulns.push(details);
      }
    }
    if (vulns.length > 0) {
      vulnerabilityMap.set(pkgKey, vulns);
    }
  }

  logger.debug(
    { totalVulnerabilities: vulnDetailsCache.size, packagesWithVulns: vulnerabilityMap.size },
    'Vulnerability details fetched successfully',
  );

  return vulnerabilityMap;
}
