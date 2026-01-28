// OSV API client with token bucket rate limiting (10 req/sec)

import type { Logger } from 'pino';

import { extractErrorMessage } from '@/lib/errors';
import type { StandardSeverity } from '../scanner-common';
import type { ExtractedPackage } from './maven/types';
import { parseCVSSVector } from './cvss-parser';

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

    this.processQueue();
  }

  private processQueue(): void {
    while (this.queue.length > 0 && this.tokens >= 1) {
      const resolve = this.queue.shift();
      this.tokens -= 1;
      if (resolve) resolve();
    }

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
    while (this.queue.length > 0) {
      const resolve = this.queue.shift();
      if (resolve) resolve();
    }
  }
}

export const osvRateLimiter = new RateLimiter(10, 10);

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

export function extractSeverity(vuln: OSVVulnerability): StandardSeverity {
  if (vuln.severity?.length) {
    for (const sev of vuln.severity) {
      try {
        if (sev.type === 'CVSS_V3' || sev.type === 'CVSS_V2') {
          const result = parseCVSSVector(sev.score);
          return result.severity as StandardSeverity;
        }
      } catch {
        // If CVSS parsing fails, continue to next severity entry
        continue;
      }
    }
  }

  return 'UNKNOWN';
}

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

async function fetchVulnerabilityDetails(
  vulnId: string,
  _logger: Logger,
): Promise<OSVVulnerability | null> {
  // Validate and sanitize vulnId to prevent URL injection
  if (!vulnId || typeof vulnId !== 'string') {
    return null;
  }

  // Trim whitespace and validate format (OSV IDs typically follow patterns like CVE-*, GHSA-*, etc.)
  const sanitizedVulnId = vulnId.trim();

  // Reject IDs with potentially dangerous characters
  if (!/^[A-Za-z0-9._-]+$/.test(sanitizedVulnId)) {
    return null;
  }

  const maxRetries = 3;

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      await osvRateLimiter.acquire();

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 30000);

      try {
        // Use encodeURIComponent for additional safety
        const encodedVulnId = encodeURIComponent(sanitizedVulnId);
        const response = await fetch(`https://api.osv.dev/v1/vulns/${encodedVulnId}`, {
          method: 'GET',
          headers: { 'Content-Type': 'application/json' },
          signal: controller.signal,
        });

        if (!response.ok) {
          if (response.status === 429 && attempt < maxRetries) {
            const retryAfter = response.headers.get('Retry-After');
            const waitTime = retryAfter ? parseInt(retryAfter, 10) * 1000 : 2000;
            await new Promise((resolve) => setTimeout(resolve, waitTime));
            continue;
          }
          return null;
        }

        return (await response.json()) as OSVVulnerability;
      } finally {
        clearTimeout(timeoutId);
      }
    } catch {
      if (attempt < maxRetries) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        continue;
      }
      return null;
    }
  }

  return null;
}

async function fetchBatchVulnerabilityIds(
  packages: ExtractedPackage[],
  logger: Logger,
): Promise<Map<string, string[]>> {
  const vulnIdsByPackage = new Map<string, string[]>();
  const batchSize = 1000;
  const maxRetries = 3;

  for (let i = 0; i < packages.length; i += batchSize) {
    const batch = packages.slice(i, i + batchSize);
    const query: OSVQueryBatch = {
      queries: batch.map((pkg) => ({
        package: { name: pkg.name, ecosystem: pkg.ecosystem },
        version: pkg.version,
      })),
    };

    logger.debug(
      {
        batchIndex: i / batchSize,
        batchSize: batch.length,
        sampleQueries: query.queries.slice(0, 5),
      },
      'Querying OSV batch',
    );

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        await osvRateLimiter.acquire();

        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 60000);

        try {
          const response = await fetch('https://api.osv.dev/v1/querybatch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(query),
            signal: controller.signal,
          });

          if (!response.ok) {
            if (response.status === 429 && attempt < maxRetries) {
              const retryAfter = response.headers.get('Retry-After');
              const waitTime = retryAfter ? parseInt(retryAfter, 10) * 1000 : 5000;
              await new Promise((resolve) => setTimeout(resolve, waitTime));
              continue;
            }
            break;
          }

          const data = (await response.json()) as OSVBatchResponse;

          for (let j = 0; j < data.results.length; j++) {
            const result = data.results[j];
            if (result?.vulns?.length) {
              const pkg = batch[j];
              if (pkg) {
                const pkgKey = `${pkg.name}@${pkg.version}`;
                vulnIdsByPackage.set(
                  pkgKey,
                  result.vulns.map((v) => v.id),
                );
              }
            }
          }

          break;
        } finally {
          clearTimeout(timeoutId);
        }
      } catch (error) {
        if (attempt < maxRetries) {
          await new Promise((resolve) => setTimeout(resolve, 2000));
          continue;
        }
        logger.warn({ error: extractErrorMessage(error) }, 'Failed to fetch batch');
        break;
      }
    }
  }

  return vulnIdsByPackage;
}

export async function queryOSVBatch(
  packages: ExtractedPackage[],
  logger: Logger,
): Promise<Map<string, OSVVulnerability[]>> {
  if (packages.length === 0) return new Map();

  const vulnIdsByPackage = await fetchBatchVulnerabilityIds(packages, logger);

  const uniqueVulnIds = new Set<string>();
  for (const vulnIds of vulnIdsByPackage.values()) {
    vulnIds.forEach((id) => uniqueVulnIds.add(id));
  }

  const vulnDetailsCache = new Map<string, OSVVulnerability>();
  for (const vulnId of uniqueVulnIds) {
    const details = await fetchVulnerabilityDetails(vulnId, logger);
    if (details) vulnDetailsCache.set(vulnId, details);
  }

  const vulnerabilityMap = new Map<string, OSVVulnerability[]>();
  for (const [pkgKey, vulnIds] of vulnIdsByPackage.entries()) {
    const vulns = vulnIds
      .map((id) => vulnDetailsCache.get(id))
      .filter(Boolean) as OSVVulnerability[];
    if (vulns.length > 0) vulnerabilityMap.set(pkgKey, vulns);
  }

  return vulnerabilityMap;
}
