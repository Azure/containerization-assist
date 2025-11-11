/**
 * Template Merger Utility
 *
 * Provides utilities for merging policy-driven templates into tool plans.
 * Templates can inject organizational standards (CA certs, observability agents,
 * sidecars, etc.) into generated Dockerfiles and Kubernetes manifests.
 */

import type {
  DockerfileTemplateSnippet,
  K8sTemplateComponent,
  TemplateAdditions,
} from '@/config/policy-generation-config';

/**
 * Recommendation with optional policy-driven flag
 */
export interface PolicyDrivenRecommendation {
  id: string;
  category: string;
  recommendation: string;
  tags?: string[];
  matchScore: number;
  policyDriven?: boolean;
}

/**
 * Filter Dockerfile templates based on conditions
 */
function filterDockerfileTemplates(
  templates: DockerfileTemplateSnippet[],
  context: {
    language?: string | undefined;
    environment?: string | undefined;
    framework?: string | undefined;
  },
): DockerfileTemplateSnippet[] {
  return templates.filter((template) => {
    if (!template.conditions) {
      return true;
    }

    const { languages, environments, frameworks } = template.conditions;

    if (languages && languages.length > 0 && context.language) {
      if (!languages.includes(context.language)) {
        return false;
      }
    }

    if (environments && environments.length > 0 && context.environment) {
      if (!environments.includes(context.environment)) {
        return false;
      }
    }

    if (frameworks && frameworks.length > 0 && context.framework) {
      if (!frameworks.includes(context.framework)) {
        return false;
      }
    }

    return true;
  });
}

/**
 * Filter K8s templates based on conditions
 */
function filterK8sTemplates(
  templates: K8sTemplateComponent[],
  context: {
    language?: string | undefined;
    environment?: string | undefined;
    framework?: string | undefined;
  },
): K8sTemplateComponent[] {
  return templates.filter((template) => {
    if (!template.conditions) {
      return true;
    }

    const { languages, environments, frameworks } = template.conditions;

    if (languages && languages.length > 0 && context.language) {
      if (!languages.includes(context.language)) {
        return false;
      }
    }

    if (environments && environments.length > 0 && context.environment) {
      if (!environments.includes(context.environment)) {
        return false;
      }
    }

    if (frameworks && frameworks.length > 0 && context.framework) {
      if (!frameworks.includes(context.framework)) {
        return false;
      }
    }

    return true;
  });
}

/**
 * Convert Dockerfile template snippets to recommendations
 */
export function dockerfileTemplatesToRecommendations(
  templates: DockerfileTemplateSnippet[],
  context: {
    language?: string | undefined;
    environment?: string | undefined;
    framework?: string | undefined;
  },
): PolicyDrivenRecommendation[] {
  // Filter templates by conditions
  const applicableTemplates = filterDockerfileTemplates(templates, context);

  // Sort by priority (higher priority first) and section
  const sortedTemplates = applicableTemplates.sort((a, b) => {
    const priorityA = a.priority ?? 0;
    const priorityB = b.priority ?? 0;
    return priorityB - priorityA;
  });

  // Convert to recommendations
  return sortedTemplates.map((template) => ({
    id: template.id,
    category: `dockerfile-template-${template.section}`,
    recommendation: `${template.description}\n\n${template.content}`,
    tags: ['policy-template', `template-${template.section}`],
    matchScore: 100, // High score to ensure templates appear prominently
    policyDriven: true,
  }));
}

/**
 * Convert K8s template components to recommendations
 */
export function k8sTemplatesToRecommendations(
  templates: K8sTemplateComponent[],
  context: {
    language?: string | undefined;
    environment?: string | undefined;
    framework?: string | undefined;
  },
): PolicyDrivenRecommendation[] {
  // Filter templates by conditions
  const applicableTemplates = filterK8sTemplates(templates, context);

  // Sort by priority (higher priority first)
  const sortedTemplates = applicableTemplates.sort((a, b) => {
    const priorityA = a.priority ?? 0;
    const priorityB = b.priority ?? 0;
    return priorityB - priorityA;
  });

  // Convert to recommendations
  return sortedTemplates.map((template) => ({
    id: template.id,
    category: `k8s-template-${template.type}`,
    recommendation: `${template.description}\n\nComponent Type: ${template.type}\nSpecification: ${JSON.stringify(template.spec, null, 2)}`,
    tags: ['policy-template', `template-${template.type}`],
    matchScore: 100, // High score to ensure templates appear prominently
    policyDriven: true,
  }));
}

