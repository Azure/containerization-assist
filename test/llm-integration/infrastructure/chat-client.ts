/**
 * Chat Client for LLM Integration Testing
 * Uses Vercel AI SDK for enhanced LLM interactions with native tool calling and streaming
 */

import { promises as fs } from 'fs';
import { join } from 'path';
import { generateText, tool, Experimental_Agent as Agent, stepCountIs } from 'ai';
import { createAzure } from '@ai-sdk/azure';
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

  constructor() {
    // Get Azure configuration from environment variables, validated to fail fast
    const model = process.env.AZURE_OPENAI_DEPLOYMENT_ID;
    const resourceName = process.env.AZURE_OPENAI_RESOURCE;
    const apiKey = process.env.AZURE_OPENAI_API_KEY;

    if (!resourceName || !apiKey || !model) {
      throw new Error('AZURE_OPENAI_RESOURCE, AZURE_OPENAI_API_KEY, and AZURE_OPENAI_DEPLOYMENT_ID environment variables must be set');
    }

    this.model = model;

    console.log('🔧 Azure Config: Using resourceName:', resourceName);

    this.provider = createAzure({
      resourceName,
      apiKey,
    });
  }

  async validateConnection(): Promise<boolean> {
    try {
      // Simple test message to validate the connection using AI SDK
      console.log('AI SDK: Attempting validation with model:', this.model);

      const result = await generateText({
        model: this.provider(this.model), // Use deployment name directly
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
          execute: async () => ({ placeholder: true }) // Placeholder - execution handled by test harness
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
              output: { type: 'text' as const, value: msg.content }, // Proper AI SDK format
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
   * Execute a request autonomously using AI SDK's Agent class with multi-step tool orchestration.
   * The Agent will automatically call tools, process responses, and continue until task completion.
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

    // Load system prompt from gpt-5.txt file as-is
    let systemPrompt: string;
    try {
      const promptPath = join(__dirname, 'gpt-5.txt');
      systemPrompt = await fs.readFile(promptPath, 'utf8');
      console.log('📝 Loaded GitHub Copilot system prompt from gpt-5.txt');
    } catch (error) {
      console.warn('⚠️ Failed to load gpt-5.txt, using fallback prompt');
      systemPrompt = 'You are GitHub Copilot, an expert AI programming assistant working in VS Code.';
    }

    try {
      console.log('🔧 Available SDK Tools:', Object.keys(sdkTools));

      // Create Agent with multi-step orchestration
      const agent = new Agent({
        model: this.provider(this.model),
        system: systemPrompt,
        tools: sdkTools,
        stopWhen: stepCountIs(10), // Allow up to 10 steps for complete workflow
        // Optional: Add step-by-step hooks for progress tracking
        onStepFinish: ({ text, toolCalls }) => {
          console.log(`🔄 Step completed:`);
          if (toolCalls?.length) {
            console.log(`   🔧 Tools called: ${toolCalls.map(tc => tc.toolName).join(', ')}`);
          }
          if (text) {
            console.log(`   💭 Reasoning: ${text.substring(0, 100)}...`);
          }
        }
      });

      // Execute the agent autonomously - let it handle the workflow naturally
      console.log('🚀 Agent starting autonomous execution...');

      const result = await agent.generate({
        prompt: userMessage
      });

      const { text, steps } = result;
      const finalResponse = text || 'Task completed successfully.';

      console.log(`📊 Agent completed workflow with ${steps.length} steps`);

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