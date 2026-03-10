/**
 * MCP Prompts Registration
 *
 * Registers all reusable prompts with the MCP server.
 * Prompts return seeded conversation messages that guide an LLM through
 * multi-step containerization workflows using the available MCP tools.
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { type LocalKindDevLoopArgs, localKindDevLoopSchema } from './kind-loop/schema';
import { buildLocalKindDevLoopPrompt } from './kind-loop/prompt';
import { type AksRemoteDevLoopArgs, aksRemoteDevLoopSchema } from './aks-loop/schema';
import { buildAksRemoteDevLoopPrompt } from './aks-loop/prompt';

/**
 * Helper to register an MCP prompt without triggering TS2589.
 *
 * The SDK's `server.registerPrompt()` generic infers `ShapeOutput<Args>`
 * which recurses through `SchemaOutput` Zod v3/v4 compat conditional types.
 * Under `moduleResolution: "node"` (CJS build) this exceeds TypeScript's
 * type-instantiation depth limit. Same pattern as tool-registration.ts L270.
 *
 * Workaround: call `server.registerPrompt()` through a loosely-typed wrapper
 * so the generic is not inferred from the schema literal. The callback is
 * explicitly typed at each call site for full type safety.
 */
type AnyPromptSchema = Record<string, any>;

function registerPrompt(
  server: McpServer,
  name: string,
  description: string,
  argsSchema: AnyPromptSchema,
  cb: (args: any) => { messages: Array<{ role: 'user'; content: { type: 'text'; text: string } }> },
): void {
  server.registerPrompt(name, { description, argsSchema }, cb);
}

/**
 * Register all MCP prompts on the given server instance.
 */
export function registerPrompts(server: McpServer): void {
  // --- kind-loop ---
  registerPrompt(
    server,
    'kind-loop',
    'Drive a full local Kind cluster development iteration loop: analyze, build, scan, deploy, and verify using containerization-assist tools',
    localKindDevLoopSchema,
    (args: LocalKindDevLoopArgs) => ({
      messages: [
        {
          role: 'user' as const,
          content: {
            type: 'text' as const,
            text: buildLocalKindDevLoopPrompt(args),
          },
        },
      ],
    }),
  );

  // --- aks-loop ---
  registerPrompt(
    server,
    'aks-loop',
    'Drive a full AKS remote cluster deployment iteration loop: analyze, build, scan, push to ACR, deploy, and verify using containerization-assist tools',
    aksRemoteDevLoopSchema,
    (args: AksRemoteDevLoopArgs) => ({
      messages: [
        {
          role: 'user' as const,
          content: {
            type: 'text' as const,
            text: buildAksRemoteDevLoopPrompt(args),
          },
        },
      ],
    }),
  );
}
