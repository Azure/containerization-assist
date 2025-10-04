/**
 * CLI Options Validation Module
 * Validates CLI options including workspace directory, config file, and log levels
 */

import { statSync } from 'node:fs';
import { extractErrorMessage } from '@/lib/error-utils';

/**
 * Validation result containing validity status and error messages
 */
export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

/**
 * CLI options interface (subset used for validation)
 */
export interface CLIOptions {
  logLevel?: string;
  workspace?: string;
  config?: string;
  dockerSocket?: string;
  [key: string]: any;
}

/**
 * Docker socket validation result
 */
export interface DockerSocketValidation {
  dockerSocket: string;
  warnings: string[];
}

/**
 * Valid log levels for the CLI
 */
const VALID_LOG_LEVELS = ['debug', 'info', 'warn', 'error'] as const;

/**
 * Validates log level option
 */
function validateLogLevel(logLevel: string | undefined): string[] {
  const errors: string[] = [];

  if (logLevel && !VALID_LOG_LEVELS.includes(logLevel as any)) {
    errors.push(`Invalid log level: ${logLevel}. Valid options: ${VALID_LOG_LEVELS.join(', ')}`);
  }

  return errors;
}

/**
 * Validates workspace directory exists and is accessible
 */
function validateWorkspace(workspace: string | undefined): string[] {
  const errors: string[] = [];

  if (!workspace) {
    return errors;
  }

  try {
    const stat = statSync(workspace);
    if (!stat.isDirectory()) {
      errors.push(`Workspace path is not a directory: ${workspace}`);
    }
  } catch (error) {
    const errorMsg = extractErrorMessage(error);
    if (errorMsg.includes('ENOENT')) {
      errors.push(`Workspace directory does not exist: ${workspace}`);
    } else if (errorMsg.includes('EACCES')) {
      errors.push(`Permission denied accessing workspace: ${workspace}`);
    } else {
      errors.push(`Cannot access workspace directory: ${workspace} (${errorMsg})`);
    }
  }

  return errors;
}

/**
 * Validates config file exists and is accessible
 */
function validateConfigFile(configPath: string | undefined): string[] {
  const errors: string[] = [];

  if (!configPath) {
    return errors;
  }

  try {
    statSync(configPath);
  } catch (error) {
    const errorMsg = extractErrorMessage(error);
    errors.push(`Configuration file not found: ${configPath} - ${errorMsg}`);
  }

  return errors;
}

/**
 * Validates CLI options and returns validation result
 *
 * @param opts - CLI options to validate
 * @param dockerValidation - Optional Docker socket validation result to include
 * @returns Validation result with errors
 */
export function validateOptions(
  opts: CLIOptions,
  dockerValidation?: DockerSocketValidation,
): ValidationResult {
  const errors: string[] = [];

  // Validate log level
  errors.push(...validateLogLevel(opts.logLevel));

  // Validate workspace directory
  errors.push(...validateWorkspace(opts.workspace));

  // Validate config file
  errors.push(...validateConfigFile(opts.config));

  // Include Docker socket validation warnings as errors if provided
  if (dockerValidation) {
    // Update the opts with the validated docker socket
    opts.dockerSocket = dockerValidation.dockerSocket;

    // Add warnings as non-fatal errors for user awareness
    if (dockerValidation.warnings.length > 0) {
      dockerValidation.warnings.forEach((warning) => {
        if (warning.includes('No valid Docker socket')) {
          errors.push(warning);
        } else if (!process.env.MCP_MODE) {
          console.error(`⚠️  ${warning}`);
        }
      });
    }
  }

  return { valid: errors.length === 0, errors };
}
