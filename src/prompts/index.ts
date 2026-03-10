/**
 * MCP Prompts Registration
 *
 * Registers all reusable prompts with the MCP server.
 * Prompts return seeded conversation messages that guide an LLM through
 * multi-step containerization workflows using the available MCP tools.
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { localKindDevLoopSchema } from './kind-loop/schema';
import { buildLocalKindDevLoopPrompt } from './kind-loop/prompt';
import { aksRemoteDevLoopSchema } from './aks-loop/schema';
import { buildAksRemoteDevLoopPrompt } from './aks-loop/prompt';

/**
 * Register all MCP prompts on the given server instance.
 */
export function registerPrompts(server: McpServer): void {
  // --- kind-loop ---
  server.registerPrompt(
    'kind-loop',
    {
      description:
        'Drive a full local Kind cluster development iteration loop: analyze, build, scan, deploy, and verify using containerization-assist tools',
      argsSchema: localKindDevLoopSchema,
    },
    (args) => ({
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
  server.registerPrompt(
    'aks-loop',
    {
      description:
        'Drive a full AKS remote cluster deployment iteration loop: analyze, build, scan, push to ACR, deploy, and verify using containerization-assist tools',
      argsSchema: aksRemoteDevLoopSchema,
    },
    (args) => ({
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
