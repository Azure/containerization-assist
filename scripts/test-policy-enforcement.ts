/**
 * Integration Test: Policy Enforcement with fix-dockerfile
 *
 * Tests the complete flow of:
 * 1. Loading built-in Rego policies
 * 2. Running fix-dockerfile on happy/sad test cases
 * 3. Verifying violations are detected correctly
 * 4. Ensuring policy validation passes/fails as expected
 */

import { createToolContext } from '../dist/src/mcp/context.js';
import fixDockerfileTool from '../dist/src/tools/fix-dockerfile/tool.js';
import { createLogger } from '../dist/src/lib/logger.js';
import { readdirSync, readFileSync, writeFileSync } from 'fs';
import { join } from 'path';
import { loadAndMergeRegoPolicies } from '../dist/src/config/policy-rego.js';

const logger = createLogger({ name: 'policy-enforcement-test', level: 'error' });

interface TestCase {
  name: string;
  file: string;
  expectedResult: 'pass' | 'fail' | 'warn';
  expectedViolations?: string[];
  category: 'blocking' | 'warning';
}

interface PolicyRule {
  ruleId: string;
  severity: 'block' | 'warn' | 'suggest';
  policyFile: string;
}

const TEST_CASES: TestCase[] = [
  // Blocking Violations
  {
    name: 'Microsoft Images - Pass',
    file: 'happy/microsoft-images.Dockerfile',
    expectedResult: 'pass',
    category: 'blocking',
  },
  {
    name: 'Microsoft Images - Fail',
    file: 'sad/non-microsoft-images.Dockerfile',
    expectedResult: 'fail',
    expectedViolations: ['require-microsoft-images'],
    category: 'blocking',
  },
  {
    name: 'Root User - Pass',
    file: 'happy/non-root-user.Dockerfile',
    expectedResult: 'pass',
    category: 'blocking',
  },
  {
    name: 'Root User - Fail',
    file: 'sad/root-user.Dockerfile',
    expectedResult: 'fail',
    expectedViolations: ['block-root-user'],
    category: 'blocking',
  },
  {
    name: 'Hardcoded Secrets - Pass',
    file: 'happy/no-secrets.Dockerfile',
    expectedResult: 'pass',
    category: 'blocking',
  },
  {
    name: 'Hardcoded Secrets - Fail',
    file: 'sad/hardcoded-secrets.Dockerfile',
    expectedResult: 'fail',
    expectedViolations: ['block-secrets-in-env'],
    category: 'blocking',
  },
  {
    name: 'Latest Tag - Pass',
    file: 'happy/specific-tags.Dockerfile',
    expectedResult: 'pass',
    category: 'blocking',
  },
  {
    name: 'Latest Tag - Fail',
    file: 'sad/latest-tag.Dockerfile',
    expectedResult: 'fail',
    expectedViolations: ['block-latest-tag'],
    category: 'blocking',
  },
  {
    name: 'Deprecated Node - Pass',
    file: 'happy/modern-node.Dockerfile',
    expectedResult: 'pass',
    category: 'blocking',
  },
  {
    name: 'Deprecated Node - Fail',
    file: 'sad/deprecated-node.Dockerfile',
    expectedResult: 'fail',
    expectedViolations: ['block-deprecated-node'],
    category: 'blocking',
  },
  {
    name: 'Deprecated Python - Pass',
    file: 'happy/modern-python.Dockerfile',
    expectedResult: 'pass',
    category: 'blocking',
  },
  {
    name: 'Deprecated Python - Fail',
    file: 'sad/deprecated-python.Dockerfile',
    expectedResult: 'fail',
    expectedViolations: ['block-deprecated-python'],
    category: 'blocking',
  },
  // Warnings
  {
    name: 'USER Directive - Pass',
    file: 'happy/with-user-directive.Dockerfile',
    expectedResult: 'pass',
    category: 'warning',
  },
  {
    name: 'USER Directive - Warn',
    file: 'sad/missing-user-directive.Dockerfile',
    expectedResult: 'warn',
    expectedViolations: ['require-user-directive'],
    category: 'warning',
  },
  {
    name: 'HEALTHCHECK - Pass',
    file: 'happy/with-healthcheck.Dockerfile',
    expectedResult: 'pass',
    category: 'warning',
  },
  {
    name: 'HEALTHCHECK - Warn',
    file: 'sad/missing-healthcheck.Dockerfile',
    expectedResult: 'warn',
    expectedViolations: ['require-healthcheck'],
    category: 'warning',
  },
  {
    name: 'Apt Upgrade - Pass',
    file: 'happy/no-apt-upgrade.Dockerfile',
    expectedResult: 'pass',
    category: 'warning',
  },
  {
    name: 'Apt Upgrade - Warn',
    file: 'sad/apt-upgrade.Dockerfile',
    expectedResult: 'warn',
    expectedViolations: ['avoid-apt-upgrade'],
    category: 'warning',
  },
  {
    name: 'Alpine Variant - Pass',
    file: 'happy/alpine-variant.Dockerfile',
    expectedResult: 'pass',
    category: 'warning',
  },
  {
    name: 'Alpine Variant - Warn',
    file: 'sad/non-alpine-variant.Dockerfile',
    expectedResult: 'warn',
    expectedViolations: ['recommend-alpine'],
    category: 'warning',
  },
  {
    name: 'Base Image Size - Pass',
    file: 'happy/appropriate-size.Dockerfile',
    expectedResult: 'pass',
    category: 'warning',
  },
  {
    name: 'Base Image Size - Warn',
    file: 'sad/oversized-base.Dockerfile',
    expectedResult: 'warn',
    expectedViolations: ['block-oversized-base'],
    category: 'warning',
  },
  {
    name: 'WORKDIR - Pass',
    file: 'happy/with-workdir.Dockerfile',
    expectedResult: 'pass',
    category: 'warning',
  },
  {
    name: 'WORKDIR - Warn',
    file: 'sad/missing-workdir.Dockerfile',
    expectedResult: 'warn',
    expectedViolations: ['require-workdir'],
    category: 'warning',
  },
  {
    name: 'Sudo Usage - Pass',
    file: 'happy/no-sudo.Dockerfile',
    expectedResult: 'pass',
    category: 'warning',
  },
  {
    name: 'Sudo Usage - Warn',
    file: 'sad/with-sudo.Dockerfile',
    expectedResult: 'warn',
    expectedViolations: ['avoid-sudo'],
    category: 'warning',
  },
];

