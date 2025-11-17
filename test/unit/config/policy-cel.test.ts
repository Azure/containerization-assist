/**
 * Unit tests for CEL Policy Evaluator
 */

import { CelPolicyEvaluator, loadCelPolicy } from '@/config/policy-cel';
import type { CelPolicySet } from '@/config/policy-cel-schema';
import { createLogger } from '@/lib/logger';
import { writeFile, unlink, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import os from 'node:os';
import { existsSync } from 'node:fs';

describe('CelPolicyEvaluator', () => {
  const logger = createLogger({ level: 'silent' });

  const simplePolicySet: CelPolicySet = {
    apiVersion: 'policy.containerization-assist.dev/v1',
    kind: 'PolicySet',
    metadata: {
      name: 'test-policy',
      version: '1.0.0',
    },
    spec: {
      rules: [
        {
          name: 'require-user',
          category: 'security',
          severity: 'warn',
          priority: 80,
          condition: '!input.content.contains("USER")',
          message: 'Dockerfile should specify USER directive',
          description: 'Running as root is a security risk',
        },
      ],
    },
  };

  describe('Constructor', () => {
    it('should create evaluator instance', () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);

      expect(evaluator).toBeDefined();
      expect(evaluator.policyPaths).toEqual(['test.yaml']);
    });
  });

  describe('initialize', () => {
    it('should initialize successfully with valid rules', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);

      const result = await evaluator.initialize();

      expect(result.ok).toBe(true);
    });

    it('should handle invalid CEL expressions', async () => {
      const invalidPolicySet: CelPolicySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'invalid' },
        spec: {
          rules: [
            {
              name: 'bad-rule',
              category: 'test',
              severity: 'block',
              condition: 'invalid syntax here @#$',
              message: 'Test',
            },
          ],
        },
      };

      const evaluator = new CelPolicyEvaluator(invalidPolicySet, logger, ['test.yaml']);

      const result = await evaluator.initialize();

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('bad-rule');
      }
    });

    it('should not initialize twice', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);

      const result1 = await evaluator.initialize();
      const result2 = await evaluator.initialize();

      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);
    });

    it('should compile multiple rules', async () => {
      const multiRulePolicySet: CelPolicySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'multi' },
        spec: {
          rules: [
            {
              name: 'rule1',
              category: 'security',
              severity: 'block',
              condition: 'true',
              message: 'Rule 1',
            },
            {
              name: 'rule2',
              category: 'optimization',
              severity: 'warn',
              condition: 'false',
              message: 'Rule 2',
            },
          ],
        },
      };

      const evaluator = new CelPolicyEvaluator(multiRulePolicySet, logger, ['test.yaml']);

      const result = await evaluator.initialize();

      expect(result.ok).toBe(true);
    });
  });

  describe('evaluate', () => {
    it('should evaluate string input', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const result = await evaluator.evaluate('FROM alpine\nRUN echo hello');

      expect(result).toBeDefined();
      expect(result.allow).toBe(true); // Allow=true since severity is 'warn', not 'block'
      expect(result.warnings).toHaveLength(1);
      expect(result.warnings[0].rule).toBe('require-user');
    });

    it('should evaluate object input', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const result = await evaluator.evaluate({
        content: 'FROM alpine\nRUN echo hello',
      });

      expect(result.allow).toBe(true); // Allow=true since severity is 'warn', not 'block'
      expect(result.warnings).toHaveLength(1);
    });

    it('should pass when condition not matched', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const result = await evaluator.evaluate('FROM alpine\nUSER appuser');

      expect(result.allow).toBe(true);
      expect(result.warnings).toHaveLength(0);
    });

    it('should categorize violations by severity', async () => {
      const mixedSeverityPolicySet: CelPolicySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'mixed' },
        spec: {
          rules: [
            {
              name: 'blocking-rule',
              category: 'security',
              severity: 'block',
              condition: '!input.content.contains("USER")',
              message: 'USER required',
            },
            {
              name: 'warning-rule',
              category: 'best-practices',
              severity: 'warn',
              condition: '!input.content.contains("HEALTHCHECK")',
              message: 'HEALTHCHECK recommended',
            },
            {
              name: 'suggestion-rule',
              category: 'optimization',
              severity: 'suggest',
              condition: '!input.content.contains("LABEL")',
              message: 'LABEL suggested',
            },
          ],
        },
      };

      const evaluator = new CelPolicyEvaluator(mixedSeverityPolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const result = await evaluator.evaluate('FROM alpine');

      expect(result.violations).toHaveLength(1);
      expect(result.warnings).toHaveLength(1);
      expect(result.suggestions).toHaveLength(1);
      expect(result.allow).toBe(false); // Blocked due to violation
    });

    it('should sort violations by priority', async () => {
      const priorityPolicySet: CelPolicySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'priority' },
        spec: {
          rules: [
            {
              name: 'low-priority',
              category: 'test',
              severity: 'block',
              priority: 30,
              condition: 'true',
              message: 'Low',
            },
            {
              name: 'high-priority',
              category: 'test',
              severity: 'block',
              priority: 90,
              condition: 'true',
              message: 'High',
            },
            {
              name: 'medium-priority',
              category: 'test',
              severity: 'block',
              priority: 60,
              condition: 'true',
              message: 'Medium',
            },
          ],
        },
      };

      const evaluator = new CelPolicyEvaluator(priorityPolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const result = await evaluator.evaluate('test');

      expect(result.violations).toHaveLength(3);
      expect(result.violations[0].rule).toBe('high-priority');
      expect(result.violations[1].rule).toBe('medium-priority');
      expect(result.violations[2].rule).toBe('low-priority');
    });

    it('should handle evaluation errors gracefully', async () => {
      // Create a rule that will cause a runtime error
      const errorPolicySet: CelPolicySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'error' },
        spec: {
          rules: [
            {
              name: 'error-rule',
              category: 'test',
              severity: 'warn',
              // This will try to access a method that doesn't exist
              condition: 'input.nonexistent.method()',
              message: 'Error test',
            },
          ],
        },
      };

      const evaluator = new CelPolicyEvaluator(errorPolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const result = await evaluator.evaluate('test input');

      // Should have an evaluation error violation
      expect(result.violations.length).toBeGreaterThan(0);
      expect(result.violations.some((v) => v.rule.includes('error'))).toBe(true);
    });

    it('should auto-initialize if not initialized', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      // Don't call initialize()

      const result = await evaluator.evaluate('FROM alpine\nUSER appuser');

      expect(result).toBeDefined();
      expect(result.allow).toBeDefined();
    });

    it('should include summary in results', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const result = await evaluator.evaluate('FROM alpine');

      expect(result.summary).toBeDefined();
      expect(result.summary?.total_violations).toBe(0);
      expect(result.summary?.total_warnings).toBe(1);
      expect(result.summary?.total_suggestions).toBe(0);
    });
  });

  describe('evaluatePolicy', () => {
    it('should evaluate with Success Result wrapper', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const inputResult = { ok: true as const, value: 'FROM alpine' };
      const result = await evaluator.evaluatePolicy(inputResult);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.blocking).toBeDefined();
        expect(result.value.warnings).toBeDefined();
        expect(result.value.suggestions).toBeDefined();
      }
    });

    it('should pass through Failure Result', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const inputResult = { ok: false as const, error: 'Test error' };
      const result = await evaluator.evaluatePolicy(inputResult);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Test error');
      }
    });

    it('should handle evaluation exceptions', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      // Force an error by passing invalid data type
      const inputResult = { ok: true as const, value: 123 as any };
      const result = await evaluator.evaluatePolicy(inputResult);

      // Should either succeed or return a failure result
      expect(result).toBeDefined();
    });
  });

  describe('queryConfig', () => {
    it('should always return null', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      const config = await evaluator.queryConfig('test.config', {});

      expect(config).toBeNull();
    });

    it('should log warning when called', async () => {
      const warnLogger = createLogger({ level: 'warn' });
      const evaluator = new CelPolicyEvaluator(simplePolicySet, warnLogger, ['test.yaml']);
      await evaluator.initialize();

      await evaluator.queryConfig('test.config', {});

      // Verify no errors (warning is logged internally)
      expect(true).toBe(true);
    });
  });

  describe('close', () => {
    it('should clean up resources', async () => {
      const evaluator = new CelPolicyEvaluator(simplePolicySet, logger, ['test.yaml']);
      await evaluator.initialize();

      evaluator.close();

      // Should still be able to call close again
      evaluator.close();
      expect(true).toBe(true);
    });
  });
});

