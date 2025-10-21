/**
 * LLM Client Types and Interfaces
 * Abstractions for different LLM providers in testing
 */

export interface LLMMessage {
  role: 'user' | 'assistant' | 'system' | 'tool';
  content: string;
  toolCallId?: string;
  name?: string;
  toolCalls?: ToolCall[];
}

export interface ToolCall {
  id: string;
  name: string;
  arguments: Record<string, unknown>;
}

export interface ToolResponse {
  toolCallId: string;
  toolName: string;
  content: unknown;
  error?: string;
}

export interface LLMResponse {
  content: string;
  toolCalls: ToolCall[];
  finishReason: 'stop' | 'tool_calls' | 'length' | 'error';
  usage?: {
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
  };
  metadata?: {
    model: string;
    latency: number;
    timestamp: Date;
  };
}

export interface ConversationSession {
  id: string;
  messages: LLMMessage[];
  toolCalls: ToolCall[];
  toolResponses: ToolResponse[];
  metadata: {
    model: string;
    createdAt: string;
  };
}

export interface LLMClientConfig {
  model: string;
  apiKey: string;
  baseURL?: string;
  timeout?: number;
  maxRetries?: number;
}

export interface LLMClient {
  readonly model: string;

  /**
   * Send a message and get a response
   */
  sendMessage(
    messages: LLMMessage[],
    options?: {
      tools?: ToolDefinition[];
      temperature?: number;
      maxTokens?: number;
    }
  ): Promise<LLMResponse>;

  /**
   * Create a new conversation session
   */
  createSession(): ConversationSession;

  /**
   * Continue an existing conversation
   */
  continueSession(
    session: ConversationSession,
    message: string,
    toolResponses?: ToolResponse[]
  ): Promise<LLMResponse>;

  /**
   * Validate that the client is properly configured
   */
  validateConnection(): Promise<boolean>;
}

export interface ToolDefinition {
  name: string;
  description: string;
  inputSchema: {
    type: 'object';
    properties: Record<string, unknown>;
    required?: string[];
  } | any; // Allow Zod schema as well
  zodSchema?: any; // Optional Zod schema for direct access
}

export interface LLMTestContext {
  client: LLMClient;
  session: ConversationSession;
  mcpServer: MCPTestServer;
}

export interface MCPTestServer {
  url: string;
  tools: ToolDefinition[];
  executeToolCall(toolCall: ToolCall): Promise<ToolResponse>;
}

export interface ClientValidationResult {
  valid: boolean;
  error?: string;
  latency: number;
}