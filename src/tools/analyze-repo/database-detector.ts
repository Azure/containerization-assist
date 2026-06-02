/**
 * Database detection from dependency lists.
 *
 * Pure mapping function — takes an array of dependency names
 * and returns standardized database types with the triggering deps.
 */

import { z } from 'zod';

export interface DetectedDatabase {
  dbType: string;
  dependencies: string[];
}

/** Zod schema for DetectedDatabase — shared across tool schemas */
export const detectedDatabaseSchema = z.object({
  dbType: z.string().describe('Standardized database type (e.g., postgres, mysql, mongodb, redis)'),
  dependencies: z.array(z.string()).describe('Dependency names that triggered detection'),
});

/**
 * Database types that represent managed/networked services suitable for
 * workload identity authentication. Excludes embedded databases like SQLite.
 */
export const MANAGED_DB_TYPES = new Set([
  'postgres',
  'mysql',
  'mongodb',
  'redis',
  'mssql',
  'cosmosdb',
]);

/**
 * Exact-match mapping from dependency name (lowercased) to standardized dbType.
 * Covers Node, Python, Java, .NET, Go, and Rust ecosystems.
 *
 * Note: Go full module paths and .NET EF provider names are handled by
 * DEP_PATTERNS below (which also matches versioned variants like pgx/v5).
 * Only short/unambiguous names belong here.
 */
const DEP_TO_DB: Record<string, string> = {
  // PostgreSQL
  pg: 'postgres',
  'pg-pool': 'postgres',
  'pg-native': 'postgres',
  postgres: 'postgres',
  postgresql: 'postgres',
  psycopg2: 'postgres',
  'psycopg2-binary': 'postgres',
  asyncpg: 'postgres',
  npgsql: 'postgres',
  'tokio-postgres': 'postgres',

  // MySQL
  mysql: 'mysql',
  mysql2: 'mysql',
  pymysql: 'mysql',
  mysqlclient: 'mysql',
  'mysql-connector-java': 'mysql',
  'mysql-connector-python': 'mysql',

  // MongoDB
  mongodb: 'mongodb',
  mongoose: 'mongodb',
  pymongo: 'mongodb',
  motor: 'mongodb',
  'mongodb.driver': 'mongodb',
  'mongo-go-driver': 'mongodb',

  // Redis
  redis: 'redis',
  ioredis: 'redis',
  'redis-py': 'redis',
  'go-redis': 'redis',
  'stackexchange.redis': 'redis',

  // MSSQL / SQL Server
  mssql: 'mssql',
  tedious: 'mssql',
  pymssql: 'mssql',

  // SQLite
  sqlite3: 'sqlite',
  'better-sqlite3': 'sqlite',
  aiosqlite: 'sqlite',

  // Cosmos DB
  '@azure/cosmos': 'cosmosdb',
  'microsoft.azure.cosmos': 'cosmosdb',

  // Elasticsearch
  '@elastic/elasticsearch': 'elasticsearch',
  elasticsearch: 'elasticsearch',
  'elasticsearch-py': 'elasticsearch',
  'olivere/elastic': 'elasticsearch',
};

/**
 * Pattern-based matching for composite dependency strings.
 * Handles Java `groupId:artifactId` (e.g., "org.postgresql:postgresql",
 * "com.mysql:mysql-connector-j") and Go module paths
 * (e.g., "github.com/jackc/pgx/v5").
 *
 * Each entry: [regex applied to lowercased dep, dbType]
 */
const DEP_PATTERNS: Array<[RegExp, string]> = [
  // PostgreSQL — Java (org.postgresql:*), Go (jackc/pgx variants)
  [/(?:^|:)postgresql(?:$|:)/, 'postgres'],
  [/^org\.postgresql:/, 'postgres'],
  [/jackc\/pgx(?:\/|$)/, 'postgres'],
  [/(?:^|\/)lib\/pq(?:$|\/)/, 'postgres'],

  // MySQL — Java (com.mysql:mysql-connector-j, mysql:mysql-connector-java)
  [/(?:^|:)mysql[_-]connector/, 'mysql'],
  [/^go-sql-driver\/mysql/, 'mysql'],

  // MongoDB — Go full paths
  [/^go\.mongodb\.org\/mongo-driver/, 'mongodb'],

  // Redis — Go full paths
  [/^github\.com\/go-redis\/redis/, 'redis'],
  [/^github\.com\/redis\/go-redis/, 'redis'],

  // MSSQL — Java (com.microsoft.sqlserver:mssql-jdbc)
  [/(?:^|:)mssql-jdbc(?:$|:)/, 'mssql'],
  [/^com\.microsoft\.sqlserver:/, 'mssql'],

  // .NET EF providers (case-insensitive via lowercased input)
  [/entityframeworkcore\.sqlserver/, 'mssql'],
  [/entityframeworkcore\.sqlite/, 'sqlite'],
  [/entityframeworkcore\.mysql/, 'mysql'],
  [/entityframework\.sqlserver/, 'mssql'],
];

/**
 * Strip version specifiers and extras from a dependency string.
 * Handles: "psycopg2==2.9.9", "pymongo>=4", "psycopg2-binary (>=2.9)",
 * "redis[hiredis]>=4.0", "package~=1.0;python_version>='3.8'"
 */
function normalizeDep(dep: string): string {
  return dep
    .replace(/\[.*?\]/g, '') // strip extras like [hiredis]
    .replace(/\(.*?\)/g, '') // strip parenthesized constraints
    .replace(/;.*$/, '') // strip environment markers (e.g., ;python_version>='3.8')
    .replace(/[><=!~]=?.*$/, '') // strip version operators and everything after
    .trim();
}

/**
 * Try to match a dependency string against the exact map (using normalized form),
 * then fall back to pattern matching (using original lowercased form).
 */
function matchDep(normalized: string, original: string): string | undefined {
  // Exact match on normalized name
  const exact = DEP_TO_DB[normalized];
  if (exact) return exact;

  // Pattern match on original lowercased string (preserves Go paths, Java groupId:artifactId)
  for (const [pattern, dbType] of DEP_PATTERNS) {
    if (pattern.test(original)) return dbType;
  }

  return undefined;
}

/**
 * Detect databases from a list of dependency names.
 * Deduplicates by dbType and aggregates triggering dependency names.
 */
export function detectDatabases(deps: string[]): DetectedDatabase[] {
  const grouped = new Map<string, string[]>();

  for (const dep of deps) {
    const lower = dep.toLowerCase();
    const normalized = normalizeDep(lower);
    const dbType = matchDep(normalized, lower);
    if (dbType) {
      const existing = grouped.get(dbType);
      if (existing) {
        existing.push(dep);
      } else {
        grouped.set(dbType, [dep]);
      }
    }
  }

  return Array.from(grouped.entries()).map(([dbType, dependencies]) => ({
    dbType,
    dependencies,
  }));
}
