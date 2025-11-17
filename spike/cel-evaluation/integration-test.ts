/**
 * Phase 0.3: Integration Proof-of-Concept
 *
 * Tests that CEL can integrate with the existing RegoEvaluator interface.
 */

import { evaluate, parse } from '@marcbachmann/cel-js';
import yaml from 'js-yaml';
import { z } from 'zod';

// Import types from the actual codebase
type Result<T> =
  | { ok: true; value: T }
  | { ok: false; error: string; message?: string; hint?: string; resolution?: string };

function Success<T>(value: T): Result<T> {
  return { ok: true, value };
}

function Failure(error: string, details?: { message?: string; hint?: string; resolution?: string }): Result<never> {
  return { ok: false, error, ...details };
}

// Mock policy interfaces (from src/config/policy-rego.ts)
interface RegoPolicyViolation {
  rule: string;
  message: string;
  severity: 'block' | 'warn' | 'suggest';
  category: string;
  priority?: number;
  description?: string;
}

interface RegoPolicyResult {
  allow: boolean;
  violations: RegoPolicyViolation[];
  warnings: RegoPolicyViolation[];
  suggestions: RegoPolicyViolation[];
  summary?: {
    total_violations: number;
    total_warnings: number;
    total_suggestions: number;
  };
}

interface CategorizedViolations {
  blocking: RegoPolicyViolation[];
  warnings: RegoPolicyViolation[];
  suggestions: RegoPolicyViolation[];
}

interface RegoEvaluator {
  evaluate(input: string | Record<string, unknown>): Promise<RegoPolicyResult>;
  evaluatePolicy<T>(
    result: Result<T>,
    packageName?: string
  ): Promise<Result<CategorizedViolations>>;
  queryConfig<T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null>;
  close(): void;
  policyPaths: string[];
}

// CEL Policy Schema (same as yaml-schema-test.ts)
const celPolicyRuleSchema = z.object({
  name: z.string(),
  category: z.string(),
  severity: z.enum(['block', 'warn', 'suggest']),
  priority: z.number().int().min(0).max(100).optional().default(50),
  condition: z.string(),
  message: z.string(),
  description: z.string().optional(),
});

const celPolicySetSchema = z.object({
  apiVersion: z.literal('policy.containerization-assist.dev/v1'),
  kind: z.literal('PolicySet'),
  metadata: z.object({
    name: z.string(),
    version: z.string().optional(),
    description: z.string().optional(),
  }),
  spec: z.object({
    rules: z.array(celPolicyRuleSchema),
  }),
});

type CelPolicySet = z.infer<typeof celPolicySetSchema>;
type CelPolicyRule = z.infer<typeof celPolicyRuleSchema>;

// Mock CEL Evaluator implementing RegoEvaluator interface
class MockCelEvaluator implements RegoEvaluator {
  private compiledRules: Array<{ rule: CelPolicyRule; program: any }> = [];

  constructor(
    private policySet: CelPolicySet,
    public policyPaths: string[]
  ) {
    // Compile all CEL expressions
    for (const rule of policySet.spec.rules) {
      try {
        const program = parse(rule.condition);
        this.compiledRules.push({ rule, program });
      } catch (error) {
        console.error(`Failed to compile rule ${rule.name}:`, error);
      }
    }
  }

  async evaluate(input: string | Record<string, unknown>): Promise<RegoPolicyResult> {
    const inputData = typeof input === 'string' ? { content: input } : input;

    const violations: RegoPolicyViolation[] = [];
    const warnings: RegoPolicyViolation[] = [];
    const suggestions: RegoPolicyViolation[] = [];

    for (const { rule, program } of this.compiledRules) {
      try {
        // Evaluate CEL expression
        const result = program({ input: inputData });
        const matched = Boolean(result);

        if (matched) {
          const violation: RegoPolicyViolation = {
            rule: rule.name,
            category: rule.category,
            severity: rule.severity,
            message: rule.message,
            description: rule.description,
            priority: rule.priority,
          };

          // Categorize by severity
          if (rule.severity === 'block') {
            violations.push(violation);
          } else if (rule.severity === 'warn') {
            warnings.push(violation);
          } else {
            suggestions.push(violation);
          }
        }
      } catch (error) {
        console.error(`Error evaluating rule ${rule.name}:`, error);
      }
    }

    const allow = violations.length === 0;

    return {
      allow,
      violations,
      warnings,
      suggestions,
      summary: {
        total_violations: violations.length,
        total_warnings: warnings.length,
        total_suggestions: suggestions.length,
      },
    };
  }

  async evaluatePolicy<T>(
    result: Result<T>,
    _packageName?: string
  ): Promise<Result<CategorizedViolations>> {
    if (!result.ok) {
      return result as Result<never>;
    }

    try {
      const policyResult = await this.evaluate(result.value as string | Record<string, unknown>);

      return Success({
        blocking: policyResult.violations,
        warnings: policyResult.warnings,
        suggestions: policyResult.suggestions,
      });
    } catch (error) {
      return Failure(`CEL policy evaluation failed: ${error}`);
    }
  }

  async queryConfig<T = unknown>(
    _packageName: string,
    _input: Record<string, unknown>
  ): Promise<T | null> {
    // CEL policies do not support configuration queries
    console.log('CEL policies do not support queryConfig - returning null');
    return null;
  }

  close(): void {
    // No resources to clean up
  }
}

