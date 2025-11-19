export interface ITelemetryProperties {
  commandName?: string;
  functionName?: string;
  resultCode?: string;
  latency?: number;
  error?: string;
}

export enum ResultCode {
  PreInvoked = "PreInvoked",
  Invoked = "Invoked",
  InProgress = "InProgress",
  Success = "Success",
  Failure = "Failure",
  Cancelled = "Cancelled",
}

export interface McpTelemetryProperties extends ITelemetryProperties {
  time: string;
  responseLatency: number;
  resultCode: ResultCode;
  devDeviceId: string;
  correlationId: string;
  error?: string;
};

export interface ContainerAssistToolTelemetryProperties extends McpTelemetryProperties {
  serializedInput?: string;
  serializedOutput?: string;
}