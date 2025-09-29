/**
 * Tool Discovery Service for AI Enhancement Capabilities
 *
 * Provides introspection and discovery of tool capabilities,
 * AI enhancement status, and metadata across all MCP tools.
 */

import { Result, Success, Failure } from '@/types';
import { getAllInternalTools } from '@/tools';

export interface ToolCapabilities {
  name: string;
  description: string;
  category: string | undefined;
  version: string | undefined;
  aiDriven: boolean;
  knowledgeEnhanced: boolean;
  samplingStrategy: string;
  enhancementCapabilities: string[];
  validationSupport: boolean;
}

export interface ToolDiscoveryOptions {
  /** Include only AI-driven tools */
  aiDrivenOnly?: boolean;
  /** Include only knowledge-enhanced tools */
  knowledgeEnhancedOnly?: boolean;
  /** Filter by sampling strategy */
  samplingStrategy?: 'rerank' | 'single' | 'none';
  /** Filter by enhancement capabilities */
  hasCapability?: string;
  /** Include detailed metadata in results */
  includeDetails?: boolean;
}

/**
 * Discovers and analyzes all available tools and their capabilities
 */
export async function discoverToolCapabilities(
  options: ToolDiscoveryOptions = {},
): Promise<Result<ToolCapabilities[]>> {
  try {
    const allTools = getAllInternalTools();
    const capabilities: ToolCapabilities[] = [];

    for (const tool of allTools) {
      // Skip metadata validation for now since validateToolMetadata expects different interface
      const capability: ToolCapabilities = {
        name: tool.name,
        description: tool.description,
        category: tool.category || undefined,
        version: tool.version || undefined,
        aiDriven: tool.metadata.aiDriven,
        knowledgeEnhanced: tool.metadata.knowledgeEnhanced,
        samplingStrategy: tool.metadata.samplingStrategy,
        enhancementCapabilities: tool.metadata.enhancementCapabilities,
        validationSupport: hasValidationSupport(tool.metadata.enhancementCapabilities),
      };

      // Apply filters
      if (options.aiDrivenOnly && !capability.aiDriven) continue;
      if (options.knowledgeEnhancedOnly && !capability.knowledgeEnhanced) continue;
      if (options.samplingStrategy && capability.samplingStrategy !== options.samplingStrategy)
        continue;
      if (
        options.hasCapability &&
        !capability.enhancementCapabilities.includes(options.hasCapability)
      )
        continue;

      capabilities.push(capability);
    }

    return Success(capabilities.sort((a, b) => a.name.localeCompare(b.name)));
  } catch (error) {
    return Failure(`Failed to discover tool capabilities: ${error}`);
  }
}

/**
 * Get list of AI-driven tools
 */
export async function getAIDrivenTools(): Promise<Result<string[]>> {
  const result = await discoverToolCapabilities({ aiDrivenOnly: true });
  if (!result.ok) return result;

  return Success(result.value.map((tool) => tool.name));
}

/**
 * Get list of knowledge-enhanced tools
 */
export async function getKnowledgeEnhancedTools(): Promise<Result<string[]>> {
  const result = await discoverToolCapabilities({ knowledgeEnhancedOnly: true });
  if (!result.ok) return result;

  return Success(result.value.map((tool) => tool.name));
}

/**
 * Get tools by sampling strategy
 */
export async function getToolsBySamplingStrategy(
  strategy: 'rerank' | 'single' | 'none',
): Promise<Result<string[]>> {
  const result = await discoverToolCapabilities({ samplingStrategy: strategy });
  if (!result.ok) return result;

  return Success(result.value.map((tool) => tool.name));
}

/**
 * Get tools with specific enhancement capability
 */
export async function getToolsWithCapability(capability: string): Promise<Result<string[]>> {
  const result = await discoverToolCapabilities({ hasCapability: capability });
  if (!result.ok) return result;

  return Success(result.value.map((tool) => tool.name));
}

/**
 * Get comprehensive tool statistics
 */
export async function getToolStatistics(): Promise<
  Result<{
    total: number;
    aiDriven: number;
    knowledgeEnhanced: number;
    samplingStrategies: Record<string, number>;
    enhancementCapabilities: Record<string, number>;
    categories: Record<string, number>;
  }>
> {
  const result = await discoverToolCapabilities();
  if (!result.ok) return result;

  const tools = result.value;
  const stats = {
    total: tools.length,
    aiDriven: tools.filter((t) => t.aiDriven).length,
    knowledgeEnhanced: tools.filter((t) => t.knowledgeEnhanced).length,
    samplingStrategies: {} as Record<string, number>,
    enhancementCapabilities: {} as Record<string, number>,
    categories: {} as Record<string, number>,
  };

  // Count sampling strategies
  for (const tool of tools) {
    stats.samplingStrategies[tool.samplingStrategy] =
      (stats.samplingStrategies[tool.samplingStrategy] || 0) + 1;
  }

  // Count enhancement capabilities
  for (const tool of tools) {
    for (const capability of tool.enhancementCapabilities) {
      stats.enhancementCapabilities[capability] =
        (stats.enhancementCapabilities[capability] || 0) + 1;
    }
  }

  // Count categories
  for (const tool of tools) {
    const category = tool.category || 'uncategorized';
    stats.categories[category] = (stats.categories[category] || 0) + 1;
  }

  return Success(stats);
}

