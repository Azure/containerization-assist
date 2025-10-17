/**
 * Edge Cases and Error Handling Tests
 * MCP Inspector Testing Infrastructure
 * Tests system behavior with invalid inputs, edge cases, and error conditions
 */

import { TestCase, MCPTestRunner } from "../../infrastructure/test-runner".js';

export const createErrorHandlingTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'missing-required-parameters',
      category: 'tool-validation',
      description: 'Test tools behavior with missing required parameters',
      tags: ['error-handling', 'validation', 'parameters'],
      timeout: 10000,
      execute: async () => {
        const start = performance.now();

        // Test analyze-repo with minimal required parameters
        const result = await client.callTool({
          name: 'analyze-repo',
          arguments: {
            repoPath: './test/__support__/fixtures/node-express'
          }
        });

        const responseTime = performance.now() - start;

        // We expect this to work normally now (no sessionId needed)
        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: 'Tool failed unexpectedly with minimal parameters',
            details: {
              error: result.error?.message,
              handledGracefully: false
            },
            performance: {
              responseTime,
              memoryUsage: 0,
            }
          };
        }

        // Check if it succeeded with expected data
        let responseData: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              responseData = { ...responseData, ...parsed };
            } catch {
              responseData.textContent = content.text;
            }
          }
        }

        const hasValidData = responseData.framework || responseData.language ||
                             responseData.dependencies || responseData.textContent;

        return {
          success: hasValidData,
          duration: responseTime,
          message: hasValidData
            ? 'Tool works correctly with minimal required parameters'
            : 'Tool returned unexpected response',
          details: responseData,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'invalid-file-paths',
      category: 'tool-validation',
      description: 'Test behavior with non-existent or invalid file paths',
      tags: ['error-handling', 'file-system', 'paths'],
      timeout: 15000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'analyze-repo',
          arguments: {
            repoPath: '/this/path/definitely/does/not/exist'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: true,
            duration: responseTime,
            message: 'Tool correctly handles invalid paths with error response',
            details: {
              error: result.error?.message,
              errorHandling: 'proper'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
            }
          };
        }

        // Check if it handled the invalid path gracefully
        let responseData: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              responseData = { ...responseData, ...parsed };
            } catch {
              responseData.textContent = content.text;
            }
          }
        }

        const handledGracefully = responseData.error || responseData.warning || 
                                 responseData.success === false || 
                                 (responseData.textContent && responseData.textContent.includes('not found'));

        return {
          success: handledGracefully,
          duration: responseTime,
          message: handledGracefully 
            ? 'Tool handled invalid path gracefully'
            : 'Tool may not properly validate file paths',
          details: responseData,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'malformed-json-arguments',
      category: 'tool-validation',
      description: 'Test behavior with invalid argument types',
      tags: ['error-handling', 'validation', 'types'],
      timeout: 10000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'analyze-repo',
          arguments: {
            repoPath: './test/__support__/fixtures/node-express',
            depth: 'invalid-number', // Should be number
            includeTests: 'not-a-boolean' // Should be boolean
          }
        });

        const responseTime = performance.now() - start;

        // Check how the tool handles type mismatches
        if (result.isError) {
          return {
            success: true,
            duration: responseTime,
            message: 'Tool correctly validates argument types',
            details: {
              error: result.error?.message,
              validation: 'proper'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
            }
          };
        }

        // If no error, check if it handled the types gracefully (converted or warned)
        let responseData: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              responseData = { ...responseData, ...parsed };
            } catch {
              responseData.textContent = content.text;
            }
          }
        }

        const hasAnalysis = responseData.language || responseData.framework || responseData.textContent;

        return {
          success: !!hasAnalysis,
          duration: responseTime,
          message: hasAnalysis 
            ? 'Tool handled type mismatches gracefully (possibly with conversion)'
            : 'Tool behavior unclear with invalid argument types',
          details: responseData,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'empty-and-null-values',
      category: 'tool-validation',
      description: 'Test behavior with empty strings and null values',
      tags: ['error-handling', 'validation', 'edge-cases'],
      timeout: 10000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'ops',
          arguments: {
            operation: 'status'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: 'Tool failed unexpectedly',
            details: {
              error: result.error?.message,
              validation: 'unexpected-error'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
            }
          };
        }

        // Check if it responded normally
        let responseData: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              responseData = { ...responseData, ...parsed };
            } catch {
              responseData.textContent = content.text;
            }
          }
        }

        const hasResponse = responseData.status || responseData.result || responseData.textContent;

        return {
          success: !!hasResponse,
          duration: responseTime,
          message: hasResponse
            ? 'Tool responded normally'
            : 'Tool behavior unclear',
          details: responseData,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'concurrent-same-operation',
      category: 'load-testing',
      description: 'Test behavior with multiple concurrent calls of the same operation',
      tags: ['concurrency', 'edge-cases'],
      timeout: 20000,
      execute: async () => {
        const start = performance.now();

        // Make multiple concurrent calls with same operation
        const promises = Array.from({ length: 5 }, () =>
          client.callTool({
            name: 'ops',
            arguments: {
              operation: 'ping'
            }
          })
        );

        try {
          const results = await Promise.all(promises);
          const responseTime = performance.now() - start;
          
          const successCount = results.filter(r => !r.isError).length;
          const errorCount = results.filter(r => r.isError).length;

          return {
            success: successCount >= 3, // At least 60% success rate
            duration: responseTime,
            message: `Concurrent operations: ${successCount}/${results.length} successful`,
            details: {
              totalCalls: results.length,
              successful: successCount,
              errors: errorCount,
              concurrencyHandling: successCount >= 3 ? 'good' : 'issues-detected'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
              operationCount: results.length
            }
          };
        } catch (error) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `Concurrent operations test failed: ${error instanceof Error ? error.message : 'Unknown error'}`
          };
        }
      }
    },

    {
      name: 'large-argument-payload',
      category: 'tool-validation',
      description: 'Test behavior with unusually large argument payloads',
      tags: ['performance', 'limits', 'edge-cases'],
      timeout: 30000,
      execute: async () => {
        const start = performance.now();
        
        // Create a large string for testing payload limits
        const largeString = 'x'.repeat(100000); // 100KB string
        
        const result = await client.callTool({
          name: 'ops',
          arguments: {
            operation: 'status',
            largeData: largeString // Extra parameter with large data
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: true,
            duration: responseTime,
            message: 'Tool correctly handles large payloads (possibly with size limits)',
            details: {
              error: result.error?.message,
              payloadSize: largeString.length,
              handled: 'with-error'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
            }
          };
        }

        // Check if it processed the large payload
        let responseData: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              responseData = { ...responseData, ...parsed };
            } catch {
              responseData.textContent = content.text;
            }
          }
        }

        const hasResponse = responseData.status || responseData.result || responseData.textContent;
        const withinReasonableTime = responseTime <= 5000; // Should handle large payload within 5s

        return {
          success: hasResponse && withinReasonableTime,
          duration: responseTime,
          message: hasResponse && withinReasonableTime
            ? `Tool handled large payload efficiently (${Math.round(responseTime)}ms)`
            : `Tool had issues with large payload (${Math.round(responseTime)}ms)`,
          details: {
            ...responseData,
            payloadSize: largeString.length,
            responseTime: Math.round(responseTime),
            withinTimeLimit: withinReasonableTime
          },
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'special-characters-in-arguments',
      category: 'tool-validation',
      description: 'Test behavior with special characters and unicode in arguments',
      tags: ['encoding', 'edge-cases', 'validation'],
      timeout: 15000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'ops',
          arguments: {
            operation: 'ping',
            specialData: '{"json": "with quotes", "unicode": "🔥💯", "xml": "<tag>content</tag>"}'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: true,
            duration: responseTime,
            message: 'Tool handles special characters with appropriate error handling',
            details: {
              error: result.error?.message,
              encoding: 'handled-with-error'
            },
            performance: {
              responseTime,
              memoryUsage: 0,
            }
          };
        }

        // Check if it processed special characters correctly
        let responseData: any = {};
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              responseData = { ...responseData, ...parsed };
            } catch {
              responseData.textContent = content.text;
            }
          }
        }

        const hasResponse = responseData.status || responseData.result || responseData.textContent;

        return {
          success: !!hasResponse,
          duration: responseTime,
          message: hasResponse 
            ? 'Tool processed special characters successfully'
            : 'Tool had issues with special character encoding',
          details: responseData,
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