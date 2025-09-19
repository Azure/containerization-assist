/**
 * Tool Registry - Central registration of all MCP tools
 *
 * NO SIDE EFFECTS: Does not execute tool code during import.
 * Tools must explicitly declare their type.
 */

import type { RouterTool } from '@/mcp/tool-router';
import type { ToolName } from '@/exports/tools';
import { type NormalizedTool, normalizeToolExport, type ToolExport } from './types';

// Tool imports
import { analyzeRepo } from '@tools/analyze-repo/tool';
import { buildImage } from '@tools/build-image/tool';
import { convertAcaToK8s } from '@tools/convert-aca-to-k8s/tool';
import { deployApplication } from '@tools/deploy/tool';
import { tool as fixDockerfile } from '@tools/fix-dockerfile/tool';
import { tool as generateAcaManifests } from '@tools/generate-aca-manifests/tool';
import { tool as generateDockerfile } from '@tools/generate-dockerfile/tool';
import { tool as generateHelmCharts } from '@tools/generate-helm-charts/tool';
import { tool as generateK8sManifests } from '@tools/generate-k8s-manifests/tool';
import { tool as inspectSession } from '@tools/inspect-session/tool';
import { tool as ops } from '@tools/ops/tool';
import { prepareCluster } from '@tools/prepare-cluster/tool';
import { tool as pushImage } from '@tools/push-image/tool';
import { tool as resolveBaseImages } from '@tools/resolve-base-images/tool';
import { tool as scan } from '@tools/scan/tool';
import { tool as tagImage } from '@tools/tag-image/tool';
import { verifyDeploy } from '@tools/verify-deploy/tool';

/**
 * Safe type detection without executing user code
 */
function inferToolType(tool: any): ToolExport {
  // Explicit type declaration takes precedence
  if (tool.type) {
    return tool as ToolExport;
  }

  // Check for nested pattern (safe property check)
  if (
    tool.execute &&
    typeof tool.execute === 'object' &&
    typeof tool.execute.execute === 'function'
  ) {
    return { ...tool, type: 'nested' } as ToolExport;
  }

  // Default to standard
  if (process.env.NODE_ENV === 'development') {
    console.warn(
      `[Tool Registry] Missing explicit type on tool "${tool.name}". ` +
        `Please add type: 'standard' | 'nested' | 'factory' to the export.`,
    );
  }

  return { ...tool, type: 'standard' } as ToolExport;
}

/**
 * Convert normalized tool to RouterTool interface
 */
function toRouterTool(normalized: NormalizedTool): RouterTool {
  return {
    name: normalized.name,
    schema: normalized.schema,
    handler: normalized.handler,
  };
}

/**
 * Tool list - single source of truth
 */
const toolList: Array<[ToolName, any]> = [
  ['analyze-repo', analyzeRepo],
  ['build-image', buildImage],
  ['convert-aca-to-k8s', convertAcaToK8s],
  ['deploy', deployApplication],
  ['fix-dockerfile', fixDockerfile],
  ['generate-aca-manifests', generateAcaManifests],
  ['generate-dockerfile', generateDockerfile],
  ['generate-helm-charts', generateHelmCharts],
  ['generate-k8s-manifests', generateK8sManifests],
  ['inspect-session', inspectSession],
  ['ops', ops],
  ['prepare-cluster', prepareCluster],
  ['push-image', pushImage],
  ['resolve-base-images', resolveBaseImages],
  ['scan', scan],
  ['tag-image', tagImage],
  ['verify-deploy', verifyDeploy],
];

/**
 * Central tool registry
 */
export const registry = new Map<ToolName, RouterTool>(
  toolList.map(([name, module]) => {
    const typed = inferToolType(module);
    const normalized = normalizeToolExport(typed);
    return [name, toRouterTool(normalized)];
  }),
);

// Public API
export function getToolNames(): string[] {
  return Array.from(registry.keys());
}

export function getTool(name: string): RouterTool | undefined {
  return registry.get(name as ToolName);
}

export function hasTool(name: string): boolean {
  return registry.has(name as ToolName);
}

export function getToolRegistry(): Map<ToolName, RouterTool> {
  return registry;
}
