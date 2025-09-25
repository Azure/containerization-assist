/**
 * Tool Registry
 * Simple registry for managing tools
 */

import type { Tool } from '@/types/tool';

type AnyTool = Tool<any, any>;

export interface ToolRegistry {
  get(name: string): AnyTool | undefined;
  list(): AnyTool[];
  has(name: string): boolean;
}

/**
 * Create a tool registry
 */
export function createToolRegistry(tools: AnyTool[]): ToolRegistry {
  const registry = new Map<string, AnyTool>();

  for (const tool of tools) {
    registry.set(tool.name, tool);
  }

  return {
    get: (name: string) => registry.get(name),
    list: () => Array.from(registry.values()),
    has: (name: string) => registry.has(name),
  };
}
