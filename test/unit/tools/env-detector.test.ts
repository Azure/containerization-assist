/**
 * Unit tests for env-detector
 */

import { describe, it, expect } from '@jest/globals';
import {
  classifyEnvVar,
  detectEnvVarsFromEnvFile,
  detectEnvVarsFromDockerCompose,
  detectEnvVarsFromSpringConfig,
  inferFrameworkEnvVars,
  deduplicateEnvVars,
  type DetectedEnvVar,
} from '@/tools/analyze-repo/env-detector';

describe('classifyEnvVar', () => {
  it('should classify password-related vars as secret', () => {
    expect(classifyEnvVar('DB_PASSWORD')).toBe('secret');
    expect(classifyEnvVar('ADMIN_PASSWD')).toBe('secret');
  });

  it('should classify token/key vars as secret', () => {
    expect(classifyEnvVar('API_TOKEN')).toBe('secret');
    expect(classifyEnvVar('API_KEY')).toBe('secret');
    expect(classifyEnvVar('SECRET_KEY')).toBe('secret');
    expect(classifyEnvVar('PRIVATE_KEY')).toBe('secret');
    expect(classifyEnvVar('ACCESS_KEY')).toBe('secret');
  });

  it('should classify credential vars as secret', () => {
    expect(classifyEnvVar('AWS_CREDENTIAL')).toBe('secret');
    expect(classifyEnvVar('JWT_SECRET')).toBe('secret');
  });

  it('should classify DATABASE_URL as secret (contains credentials)', () => {
    expect(classifyEnvVar('DATABASE_URL')).toBe('secret');
  });

  it('should classify REDIS_URL and MONGODB_URI as secret', () => {
    expect(classifyEnvVar('REDIS_URL')).toBe('secret');
    expect(classifyEnvVar('MONGODB_URI')).toBe('secret');
    expect(classifyEnvVar('MONGO_URI')).toBe('secret');
  });

  it('should classify DSN as secret', () => {
    expect(classifyEnvVar('SENTRY_DSN')).toBe('secret');
  });

  it('should classify DB_ prefixed vars as database', () => {
    expect(classifyEnvVar('DB_HOST')).toBe('database');
    expect(classifyEnvVar('DB_PORT')).toBe('database');
    expect(classifyEnvVar('DB_NAME')).toBe('database');
  });

  it('should classify REDIS_ prefixed vars as database', () => {
    expect(classifyEnvVar('REDIS_HOST')).toBe('database');
    expect(classifyEnvVar('REDIS_PORT')).toBe('database');
  });

  it('should classify MONGO_ prefixed vars as database', () => {
    expect(classifyEnvVar('MONGO_HOST')).toBe('database');
  });

  it('should classify MYSQL_ prefixed vars as database', () => {
    expect(classifyEnvVar('MYSQL_HOST')).toBe('database');
  });

  it('should classify PG_ and POSTGRES_ prefixed vars as database', () => {
    expect(classifyEnvVar('PG_HOST')).toBe('database');
    expect(classifyEnvVar('POSTGRES_DB')).toBe('database');
  });

  it('should classify AUTH-related vars as secret (word-boundary anchored)', () => {
    expect(classifyEnvVar('BASIC_AUTH')).toBe('secret');
    expect(classifyEnvVar('AUTH_TOKEN')).toBe('secret');
    expect(classifyEnvVar('AUTH')).toBe('secret');
  });

  it('should NOT classify AUTH-substring vars as secret', () => {
    expect(classifyEnvVar('AUTHOR')).toBe('config');
    expect(classifyEnvVar('AUTHORIZATION_TYPE')).toBe('config');
    expect(classifyEnvVar('OAUTH_CLIENT_ID')).toBe('config');
    expect(classifyEnvVar('OAUTH_PROVIDER')).toBe('config');
  });

  it('should NOT classify generic URI vars as secret', () => {
    expect(classifyEnvVar('REDIRECT_URI')).toBe('config');
    expect(classifyEnvVar('BASE_URI')).toBe('config');
    expect(classifyEnvVar('API_URI')).toBe('config');
  });

  it('should classify CONNECTION_STRING as secret', () => {
    expect(classifyEnvVar('DB_CONNECTION_STRING')).toBe('secret');
  });

  it('should classify everything else as config', () => {
    expect(classifyEnvVar('NODE_ENV')).toBe('config');
    expect(classifyEnvVar('PORT')).toBe('config');
    expect(classifyEnvVar('LOG_LEVEL')).toBe('config');
    expect(classifyEnvVar('APP_NAME')).toBe('config');
  });

  it('should be case-insensitive', () => {
    expect(classifyEnvVar('db_password')).toBe('secret');
    expect(classifyEnvVar('api_token')).toBe('secret');
    expect(classifyEnvVar('db_host')).toBe('database');
  });
});

