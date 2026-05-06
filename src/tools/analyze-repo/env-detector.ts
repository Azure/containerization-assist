/**
 * Environment variable detection from repository files.
 *
 * Pure detection module — scans .env files, docker-compose.yml,
 * Spring config files, and infers framework-specific env vars.
 *
 * TODO: Extract into a separate detection task within analyze-repo
 * so env detection runs as an independent step (see PR #630 discussion).
 */

import { z } from 'zod';
import yaml from 'js-yaml';

export const detectedEnvVarSchema = z.object({
  name: z.string().describe('Environment variable name'),
  classification: z
    .enum(['secret', 'database', 'config'])
    .describe('Classification based on naming patterns'),
  source: z
    .string()
    .describe(
      'Where this env var was detected (e.g., ".env.example", "docker-compose.yml", "framework-inferred")',
    ),
  required: z.boolean().describe('Whether this variable appears to be required (no default value)'),
  defaultValue: z.string().optional().describe('Default value if present'),
});

export type DetectedEnvVar = z.infer<typeof detectedEnvVarSchema>;

// Pre-compiled classification regexes
const CONNECTION_STRING_RE = /(?:DATABASE_URL|CONNECTION_STRING)$/;
const DSN_RE = /(?:^|_)DSN$/;
const SECRET_KEYWORDS_RE =
  /(?:PASSWORD|PASSWD|TOKEN|SECRET|CREDENTIAL|API_KEY|PRIVATE_KEY|ACCESS_KEY)/;
const AUTH_ANCHORED_RE = /(?:^|_)AUTH(?:_|$)/;
const DB_PREFIX_RE = /^(?:DB_|REDIS_|MONGO_|MYSQL_|PG_|POSTGRES_|MSSQL_|DATABASE_)/;

/**
 * Classify an environment variable name as secret, database, or config.
 *
 * - SECRET: contains PASSWORD, TOKEN, KEY, SECRET, CREDENTIAL, or is a DATABASE_URL/DSN/URI
 * - DATABASE: starts with DB_, REDIS_, MONGO_, MYSQL_, PG_, POSTGRES_, or MSSQL_
 * - CONFIG: everything else
 */
export function classifyEnvVar(name: string): 'secret' | 'database' | 'config' {
  const upper = name.toUpperCase();

  if (
    CONNECTION_STRING_RE.test(upper) ||
    upper === 'REDIS_URL' ||
    upper === 'MONGODB_URI' ||
    upper === 'MONGO_URI'
  ) {
    return 'secret';
  }

  // DSN (e.g., SENTRY_DSN) — anchored to avoid matching unrelated vars
  if (DSN_RE.test(upper)) {
    return 'secret';
  }

  // AUTH is anchored to avoid matching AUTHOR, OAUTH_PROVIDER, etc.
  if (SECRET_KEYWORDS_RE.test(upper) || AUTH_ANCHORED_RE.test(upper)) {
    return 'secret';
  }

  if (DB_PREFIX_RE.test(upper)) {
    return 'database';
  }

  return 'config';
}

/**
 * Partition env vars into config, database, and secret name lists in a single pass.
 * Database vars (DB_HOST, DB_PORT, etc.) are non-secret connection details and
 * belong in ConfigMaps, not Secrets.
 */
export function partitionEnvVarNames(vars: DetectedEnvVar[]): {
  configNames: string[];
  databaseNames: string[];
  secretNames: string[];
} {
  const configNames: string[] = [];
  const databaseNames: string[] = [];
  const secretNames: string[] = [];
  for (const v of vars) {
    if (v.classification === 'secret') {
      secretNames.push(v.name);
    } else if (v.classification === 'database') {
      databaseNames.push(v.name);
    } else {
      configNames.push(v.name);
    }
  }
  return { configNames, databaseNames, secretNames };
}

