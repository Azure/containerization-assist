import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';
import { z } from 'zod';
import type { Tool } from '@/types/tool';
import { Success } from '@/types';
import type { Logger } from 'pino';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';

import * as OrchestratorModule from '@/app/orchestrator';
import * as MCPServerModule from '@/mcp/mcp-server';
import { registerToolsWithServer, OUTPUTFORMAT } from '@/mcp/mcp-server';
import { CHAINHINTSMODE } from '@/app/orchestrator-types';
import { createApp } from '@/app';

const { createOrchestrator } = OrchestratorModule;
const { createMCPServer, registerToolsWithServer } = MCPServerModule;

let orchestratorExecute: jest.Mock;
let orchestratorClose: jest.Mock;
let createOrchestratorSpy: jest.SpiedFunction<typeof createOrchestrator>;
let createMCPServerSpy: jest.SpiedFunction<typeof createMCPServer>;
let registerToolsSpy: jest.SpiedFunction<typeof registerToolsWithServer>;

function createTool(name: string): Tool<ReturnType<typeof z.object>, unknown> {
  return {
    name,
    description: `${name} description`,
    version: '1.0.0',
    category: 'testing',
    schema: z.object({ foo: z.string().optional() }),
    run: jest.fn(),
  };
}

function createLoggerStub(): Logger {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as unknown as Logger;
}

beforeEach(() => {
  orchestratorExecute = jest.fn().mockResolvedValue(Success({ ok: true }));
  orchestratorClose = jest.fn();

  createOrchestratorSpy = jest
    .spyOn(OrchestratorModule, 'createOrchestrator')
    .mockReturnValue({
      execute: orchestratorExecute,
      close: orchestratorClose,
    });

  createMCPServerSpy = jest
    .spyOn(MCPServerModule, 'createMCPServer')
    .mockImplementation(() => ({
      start: jest.fn().mockResolvedValue(undefined),
      stop: jest.fn().mockResolvedValue(undefined),
      getServer: jest.fn().mockReturnValue({} as Server),
      getTools: jest.fn().mockReturnValue([]),
    }));

  registerToolsSpy = jest
    .spyOn(MCPServerModule, 'registerToolsWithServer')
    .mockImplementation(() => undefined);
});

afterEach(() => {
  jest.restoreAllMocks();
});

describe('createApp AppRuntime interface', () => {
  it('should implement AppRuntime interface correctly', () => {
    const tool = createTool('demo');
    const app = createApp({ tools: [tool], logger: createLoggerStub() });

    expect(app).toHaveProperty('execute');
    expect(app).toHaveProperty('listTools');
    expect(app).toHaveProperty('startServer');
    expect(app).toHaveProperty('bindToMCP');
    expect(app).toHaveProperty('healthCheck');
    expect(app).toHaveProperty('stop');
    expect(app).toHaveProperty('getLogFilePath');
  });

  it('should list tools with correct metadata', () => {
    const tool = createTool('test-tool');
    const app = createApp({ tools: [tool], logger: createLoggerStub() });

    const tools = app.listTools();
    expect(Array.isArray(tools)).toBe(true);
    expect(tools.length).toBe(1);

    const listedTool = tools[0];
    expect(listedTool.name).toBe('test-tool');
    expect(listedTool.description).toBe('test-tool description');
    expect(typeof listedTool.version).toBe('string');
    expect(typeof listedTool.category).toBe('string');
  });

  it('should perform health check without server', async () => {
    const tool = createTool('health-tool');
    const app = createApp({ tools: [tool], logger: createLoggerStub() });

    const health = await app.healthCheck();
    expect(health).toHaveProperty('status');
    expect(health).toHaveProperty('tools');
    expect(health).toHaveProperty('message');
    expect(health.status).toMatch(/^(healthy|unhealthy)$/);
    expect(health.tools).toBe(1);
  });


  it('should not write to stdout during programmatic execution', async () => {
    const originalStdout = process.stdout.write;
    let stdoutCalls = 0;

    process.stdout.write = function (...args) {
      stdoutCalls++;
      return originalStdout.apply(this, args);
    };

    try {
      const tool = createTool('stdio-test');
      const app = createApp({ tools: [tool], logger: createLoggerStub() });

      await app.execute(tool.name, { foo: 'test' });

      // Verify no stdout output for programmatic execution
      expect(stdoutCalls).toBe(0);
    } finally {
      process.stdout.write = originalStdout;
    }
  });
});

