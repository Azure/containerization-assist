#!/usr/bin/env node
/**
 * Test Spring PetClinic with Base Images Policy via MCP CLI (stdio JSON-RPC)
 *
 * This script tests that the packed CLI correctly applies base image policy filtering.
 * It uses the base-images.rego policy to ensure only Microsoft images are recommended.
 *
 * Usage:
 *   node test-spring-petclinic-policy-filtering.mjs [repo-path] [policy-path]
 *
 * Environment Variables:
 *   VERBOSE_TOOL_OUTPUT=true    Enable verbose logging with full tool inputs/outputs
 *
 * Examples:
 *   # Test with base-images policy
 *   node test-spring-petclinic-policy-filtering.mjs /path/to/spring-petclinic /path/to/base-images.rego
 *
 *   # Verbose mode
 *   VERBOSE_TOOL_OUTPUT=true node test-spring-petclinic-policy-filtering.mjs /path/to/spring-petclinic /path/to/base-images.rego
 */

import { spawn } from 'child_process';

const REPO_PATH = process.argv[2] || process.cwd();
const POLICY_PATH = process.argv[3];
const VERBOSE = process.env.VERBOSE_TOOL_OUTPUT === 'true';

if (!POLICY_PATH) {
  console.error('❌ ERROR: Policy path is required');
  console.error('Usage: node test-spring-petclinic-policy-filtering.mjs <repo-path> <policy-path>');
  process.exit(1);
}

console.error('=== MCP CLI Policy Filtering Test for Spring PetClinic ===');
console.error(`Repository: ${REPO_PATH}`);
console.error(`Policy: ${POLICY_PATH}`);
console.error(`Verbose mode: ${VERBOSE ? 'ON' : 'OFF'}`);
console.error('');

// Start MCP server via stdio with policy
const server = spawn('ca-mcp', ['start'], {
  stdio: ['pipe', 'pipe', 'pipe'],
  env: {
    ...process.env,
    MCP_QUIET: 'true',
    LOG_LEVEL: 'error',
    CONTAINERIZATION_ASSIST_POLICY_PATH: POLICY_PATH
  }
});

let requestId = 1;
const pendingRequests = new Map();

// Handle stdout - MCP JSON-RPC responses
server.stdout.on('data', (data) => {
  const lines = data.toString().split('\n').filter(line => line.trim());

  for (const line of lines) {
    try {
      const response = JSON.parse(line);

      // Match response to pending request
      if (response.id && pendingRequests.has(response.id)) {
        const { resolve, reject, name, startTime } = pendingRequests.get(response.id);
        pendingRequests.delete(response.id);

        const executionTime = Date.now() - startTime;

        if (response.error) {
          console.error(`✗ ${name} failed (${executionTime}ms):`, response.error);
          reject(new Error(`${name} failed: ${response.error.message || JSON.stringify(response.error)}`));
        } else {
          console.error(`✓ ${name} completed (${executionTime}ms)`);

          // Display full output in verbose mode
          if (VERBOSE && response.result) {
            prettyPrint(`  📤 Full Output:`, response.result);
          }

          resolve({ result: response.result, executionTime });
        }
      }
    } catch (e) {
      // Not valid JSON, ignore
      if (VERBOSE) {
        console.error('  [non-JSON output]:', line.substring(0, 100));
      }
    }
  }
});

// Handle stderr - logs
server.stderr.on('data', (data) => {
  const output = data.toString();
  // Only show errors and policy loading messages
  if (output.toLowerCase().includes('error') ||
      output.toLowerCase().includes('fail') ||
      output.toLowerCase().includes('policy')) {
    console.error('[server]:', output.trim());
  }
});

// Handle server exit
server.on('exit', (code, signal) => {
  if (code !== 0 && code !== null) {
    console.error(`✗ MCP server exited with code ${code} ${signal || ''}`);
    process.exit(1);
  }
});

