/**
 * Tool Registry
 * Simple registry for managing tools
 */

import type { Tool } from '@/types/tool';
import type { ZodTypeAny } from 'zod';

// Use the base Tool interface with generics for flexibility
// This allows any tool that conforms to the Tool interface
export interface ToolRegistry<T extends Tool<ZodTypeAny, any> = Tool<ZodTypeAny, any>> {
  get(name: string): T | undefined;
  list(): T[];
  has(name: string): boolean;
}

/**
 * Create a tool registry
 */
export function createToolRegistry<T extends Tool<ZodTypeAny, any>>(
  tools: readonly T[],
): ToolRegistry<T> {
  const registry = new Map<string, T>();

  for (const tool of tools) {
    registry.set(tool.name, tool);
  }

  return {
    get: (name: string) => registry.get(name),
    list: () => Array.from(registry.values()),
    has: (name: string) => registry.has(name),
  };
}
