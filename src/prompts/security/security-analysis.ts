import type { PromptFile } from '../../core/prompts/static-loader';

export const securityAnalysis: PromptFile = {
  metadata: {
    name: 'security-analysis',
    category: 'security',
    description: 'Analyze container configuration for security vulnerabilities',
    version: '1.0',
    parameters: [
      {
        name: 'configType',
        type: 'string',
        required: true,
        description: 'Type of configuration (dockerfile, k8s-manifest)',
      },
      {
        name: 'content',
        type: 'string',
        required: false,
        description: 'Configuration content to analyze',
      },
      {
        name: 'complianceStandard',
        type: 'string',
        required: false,
        description: 'Compliance standard to check against (CIS, NIST, SOC2)',
      },
      {
        name: 'environment',
        type: 'string',
        required: false,
        description: 'Target environment for production-specific checks',
      },
    ],
  },
  template: `Analyze {{configType}} configuration for security vulnerabilities.
{{#complianceStandard}}Apply {{complianceStandard}} compliance checks.{{/complianceStandard}}
{{#content}}

Configuration to analyze:
{{content}}
{{/content}}

Analysis should include:
- Vulnerability assessment
- Best practice compliance
- Risk evaluation
- Remediation recommendations
{{#environment}}
{{#if (eq environment "production")}}
- Production-specific security checks
{{/if}}
{{/environment}}
`,
};
