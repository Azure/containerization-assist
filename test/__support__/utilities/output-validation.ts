import { Result, Success, Failure } from "../../src/types"
import { Logger } from 'pino';
import * as fs from 'fs/promises';
import * as path from 'path';
import * as yaml from 'yaml';

export interface ValidationRule {
  name: string;
  description: string;
  type: 'dockerfile' | 'k8s' | 'compose' | 'json' | 'yaml' | 'custom';
  validator: (content: any, context?: ValidationContext) => ValidationResult;
  required?: boolean;
  severity?: 'error' | 'warning' | 'info';
}

export interface ValidationContext {
  repositoryType: string;
  language: string;
  framework: string;
  environment: string;
  expectedFeatures: string[];
  testData?: any;
}

export interface ValidationResult {
  passed: boolean;
  message: string;
  details?: string[];
  score?: number;
  suggestions?: string[];
}

export interface ExpectedOutput {
  testName: string;
  repositoryType: string;
  expectedFiles: ExpectedFile[];
  validationRules: ValidationRule[];
  customValidators?: Record<string, (content: any) => ValidationResult>;
}

export interface ExpectedFile {
  path: string;
  type: 'dockerfile' | 'k8s-manifest' | 'docker-compose' | 'json' | 'yaml' | 'text';
  required: boolean;
  contentRules: ValidationRule[];
}

export interface OutputValidationReport {
  testName: string;
  passed: boolean;
  totalChecks: number;
  passedChecks: number;
  failedChecks: number;
  warnings: number;
  errors: number;
  score: number;
  results: FileValidationResult[];
  summary: string;
  suggestions: string[];
}

export interface FileValidationResult {
  filePath: string;
  exists: boolean;
  passed: boolean;
  ruleResults: RuleValidationResult[];
  score: number;
}

export interface RuleValidationResult {
  rule: string;
  passed: boolean;
  severity: 'error' | 'warning' | 'info';
  message: string;
  details?: string[];
  suggestions?: string[];
}

export class OutputValidationFramework {
  private expectedOutputs: Map<string, ExpectedOutput> = new Map();
  private defaultRules: Map<string, ValidationRule[]> = new Map();

  constructor(
    private logger: Logger,
    private validationDataPath: string = './test/fixtures/expected-outputs'
  ) {
    this.initializeDefaultRules();
  }

  async initialize(): Promise<Result<void>> {
    try {
      await this.loadExpectedOutputs();
      this.logger.info('Output validation framework initialized');
      return Success(undefined);
    } catch (error) {
      return Failure(`Failed to initialize validation framework: ${error.message}`);
    }
  }

  async validateOutput(
    testName: string,
    actualOutputPath: string,
    context: ValidationContext
  ): Promise<Result<OutputValidationReport>> {
    try {
      const expectedOutput = this.expectedOutputs.get(testName);
      if (!expectedOutput) {
        return Failure(`No expected output configuration found for test: ${testName}`);
      }

      const report: OutputValidationReport = {
        testName,
        passed: true,
        totalChecks: 0,
        passedChecks: 0,
        failedChecks: 0,
        warnings: 0,
        errors: 0,
        score: 0,
        results: [],
        summary: '',
        suggestions: []
      };

      // Validate each expected file
      for (const expectedFile of expectedOutput.expectedFiles) {
        const filePath = path.join(actualOutputPath, expectedFile.path);
        const fileResult = await this.validateFile(filePath, expectedFile, context);
        
        report.results.push(fileResult);
        report.totalChecks += fileResult.ruleResults.length;
        
        for (const ruleResult of fileResult.ruleResults) {
          if (ruleResult.passed) {
            report.passedChecks++;
          } else {
            report.failedChecks++;
            if (ruleResult.severity === 'error') {
              report.errors++;
              report.passed = false;
            } else if (ruleResult.severity === 'warning') {
              report.warnings++;
            }
          }
        }
        
        report.score += fileResult.score;
      }

      // Apply custom validators
      if (expectedOutput.customValidators) {
        for (const [name, validator] of Object.entries(expectedOutput.customValidators)) {
          const customResult = validator(actualOutputPath);
          if (!customResult.passed && customResult.message) {
            report.suggestions.push(`Custom validation '${name}': ${customResult.message}`);
            if (customResult.suggestions) {
              report.suggestions.push(...customResult.suggestions);
            }
          }
        }
      }

      // Calculate final score
      if (report.totalChecks > 0) {
        report.score = Math.round((report.passedChecks / report.totalChecks) * 100);
      }

      // Generate summary
      report.summary = this.generateSummary(report);

      this.logger.info('Output validation completed', {
        testName,
        passed: report.passed,
        score: report.score,
        checks: `${report.passedChecks}/${report.totalChecks}`
      });

      return Success(report);

    } catch (error) {
      return Failure(`Output validation failed: ${error.message}`);
    }
  }

