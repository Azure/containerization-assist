/**
 * kind-loop prompt
 *
 * Returns a seeded user message that drives a full local Kind cluster
 * development iteration loop using the containerization-assist MCP tools.
 */

import { TOOL_NAME } from '@/tools';
import { buildLoopPrompt } from '../shared/build-loop-prompt';
import type { LocalKindDevLoopArgs } from './schema';

/**
 * Build the prompt text for a local Kind development loop.
 */
export function buildLocalKindDevLoopPrompt(args: LocalKindDevLoopArgs): string {
  const nsClause = args.namespace
    ? `Use the namespace **${args.namespace}**.`
    : 'Generate a unique namespace name (e.g., `dev-<short-hash>`) for isolation.';
  const imageClause = args.imageName
    ? `Use the image name **${args.imageName}**.`
    : 'Derive the image name from the repository directory name.';

  return buildLoopPrompt(nsClause, imageClause, {
    title: 'local Kind cluster development iteration loop',
    contextLines: [
      '- Target environment: **development** (local Kind cluster with local image registry)',
      '- Use the **local system architecture** for Kind testing (detect it automatically).',
    ],
    buildPlatform:
      'the **local system platform** (e.g., \\`linux/arm64\\` or \\`linux/amd64\\` — detect automatically)',
    prepareStep: [
      `1. Call **${TOOL_NAME.PREPARE_CLUSTER}** with:`,
      '   - \\`clusterType: "kind"\\` (this creates a local Kind cluster with a local registry)',
      '   - \\`namespace\\`: the namespace from context above',
      '   - \\`targetPlatform\\`: the local system architecture',
      '2. Capture the **local registry address** (e.g., \\`localhost:6xxx\\`) from the result for use when tagging, pushing, and generating manifests.',
      '3. Retry up to **2 times** on failure.',
    ].join('\n'),
    tagInstruction: `Call **${TOOL_NAME.TAG_IMAGE}** to tag the image using the **local registry address returned by \\\`${TOOL_NAME.PREPARE_CLUSTER}\\\`** (e.g., \\\`<registry-address>/<image>:<tag>\\\`).`,
    pushInstructions: [
      `1. Call **${TOOL_NAME.PUSH_IMAGE}** to push the tagged image to the **same local registry address returned by \\\`${TOOL_NAME.PREPARE_CLUSTER}\\\`**.`,
      '2. Retry up to **2 times** on failure.',
    ].join('\n'),
    manifestRegistryClause: `Set the image reference in the manifests to use the local registry prefix returned by \\\`${TOOL_NAME.PREPARE_CLUSTER}\\\` (e.g., \\\`<registry-address>/<image>:<tag>\\\`).`,
    deployTarget: 'the Kind cluster',
    platformRule: 'Use the **local system architecture** for all platform-related parameters.',
    extraRules: [],
  });
}
