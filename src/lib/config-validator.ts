/**
 * Configuration Validation Utilities
 *
 * Provides validation for sampling configuration files
 */

import { ConfigurationManager, type SamplingConfiguration } from './sampling-config';
import { ValidationResult } from '../validation/core-types';

export class ConfigValidator {
  constructor(private configManager: ConfigurationManager) {}

  /**
   * Validate complete configuration
   */
  async validate(config: SamplingConfiguration): Promise<ValidationResult> {
    return this.configManager.validateConfiguration(config);
  }

  /**
   * Validate configuration file at startup
   */
  async validateStartup(): Promise<ValidationResult> {
    const loadResult = await this.configManager.loadConfiguration();

    if (!loadResult.ok) {
      return {
        isValid: false,
        errors: [loadResult.error],
      };
    }

    const config = this.configManager.getConfiguration();
    return this.validate(config);
  }
}

/**
 * Create validator instance
 */
export function createConfigValidator(configPath?: string): ConfigValidator {
  const configManager = new ConfigurationManager(configPath);
  return new ConfigValidator(configManager);
}
