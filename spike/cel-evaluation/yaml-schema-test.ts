/**
 * Phase 0.2: YAML Schema Validation
 *
 * Tests YAML parsing and Zod schema validation for CEL policy format.
 */

import { z } from 'zod';
import yaml from 'js-yaml';

// Define CEL policy schema (as per the plan)
const celPolicyRuleSchema = z.object({
  name: z.string().min(1, 'Rule name required'),
  category: z.string().min(1, 'Category required'),
  severity: z.enum(['block', 'warn', 'suggest'], {
    errorMap: () => ({ message: 'Severity must be block, warn, or suggest' }),
  }),
  priority: z.number().int().min(0).max(100).optional().default(50),
  condition: z.string().min(1, 'Condition expression required'),
  message: z.string().min(1, 'Message required'),
  description: z.string().optional(),
});

const celPolicyMetadataSchema = z.object({
  name: z.string().min(1, 'Policy name required'),
  version: z.string().optional(),
  description: z.string().optional(),
});

const celPolicySpecSchema = z.object({
  rules: z.array(celPolicyRuleSchema).min(1, 'At least one rule required'),
});

const celPolicySetSchema = z.object({
  apiVersion: z.literal('policy.containerization-assist.dev/v1'),
  kind: z.literal('PolicySet'),
  metadata: celPolicyMetadataSchema,
  spec: celPolicySpecSchema,
});

type CelPolicySet = z.infer<typeof celPolicySetSchema>;

