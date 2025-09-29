import Ajv, { type ErrorObject } from 'ajv';
import addFormats from 'ajv-formats';
import { parseAllDocuments } from 'yaml';
import type { ValidationReport, ValidationResult, ValidationSeverity } from './core-types';

export interface K8sSchemaOptions {
  k8sVersion?: string;
  strict?: boolean;
  allowUnknownResources?: boolean;
}

export class K8sSchemaValidator {
  private ajv: Ajv;
  private schemaCache = new Map<string, Record<string, unknown>>();
  private schemasLoaded = false;

  constructor(private options: K8sSchemaOptions = {}) {
    this.ajv = new Ajv({
      allErrors: true,
      strict: options.strict ?? false,
      verbose: false,
    });
    addFormats(this.ajv);
  }

  private async ensureSchemasLoaded(): Promise<void> {
    if (this.schemasLoaded) return;

    // Load fallback schemas since kubernetes-json-schema doesn't exist
    this.loadFallbackSchemas();
    this.schemasLoaded = true;
  }

  private loadFallbackSchemas(): void {
    // Minimal schemas for most common resources
    const commonSchemas = [
      {
        gvk: { group: 'apps', version: 'v1', kind: 'Deployment' },
        schema: {
          type: 'object',
          required: ['apiVersion', 'kind', 'metadata', 'spec'],
          properties: {
            apiVersion: { const: 'apps/v1' },
            kind: { const: 'Deployment' },
            metadata: {
              type: 'object',
              required: ['name'],
              properties: {
                name: { type: 'string', minLength: 1 },
                namespace: { type: 'string' },
                labels: { type: 'object' },
                annotations: { type: 'object' },
              },
            },
            spec: {
              type: 'object',
              required: ['selector', 'template'],
              properties: {
                replicas: { type: 'number', minimum: 0 },
                selector: {
                  type: 'object',
                  required: ['matchLabels'],
                  properties: {
                    matchLabels: { type: 'object' },
                  },
                },
                template: {
                  type: 'object',
                  required: ['metadata', 'spec'],
                  properties: {
                    metadata: {
                      type: 'object',
                      required: ['labels'],
                      properties: {
                        labels: { type: 'object' },
                      },
                    },
                    spec: {
                      type: 'object',
                      required: ['containers'],
                      properties: {
                        containers: {
                          type: 'array',
                          minItems: 1,
                          items: {
                            type: 'object',
                            required: ['name', 'image'],
                            properties: {
                              name: { type: 'string', minLength: 1 },
                              image: { type: 'string', minLength: 1 },
                              ports: { type: 'array' },
                              resources: { type: 'object' },
                              securityContext: { type: 'object' },
                            },
                          },
                        },
                        securityContext: { type: 'object' },
                      },
                    },
                  },
                },
              },
            },
          },
        },
      },
      {
        gvk: { group: '', version: 'v1', kind: 'Service' },
        schema: {
          type: 'object',
          required: ['apiVersion', 'kind', 'metadata', 'spec'],
          properties: {
            apiVersion: { const: 'v1' },
            kind: { const: 'Service' },
            metadata: {
              type: 'object',
              required: ['name'],
              properties: {
                name: { type: 'string', minLength: 1 },
                namespace: { type: 'string' },
                labels: { type: 'object' },
              },
            },
            spec: {
              type: 'object',
              properties: {
                selector: { type: 'object' },
                ports: {
                  type: 'array',
                  minItems: 1,
                  items: {
                    type: 'object',
                    required: ['port'],
                    properties: {
                      port: { type: 'number', minimum: 1, maximum: 65535 },
                      targetPort: { oneOf: [{ type: 'number' }, { type: 'string' }] },
                      protocol: { enum: ['TCP', 'UDP', 'SCTP'] },
                      name: { type: 'string' },
                    },
                  },
                },
                type: { enum: ['ClusterIP', 'NodePort', 'LoadBalancer', 'ExternalName'] },
              },
            },
          },
        },
      },
      {
        gvk: { group: '', version: 'v1', kind: 'ConfigMap' },
        schema: {
          type: 'object',
          required: ['apiVersion', 'kind', 'metadata'],
          properties: {
            apiVersion: { const: 'v1' },
            kind: { const: 'ConfigMap' },
            metadata: {
              type: 'object',
              required: ['name'],
              properties: {
                name: { type: 'string', minLength: 1 },
                namespace: { type: 'string' },
              },
            },
            data: { type: 'object' },
            binaryData: { type: 'object' },
          },
        },
      },
      {
        gvk: { group: '', version: 'v1', kind: 'Secret' },
        schema: {
          type: 'object',
          required: ['apiVersion', 'kind', 'metadata'],
          properties: {
            apiVersion: { const: 'v1' },
            kind: { const: 'Secret' },
            metadata: {
              type: 'object',
              required: ['name'],
              properties: {
                name: { type: 'string', minLength: 1 },
                namespace: { type: 'string' },
              },
            },
            type: { type: 'string' },
            data: { type: 'object' },
            stringData: { type: 'object' },
          },
        },
      },
    ];

    for (const { gvk, schema } of commonSchemas) {
      const key = this.gvkToKey(gvk);
      this.schemaCache.set(key, schema);
      this.ajv.addSchema(schema, key);
    }
  }

  private gvkToKey(gvk: { group: string; version: string; kind: string }): string {
    const group = gvk.group || 'core';
    return `${group}/${gvk.version}/${gvk.kind}`;
  }