/**
 * Merge template recommendations into existing recommendations
 * Template recommendations are inserted at the beginning of each category
 */
export function mergeTemplateRecommendations<T extends { id: string; policyDriven?: boolean }>(
  existing: T[],
  templates: PolicyDrivenRecommendation[],
): (T | PolicyDrivenRecommendation)[] {
  // Mark existing recommendations as not policy-driven if not already marked
  const markedExisting = existing.map((rec) => ({
    ...rec,
    policyDriven: rec.policyDriven ?? false,
  }));

  // Templates go first (they're high priority)
  return [...templates, ...markedExisting];
}

/**
 * Merge all template additions into plan recommendations
 */
export function mergeTemplatesIntoPlan<
  TPlan extends {
    recommendations: Record<string, unknown[] | unknown>;
  }
>(
  plan: TPlan,
  templateAdditions: TemplateAdditions | null | undefined,
  context: {
    language?: string | undefined;
    environment?: string | undefined;
    framework?: string | undefined;
  },
): TPlan {
  if (!templateAdditions) {
    return plan;
  }

  const updatedPlan = { ...plan };

  // Merge Dockerfile templates if present
  if (templateAdditions.dockerfile && templateAdditions.dockerfile.length > 0) {
    const dockerfileTemplateRecs = dockerfileTemplatesToRecommendations(
      templateAdditions.dockerfile,
      context,
    );

    // Merge into appropriate recommendation categories
    // For Dockerfile plans, merge into securityConsiderations and bestPractices
    if ('securityConsiderations' in updatedPlan.recommendations) {
      const securityTemplates = dockerfileTemplateRecs.filter((rec) =>
        rec.category.includes('security'),
      );
      const securityRecs = updatedPlan.recommendations.securityConsiderations;
      if (Array.isArray(securityRecs)) {
        updatedPlan.recommendations = {
          ...updatedPlan.recommendations,
          securityConsiderations: mergeTemplateRecommendations(
            securityRecs as { id: string; policyDriven?: boolean }[],
            securityTemplates,
          ),
        };
      }
    }

    if ('bestPractices' in updatedPlan.recommendations) {
      const otherTemplates = dockerfileTemplateRecs.filter(
        (rec) => !rec.category.includes('security'),
      );
      const bestPracticesRecs = updatedPlan.recommendations.bestPractices;
      if (Array.isArray(bestPracticesRecs)) {
        updatedPlan.recommendations = {
          ...updatedPlan.recommendations,
          bestPractices: mergeTemplateRecommendations(
            bestPracticesRecs as { id: string; policyDriven?: boolean }[],
            otherTemplates,
          ),
        };
      }
    }
  }

  // Merge K8s templates if present
  if (templateAdditions.kubernetes && templateAdditions.kubernetes.length > 0) {
    const k8sTemplateRecs = k8sTemplatesToRecommendations(
      templateAdditions.kubernetes,
      context,
    );

    // Merge into appropriate recommendation categories
    // For K8s plans, merge into securityConsiderations and bestPractices
    if ('securityConsiderations' in updatedPlan.recommendations) {
      const securityTemplates = k8sTemplateRecs.filter(
        (rec) => rec.category.includes('security') || rec.recommendation.toLowerCase().includes('security'),
      );
      const securityRecs = updatedPlan.recommendations.securityConsiderations;
      if (Array.isArray(securityRecs)) {
        updatedPlan.recommendations = {
          ...updatedPlan.recommendations,
          securityConsiderations: mergeTemplateRecommendations(
            securityRecs as { id: string; policyDriven?: boolean }[],
            securityTemplates,
          ),
        };
      }
    }

    if ('bestPractices' in updatedPlan.recommendations) {
      const otherTemplates = k8sTemplateRecs.filter(
        (rec) => !rec.category.includes('security') && !rec.recommendation.toLowerCase().includes('security'),
      );
      const bestPracticesRecs = updatedPlan.recommendations.bestPractices;
      if (Array.isArray(bestPracticesRecs)) {
        updatedPlan.recommendations = {
          ...updatedPlan.recommendations,
          bestPractices: mergeTemplateRecommendations(
            bestPracticesRecs as { id: string; policyDriven?: boolean }[],
            otherTemplates,
          ),
        };
      }
    }
  }

  return updatedPlan;
}

/**
 * Extract template additions from generation config
 */
export function extractTemplateAdditions(
  generationConfig: { templates?: TemplateAdditions } | null | undefined,
): TemplateAdditions | null {
  if (!generationConfig?.templates) {
    return null;
  }

  return generationConfig.templates;
}
