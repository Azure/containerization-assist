/**
 * Inspect Tools CLI Command
 *
 * Provides command-line interface for tool discovery, capability analysis,
 * and metadata inspection for AI enhancement features.
 */

import { Command } from 'commander';
import { ALL_TOOLS, type Tool } from '@/tools';
import { validateAllToolMetadata, type ValidatableTool } from '@/types/tool-metadata';
import { writeFileSync } from 'node:fs';
import path from 'node:path';
import { handleResultError, handleGenericError, formatError } from '../error-formatting';
import { renderTools } from '../render';
import { Result, Success, Failure } from '@/types';

// Tool discovery options interface
export interface ToolDiscoveryOptions {
  /** Include only tools with sampling enabled */
  samplingEnabledOnly?: boolean;
  /** Include only knowledge-enhanced tools */
  knowledgeEnhancedOnly?: boolean;
  /** Filter by sampling strategy */
  samplingStrategy?: 'single' | 'none';
  /** Filter by enhancement capabilities */
  hasCapability?: string;
  /** Include detailed metadata in results */
  includeDetails?: boolean;
}

/**
 * Discovers and analyzes all available tools and their capabilities
 * Inlined from tool-discovery service to eliminate unnecessary abstraction layer
 */
async function discoverToolCapabilities(
  options: ToolDiscoveryOptions = {},
): Promise<Result<Tool[]>> {
  try {
    const filteredTools: Tool[] = [];

    for (const tool of ALL_TOOLS) {
      // Apply filters
      if (options.samplingEnabledOnly && tool.metadata.samplingStrategy !== 'single') continue;
      if (options.knowledgeEnhancedOnly && !tool.metadata.knowledgeEnhanced) continue;
      if (options.samplingStrategy && tool.metadata.samplingStrategy !== options.samplingStrategy)
        continue;
      if (
        options.hasCapability &&
        !tool.metadata.enhancementCapabilities.includes(options.hasCapability as any)
      )
        continue;

      filteredTools.push(tool);
    }

    return Success(filteredTools.sort((a, b) => a.name.localeCompare(b.name)));
  } catch (error) {
    return Failure(`Failed to discover tool capabilities: ${error}`);
  }
}

/**
 * Get comprehensive tool statistics
 */