describe('detectEnvVarsFromEnvFile', () => {
  it('should return empty array for empty content', () => {
    expect(detectEnvVarsFromEnvFile('', '.env.example')).toEqual([]);
  });

  it('should skip comments and blank lines', () => {
    const content = `# This is a comment

# Another comment
`;
    expect(detectEnvVarsFromEnvFile(content, '.env.example')).toEqual([]);
  });

  it('should parse KEY=value pairs', () => {
    const content = 'NODE_ENV=production\nPORT=3000';
    const result = detectEnvVarsFromEnvFile(content, '.env.example');
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({
      name: 'NODE_ENV',
      classification: 'config',
      source: '.env.example',
      required: false,
      defaultValue: 'production',
    });
    expect(result[1]).toEqual({
      name: 'PORT',
      classification: 'config',
      source: '.env.example',
      required: false,
      defaultValue: '3000',
    });
  });

  it('should mark vars without values as required', () => {
    const content = 'API_KEY=\nDB_PASSWORD';
    const result = detectEnvVarsFromEnvFile(content, '.env.example');
    expect(result[0]!.name).toBe('API_KEY');
    expect(result[0]!.required).toBe(true);
    expect(result[1]!.name).toBe('DB_PASSWORD');
    expect(result[1]!.required).toBe(true);
  });

  it('should strip surrounding quotes from values', () => {
    const content = 'APP_NAME="my-app"\nLOG_LEVEL=\'debug\'';
    const result = detectEnvVarsFromEnvFile(content, '.env');
    expect(result[0]!.defaultValue).toBe('my-app');
    expect(result[1]!.defaultValue).toBe('debug');
  });

  it('should classify vars correctly', () => {
    const content = 'DB_PASSWORD=secret123\nDB_HOST=localhost\nNODE_ENV=dev';
    const result = detectEnvVarsFromEnvFile(content, '.env.example');
    expect(result[0]!.classification).toBe('secret');
    expect(result[1]!.classification).toBe('database');
    expect(result[2]!.classification).toBe('config');
  });
});

describe('detectEnvVarsFromDockerCompose', () => {
  it('should return empty vars with warning for invalid YAML', () => {
    const result = detectEnvVarsFromDockerCompose('{{invalid}');
    expect(result.vars).toEqual([]);
    expect(result.warning).toBeDefined();
  });

  it('should parse list format environment vars', () => {
    const content = `
services:
  app:
    environment:
      - NODE_ENV=production
      - API_KEY
      - PORT=3000
`;
    const result = detectEnvVarsFromDockerCompose(content);
    expect(result.vars).toHaveLength(3);
    expect(result.vars[0]).toMatchObject({ name: 'NODE_ENV', required: false, defaultValue: 'production' });
    expect(result.vars[1]).toMatchObject({ name: 'API_KEY', required: true });
    expect(result.vars[2]).toMatchObject({ name: 'PORT', required: false, defaultValue: '3000' });
  });

  it('should parse map format environment vars', () => {
    const content = `
services:
  app:
    environment:
      NODE_ENV: production
      DB_HOST: localhost
      API_KEY:
`;
    const result = detectEnvVarsFromDockerCompose(content);
    expect(result.vars).toHaveLength(3);
    expect(result.vars[0]).toMatchObject({ name: 'NODE_ENV', defaultValue: 'production' });
    expect(result.vars[1]).toMatchObject({ name: 'DB_HOST', defaultValue: 'localhost' });
    expect(result.vars[2]).toMatchObject({ name: 'API_KEY', required: true });
  });

  it('should handle multiple services', () => {
    const content = `
services:
  web:
    environment:
      - PORT=3000
  api:
    environment:
      - PORT=8080
`;
    const result = detectEnvVarsFromDockerCompose(content);
    expect(result.vars).toHaveLength(2);
  });
});

