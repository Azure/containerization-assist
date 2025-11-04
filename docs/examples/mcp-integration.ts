/**
 * Example: MCP Server Integration with Container Assist
 *
 * This example demonstrates the standard pattern for integrating Container Assist
 * tools into your own MCP server using createApp() and bindToMCP().
 */

import { z } from 'zod';
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { CallToolResult } from '@modelcontextprotocol/sdk/types.js';
import { createApp, analyzeRepoTool, generateDockerfileTool } from 'containerization-assist-mcp';

/**
 * Step 1: Define your custom tool schemas and types
 */
const GetContainerizationPlanSchema = z.object({
  workspaceFolder: z.string()
    .describe("The workspace folder of the current project"),
  servicePaths: z.array(z.string())
    .describe("The absolute paths of each service to containerize"),
});

type GetContainerizationPlanParams = z.infer<typeof GetContainerizationPlanSchema>;

interface ContainerToolNames {
  analyzeRepo: string;
  generateDockerfile: string;
}

/**
 * Step 2: Implement your custom tool logic
 *
 * This tool orchestrates Container Assist tools to generate a containerization plan
 */
async function handleContainerizationPlan(
  params: GetContainerizationPlanParams,
  toolNames: ContainerToolNames
): Promise<string> {
  // Your orchestration logic here
  // This example returns a template that references the Container Assist tools

  const serviceList = params.servicePaths
    .map(p => `- ${p}`)
    .join('\n');

  return `
# Containerization Plan

## Services to Containerize
${serviceList}

## Execution Steps

1. **Scan Repository**: Use tool '${toolNames.analyzeRepo}' to analyze each service
2. **Generate Dockerfiles**: Use tool '${toolNames.generateDockerfile}' to create Dockerfiles
3. **Build Images**: Build Docker images for each service
4. **Verify**: Test that images run correctly

Follow these steps to containerize your application.
`;
}

/**
 * Step 3: Format tool results for MCP
 */
function formatToolResult(toolResult: string, isError = false): CallToolResult {
  return {
    isError,
    content: [{ type: "text", text: toolResult }]
  };
}

/**
 * Step 4: Set up your MCP server with Container Assist integration
 */
export function setupMCPServer(): void {
  // Create your MCP server
  const server = new McpServer(
    {
      name: "my-containerization-server",
      version: "1.0.0",
    },
    {
      capabilities: {
        logging: {},
        tools: {},
      }
    }
  );

  // Define custom names for Container Assist tools
  const customAnalyzeRepoName = 'my-analyze-repository';
  const customGenerateDockerfileName = 'my-generate-dockerfile';

  // Create Container Assist app with selective tools and configuration
  const containerAssistApp = createApp({
    // Register only the tools you need
    tools: [analyzeRepoTool, generateDockerfileTool],

    // Use custom names (aliases) for the tools
    toolAliases: {
      'analyze-repo': customAnalyzeRepoName,
      'generate-dockerfile': customGenerateDockerfileName
    },

    // Use natural language format for rich, user-friendly output
    outputFormat: "natural-language"

    // Note: chainHintsMode defaults to 'enabled' - omit to use default
    // Set to 'disabled' only if you want to suppress next-step suggestions
  });

  // Bind Container Assist tools to your MCP server
  // This automatically handles context creation and tool registration
  containerAssistApp.bindToMCP(server);

  // Store the tool names for use in custom orchestration tools
  const toolNames: ContainerToolNames = {
    analyzeRepo: customAnalyzeRepoName,
    generateDockerfile: customGenerateDockerfileName
  };

  // Register your custom orchestration tool
  server.tool(
    "get-containerization-plan",
    "Generate a containerization plan for the application",
    GetContainerizationPlanSchema.shape,
    { readOnlyHint: true },
    async (args, _extra) => {
      try {
        const response = await handleContainerizationPlan(args, toolNames);
        return formatToolResult(response);
      } catch (error) {
        console.error('Error generating plan:', error);
        return formatToolResult(`Failed to generate plan: ${error}`, true);
      }
    }
  );

  // Optional: Register a prompt to guide the agent
  server.prompt(
    "containerization-workflow",
    "Generate a workflow for containerizing the application",
    async (_args) => {
      return {
        messages: [
          {
            role: "user",
            content: {
              type: "text",
              text: `Help me containerize my application using the get-containerization-plan tool.`
            }
          }
        ]
      };
    }
  );

  // Connect to stdio transport
  const transport = new StdioServerTransport();
  server
    .connect(transport)
    .then(() => {
      console.error('MCP server connected successfully.');
    })
    .catch((error: Error) => {
      console.error('Failed to connect MCP server:', error);
      process.exit(1);
    });
}

// Run the server if executed directly
if (import.meta.url === `file://${process.argv[1]}`) {
  setupMCPServer();
}
