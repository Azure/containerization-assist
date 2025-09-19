/**
 * Canonical MCP Server Entry
 *
 * Responsibilities:
 * - Build the server with a single factory: createServer(...)
 * - Precompute tool schemas once (no per-request synthesis)
 * - Provide minimal MCP handlers: tools/list, tools/call
 * - Keep orchestration thin; delegate execution to router
 *
 * Out of scope (by design):
 * - UX/workflow hints (compose outside)
 * - AI assistance policy (compose outside)
 * - Transport wiring (HTTP/stdio/websocket composed by host)
 */

import type { Logger } from 'pino';
import type { JSONSchema7 } from 'json-schema';
import { zodToJsonSchema } from 'zod-to-json-schema';
import type { Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { SessionManager } from '@/lib/session';
import { createToolRouter, type ToolRouter, type RouterTool } from '@/mcp/tool-router';
import type { ToolName } from '@/exports/tools';

// ---- Types ------------------------------------------------------------------

export type ToolDef = RouterTool & {
  // Optional name override; otherwise `name` is required in RouterTool
  name?: string;
};

export interface CreateServerOptions {
  logger: Logger;
  sessionManager: SessionManager;

  /**
   * Tools registry. Prefer constructing this map in a dedicated registry module:
   *   import { registry } from '@/mcp/tools/registry'
   *   tools: registry
   */
  tools: Map<string, ToolDef>;

  /**
   * Optional context factory to enrich ToolContext (e.g., inject k8s client).
   * If omitted, a minimal context with { logger, sessionId } is provided.
   */
  makeContext?: (sessionId: string | undefined) => ToolContext;
}

export interface McpServer {
  /**
   * Handle an MCP JSON-RPC request. You can wire this to any transport
   * (stdio, WebSocket, HTTP) in a thin adapter layer.
   */
  handleRequest: (req: McpRequest) => Promise<McpResponse>;

  /**
   * Access to internals for testing/composition.
   */
  router: ToolRouter;
  listTools: () => ToolListResponse['tools'];
}

// ---- Minimal MCP-ish request/response shapes --------------------------------

type McpRequest =
  | {
      method: 'initialize';
      id?: string | number;
      params?: { protocolVersion?: string; clientInfo?: any };
    }
  | { method: 'tools/list'; id?: string | number }
  | {
      method: 'tools/call';
      id?: string | number;
      params: { name: string; arguments?: Record<string, unknown>; sessionId?: string };
    }
  | { method: 'ping'; id?: string | number };

type McpResponse =
  | { id?: string | number; result: unknown }
  | { id?: string | number; error: { code: number; message: string } };

type ToolListItem = {
  name: string;
  // Keep schema value JSON-serializable; we store zod-inferred JSON schema result
  inputSchema?: JSONSchema7;
};

type ToolListResponse = {
  tools: ToolListItem[];
};

// ---- Server factory ----------------------------------------------------------

export function createServer(opts: CreateServerOptions): McpServer {
  const { logger, sessionManager, tools, makeContext } = opts;

  logger.debug(`Creating MCP server with tools: ${Array.from(tools.keys()).join(', ')}`);

  // Freeze registry and compute schemas ONCE.
  const frozenTools = new Map<string, ToolDef>();
  const routerTools = new Map<ToolName, RouterTool>();
  for (const [k, v] of tools.entries()) {
    const name = v.name ?? k;
    if (!name) throw new Error('Tool missing name');
    frozenTools.set(name, { ...v, name });
    routerTools.set(name as ToolName, { ...v, name });
  }
  Object.freeze(frozenTools);
  Object.freeze(routerTools);

  logger.debug(`Frozen tools: ${frozenTools.size}`);

  const toolSchemas = precomputeSchemas(frozenTools, logger);
  logger.debug(`Tool schemas computed: ${toolSchemas.size}`);

  const router = createToolRouter({ sessionManager, logger, tools: routerTools });
  logger.debug('Tool router created');

  const listTools = (): ToolListResponse['tools'] =>
    [...frozenTools.values()].map((t) => ({
      name: t.name,
      inputSchema: toolSchemas.get(t.name) ?? undefined,
    }));

  const handleRequest = async (req: McpRequest): Promise<McpResponse> => {
    try {
      switch (req.method) {
        case 'initialize': {
          const result = {
            protocolVersion: '2024-11-05',
            capabilities: {
              tools: {},
            },
            serverInfo: {
              name: 'containerization-assist',
              version: '1.4.2',
            },
          };
          return req.id !== undefined ? { id: req.id, result } : { result };
        }

        case 'ping':
          return req.id !== undefined
            ? { id: req.id, result: { ok: true } }
            : { result: { ok: true } };

        case 'tools/list': {
          return req.id !== undefined
            ? { id: req.id, result: { tools: listTools() } }
            : { result: { tools: listTools() } };
        }

        case 'tools/call': {
          const name = req.params?.name;
          const args = req.params?.arguments ?? {};
          const sessionId = req.params?.sessionId;
          if (!name) return error(req, -32602, 'Missing tool name');

          // Inject a minimal, idiomatic ToolContext; allow composition to enrich.
          const baseContext: ToolContext =
            makeContext?.(sessionId) ??
            ({
              logger,
              sessionId,
            } as unknown as ToolContext);

          const result = await router.route({
            toolName: name as ToolName,
            params: args,
            sessionId: sessionId ?? undefined,
            context: baseContext,
          });

          if (result.ok)
            return req.id !== undefined
              ? { id: req.id, result: unwrap(result) }
              : { result: unwrap(result) };
          return error(req, 400, result.error ?? `Tool "${name}" failed`);
        }

        default:
          return error(req, -32601, `Method not found: ${(req as any).method}`);
      }
    } catch (e: any) {
      logger.error({ err: e }, 'Unhandled server error');
      return error(req, -32000, e?.message ?? 'Internal error');
    }
  };

  return { handleRequest, router, listTools };
}

// ---- Helpers -----------------------------------------------------------------

function error(req: { id?: string | number }, code: number, message: string): McpResponse {
  return req.id !== undefined
    ? { id: req.id, error: { code, message } }
    : { error: { code, message } };
}

function unwrap<T>(r: Result<T>): T {
  if (r.ok) return r.value;
  throw new Error(r.error ?? 'Unknown error');
}

/**
 * Convert zod schemas to valid JSON Schema 7 format up-front.
 * If a tool has no schema, omit it from the list to keep payloads lean.
 */
function precomputeSchemas(tools: Map<string, ToolDef>, logger: Logger): Map<string, JSONSchema7> {
  const out = new Map<string, JSONSchema7>();
  for (const [name, tool] of tools) {
    if (!tool.schema) continue;
    try {
      // First try explicit jsonSchema property
      if ((tool as any).jsonSchema) {
        out.set(name, (tool as any).jsonSchema);
        continue;
      }

      // Then try schema toJSON method
      if (typeof (tool.schema as any).toJSON === 'function') {
        out.set(name, (tool.schema as any).toJSON());
        continue;
      }

      // Use zod-to-json-schema for proper conversion
      const jsonSchema = zodToJsonSchema(tool.schema, {
        name,
        target: 'jsonSchema7',
        $refStrategy: 'none', // Inline all references for simplicity
      });

      out.set(name, jsonSchema as JSONSchema7);
    } catch (e: any) {
      logger.warn({ tool: name, err: e?.message }, 'Failed to serialize tool schema; omitting');
    }
  }
  return out;
}
