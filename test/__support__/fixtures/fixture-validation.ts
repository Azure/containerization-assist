/**
 * Fixture Validation Utilities
 * Comprehensive validation for test fixtures and golden files
 */

import { promises as fs } from 'fs';
import * as path from 'path';
import { Result, Success, Failure } from '../../../src/types';
import { fixtureRegistry, FixtureMetadata } from './fixture-registry';
import { goldenFileLoader } from './golden-file-loader';

export interface ValidationRule<T = unknown> {
  name: string;
  description: string;
  validate: (data: T, metadata?: FixtureMetadata) => ValidationRuleResult;
}

export interface ValidationRuleResult {
  passed: boolean;
  message?: string;
  severity: 'error' | 'warning' | 'info';
}

export interface FixtureValidationReport {
  fixtureId: string;
  valid: boolean;
  score: number; // 0-100 validation score
  summary: {
    errors: number;
    warnings: number;
    info: number;
    total: number;
  };
  results: Array<{
    rule: string;
    result: ValidationRuleResult;
  }>;
  suggestions?: string[];
}

export interface ValidationOptions {
  includeWarnings: boolean;
  strictMode: boolean;
  customRules?: ValidationRule[];
  skipRules?: string[];
}

/**
 * Fixture validation engine
 */
export class FixtureValidator {
  private rules = new Map<string, ValidationRule>();

  constructor() {
    this.loadDefaultRules();
  }

  /**
   * Add custom validation rule
   */
  addRule<T>(rule: ValidationRule<T>): void {
    this.rules.set(rule.name, rule as ValidationRule);
  }

  /**
   * Remove validation rule
   */
  removeRule(ruleName: string): void {
    this.rules.delete(ruleName);
  }