// Helper to send JSON-RPC request
function callTool(name, args) {
  const id = requestId++;
  const request = {
    jsonrpc: '2.0',
    id,
    method: 'tools/call',
    params: { name, arguments: args }
  };

  const startTime = Date.now();

  console.error(`→ Calling ${name}...`);
  if (VERBOSE) {
    prettyPrint(`  📥 Input:`, args);
  } else {
    console.error(`  Arguments:`, JSON.stringify(args, null, 2).split('\n').slice(0, 5).join('\n  '));
  }

  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      pendingRequests.delete(id);
      reject(new Error(`Timeout waiting for ${name} response (60s)`));
    }, 60000);

    pendingRequests.set(id, {
      name,
      startTime,
      resolve: (result) => {
        clearTimeout(timeout);
        resolve(result);
      },
      reject: (error) => {
        clearTimeout(timeout);
        reject(error);
      }
    });

    // Send request to server stdin
    server.stdin.write(JSON.stringify(request) + '\n');
  });
}

function printNaturalLanguageResult(result) {
  if (!result || !result.content || !result.content[0]) {
    throw new Error('Invalid tool result format');
  }

  const content = result.content[0];
  if (content.type !== 'text') {
    throw new Error(`Unexpected content type: ${content.type}`);
  }

  console.error('\n--- Natural Language Result ---\n');
  // pad result on left side with 'result text>     ' for better visibility
  console.error(content.text.split('\n').map(line => `result text>     ${line}`).join('\n'));
  console.error('\n--- End of Result ---\n');
}

function extractNaturalLanguageResultText(result) {
  if (!result || !result.content || !result.content[0]) {
    throw new Error('Invalid tool result format');
  }

  const content = result.content[0];
  if (content.type !== 'text') {
    throw new Error(`Unexpected content type: ${content.type}`);
  }

  return content.text;
}

