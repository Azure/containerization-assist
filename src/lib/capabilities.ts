/**
 * Type-Safe Capability Management
 *
 * Provides a type-safe, immutable capability management system for tools.
 * Replaces array-based capability handling with a proper class-based approach.
 */

import type { EnhancementCapability } from '@/types/tool-metadata';

/**
 * Immutable capability set for tool enhancement capabilities
 *
 * Provides type-safe operations for managing and querying tool capabilities
 * without exposing internal state for mutation.
 */
export class CapabilitySet {
  private readonly capabilities: ReadonlySet<EnhancementCapability>;

  /**
   * Create a new capability set from an array of capabilities
   *
   * @param capabilities - Array of enhancement capabilities
   */
  constructor(capabilities: readonly EnhancementCapability[]) {
    // Validate input
    if (!Array.isArray(capabilities)) {
      throw new Error('Capabilities must be an array');
    }

    // Remove duplicates and create immutable set
    this.capabilities = new Set(capabilities);
  }

  /**
   * Check if the set contains a specific capability
   *
   * @param capability - The capability to check for
   * @returns True if the capability is present
   */
  has(capability: EnhancementCapability): boolean {
    return this.capabilities.has(capability);
  }

  /**
   * Check if the set contains any of the specified capabilities
   *
   * @param capabilities - Array of capabilities to check for
   * @returns True if any of the capabilities are present
   */
  hasAny(capabilities: readonly EnhancementCapability[]): boolean {
    return capabilities.some((capability) => this.capabilities.has(capability));
  }

  /**
   * Check if the set contains all of the specified capabilities
   *
   * @param capabilities - Array of capabilities to check for
   * @returns True if all capabilities are present
   */
  hasAll(capabilities: readonly EnhancementCapability[]): boolean {
    return capabilities.every((capability) => this.capabilities.has(capability));
  }

  /**
   * Check if this set intersects with another capability set
   *
   * @param other - Another capability set to compare with
   * @returns True if the sets have any capabilities in common
   */
  intersects(other: CapabilitySet): boolean {
    for (const capability of this.capabilities) {
      if (other.capabilities.has(capability)) {
        return true;
      }
    }
    return false;
  }

  /**
   * Get the intersection of this set with another capability set
   *
   * @param other - Another capability set to intersect with
   * @returns New capability set containing common capabilities
   */
  intersection(other: CapabilitySet): CapabilitySet {
    const commonCapabilities: EnhancementCapability[] = [];
    for (const capability of this.capabilities) {
      if (other.capabilities.has(capability)) {
        commonCapabilities.push(capability);
      }
    }
    return new CapabilitySet(commonCapabilities);
  }

  /**
   * Create a union of this set with another capability set
   *
   * @param other - Another capability set to union with
   * @returns New capability set containing all capabilities from both sets
   */
  union(other: CapabilitySet): CapabilitySet {
    const allCapabilities = [...this.capabilities, ...other.capabilities];
    return new CapabilitySet(allCapabilities);
  }

  /**
   * Create a new set with additional capabilities
   *
   * @param capabilities - Additional capabilities to include
   * @returns New capability set with added capabilities
   */
  add(capabilities: readonly EnhancementCapability[]): CapabilitySet {
    const newCapabilities = [...this.capabilities, ...capabilities];
    return new CapabilitySet(newCapabilities);
  }

  /**
   * Create a new set without specified capabilities
   *
   * @param capabilities - Capabilities to remove
   * @returns New capability set without the specified capabilities
   */
  remove(capabilities: readonly EnhancementCapability[]): CapabilitySet {
    const removeSet = new Set(capabilities);
    const filteredCapabilities = Array.from(this.capabilities).filter(
      (capability) => !removeSet.has(capability),
    );
    return new CapabilitySet(filteredCapabilities);
  }

  /**
   * Get the capabilities as a readonly array
   *
   * @returns Readonly array of capabilities
   */
  toArray(): readonly EnhancementCapability[] {
    return Array.from(this.capabilities);
  }

  /**
   * Get the number of capabilities in the set
   *
   * @returns Number of capabilities
   */
  get size(): number {
    return this.capabilities.size;
  }

  /**
   * Check if the set is empty
   *
   * @returns True if the set contains no capabilities
   */
  get isEmpty(): boolean {
    return this.capabilities.size === 0;
  }

  /**
   * Get string representation of the capability set
   *
   * @returns String representation for debugging
   */
  toString(): string {
    const capabilities = Array.from(this.capabilities).sort();
    return `CapabilitySet[${capabilities.join(', ')}]`;
  }

  /**
   * Compare this set with another capability set for equality
   *
   * @param other - Another capability set to compare with
   * @returns True if both sets contain exactly the same capabilities
   */
  equals(other: CapabilitySet): boolean {
    if (this.capabilities.size !== other.capabilities.size) {
      return false;
    }

    for (const capability of this.capabilities) {
      if (!other.capabilities.has(capability)) {
        return false;
      }
    }

    return true;
  }