  /**
   * Validate a fixture by ID
   */
  async validateFixture(
    fixtureId: string,
    options: ValidationOptions = this.getDefaultOptions()
  ): Promise<Result<FixtureValidationReport>> {
    try {
      // Initialize registry if needed
      await fixtureRegistry.initialize();
      
      const metadata = fixtureRegistry.getMetadata(fixtureId);
      if (!metadata) {
        return Failure(`Fixture not found: ${fixtureId}`);
      }

      // Load fixture data
      const loadResult = await fixtureRegistry.load(fixtureId);
      if (!loadResult.success) {
        return Failure(`Failed to load fixture ${fixtureId}: ${loadResult.error}`);
      }

      const data = loadResult.data;
      
      // Run validation rules
      const report = await this.runValidationRules(
        fixtureId,
        data,
        metadata,
        options
      );

      return Success(report);
    } catch (error) {
      return Failure(`Validation error for ${fixtureId}: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Validate all fixtures of a specific type
   */
  async validateFixturesByType(
    type: FixtureMetadata['type'],
    options: ValidationOptions = this.getDefaultOptions()
  ): Promise<Result<FixtureValidationReport[]>> {
    try {
      await fixtureRegistry.initialize();
      
      const fixtures = fixtureRegistry.find({ type });
      const reports: FixtureValidationReport[] = [];

      for (const fixture of fixtures) {
        const result = await this.validateFixture(fixture.id, options);
        if (result.success) {
          reports.push(result.data);
        } else {
          // Create error report
          reports.push({
            fixtureId: fixture.id,
            valid: false,
            score: 0,
            summary: { errors: 1, warnings: 0, info: 0, total: 1 },
            results: [{
              rule: 'load-fixture',
              result: {
                passed: false,
                message: result.error,
                severity: 'error'
              }
            }]
          });
        }
      }

      return Success(reports);
    } catch (error) {
      return Failure(`Failed to validate fixtures by type: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Validate golden files consistency
   */
  async validateGoldenFiles(): Promise<Result<FixtureValidationReport[]>> {
    const reports: FixtureValidationReport[] = [];

    try {
      // Load metadata to get expected golden files
      const metadataResult = await goldenFileLoader.loadMetadata();
      if (!metadataResult.success) {
        return Failure(`Failed to load golden file metadata: ${metadataResult.error}`);
      }

      const metadata = metadataResult.data;

      // Validate each tool's golden files
      for (const [toolName, toolInfo] of Object.entries(metadata.tools)) {
        for (const fixture of toolInfo.fixtures) {
          const report = await this.validateGoldenFile(toolName, fixture, toolInfo);
          reports.push(report);
        }
      }

      // Validate workflow golden files
      for (const [workflowName, workflowInfo] of Object.entries(metadata.workflows)) {
        for (const fixture of workflowInfo.fixtures) {
          const report = await this.validateWorkflowGoldenFile(workflowName, fixture);
          reports.push(report);
        }
      }

      return Success(reports);
    } catch (error) {
      return Failure(`Golden file validation failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Generate validation summary report
   */
  generateSummaryReport(reports: FixtureValidationReport[]): string {
    const summary = reports.reduce(
      (acc, report) => ({
        total: acc.total + 1,
        valid: acc.valid + (report.valid ? 1 : 0),
        errors: acc.errors + report.summary.errors,
        warnings: acc.warnings + report.summary.warnings,
        avgScore: acc.avgScore + report.score,
      }),
      { total: 0, valid: 0, errors: 0, warnings: 0, avgScore: 0 }
    );

    const avgScore = summary.total > 0 ? Math.round(summary.avgScore / summary.total) : 0;
    const successRate = summary.total > 0 ? Math.round((summary.valid / summary.total) * 100) : 0;

    const lines = [
      '=== Fixture Validation Summary ===',
      '',
      `📊 Overall Stats:`,
      `   Total Fixtures: ${summary.total}`,
      `   Valid Fixtures: ${summary.valid}`,
      `   Success Rate: ${successRate}%`,
      `   Average Score: ${avgScore}/100`,
      '',
      `🔍 Issues Found:`,
      `   Errors: ${summary.errors}`,
      `   Warnings: ${summary.warnings}`,
      '',
      '📋 Detailed Results:',
      ''
    ];

    // Add top failures
    const failures = reports
      .filter(r => !r.valid)
      .sort((a, b) => a.score - b.score)
      .slice(0, 10);

    if (failures.length > 0) {
      lines.push('❌ Top Failures:');
      failures.forEach(failure => {
        lines.push(`   ${failure.fixtureId} (Score: ${failure.score}/100)`);
        const mainError = failure.results.find(r => r.result.severity === 'error');
        if (mainError) {
          lines.push(`     Error: ${mainError.result.message}`);
        }
      });
      lines.push('');
    }

    // Add top performers
    const successes = reports
      .filter(r => r.valid)
      .sort((a, b) => b.score - a.score)
      .slice(0, 5);

    if (successes.length > 0) {
      lines.push('✅ Top Performers:');
      successes.forEach(success => {
        lines.push(`   ${success.fixtureId} (Score: ${success.score}/100)`);
      });
    }

    return lines.join('\n');
  }

  private async runValidationRules(
    fixtureId: string,
    data: unknown,
    metadata: FixtureMetadata,
    options: ValidationOptions
  ): Promise<FixtureValidationReport> {
    const results: FixtureValidationReport['results'] = [];
    
    // Get applicable rules
    const applicableRules = Array.from(this.rules.values()).filter(rule => 
      !options.skipRules?.includes(rule.name)
    );

    // Run each rule
    for (const rule of applicableRules) {
      try {
        const result = rule.validate(data, metadata);
        
        // Filter based on options
        if (!options.includeWarnings && result.severity === 'warning') {
          continue;
        }

        results.push({
          rule: rule.name,
          result
        });
      } catch (error) {
        results.push({
          rule: rule.name,
          result: {
            passed: false,
            message: `Rule execution failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
            severity: 'error'
          }
        });
      }
    }

    // Calculate summary
    const summary = results.reduce(
      (acc, { result }) => {
        acc.total++;
        if (result.severity === 'error') acc.errors++;
        else if (result.severity === 'warning') acc.warnings++;
        else acc.info++;
        return acc;
      },
      { errors: 0, warnings: 0, info: 0, total: 0 }
    );

    // Calculate score (0-100)
    const passed = results.filter(r => r.result.passed).length;
    const score = summary.total > 0 ? Math.round((passed / summary.total) * 100) : 100;
    
    // Determine validity (no errors in strict mode, or score above threshold)
    const valid = options.strictMode 
      ? summary.errors === 0
      : summary.errors === 0 && score >= 80;

    return {
      fixtureId,
      valid,
      score,
      summary,
      results,
      suggestions: this.generateSuggestions(results, metadata)
    };
  }

