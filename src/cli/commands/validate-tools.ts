/**
 * Validate Tools CLI Command
 *
 * Provides comprehensive validation of tool metadata compliance,
 * consistency checks, and detailed reporting for PR #3 requirements.
 */

import { Command } from 'commander';
import { getAllInternalTools } from '@/tools';
import {
  validateAllToolMetadata as validateAllToolMetadataCore,
  type ValidationReport,
} from '@/types/tool-metadata';
import {
  handleResultError,
  handleGenericError,
  handleCIError,
  formatGitHubAnnotation,
} from '../error-formatting';

/**
 * Validates all tool metadata and returns comprehensive report
 */
export async function validateAllToolMetadata(): Promise<
  { ok: true; value: ValidationReport } | { ok: false; error: string }
> {
  const allTools = getAllInternalTools();
  const result = await validateAllToolMetadataCore([...allTools]);

  if (result.ok) {
    return { ok: true, value: result.value };
  } else {
    return { ok: false, error: result.error };
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
          handleResultError(result, 'Validation failed');
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
        handleGenericError('Error during validation', error);
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
          handleResultError(result, 'Quick validation failed');
        }

        const report = result.value;
        console.info(
          `${report.summary.compliancePercentage}% compliant (${report.summary.validTools}/${report.summary.totalTools})`,
        );

        if (report.summary.invalidTools > 0) {
          process.exit(1);
        }
      } catch (error) {
        handleGenericError('Error during quick validation', error);
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
          handleCIError('Tool validation failed', result.error);
        }

        const report = result.value;

        // Output GitHub Actions annotations
        report.metadataErrors.forEach(({ name, error }) => {
          console.error(
            formatGitHubAnnotation(
              'error',
              `Schema validation failed: ${error}`,
              `src/tools/${name}/tool.ts`,
            ),
          );
        });

        report.consistencyErrors.forEach(({ name, error }) => {
          console.error(
            formatGitHubAnnotation(
              'error',
              `Consistency check failed: ${error}`,
              `src/tools/${name}/tool.ts`,
            ),
          );
        });

        report.invalidTools.forEach(({ name, issues }) => {
          issues.forEach((issue) => {
            console.error(formatGitHubAnnotation('error', issue, `src/tools/${name}/tool.ts`));
          });
        });

        if (report.summary.invalidTools > 0) {
          console.error(
            formatGitHubAnnotation(
              'error',
              `${report.summary.invalidTools} tools failed validation`,
            ),
          );
          process.exit(1);
        } else {
          console.info(
            `::notice::All ${report.summary.totalTools} tools passed validation (${report.summary.compliancePercentage}% compliant)`,
          );
        }
      } catch (error) {
        handleCIError('Validation error', error);
      }
    });

  return cmd;
}

/**
 * Default export for CLI integration
 */
export default createValidateToolsCommand;