async function getToolStatistics(): Promise<
  Result<{
    total: number;
    samplingEnabled: number;
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
    samplingEnabled: tools.filter((t) => t.metadata.samplingStrategy === 'single').length,
    knowledgeEnhanced: tools.filter((t) => t.metadata.knowledgeEnhanced).length,
    samplingStrategies: {} as Record<string, number>,
    enhancementCapabilities: {} as Record<string, number>,
    categories: {} as Record<string, number>,
  };

  // Count sampling strategies
  for (const tool of tools) {
    stats.samplingStrategies[tool.metadata.samplingStrategy] =
      (stats.samplingStrategies[tool.metadata.samplingStrategy] || 0) + 1;
  }

  // Count enhancement capabilities
  for (const tool of tools) {
    for (const capability of tool.metadata.enhancementCapabilities) {
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
 * Validate tool metadata using centralized validation
 */
async function validateToolMetadata(): Promise<
  Result<{
    valid: string[];
    invalid: Array<{ name: string; issues: string[] }>;
  }>
> {
  const toolsResult = await discoverToolCapabilities();
  if (!toolsResult.ok) return toolsResult;

  // Convert to ValidatableTool format for centralized validation
  const validatableTools: ValidatableTool[] = toolsResult.value.map((tool) => ({
    name: tool.name,
    metadata: tool.metadata,
  }));

  const validationResult = await validateAllToolMetadata(validatableTools);
  if (!validationResult.ok) return validationResult;

  // Convert back to the expected format
  return Success({
    valid: validationResult.value.validTools,
    invalid: validationResult.value.invalidTools.map((invalid) => ({
      name: invalid.name,
      issues: invalid.issues,
    })),
  });
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
 * Generate comprehensive tool capabilities report
 */
async function generateToolCapabilitiesReport(): Promise<Result<string>> {
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
- **Sampling-Enabled Tools**: ${stats.samplingEnabled} (${Math.round((stats.samplingEnabled / stats.total) * 100)}%)
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
    const validationSupport = hasValidationSupport(tool.metadata.enhancementCapabilities);
    report += `### ${tool.name}
- **Description**: ${tool.description}
- **Category**: ${tool.category || 'uncategorized'}
- **AI-Driven**: ${tool.metadata.samplingStrategy === 'single' ? '✅' : '❌'}
- **Knowledge-Enhanced**: ${tool.metadata.knowledgeEnhanced ? '✅' : '❌'}
- **Sampling Strategy**: ${tool.metadata.samplingStrategy}
- **Enhancement Capabilities**: ${tool.metadata.enhancementCapabilities.join(', ') || 'None'}
- **Validation Support**: ${validationSupport ? '✅' : '❌'}

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

/**
 * Create inspect-tools CLI command
 */
export function createInspectToolsCommand(): Command {
  const cmd = new Command('inspect-tools');
  cmd.description('Inspect and analyze tool capabilities and AI enhancement status');

  // List all tools with capabilities
  cmd
    .command('list')
    .description('List all tools with their AI enhancement capabilities')
    .option('--sampling-enabled', 'Show only AI-driven tools')
    .option('--knowledge-enhanced', 'Show only knowledge-enhanced tools')
    .option('--sampling <strategy>', 'Filter by sampling strategy (single, none)')
    .option('--capability <name>', 'Filter by enhancement capability')
    .option('--format <format>', 'Output format (table, json, csv)', 'table')
    .option('--detailed', 'Include detailed metadata')
    .action(async (options) => {
      try {
        const discoveryOptions: ToolDiscoveryOptions = {
          samplingEnabledOnly: options.samplingEnabled,
          knowledgeEnhancedOnly: options.knowledgeEnhanced,
          samplingStrategy: options.sampling,
          hasCapability: options.capability,
          includeDetails: options.detailed,
        };

        const result = await discoverToolCapabilities(discoveryOptions);
        if (!result.ok) {
          handleResultError(result, 'Failed to discover tools');
        }

        const tools = result.value;

        renderTools(tools, {
          format: options.format as 'table' | 'csv' | 'json',
          detailed: options.detailed,
        });
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  // Statistics command
  cmd
    .command('stats')
    .description('Show tool statistics and AI enhancement overview')
    .option('--json', 'Output as JSON')
    .action(async (options) => {
      try {
        const result = await getToolStatistics();
        if (!result.ok) {
          handleResultError(result, 'Failed to get statistics');
        }

        const stats = result.value;

        if (options.json) {
          console.info(JSON.stringify(stats, null, 2));
        } else {
          console.info('📊 Tool Statistics\n');
          console.info(`Total Tools: ${stats.total}`);
          console.info(
            `Sampling-Enabled: ${stats.samplingEnabled} (${Math.round((stats.samplingEnabled / stats.total) * 100)}%)`,
          );
          console.info(
            `Knowledge-Enhanced: ${stats.knowledgeEnhanced} (${Math.round((stats.knowledgeEnhanced / stats.total) * 100)}%)`,
          );

          console.info('\n🎯 Sampling Strategies:');
          for (const [strategy, count] of Object.entries(stats.samplingStrategies)) {
            console.info(`  ${strategy}: ${count} tools`);
          }

          console.info('\n⚡ Enhancement Capabilities:');
          for (const [capability, count] of Object.entries(stats.enhancementCapabilities)) {
            console.info(`  ${capability}: ${count} tools`);
          }

          console.info('\n📁 Categories:');
          for (const [category, count] of Object.entries(stats.categories)) {
            console.info(`  ${category}: ${count} tools`);
          }
        }
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  // AI-driven tools command
  cmd
    .command('ai-driven')
    .description('List tools that use AI for content generation')
    .option('--count', 'Show only count')
    .action(async (options) => {
      try {
        const result = await discoverToolCapabilities({ samplingEnabledOnly: true });
        if (!result.ok) {
          handleResultError(result, 'Failed to get AI-driven tools');
        }

        const tools = result.value.map((tool) => tool.name);

        if (options.count) {
          console.info(tools.length);
        } else {
          console.info('🤖 Sampling-Enabled Tools:\n');
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  // Knowledge-enhanced tools command
  cmd
    .command('knowledge-enhanced')
    .description('List tools that use knowledge enhancement')
    .option('--count', 'Show only count')
    .action(async (options) => {
      try {
        const result = await discoverToolCapabilities({ knowledgeEnhancedOnly: true });
        if (!result.ok) {
          handleResultError(result, 'Failed to get knowledge-enhanced tools');
        }

        const tools = result.value.map((tool) => tool.name);

        if (options.count) {
          console.info(tools.length);
        } else {
          console.info('📚 Knowledge-Enhanced Tools:\n');
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  // Validate command
  cmd
    .command('validate')
    .description('Validate tool metadata consistency and compliance')
    .option('--json', 'Output as JSON')
    .action(async (options) => {
      try {
        const result = await validateToolMetadata();
        if (!result.ok) {
          handleResultError(result, 'Failed to validate metadata');
        }

        const validation = result.value;

        if (options.json) {
          console.info(JSON.stringify(validation, null, 2));
        } else {
          console.info('🔍 Metadata Validation Results\n');

          if (validation.valid.length > 0) {
            console.info(`✅ Valid Tools (${validation.valid.length}):`);
            validation.valid.forEach((tool) => console.info(`  • ${tool}`));
            console.info('');
          }

          if (validation.invalid.length > 0) {
            console.info(`❌ Tools with Issues (${validation.invalid.length}):`);
            validation.invalid.forEach((item) => {
              console.info(`  • ${item.name}:`);
              item.issues.forEach((issue) => console.info(`    - ${issue}`));
            });
            console.info('');
          }

          const totalTools = validation.valid.length + validation.invalid.length;
          const validPercentage = Math.round((validation.valid.length / totalTools) * 100);
          console.info(
            `Overall: ${validation.valid.length}/${totalTools} tools valid (${validPercentage}%)`,
          );
        }
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  // Report command
  cmd
    .command('report')
    .description('Generate comprehensive tool capabilities report')
    .option('--output <file>', 'Output file path (markdown format)')
    .option('--stdout', 'Output to stdout instead of file')
    .action(async (options) => {
      try {
        const result = await generateToolCapabilitiesReport();
        if (!result.ok) {
          handleResultError(result, 'Failed to generate report');
        }

        const report = result.value;

        if (options.stdout) {
          console.info(report);
        } else {
          const outputPath =
            options.output || path.join(process.cwd(), 'tool-capabilities-report.md');
          writeFileSync(outputPath, report, 'utf-8');
          console.info(`📄 Report generated: ${outputPath}`);
        }
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  // Capability command
  cmd
    .command('capability <capability>')
    .description('List tools with specific enhancement capability')
    .action(async (capability) => {
      try {
        const result = await discoverToolCapabilities({ hasCapability: capability });
        if (!result.ok) {
          handleResultError(result, 'Failed to get tools with capability');
        }

        const tools = result.value.map((tool) => tool.name);

        if (tools.length === 0) {
          console.info(`No tools found with capability: ${capability}`);
        } else {
          console.info(`🎯 Tools with capability "${capability}":\n`);
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  // Sampling command
  cmd
    .command('sampling <strategy>')
    .description('List tools using specific sampling strategy')
    .action(async (strategy) => {
      try {
        if (!['single', 'none'].includes(strategy)) {
          console.error(formatError('Invalid sampling strategy. Use: single or none'));
          process.exit(1);
        }

        const result = await discoverToolCapabilities({
          samplingStrategy: strategy as 'single' | 'none',
        });
        if (!result.ok) {
          handleResultError(result, 'Failed to get tools with sampling strategy');
        }

        const tools = result.value.map((tool) => tool.name);

        if (tools.length === 0) {
          console.info(`No tools found with sampling strategy: ${strategy}`);
        } else {
          console.info(`🎲 Tools using "${strategy}" sampling:\n`);
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        handleGenericError('Error during operation', error);
      }
    });

  return cmd;
}