  private async validateGoldenFile(
    toolName: string,
    fixture: string,
    toolInfo: any
  ): Promise<FixtureValidationReport> {
    const fixtureId = `golden-${toolName}-${fixture}`;
    
    try {
      const goldenData = await goldenFileLoader.loadToolGoldenFile(toolName, fixture);
      
      if (!goldenData.success || !goldenData.data) {
        return {
          fixtureId,
          valid: false,
          score: 0,
          summary: { errors: 1, warnings: 0, info: 0, total: 1 },
          results: [{
            rule: 'golden-file-exists',
            result: {
              passed: false,
              message: `Golden file not found: ${toolName}/${fixture}`,
              severity: 'error'
            }
          }]
        };
      }

      // Validate golden file structure
      const data = goldenData.data;
      const metadata: FixtureMetadata = {
        id: fixtureId,
        name: `Golden file for ${toolName}`,
        type: 'golden',
        category: toolName,
        tags: ['golden', toolName, fixture],
        description: `Expected output for ${toolName} tool`,
        version: '1.0.0',
        lastUpdated: new Date(),
        path: `golden/tools/${toolName}/${fixture}.json`
      };

      return await this.runValidationRules(fixtureId, data, metadata, this.getDefaultOptions());
    } catch (error) {
      return {
        fixtureId,
        valid: false,
        score: 0,
        summary: { errors: 1, warnings: 0, info: 0, total: 1 },
        results: [{
          rule: 'golden-file-validation',
          result: {
            passed: false,
            message: `Validation error: ${error instanceof Error ? error.message : 'Unknown error'}`,
            severity: 'error'
          }
        }]
      };
    }
  }

  private async validateWorkflowGoldenFile(
    workflowName: string,
    fixture: string
  ): Promise<FixtureValidationReport> {
    const fixtureId = `golden-workflow-${workflowName}-${fixture}`;
    
    try {
      const goldenData = await goldenFileLoader.loadWorkflowGoldenFile(workflowName, fixture);
      
      if (!goldenData.success || !goldenData.data) {
        return {
          fixtureId,
          valid: false,
          score: 0,
          summary: { errors: 1, warnings: 0, info: 0, total: 1 },
          results: [{
            rule: 'workflow-golden-file-exists',
            result: {
              passed: false,
              message: `Workflow golden file not found: ${workflowName}/${fixture}`,
              severity: 'error'
            }
          }]
        };
      }

      const data = goldenData.data;
      const metadata: FixtureMetadata = {
        id: fixtureId,
        name: `Workflow golden file for ${workflowName}`,
        type: 'golden',
        category: 'workflow',
        tags: ['golden', 'workflow', workflowName, fixture],
        description: `Expected output for ${workflowName} workflow`,
        version: '1.0.0',
        lastUpdated: new Date(),
        path: `golden/workflows/${workflowName}-${fixture}.json`
      };

      return await this.runValidationRules(fixtureId, data, metadata, this.getDefaultOptions());
    } catch (error) {
      return {
        fixtureId,
        valid: false,
        score: 0,
        summary: { errors: 1, warnings: 0, info: 0, total: 1 },
        results: [{
          rule: 'workflow-validation',
          result: {
            passed: false,
            message: `Validation error: ${error instanceof Error ? error.message : 'Unknown error'}`,
            severity: 'error'
          }
        }]
      };
    }
  }

  private generateSuggestions(
    results: FixtureValidationReport['results'],
    metadata: FixtureMetadata
  ): string[] {
    const suggestions: string[] = [];

    const errorCount = results.filter(r => r.result.severity === 'error').length;
    const warningCount = results.filter(r => r.result.severity === 'warning').length;

    if (errorCount > 0) {
      suggestions.push(`Fix ${errorCount} error${errorCount > 1 ? 's' : ''} to make fixture valid`);
    }

    if (warningCount > 0) {
      suggestions.push(`Address ${warningCount} warning${warningCount > 1 ? 's' : ''} to improve quality`);
    }

    // Type-specific suggestions
    if (metadata.type === 'golden') {
      suggestions.push('Ensure golden file matches current tool output format');
      suggestions.push('Update golden file if tool behavior has changed');
    }

    if (metadata.type === 'project') {
      suggestions.push('Verify project structure matches real-world examples');
      suggestions.push('Add missing configuration files if needed');
    }

    return suggestions;
  }

