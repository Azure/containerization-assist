/**
 * Shared CLI rendering utilities for consistent output formatting
 */

import type { Tool } from '@/tools';

export interface RenderOptions {
  format: 'table' | 'csv' | 'json';
  detailed?: boolean;
}

/**
 * Render tools in the specified format
 */
export function renderTools(tools: Tool[], options: RenderOptions): void {
  switch (options.format) {
    case 'table':
      outputTable(tools, options.detailed ?? false);
      break;
    case 'csv':
      outputCSV(tools);
      break;
    case 'json':
      console.info(JSON.stringify(tools, null, 2));
      break;
    default:
      throw new Error(`Unsupported format: ${options.format}`);
  }
}

/**
 * Output tools in table format
 */
function outputTable(tools: Tool[], _detailed: boolean = false): void {
  if (tools.length === 0) {
    console.info('No tools found matching criteria');
    return;
  }

  // Calculate column widths
  const nameWidth = Math.max(4, Math.max(...tools.map((t) => t.name.length)));
  const categoryWidth = Math.max(8, Math.max(...tools.map((t) => (t.category || '').length)));

  // Header
  console.info(`┌─${'─'.repeat(nameWidth)}─┬─${'─'.repeat(categoryWidth)}─┬─KE─┐`);
  console.info(`│ ${'Name'.padEnd(nameWidth)} │ ${'Category'.padEnd(categoryWidth)} │ KE │`);
  console.info(`├─${'─'.repeat(nameWidth)}─┼─${'─'.repeat(categoryWidth)}─┼────┤`);

  // Rows
  for (const tool of tools) {
    const name = tool.name.padEnd(nameWidth);
    const category = (tool.category || '').padEnd(categoryWidth);
    const ke = tool.metadata.knowledgeEnhanced ? ' ✅ ' : ' ❌ ';

    console.info(`│ ${name} │ ${category} │${ke}│`);
  }

  console.info(`└─${'─'.repeat(nameWidth)}─┴─${'─'.repeat(categoryWidth)}─┴────┘`);
  console.info(`\nTotal: ${tools.length} tools`);
}

/**
 * Output tools in CSV format
 */
function outputCSV(tools: Tool[]): void {
  console.info('Name,Category,Knowledge-Enhanced');
  for (const tool of tools) {
    console.info(`${tool.name},${tool.category || ''},${tool.metadata.knowledgeEnhanced}`);
  }
}
