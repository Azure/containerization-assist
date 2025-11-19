#!/usr/bin/env node
/* eslint-disable @typescript-eslint/no-unused-vars */
import { z } from "zod";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { ContainerAssistToolNames } from "./models";
import { populateGetContainerizationPlanPrompts } from "./containerizationPlan";
import { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import {
  createApp,
  analyzeRepoTool,
  generateDockerfileTool
} from "containerization-assist-mcp";
import { ContainerAssistMCPServerTool } from "./containerAssistMCPServerTool";
import { TOOL_NAME } from "containerization-assist-mcp/tools";
import { OUTPUTFORMAT } from "containerization-assist-mcp/server";

export interface GetContainerizationPlanParameters {
  workspaceFolder: string;
  servicePaths: string[];
}

const GetContainerizationPlanSchema = z.object({
  workspaceFolder: z.string()
    .describe("The workspace folder of the current project. Use '/' as path separator."),
  servicePaths: z.array(z.string())
    .describe("The absolute paths of each service to deploy. Each service is a separately deployed compute service. Use '/' as path separator."),
});

const GetContainerizationPlanDescription = "Entry point to help the agent generate a containerization plan for the application. Agent should read its output and generate a containerization plan in '.azure/containerization.copilotmd' for execution steps, and steps needed to convert the projects to container-ready with Dockerfile. Before calling this tool, please scan this workspace to detect the services to containerize.";

async function handleContainerizationPlan(request: z.infer<typeof GetContainerizationPlanSchema>, containerAssistToolNames: ContainerAssistToolNames): Promise<string> {
  const mappedInput = request as GetContainerizationPlanParameters;

  return populateGetContainerizationPlanPrompts(
    mappedInput.workspaceFolder,
    mappedInput.servicePaths,
    containerAssistToolNames
  );
}

function formatToolResult(toolResult: string, isError?: boolean): CallToolResult  {
  return {
    isError: isError || false,
    content: [
      {
        type: "text",
        text: toolResult
      }
    ]
  };
}

export function setupJavaMigrationMcpServer(): void {
  const server = new McpServer(
    {
      name: "testServer",
      version: "0.0.1",
    },
    {
      capabilities: {
        logging: {},
      }
    }
  );

  function registerContainerAssistTools(): ContainerAssistToolNames {
    const overrideAnalyzeRepositoryName = 'appmod-analyze-repository';
    const overrideGenerateDockerfileName = 'appmod-plan-generate-dockerfile';

    const containerAssistApp = createApp({
      // Register only the tools we need
      tools: [
        analyzeRepoTool,
        generateDockerfileTool
      ],

      // Disable chain hints (automatic next-step suggestions)
      chainHintsMode: "disabled",

      // Use natural language format for rich, user-friendly output
      outputFormat: OUTPUTFORMAT.NATURAL_LANGUAGE,
    });

    const tools: ContainerAssistMCPServerTool[] = [
      new ContainerAssistMCPServerTool(
        containerAssistApp,
        analyzeRepoTool,
        TOOL_NAME.ANALYZE_REPO,
        overrideAnalyzeRepositoryName),
      new ContainerAssistMCPServerTool(
        containerAssistApp,
        generateDockerfileTool,
        TOOL_NAME.GENERATE_DOCKERFILE,
        overrideGenerateDockerfileName),
    ]

    // Register each tool with telemetry using createToolHandler
    // This automatically handles ToolContext creation and formats output correctly
    for (const tool of tools) {
      server.tool(
        tool.getName(),
        tool.getDescription(),
        tool.getInputSchema(),
        { readOnlyHint: true },
        async (params, extra) => {
          return await tool.handler(params, extra);
        }
      );
    }

    console.log(`✅ Registered ${tools.length} Container Assist tools with telemetry`);

    return {
      analyzeRepo: overrideAnalyzeRepositoryName,
      generateDockerfile: overrideGenerateDockerfileName
    };
  };

  const containerAssistToolNames = registerContainerAssistTools();

  server.prompt(
    "containerization-workflow",
    "Generate a workflow for the App Modernization containerization process",
    async (_args) => {
      return {
        messages: [
          {
            role: "user",
            content: {
              type: "text",
              text: `Scan my project and help me plan how to containerize my application using the #appmod-get-containerization-plan tool. Execute the plan. The end goal is to have Dockerfiles that are able to be built.`
            }
          }
        ]
      };
    }
  );

  server.tool(
    "appmod-get-containerization-plan",
    GetContainerizationPlanDescription,
    GetContainerizationPlanSchema.shape,
    { readOnlyHint: true },
    async (args) => {
      try {
        const response = await handleContainerizationPlan(args, containerAssistToolNames);
        return formatToolResult(response);
      } catch (error) {
        console.error('Error occurred while handling tool request:', error);
        return formatToolResult(`MCP Server Tool failed: ${error}`, true);
      }
    }
  );

  const transport = new StdioServerTransport();
  server
    .connect(transport)
    .then(async () => {
      console.info('MCP server connected successfully.');
    })
    .catch((error: Error) => {
      console.error('Error occurred while connect to MCP server:', error);
    })
    .finally(() => {
      console.info('MCP server is running. Press Ctrl+C to exit.');
    });
}