/**
 * Validate tool metadata consistency
 *
 * This function now validates all tools have proper metadata and consistency
 */
export async function validateToolMetadata(): Promise<
  Result<{
    valid: string[];
    invalid: Array<{ name: string; issues: string[] }>;
  }>
> {
  const result = await discoverToolCapabilities();
  if (!result.ok) return result;

  const valid: string[] = [];
  const invalid: Array<{ name: string; issues: string[] }> = [];

  for (const tool of result.value) {
    const issues: string[] = [];

    // Check AI-driven tools have appropriate sampling strategy
    if (tool.aiDriven && tool.samplingStrategy === 'none') {
      issues.push('AI-driven tool should have sampling strategy other than "none"');
    }

    // Check knowledge-enhanced tools have appropriate capabilities
    if (tool.knowledgeEnhanced && tool.enhancementCapabilities.length === 0) {
      issues.push('Knowledge-enhanced tool should have enhancement capabilities');
    }

    // Check consistency between AI-driven and knowledge-enhanced
    if (tool.knowledgeEnhanced && !tool.aiDriven) {
      issues.push('Knowledge-enhanced tools should typically be AI-driven');
    }

    if (issues.length > 0) {
      invalid.push({ name: tool.name, issues });
    } else {
      valid.push(tool.name);
    }
  }

  return Success({ valid, invalid });
}

/**
 * Helper function to determine if enhancement capabilities include validation
 */
function hasValidationSupport(capabilities: string[]): boolean {
  const validationKeywords = ['validation', 'repair', 'fix', 'security', 'optimization'];
  return capabilities.some((cap) =>
    validationKeywords.some((keyword) => cap.toLowerCase().includes(keyword)),
  );
}

/**
 * Generate tool capabilities report in markdown format
 */
export async function generateToolCapabilitiesReport(): Promise<Result<string>> {
  const [capabilitiesResult, statsResult, validationResult] = await Promise.all([
    discoverToolCapabilities({ includeDetails: true }),
    getToolStatistics(),
    validateToolMetadata(),
  ]);

  if (!capabilitiesResult.ok) return capabilitiesResult;
  if (!statsResult.ok) return statsResult;
  if (!validationResult.ok) return validationResult;

  const capabilities = capabilitiesResult.value;
  const stats = statsResult.value;
  const validation = validationResult.value;

  let report = `# Tool Capabilities Report

Generated: ${new Date().toISOString()}

## Summary Statistics

- **Total Tools**: ${stats.total}
- **AI-Driven Tools**: ${stats.aiDriven} (${Math.round((stats.aiDriven / stats.total) * 100)}%)
- **Knowledge-Enhanced Tools**: ${stats.knowledgeEnhanced} (${Math.round((stats.knowledgeEnhanced / stats.total) * 100)}%)

### Sampling Strategies
`;

  for (const [strategy, count] of Object.entries(stats.samplingStrategies)) {
    report += `- **${strategy}**: ${count} tools\n`;
  }

  report += `\n### Enhancement Capabilities\n`;
  for (const [capability, count] of Object.entries(stats.enhancementCapabilities)) {
    report += `- **${capability}**: ${count} tools\n`;
  }

  report += `\n### Categories\n`;
  for (const [category, count] of Object.entries(stats.categories)) {
    report += `- **${category}**: ${count} tools\n`;
  }

  report += `\n## Tool Details\n\n`;
  for (const tool of capabilities) {
    report += `### ${tool.name}
- **Description**: ${tool.description}
- **Category**: ${tool.category || 'uncategorized'}
- **AI-Driven**: ${tool.aiDriven ? '✅' : '❌'}
- **Knowledge-Enhanced**: ${tool.knowledgeEnhanced ? '✅' : '❌'}
- **Sampling Strategy**: ${tool.samplingStrategy}
- **Enhancement Capabilities**: ${tool.enhancementCapabilities.join(', ') || 'None'}
- **Validation Support**: ${tool.validationSupport ? '✅' : '❌'}

`;
  }

  if (validation.invalid.length > 0) {
    report += `\n## Metadata Issues\n\n`;
    for (const issue of validation.invalid) {
      report += `### ${issue.name}\n`;
      for (const problem of issue.issues) {
        report += `- ⚠️ ${problem}\n`;
      }
      report += '\n';
    }
  }

  return Success(report);
}
