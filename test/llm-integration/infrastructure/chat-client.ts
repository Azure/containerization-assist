/**
 * Chat Client for LLM Integration Testing
 * Uses Vercel AI SDK for enhanced LLM interactions with native tool calling and streaming
 */

import { promises as fs } from 'fs';
import { join } from 'path';
import { generateText, tool } from 'ai';
import { createAzure } from '@ai-sdk/azure';
import { createOpenAI } from '@ai-sdk/openai';
import { z } from 'zod';
import type {
  LLMClient,
  LLMMessage,
  LLMResponse,
  ConversationSession,
  ToolResponse,
  ToolDefinition,
  ToolCall,
} from './llm-client-types.js';

export class ChatClient implements LLMClient {
  readonly model: string;
  private readonly provider: any;
  private readonly isAzure: boolean;

  constructor(
    options: {
      model?: string;
      apiKey?: string;
      baseURL?: string;
    } = {}
  ) {
    // Get configuration from environment variables, validated to fail fast
    const model = process.env.OPENAI_DEPLOYMENT_ID;
    const baseURL = process.env.OPENAI_BASE_URL;
    const apiKey = process.env.OPENAI_API_KEY;

    if (!baseURL || !apiKey || !model) {
      throw new Error('OPENAI_BASE_URL, OPENAI_API_KEY, and OPENAI_DEPLOYMENT_ID environment variables must be set');
    }

    // STRICT: Prevent any mock endpoint usage in LLM integration tests
    if (baseURL.includes('localhost') || baseURL.includes('127.0.0.1') || baseURL.includes('4141')) {
      throw new Error('❌ MOCK ENDPOINTS FORBIDDEN: LLM integration tests must use real Azure credentials. Run "source ../azure_keys.sh" to load proper credentials.');
    }

    this.model = model;
    this.isAzure = baseURL.includes('cognitiveservices.azure.com');

    if (this.isAzure) {
      // Extract resource name from Azure URL (e.g., "containerization-assist-e2e" from the URL)
      const resourceMatch = baseURL.match(/https:\/\/([^.]+)\.cognitiveservices\.azure\.com/);
      const resourceName = resourceMatch ? resourceMatch[1] : '';

      this.provider = createAzure({
        resourceName,
        apiKey,
        apiVersion: '2024-08-01-preview', // Use working API version for GPT-5 support
        useDeploymentBasedUrls: true, // Enable deployment-based URLs for Azure
      });
    } else {
      this.provider = createOpenAI({
        baseURL,
        apiKey,
      });
    }
  }

  async validateConnection(): Promise<boolean> {
    try {
      // Simple test message to validate the connection using AI SDK
      console.log('AI SDK: Attempting validation with model:', this.model);

      const result = await generateText({
        model: this.provider(this.model), // Pass deployment name to provider
        prompt: 'Say hi',
        maxOutputTokens: 200,
      });

      console.log('Chat Client: Direct validation result:', result.text);

      // For GPT-5 reasoning models, validate that we got a proper response
      const hasText = result.text !== undefined && result.text !== null && result.text.length > 0;
      const hasUsage = result.usage?.totalTokens ? result.usage.totalTokens > 0 : false;

      console.log('Validation checks:', { hasText, hasUsage, textLength: result.text?.length });
      return hasText && hasUsage;
    } catch (error) {
      console.warn(`Chat client validation failed:`, error instanceof Error ? error.message : String(error));
      console.error('Full error details:', error);
      return false;
    }
  }

  createSession(): ConversationSession {
    return {
      id: `chat-session-${Date.now()}`,
      messages: [],
      toolCalls: [],
      toolResponses: [],
      metadata: {
        model: this.model,
        createdAt: new Date().toISOString(),
      },
    };
  }

  async sendMessage(
    messages: LLMMessage[],
    options: {
      tools?: ToolDefinition[];
      temperature?: number;
      maxTokens?: number;
    } = {}
  ): Promise<LLMResponse> {
    const startTime = Date.now();
    return this.sendChatMessage(messages, options, startTime);
  }




