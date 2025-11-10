#!/usr/bin/env node
/**
 * Test Spring PetClinic via MCP CLI (stdio JSON-RPC)
 *
 * This script tests the packed CLI by calling tools via the MCP protocol over stdio.
 * It clones Spring PetClinic, calls analyze-repo and generate-dockerfile tools,
 * and verifies that azurelinux images are recommended.
 *
 * Usage:
 *   node test-spring-petclinic-mcp.mjs [repo-path] [output-dir]
 *
 * Environment Variables:
 *   VERBOSE_TOOL_OUTPUT=true    Enable verbose logging with full tool inputs/outputs
 *
 * Examples:
 *   # Normal mode (concise output)
 *   node test-spring-petclinic-mcp.mjs /path/to/spring-petclinic /tmp
 *
 *   # Verbose mode (full tool outputs)
 *   VERBOSE_TOOL_OUTPUT=true node test-spring-petclinic-mcp.mjs /path/to/spring-petclinic /tmp
 */

import { spawn } from 'child_process';

const REPO_PATH = process.argv[2] || process.cwd();
const OUTPUT_DIR = process.argv[3] || '.';
const VERBOSE = process.env.VERBOSE_TOOL_OUTPUT === 'true';

console.error('=== MCP CLI Test for Spring PetClinic ===');
console.error(`Repository: ${REPO_PATH}`);
console.error(`Output directory: ${OUTPUT_DIR}`);
console.error(`Verbose mode: ${VERBOSE ? 'ON' : 'OFF'}`);
console.error('');

// Start MCP server via stdio
const server = spawn('ca-mcp', ['start'], {
  stdio: ['pipe', 'pipe', 'pipe'],
  env: { ...process.env, MCP_QUIET: 'true', LOG_LEVEL: 'error' }
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
  // Only show errors
  if (output.toLowerCase().includes('error') || output.toLowerCase().includes('fail')) {
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


    console.error('\n--- Test 2: generate-dockerfile ---');

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

    console.error('\n--- Test 3: Verify azurelinux images ---');
    const jdkVersions = ['8', '11', '17', '21', '25'];
    const distroLessImages = jdkVersions.map(v => `mcr.microsoft.com/openjdk/jdk:${v}-distroless`);
    const azureLinuxImages = jdkVersions.map(v => `mcr.microsoft.com/openjdk/jdk:${v}-azurelinux`);

    const mentionedAzureLinuxImages = azureLinuxImages.filter(img => dockerfileText.includes(img));
    if (mentionedAzureLinuxImages.length === 0) {
      throw new Error('No azurelinux images mentioned in generate-dockerfile output');
    }
    console.error(`✅ Found ${mentionedAzureLinuxImages.length} azurelinux image(s) mentioned in output.`);
    console.error(mentionedAzureLinuxImages.map(img => `  - ${img}`).join('\n'));

    const mentionedDistroLessImages = distroLessImages.filter(img => dockerfileText.includes(img));
    if (mentionedDistroLessImages.length === 0) {
      throw new Error('No distroless images mentioned in generate-dockerfile output');
    }
    console.error(`✅ Found ${mentionedDistroLessImages.length} distroless image(s) mentioned in output.`);
    console.error(mentionedDistroLessImages.map(img => `  - ${img}`).join('\n'));

    console.error('\n=== ALL TESTS PASSED ===');
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
