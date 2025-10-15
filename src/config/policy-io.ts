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

type CacheKey = `${string}:${string}`;
type CacheVal = { value: Policy; expiresAt: number };
const CACHE = new Map<CacheKey, CacheVal>();

const k = (file: string, env?: string): CacheKey => `${path.resolve(file)}:${env ?? 'default'}`;
const now = (): number => Date.now();

function getCached(file: string, env?: string): Policy | null {
  const v = CACHE.get(k(file, env));
  if (!v) return null;
  if (v.expiresAt <= now()) {
    CACHE.delete(k(file, env));
    return null;
  }
  return v.value;
}
function putCached(file: string, env: string | undefined, policy: Policy, ttlSec: number): void {
  CACHE.set(k(file, env), { value: policy, expiresAt: now() + ttlSec * 1000 });
}

/** Validate policy via Zod and return Result */
export function validatePolicy(p: unknown): Result<Policy> {
  try {
    return Success(PolicySchema.parse(p) as Policy);
  } catch (e) {
    if (e instanceof z.ZodError) {
      const issues = e.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
      return Failure(ERROR_MESSAGES.POLICY_VALIDATION_FAILED(issues));
    }
    return Failure(ERROR_MESSAGES.POLICY_VALIDATION_FAILED(String(e)));
  }
}

/** Resolve environment overrides and keep rules sorted by priority desc */
export function resolveEnvironment(policy: Policy, env: string): Policy {
  const cfg = policy.environments?.[env];
  const out: Policy = JSON.parse(JSON.stringify(policy));

  if (cfg?.defaults) out.defaults = { ...out.defaults, ...cfg.defaults };
  if (cfg?.overrides?.length) {
    for (const ov of cfg.overrides) {
      const idx = out.rules.findIndex((r) => r.id === ov.rule_id);
      if (idx === -1) continue;
      if (ov.enabled === false) {
        out.rules.splice(idx, 1);
      } else {
        const r = out.rules[idx];
        if (r) {
          if (ov.priority !== undefined) r.priority = ov.priority;
          if (ov.actions) r.actions = { ...r.actions, ...ov.actions };
        }
      }
    }
  }
  out.rules.sort((a, b) => b.priority - a.priority);
  return out;
}

/** Load + cache policy; optional env application */
export function loadPolicy(file: string, env?: string): Result<Policy> {
  try {
    const cached = getCached(file, env);
    if (cached) {
      log.debug({ file, env }, 'Using cached policy');
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

    const resolved = env ? resolveEnvironment(base.value, env) : base.value;

    // Sort rules by priority descending
    resolved.rules.sort((a, b) => b.priority - a.priority);

    const final = validatePolicy(resolved);
    if (!final.ok) return final;

    const ttl = final.value.cache?.ttl ?? 300;
    putCached(file, env, final.value, ttl);
    return final;
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