describe('createApp orchestration integration', () => {
  it('routes programmatic execute through the orchestrator', async () => {
    const tool = createTool('demo');
    const app = createApp({ tools: [tool], logger: createLoggerStub() });

    await app.execute(tool.name, { foo: 'bar' });

    expect(orchestratorExecute).toHaveBeenCalledWith(
      expect.objectContaining({
        toolName: tool.name,
        params: { foo: 'bar' },
        metadata: {
          loggerContext: { transport: 'programmatic' },
        },
      }),
    );
  });

  it('provides orchestrator executor to createMCPServer during startServer', async () => {
    const tool = createTool('start-demo');
    const fakeServer: { start: jest.Mock; stop: jest.Mock; getServer: jest.Mock; getTools: jest.Mock } = {
      start: jest.fn().mockResolvedValue(undefined),
      stop: jest.fn().mockResolvedValue(undefined),
      getServer: jest.fn().mockReturnValue({} as Server),
      getTools: jest.fn().mockReturnValue([]),
    };
    createMCPServerSpy.mockReturnValue(fakeServer);

    const app = createApp({ tools: [tool], logger: createLoggerStub() });
    await app.startServer({ transport: 'stdio' });

    expect(createMCPServerSpy).toHaveBeenCalledTimes(1);
    const executor = createMCPServerSpy.mock.calls[0][2];
    expect(typeof executor).toBe('function');

    await executor({ toolName: tool.name, params: { foo: 'baz' } });
    expect(orchestratorExecute).toHaveBeenCalledWith(
      expect.objectContaining({
        toolName: tool.name,
        params: { foo: 'baz' },
      }),
    );
  });

  it('closes orchestrator resources when stop is invoked', async () => {
    const tool = createTool('stop-demo');
    const app = createApp({ tools: [tool], logger: createLoggerStub() });

    await app.stop();

    expect(orchestratorClose).toHaveBeenCalledTimes(1);
  });

  it('reinitializes orchestrator after stop before executing again', async () => {
    const tool = createTool('restart-demo');
    const app = createApp({ tools: [tool], logger: createLoggerStub() });

    const initialCalls = createOrchestratorSpy.mock.calls.length;

    await app.stop();
    await app.execute(tool.name, { foo: 'later' });

    expect(createOrchestratorSpy.mock.calls.length).toBe(initialCalls + 1);
  });

  it('registers tools with external MCP server via orchestrator executor', () => {
    const tool = createTool('bind-demo');
    const app = createApp({ tools: [tool], logger: createLoggerStub() });

    const fakeServer = {
      tool: jest.fn(),
    } as unknown as McpServer;

    app.bindToMCP(fakeServer, 'external');

    expect(registerToolsSpy).toHaveBeenCalledWith({
      outputFormat: OUTPUTFORMAT.NATURAL_LANGUAGE,
      chainHintsMode: CHAINHINTSMODE.ENABLED,
      server: fakeServer,
      tools: expect.any(Array),
      logger: expect.any(Object),
      transport: 'external',
      execute: expect.any(Function),
    });
  });
});

describe('createApp tool aliases', () => {
  it('should apply tool aliases correctly', () => {
    const tool1 = createTool('analyze-repo');
    const tool2 = createTool('build-image');
    const app = createApp({
      tools: [tool1, tool2],
      toolAliases: {
        'analyze-repo': 'project_analyze',
        'build-image': 'docker_build',
      },
      logger: createLoggerStub(),
    });

    const tools = app.listTools();
    expect(tools).toHaveLength(2);

    const toolNames = tools.map(t => t.name);
    expect(toolNames).toContain('project_analyze');
    expect(toolNames).toContain('docker_build');
    expect(toolNames).not.toContain('analyze-repo');
    expect(toolNames).not.toContain('build-image');
  });

  it('should preserve tools without aliases', () => {
    const tool1 = createTool('analyze-repo');
    const tool2 = createTool('build-image');
    const app = createApp({
      tools: [tool1, tool2],
      toolAliases: {
        'analyze-repo': 'project_analyze',
      },
      logger: createLoggerStub(),
    });

    const tools = app.listTools();
    expect(tools).toHaveLength(2);

    const toolNames = tools.map(t => t.name);
    expect(toolNames).toContain('project_analyze');
    expect(toolNames).toContain('build-image'); // Not aliased
    expect(toolNames).not.toContain('analyze-repo');
  });

  it('should execute tools using aliased names', async () => {
    const tool = createTool('analyze-repo');
    const app = createApp({
      tools: [tool],
      toolAliases: {
        'analyze-repo': 'project_analyze',
      },
      logger: createLoggerStub(),
    });

    await app.execute('project_analyze' as any, { foo: 'bar' });

    expect(orchestratorExecute).toHaveBeenCalledWith(
      expect.objectContaining({
        toolName: 'project_analyze',
        params: { foo: 'bar' },
      }),
    );
  });

  it('should handle empty tool aliases', () => {
    const tool = createTool('analyze-repo');
    const app = createApp({
      tools: [tool],
      toolAliases: {},
      logger: createLoggerStub(),
    });

    const tools = app.listTools();
    expect(tools).toHaveLength(1);
    expect(tools[0].name).toBe('analyze-repo');
  });

  it('should handle undefined tool aliases', () => {
    const tool = createTool('analyze-repo');
    const app = createApp({
      tools: [tool],
      logger: createLoggerStub(),
    });

    const tools = app.listTools();
    expect(tools).toHaveLength(1);
    expect(tools[0].name).toBe('analyze-repo');
  });
});
