/**
 * AI Template Constants
 *
 * TypeScript constants converted from JSON templates in resources/ai-templates/
 * These templates provide structured prompts for various AI-powered operations.
 */

import type { PromptFile, ParameterSpec } from '../prompts/static-loader';
import type { AITemplate, TemplateVariable } from './types';

// Import all template constants
import { BASE_IMAGE_RESOLUTION } from './base-image-resolution';
import { DOCKERFILE_GENERATION } from './dockerfile-generation';
import { DOCKERFILE_FIX } from './dockerfile-fix';
import { K8S_GENERATION } from './k8s-generation';
import { REPOSITORY_ANALYSIS } from './repository-analysis';
import { ERROR_ANALYSIS } from './error-analysis';
import { K8S_FIX } from './k8s-fix';
import { JSON_REPAIR } from './json-repair';
import { DOTNET_ANALYSIS } from './dotnet-analysis';
import { JVM_ANALYSIS } from './jvm-analysis';
import { OPTIMIZATION_SUGGESTION } from './optimization-suggestion';

// Re-export types
export type { AITemplate, TemplateVariable } from './types';

// Conversion functions from AI templates to PromptFile format
function convertVariablesToParameters(variables?: TemplateVariable[]): ParameterSpec[] {
  if (!variables) return [];

  return variables.map((variable) => {
    const parameter: ParameterSpec = {
      name: variable.name,
      type: 'string', // AI templates primarily use strings
      required: variable.required,
      description: variable.description,
    };

    if (variable.default !== undefined) {
      parameter.default = variable.default;
    }

    return parameter;
  });
}

function convertAITemplateToPromptFile(template: AITemplate): PromptFile {
  // Extract category from ID (e.g., "dockerfile-generation" -> "containerization")
  const categoryMap: Record<string, string> = {
    'base-image-resolution': 'analysis',
    'dockerfile-generation': 'containerization',
    'dockerfile-fix': 'containerization',
    'k8s-generation': 'orchestration',
    'k8s-fix': 'orchestration',
    'repository-analysis': 'analysis',
    'error-analysis': 'debugging',
    'json-repair': 'utility',
    'dotnet-analysis': 'analysis',
    'jvm-analysis': 'analysis',
    'optimization-suggestion': 'optimization',
  };

  const category = categoryMap[template.id] || 'general';

  // Combine system and user prompts
  const combinedTemplate = `${template.system}\n\n${template.user}`;

  return {
    metadata: {
      name: template.id,
      category,
      description: template.description,
      version: template.version,
      parameters: convertVariablesToParameters(template.variables),
    },
    template: combinedTemplate,
  };
}

// Export converted prompt files for integration with existing prompt system
export const AI_TEMPLATE_PROMPT_FILES: PromptFile[] = [
  convertAITemplateToPromptFile(BASE_IMAGE_RESOLUTION),
  convertAITemplateToPromptFile(DOCKERFILE_GENERATION),
  convertAITemplateToPromptFile(K8S_GENERATION),
  convertAITemplateToPromptFile(DOCKERFILE_FIX),
  convertAITemplateToPromptFile(REPOSITORY_ANALYSIS),
  convertAITemplateToPromptFile(K8S_FIX),
  convertAITemplateToPromptFile(ERROR_ANALYSIS),
  convertAITemplateToPromptFile(JSON_REPAIR),
  convertAITemplateToPromptFile(DOTNET_ANALYSIS),
  convertAITemplateToPromptFile(JVM_ANALYSIS),
  convertAITemplateToPromptFile(OPTIMIZATION_SUGGESTION),
];
