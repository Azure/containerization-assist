/**
 * Single Tool Interaction Tests
 * Tests individual tool usage with real LLM interactions
 */

import { describe, it, expect, beforeAll, afterAll, beforeEach } from '@jest/globals';
import { ChatClient } from '../infrastructure/chat-client.js';
import { getMCPTestHarness, cleanupMCPTestHarness } from '../infrastructure/mcp-test-harness';
import { createProjectFixture, SPRING_BOOT_REST_API, PROBLEMATIC_DOCKERFILE } from '../fixtures/spring-boot-app';
import type { LLMClient, LLMTestContext, ToolCall } from '../infrastructure/llm-client-types';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { promises as fs } from 'node:fs';

// LLM integration tests - always run

describe('Single Tool LLM Integration Tests', () => {
  let testContext: LLMTestContext;
  let testWorkspace: string;

  beforeAll(async () => {

    // Create test workspace
    testWorkspace = await fs.mkdtemp(join(tmpdir(), 'llm-tool-test-'));

    // Get MCP test harness
    const harness = getMCPTestHarness();

    // Create test context with real server
    testContext = await harness.createTestContext({
      serverName: 'single-tool-test',
      mcpConfig: {
        workingDirectory: testWorkspace,
        // Enable specific tools for focused testing
        enableTools: [
          'analyze-repo',
          'validate-dockerfile',
          'generate-dockerfile',
          'build-image',
          'generate-k8s-manifests',
          'scan',
        ],
      },
    });

    // Validate the LLM client
    const client = new ChatClient('gpt-4o');

    try {
      await client.validateConnection();
      console.log('LLM client validated successfully');
      testContext.client = client;
    } catch (error) {
      console.warn('LLM client validation failed:', error);
    }
  }, 30000); // 30 second timeout for setup

  afterAll(async () => {
    await cleanupMCPTestHarness();

    if (testWorkspace) {
      await fs.rm(testWorkspace, { recursive: true, force: true });
    }
  });

  beforeEach(() => {
    // Create a fresh session for each test to avoid conversation flow conflicts
    if (testContext?.client) {
      testContext.session = testContext.client.createSession();
    }
  });

  describe('validate-dockerfile tool', () => {
    it('should successfully validate a good Dockerfile through LLM interaction', async () => {
      // Create Spring Boot project with proper Dockerfile
      const projectDir = await createProjectFixture(SPRING_BOOT_REST_API, testWorkspace);

      // Write a good Dockerfile
      const dockerfileContent = SPRING_BOOT_REST_API.expectedDockerfile!;
      await fs.writeFile(join(projectDir, 'Dockerfile'), dockerfileContent);

      // Interact with LLM to validate Dockerfile
      const response = await testContext.client.continueSession(
        testContext.session,
        `Please validate the Dockerfile in the directory ${projectDir}. Check for security best practices and optimization opportunities.`,
      );

      expect(response.finishReason).toBe('tool_calls');
      expect(response.toolCalls).toHaveLength(1);

      const toolCall = response.toolCalls[0];
      expect(toolCall.name).toBe('validate-dockerfile');
      expect(toolCall.arguments).toHaveProperty('path');

      // Execute the tool call through MCP
      const harness = getMCPTestHarness();
      const toolResponse = await harness.executeToolCall('single-tool-test', toolCall);

      expect(toolResponse.error).toBeUndefined();
      expect(toolResponse.content).toBeDefined();

      // Continue conversation with tool result
      const followUp = await testContext.client.continueSession(
        testContext.session,
        'What did the validation show? Are there any issues?',
        [toolResponse]
      );

      expect(followUp.content).toBeTruthy();
      expect(followUp.content.toLowerCase()).toContain('dockerfile');

      // Verify the session captured the interaction correctly
      expect(testContext.session.toolCalls).toHaveLength(1);
      expect(testContext.session.toolResponses).toHaveLength(1);
    });

    it('should identify security issues in problematic Dockerfile', async () => {
      // Create project with problematic Dockerfile
      const projectDir = await createProjectFixture(PROBLEMATIC_DOCKERFILE, testWorkspace);

      const response = await testContext.client.continueSession(
        testContext.session,
        `Please analyze the Dockerfile in ${projectDir} for security vulnerabilities and best practices. I'm particularly concerned about security issues.`,
      );

      expect(response.toolCalls).toHaveLength(1);

      const toolCall = response.toolCalls[0];
      expect(toolCall.name).toBe('validate-dockerfile');

      const harness = getMCPTestHarness();
      const toolResponse = await harness.executeToolCall('single-tool-test', toolCall);

      // Continue with tool result to get analysis
      const analysis = await testContext.client.continueSession(
        testContext.session,
        'What security issues did you find? Please provide specific recommendations.',
        [toolResponse]
      );

      // LLM should identify common security issues
      const analysisText = analysis.content.toLowerCase();
      expect(
        analysisText.includes('root') ||
        analysisText.includes('user') ||
        analysisText.includes('security')
      ).toBe(true);
    });
  });

  describe('build-image tool', () => {
    it('should build Docker image through LLM interaction', async () => {
      const projectDir = await createProjectFixture(SPRING_BOOT_REST_API, testWorkspace);

      // Write Dockerfile
      await fs.writeFile(join(projectDir, 'Dockerfile'), SPRING_BOOT_REST_API.expectedDockerfile!);

      const response = await testContext.client.continueSession(
        testContext.session,
        `I have a Spring Boot project ready in ${projectDir} with a working Dockerfile. Please build a Docker image from the existing Dockerfile and tag it as 'spring-demo:test'.`,
      );

      expect(response.toolCalls.length).toBeGreaterThan(0);

      // Debug: Log what tools are actually being called
      console.log('Build test response:', JSON.stringify({
        content: response.content,
        toolCalls: response.toolCalls.map(call => ({ name: call.name, arguments: call.arguments }))
      }, null, 2));

      // Should call build-image tool
      const buildToolCall = response.toolCalls.find(call => call.name === 'build-image');
      expect(buildToolCall).toBeDefined();
      expect(buildToolCall!.arguments).toHaveProperty('tag');

      const harness = getMCPTestHarness();
      const toolResponse = await harness.executeToolCall('single-tool-test', buildToolCall!);

      // Get LLM's interpretation of the build result
      const followUp = await testContext.client.continueSession(
        testContext.session,
        'Was the build successful? What\'s the next step?',
        [toolResponse]
      );

      expect(followUp.content).toBeTruthy();
    });
  });

  describe('generate-k8s-manifests tool', () => {
    it('should generate Kubernetes manifests through LLM interaction', async () => {
      const response = await testContext.client.continueSession(
        testContext.session,
        'I need to deploy my Spring Boot application "demo-api" to Kubernetes. The image is "demo-api:v1.0" and it runs on port 8080. Please create the necessary Kubernetes manifests.',
      );

      expect(response.toolCalls.length).toBeGreaterThan(0);

      const manifestCall = response.toolCalls.find(call => call.name === 'generate-k8s-manifests');
      expect(manifestCall).toBeDefined();
      expect(manifestCall!.arguments).toMatchObject({
        appName: expect.stringMatching(/demo/i),
        imageName: expect.stringMatching(/demo-api/i),
      });

      const harness = getMCPTestHarness();
      const toolResponse = await harness.executeToolCall('single-tool-test', manifestCall!);

      const followUp = await testContext.client.continueSession(
        testContext.session,
        'What Kubernetes resources were created? How do I deploy this?',
        [toolResponse]
      );

      expect(followUp.content.toLowerCase()).toContain('kubernetes');
    });
  });

  describe('Multi-step workflow coordination', () => {
    it('should handle a complete containerization workflow', async () => {
      const projectDir = await createProjectFixture(SPRING_BOOT_REST_API, testWorkspace);

      // Start with a comprehensive request that should trigger multiple tools
      const response = await testContext.client.continueSession(
        testContext.session,
        `I have a Spring Boot project in ${projectDir}. I need to:
1. Create an optimized, secure Dockerfile
2. Build a Docker image tagged as 'complete-demo:v1'
3. Create Kubernetes deployment manifests for production

Please help me complete this entire workflow step by step.`,
      );

      // Should trigger multiple tool calls or at least start the workflow
      expect(response.toolCalls.length).toBeGreaterThan(0);

      // Log what tool calls the LLM decided to make
      console.log(`🤖 LLM decided to call ${response.toolCalls.length} tool(s):`);
      response.toolCalls.forEach((toolCall, index) => {
        console.log(`  ${index + 1}. ${toolCall.name} with:`, JSON.stringify(toolCall.arguments, null, 4));
      });
      console.log('═'.repeat(80));

      // Execute all tool calls
      const harness = getMCPTestHarness();
      const toolResponses = [];

      for (const toolCall of response.toolCalls) {
        const toolResponse = await harness.executeToolCall('single-tool-test', toolCall);
        toolResponses.push(toolResponse);
      }

      // Get LLM's next steps with tool responses
      const nextSteps = await testContext.client.continueSession(
        testContext.session,
        'Based on the tool results above, what should I do next to complete the deployment? Please provide specific next steps.',
        toolResponses
      );

      // If content is empty but we have tool calls, that might still be valid workflow behavior
      const hasContent = nextSteps.content && nextSteps.content.trim().length > 0;
      const hasNewToolCalls = nextSteps.toolCalls && nextSteps.toolCalls.length > 0;

      expect(hasContent || hasNewToolCalls).toBe(true);

      // Verify the session captured the complete workflow
      expect(testContext.session.toolCalls.length).toBeGreaterThan(0);
      expect(testContext.session.messages.length).toBeGreaterThan(2); // At least user + assistant messages
    });
  });

  describe('Error handling and recovery', () => {
    it('should gracefully handle tool execution errors', async () => {
      // Try to validate a non-existent Dockerfile
      const response = await testContext.client.continueSession(
        testContext.session,
        'Please validate the Dockerfile in /nonexistent/path/Dockerfile',
      );

      if (response.toolCalls.length > 0) {
        const toolCall = response.toolCalls[0];
        const harness = getMCPTestHarness();
        const toolResponse = await harness.executeToolCall('single-tool-test', toolCall);

        // LLM should handle the result gracefully (whether error or success)
        const errorHandling = await testContext.client.continueSession(
          testContext.session,
          'What went wrong? How can I fix this?',
          [toolResponse]
        );

        expect(errorHandling.content).toBeTruthy();
        // Should mention relevant concepts regardless of whether tool succeeded or failed
        expect(errorHandling.content.toLowerCase()).toMatch(/(error|file|path|dockerfile|not|found|check|exist)/);
      } else {
        // If no tool calls, LLM should still provide helpful guidance
        expect(response.content.toLowerCase()).toMatch(/(error|file|path|dockerfile|not|found|check|exist)/);
      }
    });
  });

  describe('Performance and response quality', () => {
    it('should complete tool interactions within reasonable time limits', async () => {
      const startTime = Date.now();

      const response = await testContext.client.sendMessage([
        { role: 'user', content: 'List the available containerization tools and their purposes.' }
      ], {
        tools: testContext.mcpServer.tools,
      });

      const endTime = Date.now();
      const responseTime = endTime - startTime;

      // Should respond within 10 seconds (adjust based on actual LLM performance)
      expect(responseTime).toBeLessThan(10000);

      // Response should be informative
      expect(response.content.length).toBeGreaterThan(50);
    });

    it('should provide contextually appropriate tool calls', async () => {
      const response = await testContext.client.continueSession(
        testContext.session,
        'I need to scan my Docker image "myapp:latest" for security vulnerabilities. Please run a security scan.',
      );

      console.log('Scan test response:', JSON.stringify(response, null, 2));

      // Should call the scan tool specifically
      expect(response.toolCalls.length).toBeGreaterThan(0);
      const scanCall = response.toolCalls.find(call => call.name === 'scan');
      expect(scanCall).toBeDefined();

      // Check that arguments contain the image reference (might be different field names)
      const scanArgs = scanCall!.arguments;
      const hasImageRef =
        scanArgs.imageTag === 'myapp:latest' ||
        scanArgs.imageName === 'myapp:latest' ||
        scanArgs.image === 'myapp:latest' ||
        Object.values(scanArgs).some(val => String(val).includes('myapp:latest'));

      expect(hasImageRef).toBe(true);
    });
  });
}); // timeout handled by individual tests