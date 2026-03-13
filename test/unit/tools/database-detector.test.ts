/**
 * Unit tests for database-detector
 */

import { describe, it, expect } from '@jest/globals';
import { detectDatabases } from '@/tools/analyze-repo/database-detector';

describe('detectDatabases', () => {
  it('should return empty array for no dependencies', () => {
    expect(detectDatabases([])).toEqual([]);
  });

  it('should return empty array when no DB deps are present', () => {
    expect(detectDatabases(['express', 'react', 'lodash'])).toEqual([]);
  });

  it('should detect PostgreSQL from pg', () => {
    const result = detectDatabases(['pg']);
    expect(result).toEqual([{ dbType: 'postgres', dependencies: ['pg'] }]);
  });

  it('should detect MySQL from mysql2', () => {
    const result = detectDatabases(['mysql2']);
    expect(result).toEqual([{ dbType: 'mysql', dependencies: ['mysql2'] }]);
  });

  it('should detect MongoDB from mongoose', () => {
    const result = detectDatabases(['mongoose']);
    expect(result).toEqual([{ dbType: 'mongodb', dependencies: ['mongoose'] }]);
  });

  it('should detect Redis from ioredis', () => {
    const result = detectDatabases(['ioredis']);
    expect(result).toEqual([{ dbType: 'redis', dependencies: ['ioredis'] }]);
  });

  it('should detect MSSQL from tedious', () => {
    const result = detectDatabases(['tedious']);
    expect(result).toEqual([{ dbType: 'mssql', dependencies: ['tedious'] }]);
  });

  it('should detect SQLite from better-sqlite3', () => {
    const result = detectDatabases(['better-sqlite3']);
    expect(result).toEqual([{ dbType: 'sqlite', dependencies: ['better-sqlite3'] }]);
  });

  it('should detect CosmosDB from @azure/cosmos', () => {
    const result = detectDatabases(['@azure/cosmos']);
    expect(result).toEqual([{ dbType: 'cosmosdb', dependencies: ['@azure/cosmos'] }]);
  });

  it('should detect Elasticsearch from @elastic/elasticsearch', () => {
    const result = detectDatabases(['@elastic/elasticsearch']);
    expect(result).toEqual([
      { dbType: 'elasticsearch', dependencies: ['@elastic/elasticsearch'] },
    ]);
  });

  it('should dedup by dbType and aggregate deps', () => {
    const result = detectDatabases(['pg', 'pg-pool']);
    expect(result).toEqual([{ dbType: 'postgres', dependencies: ['pg', 'pg-pool'] }]);
  });

  it('should detect multiple database types', () => {
    const result = detectDatabases(['pg', 'ioredis', 'express']);
    expect(result).toHaveLength(2);
    expect(result.find((d) => d.dbType === 'postgres')).toBeDefined();
    expect(result.find((d) => d.dbType === 'redis')).toBeDefined();
  });

  it('should be case-insensitive', () => {
    const result = detectDatabases(['PG', 'Mongoose']);
    expect(result).toHaveLength(2);
    expect(result.find((d) => d.dbType === 'postgres')).toBeDefined();
    expect(result.find((d) => d.dbType === 'mongodb')).toBeDefined();
  });

  // Python ecosystem
  it('should detect PostgreSQL from psycopg2', () => {
    const result = detectDatabases(['psycopg2']);
    expect(result).toEqual([{ dbType: 'postgres', dependencies: ['psycopg2'] }]);
  });

  it('should detect MongoDB from pymongo', () => {
    const result = detectDatabases(['pymongo']);
    expect(result).toEqual([{ dbType: 'mongodb', dependencies: ['pymongo'] }]);
  });

  it('should detect PostgreSQL from versioned psycopg2 specifier', () => {
    const result = detectDatabases(['psycopg2==2.9.9']);
    expect(result.find((d) => d.dbType === 'postgres')).toBeDefined();
  });

  it('should detect MongoDB from versioned pymongo specifier', () => {
    const result = detectDatabases(['pymongo>=4']);
    expect(result.find((d) => d.dbType === 'mongodb')).toBeDefined();
  });
  it('should detect Redis from dep with extras and version', () => {
    const result = detectDatabases(['redis[hiredis]>=4.0']);
    expect(result.find((d) => d.dbType === 'redis')).toBeDefined();
  });

  it('should detect PostgreSQL from pyproject-style dep with parenthesized constraint', () => {
    const result = detectDatabases(['psycopg2-binary (>=2.9)']);
    expect(result.find((d) => d.dbType === 'postgres')).toBeDefined();
  });

  // .NET ecosystem
  it('should detect Postgres from Npgsql', () => {
    const result = detectDatabases(['Npgsql']);
    expect(result).toEqual([{ dbType: 'postgres', dependencies: ['Npgsql'] }]);
  });

  it('should detect MSSQL from Microsoft.EntityFrameworkCore.SqlServer', () => {
    const result = detectDatabases(['Microsoft.EntityFrameworkCore.SqlServer']);
    expect(result).toEqual([
      { dbType: 'mssql', dependencies: ['Microsoft.EntityFrameworkCore.SqlServer'] },
    ]);
  });

  // Go ecosystem
  it('should detect Postgres from jackc/pgx', () => {
    const result = detectDatabases(['jackc/pgx']);
    expect(result).toEqual([{ dbType: 'postgres', dependencies: ['jackc/pgx'] }]);
  });

  // Rust ecosystem
  it('should detect Postgres from tokio-postgres', () => {
    const result = detectDatabases(['tokio-postgres']);
    expect(result).toEqual([{ dbType: 'postgres', dependencies: ['tokio-postgres'] }]);
  });

  // Java groupId:artifactId format (from pom.xml parser)
  it('should detect Postgres from org.postgresql:postgresql', () => {
    const result = detectDatabases(['org.postgresql:postgresql']);
    expect(result).toEqual([
      { dbType: 'postgres', dependencies: ['org.postgresql:postgresql'] },
    ]);
  });

  it('should detect MySQL from com.mysql:mysql-connector-j', () => {
    const result = detectDatabases(['com.mysql:mysql-connector-j']);
    expect(result).toEqual([
      { dbType: 'mysql', dependencies: ['com.mysql:mysql-connector-j'] },
    ]);
  });

  it('should detect MSSQL from com.microsoft.sqlserver:mssql-jdbc', () => {
    const result = detectDatabases(['com.microsoft.sqlserver:mssql-jdbc']);
    expect(result).toEqual([
      { dbType: 'mssql', dependencies: ['com.microsoft.sqlserver:mssql-jdbc'] },
    ]);
  });

  // Go full module paths (from go.mod parser)
  it('should detect Postgres from github.com/jackc/pgx/v5', () => {
    const result = detectDatabases(['github.com/jackc/pgx/v5']);
    expect(result).toEqual([
      { dbType: 'postgres', dependencies: ['github.com/jackc/pgx/v5'] },
    ]);
  });

  it('should detect MongoDB from go.mongodb.org/mongo-driver', () => {
    const result = detectDatabases(['go.mongodb.org/mongo-driver']);
    expect(result).toEqual([
      { dbType: 'mongodb', dependencies: ['go.mongodb.org/mongo-driver'] },
    ]);
  });

  it('should detect Redis from github.com/go-redis/redis/v9', () => {
    const result = detectDatabases(['github.com/go-redis/redis/v9']);
    expect(result).toEqual([
      { dbType: 'redis', dependencies: ['github.com/go-redis/redis/v9'] },
    ]);
  });

  // .NET EF pattern matching
  it('should detect MSSQL from EntityFramework.SqlServer (old-style .NET)', () => {
    const result = detectDatabases(['EntityFramework.SqlServer']);
    expect(result).toEqual([
      { dbType: 'mssql', dependencies: ['EntityFramework.SqlServer'] },
    ]);
  });

  // Spring petclinic style: multiple Java DB deps together
  it('should detect both postgres and mysql from spring-petclinic style deps', () => {
    const result = detectDatabases([
      'org.springframework.boot:spring-boot-starter-data-jpa',
      'org.postgresql:postgresql',
      'com.mysql:mysql-connector-j',
      'com.h2database:h2',
    ]);
    expect(result).toHaveLength(2);
    expect(result.find((d) => d.dbType === 'postgres')).toBeDefined();
    expect(result.find((d) => d.dbType === 'mysql')).toBeDefined();
  });
});
