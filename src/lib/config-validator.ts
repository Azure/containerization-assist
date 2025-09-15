/**
 * Configuration Validation Utilities
 *
 * Provides validation for sampling configuration files
 */

import {
  createConfigurationManager,
  validateConfigurationPure,
  type SamplingConfiguration,
} from './sampling-config';
import { ValidationResult } from '@/validation/core-types';

export interface ConfigValidatorInterface {
  validate(config: SamplingConfiguration): Promise<ValidationResult>;
  validateStartup(): Promise<ValidationResult>;
}

export function createConfigValidator(configPath?: string): ConfigValidatorInterface {
  const configManager = createConfigurationManager(configPath);

  return {
    /**
     * Validate complete configuration
     */
    async validate(config: SamplingConfiguration): Promise<ValidationResult> {
      return validateConfigurationPure(config);
    },

    /**
     * Validate configuration file at startup
     */
    async validateStartup(): Promise<ValidationResult> {
      const loadResult = await configManager.loadConfiguration();

      if (!loadResult.ok) {
        return {
          isValid: false,
          errors: [loadResult.error],
        };
      }

      const config = configManager.getConfiguration();
      return validateConfigurationPure(config);
    },
  };
}