  private async sendChatMessage(
    messages: LLMMessage[],
    options: {
      tools?: ToolDefinition[];
      temperature?: number;
      maxTokens?: number;
    },
    startTime: number
  ): Promise<LLMResponse> {
    // Convert MCP tools to AI SDK format if provided
    const sdkTools: Record<string, any> = {};

    if (options.tools && options.tools.length > 0) {
      for (const toolDef of options.tools) {
        const schema = toolDef.zodSchema || toolDef.inputSchema;
        sdkTools[toolDef.name] = tool({
          description: toolDef.description,
          inputSchema: schema,
          execute: async (params: any) => {
            // Return placeholder - actual execution happens in test harness
            return { success: true, toolCall: toolDef.name, params };
          }
        });
      }
    }

    try {
      // Convert LLMMessage[] to AI SDK messages format
      const aiMessages = messages.map(msg => {
        if (msg.role === 'tool') {
          return {
            role: 'tool' as const,
            content: [{
              type: 'tool-result' as const,
              toolCallId: msg.toolCallId || '',
              toolName: msg.name || 'unknown',
              result: msg.content,
              output: { type: 'text' as const, value: msg.content },
            }],
          };
        }
        return {
          role: msg.role as 'user' | 'assistant' | 'system',
          content: msg.content,
        };
      });

      const result = await generateText({
        model: this.provider(this.model),
        messages: aiMessages,
        tools: Object.keys(sdkTools).length > 0 ? sdkTools : undefined,
        maxOutputTokens: options.maxTokens || 4000,
        temperature: options.temperature,
      });

      const endTime = Date.now();

      // Convert AI SDK tool calls to our format
      const toolCalls: ToolCall[] = result.toolCalls?.map(tc => ({
        id: tc.toolCallId,
        name: tc.toolName,
        arguments: (tc as any).args || {},
      })) || [];

      return {
        content: result.text || '',
        toolCalls,
        finishReason: result.finishReason === 'tool-calls' ? 'tool_calls' :
                     result.finishReason === 'length' ? 'length' :
                     result.finishReason === 'error' ? 'error' : 'stop',
        usage: result.usage ? {
          promptTokens: result.usage.inputTokens || 0,
          completionTokens: result.usage.outputTokens || 0,
          totalTokens: (result.usage.inputTokens || 0) + (result.usage.outputTokens || 0),
        } : undefined,
        metadata: {
          model: this.model,
          latency: endTime - startTime,
          timestamp: new Date(),
        },
      };
    } catch (error) {
      throw new Error(`AI SDK error: ${error instanceof Error ? error.message : String(error)}`);
    }
  }


  async continueSession(
    session: ConversationSession,
    message: string,
    toolResponses?: ToolResponse[],
    availableTools: ToolDefinition[] = []
  ): Promise<LLMResponse> {
    // Check if the last message was an assistant message with tool calls
    const lastMessage = session.messages[session.messages.length - 1];
    const hasUnresolvedToolCalls = lastMessage?.role === 'assistant' &&
                                   lastMessage.toolCalls &&
                                   lastMessage.toolCalls.length > 0;

    // If we have tool responses, they must be added before any new user message
    if (toolResponses && toolResponses.length > 0) {
      for (const toolResponse of toolResponses) {
        session.messages.push({
          role: 'tool',
          content: toolResponse.error
            ? `Error: ${toolResponse.error}`
            : JSON.stringify(toolResponse.content, null, 2),
          toolCallId: toolResponse.toolCallId,
          name: toolResponse.toolName,
        });
        // Track tool responses in session
        session.toolResponses.push(toolResponse);
      }
    } else if (hasUnresolvedToolCalls) {
      // If there are unresolved tool calls but no tool responses provided,
      // this is a test design error - the test should provide proper tool responses
      throw new Error(
        `Session has unresolved tool calls but no tool responses were provided. ` +
        `Tool calls from last message: ${lastMessage.toolCalls!.map(tc => tc.name).join(', ')}. ` +
        `Tests must execute tool calls and provide responses before continuing conversation.`
      );
    }

    // Add system message on first message to encourage tool use
    if (session.messages.length === 0) {
      const toolNames = availableTools.map(t => t.name).join(', ');
      session.messages.push({
        role: 'system',
        content: `You are a helpful assistant. You have access to these tools: ${toolNames}. Use these tools when appropriate to complete user requests.`,
      });
    }

    // Add user message
    session.messages.push({
      role: 'user',
      content: message,
    });

    // Use the provided available tools from MCP harness
    const response = await this.sendMessage(session.messages, { tools: availableTools });

    // Update session with response
    session.messages.push({
      role: 'assistant',
      content: response.content,
      toolCalls: response.toolCalls,
    });

    session.toolCalls.push(...response.toolCalls);

    return response;
  }



