/**
 * Validate Tools CLI Command
 *
 * Provides comprehensive validation of tool metadata compliance,
 * consistency checks, and detailed reporting for PR #3 requirements.
 */

import { Command } from 'commander';
import { getAllInternalTools } from '@/tools';
import {
  validateToolMetadata,
  validateMetadataConsistency as validateSingleTool,
} from '@/types/tool-metadata';

export interface ValidationReport {
  summary: {
    totalTools: number;
    validTools: number;
    invalidTools: number;
    compliancePercentage: number;
  };
  validTools: string[];
  invalidTools: Array<{
    name: string;
    issues: string[];
    suggestions: string[];
  }>;
  metadataErrors: Array<{
    name: string;
    error: string;
  }>;
  consistencyErrors: Array<{
    name: string;
    error: string;
  }>;
}

/**
 * Validates all tool metadata and returns comprehensive report
 */
export async function validateAllToolMetadata(): Promise<
  { ok: true; value: ValidationReport } | { ok: false; error: string }
> {
  try {
    const allTools = getAllInternalTools();
    const validTools: string[] = [];
    const invalidTools: Array<{ name: string; issues: string[]; suggestions: string[] }> = [];
    const metadataErrors: Array<{ name: string; error: string }> = [];
    const consistencyErrors: Array<{ name: string; error: string }> = [];

    for (const tool of allTools) {
      const issues: string[] = [];
      const suggestions: string[] = [];

      // Validate metadata schema compliance
      const metadataValidation = validateToolMetadata(tool.metadata);
      if (!metadataValidation.ok) {
        metadataErrors.push({
          name: tool.name,
          error: metadataValidation.error,
        });
        issues.push('Invalid metadata schema');
        suggestions.push('Fix metadata schema validation errors');
        continue;
      }

      // Validate metadata consistency
      const consistencyValidation = validateSingleTool(tool.name, tool.metadata);
      if (!consistencyValidation.ok) {
        consistencyErrors.push({
          name: tool.name,
          error: consistencyValidation.error,
        });
        issues.push('Metadata consistency issues');
        suggestions.push('Review and fix metadata consistency');
      }

      // Additional validation checks
      if (tool.metadata.aiDriven && tool.metadata.samplingStrategy === 'none') {
        issues.push('AI-driven tool should have sampling strategy');
        suggestions.push('Set samplingStrategy to "single" or "rerank"');
      }

      if (tool.metadata.knowledgeEnhanced && tool.metadata.enhancementCapabilities.length === 0) {
        issues.push('Knowledge-enhanced tool missing capabilities');
        suggestions.push('Add appropriate enhancementCapabilities array');
      }

      if (tool.metadata.knowledgeEnhanced && !tool.metadata.aiDriven) {
        issues.push('Knowledge-enhanced should be AI-driven');
        suggestions.push('Set aiDriven: true for knowledge-enhanced tools');
      }

      // Check for proper capability specification
      if (tool.metadata.aiDriven && tool.metadata.enhancementCapabilities.length === 0) {
        issues.push('AI-driven tool should specify capabilities');
        suggestions.push('Add relevant enhancementCapabilities');
      }

      if (issues.length > 0) {
        invalidTools.push({
          name: tool.name,
          issues,
          suggestions,
        });
      } else {
        validTools.push(tool.name);
      }
    }

    const totalTools = allTools.length;
    const validCount = validTools.length;
    const compliancePercentage = Math.round((validCount / totalTools) * 100);

    return {
      ok: true,
      value: {
        summary: {
          totalTools,
          validTools: validCount,
          invalidTools: totalTools - validCount,
          compliancePercentage,
        },
        validTools,
        invalidTools,
        metadataErrors,
        consistencyErrors,
      },
    };
  } catch (error) {
    return {
      ok: false,
      error: `Validation failed: ${error instanceof Error ? error.message : String(error)}`,
    };
  }
}

/**
 * Create validate-tools CLI command
 */
