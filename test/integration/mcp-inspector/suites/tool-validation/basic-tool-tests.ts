/**
 * Basic Tool Validation Tests
 * MCP Inspector Testing Infrastructure
 */

import { TestCase, MCPTestRunner } from "../../infrastructure/test-runner".js';

export const createBasicToolTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'ops-ping-responds',
      category: 'tool-validation',
      description: 'Verify ops tool ping operation responds correctly',
      tags: ['basic', 'connectivity'],
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'ops',
          arguments: {
            operation: 'ping'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Ops ping failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        const content = result.content[0];
        if (!content || content.type !== 'text') {
          return {
            success: false,
            duration: responseTime,
            message: 'Ops ping returned unexpected content format'
          };
        }

        const response = JSON.parse(content.text || '{}');
        
        return {
          success: response.status === 'success' || response.result === 'pong',
          duration: responseTime,
          message: response.status === 'success' ? 'Ops ping successful' : 'Ops ping returned unexpected status',
          details: response,
          performance: {
            responseTime,
            memoryUsage: 0, // Will be calculated by test runner
          }
        };
      }
    },

    {
      name: 'ops-status-tool',
      category: 'tool-validation',
      description: 'Verify ops tool status operation provides system information',
      tags: ['basic', 'system'],
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
            message: `Ops status failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        const content = result.content[0];
        if (!content || content.type !== 'text') {
          return {
            success: false,
            duration: responseTime,
            message: 'Ops status returned unexpected content format'
          };
        }

        const status = JSON.parse(content.text || '{}');
        
        // Validate response has some status information
        const hasStatusInfo = status.status || status.server || status.system || status.result;
        
        if (!hasStatusInfo) {
          return {
            success: false,
            duration: responseTime,
            message: 'Ops status missing expected status information'
          };
        }

        return {
          success: true,
          duration: responseTime,
          message: 'Ops status tool working correctly',
          details: status,
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'analyze-repository-basic',
      category: 'tool-validation',
      description: 'Test analyze_repository tool with a basic test fixture',
      tags: ['analysis', 'repository'],
      timeout: 30000, // 30 seconds for analysis
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'analyze-repo',
          arguments: {
            repoPath: './test/__support__/fixtures/node-express'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Repository analysis failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        let analysisData: any = {};
        let resourceSize = 0;

        // Process all content items
        for (const content of result.content) {
          if (content.type === 'text' && content.text) {
            try {
              const parsed = JSON.parse(content.text);
              analysisData = { ...analysisData, ...parsed };
            } catch {
              // If not JSON, treat as text content
              analysisData.textContent = content.text;
            }
          } else if (content.type === 'resource' && content.resource) {
            // Resource link found - this is expected for large analysis results
            const resourceData = await client.readResource({
              uri: content.resource.uri
            });
            
            if (resourceData.contents) {
              resourceSize = Buffer.byteLength(JSON.stringify(resourceData.contents));
              if (typeof resourceData.contents === 'string') {
                try {
                  const parsed = JSON.parse(resourceData.contents);
                  analysisData = { ...analysisData, ...parsed };
                } catch {
                  analysisData.resourceContent = resourceData.contents;
                }
              } else {
                analysisData = { ...analysisData, ...resourceData.contents };
              }
            }
          }
        }

        // Validate analysis contains expected structure
        const hasExpectedFields = analysisData.framework || analysisData.language || analysisData.dependencies;
        
        if (!hasExpectedFields) {
          return {
            success: false,
            duration: responseTime,
            message: 'Analysis result missing expected framework/language/dependencies information',
            details: analysisData
          };
        }

        return {
          success: true,
          duration: responseTime,
          message: 'Repository analysis completed successfully',
          details: {
            framework: analysisData.framework,
            language: analysisData.language,
            dependencyCount: analysisData.dependencies?.length || 0
          },
          performance: {
            responseTime,
            memoryUsage: 0,
            resourceSize
          }
        };
      }
    },

    {
      name: 'tool-response-time-validation',
      category: 'tool-validation', 
      description: 'Validate all basic tools respond within performance targets',
      tags: ['performance', 'baseline'],
      execute: async () => {
        const toolTests = [
          { name: 'ops', args: { operation: 'ping' } },
          { name: 'ops', args: { operation: 'status' } }
        ];

        const results = [];
        const targetResponseTime = 100; // 100ms target for metadata-only tools

        for (const tool of toolTests) {
          const start = performance.now();
          
          const result = await client.callTool({
            name: tool.name,
            arguments: tool.args
          });

          const responseTime = performance.now() - start;
          
          results.push({
            tool: tool.name,
            responseTime,
            withinTarget: responseTime <= targetResponseTime,
            success: !result.isError
          });
        }

        const allWithinTarget = results.every(r => r.withinTarget);
        const allSuccessful = results.every(r => r.success);

        return {
          success: allWithinTarget && allSuccessful,
          duration: results.reduce((sum, r) => sum + r.responseTime, 0),
          message: allWithinTarget && allSuccessful 
            ? 'All tools meet performance targets'
            : 'Some tools exceed performance targets or failed',
          details: results,
          performance: {
            responseTime: Math.max(...results.map(r => r.responseTime)),
            memoryUsage: 0
          }
        };
      }
    }
  ];

  return tests;
};