/**
 * Extract all rule IDs from Rego policy files
 * Filters out Kubernetes-only rules (input_type == "kubernetes")
 */
function extractPolicyRules(policyFiles: string[]): PolicyRule[] {
  const rules: PolicyRule[] = [];

  for (const policyFile of policyFiles) {
    const content = readFileSync(policyFile, 'utf-8');
    const fileName = policyFile.split('/').pop() || '';

    // Split content into rule blocks to analyze each rule's context
    // Match: violations/warnings/suggestions contains result if { ... }
    const ruleBlockPattern = /(violations|warnings|suggestions)\s+contains\s+result\s+if\s*\{([^}]*\{[^}]*\}[^}]*)\}/gs;
    
    let blockMatch;
    while ((blockMatch = ruleBlockPattern.exec(content)) !== null) {
      const ruleBlock = blockMatch[0];
      
      // Skip Kubernetes-only rules (those with input_type == "kubernetes")
      if (ruleBlock.includes('input_type == "kubernetes"')) {
        continue;
      }
      
      // Extract rule ID and severity from this block
      const ruleIdMatch = ruleBlock.match(/"rule":\s*"([^"]+)"/);
      const severityMatch = ruleBlock.match(/"severity":\s*"(block|warn|suggest)"/);
      
      if (ruleIdMatch && severityMatch) {
        const [, ruleId] = ruleIdMatch;
        const [, severity] = severityMatch;
        
        rules.push({
          ruleId,
          severity: severity as 'block' | 'warn' | 'suggest',
          policyFile: fileName,
        });
      }
    }
  }

  return rules;
}

/**
 * Validate that all policy rules have corresponding test cases
 */
function validateTestCoverage(policyRules: PolicyRule[], testCases: TestCase[]): {
  covered: string[];
  missing: Array<{ ruleId: string; severity: string; policyFile: string }>;
} {
  const coveredRules = new Set<string>();
  const missing: Array<{ ruleId: string; severity: string; policyFile: string }> = [];

  // Extract all rule IDs being tested
  testCases.forEach((tc) => {
    if (tc.expectedViolations) {
      tc.expectedViolations.forEach((ruleId) => coveredRules.add(ruleId));
    }
  });

  // Check which rules are missing test coverage
  for (const rule of policyRules) {
    // Skip suggestion rules (they're optional and don't block/warn)
    if (rule.severity === 'suggest') {
      continue;
    }

    if (!coveredRules.has(rule.ruleId)) {
      missing.push({
        ruleId: rule.ruleId,
        severity: rule.severity,
        policyFile: rule.policyFile,
      });
    }
  }

  return {
    covered: Array.from(coveredRules),
    missing,
  };
}

