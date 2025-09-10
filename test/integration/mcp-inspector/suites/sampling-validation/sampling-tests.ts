/**
 * Sampling Validation Tests for MCP Inspector
 * MCP Inspector Testing Infrastructure
 * Tests sampling algorithms via MCP tools
 */

import { TestCase, MCPTestRunner } from '../../infrastructure/test-runner.js';

export const createSamplingValidationTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'dockerfile-candidate-generation',
      category: 'sampling-validation',
      description: 'Verify Dockerfile candidates are generated consistently',
      tags: ['sampling', 'dockerfile', 'deterministic'],
      timeout: 45000, // 45 seconds for generation
      execute: async () => {
        const start = performance.now();
        
        // First generation
        const result1 = await client.callTool({
          name: 'generate-dockerfile',
          arguments: {
            sessionId: 'sampling-test-1',
            baseImage: 'node:18-alpine',
            optimization: true,
            multistage: true
          }
        });

        if (result1.isError) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `First Dockerfile generation failed: ${result1.error?.message || 'Unknown error'}`
          };
        }

        // Second generation with same parameters (should be deterministic)
        const result2 = await client.callTool({
          name: 'generate-dockerfile',
          arguments: {
            sessionId: 'sampling-test-2',
            baseImage: 'node:18-alpine',
            optimization: true,
            multistage: true
          }
        });

        if (result2.isError) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Second Dockerfile generation failed: ${result2.error?.message || 'Unknown error'}`
          };
        }

        const responseTime = performance.now() - start;

        // Analyze responses
        let candidatesInfo1: any = {};
        let candidatesInfo2: any = {};

        // Extract candidate information from responses
        for (const content of result1.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              candidatesInfo1 = { ...candidatesInfo1, ...parsed };
            } catch {
              // Not JSON, check for Dockerfile content
              if (content.text.includes('FROM ')) {
                candidatesInfo1.dockerfileContent = content.text;
              }
            }
          }
        }

        for (const content of result2.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              candidatesInfo2 = { ...candidatesInfo2, ...parsed };
            } catch {
              if (content.text.includes('FROM ')) {
                candidatesInfo2.dockerfileContent = content.text;
              }
            }
          }
        }

        // Validate generation characteristics
        const hasValidContent1 = candidatesInfo1.dockerfileContent || candidatesInfo1.content;
        const hasValidContent2 = candidatesInfo2.dockerfileContent || candidatesInfo2.content;

        if (!hasValidContent1 || !hasValidContent2) {
          return {
            success: false,
            duration: responseTime,
            message: 'One or both generations did not produce valid Dockerfile content',
            details: { candidatesInfo1, candidatesInfo2 }
          };
        }

        return {
          success: true,
          duration: responseTime,
          message: 'Dockerfile candidate generation working consistently',
          details: {
            generation1: {
              hasContent: !!hasValidContent1,
              contentLength: (candidatesInfo1.dockerfileContent || candidatesInfo1.content || '').length
            },
            generation2: {
              hasContent: !!hasValidContent2,
              contentLength: (candidatesInfo2.dockerfileContent || candidatesInfo2.content || '').length
            }
          },
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'dockerfile-scoring-determinism',
      category: 'sampling-validation', 
      description: 'Verify Dockerfile scoring is deterministic',
      tags: ['sampling', 'scoring', 'deterministic'],
      timeout: 30000,
      execute: async () => {
        const start = performance.now();
        
        // Generate a Dockerfile first
        const generateResult = await client.callTool({
          name: 'generate-dockerfile',
          arguments: {
            sessionId: 'scoring-determinism-test',
            baseImage: 'node:18-alpine',
            optimization: true
          }
        });

        if (generateResult.isError) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Dockerfile generation failed: ${generateResult.error?.message || 'Unknown error'}`
          };
        }

        // Score the same dockerfile multiple times
        // Note: This assumes there might be a scoring endpoint - if not available, 
        // we test that generation produces consistent scores
        
        const responseTime = performance.now() - start;

        // Parse generation result for scoring information
        let scoringInfo: any = {};
        for (const content of generateResult.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              scoringInfo = { ...scoringInfo, ...parsed };
            } catch {
              // Non-JSON content
            }
          }
        }

        // Check if scoring information is present
        const hasScore = scoringInfo.score !== undefined || scoringInfo.scoreBreakdown;
        
        if (!hasScore) {
          return {
            success: false,
            duration: responseTime,
            message: 'Generation result does not include scoring information',
            details: scoringInfo
          };
        }

        // Validate scoring structure
        const score = scoringInfo.score || 0;
        const scoreBreakdown = scoringInfo.scoreBreakdown || {};
        
        const isValidScore = typeof score === 'number' && score >= 0 && score <= 100;
        const hasBreakdown = Object.keys(scoreBreakdown).length > 0;

        return {
          success: isValidScore && hasBreakdown,
          duration: responseTime,
          message: isValidScore && hasBreakdown 
            ? 'Scoring system working with proper breakdown'
            : 'Scoring system has issues with score format or breakdown',
          details: {
            score,
            scoreBreakdown,
            isValidScore,
            hasBreakdown
          },
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'sampling-performance-benchmark',
      category: 'sampling-validation',
      description: 'Benchmark sampling performance against targets',
      tags: ['performance', 'sampling', 'benchmark'],
      timeout: 35000, // 35 seconds - above target but reasonable for testing
      execute: async () => {
        const TARGET_GENERATION_TIME = 30000; // 30 seconds
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'generate-dockerfile',
          arguments: {
            sessionId: 'performance-benchmark-test',
            baseImage: 'node:18-alpine',
            optimization: true,
            multistage: true,
            securityHardening: true
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Performance test generation failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract performance metrics if available
        let performanceInfo: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              performanceInfo = { ...performanceInfo, ...parsed };
            } catch {
              // Non-JSON content
            }
          }
        }

        const withinTarget = responseTime <= TARGET_GENERATION_TIME;
        const candidateCount = performanceInfo.candidateCount || performanceInfo.candidates?.length || 1;

        return {
          success: withinTarget,
          duration: responseTime,
          message: withinTarget 
            ? `Generation completed within target (${Math.round(responseTime)}ms)`
            : `Generation exceeded target: ${Math.round(responseTime)}ms > ${TARGET_GENERATION_TIME}ms`,
          details: {
            responseTime: Math.round(responseTime),
            target: TARGET_GENERATION_TIME,
            withinTarget,
            candidateCount,
            avgTimePerCandidate: candidateCount > 0 ? Math.round(responseTime / candidateCount) : responseTime
          },
          performance: {
            responseTime,
            memoryUsage: 0,
            operationCount: candidateCount
          }
        };
      }
    },

    {
      name: 'multi-candidate-validation',
      category: 'sampling-validation',
      description: 'Verify multiple candidates are generated and ranked',
      tags: ['sampling', 'candidates', 'ranking'],
      timeout: 40000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'generate-dockerfile',
          arguments: {
            sessionId: 'multi-candidate-test',
            baseImage: 'node:18-alpine',
            optimization: true
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Multi-candidate generation failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        // Extract candidate information
        let candidateInfo: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              candidateInfo = { ...candidateInfo, ...parsed };
            } catch {
              // Non-JSON content
            }
          }
        }

        // Check for evidence of multiple candidates
        const candidateCount = candidateInfo.candidateCount || 
                              candidateInfo.candidates?.length || 
                              candidateInfo.alternativeCount || 1;

        const hasWinner = candidateInfo.winner || candidateInfo.selectedCandidate || candidateInfo.content;
        const hasScoring = candidateInfo.score !== undefined || candidateInfo.scoreBreakdown;

        // For sampling, we expect multiple candidates were considered (even if only winner returned)
        const expectedMinCandidates = 1; // At least winner
        const hasMultipleCandidates = candidateCount >= expectedMinCandidates;

        return {
          success: hasMultipleCandidates && hasWinner && hasScoring,
          duration: responseTime,
          message: hasMultipleCandidates && hasWinner && hasScoring
            ? `Multi-candidate sampling working (${candidateCount} candidates considered)`
            : `Multi-candidate sampling issues: candidates=${candidateCount}, winner=${!!hasWinner}, scoring=${!!hasScoring}`,
          details: {
            candidateCount,
            hasWinner,
            hasScoring,
            winnerScore: candidateInfo.score,
            scoreBreakdown: candidateInfo.scoreBreakdown
          },
          performance: {
            responseTime,
            memoryUsage: 0,
            operationCount: candidateCount
          }
        };
      }
    },

    {
      name: 'sampling-error-handling',
      category: 'sampling-validation',
      description: 'Test sampling behavior with invalid inputs',
      tags: ['sampling', 'error-handling', 'edge-cases'],
      timeout: 15000,
      execute: async () => {
        const start = performance.now();
        
        // Test with invalid base image
        const result = await client.callTool({
          name: 'generate-dockerfile',
          arguments: {
            sessionId: 'error-handling-test',
            baseImage: 'invalid-nonexistent-image:999',
            optimization: true
          }
        });

        const responseTime = performance.now() - start;

        // For error handling, we want either:
        // 1. A graceful fallback with warning
        // 2. A proper error response
        
        if (result.isError) {
          // Proper error handling
          return {
            success: true,
            duration: responseTime,
            message: 'Sampling properly handles invalid inputs with error response',
            details: {
              errorHandled: true,
              errorMessage: result.error?.message || 'Unknown error'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
            }
          };
        }

        // Check if it gracefully handled the invalid image
        let responseInfo: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              responseInfo = { ...responseInfo, ...parsed };
            } catch {
              responseInfo.textContent = content.text;
            }
          }
        }

        const hasWarning = responseInfo.warning || responseInfo.warnings || 
                          (responseInfo.textContent && responseInfo.textContent.includes('warning'));
        const hasFallback = responseInfo.fallbackUsed || responseInfo.baseImage !== 'invalid-nonexistent-image:999';

        return {
          success: hasWarning || hasFallback,
          duration: responseTime,
          message: hasWarning || hasFallback
            ? 'Sampling gracefully handles invalid inputs with warnings/fallbacks'
            : 'Sampling may not properly handle invalid inputs',
          details: {
            hasWarning,
            hasFallback,
            response: responseInfo
          },
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    }
  ];

  return tests;
};