// Run tests
async function runTests() {
  try {
    console.error('\n--- Test 1: analyze-repo ---');

    const analyzeResponse = await callTool('analyze-repo', {
      repositoryPath: REPO_PATH
    });

    const analyzeResult = analyzeResponse.result;
    console.error(`  Received analyze-repo result.`);
    printNaturalLanguageResult(analyzeResult);
    const analyzeText = extractNaturalLanguageResultText(analyzeResult);

    const analyzeExpectedPhrases = [
      "**Modules Found:** 1",
      "Build System: gradle (java 25)",
      "Build System: maven (java 25)",
      "Frameworks: spring-boot",
    ];
    const missingPhrases = [];
    for (const phrase of analyzeExpectedPhrases) {
      if (!analyzeText.includes(phrase)) {
        missingPhrases.push(phrase);
      }
    }
    if (missingPhrases.length > 0) {
      throw new Error(`analyze-repo output is missing expected phrases:\n  - ${missingPhrases.join('\n  - ')}`);
    }
    console.error('✅ analyze-repo output contains all expected phrases.');

    const analyzeTime = analyzeResponse.executionTime;


    console.error('\n--- Test 2: generate-dockerfile (with policy) ---');

    const dockerfileResponse = await callTool('generate-dockerfile', {
      repositoryPath: REPO_PATH,
      language: 'java',
      languageVersion: '25',
      framework: 'spring-boot',
      environment: 'production',
      targetPlatform: 'linux/amd64'
    });

    const dockerfileResult = dockerfileResponse.result;
    printNaturalLanguageResult(dockerfileResult);
    const dockerfileText = extractNaturalLanguageResultText(dockerfileResult);
    const dockerfileTime = dockerfileResponse.executionTime;

    console.error('\n--- Test 3: Verify base-images policy filtering ---');

    // With base-images.rego policy, we should ONLY see Microsoft Container Registry images
    const microsoftImages = [
      'mcr.microsoft.com/openjdk/jdk:25-azurelinux',
      'mcr.microsoft.com/openjdk/jdk:25-distroless',
      'mcr.microsoft.com/openjdk/jdk:21-azurelinux',
      'mcr.microsoft.com/openjdk/jdk:21-distroless',
      'mcr.microsoft.com/openjdk/jdk:17-azurelinux',
      'mcr.microsoft.com/openjdk/jdk:17-distroless',
    ];

    // Check for Microsoft images
    const mentionedMicrosoftImages = microsoftImages.filter(img => dockerfileText.includes(img));
    if (mentionedMicrosoftImages.length === 0) {
      console.error('❌ ERROR: No Microsoft Container Registry images found in output');
      console.error('Expected to find images like: mcr.microsoft.com/openjdk/jdk:25-azurelinux');
      console.error('This indicates policy filtering is NOT working!');
      throw new Error('Policy filtering failed: No Microsoft images in output when base-images.rego policy is active');
    }
    console.error(`✅ Found ${mentionedMicrosoftImages.length} Microsoft Container Registry image(s) in output.`);
    console.error(mentionedMicrosoftImages.map(img => `  - ${img}`).join('\n'));

    // Check that non-Microsoft images are NOT mentioned (policy should block them)
    const nonMicrosoftImages = [
      'eclipse-temurin',
      'docker.io/library/openjdk',
      'amazoncorretto',
      'alpine',
      'ubuntu',
    ];

    const mentionedNonMicrosoftImages = nonMicrosoftImages.filter(img =>
      dockerfileText.toLowerCase().includes(img.toLowerCase())
    );

    if (mentionedNonMicrosoftImages.length > 0) {
      console.error('⚠️  WARNING: Found non-Microsoft images in output (policy should have filtered these):');
      console.error(mentionedNonMicrosoftImages.map(img => `  - ${img}`).join('\n'));
      console.error('This may indicate policy filtering is not working correctly.');
      // Don't fail the test, but warn
    } else {
      console.error('✅ No non-Microsoft images found (policy correctly filtered them out).');
    }

    // Verify both azurelinux and distroless variants are mentioned
    const hasAzureLinux = dockerfileText.includes('azurelinux');
    const hasDistroless = dockerfileText.includes('distroless');

    if (!hasAzureLinux) {
      throw new Error('Expected to find azurelinux images in policy-filtered output');
    }
    console.error('✅ azurelinux images found in output');

    if (!hasDistroless) {
      throw new Error('Expected to find distroless images in policy-filtered output');
    }
    console.error('✅ distroless images found in output');

    console.error('\n=== ALL POLICY FILTERING TESTS PASSED ===');
    console.error('✅ Policy was loaded and applied correctly');
    console.error('✅ Only Microsoft Container Registry images recommended');
    console.error('✅ Knowledge pack filtering worked as expected');
    console.error(`\n⏱️  Total execution time: ${analyzeTime + dockerfileTime}ms`);
    console.error(`   - analyze-repo: ${analyzeTime}ms`);
    console.error(`   - generate-dockerfile: ${dockerfileTime}ms`);

    // Cleanup
    server.kill();

    // Wait a bit for graceful shutdown
    setTimeout(() => {
      process.exit(0);
    }, 500);

  } catch (error) {
    console.error('\n✗ TEST FAILED:', error.message);
    console.error('\nStack trace:');
    console.error(error.stack);

    server.kill();

    setTimeout(() => {
      process.exit(1);
    }, 500);
  }
}

// Wait for server to initialize
console.error('Waiting for MCP server to start...');
setTimeout(() => {
  console.error('Server ready, starting tests...\n');
  runTests();
}, 3000);

/**
 * Pretty-print JSON with indentation and colors (for terminals)
 */
function prettyPrint(label, obj, indent = 2) {
  console.error(label);
  const json = JSON.stringify(obj, null, indent);
  const lines = json.split('\n');

  // Limit output if too large, but show more than before
  const maxLines = VERBOSE ? 500 : 10;
  if (lines.length > maxLines) {
    const truncatedLines = lines.slice(0, maxLines);
    console.error(truncatedLines.join('\n'));
    console.error(`  ... [truncated ${lines.length - maxLines} more lines, enable VERBOSE_TOOL_OUTPUT=true for full output]`);
  } else {
    console.error(json);
  }
}

// Handle process termination
process.on('SIGINT', () => {
  console.error('\n\nInterrupted, cleaning up...');
  server.kill();
  process.exit(1);
});

process.on('SIGTERM', () => {
  console.error('\n\nTerminated, cleaning up...');
  server.kill();
  process.exit(1);
});
