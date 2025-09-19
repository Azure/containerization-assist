/**
 * Unit tests for Configuration-Driven Sampling System
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { join } from 'path';
import { mkdtempSync, writeFileSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import { createConfigurationManager, type ConfigurationManagerInterface } from '../../../../src/lib/sampling-config';
import { createConfigValidator } from '../../../../src/lib/config-validator';
import { createConfigScoringEngine } from '../../../../src/lib/scoring/internal/config-scoring-engine';

describe('ConfigurationManager', () => {
  let tempDir: string;
  let configManager: ConfigurationManagerInterface;

  beforeEach(() => {
    tempDir = mkdtempSync(join(tmpdir(), 'sampling-config-test-'));
    configManager = createConfigurationManager(tempDir);
  });

  afterEach(() => {
    rmSync(tempDir, { recursive: true, force: true });
  });

  const createTestConfig = () => {
    // Create test scoring profile
    const dockerfileConfig = {
      name: 'dockerfile',
      version: '1.0.0',
      metadata: {
        description: 'Test dockerfile profile',
        created: '2024-12-19',
        author: 'test',
      },
      base_score: 30,
      max_score: 100,
      timeout_ms: 2000,
      category_weights: {
        security: 1.2,
        performance: 1.1,
      },
      rules: {
        security: [
          {
            name: 'test_rule',
            matcher: {
              type: 'regex',
              pattern: 'FROM',
            },
            points: 10,
            weight: 1.0,
            category: 'security',
            description: 'Test rule',
          },
        ],
      },
    };

    // Create test strategies
    const strategiesConfig = {
      version: '1.0.0',
      strategies: {
        dockerfile: ['Create Dockerfile', 'Create optimized Dockerfile'],
      },
      selection_rules: {
        dockerfile: {
          conditions: [],
          default_strategy_index: 0,
        },
      },
    };

    // Create test environment override
    const envConfig = {
      environment: 'test',
      overrides: {
        scoring: {
          dockerfile: {
            category_weights: {
              security: 1.5,
            },
          },
        },
      },
    };

    // Write files
    writeFileSync(join(tempDir, 'scoring'), '');
    rmSync(join(tempDir, 'scoring'));
    writeFileSync(join(tempDir, 'environments'), '');
    rmSync(join(tempDir, 'environments'));

    const scoringDir = join(tempDir, 'scoring');
    const envDir = join(tempDir, 'environments');
    
    writeFileSync(scoringDir, '');
    rmSync(scoringDir);
    writeFileSync(envDir, '');
    rmSync(envDir);

    const fs = require('fs');
    fs.mkdirSync(join(tempDir, 'scoring'), { recursive: true });
    fs.mkdirSync(join(tempDir, 'environments'), { recursive: true });

    writeFileSync(
      join(tempDir, 'scoring', 'dockerfile.yml'),
      `# Test dockerfile config
name: "dockerfile"
version: "1.0.0"
metadata:
  description: "Test dockerfile profile"
  created: "2024-12-19"
  author: "test"
base_score: 30
max_score: 100
timeout_ms: 2000
category_weights:
  security: 1.2
  performance: 1.1
rules:
  security:
    - name: "test_rule"
      matcher:
        type: "regex"
        pattern: "FROM"
      points: 10
      weight: 1.0
      category: "security"
      description: "Test rule"`
    );

    writeFileSync(
      join(tempDir, 'strategies.yml'),
      `# Test strategies config
version: "1.0.0"
strategies:
  dockerfile:
    - "Create Dockerfile"
    - "Create optimized Dockerfile"
selection_rules:
  dockerfile:
    conditions: []
    default_strategy_index: 0`
    );

    writeFileSync(
      join(tempDir, 'environments', 'production.yml'),
      `# Test production config
environment: "production"
overrides:
  scoring:
    dockerfile:
      category_weights:
        security: 1.5`
    );

    writeFileSync(
      join(tempDir, 'environments', 'development.yml'),
      `# Test development config
environment: "development"
overrides:
  scoring:
    dockerfile:
      category_weights:
        security: 1.0`
    );
  };

  describe('loadConfiguration', () => {
    it('should load valid configuration successfully', async () => {
      createTestConfig();
      
      const result = await configManager.loadConfiguration();
      
      expect(result.ok).toBe(true);
    });

    it('should fail to load missing configuration files', async () => {
      const result = await configManager.loadConfiguration();
      
      expect(result.ok).toBe(false);
      expect(result.error).toContain('Failed to load dockerfile scoring config');
    });
  });

  describe('validateConfiguration', () => {
    it('should validate correct configuration', async () => {
      createTestConfig();
      await configManager.loadConfiguration();
      
      const config = configManager.getConfiguration();
      const result = await configManager.validateConfiguration(config);
      
      expect(result.isValid).toBe(true);
      expect(result.errors).toHaveLength(0);
    });
  });

  describe('resolveForEnvironment', () => {
    it('should apply environment overrides correctly', async () => {
      createTestConfig();
      await configManager.loadConfiguration();
      
      const prodConfig = configManager.resolveForEnvironment('production');
      const devConfig = configManager.resolveForEnvironment('development');
      
      expect(prodConfig.scoring.dockerfile.category_weights.security).toBe(1.5);
      expect(devConfig.scoring.dockerfile.category_weights.security).toBe(1.0);
    });

    it('should return base config for unknown environment', async () => {
      createTestConfig();
      await configManager.loadConfiguration();
      
      const config = configManager.resolveForEnvironment('unknown');
      
      expect(config.scoring.dockerfile.category_weights.security).toBe(1.2); // Base value
    });
  });
});

describe('ConfigValidator', () => {
  let tempDir: string;

  beforeEach(() => {
    tempDir = mkdtempSync(join(tmpdir(), 'validator-test-'));
  });

  afterEach(() => {
    rmSync(tempDir, { recursive: true, force: true });
  });

  it('should validate startup configuration', async () => {
    // Create minimal valid config
    const fs = require('fs');
    fs.mkdirSync(join(tempDir, 'scoring'), { recursive: true });
    fs.mkdirSync(join(tempDir, 'environments'), { recursive: true });

    writeFileSync(
      join(tempDir, 'scoring', 'dockerfile.yml'),
      `name: "dockerfile"
version: "1.0.0"
metadata:
  description: "Test"
  created: "2024-12-19"
  author: "test"
base_score: 30
max_score: 100
timeout_ms: 2000
category_weights:
  security: 1.0
rules:
  security:
    - name: "test"
      matcher:
        type: "regex"
        pattern: "FROM"
      points: 10
      weight: 1.0
      category: "security"
      description: "Test"`
    );

    writeFileSync(
      join(tempDir, 'strategies.yml'),
      `version: "1.0.0"
strategies:
  dockerfile: ["test"]
selection_rules:
  dockerfile:
    conditions: []
    default_strategy_index: 0`
    );

    writeFileSync(
      join(tempDir, 'environments', 'production.yml'),
      `environment: "production"
overrides: {}`
    );

    writeFileSync(
      join(tempDir, 'environments', 'development.yml'),
      `environment: "development"  
overrides: {}`
    );

    const validator = createConfigValidator(tempDir);
    const result = await validator.validateStartup();
    
    expect(result.isValid).toBe(true);
  });
});

describe('ConfigScoringEngine', () => {
  let engine: ReturnType<typeof createConfigScoringEngine>;

  beforeEach(() => {
    engine = createConfigScoringEngine(false);
  });

  it('should score content using configuration profile', () => {
    const profile = {
      name: 'test',
      version: '1.0.0',
      metadata: {
        description: 'Test profile',
        created: '2024-12-19',
        author: 'test',
      },
      base_score: 50,
      max_score: 100,
      timeout_ms: 2000,
      category_weights: {
        test: 1.0,
      },
      rules: {
        test: [
          {
            name: 'has_from',
            matcher: {
              type: 'regex' as const,
              pattern: 'FROM',
            },
            points: 20,
            weight: 1.0,
            category: 'test',
            description: 'Has FROM statement',
          },
        ],
      },
    };

    const content = 'FROM alpine:latest\nRUN echo "hello"';
    const result = engine.score(content, profile);
    
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.total).toBeGreaterThan(50);
      expect(result.value.matchedRules).toContain('has_from');
    }
  });

  it('should handle penalties correctly', () => {
    const profile = {
      name: 'test',
      version: '1.0.0',
      metadata: {
        description: 'Test profile',
        created: '2024-12-19',
        author: 'test',
      },
      base_score: 50,
      max_score: 100,
      timeout_ms: 2000,
      category_weights: {
        test: 1.0,
      },
      rules: {
        test: [],
      },
      penalties: [
        {
          name: 'bad_pattern',
          matcher: {
            type: 'regex' as const,
            pattern: 'BAD',
          },
          points: -10,
          description: 'Bad pattern found',
        },
      ],
    };

    const content = 'FROM alpine:latest\nRUN echo "BAD"';
    const result = engine.score(content, profile);
    
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.total).toBe(40); // 50 base - 10 penalty
      expect(result.value.appliedPenalties).toContain('bad_pattern');
    }
  });
});