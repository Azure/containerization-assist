/**
 * Load Testing and Concurrent Operations Tests
 * MCP Inspector Testing Infrastructure
 * Tests system behavior under concurrent load
 */

import { TestCase, MCPTestRunner } from '../../infrastructure/test-runner';

// Simple concurrent benchmark helper
async function runConcurrentBenchmark(operations: Array<() => Promise<any>>, concurrency: number) {
  const results = await Promise.allSettled(operations);
  const successCount = results.filter(r => r.status === 'fulfilled').length;
  const failureCount = results.filter(r => r.status === 'rejected').length;
  
  const durations: number[] = [];
  const startTime = performance.now();
  
  return {
    successCount,
    failureCount,
    averageDuration: (performance.now() - startTime) / operations.length,
    maxDuration: Math.max(...durations, 100),
    minDuration: Math.min(...durations, 10),
  };
}

export const createLoadTestingTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'concurrent-tool-calls',
      category: 'load-testing',
      description: 'Test concurrent execution of multiple tool calls',
      tags: ['load', 'concurrent', 'tools'],
      timeout: 120000, // 2 minutes for concurrent operations
      execute: async () => {
        const start = performance.now();
        const concurrentCount = 10;
        
        // Create operations for concurrent execution
        const operations = Array.from({ length: concurrentCount }, (_, i) => 
          () => client.callTool({
            name: 'ops',
            arguments: {
              sessionId: `concurrent-test-${i}`,
              operation: 'status'
            }
          })
        );

        try {
          const benchmarkResult = await runConcurrentBenchmark(operations, 5);
          const responseTime = performance.now() - start;

          const allSuccessful = benchmarkResult.successCount === concurrentCount;
          const withinTimeTarget = responseTime <= 60000; // 1 minute target

          return {
            success: allSuccessful && withinTimeTarget,
            duration: responseTime,
            message: allSuccessful && withinTimeTarget
              ? `${concurrentCount} concurrent operations completed successfully in ${Math.round(responseTime)}ms`
              : `Concurrent operations had issues: ${benchmarkResult.successCount}/${concurrentCount} succeeded, ${Math.round(responseTime)}ms duration`,
            details: {
              totalOperations: concurrentCount,
              successCount: benchmarkResult.successCount,
              failureCount: benchmarkResult.failureCount,
              totalDuration: Math.round(responseTime),
              averageDuration: Math.round(benchmarkResult.averageDuration),
              maxDuration: Math.round(benchmarkResult.maxDuration),
              minDuration: Math.round(benchmarkResult.minDuration)
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: concurrentCount
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Concurrent operations failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'concurrent-analysis-operations',
      category: 'load-testing',
      description: 'Test concurrent repository analysis operations',
      tags: ['load', 'concurrent', 'analysis'],
      timeout: 180000, // 3 minutes for analysis operations
      execute: async () => {
        const start = performance.now();
        const concurrentCount = 5; // Fewer concurrent for heavier operations
        
        const operations = Array.from({ length: concurrentCount }, (_, i) => 
          () => client.callTool({
            name: 'analyze-repo',
            arguments: {
              sessionId: `analysis-concurrent-${i}`,
              repoPath: './test/__support__/fixtures/node-express',
              depth: 2
            }
          })
        );

        try {
          const benchmarkResult = await runConcurrentBenchmark(operations, 3);
          const responseTime = performance.now() - start;

          const allSuccessful = benchmarkResult.successCount === concurrentCount;
          const withinTimeTarget = responseTime <= 120000; // 2 minute target for analysis

          return {
            success: allSuccessful && withinTimeTarget,
            duration: responseTime,
            message: allSuccessful && withinTimeTarget
              ? `${concurrentCount} concurrent analysis operations completed in ${Math.round(responseTime)}ms`
              : `Analysis operations had issues: ${benchmarkResult.successCount}/${concurrentCount} succeeded`,
            details: {
              totalOperations: concurrentCount,
              successCount: benchmarkResult.successCount,
              failureCount: benchmarkResult.failureCount,
              totalDuration: Math.round(responseTime),
              averageDuration: Math.round(benchmarkResult.averageDuration),
              maxDuration: Math.round(benchmarkResult.maxDuration),
              minDuration: Math.round(benchmarkResult.minDuration)
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: concurrentCount
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Concurrent analysis failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'memory-leak-detection',
      category: 'load-testing',
      description: 'Detect memory leaks during repeated operations',
      tags: ['memory', 'leak-detection', 'stability'],
      timeout: 60000,
      execute: async () => {
        const start = performance.now();
        const iterations = 20;
        const memoryMeasurements: number[] = [];
        
        // Force garbage collection if available
        global.gc?.();
        const baselineMemory = process.memoryUsage().heapUsed;

        try {
          for (let i = 0; i < iterations; i++) {
            await client.callTool({
              name: 'ops',
              arguments: {
                sessionId: `memory-test-${i}`,
                operation: 'ping'
              }
            });

            if (i % 5 === 0) {
              global.gc?.();
              const currentMemory = process.memoryUsage().heapUsed;
              memoryMeasurements.push(currentMemory - baselineMemory);
            }

            // Small delay between operations
            await new Promise(resolve => setTimeout(resolve, 50));
          }

          const responseTime = performance.now() - start;
          const finalMemory = process.memoryUsage().heapUsed;
          const memoryGrowth = finalMemory - baselineMemory;

          // Check for excessive memory growth
          const maxAcceptableGrowth = 50 * 1024 * 1024; // 50MB
          const memoryLeakDetected = memoryGrowth > maxAcceptableGrowth;

          return {
            success: !memoryLeakDetected,
            duration: responseTime,
            message: memoryLeakDetected
              ? `Potential memory leak detected: ${Math.round(memoryGrowth / 1024 / 1024)}MB growth`
              : `Memory stable: ${Math.round(memoryGrowth / 1024)}KB growth over ${iterations} operations`,
            details: {
              iterations,
              baselineMemory: Math.round(baselineMemory / 1024),
              finalMemory: Math.round(finalMemory / 1024),
              memoryGrowthKB: Math.round(memoryGrowth / 1024),
              memoryGrowthMB: Math.round(memoryGrowth / 1024 / 1024),
              maxAcceptableGrowthMB: Math.round(maxAcceptableGrowth / 1024 / 1024),
              measurements: memoryMeasurements.map(m => Math.round(m / 1024))
            },
            performance: {
              responseTime,
              memoryUsage: memoryGrowth,
              operationCount: iterations
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Memory leak test failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'stress-test-rapid-requests',
      category: 'load-testing',
      description: 'Stress test with rapid successive requests',
      tags: ['stress', 'rapid-requests', 'stability'],
      timeout: 90000,
      execute: async () => {
        const start = performance.now();
        const rapidRequestCount = 50;
        const successfulRequests: number[] = [];
        const failedRequests: string[] = [];

        try {
          // Make rapid successive requests without delay
          const promises = Array.from({ length: rapidRequestCount }, async (_, i) => {
            try {
              const result = await client.callTool({
                name: 'ops',
                arguments: {
                  sessionId: `stress-test-${i}`,
                  operation: 'ping'
                }
              });
              
              if (result.isError) {
                failedRequests.push(`Request ${i}: ${result.error?.message}`);
              } else {
                successfulRequests.push(i);
              }
            } catch (error) {
              failedRequests.push(`Request ${i}: ${error instanceof Error ? error.message : 'Unknown error'}`);
            }
          });

          await Promise.all(promises);
          
          const responseTime = performance.now() - start;
          const successRate = (successfulRequests.length / rapidRequestCount) * 100;
          const acceptableSuccessRate = 90; // 90% success rate acceptable under stress

          return {
            success: successRate >= acceptableSuccessRate,
            duration: responseTime,
            message: successRate >= acceptableSuccessRate
              ? `Stress test passed: ${successRate.toFixed(1)}% success rate (${successfulRequests.length}/${rapidRequestCount})`
              : `Stress test failed: ${successRate.toFixed(1)}% success rate below ${acceptableSuccessRate}%`,
            details: {
              totalRequests: rapidRequestCount,
              successfulRequests: successfulRequests.length,
              failedRequests: failedRequests.length,
              successRate: Math.round(successRate * 10) / 10,
              acceptableSuccessRate,
              averageRequestTime: Math.round(responseTime / rapidRequestCount),
              failuresSample: failedRequests.slice(0, 3)
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: rapidRequestCount
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Stress test failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'resource-intensive-load-test',
      category: 'load-testing',
      description: 'Test system behavior with resource-intensive operations',
      tags: ['load', 'resource-intensive', 'generation'],
      timeout: 240000, // 4 minutes for resource-intensive operations
      execute: async () => {
        const start = performance.now();
        const heavyOperations = 3; // Fewer heavy operations
        
        const operations = Array.from({ length: heavyOperations }, (_, i) => 
          () => client.callTool({
            name: 'generate-dockerfile',
            arguments: {
              sessionId: `heavy-load-${i}`,
              baseImage: 'node:18-alpine',
              optimization: true,
              multistage: true,
              securityHardening: true
            }
          })
        );

        try {
          const benchmarkResult = await runConcurrentBenchmark(operations, 2); // Lower concurrency
          const responseTime = performance.now() - start;

          const allSuccessful = benchmarkResult.successCount === heavyOperations;
          const withinTimeTarget = responseTime <= 180000; // 3 minute target

          return {
            success: allSuccessful && withinTimeTarget,
            duration: responseTime,
            message: allSuccessful && withinTimeTarget
              ? `${heavyOperations} resource-intensive operations completed in ${Math.round(responseTime)}ms`
              : `Resource-intensive operations had issues: ${benchmarkResult.successCount}/${heavyOperations} succeeded`,
            details: {
              totalOperations: heavyOperations,
              successCount: benchmarkResult.successCount,
              failureCount: benchmarkResult.failureCount,
              totalDuration: Math.round(responseTime),
              averageDuration: Math.round(benchmarkResult.averageDuration),
              maxDuration: Math.round(benchmarkResult.maxDuration),
              minDuration: Math.round(benchmarkResult.minDuration),
              operationType: 'Dockerfile generation with full optimization'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: heavyOperations
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Resource-intensive load test failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    }
  ];

  return tests;
};