  private async validateFile(
    filePath: string,
    expectedFile: ExpectedFile,
    context: ValidationContext
  ): Promise<FileValidationResult> {
    const result: FileValidationResult = {
      filePath: expectedFile.path,
      exists: false,
      passed: true,
      ruleResults: [],
      score: 0
    };

    try {
      await fs.access(filePath);
      result.exists = true;
    } catch {
      result.exists = false;
      if (expectedFile.required) {
        result.ruleResults.push({
          rule: 'file-exists',
          passed: false,
          severity: 'error',
          message: `Required file '${expectedFile.path}' does not exist`,
          suggestions: [`Create the file '${expectedFile.path}'`]
        });
        result.passed = false;
        return result;
      }
    }

    if (!result.exists) {
      return result;
    }

    try {
      // Read and parse file content
      const content = await fs.readFile(filePath, 'utf8');
      let parsedContent: any = content;

      if (expectedFile.type === 'json') {
        parsedContent = JSON.parse(content);
      } else if (expectedFile.type === 'yaml' || expectedFile.type === 'k8s-manifest') {
        parsedContent = yaml.parse(content);
      }

      // Apply content rules
      let passedRules = 0;
      for (const rule of expectedFile.contentRules) {
        const ruleResult = rule.validator(parsedContent, context);
        
        result.ruleResults.push({
          rule: rule.name,
          passed: ruleResult.passed,
          severity: rule.severity || 'error',
          message: ruleResult.message,
          details: ruleResult.details,
          suggestions: ruleResult.suggestions
        });

        if (ruleResult.passed) {
          passedRules++;
        } else if (rule.severity === 'error') {
          result.passed = false;
        }
      }

      // Calculate file score
      if (expectedFile.contentRules.length > 0) {
        result.score = Math.round((passedRules / expectedFile.contentRules.length) * 100);
      } else {
        result.score = 100; // File exists and no specific rules
      }

    } catch (error) {
      result.ruleResults.push({
        rule: 'file-parse',
        passed: false,
        severity: 'error',
        message: `Failed to parse file: ${error.message}`,
        suggestions: ['Check file format and syntax']
      });
      result.passed = false;
      result.score = 0;
    }

    return result;
  }

  private async loadExpectedOutputs(): Promise<void> {
    try {
      await fs.access(this.validationDataPath);
      const files = await fs.readdir(this.validationDataPath);
      
      for (const file of files) {
        if (file.endsWith('.json')) {
          const filePath = path.join(this.validationDataPath, file);
          const content = await fs.readFile(filePath, 'utf8');
          const expectedOutput: ExpectedOutput = JSON.parse(content);
          this.expectedOutputs.set(expectedOutput.testName, expectedOutput);
        }
      }
    } catch (error) {
      this.logger.warn('No validation data directory found, using default rules only');
    }
  }

