/* eslint-disable @typescript-eslint/no-unused-vars */
import { AppRuntime, createToolHandler, ToolHandlerOptions, ZodRawShape } from "containerization-assist-mcp";
import { Tool, ToolName } from "containerization-assist-mcp/tools";
import { ContainerAssistToolTelemetryProperties, ResultCode } from "./telemetryProperties";
import { RequestHandlerExtra } from "@modelcontextprotocol/sdk/shared/protocol.js";
import { CallToolResult, ServerNotification, ServerRequest } from "@modelcontextprotocol/sdk/types.js";

export class ContainerAssistMCPServerTool {
  protected containerAssistApp: AppRuntime;
  protected containerAssistToolName: ToolName;
  protected overrideName: string;
  protected mcpToolDescription: string;
  protected schema: ZodRawShape;

  constructor(containerAssistApp: AppRuntime, containerAssistTool: Tool, containerAssistToolName: ToolName, overrideName: string) {
    this.containerAssistApp = containerAssistApp;
    this.containerAssistToolName = containerAssistToolName;
    this.overrideName = overrideName;
    this.schema = containerAssistTool.inputSchema;
    this.mcpToolDescription = containerAssistTool.description;
  }

  getContainerAssistApp(): AppRuntime {
    return this.containerAssistApp;
  }

  getContainerAssistToolName(): ToolName {
    return this.containerAssistToolName;
  }

  getName(): string {
    return this.overrideName;
  }

  getDescription(): string {
    return this.mcpToolDescription;
  }

  getInputSchema(): ZodRawShape {
    return this.schema;
  }

  updateTelemetryPropertiesWithInput(params: Record<string, unknown> | undefined, telemetryProperties: ContainerAssistToolTelemetryProperties): void {
    telemetryProperties.serializedInput = params ? JSON.stringify(params) : '';
    telemetryProperties.responseLatency = Date.now() - (new Date(telemetryProperties.time)).getTime();
    telemetryProperties.resultCode = ResultCode.InProgress;
  }

  updateTelemetryPropertiesWithToolResponse(output: string, telemetryProperties: ContainerAssistToolTelemetryProperties): void {
    telemetryProperties.serializedOutput = output;
    telemetryProperties.responseLatency = Date.now() - (new Date(telemetryProperties.time)).getTime();
    telemetryProperties.resultCode = ResultCode.Success;
  }

  updateTelemetryPropertiesWithError(error: unknown, telemetryProperties: ContainerAssistToolTelemetryProperties): void {
    const errorMessage = error instanceof Error ? error.message : String(error);

    telemetryProperties.responseLatency = Date.now() - (new Date(telemetryProperties.time)).getTime();
    telemetryProperties.error = errorMessage;
    telemetryProperties.resultCode = ResultCode.Failure;
  }

  async handler(params: Record<string, unknown> | undefined, extra: RequestHandlerExtra<ServerRequest, ServerNotification>): Promise<CallToolResult> {
    const telemetryProperties: ContainerAssistToolTelemetryProperties = {
      time: (new Date()).toUTCString(),
      responseLatency: 0,
      resultCode: ResultCode.InProgress,
      devDeviceId: "FakeDeviceId",
      correlationId: "FakeCorrelationId",
    };

    this.updateTelemetryPropertiesWithInput(params, telemetryProperties);

    const telemetryOptions: ToolHandlerOptions = {
      transport: 'app-modernization-mcp',

      // Track successful executions
      // Note: The toolName parameter from createToolHandler will be the Container Assist
      // tool name ('analyze-repo'), but telemetryProperties.toolName contains the MCP
      // override name ('appmod-analyze-repository') for proper telemetry tracking
      onSuccess: (_result, _toolName, _params) => {
        const resultString = _result != null ? JSON.stringify(_result) : '';
        this.updateTelemetryPropertiesWithToolResponse(resultString, telemetryProperties);
      },

      // Track errors
      onError: (error, _toolName, _params) => {
        const errorMessage = error instanceof Error ? error.message : String(error);
        this.updateTelemetryPropertiesWithError(errorMessage, telemetryProperties);
      },
    };

    // Use the actual Container Assist tool name for execution, not the MCP override name
    const handler = createToolHandler(this.getContainerAssistApp(), this.getContainerAssistToolName(), telemetryOptions);
    return await handler(params, extra);
  }
}