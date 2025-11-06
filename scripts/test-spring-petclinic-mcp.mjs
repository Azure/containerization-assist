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
import { writeFileSync } from 'fs';
import { resolve } from 'path';

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

// Parse MCP tool result content
function parseToolResult(result) {
  if (!result || !result.content || !result.content[0]) {
    throw new Error('Invalid tool result format');
  }

  const content = result.content[0];
  if (content.type !== 'text') {
    throw new Error(`Unexpected content type: ${content.type}`);
  }

  // Try to parse as JSON if possible
  try {
    return JSON.parse(content.text);
  } catch (e) {
    // If not direct JSON, try to extract JSON from markdown code block
    const jsonMatch = content.text.match(/```json\s*\n([\s\S]*?)\n```/);
    if (jsonMatch) {
      try {
        return JSON.parse(jsonMatch[1]);
      } catch (parseError) {
        throw new Error(`Failed to parse JSON from code block: ${parseError.message}`);
      }
    }
    // Return raw text if not JSON
    return { raw: content.text };
  }
}

// Run tests
async function runTests() {
  try {
    console.error('\n--- Test 1: analyze-repo ---');

    const analyzeResponse = await callTool('analyze-repo', {
      repositoryPath: REPO_PATH
    });

    const analyzeResult = analyzeResponse.result;
    const analyzeTime = analyzeResponse.executionTime;

    const analyzeParsed = parseToolResult(analyzeResult);
    const analyzeOutput = resolve(OUTPUT_DIR, 'analyze-result.json');
    writeFileSync(analyzeOutput, JSON.stringify(analyzeResult, null, 2));
    console.error(`  Saved to: ${analyzeOutput}`);

    // Display summary
    displayToolSummary('analyze-repo', analyzeParsed, analyzeTime);

    // Extract module info for next step
    const modules = analyzeParsed.modules || [];
    const firstModule = modules[0] || {};
    const detectedLanguage = firstModule.language || 'java';
    const detectedFramework = firstModule.frameworks?.[0]?.name || 'spring-boot';

    console.error(`\n  🔍 Detected: ${detectedLanguage} (${detectedFramework})`);
    console.error(`  📦 Modules: ${modules.length}`);

    console.error('\n--- Test 2: generate-dockerfile ---');

    const dockerfileResponse = await callTool('generate-dockerfile', {
      repositoryPath: REPO_PATH,
      language: detectedLanguage,
      languageVersion: '21',
      framework: detectedFramework,
      environment: 'production',
      targetPlatform: 'linux/amd64'
    });

    const dockerfileResult = dockerfileResponse.result;
    const dockerfileTime = dockerfileResponse.executionTime;

    const dockerfileParsed = parseToolResult(dockerfileResult);
    const dockerfileOutput = resolve(OUTPUT_DIR, 'generate-dockerfile-result.json');
    writeFileSync(dockerfileOutput, JSON.stringify(dockerfileResult, null, 2));
    console.error(`  Saved to: ${dockerfileOutput}`);

    // Display summary
    displayToolSummary('generate-dockerfile', dockerfileParsed, dockerfileTime);

    // Extract base image recommendations
    const baseImages = dockerfileParsed.recommendations?.baseImages || [];
    console.error(`\n  🐋 Base images recommended: ${baseImages.length}`);

    console.error('\n--- Test 3: Verify azurelinux images ---');

    const azurelinuxVersions = ['25-azurelinux', '21-azurelinux', '17-azurelinux', '11-azurelinux', '8-azurelinux'];
    const foundAzurelinux = baseImages.filter(img =>
      azurelinuxVersions.some(v => img.image && img.image.includes(v))
    );

    if (foundAzurelinux.length === 0) {
      console.error('\n✗ FAILURE: No azurelinux images found in recommendations');
      console.error('\nAll recommended images:');
      baseImages.forEach((img, idx) => {
        console.error(`  ${idx + 1}. ${img.image || 'unknown'}`);
        console.error(`     Category: ${img.category || 'unknown'}`);
        console.error(`     Reason: ${img.reason ? img.reason.substring(0, 80) : 'N/A'}...`);
      });
      throw new Error('No azurelinux images in recommendations');
    }

    console.error(`✓ Found ${foundAzurelinux.length} azurelinux image(s):`);
    foundAzurelinux.forEach((img, idx) => {
      console.error(`  ${idx + 1}. ${img.image}`);
      console.error(`     Category: ${img.category || 'unknown'}`);
    });

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

/**
 * Display tool execution summary
 */
function displayToolSummary(toolName, parsedResult, executionTimeMs) {
  console.error(`\n📊 ${toolName} Summary:`);
  console.error(`  ⏱️  Execution time: ${executionTimeMs}ms`);

  // Tool-specific insights
  if (toolName === 'analyze-repo') {
    const modules = parsedResult.modules || [];
    const summary = parsedResult.summary || '';
    console.error(`  📦 Modules detected: ${modules.length}`);
    if (modules.length > 0) {
      modules.forEach((mod, idx) => {
        const frameworks = mod.frameworks?.map(f => f.name).join(', ') || 'none';
        console.error(`     ${idx + 1}. ${mod.language} (${frameworks})`);
      });
    }
    if (summary) {
      console.error(`  💬 ${summary}`);
    }
  } else if (toolName === 'generate-dockerfile') {
    const baseImages = parsedResult.recommendations?.baseImages || [];
    const dockerfile = parsedResult.dockerfile;
    console.error(`  🐋 Base images recommended: ${baseImages.length}`);
    if (baseImages.length > 0 && VERBOSE) {
      baseImages.slice(0, 5).forEach((img, idx) => {
        console.error(`     ${idx + 1}. ${img.image || 'unknown'} (${img.category || 'unknown'})`);
        if (img.reason) {
          console.error(`        ${img.reason.substring(0, 100)}${img.reason.length > 100 ? '...' : ''}`);
        }
      });
    }
    if (dockerfile && VERBOSE) {
      const lines = dockerfile.split('\n').length;
      console.error(`  📝 Dockerfile generated: ${lines} lines`);
    }
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