describe('detectEnvVarsFromSpringConfig', () => {
  it('should extract ${VAR} placeholders', () => {
    const content = 'spring.datasource.url=jdbc:postgresql://${DB_HOST}:${DB_PORT}/mydb';
    const result = detectEnvVarsFromSpringConfig(content);
    expect(result).toHaveLength(2);
    expect(result[0]).toMatchObject({ name: 'DB_HOST', required: true });
    expect(result[1]).toMatchObject({ name: 'DB_PORT', required: true });
  });

  it('should extract ${VAR:default} with defaults', () => {
    const content = 'server.port=${SERVER_PORT:8080}';
    const result = detectEnvVarsFromSpringConfig(content);
    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ name: 'SERVER_PORT', required: false, defaultValue: '8080' });
  });

  it('should deduplicate within the same file', () => {
    const content = `
spring.datasource.url=jdbc:postgresql://\${DB_HOST}/mydb
spring.datasource.host=\${DB_HOST}
`;
    const result = detectEnvVarsFromSpringConfig(content);
    expect(result).toHaveLength(1);
  });

  it('should handle YAML-style Spring config', () => {
    const content = `
spring:
  datasource:
    url: jdbc:postgresql://\${DB_HOST:localhost}:\${DB_PORT:5432}/\${DB_NAME}
    username: \${DB_USER}
    password: \${DB_PASSWORD}
`;
    const result = detectEnvVarsFromSpringConfig(content);
    expect(result).toHaveLength(5);
    expect(result.find((v) => v.name === 'DB_HOST')?.defaultValue).toBe('localhost');
    expect(result.find((v) => v.name === 'DB_PORT')?.defaultValue).toBe('5432');
    expect(result.find((v) => v.name === 'DB_NAME')?.required).toBe(true);
    expect(result.find((v) => v.name === 'DB_PASSWORD')?.classification).toBe('secret');
  });
});