async function testIntegration() {
  console.log('=== Integration Proof-of-Concept Tests ===\n');

  // Test 1: Create mock CEL evaluator from YAML
  console.log('Test 1: Create CEL evaluator from YAML policy');
  try {
    const yamlPolicy = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: integration-test
spec:
  rules:
    - name: require-user
      category: security
      severity: warn
      priority: 80
      condition: '!input.content.contains("USER")'
      message: "Dockerfile should specify USER directive"
      description: "Running as root is a security risk"
`;

    const parsed = yaml.load(yamlPolicy);
    const validated = celPolicySetSchema.parse(parsed);
    const evaluator = new MockCelEvaluator(validated, ['spike.yaml']);

    console.log('  Created evaluator:', {
      policyName: validated.metadata.name,
      ruleCount: validated.spec.rules.length,
    });
    console.log('  ✅ PASS: CEL evaluator created successfully\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 2: Type checking - verify it implements RegoEvaluator
  console.log('Test 2: Type compatibility with RegoEvaluator interface');
  try {
    const yamlPolicy = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: type-test
spec:
  rules:
    - name: test-rule
      category: test
      severity: suggest
      condition: 'true'
      message: "Test message"
`;

    const parsed = yaml.load(yamlPolicy);
    const validated = celPolicySetSchema.parse(parsed);
    const evaluator: RegoEvaluator = new MockCelEvaluator(validated, ['test.yaml']);

    // Verify interface compliance
    console.log('  Interface properties:', {
      hasEvaluate: typeof evaluator.evaluate === 'function',
      hasEvaluatePolicy: typeof evaluator.evaluatePolicy === 'function',
      hasQueryConfig: typeof evaluator.queryConfig === 'function',
      hasClose: typeof evaluator.close === 'function',
      hasPolicyPaths: Array.isArray(evaluator.policyPaths),
    });
    console.log('  ✅ PASS: Type checking passes\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 3: Evaluate method returns correct structure
  console.log('Test 3: evaluate() returns RegoPolicyResult structure');
  try {
    const yamlPolicy = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: structure-test
spec:
  rules:
    - name: require-user
      category: security
      severity: block
      condition: '!input.content.contains("USER")'
      message: "USER directive required"
`;

    const parsed = yaml.load(yamlPolicy);
    const validated = celPolicySetSchema.parse(parsed);
    const evaluator = new MockCelEvaluator(validated, ['test.yaml']);

    const result = await evaluator.evaluate('FROM alpine\nRUN echo hello');

    console.log('  Result structure:', {
      hasAllow: typeof result.allow === 'boolean',
      hasViolations: Array.isArray(result.violations),
      hasWarnings: Array.isArray(result.warnings),
      hasSuggestions: Array.isArray(result.suggestions),
      hasSummary: result.summary !== undefined,
    });

    console.log('  Result values:', {
      allow: result.allow,
      violations: result.violations.length,
      warnings: result.warnings.length,
      suggestions: result.suggestions.length,
    });

    console.log('  Violation details:', result.violations[0]);

    // Verify structure matches Rego format
    if (
      typeof result.allow === 'boolean' &&
      Array.isArray(result.violations) &&
      Array.isArray(result.warnings) &&
      Array.isArray(result.suggestions) &&
      result.violations[0].rule === 'require-user' &&
      result.violations[0].severity === 'block'
    ) {
      console.log('  ✅ PASS: Results structure matches Rego format\n');
    } else {
      console.error('  ❌ FAIL: Structure mismatch');
      return false;
    }
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 4: evaluatePolicy with Result wrapper
  console.log('Test 4: evaluatePolicy() with Result wrapper');
  try {
    const yamlPolicy = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: wrapper-test
spec:
  rules:
    - name: healthcheck-recommended
      category: health
      severity: suggest
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "HEALTHCHECK directive recommended"
`;

    const parsed = yaml.load(yamlPolicy);
    const validated = celPolicySetSchema.parse(parsed);
    const evaluator = new MockCelEvaluator(validated, ['test.yaml']);

    const inputResult = Success('FROM alpine\nUSER appuser');
    const policyResult = await evaluator.evaluatePolicy(inputResult);

    if (policyResult.ok) {
      console.log('  Result:', {
        blocking: policyResult.value.blocking.length,
        warnings: policyResult.value.warnings.length,
        suggestions: policyResult.value.suggestions.length,
      });
      console.log('  ✅ PASS: evaluatePolicy works with Result wrapper\n');
    } else {
      console.error('  ❌ FAIL: evaluatePolicy returned failure');
      return false;
    }
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 5: queryConfig returns null for CEL
  console.log('Test 5: queryConfig() returns null for CEL policies');
  try {
    const yamlPolicy = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: config-test
spec:
  rules:
    - name: test
      category: test
      severity: suggest
      condition: 'false'
      message: "Test"
`;

    const parsed = yaml.load(yamlPolicy);
    const validated = celPolicySetSchema.parse(parsed);
    const evaluator = new MockCelEvaluator(validated, ['test.yaml']);

    const config = await evaluator.queryConfig('test.config', {});

    if (config === null) {
      console.log('  ✅ PASS: queryConfig returns null as expected\n');
    } else {
      console.error('  ❌ FAIL: queryConfig should return null for CEL');
      return false;
    }
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  console.log('=== All Integration Tests Passed! ===\n');
  return true;
}

// Run tests
testIntegration()
  .then((success) => {
    if (success) {
      console.log('✅ Integration proof-of-concept successful!');
      console.log('\nAcceptance Criteria:');
      console.log('  ✅ CEL evaluator can implement RegoEvaluator interface');
      console.log('  ✅ Type checking passes');
      console.log('  ✅ Results structure matches Rego format');
      process.exit(0);
    } else {
      console.error('❌ Integration proof-of-concept failed');
      process.exit(1);
    }
  })
  .catch((error) => {
    console.error('❌ Unexpected error:', error);
    process.exit(1);
  });
