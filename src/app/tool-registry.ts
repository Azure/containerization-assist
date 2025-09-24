/**
 * Tool Registry
 * Simple registry for managing tools
 */

import type { Tool } from '@/types';

export interface ToolRegistry {
  get(name: string): Tool | undefined;
  list(): Tool[];
  has(name: string): boolean;
}

/**
 * Create a tool registry
 */
export function createToolRegistry(tools: Tool[]): ToolRegistry {
  const registry = new Map<string, Tool>();

  for (const tool of tools) {
    registry.set(tool.name, tool);
  }

  return {
    get: (name: string) => registry.get(name),
    list: () => Array.from(registry.values()),
    has: (name: string) => registry.has(name),
  };
}