  /**
   * Create an iterator for the capabilities
   *
   * @returns Iterator over the capabilities
   */
  [Symbol.iterator](): Iterator<EnhancementCapability> {
    return this.capabilities.values();
  }

  /**
   * Convert capability set to JSON-serializable format
   *
   * @returns Plain array of capabilities for JSON serialization
   */
  toJSON(): EnhancementCapability[] {
    return Array.from(this.capabilities).sort();
  }
}

/**
 * Predefined capability sets for common tool types
 */
export const PredefinedCapabilities = {
  /**
   * Capabilities for basic validation tools
   */
  VALIDATION: new CapabilitySet(['validation', 'analysis'] as const),

  /**
   * Capabilities for security-focused tools
   */
  SECURITY: new CapabilitySet(['validation', 'security', 'analysis'] as const),

  /**
   * Capabilities for generation tools
   */
  GENERATION: new CapabilitySet(['generation', 'optimization', 'analysis'] as const),

  /**
   * Capabilities for repair and enhancement tools
   */
  ENHANCEMENT: new CapabilitySet(['repair', 'enhancement', 'optimization'] as const),

  /**
   * Full-featured AI tool capabilities
   */
  FULL_AI: new CapabilitySet([
    'validation',
    'repair',
    'security',
    'optimization',
    'analysis',
    'generation',
    'enhancement',
  ] as const),

  /**
   * Empty capability set for non-AI tools
   */
  NONE: new CapabilitySet([]),
} as const;

/**
 * Create a capability set from an array of capabilities
 *
 * Convenience function for creating capability sets with validation.
 *
 * @param capabilities - Array of enhancement capabilities
 * @returns New capability set
 */
export function createCapabilitySet(capabilities: readonly EnhancementCapability[]): CapabilitySet {
  return new CapabilitySet(capabilities);
}

/**
 * Create a capability set from multiple sources
 *
 * Merges capabilities from multiple arrays or sets into a single set.
 *
 * @param sources - Arrays or sets of capabilities to merge
 * @returns New capability set containing all unique capabilities
 */
export function mergeCapabilities(
  ...sources: readonly (readonly EnhancementCapability[] | CapabilitySet)[]
): CapabilitySet {
  const allCapabilities: EnhancementCapability[] = [];

  for (const source of sources) {
    if (source instanceof CapabilitySet) {
      allCapabilities.push(...source.toArray());
    } else {
      allCapabilities.push(...source);
    }
  }

  return new CapabilitySet(allCapabilities);
}

/**
 * Check if a tool with given capabilities can handle a specific task
 *
 * Utility function to determine if a tool's capabilities match task requirements.
 *
 * @param toolCapabilities - The tool's capabilities
 * @param requiredCapabilities - Required capabilities for the task
 * @param requireAll - Whether all capabilities are required (default: false, any will do)
 * @returns True if the tool can handle the task
 */
export function canHandleTask(
  toolCapabilities: CapabilitySet | readonly EnhancementCapability[],
  requiredCapabilities: readonly EnhancementCapability[],
  requireAll: boolean = false,
): boolean {
  const capabilities =
    toolCapabilities instanceof CapabilitySet
      ? toolCapabilities
      : new CapabilitySet(toolCapabilities);

  return requireAll
    ? capabilities.hasAll(requiredCapabilities)
    : capabilities.hasAny(requiredCapabilities);
}

/**
 * Find tools that can handle specific capability requirements
 *
 * Utility function to filter tools based on capability requirements.
 *
 * @param tools - Array of tools with their capabilities
 * @param requiredCapabilities - Required capabilities
 * @param requireAll - Whether all capabilities are required
 * @returns Array of tools that meet the capability requirements
 */
export function findCapableTools<
  T extends { capabilities: CapabilitySet | readonly EnhancementCapability[] },
>(
  tools: readonly T[],
  requiredCapabilities: readonly EnhancementCapability[],
  requireAll: boolean = false,
): T[] {
  return tools.filter((tool) => canHandleTask(tool.capabilities, requiredCapabilities, requireAll));
}

/**
 * Get capability recommendations based on tool type and use case
 *
 * Provides suggested capability sets for different types of tools.
 *
 * @param toolType - The type of tool ('validator', 'generator', 'enhancer', 'analyzer', 'security')
 * @returns Recommended capability set
 */
export function getRecommendedCapabilities(
  toolType: 'validator' | 'generator' | 'enhancer' | 'analyzer' | 'security',
): CapabilitySet {
  switch (toolType) {
    case 'validator':
      return PredefinedCapabilities.VALIDATION;

    case 'generator':
      return PredefinedCapabilities.GENERATION;

    case 'enhancer':
      return PredefinedCapabilities.ENHANCEMENT;

    case 'analyzer':
      return new CapabilitySet(['analysis', 'validation']);

    case 'security':
      return PredefinedCapabilities.SECURITY;

    default: {
      // TypeScript ensures this is never reached
      const _exhaustive: never = toolType;
      throw new Error(`Unknown tool type: ${_exhaustive}`);
    }
  }
}
