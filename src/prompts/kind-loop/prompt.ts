/**
 * kind-loop prompt
 *
 * Returns a seeded user message that drives a full local Kind cluster
 * development iteration loop using the containerization-assist MCP tools.
 */

import { TOOL_NAME } from '@/tools';
import {
  analyzeStep,
  assemblePrompt,
  deployStep,
  generateDockerfileStep,
  scanStep,
  sharedRules,
  verifyStep,
} from '../shared/steps';
import type { LocalKindDevLoopArgs } from './schema';

export function buildLocalKindDevLoopPrompt(args: LocalKindDevLoopArgs): string {
  const nsClause = args.namespace
    ? `Use the namespace **${args.namespace}**.`
    : 'Generate a unique namespace name (e.g., `dev-<short-hash>`) for isolation.';
  const imageClause = args.imageName
    ? `Use the image name **${args.imageName}**.`
    : 'Derive the image name from the repository directory name.';

  return assemblePrompt(
    'local Kind cluster development iteration loop',
    [
      '- Repository: use the **current working directory** (confirm with the user before proceeding).',
      `- ${nsClause}`,
      `- ${imageClause}`,
      '- Target environment: **development** (local Kind cluster with local image registry)',
      '- Use the **local system architecture** for Kind testing (detect it automatically).',
    ],
    [
      analyzeStep(),
      generateDockerfileStep(),
      {
        heading: 'Build the image',
        body: [
          `1. Call **${TOOL_NAME.BUILD_IMAGE_CONTEXT}** with the repository path, image name, and the **local system platform** (e.g., \`linux/arm64\` or \`linux/amd64\` — detect automatically).`,
          '2. Execute the returned build command to produce the Docker image.',
          `3. Retry up to **2 times** on failure; call **${TOOL_NAME.FIX_DOCKERFILE}** if the build fails due to Dockerfile issues.`,
        ].join('\n'),
      },
      scanStep(),
      {
        heading: 'Prepare the Kind cluster',
        body: [
          `1. Call **${TOOL_NAME.PREPARE_CLUSTER}** with:`,
          '   - `clusterType: "kind"` (this creates a local Kind cluster with a local registry)',
          '   - `namespace`: the namespace from context above',
          '   - `targetPlatform`: the local system architecture',
          '2. Capture the **local registry address** (e.g., `localhost:6xxx`) from the result for use when tagging, pushing, and generating manifests.',
          '3. Retry up to **2 times** on failure.',
        ].join('\n'),
      },
      {
        heading: 'Tag the image for the local registry',
        body: [
          `1. Call **${TOOL_NAME.TAG_IMAGE}** to tag the image using the **local registry address returned by \`${TOOL_NAME.PREPARE_CLUSTER}\`** (e.g., \`<registry-address>/<image>:<tag>\`).`,
          '2. Retry up to **2 times** on failure.',
        ].join('\n'),
      },
      {
        heading: 'Push the image to the local registry',
        body: [
          `1. Call **${TOOL_NAME.PUSH_IMAGE}** to push the tagged image to the **same local registry address returned by \`${TOOL_NAME.PREPARE_CLUSTER}\`**.`,
          '2. Retry up to **2 times** on failure.',
        ].join('\n'),
      },
      {
        heading: 'Generate Kubernetes manifests (if missing)',
        body: [
          'If no K8s manifests exist for the target module(s):',
          `1. Call **${TOOL_NAME.GENERATE_K8S_MANIFESTS}** with the repository path, namespace, and analysis context. Set the image reference in the manifests to use the local registry prefix returned by \`${TOOL_NAME.PREPARE_CLUSTER}\` (e.g., \`<registry-address>/<image>:<tag>\`).`,
          "2. Follow the tool's guidance to create manifest files on disk.",
          '3. Retry up to **2 times** if generation fails.',
        ].join('\n'),
      },
      deployStep('the Kind cluster'),
      verifyStep(),
    ],
    [
      ...sharedRules,
      '- Use the **local system architecture** for all platform-related parameters.',
    ],
  );
}
