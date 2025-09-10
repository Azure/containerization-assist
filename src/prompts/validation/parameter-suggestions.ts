import type { PromptFile } from '../../core/prompts/static-loader';

export const parameterSuggestions: PromptFile = {
  metadata: {
    name: 'parameter-suggestions',
    category: 'validation',
    description: 'Generate parameter suggestions with AI assistance',
    version: '1.0',
    parameters: [
      {
        name: 'toolName',
        type: 'string',
        required: true,
        description: 'Name of the tool needing parameters',
      },
      {
        name: 'partialParameters',
        type: 'string',
        required: true,
        description: 'Existing partial parameters (JSON format)',
      },
      {
        name: 'context',
        type: 'string',
        required: false,
        description: 'Parameter suggestion context (JSON format)',
      },
      {
        name: 'existingParams',
        type: 'string',
        required: false,
        description: 'List of existing parameter names',
      },
      {
        name: 'targetType',
        type: 'string',
        required: false,
        description: 'Target type (dockerfile, kubernetes, analysis, general)',
      },
      {
        name: 'framework',
        type: 'string',
        required: false,
        description: 'Framework for framework-specific optimizations',
      },
    ],
  },
  template: `Generate parameter suggestions for "{{toolName}}".

Current parameters: {{partialParameters}}
{{#context}}Context: {{context}}{{/context}}
{{#existingParams}}Existing parameter keys: {{existingParams}}{{/existingParams}}

Suggest:
- Missing required parameters
- Optimal parameter values
- Performance improvements
- Security enhancements
- Best practice configurations
{{#targetType}}
- {{targetType}}-specific optimizations
{{/targetType}}
{{#framework}}
- {{framework}} framework optimizations
{{/framework}}

Provide practical suggestions with explanations.
`,
};
