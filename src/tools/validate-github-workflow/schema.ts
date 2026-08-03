/**
 * Schema definition for the validate-github-workflow tool.
 *
 * Reuses the shared validation types (ValidationResult / ValidationReport) instead
 * of inventing new ones — the same convention fix-dockerfile follows.
 */

import { z } from 'zod';
import { repositoryPath, type ToolNextAction } from '../shared/schemas';
import type { ValidationReport, ValidationResult } from '@/validation/core-types';

/** The four independently toggleable validation layers. */
export const WORKFLOW_LAYERS = ['yaml', 'schema', 'refs', 'semantic'] as const;
export type WorkflowLayer = (typeof WORKFLOW_LAYERS)[number];

export const validateGithubWorkflowSchema = z.object({
  repositoryPath: repositoryPath.describe(
    'Repository root (automatically normalized to forward slashes). The workflow file is read from <repositoryPath>/.github/workflows/ unless workflowContent is supplied.',
  ),

  workflowFileName: z
    .string()
    .optional()
    .default('deploy.yml')
    .describe(
      'File under .github/workflows/ to validate. Basename-sanitized so it cannot escape the workflows directory. Defaults to "deploy.yml".',
    ),

  workflowContent: z
    .string()
    .optional()
    .describe(
      'Raw workflow YAML. When provided, takes precedence over reading from disk (enables pre-commit validation).',
    ),

  manifestFormat: z
    .enum(['k8s', 'helm', 'kustomize'])
    .optional()
    .default('k8s')
    .describe(
      'Manifest format the workflow deploys. Tailors Layer-4 deploy-step expectations (helm/kustomize expect an azure/k8s-bake step).',
    ),

  layers: z
    .array(z.enum(WORKFLOW_LAYERS))
    .optional()
    .describe(
      'Subset of validation layers to run. Defaults to all four (yaml, schema, refs, semantic).',
    ),

  language: z
    .string()
    .optional()
    .describe('Primary programming language from analyze-repo (knowledge query hint).'),

  framework: z.string().optional().describe('Framework from analyze-repo (knowledge query hint).'),
});

export type ValidateGithubWorkflowParams = z.infer<typeof validateGithubWorkflowSchema>;

/**
 * A single workflow validation finding.
 *
 * Extends the shared {@link ValidationResult} (mirroring fix-dockerfile's
 * ValidationIssue) with two workflow-specific fields. The `layer` is also encoded
 * in the `ruleId` prefix (e.g. `refs/sha-pin`).
 */
export interface WorkflowValidationIssue extends ValidationResult {
  /** Which validation layer produced this finding. */
  layer?: WorkflowLayer;
  /** The offending `owner/repo@ref` when this finding concerns a `uses:` reference. */
  actionRef?: string;
  /**
   * 1-based line the finding points at, when it can be located in the source.
   *
   * Splits *where* from *what*: this is the position, while `metadata.location` describes the
   * subject (`job "deploy"`, `uses: actions/checkout`). Consumers join them as
   * `line 27, job "deploy"`, so `location` must never contain a line number itself. Absent for
   * findings about *missing* structure, which has no position — e.g. a workflow with no `on:`
   * block at all.
   */
  line?: number;
}

/**
 * The reused {@link ValidationReport}, narrowed so `results` carries the workflow-specific
 * finding type.
 *
 * `ValidationReport.results` is `ValidationResult[]`, which would force every consumer to cast
 * in order to read `line` or `layer` — and a cast would keep compiling if those fields were
 * ever dropped from the contract. Narrowing here keeps the shared report shape (score, grade,
 * tallies) while making the extra fields visible to the type system.
 */
export interface WorkflowValidationReport extends Omit<ValidationReport, 'results'> {
  results: WorkflowValidationIssue[];
}

/**
 * Output plan returned by the validate-github-workflow tool.
 *
 * Wraps the reused {@link ValidationReport} with the file path, an action-oriented
 * summary, and CA attribution — mirroring how DockerfileFixPlan wraps its findings.
 */
export interface WorkflowValidationPlan {
  /** The findings + tallies (reused ValidationReport shape). */
  report: WorkflowValidationReport;

  /** Validated path, or '<inline>' when workflowContent was supplied. */
  filePath: string;

  /** Human-readable, action-oriented summary. */
  summary: string;

  /** Attribution annotations tracking which CA version produced this report. */
  attributionLabels: {
    annotations: Record<string, string>;
  };

  /** Confidence (0-1) in the knowledge-derived Layer-4 recommendations. */
  confidence?: number;

  /**
   * Present ONLY when validation fails (report.errors > 0). Directs the client LLM
   * to fix the workflow and re-run validate-github-workflow. Omitted on a clean pass.
   */
  nextAction?: ToolNextAction;
}