  private initializeDefaultRules(): void {
    // Dockerfile validation rules
    this.defaultRules.set('dockerfile', [
      {
        name: 'has-from-instruction',
        description: 'Dockerfile must have a FROM instruction',
        type: 'dockerfile',
        validator: (content: string) => ({
          passed: content.includes('FROM '),
          message: content.includes('FROM ') ? 'FROM instruction found' : 'Missing FROM instruction'
        }),
        required: true,
        severity: 'error'
      },
      {
        name: 'has-workdir',
        description: 'Dockerfile should specify WORKDIR',
        type: 'dockerfile',
        validator: (content: string) => ({
          passed: content.includes('WORKDIR '),
          message: content.includes('WORKDIR ') ? 'WORKDIR specified' : 'No WORKDIR specified',
          suggestions: ['Add WORKDIR instruction to set working directory']
        }),
        severity: 'warning'
      },
      {
        name: 'uses-non-root-user',
        description: 'Dockerfile should create and use non-root user',
        type: 'dockerfile',
        validator: (content: string) => ({
          passed: content.includes('USER ') && !content.includes('USER root'),
          message: content.includes('USER ') ? 'Non-root user configured' : 'Running as root user',
          suggestions: ['Add non-root user with RUN adduser and USER instructions']
        }),
        severity: 'warning'
      },
      {
        name: 'has-healthcheck',
        description: 'Dockerfile should include health check',
        type: 'dockerfile',
        validator: (content: string) => ({
          passed: content.includes('HEALTHCHECK '),
          message: content.includes('HEALTHCHECK ') ? 'Health check configured' : 'No health check configured',
          suggestions: ['Add HEALTHCHECK instruction for container health monitoring']
        }),
        severity: 'info'
      },
      {
        name: 'exposes-ports',
        description: 'Dockerfile should expose required ports',
        type: 'dockerfile',
        validator: (content: string, context?: ValidationContext) => {
          const hasExpose = content.includes('EXPOSE ');
          const expectedPorts = context?.testData?.expectedPorts || [];
          
          if (expectedPorts.length > 0) {
            const exposedPorts = expectedPorts.some((port: number) => 
              content.includes(`EXPOSE ${port}`)
            );
            return {
              passed: exposedPorts,
              message: exposedPorts ? 'Expected ports exposed' : 'Expected ports not exposed',
              details: [`Expected ports: ${expectedPorts.join(', ')}`]
            };
          }
          
          return {
            passed: hasExpose,
            message: hasExpose ? 'Ports exposed' : 'No ports exposed'
          };
        },
        severity: 'warning'
      }
    ]);

    // Kubernetes manifest validation rules
    this.defaultRules.set('k8s', [
      {
        name: 'has-deployment',
        description: 'Should include Deployment manifest',
        type: 'k8s',
        validator: (content: any) => ({
          passed: content?.kind === 'Deployment' || (Array.isArray(content) && content.some(m => m.kind === 'Deployment')),
          message: 'Deployment manifest validation'
        }),
        required: true,
        severity: 'error'
      },
      {
        name: 'has-service',
        description: 'Should include Service manifest',
        type: 'k8s',
        validator: (content: any) => ({
          passed: content?.kind === 'Service' || (Array.isArray(content) && content.some(m => m.kind === 'Service')),
          message: 'Service manifest validation'
        }),
        severity: 'warning'
      },
      {
        name: 'has-resource-limits',
        description: 'Deployment should specify resource limits',
        type: 'k8s',
        validator: (content: any) => {
          let deployment = content;
          if (Array.isArray(content)) {
            deployment = content.find(m => m.kind === 'Deployment');
          }
          
          if (!deployment) return { passed: false, message: 'No deployment found' };
          
          const containers = deployment?.spec?.template?.spec?.containers || [];
          const hasLimits = containers.some((container: any) => 
            container.resources?.limits?.memory && container.resources?.limits?.cpu
          );
          
          return {
            passed: hasLimits,
            message: hasLimits ? 'Resource limits specified' : 'No resource limits specified',
            suggestions: hasLimits ? [] : ['Add memory and CPU limits to containers']
          };
        },
        severity: 'warning'
      },
      {
        name: 'has-security-context',
        description: 'Pods should have security context configured',
        type: 'k8s',
        validator: (content: any) => {
          let deployment = content;
          if (Array.isArray(content)) {
            deployment = content.find(m => m.kind === 'Deployment');
          }
          
          if (!deployment) return { passed: false, message: 'No deployment found' };
          
          const podSpec = deployment?.spec?.template?.spec;
          const hasSecurityContext = podSpec?.securityContext || 
            podSpec?.containers?.some((c: any) => c.securityContext);
          
          return {
            passed: hasSecurityContext,
            message: hasSecurityContext ? 'Security context configured' : 'No security context configured',
            suggestions: hasSecurityContext ? [] : ['Add pod or container security context']
          };
        },
        severity: 'info'
      }
    ]);

    // Docker Compose validation rules
    this.defaultRules.set('compose', [
      {
        name: 'has-version',
        description: 'Docker Compose file should specify version',
        type: 'compose',
        validator: (content: any) => ({
          passed: !!content?.version,
          message: content?.version ? `Version ${content.version} specified` : 'No version specified'
        }),
        required: true,
        severity: 'error'
      },
      {
        name: 'has-services',
        description: 'Docker Compose file should define services',
        type: 'compose',
        validator: (content: any) => ({
          passed: !!content?.services && Object.keys(content.services).length > 0,
          message: content?.services ? `${Object.keys(content.services).length} services defined` : 'No services defined'
        }),
        required: true,
        severity: 'error'
      },
      {
        name: 'uses-healthchecks',
        description: 'Services should include health checks',
        type: 'compose',
        validator: (content: any) => {
          if (!content?.services) return { passed: false, message: 'No services found' };
          
          const services = Object.values(content.services) as any[];
          const withHealthchecks = services.filter(service => service.healthcheck);
          
          return {
            passed: withHealthchecks.length > 0,
            message: `${withHealthchecks.length}/${services.length} services have health checks`,
            suggestions: withHealthchecks.length === services.length ? [] : ['Add health checks to remaining services']
          };
        },
        severity: 'info'
      }
    ]);
  }