describe('inferFrameworkEnvVars', () => {
  it('should infer NODE_ENV and PORT for Node.js', () => {
    const result = inferFrameworkEnvVars('javascript');
    expect(result.find((v) => v.name === 'NODE_ENV')).toBeDefined();
    expect(result.find((v) => v.name === 'PORT')).toBeDefined();
  });

  it('should infer NODE_ENV for TypeScript', () => {
    const result = inferFrameworkEnvVars('typescript');
    expect(result.find((v) => v.name === 'NODE_ENV')).toBeDefined();
  });

  it('should infer Spring vars for Java Spring Boot', () => {
    const result = inferFrameworkEnvVars('java', 'spring-boot');
    expect(result.find((v) => v.name === 'SPRING_PROFILES_ACTIVE')).toBeDefined();
    expect(result.find((v) => v.name === 'SERVER_PORT')).toBeDefined();
    expect(result.find((v) => v.name === 'JAVA_OPTS')).toBeDefined();
  });

  it('should infer JAVA_OPTS for plain Java', () => {
    const result = inferFrameworkEnvVars('java');
    expect(result.find((v) => v.name === 'JAVA_OPTS')).toBeDefined();
    expect(result.find((v) => v.name === 'SPRING_PROFILES_ACTIVE')).toBeUndefined();
  });

  it('should infer ASPNETCORE vars for .NET', () => {
    const result = inferFrameworkEnvVars('dotnet');
    expect(result.find((v) => v.name === 'ASPNETCORE_ENVIRONMENT')).toBeDefined();
    expect(result.find((v) => v.name === 'ASPNETCORE_URLS')).toBeDefined();
  });

  it('should infer Flask vars for Python Flask', () => {
    const result = inferFrameworkEnvVars('python', 'flask');
    expect(result.find((v) => v.name === 'FLASK_APP')).toBeDefined();
    expect(result.find((v) => v.name === 'FLASK_ENV')).toBeDefined();
    expect(result.find((v) => v.name === 'PYTHONUNBUFFERED')).toBeDefined();
  });

  it('should infer Django vars for Python Django', () => {
    const result = inferFrameworkEnvVars('python', 'django');
    expect(result.find((v) => v.name === 'DJANGO_SETTINGS_MODULE')).toBeDefined();
    expect(result.find((v) => v.name === 'DJANGO_SECRET_KEY')).toBeDefined();
  });

  it('should infer FastAPI vars for Python FastAPI', () => {
    const result = inferFrameworkEnvVars('python', 'fastapi');
    expect(result.find((v) => v.name === 'UVICORN_HOST')).toBeDefined();
    expect(result.find((v) => v.name === 'UVICORN_PORT')).toBeDefined();
  });

  it('should infer PORT for Go', () => {
    const result = inferFrameworkEnvVars('go');
    expect(result.find((v) => v.name === 'PORT')).toBeDefined();
  });

  it('should infer RUST_LOG for Rust', () => {
    const result = inferFrameworkEnvVars('rust');
    expect(result.find((v) => v.name === 'RUST_LOG')).toBeDefined();
  });

  it('should mark all inferred vars as framework-inferred source', () => {
    const result = inferFrameworkEnvVars('javascript');
    for (const v of result) {
      expect(v.source).toBe('framework-inferred');
    }
  });

  it('should mark all inferred vars as not required', () => {
    const result = inferFrameworkEnvVars('java', 'spring-boot');
    for (const v of result) {
      expect(v.required).toBe(false);
    }
  });
});

describe('deduplicateEnvVars', () => {
  it('should return empty array for empty input', () => {
    expect(deduplicateEnvVars([])).toEqual([]);
  });

  it('should keep unique vars', () => {
    const vars: DetectedEnvVar[] = [
      { name: 'A', classification: 'config', source: '.env', required: true },
      { name: 'B', classification: 'config', source: '.env', required: false },
    ];
    expect(deduplicateEnvVars(vars)).toHaveLength(2);
  });

  it('should deduplicate by name keeping first occurrence', () => {
    const vars: DetectedEnvVar[] = [
      { name: 'PORT', classification: 'config', source: '.env.example', required: false, defaultValue: '3000' },
      { name: 'PORT', classification: 'config', source: 'docker-compose.yml', required: false, defaultValue: '8080' },
    ];
    const result = deduplicateEnvVars(vars);
    expect(result).toHaveLength(1);
    expect(result[0]!.source).toBe('.env.example');
    expect(result[0]!.defaultValue).toBe('3000');
  });

  it('should prefer explicit source over framework-inferred', () => {
    const vars: DetectedEnvVar[] = [
      { name: 'NODE_ENV', classification: 'config', source: 'framework-inferred', required: false, defaultValue: 'production' },
      { name: 'NODE_ENV', classification: 'config', source: '.env.example', required: true },
    ];
    const result = deduplicateEnvVars(vars);
    expect(result).toHaveLength(1);
    expect(result[0]!.source).toBe('.env.example');
    expect(result[0]!.required).toBe(true);
  });

  it('should not replace explicit source with another explicit source', () => {
    const vars: DetectedEnvVar[] = [
      { name: 'PORT', classification: 'config', source: '.env.example', required: false, defaultValue: '3000' },
      { name: 'PORT', classification: 'config', source: 'docker-compose.yml', required: false, defaultValue: '8080' },
    ];
    const result = deduplicateEnvVars(vars);
    expect(result[0]!.source).toBe('.env.example');
  });
});