export function createValidateToolsCommand(): Command {
  const cmd = new Command('validate-tools');
  cmd.description('Validate tool metadata compliance and consistency (PR #3)');

  // Main validation command
  cmd
    .command('all')
    .description('Validate all tools for metadata compliance')
    .option('--json', 'Output as JSON')
    .option('--detailed', 'Include detailed error information')
    .option('--fix-suggestions', 'Include fix suggestions')
    .action(async (options) => {
      try {
        const result = await validateAllToolMetadata();
        if (!result.ok) {
          console.error('❌ Validation failed:', result.error);
          process.exit(1);
        }

        const report = result.value;

        if (options.json) {
          console.info(JSON.stringify(report, null, 2));
          return;
        }

        // Human-readable output
        console.info('🔍 Tool Metadata Validation Report');
        console.info('=====================================\n');

        // Summary
        console.info('📊 Summary:');
        console.info(`   Total Tools: ${report.summary.totalTools}`);
        console.info(`   Valid Tools: ${report.summary.validTools}`);
        console.info(`   Invalid Tools: ${report.summary.invalidTools}`);
        console.info(`   Compliance: ${report.summary.compliancePercentage}%\n`);

        // Valid tools
        if (report.validTools.length > 0) {
          console.info(`✅ Valid Tools (${report.validTools.length}):`);
          report.validTools.forEach((name) => console.info(`   • ${name}`));
          console.info('');
        }

        // Metadata errors
        if (report.metadataErrors.length > 0) {
          console.info(`❌ Schema Validation Errors (${report.metadataErrors.length}):`);
          report.metadataErrors.forEach(({ name, error }) => {
            console.info(`   • ${name}: ${error}`);
          });
          console.info('');
        }

        // Consistency errors
        if (report.consistencyErrors.length > 0) {
          console.info(`⚠️ Consistency Issues (${report.consistencyErrors.length}):`);
          report.consistencyErrors.forEach(({ name, error }) => {
            console.info(`   • ${name}: ${error}`);
          });
          console.info('');
        }

        // Invalid tools with issues
        if (report.invalidTools.length > 0) {
          console.info(`🔧 Tools Needing Fixes (${report.invalidTools.length}):`);
          report.invalidTools.forEach(({ name, issues, suggestions }) => {
            console.info(`   • ${name}:`);
            issues.forEach((issue) => console.info(`     - Issue: ${issue}`));
            if (options.fixSuggestions && suggestions.length > 0) {
              suggestions.forEach((suggestion) => console.info(`     + Fix: ${suggestion}`));
            }
          });
          console.info('');
        }

        // Exit with error code if validation failed
        if (report.summary.invalidTools > 0) {
          console.info(`❌ Validation failed: ${report.summary.invalidTools} tools need fixes`);
          process.exit(1);
        } else {
          console.info('✅ All tools have valid metadata!');
        }
      } catch (error) {
        console.error('❌ Error:', error);
        process.exit(1);
      }
    });

  // Quick compliance check
  cmd
    .command('quick')
    .description('Quick compliance check (exit code only)')
    .action(async () => {
      try {
        const result = await validateAllToolMetadata();
        if (!result.ok) {
          process.exit(1);
        }

        const report = result.value;
        console.info(
          `${report.summary.compliancePercentage}% compliant (${report.summary.validTools}/${report.summary.totalTools})`,
        );

        if (report.summary.invalidTools > 0) {
          process.exit(1);
        }
      } catch (error) {
        console.error('❌ Error:', error);
        process.exit(1);
      }
    });

  // CI-friendly validation
  cmd
    .command('ci')
    .description('CI-friendly validation with structured output')
    .action(async () => {
      try {
        const result = await validateAllToolMetadata();
        if (!result.ok) {
          console.error(`::error::Tool validation failed: ${result.error}`);
          process.exit(1);
        }

        const report = result.value;

        // Output GitHub Actions annotations
        report.metadataErrors.forEach(({ name, error }) => {
          console.error(
            `::error file=src/tools/${name}/tool.ts::Schema validation failed: ${error}`,
          );
        });

        report.consistencyErrors.forEach(({ name, error }) => {
          console.error(
            `::error file=src/tools/${name}/tool.ts::Consistency check failed: ${error}`,
          );
        });

        report.invalidTools.forEach(({ name, issues }) => {
          issues.forEach((issue) => {
            console.error(`::error file=src/tools/${name}/tool.ts::${issue}`);
          });
        });

        if (report.summary.invalidTools > 0) {
          console.error(`::error::${report.summary.invalidTools} tools failed validation`);
          process.exit(1);
        } else {
          console.info(
            `::notice::All ${report.summary.totalTools} tools passed validation (${report.summary.compliancePercentage}% compliant)`,
          );
        }
      } catch (error) {
        console.error(`::error::Validation error: ${error}`);
        process.exit(1);
      }
    });

  return cmd;
}

/**
 * Default export for CLI integration
 */
export default createValidateToolsCommand;