  getDefaultRules(type: string): ValidationRule[] {
    return this.defaultRules.get(type) || [];
  }

  addCustomRule(type: string, rule: ValidationRule): void {
    const existingRules = this.defaultRules.get(type) || [];
    existingRules.push(rule);
    this.defaultRules.set(type, existingRules);
  }

  private generateSummary(report: OutputValidationReport): string {
    const parts = [
      `${report.passedChecks}/${report.totalChecks} checks passed`,
      `Score: ${report.score}%`
    ];

    if (report.errors > 0) {
      parts.push(`${report.errors} errors`);
    }
    if (report.warnings > 0) {
      parts.push(`${report.warnings} warnings`);
    }

    return parts.join(', ');
  }

  async saveExpectedOutput(expectedOutput: ExpectedOutput): Promise<Result<void>> {
    try {
      const filePath = path.join(this.validationDataPath, `${expectedOutput.testName}.json`);
      await fs.mkdir(path.dirname(filePath), { recursive: true });
      await fs.writeFile(filePath, JSON.stringify(expectedOutput, null, 2));
      
      this.expectedOutputs.set(expectedOutput.testName, expectedOutput);
      return Success(undefined);
    } catch (error) {
      return Failure(`Failed to save expected output: ${error.message}`);
    }
  }

  async generateExpectedOutputFromActual(
    testName: string,
    actualOutputPath: string,
    context: ValidationContext
  ): Promise<Result<ExpectedOutput>> {
    try {
      const expectedFiles: ExpectedFile[] = [];
      
      // Scan actual output directory
      const files = await this.scanDirectory(actualOutputPath);
      
      for (const file of files) {
        const relativePath = path.relative(actualOutputPath, file);
        const fileType = this.determineFileType(file);
        const contentRules = this.getDefaultRules(fileType);
        
        expectedFiles.push({
          path: relativePath,
          type: fileType as any,
          required: true,
          contentRules
        });
      }

      const expectedOutput: ExpectedOutput = {
        testName,
        repositoryType: context.repositoryType,
        expectedFiles,
        validationRules: []
      };

      return Success(expectedOutput);
    } catch (error) {
      return Failure(`Failed to generate expected output: ${error.message}`);
    }
  }

  private async scanDirectory(dirPath: string): Promise<string[]> {
    const files: string[] = [];
    
    try {
      const entries = await fs.readdir(dirPath, { withFileTypes: true });
      
      for (const entry of entries) {
        const fullPath = path.join(dirPath, entry.name);
        
        if (entry.isDirectory()) {
          const subFiles = await this.scanDirectory(fullPath);
          files.push(...subFiles);
        } else {
          files.push(fullPath);
        }
      }
    } catch (error) {
      // Directory doesn't exist or can't be read
    }
    
    return files;
  }

  private determineFileType(filePath: string): string {
    const ext = path.extname(filePath).toLowerCase();
    const basename = path.basename(filePath).toLowerCase();
    
    if (basename === 'dockerfile' || basename.startsWith('dockerfile.')) {
      return 'dockerfile';
    }
    if (basename === 'docker-compose.yml' || basename === 'docker-compose.yaml') {
      return 'compose';
    }
    if (ext === '.json') {
      return 'json';
    }
    if (ext === '.yml' || ext === '.yaml') {
      // Check if it's a Kubernetes manifest by looking for common fields
      return 'k8s';
    }
    
    return 'text';
  }
}