const ENV_LINE_RE = /^([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?$/;

/**
 * Parse KEY=value lines from .env-style files.
 * Skips comments and blank lines.
 * A var is required if it has no default (no value after =, or no = at all).
 */
export function detectEnvVarsFromEnvFile(content: string, source: string): DetectedEnvVar[] {
  const vars: DetectedEnvVar[] = [];

  for (const line of content.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;

    const match = trimmed.match(ENV_LINE_RE);
    if (!match) continue;

    const name = match[1] ?? '';
    const rawValue = match[2];
    const hasDefault = rawValue !== undefined && rawValue !== '';

    // Strip surrounding quotes from value
    let defaultValue: string | undefined;
    if (hasDefault) {
      defaultValue = rawValue.replace(/^(['"])(.*)\1$/, '$2');
    }

    const classification = classifyEnvVar(name);
    vars.push({
      name,
      classification,
      source,
      required: !hasDefault,
      // Redact default values for secrets to avoid surfacing credentials
      ...(defaultValue !== undefined && classification !== 'secret' && { defaultValue }),
    });
  }

  return vars;
}

/**
 * Parse environment variables from docker-compose.yml content.
 * Handles both list format (`- VAR=val`) and map format (`VAR: val`).
 */
export function detectEnvVarsFromDockerCompose(
  content: string,
  source = 'docker-compose.yml',
): { vars: DetectedEnvVar[]; warning?: string } {
  const vars: DetectedEnvVar[] = [];

  let parsed: Record<string, unknown>;
  try {
    parsed = yaml.load(content) as Record<string, unknown>;
  } catch (e) {
    return {
      vars,
      warning: `Failed to parse ${source}: ${e instanceof Error ? e.message : String(e)}`,
    };
  }

  if (!parsed || typeof parsed !== 'object') return { vars };

  const services = (parsed.services ?? parsed) as Record<string, unknown>;
  if (!services || typeof services !== 'object') return { vars };

  for (const serviceValue of Object.values(services)) {
    const service = serviceValue as Record<string, unknown>;
    if (!service || typeof service !== 'object') continue;

    const env = service.environment;
    if (!env) continue;

    if (Array.isArray(env)) {
      // List format: - VAR=val or - VAR
      for (const entry of env) {
        if (typeof entry !== 'string') continue;
        const eqIndex = entry.indexOf('=');
        if (eqIndex === -1) {
          vars.push({ name: entry, classification: classifyEnvVar(entry), source, required: true });
        } else {
          const name = entry.substring(0, eqIndex);
          const value = entry.substring(eqIndex + 1);
          const classification = classifyEnvVar(name);
          vars.push({
            name,
            classification,
            source,
            required: !value,
            // Redact default values for secrets to avoid surfacing credentials
            ...(value && classification !== 'secret' && { defaultValue: value }),
          });
        }
      }
    } else if (typeof env === 'object') {
      // Map format: VAR: val
      for (const [name, value] of Object.entries(env as Record<string, unknown>)) {
        const strValue = value != null ? String(value) : undefined;
        const classification = classifyEnvVar(name);
        vars.push({
          name,
          classification,
          source,
          required: strValue === undefined || strValue === '',
          // Redact default values for secrets to avoid surfacing credentials
          ...(strValue && classification !== 'secret' && { defaultValue: strValue }),
        });
      }
    }
  }

  return { vars };
}

/**
 * Extract ${VAR_NAME:default} placeholders from Spring application.properties/yml content.
 * Only captures env-var-shaped names (uppercase with underscores); rejects dotted
 * Spring property references like ${spring.profiles.active}.
 */
export function detectEnvVarsFromSpringConfig(
  content: string,
  source = 'application.properties',
): DetectedEnvVar[] {
  const vars: DetectedEnvVar[] = [];
  const seen = new Set<string>();

  // Match ${VAR_NAME} or ${VAR_NAME:defaultValue} — restrict to env-var-like names
  const pattern = /\$\{([A-Za-z_][A-Za-z0-9_]*?)(?::([^}]*))?\}/g;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(content)) !== null) {
    const name = match[1] ?? '';
    if (seen.has(name)) continue;
    seen.add(name);

    const defaultValue = match[2];
    vars.push({
      name,
      classification: classifyEnvVar(name),
      source,
      required: defaultValue === undefined,
      ...(defaultValue !== undefined && { defaultValue }),
    });
  }

  return vars;
}

/**
 * Infer common framework environment variables from language/framework.
 */
export function inferFrameworkEnvVars(language: string, framework?: string): DetectedEnvVar[] {
  const source = 'framework-inferred';
  const vars: DetectedEnvVar[] = [];

  const add = (name: string, defaultValue?: string): void => {
    vars.push({
      name,
      classification: classifyEnvVar(name),
      source,
      required: false,
      ...(defaultValue !== undefined && { defaultValue }),
    });
  };

  switch (language) {
    case 'javascript':
    case 'typescript':
      add('NODE_ENV', 'production');
      add('PORT', '3000');
      break;
    case 'java':
      if (framework === 'spring-boot' || framework === 'spring') {
        add('SPRING_PROFILES_ACTIVE', 'default');
        add('SERVER_PORT', '8080');
      }
      add('JAVA_OPTS');
      break;
    case 'dotnet':
      add('ASPNETCORE_ENVIRONMENT', 'Production');
      add('ASPNETCORE_URLS', 'http://+:8080');
      break;
    case 'python':
      if (framework === 'flask') {
        add('FLASK_APP');
        add('FLASK_ENV', 'production');
      } else if (framework === 'django') {
        add('DJANGO_SETTINGS_MODULE');
        add('DJANGO_SECRET_KEY');
      } else if (framework === 'fastapi') {
        add('UVICORN_HOST', '0.0.0.0');
        add('UVICORN_PORT', '8000');
      }
      add('PYTHONUNBUFFERED', '1');
      break;
    case 'go':
      add('PORT', '8080');
      break;
    case 'rust':
      add('RUST_LOG', 'info');
      break;
  }

  return vars;
}

/**
 * Deduplicate env vars by name.
 * Prefers explicit sources over 'framework-inferred'.
 * Keeps the first default value encountered.
 */
export function deduplicateEnvVars(vars: DetectedEnvVar[]): DetectedEnvVar[] {
  const map = new Map<string, DetectedEnvVar>();

  for (const v of vars) {
    const existing = map.get(v.name);
    if (!existing) {
      map.set(v.name, v);
    } else if (existing.source === 'framework-inferred' && v.source !== 'framework-inferred') {
      // Prefer explicit source over inferred
      map.set(v.name, v);
    }
    // Otherwise keep first occurrence
  }

  return Array.from(map.values());
}
