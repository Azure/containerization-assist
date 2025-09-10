import type { AITemplate } from './types';

export const ERROR_ANALYSIS: AITemplate = {
  id: 'error-analysis',
  name: 'Universal Error Analysis',
  description: 'Analyze and provide solutions for containerization errors across all languages',
  version: '2.0.0',
  system:
    'You are an expert in troubleshooting containerization issues across ALL programming languages and frameworks.\nAnalyze errors and provide actionable solutions with root cause analysis.\n\nFocus on:\n1. Docker build failures\n2. Runtime errors\n3. Network and port issues\n4. Dependency problems\n5. Security and permission issues\n6. Resource constraints\n',
  user: 'Analyze this containerization error and provide solutions:\n\n**Error Context:**\n- Language: {{language}}\n- Framework: {{framework}}\n- Build System: {{buildSystem}}\n- Error Type: {{errorType}}\n\n**Error Details:**\n{{errorMessage}}\n\n**Build Context:**\n{{buildContext}}\n\n**Requirements:**\n1. Identify the root cause\n2. Provide step-by-step solution\n3. Suggest preventive measures\n4. Include {{language}}-specific best practices\n\nReturn a structured analysis with clear recommendations.\n',
  outputFormat: 'text',
  variables: [
    {
      name: 'language',
      description: 'Programming language',
      required: true,
    },
    {
      name: 'framework',
      description: 'Application framework',
      required: false,
    },
    {
      name: 'buildSystem',
      description: 'Build system being used',
      required: false,
    },
    {
      name: 'errorType',
      description: 'Type of error (build, runtime, network, etc.)',
      required: false,
      default: 'build',
    },
    {
      name: 'errorMessage',
      description: 'The actual error message',
      required: true,
    },
    {
      name: 'buildContext',
      description: 'Additional context about the build',
      required: false,
      default: 'Standard containerization process',
    },
  ],
  tags: ['error-analysis', 'troubleshooting', 'universal'],
} as const;
