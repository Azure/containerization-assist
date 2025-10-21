/**
 * LLM Integration Quality and Performance Metrics Tests
 * Tests response quality, accuracy, and performance characteristics
 */

import { describe, it, expect, beforeAll, afterAll, beforeEach } from '@jest/globals';
import { ChatClient } from '../infrastructure/chat-client.js';
import { getMCPTestHarness, cleanupMCPTestHarness } from '../infrastructure/mcp-test-harness';
import { metricsCollector, BASIC_PERFORMANCE_EXPECTATIONS } from '../infrastructure/quality-metrics';
import type { LLMTestContext, ToolCall, LLMResponse } from '../infrastructure/llm-client-types';

// Performance benchmark definitions
interface PerformanceBenchmark {
  operation: string;
  maxResponseTime: number;
  minContentLength: number;
  requiredToolCalls?: string[];
}

const PERFORMANCE_BENCHMARKS: PerformanceBenchmark[] = [
  {
    operation: 'simple_dockerfile_validation',
    maxResponseTime: 8000,
    minContentLength: 100,
    requiredToolCalls: ['validate-dockerfile']
  },
  {
    operation: 'complex_workflow_planning',
    maxResponseTime: 15000,
    minContentLength: 300
  }
];
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { promises as fs } from 'node:fs';

// LLM integration tests - always run