  private loadDefaultRules(): void {
    // JSON structure validation
    this.addRule({
      name: 'json-structure',
      description: 'Validate JSON structure and syntax',
      validate: (data, metadata) => {
        if (metadata?.type === 'golden' || metadata?.path.endsWith('.json')) {
          try {
            JSON.stringify(data);
            return { passed: true, severity: 'info' as const };
          } catch (error) {
            return {
              passed: false,
              message: `Invalid JSON structure: ${error instanceof Error ? error.message : 'Unknown error'}`,
              severity: 'error' as const
            };
          }
        }
        return { passed: true, severity: 'info' as const };
      }
    });

    // Required fields validation
    this.addRule({
      name: 'required-fields',
      description: 'Check for required fields in fixtures',
      validate: (data, metadata) => {
        if (metadata?.type === 'golden' && typeof data === 'object' && data !== null) {
          const goldenData = data as Record<string, unknown>;
          
          // Basic structure checks
          if (Object.keys(goldenData).length === 0) {
            return {
              passed: false,
              message: 'Golden file is empty',
              severity: 'error' as const
            };
          }
          
          // Tool-specific validations could go here
        }
        
        return { passed: true, severity: 'info' as const };
      }
    });

    // File size validation
    this.addRule({
      name: 'file-size',
      description: 'Check fixture file size is reasonable',
      validate: (data, metadata) => {
        const dataString = JSON.stringify(data);
        const sizeKB = Buffer.byteLength(dataString, 'utf8') / 1024;
        
        if (sizeKB > 1000) { // > 1MB
          return {
            passed: false,
            message: `Fixture is very large (${Math.round(sizeKB)}KB). Consider splitting or optimizing.`,
            severity: 'warning' as const
          };
        } else if (sizeKB > 100) { // > 100KB
          return {
            passed: true,
            message: `Fixture is moderately large (${Math.round(sizeKB)}KB)`,
            severity: 'info' as const
          };
        }
        
        return { passed: true, severity: 'info' as const };
      }
    });

    // Metadata consistency validation
    this.addRule({
      name: 'metadata-consistency',
      description: 'Validate fixture metadata consistency',
      validate: (data, metadata) => {
        if (!metadata) {
          return {
            passed: false,
            message: 'Missing fixture metadata',
            severity: 'error' as const
          };
        }

        const warnings: string[] = [];
        
        if (!metadata.description) {
          warnings.push('Missing description');
        }
        
        if (metadata.tags.length === 0) {
          warnings.push('No tags specified');
        }
        
        if (warnings.length > 0) {
          return {
            passed: true,
            message: `Metadata issues: ${warnings.join(', ')}`,
            severity: 'warning' as const
          };
        }
        
        return { passed: true, severity: 'info' as const };
      }
    });
  }

  private getDefaultOptions(): ValidationOptions {
    return {
      includeWarnings: true,
      strictMode: false,
      customRules: [],
      skipRules: []
    };
  }
}

/**
 * Global validator instance
 */
export const fixtureValidator = new FixtureValidator();

/**
 * Convenience functions for common validation tasks
 */
export async function validateFixture(fixtureId: string): Promise<FixtureValidationReport | null> {
  const result = await fixtureValidator.validateFixture(fixtureId);
  return result.success ? result.data : null;
}

export async function validateAllGoldenFiles(): Promise<FixtureValidationReport[]> {
  const result = await fixtureValidator.validateGoldenFiles();
  return result.success ? result.data : [];
}

export async function validateFixturesByType(
  type: FixtureMetadata['type']
): Promise<FixtureValidationReport[]> {
  const result = await fixtureValidator.validateFixturesByType(type);
  return result.success ? result.data : [];
}