async function testYamlSchema() {
  console.log('=== YAML Schema Validation Tests ===\n');

  // Test 1: Valid simple YAML
  console.log('Test 1: Valid simple YAML policy');
  try {
    const sampleYaml = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: test-policy
spec:
  rules:
    - name: require-user
      category: security
      severity: warn
      condition: 'input.content.contains("USER")'
      message: "Dockerfile should specify USER"
`;

    const parsed = yaml.load(sampleYaml);
    const validated = celPolicySetSchema.parse(parsed);

    console.log('  Parsed and validated:', {
      name: validated.metadata.name,
      ruleCount: validated.spec.rules.length,
      firstRule: validated.spec.rules[0].name,
    });
    console.log('  ✅ PASS: Valid YAML parses correctly\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 2: Valid complex YAML with multiple rules
  console.log('Test 2: Complex YAML with multiple rules');
  try {
    const complexYaml = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: security-policy
  version: "1.0.0"
  description: Security best practices
spec:
  rules:
    - name: require-user
      category: security
      severity: block
      priority: 100
      condition: '!input.content.contains("USER")'
      message: "Dockerfile must specify USER directive"
      description: "Running as root is a security risk"

    - name: no-latest-tag
      category: best-practices
      severity: warn
      priority: 80
      condition: 'input.content.contains(":latest")'
      message: "Avoid using :latest tag"

    - name: healthcheck-recommended
      category: health
      severity: suggest
      priority: 60
      condition: '!input.content.contains("HEALTHCHECK")'
      message: "Consider adding HEALTHCHECK"
`;

    const parsed = yaml.load(complexYaml);
    const validated = celPolicySetSchema.parse(parsed);

    console.log('  Parsed and validated:', {
      name: validated.metadata.name,
      version: validated.metadata.version,
      ruleCount: validated.spec.rules.length,
      severities: validated.spec.rules.map((r) => r.severity),
    });
    console.log('  ✅ PASS: Complex YAML with multiple rules works\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 3: Multi-line conditions
  console.log('Test 3: Multi-line conditions');
  try {
    const multilineYaml = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: multiline-test
spec:
  rules:
    - name: complex-check
      category: security
      severity: block
      condition: |
        input.content.contains("FROM") &&
        !input.content.contains("USER root") &&
        input.content.contains("WORKDIR")
      message: "Multi-line condition test"
`;

    const parsed = yaml.load(multilineYaml);
    const validated = celPolicySetSchema.parse(parsed);

    console.log('  Condition:', validated.spec.rules[0].condition);
    console.log('  ✅ PASS: Multi-line conditions supported\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 4: Invalid severity (should fail)
  console.log('Test 4: Invalid severity (should reject)');
  try {
    const invalidYaml = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: invalid-test
spec:
  rules:
    - name: test-rule
      category: security
      severity: critical
      condition: 'true'
      message: "Test"
`;

    const parsed = yaml.load(invalidYaml);
    const validated = celPolicySetSchema.parse(parsed);

    console.error('  ❌ FAIL: Should have rejected invalid severity');
    return false;
  } catch (error) {
    if (error instanceof z.ZodError) {
      console.log('  Expected error:', error.errors[0].message);
      console.log('  ✅ PASS: Zod validation catches invalid severity\n');
    } else {
      console.error('  ❌ FAIL: Unexpected error:', error);
      return false;
    }
  }

  // Test 5: Missing required fields (should fail)
  console.log('Test 5: Missing required fields (should reject)');
  try {
    const missingFieldsYaml = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: missing-test
spec:
  rules:
    - name: incomplete-rule
      category: security
      severity: warn
      # Missing condition and message
`;

    const parsed = yaml.load(missingFieldsYaml);
    const validated = celPolicySetSchema.parse(parsed);

    console.error('  ❌ FAIL: Should have rejected missing fields');
    return false;
  } catch (error) {
    if (error instanceof z.ZodError) {
      console.log('  Expected errors:', error.errors.map((e) => e.path.join('.') + ': ' + e.message));
      console.log('  ✅ PASS: Zod validation catches missing fields\n');
    } else {
      console.error('  ❌ FAIL: Unexpected error:', error);
      return false;
    }
  }

  // Test 6: Wrong apiVersion (should fail)
  console.log('Test 6: Wrong apiVersion (should reject)');
  try {
    const wrongVersionYaml = `
apiVersion: policy.containerization-assist.dev/v2
kind: PolicySet
metadata:
  name: wrong-version
spec:
  rules:
    - name: test
      category: test
      severity: suggest
      condition: 'true'
      message: "Test"
`;

    const parsed = yaml.load(wrongVersionYaml);
    const validated = celPolicySetSchema.parse(parsed);

    console.error('  ❌ FAIL: Should have rejected wrong apiVersion');
    return false;
  } catch (error) {
    if (error instanceof z.ZodError) {
      console.log('  Expected error:', error.errors[0].message);
      console.log('  ✅ PASS: Schema validation catches wrong apiVersion\n');
    } else {
      console.error('  ❌ FAIL: Unexpected error:', error);
      return false;
    }
  }

  // Test 7: Priority bounds validation
  console.log('Test 7: Priority bounds validation');
  try {
    const outOfBoundsPriorityYaml = `
apiVersion: policy.containerization-assist.dev/v1
kind: PolicySet
metadata:
  name: priority-test
spec:
  rules:
    - name: test
      category: test
      severity: suggest
      priority: 150
      condition: 'true'
      message: "Test"
`;

    const parsed = yaml.load(outOfBoundsPriorityYaml);
    const validated = celPolicySetSchema.parse(parsed);

    console.error('  ❌ FAIL: Should have rejected priority > 100');
    return false;
  } catch (error) {
    if (error instanceof z.ZodError) {
      console.log('  Expected error:', error.errors[0].message);
      console.log('  ✅ PASS: Priority bounds validated correctly\n');
    } else {
      console.error('  ❌ FAIL: Unexpected error:', error);
      return false;
    }
  }

  console.log('=== All YAML Schema Tests Passed! ===\n');
  return true;
}

// Run tests
testYamlSchema()
  .then((success) => {
    if (success) {
      console.log('✅ YAML schema validation successful!');
      console.log('\nAcceptance Criteria:');
      console.log('  ✅ YAML parsing works');
      console.log('  ✅ Zod validation catches errors');
      console.log('  ✅ Schema structure feels natural');
      console.log('  ✅ Multi-line conditions supported');
      process.exit(0);
    } else {
      console.error('❌ YAML schema validation failed');
      process.exit(1);
    }
  })
  .catch((error) => {
    console.error('❌ Unexpected error:', error);
    process.exit(1);
  });