describe('LLM Integration Quality and Performance Metrics', () => {
  let testContext: LLMTestContext;
  let testWorkspace: string;

  beforeAll(async () => {

    testWorkspace = await fs.mkdtemp(join(tmpdir(), 'llm-quality-test-'));

    const harness = getMCPTestHarness();

    testContext = await harness.createTestContext({
      serverName: 'quality-test',
      mcpConfig: {
        workingDirectory: testWorkspace,
        enableTools: [
          'analyze-repo',
          'validate-dockerfile',
          'generate-dockerfile',
          'generate-k8s-manifests',
          'build-image',
          'scan',
        ],
      },
    });

    const client = new ChatClient('gpt-4o');

    // Simple validation
    try {
      await client.validateConnection();
      console.log('LLM quality client validated successfully');
      testContext.client = client;
    } catch (error) {
      console.warn('LLM client validation failed:', error);
    }
  }, 30000);

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

  describe('Response Quality Assessment', () => {
    it('should provide high-quality responses for containerization questions', async () => {
      const qualityTestCases = [
        {
          prompt: 'What are the security best practices for writing Dockerfiles?',
          expectedKeywords: ['security', 'user', 'root', 'vulnerability', 'image'],
          minLength: 200,
        },
        {
          prompt: 'How do I optimize Docker image size for a Java application?',
          expectedKeywords: ['multi-stage', 'alpine', 'layer', 'cache', 'size'],
          minLength: 150,
        },
        {
          prompt: 'What\'s the difference between Docker Compose and Kubernetes?',
          expectedKeywords: ['orchestration', 'scale', 'compose', 'kubernetes', 'production'],
          minLength: 180,
        }
      ];

      for (const testCase of qualityTestCases) {
        const startTime = Date.now();

        const response = await testContext.client.continueSession(
          testContext.session,
          testCase.prompt
        );

        const responseTime = Date.now() - startTime;
        const content = response.content.toLowerCase();

        // Quality assertions
        const hasContent = response.content.length >= testCase.minLength;
        const hasToolCalls = response.toolCalls && response.toolCalls.length > 0;

        // Either should have sufficient content OR should have made relevant tool calls
        expect(hasContent || hasToolCalls).toBe(true);
        expect(responseTime).toBeLessThan(20000); // 20 second max (realistic for real LLM APIs)

        // Keyword relevance check
        const keywordMatches = testCase.expectedKeywords.filter(keyword =>
          content.includes(keyword)
        ).length;
        const relevanceScore = keywordMatches / testCase.expectedKeywords.length;

        expect(relevanceScore).toBeGreaterThan(0.4); // At least 40% keyword relevance
      }
    });

    it('should provide contextually appropriate tool recommendations', async () => {
      const toolRecommendationTests = [
        {
          scenario: 'Please validate the Dockerfile at ./Dockerfile for security issues.',
          expectedTool: 'validate-dockerfile',
          shouldCallTool: true,
        },
        {
          scenario: 'Analyze the repository structure at ./my-app to detect the language and framework.',
          expectedTool: 'analyze-repo',
          shouldCallTool: true,
        },
        {
          scenario: 'Generate a Dockerfile for my Node.js project at ./my-app',
          expectedTool: 'generate-dockerfile',
          shouldCallTool: true,
        },
        {
          scenario: 'Generate Kubernetes manifests for myapp with image myapp:latest on port 3000',
          expectedTool: 'generate-k8s-manifests',
          shouldCallTool: true,
        }
      ];

      const harness = getMCPTestHarness();

      for (const test of toolRecommendationTests) {
        const response = await testContext.client.continueSession(
          testContext.session,
          test.scenario
        );

        console.log(`\nTest scenario: "${test.scenario}"`);
        console.log(`Response content: "${response.content}"`);
        console.log(`Tool calls: ${JSON.stringify(response.toolCalls, null, 2)}`);

        if (test.shouldCallTool) {
          expect(response.toolCalls.length).toBeGreaterThan(0);

          const hasExpectedTool = response.toolCalls.some(call =>
            call.name === test.expectedTool
          );
          expect(hasExpectedTool).toBe(true);

          // Execute tool calls and provide responses
          if (response.toolCalls.length > 0) {
            const toolResponses = [];
            for (const toolCall of response.toolCalls) {
              const toolResponse = await harness.executeToolCall('quality-test', toolCall);
              toolResponses.push(toolResponse);
            }

            // Continue conversation with tool responses
            await testContext.client.continueSession(
              testContext.session,
              'What does this result tell us?',
              toolResponses
            );
          }
        }

        // Response should mention the tool or related concepts, OR successfully call expected tool
        if (test.shouldCallTool && response.toolCalls.length > 0) {
          // If expected tool was called, that's sufficient - no additional assertion needed
          // The earlier assertions already validated the expected tool was called
        } else {
          // Otherwise, check that content mentions relevant concepts
          const content = response.content.toLowerCase();
          const toolMentioned = content.includes(test.expectedTool.replace('-', ''));
          const conceptMentioned =
            (test.expectedTool === 'validate-dockerfile' && (content.includes('validate') || content.includes('check'))) ||
            (test.expectedTool === 'analyze-repo' && (content.includes('analyze') || content.includes('repository'))) ||
            (test.expectedTool === 'generate-dockerfile' && (content.includes('dockerfile') || content.includes('generate'))) ||
            (test.expectedTool === 'generate-k8s-manifests' && (content.includes('kubernetes') || content.includes('manifest')));

          expect(toolMentioned || conceptMentioned).toBe(true);
        }
      }
    });
  });

  describe('Performance Benchmarks', () => {
    it('should meet performance benchmarks for common operations', async () => {
      const performanceResults: Array<{
        benchmark: PerformanceBenchmark;
        actualTime: number;
        actualLength: number;
        passed: boolean;
      }> = [];

      // Test simple Dockerfile validation performance
      const harness = getMCPTestHarness();

      const dockerfileValidationStart = Date.now();
      const dockerfileResponse = await testContext.client.continueSession(
        testContext.session,
        'Please validate this Dockerfile: FROM ubuntu:20.04\nRUN apt-get update'
      );

      // Execute any tool calls
      if (dockerfileResponse.toolCalls.length > 0) {
        const toolResponses = [];
        for (const toolCall of dockerfileResponse.toolCalls) {
          const toolResponse = await harness.executeToolCall('quality-test', toolCall);
          toolResponses.push(toolResponse);
        }

        // Continue conversation with tool responses
        await testContext.client.continueSession(
          testContext.session,
          'Based on the validation results, what can you tell me?',
          toolResponses
        );
      }

      const dockerfileValidationTime = Date.now() - dockerfileValidationStart;

      const dockerfileBenchmark = PERFORMANCE_BENCHMARKS.find(b =>
        b.operation === 'simple_dockerfile_validation'
      )!;

      performanceResults.push({
        benchmark: dockerfileBenchmark,
        actualTime: dockerfileValidationTime,
        actualLength: dockerfileResponse.content.length,
        passed: dockerfileValidationTime <= dockerfileBenchmark.maxResponseTime &&
               (dockerfileResponse.content.length >= dockerfileBenchmark.minContentLength ||
                dockerfileResponse.toolCalls.some(call => call.name === 'validate-dockerfile'))
      });

      // Test complex workflow planning performance
      const workflowPlanningStart = Date.now();
      const workflowResponse = await testContext.client.continueSession(
        testContext.session,
        'I need to containerize a microservices architecture with 3 Spring Boot services, 2 React frontends, and a PostgreSQL database. What\'s the complete strategy?'
      );

      // Execute any tool calls from the workflow response
      if (workflowResponse.toolCalls.length > 0) {
        const toolResponses = [];
        for (const toolCall of workflowResponse.toolCalls) {
          const toolResponse = await harness.executeToolCall('quality-test', toolCall);
          toolResponses.push(toolResponse);
        }

        // Continue conversation with tool responses to complete the workflow
        await testContext.client.continueSession(
          testContext.session,
          'Based on these results, what are the key considerations?',
          toolResponses
        );
      }

      const workflowPlanningTime = Date.now() - workflowPlanningStart;

      const workflowBenchmark = PERFORMANCE_BENCHMARKS.find(b =>
        b.operation === 'complex_workflow_planning'
      )!;

      performanceResults.push({
        benchmark: workflowBenchmark,
        actualTime: workflowPlanningTime,
        actualLength: workflowResponse.content.length,
        passed: workflowPlanningTime <= workflowBenchmark.maxResponseTime &&
               (workflowResponse.content.length >= workflowBenchmark.minContentLength ||
                workflowResponse.toolCalls.length > 0) // Accept tool calls as valid response
      });

      // Verify benchmarks
      for (const result of performanceResults) {
        expect(result.actualTime).toBeLessThanOrEqual(result.benchmark.maxResponseTime);

        // For content length, accept either actual content or tool calls as valid responses
        const hasValidResponse = result.actualLength >= result.benchmark.minContentLength ||
                                result.passed; // Use the existing passed logic

        expect(hasValidResponse).toBe(true);

        if (result.benchmark.requiredToolCalls) {
          // This would need to be checked in the context of actual tool calls
        }
      }

      // Log performance metrics for analysis
      console.log('Performance Benchmark Results:', performanceResults);
    });

    it('should handle concurrent requests efficiently', async () => {
      // Create separate sessions for concurrent requests to avoid conversation flow conflicts
      const session1 = testContext.client.createSession();
      const session2 = testContext.client.createSession();
      const session3 = testContext.client.createSession();

      const concurrentPromises = [
        testContext.client.continueSession(session1, 'What is Docker?'),
        testContext.client.continueSession(session2, 'What is Kubernetes?'),
        testContext.client.continueSession(session3, 'What are containers?')
      ];

      const startTime = Date.now();
      const responses = await Promise.all(concurrentPromises);
      const totalTime = Date.now() - startTime;

      // All requests should complete
      expect(responses).toHaveLength(3);
      responses.forEach(response => {
        expect(response.content.length).toBeGreaterThan(30);
      });

      // Concurrent execution should be reasonably fast
      expect(totalTime).toBeLessThan(30000); // 30 second max for 3 concurrent
    });
  });

  describe('Error Handling Quality', () => {
    it('should provide helpful error messages and recovery suggestions', async () => {
      const errorScenarios = [
        {
          prompt: 'Please validate a Dockerfile at /nonexistent/path/Dockerfile',
          expectedErrorHandling: ['not found', 'path', 'check', 'exists'],
        },
        {
          prompt: 'Build an image with tag "invalid@tag@name"',
          expectedErrorHandling: ['invalid', 'tag', 'format', 'naming'],
        }
      ];

      for (const scenario of errorScenarios) {
        const response = await testContext.client.continueSession(
          testContext.session,
          scenario.prompt
        );

        // Should either use tools (which might error) or provide guidance
        if (response.toolCalls.length > 0) {
          const harness = getMCPTestHarness();
          const toolResponses = [];

          for (const toolCall of response.toolCalls) {
            const toolResponse = await harness.executeToolCall('quality-test', toolCall);
            toolResponses.push(toolResponse);
          }

          // Continue conversation with tool responses
          const followupResponse = await testContext.client.continueSession(
            testContext.session,
            'What went wrong? How can I fix this?',
            toolResponses
          );

          // Check if any tool responses had errors
          const hasError = toolResponses.some(tr => tr.error);
          if (hasError) {
            const errorContent = followupResponse.content.toLowerCase();
            const hasHelpfulGuidance = scenario.expectedErrorHandling.some(keyword =>
              errorContent.includes(keyword)
            );

            expect(hasHelpfulGuidance).toBe(true);
            expect(followupResponse.content.length).toBeGreaterThan(50);
          }
        } else {
          // Should provide preemptive guidance
          const content = response.content.toLowerCase();
          const hasPreemptiveGuidance = scenario.expectedErrorHandling.some(keyword =>
            content.includes(keyword)
          );
          expect(hasPreemptiveGuidance).toBe(true);
        }
      }
    });

    it('should maintain conversation flow despite errors', async () => {
      // Trigger an error scenario
      const errorResponse = await testContext.client.continueSession(
        testContext.session,
        'Please validate a Dockerfile at /fake/path/Dockerfile'
      );

      if (errorResponse.toolCalls.length > 0) {
        const harness = getMCPTestHarness();
        const toolResponses = [];

        for (const toolCall of errorResponse.toolCalls) {
          const errorToolResponse = await harness.executeToolCall('quality-test', toolCall);
          toolResponses.push(errorToolResponse);
        }

        await testContext.client.continueSession(
          testContext.session,
          'I see there was an error. Let me try a different approach.',
          toolResponses
        );
      }

      // Continue with normal conversation
      const recoveryResponse = await testContext.client.continueSession(
        testContext.session,
        'Now help me create a basic Dockerfile for a Node.js application'
      );

      expect(recoveryResponse.content).toBeTruthy();
      expect(recoveryResponse.content.length).toBeGreaterThan(50);

      // Session should still be functional
      expect(testContext.session.messages.length).toBeGreaterThan(3);
    });
  });

  describe('Content Accuracy and Relevance', () => {
    it('should provide accurate technical information', async () => {
      const accuracyTests = [
        {
          question: 'What port does a typical Spring Boot application run on by default?',
          expectedAnswers: ['8080'],
          avoidAnswers: ['80', '3000', '443']
        },
        {
          question: 'What is the default base image tag for official Node.js Docker images?',
          expectedAnswers: ['node:', 'alpine', 'lts'],
          avoidAnswers: ['ubuntu', 'centos']
        },
        {
          question: 'What Kubernetes resource type is used to expose a service externally?',
          expectedAnswers: ['service', 'ingress', 'loadbalancer'],
          avoidAnswers: ['pod', 'deployment']
        }
      ];

      for (const test of accuracyTests) {
        const response = await testContext.client.continueSession(
          testContext.session,
          test.question
        );

        const content = response.content.toLowerCase();

        // Should contain at least one expected answer
        const hasExpectedContent = test.expectedAnswers.some(answer =>
          content.includes(answer.toLowerCase())
        );
        expect(hasExpectedContent).toBe(true);

        // Should not contain obviously wrong information
        const hasIncorrectContent = test.avoidAnswers.some(avoid =>
          content.includes(avoid.toLowerCase())
        );
        expect(hasIncorrectContent).toBe(false);
      }
    });

    it('should maintain context awareness across conversation turns', async () => {
      // Establish context
      await testContext.client.continueSession(
        testContext.session,
        'I\'m working on a Java Spring Boot microservices project with 5 services.'
      );

      // Reference previous context
      const contextResponse = await testContext.client.continueSession(
        testContext.session,
        'What\'s the best containerization strategy for the project we just discussed?'
      );

      const content = contextResponse.content.toLowerCase();
      expect(content).toMatch(/(java|spring|microservice|previous|project)/);
      expect(contextResponse.content.length).toBeGreaterThan(100);

      // Further context reference
      const furtherContext = await testContext.client.continueSession(
        testContext.session,
        'How should I handle service-to-service communication in that architecture?'
      );

      expect(furtherContext.content.length).toBeGreaterThan(80);
    });
  });
}); // timeout handled by individual tests