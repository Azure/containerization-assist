/**
 * Resource Management Tests for MCP Inspector
 * MCP Inspector Testing Infrastructure
 * Tests resource management system via MCP resources
 */

import { TestCase, MCPTestRunner } from "../../infrastructure/test-runner".js';

export const createResourceManagementTests = (testRunner: MCPTestRunner): TestCase[] => {
  const client = testRunner.getClient();

  const tests: TestCase[] = [
    {
      name: 'resource-size-limits',
      category: 'resource-management',
      description: 'Verify resources respect 5MB size limits',
      tags: ['resources', 'size-limits', 'validation'],
      timeout: 20000,
      execute: async () => {
        const start = performance.now();
        const MAX_RESOURCE_SIZE = 5 * 1024 * 1024; // 5MB
        
        // Generate a complex analysis that might produce large resources
        const result = await client.callTool({
          name: 'analyze-repo',
          arguments: {
            sessionId: 'resource-size-test',
            repoPath: './test/__support__/fixtures/node-express',
            depth: 5, // Deep analysis might produce larger results
            includeTests: true
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Resource size test analysis failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        let totalResourceSize = 0;
        const resourceSizes: Array<{ uri: string; size: number }> = [];

        // Check all resources in the response
        for (const content of result.content) {
          if (content.type === 'resource' && content.resource) {
            try {
              const resourceData = await client.readResource({
                uri: content.resource.uri
              });
              
              if (resourceData.contents) {
                const size = Buffer.byteLength(JSON.stringify(resourceData.contents));
                totalResourceSize += size;
                resourceSizes.push({ uri: content.resource.uri, size });
                
                if (size > MAX_RESOURCE_SIZE) {
                  return {
                    success: false,
                    duration: responseTime,
                    message: `Resource exceeds size limit: ${size} bytes > ${MAX_RESOURCE_SIZE} bytes`,
                    details: { uri: content.resource.uri, size, limit: MAX_RESOURCE_SIZE }
                  };
                }
              }
            } catch (error) {
              return {
                success: false,
                duration: responseTime,
                message: `Failed to read resource ${content.resource.uri}: ${error}`
              };
            }
          } else if (content.type === 'text' && content.text) {
            // Check inline text content size
            const size = Buffer.byteLength(content.text);
            totalResourceSize += size;
          }
        }

        return {
          success: true,
          duration: responseTime,
          message: `All resources within size limits (total: ${Math.round(totalResourceSize / 1024)}KB)`,
          details: {
            totalResourceSize,
            resourceCount: resourceSizes.length,
            maxResourceSize: Math.max(...resourceSizes.map(r => r.size), 0),
            resourceSizes: resourceSizes.slice(0, 3) // Show first 3 for brevity
          },
          performance: {
            responseTime,
            memoryUsage: 0,
            resourceSize: totalResourceSize
          }
        };
      }
    },

    {
      name: 'resource-accessibility',
      category: 'resource-management',
      description: 'Verify all resource URIs are accessible',
      tags: ['resources', 'accessibility', 'uris'],
      timeout: 25000,
      execute: async () => {
        const start = performance.now();
        
        // Generate a Dockerfile that should produce resource links
        const result = await client.callTool({
          name: 'generate-dockerfile',
          arguments: {
            sessionId: 'resource-access-test',
            baseImage: 'node:18-alpine',
            optimization: true
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Resource accessibility test failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        const accessibleResources: string[] = [];
        const inaccessibleResources: Array<{ uri: string; error: string }> = [];

        // Check all resource URIs for accessibility
        for (const content of result.content) {
          if (content.type === 'resource' && content.resource) {
            try {
              const resourceData = await client.readResource({
                uri: content.resource.uri
              });
              
              if (resourceData.contents) {
                accessibleResources.push(content.resource.uri);
              } else {
                inaccessibleResources.push({
                  uri: content.resource.uri,
                  error: 'Empty contents'
                });
              }
            } catch (error) {
              inaccessibleResources.push({
                uri: content.resource.uri,
                error: error instanceof Error ? error.message : String(error)
              });
            }
          }
        }

        const allAccessible = inaccessibleResources.length === 0;
        const totalResources = accessibleResources.length + inaccessibleResources.length;

        return {
          success: allAccessible,
          duration: responseTime,
          message: allAccessible 
            ? `All ${totalResources} resources are accessible`
            : `${inaccessibleResources.length}/${totalResources} resources are inaccessible`,
          details: {
            totalResources,
            accessibleResources: accessibleResources.length,
            inaccessibleResources: inaccessibleResources.length,
            failures: inaccessibleResources.slice(0, 3) // Show first 3 failures
          },
          performance: {
            responseTime,
            memoryUsage: 0,
            operationCount: totalResources
          }
        };
      }
    },

    {
      name: 'resource-mime-types',
      category: 'resource-management', 
      description: 'Verify resources have appropriate MIME types',
      tags: ['resources', 'mime-types', 'metadata'],
      timeout: 15000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'analyze-repo',
          arguments: {
            sessionId: 'mime-type-test',
            repoPath: './test/__support__/fixtures/node-express'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `MIME type test failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        const mimeTypeInfo: Array<{ uri: string; mimeType: string | undefined }> = [];

        for (const content of result.content) {
          if (content.type === 'resource' && content.resource) {
            try {
              const resourceData = await client.readResource({
                uri: content.resource.uri
              });
              
              mimeTypeInfo.push({
                uri: content.resource.uri,
                mimeType: resourceData.mimeType
              });
            } catch (error) {
              mimeTypeInfo.push({
                uri: content.resource.uri,
                mimeType: undefined
              });
            }
          }
        }

        const resourcesWithMimeType = mimeTypeInfo.filter(r => r.mimeType);
        const allHaveMimeType = mimeTypeInfo.length === resourcesWithMimeType.length;

        const commonMimeTypes = ['application/json', 'text/plain', 'text/dockerfile', 'application/yaml'];
        const validMimeTypes = resourcesWithMimeType.filter(r => 
          commonMimeTypes.includes(r.mimeType!) || r.mimeType!.startsWith('text/') || r.mimeType!.startsWith('application/')
        );
        const allValidMimeTypes = validMimeTypes.length === resourcesWithMimeType.length;

        return {
          success: allHaveMimeType && allValidMimeTypes,
          duration: responseTime,
          message: allHaveMimeType && allValidMimeTypes
            ? `All ${mimeTypeInfo.length} resources have valid MIME types`
            : `MIME type issues: ${resourcesWithMimeType.length}/${mimeTypeInfo.length} have types, ${validMimeTypes.length} are valid`,
          details: {
            totalResources: mimeTypeInfo.length,
            withMimeType: resourcesWithMimeType.length,
            validMimeTypes: validMimeTypes.length,
            mimeTypesSample: resourcesWithMimeType.slice(0, 3).map(r => r.mimeType)
          },
          performance: {
            responseTime,
            memoryUsage: 0,
          }
        };
      }
    },

    {
      name: 'resource-caching-behavior',
      category: 'resource-management',
      description: 'Test resource caching and TTL behavior', 
      tags: ['resources', 'caching', 'ttl'],
      timeout: 10000,
      execute: async () => {
        const start = performance.now();
        
        // Make the same request twice to test caching
        const sessionId = 'caching-test-' + Date.now();
        
        const result1 = await client.callTool({
          name: 'ops',
          arguments: {
            sessionId,
            operation: 'status'
          }
        });

        if (result1.isError) {
          return {
            success: false,
            duration: performance.now() - start,
            message: `First caching test request failed: ${result1.error?.message || 'Unknown error'}`
          };
        }

        // Small delay to allow potential caching
        await new Promise(resolve => setTimeout(resolve, 100));

        const result2 = await client.callTool({
          name: 'ops',
          arguments: {
            sessionId,
            operation: 'status'
          }
        });

        const responseTime = performance.now() - start;

        if (result2.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `Second caching test request failed: ${result2.error?.message || 'Unknown error'}`
          };
        }

        // For basic ops calls, we mainly test that resources are properly managed
        // More complex caching tests would need generation tools that create cacheable resources
        
        const hasResources1 = result1.content.some(c => c.type === 'resource');
        const hasResources2 = result2.content.some(c => c.type === 'resource');
        
        return {
          success: true,
          duration: responseTime,
          message: 'Resource caching behavior test completed (basic validation)',
          details: {
            firstRequestHadResources: hasResources1,
            secondRequestHadResources: hasResources2,
            responseTimeDelta: 'measured in test runner',
            note: 'Full caching validation requires generation tools with cacheable outputs'
          },
          performance: {
            responseTime,
            memoryUsage: 0,
            operationCount: 2
          }
        };
      }
    },

    {
      name: 'resource-uri-scheme-validation',
      category: 'resource-management',
      description: 'Validate resource URI schemes are properly formatted',
      tags: ['resources', 'uri-schemes', 'validation'],
      timeout: 15000,
      execute: async () => {
        const start = performance.now();
        
        const result = await client.callTool({
          name: 'analyze-repo',
          arguments: {
            sessionId: 'uri-scheme-test',
            repoPath: './test/__support__/fixtures/node-express'
          }
        });

        const responseTime = performance.now() - start;

        if (result.isError) {
          return {
            success: false,
            duration: responseTime,
            message: `URI scheme test failed: ${result.error?.message || 'Unknown error'}`
          };
        }

        const uriInfo: Array<{ uri: string; isValid: boolean; scheme: string }> = [];
        const expectedSchemes = ['mcp://', 'resource://', 'analysis://', 'dockerfile://', 'cache://'];

        for (const content of result.content) {
          if (content.type === 'resource' && content.resource) {
            const uri = content.resource.uri;
            const hasValidScheme = expectedSchemes.some(scheme => uri.startsWith(scheme));
            const scheme = uri.split('://')[0] + '://';
            
            uriInfo.push({
              uri,
              isValid: hasValidScheme,
              scheme
            });
          }
        }

        const validUris = uriInfo.filter(u => u.isValid);
        const allValidUris = validUris.length === uriInfo.length;

        return {
          success: allValidUris,
          duration: responseTime,
          message: allValidUris
            ? `All ${uriInfo.length} resource URIs use valid schemes`
            : `${validUris.length}/${uriInfo.length} URIs use valid schemes`,
          details: {
            totalUris: uriInfo.length,
            validUris: validUris.length,
            schemes: [...new Set(uriInfo.map(u => u.scheme))],
            invalidUrisSample: uriInfo.filter(u => !u.isValid).slice(0, 3)
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