  /**
   * Execute a request autonomously using AI SDK's native tool calling with generateText.
   * The AI will call tools, act on their responses, and create files as needed.
   */
  async executeAutonomously(
    userMessage: string,
    workingDirectory: string,
    mcpToolExecutor: (toolName: string, params: any) => Promise<any>,
    availableTools: ToolDefinition[] = [],
    options: {
      maxSteps?: number;
      temperature?: number;
    } = {}
  ): Promise<{
    response: string;
    filesCreated: string[];
    toolCallsExecuted: Array<{ toolName: string; params: any; result: any }>;
  }> {
    const filesCreated: string[] = [];
    const toolCallsExecuted: Array<{ toolName: string; params: any; result: any }> = [];

    console.log('🤖 Starting autonomous execution with AI SDK tool calling...');

    // Convert ToolDefinition[] to AI SDK tools format
    const sdkTools: Record<string, any> = {};

    // Add createFile tool
    sdkTools.createFile = tool({
      description: 'Create a file with the specified content. Use this to actually create Dockerfiles, configuration files, etc. based on information from other tools.',
      inputSchema: z.object({
        filePath: z.string().describe('Path where the file should be created (relative to working directory)'),
        content: z.string().describe('Content to write to the file'),
        reason: z.string().describe('Why this file is being created and what it accomplishes')
      }),
      execute: async ({ filePath, content, reason }) => {
        const fullPath = join(workingDirectory, filePath);
        await fs.writeFile(fullPath, content, 'utf8');
        filesCreated.push(fullPath);
        console.log(`📝 Created file: ${filePath} - ${reason}`);
        return {
          success: true,
          filePath: fullPath,
          message: `Successfully created ${filePath}. ${reason}`,
        };
      }
    });

    // Add MCP tools using zodSchema if available, otherwise fallback
    for (const toolDef of availableTools) {
      const schema = toolDef.zodSchema || toolDef.inputSchema;

      sdkTools[toolDef.name] = tool({
        description: toolDef.description,
        inputSchema: schema,
        execute: async (params: any) => {
          const result = await mcpToolExecutor(toolDef.name, params);
          toolCallsExecuted.push({ toolName: toolDef.name, params, result });
          return result;
        }
      });
    }

    // Generic system prompt that works with any available tools
    const systemPrompt = `You are an autonomous coding assistant. Complete the user's request using the available tools.

Available tools: ${availableTools.map(t => t.name).join(', ')}${availableTools.length > 0 ? ', ' : ''}createFile

Working directory: ${workingDirectory}

Use the tools as needed to complete the task. When the user expects files to be created, use the createFile tool to write them.`;

    try {
      console.log('🔧 Available SDK Tools:', Object.keys(sdkTools));

      // Use AI SDK's generateText with system and prompt (not messages array)
      const result = await generateText({
        model: this.provider(this.model),
        system: systemPrompt,
        prompt: userMessage,
        tools: sdkTools,
        maxOutputTokens: 4000,
        temperature: this.isGPT5() ? undefined : options.temperature,
      });

      console.log('📝 AI Response:', result.text);
      console.log('🔧 AI Tool Calls Count:', result.toolCalls?.length || 0);

      // Process any tool calls made by the AI
      if (result.toolCalls && result.toolCalls.length > 0) {
        for (const toolCall of result.toolCalls) {
          const toolArgs = (toolCall as any).args;
          console.log(`🔧 Calling tool: ${toolCall.toolName} with args:`, toolArgs);

          if (toolCall.toolName === 'createFile') {
            // createFile tool is handled internally by the AI SDK
            console.log(`📝 Created file: ${toolArgs.filePath}`);
            continue;
          }

          // Execute MCP tools through the provided executor
          const toolResult = await mcpToolExecutor(toolCall.toolName, toolArgs);
          toolCallsExecuted.push({ toolName: toolCall.toolName, params: toolArgs, result: toolResult });
          console.log(`✅ Tool ${toolCall.toolName} completed`);
        }
      }

      const finalResponse = result.text || 'Task completed successfully.';

      console.log('🎯 Autonomous execution completed!');
      console.log('🔧 Tools executed:', toolCallsExecuted.map(tc => tc.toolName));
      console.log('📁 Files created:', filesCreated);

      return {
        response: finalResponse || 'Task completed successfully.',
        filesCreated,
        toolCallsExecuted,
      };
    } catch (error) {
      console.error('❌ Agent execution error:', error);
      throw new Error(`Agent execution failed: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
}