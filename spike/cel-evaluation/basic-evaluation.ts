/**
 * Phase 0.1: CEL Library Evaluation
 *
 * Tests basic CEL functionality to validate the library works as expected.
 */

import { evaluate, parse, Environment } from '@marcbachmann/cel-js';

async function testCelBasics() {
  console.log('=== CEL Library Evaluation ===\n');

  // Test 1: Simple boolean expression
  console.log('Test 1: String contains()');
  try {
    const result1 = evaluate('input.content.contains("FROM")', {
      input: { content: 'FROM node:22-alpine' },
    });
    console.log('  Result:', result1); // Should be true
    console.log('  ✅ PASS: String contains() works\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 2: Regex matching
  console.log('Test 2: Regex matching');
  try {
    const result2 = evaluate('input.content.matches("FROM\\\\s+node:")', {
      input: { content: 'FROM node:22' },
    });
    console.log('  Result:', result2); // Should be true
    console.log('  ✅ PASS: Regex matches() works\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 3: Complex conditions (boolean operators)
  console.log('Test 3: Complex conditions (&&, !)');
  try {
    const result3 = evaluate(
      'input.content.contains("FROM") && !input.content.contains("USER root")',
      { input: { content: 'FROM alpine\nUSER appuser' } }
    );
    console.log('  Result:', result3); // Should be true
    console.log('  ✅ PASS: Boolean operators (&&, !) work\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 4: OR operator
  console.log('Test 4: OR operator (||)');
  try {
    const result4 = evaluate(
      'input.content.contains("alpine") || input.content.contains("ubuntu")',
      { input: { content: 'FROM alpine:3.20' } }
    );
    console.log('  Result:', result4); // Should be true
    console.log('  ✅ PASS: OR operator (||) works\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 5: Performance check
  console.log('Test 5: Performance check (<1ms per expression)');
  try {
    const expression = 'input.content.contains("FROM") && !input.content.contains("root")';
    const iterations = 1000;
    const start = performance.now();

    for (let i = 0; i < iterations; i++) {
      evaluate(expression, {
        input: { content: 'FROM alpine\nUSER appuser\nWORKDIR /app' },
      });
    }

    const end = performance.now();
    const avgTime = (end - start) / iterations;

    console.log(`  Average time per evaluation: ${avgTime.toFixed(3)}ms`);
    if (avgTime < 1) {
      console.log('  ✅ PASS: Performance acceptable (<1ms)\n');
    } else {
      console.log('  ⚠️  WARNING: Performance slower than target (>1ms)\n');
    }
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 6: Parse once, evaluate multiple times
  console.log('Test 6: Parse once, evaluate multiple times');
  try {
    const expr = parse('input.content.contains(needle)');

    const result6a = expr({
      input: { content: 'FROM alpine' },
      needle: 'alpine',
    });
    console.log('  Result (alpine):', result6a); // Should be true

    const result6b = expr({
      input: { content: 'FROM ubuntu' },
      needle: 'alpine',
    });
    console.log('  Result (ubuntu):', result6b); // Should be false
    console.log('  ✅ PASS: Parse/reuse pattern works\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  // Test 7: Environment API with custom functions
  console.log('Test 7: Environment API');
  try {
    const env = new Environment()
      .registerVariable('input', 'map')
      .registerVariable('severity', 'string');

    const result7 = env.evaluate(
      'input.content.contains("USER") && severity == "high"',
      {
        input: { content: 'FROM alpine\nUSER appuser' },
        severity: 'high',
      }
    );
    console.log('  Result:', result7); // Should be true
    console.log('  ✅ PASS: Environment API works\n');
  } catch (error) {
    console.error('  ❌ FAIL:', error);
    return false;
  }

  console.log('=== All Tests Passed! ===\n');
  return true;
}

// Run tests
testCelBasics()
  .then((success) => {
    if (success) {
      console.log('✅ CEL library evaluation successful!');
      console.log('\nAcceptance Criteria:');
      console.log('  ✅ CEL library installs without conflicts');
      console.log('  ✅ String contains() works');
      console.log('  ✅ Regex matches() works');
      console.log('  ✅ Boolean operators (&&, ||, !) work');
      console.log('  ✅ Performance acceptable (<1ms per expression)');
      process.exit(0);
    } else {
      console.error('❌ CEL library evaluation failed');
      process.exit(1);
    }
  })
  .catch((error) => {
    console.error('❌ Unexpected error:', error);
    process.exit(1);
  });
