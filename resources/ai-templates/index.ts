import { BASE_IMAGE_RESOLUTION_TEMPLATE } from './base-image-resolution';
import { DOCKERFILE_FIX_TEMPLATE } from './dockerfile-fix';
import { DOCKERFILE_GENERATION_TEMPLATE } from './dockerfile-generation';
import { DOTNET_ANALYSIS_TEMPLATE } from './dotnet-analysis';
import { ERROR_ANALYSIS_TEMPLATE } from './error-analysis';
import { JSON_REPAIR_TEMPLATE } from './json-repair';
import { JVM_ANALYSIS_TEMPLATE } from './jvm-analysis';
import { K8S_FIX_TEMPLATE } from './k8s-fix';
import { K8S_GENERATION_TEMPLATE } from './k8s-generation';
import { OPTIMIZATION_SUGGESTION_TEMPLATE } from './optimization-suggestion';
import { REPOSITORY_ANALYSIS_TEMPLATE } from './repository-analysis';

// Map of template IDs to templates for easy lookup
export const AI_TEMPLATES = {
    'base-image-resolution': BASE_IMAGE_RESOLUTION_TEMPLATE,
    'dockerfile-fix': DOCKERFILE_FIX_TEMPLATE,
    'dockerfile-generation': DOCKERFILE_GENERATION_TEMPLATE,
    'dotnet-analysis': DOTNET_ANALYSIS_TEMPLATE,
    'error-analysis': ERROR_ANALYSIS_TEMPLATE,
    'json-repair': JSON_REPAIR_TEMPLATE,
    'jvm-analysis': JVM_ANALYSIS_TEMPLATE,
    'k8s-fix': K8S_FIX_TEMPLATE,
    'k8s-generation': K8S_GENERATION_TEMPLATE,
    'optimization-suggestion': OPTIMIZATION_SUGGESTION_TEMPLATE,
    'repository-analysis': REPOSITORY_ANALYSIS_TEMPLATE,
} as const;

// Type for template IDs
export type TemplateId = keyof typeof AI_TEMPLATES;

// Get template by ID
export function getTemplate(id: TemplateId) {
    return AI_TEMPLATES[id];
}

// Get all template IDs
export function getTemplateIds(): TemplateId[] {
    return Object.keys(AI_TEMPLATES) as TemplateId[];
}

// Get all templates
export function getAllTemplates() {
    return Object.values(AI_TEMPLATES);
}