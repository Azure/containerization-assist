/**
 * Inspect Tools CLI Command
 *
 * Provides command-line interface for tool discovery, capability analysis,
 * and metadata inspection for AI enhancement features.
 */

import { Command } from 'commander';
import {
  discoverToolCapabilities,
  getAIDrivenTools,
  getKnowledgeEnhancedTools,
  getToolStatistics,
  validateToolMetadata,
  generateToolCapabilitiesReport,
  getToolsBySamplingStrategy,
  getToolsWithCapability,
  type ToolDiscoveryOptions,
} from '@/mcp/tools/tool-discovery';
import { writeFileSync } from 'node:fs';
import path from 'node:path';

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
    .option('--ai-driven', 'Show only AI-driven tools')
    .option('--knowledge-enhanced', 'Show only knowledge-enhanced tools')
    .option('--sampling <strategy>', 'Filter by sampling strategy (rerank, single, none)')
    .option('--capability <name>', 'Filter by enhancement capability')
    .option('--format <format>', 'Output format (table, json, csv)', 'table')
    .option('--detailed', 'Include detailed metadata')
    .action(async (options) => {
      try {
        const discoveryOptions: ToolDiscoveryOptions = {
          aiDrivenOnly: options.aiDriven,
          knowledgeEnhancedOnly: options.knowledgeEnhanced,
          samplingStrategy: options.sampling,
          hasCapability: options.capability,
          includeDetails: options.detailed,
        };

        const result = await discoverToolCapabilities(discoveryOptions);
        if (!result.ok) {
          console.error('❌ Failed to discover tools:', result.error);
          process.exit(1);
        }

        const tools = result.value;

        switch (options.format) {
          case 'json':
            console.info(JSON.stringify(tools, null, 2));
            break;
          case 'csv':
            outputCSV(tools);
            break;
          case 'table':
          default:
            outputTable(tools, options.detailed);
            break;
        }
      } catch (error) {
        console.error('❌ Error:', error);
        process.exit(1);
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
          console.error('❌ Failed to get statistics:', result.error);
          process.exit(1);
        }

        const stats = result.value;

        if (options.json) {
          console.info(JSON.stringify(stats, null, 2));
        } else {
          console.info('📊 Tool Statistics\n');
          console.info(`Total Tools: ${stats.total}`);
          console.info(
            `AI-Driven: ${stats.aiDriven} (${Math.round((stats.aiDriven / stats.total) * 100)}%)`,
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
        console.error('❌ Error:', error);
        process.exit(1);
      }
    });

  // AI-driven tools command
  cmd
    .command('ai-driven')
    .description('List tools that use AI for content generation')
    .option('--count', 'Show only count')
    .action(async (options) => {
      try {
        const result = await getAIDrivenTools();
        if (!result.ok) {
          console.error('❌ Failed to get AI-driven tools:', result.error);
          process.exit(1);
        }

        const tools = result.value;

        if (options.count) {
          console.info(tools.length);
        } else {
          console.info('🤖 AI-Driven Tools:\n');
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        console.error('❌ Error:', error);
        process.exit(1);
      }
    });

  // Knowledge-enhanced tools command
  cmd
    .command('knowledge-enhanced')
    .description('List tools that use knowledge enhancement')
    .option('--count', 'Show only count')
    .action(async (options) => {
      try {
        const result = await getKnowledgeEnhancedTools();
        if (!result.ok) {
          console.error('❌ Failed to get knowledge-enhanced tools:', result.error);
          process.exit(1);
        }

        const tools = result.value;

        if (options.count) {
          console.info(tools.length);
        } else {
          console.info('📚 Knowledge-Enhanced Tools:\n');
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        console.error('❌ Error:', error);
        process.exit(1);
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
          console.error('❌ Failed to validate metadata:', result.error);
          process.exit(1);
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
        console.error('❌ Error:', error);
        process.exit(1);
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
          console.error('❌ Failed to generate report:', result.error);
          process.exit(1);
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
        console.error('❌ Error:', error);
        process.exit(1);
      }
    });

  // Capability command
  cmd
    .command('capability <capability>')
    .description('List tools with specific enhancement capability')
    .action(async (capability) => {
      try {
        const result = await getToolsWithCapability(capability);
        if (!result.ok) {
          console.error('❌ Failed to get tools with capability:', result.error);
          process.exit(1);
        }

        const tools = result.value;

        if (tools.length === 0) {
          console.info(`No tools found with capability: ${capability}`);
        } else {
          console.info(`🎯 Tools with capability "${capability}":\n`);
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        console.error('❌ Error:', error);
        process.exit(1);
      }
    });

  // Sampling command
  cmd
    .command('sampling <strategy>')
    .description('List tools using specific sampling strategy')
    .action(async (strategy) => {
      try {
        if (!['rerank', 'single', 'none'].includes(strategy)) {
          console.error('❌ Invalid sampling strategy. Use: rerank, single, or none');
          process.exit(1);
        }

        const result = await getToolsBySamplingStrategy(strategy as 'rerank' | 'single' | 'none');
        if (!result.ok) {
          console.error('❌ Failed to get tools with sampling strategy:', result.error);
          process.exit(1);
        }

        const tools = result.value;

        if (tools.length === 0) {
          console.info(`No tools found with sampling strategy: ${strategy}`);
        } else {
          console.info(`🎲 Tools using "${strategy}" sampling:\n`);
          tools.forEach((tool) => console.info(`  • ${tool}`));
          console.info(`\nTotal: ${tools.length} tools`);
        }
      } catch (error) {
        console.error('❌ Error:', error);
        process.exit(1);
      }
    });

  return cmd;
}

/**
 * Output tools in table format
 */
function outputTable(tools: any[], detailed: boolean = false): void {
  if (tools.length === 0) {
    console.info('No tools found matching criteria');
    return;
  }

  // Calculate column widths
  const nameWidth = Math.max(4, Math.max(...tools.map((t) => t.name.length)));
  const categoryWidth = Math.max(8, Math.max(...tools.map((t) => (t.category || '').length)));
  const samplingWidth = 8; // Fixed width for sampling strategy

  // Header
  console.info(
    `┌─${'─'.repeat(nameWidth)}─┬─${'─'.repeat(categoryWidth)}─┬─AI─┬─KE─┬─${'─'.repeat(samplingWidth)}─┐`,
  );
  console.info(
    `│ ${'Name'.padEnd(nameWidth)} │ ${'Category'.padEnd(categoryWidth)} │ AI │ KE │ ${'Sampling'.padEnd(samplingWidth)} │`,
  );
  console.info(
    `├─${'─'.repeat(nameWidth)}─┼─${'─'.repeat(categoryWidth)}─┼────┼────┼─${'─'.repeat(samplingWidth)}─┤`,
  );

  // Rows
  for (const tool of tools) {
    const name = tool.name.padEnd(nameWidth);
    const category = (tool.category || '').padEnd(categoryWidth);
    const ai = tool.aiDriven ? ' ✅ ' : ' ❌ ';
    const ke = tool.knowledgeEnhanced ? ' ✅ ' : ' ❌ ';
    const sampling = tool.samplingStrategy.padEnd(samplingWidth);

    console.info(`│ ${name} │ ${category} │${ai}│${ke}│ ${sampling} │`);

    if (detailed && tool.enhancementCapabilities.length > 0) {
      const caps = tool.enhancementCapabilities.join(', ');
      const wrappedCaps = wrapText(caps, nameWidth + categoryWidth + samplingWidth + 10);
      for (const line of wrappedCaps) {
        console.info(`│ ${line.padEnd(nameWidth + categoryWidth + samplingWidth + 10)} │`);
      }
    }
  }

  console.info(
    `└─${'─'.repeat(nameWidth)}─┴─${'─'.repeat(categoryWidth)}─┴────┴────┴─${'─'.repeat(samplingWidth)}─┘`,
  );
  console.info(`\nTotal: ${tools.length} tools`);
}

/**
 * Output tools in CSV format
 */
function outputCSV(tools: any[]): void {
  console.info(
    'Name,Category,AI-Driven,Knowledge-Enhanced,Sampling Strategy,Enhancement Capabilities',
  );
  for (const tool of tools) {
    const caps = tool.enhancementCapabilities.join(';');
    console.info(
      `${tool.name},${tool.category || ''},${tool.aiDriven},${tool.knowledgeEnhanced},${tool.samplingStrategy},"${caps}"`,
    );
  }
}

/**
 * Wrap text to fit within specified width
 */
function wrapText(text: string, width: number): string[] {
  if (text.length <= width) return [text];

  const lines: string[] = [];
  let current = '';

  for (const word of text.split(' ')) {
    if (current.length + word.length + 1 <= width) {
      current += (current ? ' ' : '') + word;
    } else {
      if (current) lines.push(current);
      current = word;
    }
  }

  if (current) lines.push(current);
  return lines;
}
