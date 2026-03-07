/**
 * MCP Prompts Registration
 *
 * Registers all reusable prompts with the MCP server.
 * Prompts return seeded conversation messages that guide an LLM through
 * multi-step containerization workflows using the available MCP tools.
 *
 * NOTE: We register prompts with the zero-arg `server.prompt()` overload
 * and then mutate `argsSchema` / `callback` on the returned RegisteredPrompt.
 * This works around TS2589 ("Type instantiation is excessively deep") that
 * the CJS build's moduleResolution:"node" triggers when generic schema
 * inference flows through ShapeOutput<Args> in the MCP SDK type layer.
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import { localKindDevLoopSchema, type LocalKindDevLoopArgs } from './kind-loop/schema';
import { buildLocalKindDevLoopPrompt } from './kind-loop/prompt';
import { aksRemoteDevLoopSchema, type AksRemoteDevLoopArgs } from './aks-loop/schema';
import { buildAksRemoteDevLoopPrompt } from './aks-loop/prompt';

/**
 * Register all MCP prompts on the given server instance.
 */
export function registerPrompts(server: McpServer): void {
  // --- kind-loop ---
  const kindPrompt = server.prompt(
    'kind-loop',
    'Drive a full local Kind cluster development iteration loop: analyze, build, scan, deploy, and verify using containerization-assist tools',
    (_extra) => ({
      messages: [
        {
          role: 'user' as const,
          content: {
            type: 'text' as const,
            text: 'Use the kind-loop prompt with arguments to generate a local Kind cluster development workflow.',
          },
        },
      ],
    }),
  );
  // Assign schema and real callback at runtime to avoid TS2589 in CJS build.
  // The SDK handler checks `argsSchema` and calls `callback(parsedArgs, extra)`.
  kindPrompt.argsSchema = z.object(localKindDevLoopSchema) as never;
  kindPrompt.callback = ((args: LocalKindDevLoopArgs) => ({
    messages: [
      {
        role: 'user' as const,
        content: {
          type: 'text' as const,
          text: buildLocalKindDevLoopPrompt(args),
        },
      },
    ],
  })) as never;

  // --- aks-loop ---
  const aksPrompt = server.prompt(
    'aks-loop',
    'Drive a full AKS remote cluster deployment iteration loop: analyze, build, scan, push to ACR, deploy, and verify using containerization-assist tools',
    (_extra) => ({
      messages: [
        {
          role: 'user' as const,
          content: {
            type: 'text' as const,
            text: 'Use the aks-loop prompt with arguments to generate an AKS remote deployment workflow.',
          },
        },
      ],
    }),
  );
  aksPrompt.argsSchema = z.object(aksRemoteDevLoopSchema) as never;
  aksPrompt.callback = ((args: AksRemoteDevLoopArgs) => ({
    messages: [
      {
        role: 'user' as const,
        content: {
          type: 'text' as const,
          text: buildAksRemoteDevLoopPrompt(args),
        },
      },
    ],
  })) as never;
}
