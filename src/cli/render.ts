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
function outputTable(tools: Tool[], detailed: boolean = false): void {
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
    const ai = tool.metadata.aiDriven ? ' ✅ ' : ' ❌ ';
    const ke = tool.metadata.knowledgeEnhanced ? ' ✅ ' : ' ❌ ';
    const sampling = tool.metadata.samplingStrategy.padEnd(samplingWidth);

    console.info(`│ ${name} │ ${category} │${ai}│${ke}│ ${sampling} │`);

    if (detailed && tool.metadata.enhancementCapabilities.length > 0) {
      const caps = tool.metadata.enhancementCapabilities.join(', ');
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
function outputCSV(tools: Tool[]): void {
  console.info(
    'Name,Category,AI-Driven,Knowledge-Enhanced,Sampling Strategy,Enhancement Capabilities',
  );
  for (const tool of tools) {
    const caps = tool.metadata.enhancementCapabilities.join(';');
    console.info(
      `${tool.name},${tool.category || ''},${tool.metadata.aiDriven},${tool.metadata.knowledgeEnhanced},${tool.metadata.samplingStrategy},"${caps}"`,
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