  private extractGVK(
    obj: Record<string, unknown>,
  ): { group: string; version: string; kind: string } | null {
    if (!obj?.apiVersion || !obj?.kind) return null;

    const apiVersion = obj.apiVersion as string;
    const kind = obj.kind as string;

    const parts = apiVersion.includes('/') ? apiVersion.split('/', 2) : ['', apiVersion];
    const group = parts[0] || '';
    const version = parts[1] || apiVersion;

    return { group, version, kind };
  }

  async validate(yamlContent: string): Promise<ValidationReport> {
    await this.ensureSchemasLoaded();

    const results: ValidationResult[] = [];

    try {
      const docs = parseAllDocuments(yamlContent);

      docs.forEach((doc, docIndex) => {
        const obj = doc.toJS();
        if (!obj || typeof obj !== 'object') {
          results.push({
            ruleId: 'schema-invalid-doc',
            isValid: false,
            passed: false,
            errors: [`Document ${docIndex + 1} is not a valid object`],
            warnings: [],
            message: 'Invalid document structure',
            metadata: {
              severity: 'error' as ValidationSeverity,
              location: `document-${docIndex + 1}`,
            },
          });
          return;
        }

        const gvk = this.extractGVK(obj);
        if (!gvk) {
          results.push({
            ruleId: 'schema-missing-gvk',
            isValid: false,
            passed: false,
            errors: [`Document ${docIndex + 1}: Missing apiVersion or kind`],
            warnings: [],
            message: 'Resource must have apiVersion and kind',
            metadata: {
              severity: 'error' as ValidationSeverity,
              location: `document-${docIndex + 1}`,
            },
          });
          return;
        }

        const schemaKey = this.gvkToKey(gvk);
        const hasSchema = this.schemaCache.has(schemaKey);

        if (!hasSchema) {
          const severity = this.options.allowUnknownResources
            ? ('info' as ValidationSeverity)
            : ('warning' as ValidationSeverity);

          results.push({
            ruleId: `schema-unknown-${gvk.kind}`,
            isValid: this.options.allowUnknownResources ?? true,
            passed: this.options.allowUnknownResources ?? true,
            errors: this.options.allowUnknownResources
              ? []
              : [`Unknown resource: ${this.gvkToString(gvk)}`],
            warnings: this.options.allowUnknownResources
              ? [`Unknown resource: ${this.gvkToString(gvk)}`]
              : [],
            message: `Cannot validate ${gvk.kind} (possibly a CRD or newer resource)`,
            metadata: {
              severity,
              location: `${gvk.kind}/${obj.metadata?.name || 'unnamed'}`,
            },
          });
          return;
        }

        // Validate against schema
        const isValid = this.ajv.validate(schemaKey, obj);
        if (!isValid && this.ajv.errors) {
          this.ajv.errors.forEach((error: ErrorObject) => {
            const instancePath = error.instancePath || '/';
            const resourceName = `${gvk.kind}/${obj.metadata?.name || 'unnamed'}`;

            results.push({
              ruleId: `schema-${gvk.kind}-${error.keyword}`,
              isValid: false,
              passed: false,
              errors: [`${instancePath} ${error.message}`],
              warnings: [],
              message: `${this.gvkToString(gvk)}: ${instancePath} ${error.message}`,
              metadata: {
                severity: 'error' as ValidationSeverity,
                location: resourceName,
              },
            });
          });
        } else if (isValid) {
          // Add a passed result for successfully validated resources
          results.push({
            ruleId: `schema-${gvk.kind}-valid`,
            isValid: true,
            passed: true,
            errors: [],
            warnings: [],
            message: `${this.gvkToString(gvk)} passed schema validation`,
            metadata: {
              severity: 'info' as ValidationSeverity,
              location: `${gvk.kind}/${obj.metadata?.name || 'unnamed'}`,
            },
          });
        }
      });
    } catch (error) {
      results.push({
        ruleId: 'schema-parse-error',
        isValid: false,
        passed: false,
        errors: [`Failed to parse YAML: ${error}`],
        warnings: [],
        message: 'YAML parsing failed',
        metadata: { severity: 'error' as ValidationSeverity },
      });
    }

    return this.createReport(results);
  }

  private gvkToString(gvk: { group: string; version: string; kind: string }): string {
    const group = gvk.group || 'core';
    return `${group}/${gvk.version}/${gvk.kind}`;
  }

  private createReport(results: ValidationResult[]): ValidationReport {
    const errors = results.filter((r) => r.metadata?.severity === 'error').length;
    const warnings = results.filter((r) => r.metadata?.severity === 'warning').length;
    const info = results.filter((r) => r.metadata?.severity === 'info').length;

    // Schema validation errors are serious - weight them heavily
    const score = Math.max(0, 100 - (errors * 25 + warnings * 8 + info * 2));
    const grade =
      score >= 90 ? 'A' : score >= 75 ? 'B' : score >= 60 ? 'C' : score >= 45 ? 'D' : 'F';

    return {
      results,
      score,
      grade,
      passed: results.filter((r) => r.passed).length,
      failed: results.filter((r) => !r.passed).length,
      errors,
      warnings,
      info,
      timestamp: new Date().toISOString(),
    };
  }
}

export function createK8sSchemaValidator(options?: K8sSchemaOptions): K8sSchemaValidator {
  return new K8sSchemaValidator(options);
}
