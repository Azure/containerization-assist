/**
 * Policy IO - lean cache + strict validation
 */
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as yaml from 'js-yaml';
import { z } from 'zod';
import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { createLogger } from '@/lib/logger';
import { ERROR_MESSAGES } from '@/lib/error-messages';
import { type Policy, PolicySchema } from './policy-schemas';
import { policyData } from './policy-data';

const log = createLogger().child({ module: 'policy-io' });

// Cache key is the resolved absolute file path (simplified after PR-019 removed environment parameter)
type CacheKey = string;
type CacheVal = { value: Policy; expiresAt: number };
const CACHE = new Map<CacheKey, CacheVal>();

const createCacheKey = (file: string): CacheKey => path.resolve(file);
const now = (): number => Date.now();

function getCached(file: string): Policy | null {
  const cacheEntry = CACHE.get(createCacheKey(file));
  if (!cacheEntry) return null;
  if (cacheEntry.expiresAt <= now()) {
    CACHE.delete(createCacheKey(file));
    return null;
  }
  return cacheEntry.value;
}
function putCached(file: string, policy: Policy, ttlSec: number): void {
  CACHE.set(createCacheKey(file), { value: policy, expiresAt: now() + ttlSec * 1000 });
}

/** Validate policy via Zod and return Result */
export function validatePolicy(p: unknown): Result<Policy> {
  try {
    return Success(PolicySchema.parse(p));
  } catch (e) {
    if (e instanceof z.ZodError) {
      const issues = e.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
      return Failure(ERROR_MESSAGES.POLICY_VALIDATION_FAILED(issues));
    }
    return Failure(ERROR_MESSAGES.POLICY_VALIDATION_FAILED(String(e)));
  }
}

/** Load + cache policy */
export function loadPolicy(file: string): Result<Policy> {
  try {
    const cached = getCached(file);
    if (cached) {
      log.debug({ file }, 'Using cached policy');
      return Success(cached);
    }

    let base: Result<Policy>;

    // Try to load from YAML file if it exists
    if (fs.existsSync(file)) {
      try {
        const content = fs.readFileSync(file, 'utf8');
        const parsed = yaml.load(content);
        base = validatePolicy(parsed);
        if (base.ok) {
          log.debug({ file }, 'Loaded policy from YAML file');
        }
      } catch (yamlError) {
        log.warn(
          { file, error: yamlError },
          'Failed to load YAML file, falling back to TypeScript data',
        );
        base = validatePolicy(policyData);
      }
    } else {
      // Fall back to TypeScript policy data
      log.debug({ file }, 'Policy file not found, using default TypeScript data');
      base = validatePolicy(policyData);
    }

    if (!base.ok) return base;

    // Sort rules by priority descending
    base.value.rules.sort((a, b) => b.priority - a.priority);

    const ttl = base.value.cache?.ttl ?? 300;
    putCached(file, base.value, ttl);
    log.debug({ file, rulesCount: base.value.rules.length }, 'Policy loaded and cached successfully');
    return base;
  } catch (err) {
    return Failure(ERROR_MESSAGES.POLICY_LOAD_FAILED(extractErrorMessage(err)));
  }
}

/** Create a tiny default when none exists (unchanged behavior semantics) */
export function createDefaultPolicy(): Policy {
  return {
    version: '2.0',
    metadata: {
      created: new Date().toISOString(),
      description: 'Default containerization policy',
    },
    defaults: { enforcement: 'advisory', cache_ttl: 300 },
    rules: [
      {
        id: 'security-scanning',
        category: 'security',
        priority: 100,
        conditions: [{ kind: 'regex', pattern: 'FROM .*(alpine|distroless)' }],
        actions: { enforce_scan: true, block_on_critical: true },
      },
      {
        id: 'base-image-validation',
        category: 'quality',
        priority: 90,
        conditions: [{ kind: 'function', name: 'hasPattern', args: ['FROM.*:latest'] }],
        actions: { suggest_pinned_version: true },
      },
    ],
    cache: { enabled: true, ttl: 300 },
  };
}