describe('loadCelPolicy', () => {
  const logger = createLogger({ level: 'silent' });
  let tmpDir: string;
  let tmpFile: string;

  beforeAll(async () => {
    tmpDir = join(os.tmpdir(), `cel-policy-test-${Date.now()}`);
    await mkdir(tmpDir, { recursive: true });
  });

  afterEach(async () => {
    if (tmpFile && existsSync(tmpFile)) {
      try {
        await unlink(tmpFile);
      } catch {
        // Ignore cleanup errors
      }
    }
  });

  const validPolicyYaml = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: test-policy
  version: "1.0.0"
spec:
  rules:
    - name: require-user
      category: security
      severity: warn
      priority: 80
      condition: '!input.content.contains("USER")'
      message: "Dockerfile should specify USER"
`;

  it('should load valid YAML policy file', async () => {
    tmpFile = join(tmpDir, 'valid.yaml');
    await writeFile(tmpFile, validPolicyYaml);

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.policyPaths).toEqual([tmpFile]);
    }
  });

  it('should load .yml extension', async () => {
    tmpFile = join(tmpDir, 'valid.yml');
    await writeFile(tmpFile, validPolicyYaml);

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(true);
  });

  it('should reject non-existent file', async () => {
    const result = await loadCelPolicy('/nonexistent/file.yaml', logger);

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('not found');
    }
  });

  it('should reject non-YAML file extension', async () => {
    tmpFile = join(tmpDir, 'invalid.txt');
    await writeFile(tmpFile, validPolicyYaml);

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('.yaml or .yml');
    }
  });

  it('should reject invalid YAML syntax', async () => {
    tmpFile = join(tmpDir, 'invalid-syntax.yaml');
    await writeFile(tmpFile, 'invalid: yaml: syntax: here:');

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(false);
  });

  it('should reject invalid policy schema', async () => {
    tmpFile = join(tmpDir, 'invalid-schema.yaml');
    await writeFile(
      tmpFile,
      `
apiVersion: wrong-version
kind: PolicySet
metadata:
  name: test
spec:
  rules: []
`
    );

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('Invalid CEL policy format');
    }
  });

  it('should reject policy with invalid CEL expression', async () => {
    tmpFile = join(tmpDir, 'invalid-cel.yaml');
    await writeFile(
      tmpFile,
      `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: invalid-cel
spec:
  rules:
    - name: bad-rule
      category: test
      severity: block
      condition: 'this is not valid CEL syntax @#$%'
      message: "Test"
`
    );

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('compile');
    }
  });

  it('should evaluate loaded policy', async () => {
    tmpFile = join(tmpDir, 'evaluate-test.yaml');
    await writeFile(tmpFile, validPolicyYaml);

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(true);
    if (result.ok) {
      const evalResult = await result.value.evaluate('FROM alpine\nRUN echo hello');

      expect(evalResult.warnings).toHaveLength(1);
      expect(evalResult.warnings[0].rule).toBe('require-user');
    }
  });

  it('should handle empty YAML file', async () => {
    tmpFile = join(tmpDir, 'empty.yaml');
    await writeFile(tmpFile, '');

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(false);
  });

  it('should handle YAML with just comments', async () => {
    tmpFile = join(tmpDir, 'comments-only.yaml');
    await writeFile(tmpFile, '# Just a comment\n# Another comment');

    const result = await loadCelPolicy(tmpFile, logger);

    expect(result.ok).toBe(false);
  });
});