async function main() {
  console.log('🚀 Testing policy enforcement with fix-dockerfile...\n');

  const fixturesDir = join(process.cwd(), 'test/fixtures/policy-validation');

  // Step 1: Load built-in policies
  console.log('Step 1: Loading built-in Rego policies...');
  const policiesDir = join(process.cwd(), 'policies');
  const policyFiles = readdirSync(policiesDir)
    .filter((f) => f.endsWith('.rego') && !f.endsWith('_test.rego'))
    .map((f) => join(policiesDir, f));

  console.log(`   Found ${policyFiles.length} policy files:`);
  policyFiles.forEach((p) => console.log(`   - ${p.split('/').pop()}`));

  const policyResult = await loadAndMergeRegoPolicies(policyFiles, logger);
  if (!policyResult.ok) {
    console.error('❌ Failed to load policies:', policyResult.error);
    process.exit(1);
  }
  console.log('✅ Policies loaded successfully\n');

  const policy = policyResult.value;
  const ctx = createToolContext(logger, { policy });

  // Step 2: Validate test coverage
  console.log('Step 2: Validating test coverage for all policy rules...');
  const policyRules = extractPolicyRules(policyFiles);
  console.log(`   Found ${policyRules.length} total rules in policy files`);

  const blockingRules = policyRules.filter((r) => r.severity === 'block');
  const warningRules = policyRules.filter((r) => r.severity === 'warn');
  const suggestionRules = policyRules.filter((r) => r.severity === 'suggest');

  console.log(`   - Blocking rules: ${blockingRules.length}`);
  console.log(`   - Warning rules: ${warningRules.length}`);
  console.log(`   - Suggestion rules: ${suggestionRules.length} (optional coverage)`);

  const coverage = validateTestCoverage(policyRules, TEST_CASES);

  console.log(`   - Rules covered by tests: ${coverage.covered.length}`);
  console.log(`   - Rules missing tests: ${coverage.missing.length}`);

  if (coverage.missing.length > 0) {
    console.log('\n❌ ERROR: Missing test coverage for the following rules:\n');
    coverage.missing.forEach((rule) => {
      console.log(`   - ${rule.ruleId} [${rule.severity}] in ${rule.policyFile}`);
      console.log(`     → Add test case with expectedViolations: ['${rule.ruleId}']`);
    });
    console.log('\n💡 To fix: Add happy/sad Dockerfiles and test cases for missing rules\n');
    process.exit(1);
  }

  console.log('✅ All blocking and warning rules have test coverage\n');

  // Step 3: Run tests
  console.log('Step 3: Running policy enforcement tests...\n');

  const results: Array<{
    name: string;
    passed: boolean;
    message: string;
    category: string;
  }> = [];

  let passCount = 0;
  let failCount = 0;

  for (const testCase of TEST_CASES) {
    const dockerfilePath = join(fixturesDir, testCase.file);
    
    process.stdout.write(`   Testing: ${testCase.name}... `);

    try {
      const result = await fixDockerfileTool.handler(
        {
          path: dockerfilePath,
          targetPlatform: 'linux/amd64',
          strictPlatformValidation: false,
        },
        ctx
      );

      if (!result.ok) {
        console.log('❌ TOOL ERROR');
        results.push({
          name: testCase.name,
          passed: false,
          message: `Tool execution failed: ${result.error}`,
          category: testCase.category,
        });
        failCount++;
        continue;
      }

      const report = result.value as any;
      const policyValidation = report.policyValidation;

      if (!policyValidation) {
        console.log('❌ NO POLICY VALIDATION');
        results.push({
          name: testCase.name,
          passed: false,
          message: 'Policy validation result not found in report',
          category: testCase.category,
        });
        failCount++;
        continue;
      }

      // Validate result matches expectations
      let testPassed = false;
      let message = '';

      if (testCase.expectedResult === 'pass') {
        // Should have no blocking violations (and no warnings if testing warning category)
        const hasBlockingViolations = policyValidation.violations.length > 0;
        const hasWarnings = policyValidation.warnings.length > 0;
        
        if (testCase.category === 'warning') {
          // For warning tests, check both violations and warnings
          testPassed = !hasBlockingViolations && !hasWarnings;
          if (!testPassed) {
            if (hasBlockingViolations) {
              message = `Unexpected violations: ${policyValidation.violations.map((v: any) => v.ruleId).join(', ')}`;
            } else {
              message = `Unexpected warnings: ${policyValidation.warnings.map((v: any) => v.ruleId).join(', ')}`;
            }
          } else {
            message = 'Passed validation as expected';
          }
        } else {
          // For blocking tests, only check violations
          testPassed = !hasBlockingViolations;
          message = testPassed
            ? 'Passed validation as expected'
            : `Unexpected violations: ${policyValidation.violations.map((v: any) => v.ruleId).join(', ')}`;
        }
      } else if (testCase.expectedResult === 'fail') {
        // Should have blocking violations
        const hasBlockingViolations = policyValidation.violations.length > 0;
        testPassed = hasBlockingViolations;

        if (testPassed && testCase.expectedViolations) {
          // Verify expected violations are present
          const actualViolations = policyValidation.violations.map((v: any) => v.ruleId);
          const hasExpectedViolations = testCase.expectedViolations.every((expected) =>
            actualViolations.includes(expected)
          );
          testPassed = hasExpectedViolations;
          message = hasExpectedViolations
            ? `Failed with expected violations: ${actualViolations.join(', ')}`
            : `Missing expected violations. Expected: ${testCase.expectedViolations.join(', ')}, Got: ${actualViolations.join(', ')}`;
        } else {
          message = 'Failed validation as expected';
        }
      } else if (testCase.expectedResult === 'warn') {
        // Should have warnings but no blocking violations
        const hasWarnings = policyValidation.warnings.length > 0;
        const hasBlockingViolations = policyValidation.violations.length > 0;
        testPassed = hasWarnings && !hasBlockingViolations;

        if (testPassed && testCase.expectedViolations) {
          // Verify expected warnings are present
          const actualWarnings = policyValidation.warnings.map((w: any) => w.ruleId);
          const hasExpectedWarnings = testCase.expectedViolations.every((expected) =>
            actualWarnings.includes(expected)
          );
          testPassed = hasExpectedWarnings;
          message = hasExpectedWarnings
            ? `Warned as expected: ${actualWarnings.join(', ')}`
            : `Missing expected warnings. Expected: ${testCase.expectedViolations.join(', ')}, Got: ${actualWarnings.join(', ')}`;
        } else {
          message = 'Warned as expected';
        }
      }

      if (testPassed) {
        console.log('✅');
        passCount++;
      } else {
        console.log('❌');
        failCount++;
      }

      results.push({
        name: testCase.name,
        passed: testPassed,
        message,
        category: testCase.category,
      });
    } catch (error) {
      console.log('❌ EXCEPTION');
      results.push({
        name: testCase.name,
        passed: false,
        message: `Exception: ${error instanceof Error ? error.message : String(error)}`,
        category: testCase.category,
      });
      failCount++;
    }
  }

  // Step 4: Print summary
  console.log('\n' + '='.repeat(80));
  console.log('Test Results Summary');
  console.log('='.repeat(80));

  console.log(`\n✅ Passed: ${passCount}/${TEST_CASES.length}`);
  console.log(`❌ Failed: ${failCount}/${TEST_CASES.length}`);

  // Group by category
  const blockingResults = results.filter((r) => r.category === 'blocking');
  const warningResults = results.filter((r) => r.category === 'warning');

  console.log('\nBlocking Violations:');
  blockingResults.forEach((r) => {
    const icon = r.passed ? '✅' : '❌';
    console.log(`  ${icon} ${r.name}: ${r.message}`);
  });

  console.log('\nWarnings:');
  warningResults.forEach((r) => {
    const icon = r.passed ? '✅' : '❌';
    console.log(`  ${icon} ${r.name}: ${r.message}`);
  });

  // Write results to file for GitHub Actions
  const resultsJson = JSON.stringify(
    {
      total: TEST_CASES.length,
      passed: passCount,
      failed: failCount,
      results,
      coverage: {
        totalRules: policyRules.length,
        blockingRules: blockingRules.length,
        warningRules: warningRules.length,
        suggestionRules: suggestionRules.length,
        coveredRules: coverage.covered.length,
        missingRules: coverage.missing.length,
        // Include detailed rule information for dynamic display
        blockingRuleDetails: blockingRules.map(r => ({
          ruleId: r.ruleId,
          policyFile: r.policyFile,
          tested: coverage.covered.includes(r.ruleId)
        })),
        warningRuleDetails: warningRules.map(r => ({
          ruleId: r.ruleId,
          policyFile: r.policyFile,
          tested: coverage.covered.includes(r.ruleId)
        })),
        policyFiles: policyFiles.map(p => p.split('/').pop()),
      },
    },
    null,
    2
  );
  writeFileSync('policy-test-results.json', resultsJson);
  console.log('\n📄 Results written to policy-test-results.json');

  // Cleanup resources before exiting
  policy.close();

  // Exit with appropriate code
  if (failCount > 0) {
    console.log('\n❌ Some tests failed!');
    process.exit(1);
  } else {
    console.log('\n🎉 All tests passed!');
  }
}

main().catch((error) => {
  console.error('❌ Test execution failed:', error);
  process.exit(1);
});
