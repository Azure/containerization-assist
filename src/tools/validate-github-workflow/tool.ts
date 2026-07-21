/**
 * Validate GitHub Workflow Tool
 *
 * Deterministically validates a generated GitHub Actions deploy workflow across four
 * layers — YAML validity, structural schema, `uses:` SHA-pinning, and CA semantic
 * invariants — and returns the reused ValidationReport (wrapped in a
 * WorkflowValidationPlan). On failure it emits a `fix-files` nextAction enumerating
 * every required issue so the client LLM can edit and re-validate.
 *
 * Uses the knowledge-tool-pattern; makes NO AI calls. The knowledge base supplies the
 * Layer-4 recommendation text (shared with generate-github-workflow via the same pack).
 *
 * ⚠️  Do NOT use import.meta in this file — the CJS build forbids it.
 */

import { type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/core/context';
import { tool } from '@/types/tool';
import { CATEGORY } from '@/knowledge/types';
import { ValidationSeverity, type ValidationGrade, type ValidationReport } from '@/validation/core-types';
import { PACKAGE_VERSION } from '@/lib/package-version';
import { pluralize } from '@/lib/summary-helpers';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import { TOOL_NAME } from '../shared/toolDefinition';
import {
  validateGithubWorkflowSchema,
  type ValidateGithubWorkflowParams,
  type WorkflowLayer,
  type WorkflowValidationIssue,
  type WorkflowValidationPlan,
} from './schema';
import { validateGithubWorkflowToolDefinition } from './types';
import { makeIssue, resolveWorkflowSource, workflowRelativePath } from './checks/helpers';
import { parseWorkflow, checkYaml } from './checks/yaml-check';
import { checkSchema } from './checks/schema-check';
import { checkRefs } from './checks/refs-check';
import { checkSemantic } from './checks/semantic-check';

const { name } = validateGithubWorkflowToolDefinition;

// Score penalties per finding severity (100 = clean).
const PENALTY = { ERROR: 25, WARNING: 8, INFO: 2 } as const;

type WorkflowCategory = 'auth' | 'build' | 'deploy' | 'bestPractices';

interface WorkflowRules {
  findings: WorkflowValidationIssue[];
  filePath: string;
}

// ─── Knowledge-tool pattern ───────────────────────────────────────────────────

const runPattern = createKnowledgeTool<
  ValidateGithubWorkflowParams,
  WorkflowValidationPlan,
  WorkflowCategory,
  WorkflowRules
>({
  name,

  query: {
    topic: TOPICS.GITHUB_WORKFLOW,
    category: CATEGORY.CICD,
    maxChars: 6000,
    maxSnippets: 15,
    extractFilters: (input) => ({
      language: input.language,
      framework: input.framework,
    }),
  },

  categorization: {
    categoryNames: ['auth', 'build', 'deploy', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<WorkflowCategory>({
      auth: (s) => Boolean(s.tags?.includes('azure-oidc') || s.tags?.includes('azure-login')),
      build: (s) =>
        Boolean(
          s.tags?.includes('docker-build') ||
            s.tags?.includes('acr') ||
            s.tags?.includes('registry'),
        ),
      deploy: (s) =>
        Boolean(
          s.tags?.includes('aks') ||
            s.tags?.includes('kubectl') ||
            s.tags?.includes('k8s-deploy') ||
            s.tags?.includes('k8s-bake'),
        ),
      bestPractices: () => true,
    }),
  },

  rules: {
    applyRules: async (input, knowledge, ctx): Promise<WorkflowRules> => {
      const selected = new Set<WorkflowLayer>(
        input.layers && input.layers.length > 0
          ? input.layers
          : ['yaml', 'schema', 'refs', 'semantic'],
      );

      const source = await resolveWorkflowSource(input, ctx);
      if (!source.ok) {
        return {
          filePath: input.workflowContent ? '<inline>' : workflowRelativePath(input),
          findings: [
            makeIssue({
              layer: 'yaml',
              ruleId: 'source/not-found',
              severity: 'required',
              message: source.error,
            }),
          ],
        };
      }

      const { content, filePath } = source.value;
      const findings: WorkflowValidationIssue[] = [];

      // Parse once, only if a doc-consuming layer is selected.
      const needsDoc = selected.has('yaml') || selected.has('schema') || selected.has('semantic');
      let doc = null as ReturnType<typeof parseWorkflow>['doc'];
      let fatal = false;
      if (needsDoc) {
        const parsed = parseWorkflow(content);
        doc = parsed.doc;
        fatal = parsed.fatal;
        if (selected.has('yaml') || fatal) findings.push(...parsed.findings);
        if (selected.has('yaml')) findings.push(...checkYaml(content, doc));
      }

      // Layer 3 (refs) is line-oriented and works even on structurally-broken YAML.
      if (selected.has('refs')) {
        findings.push(
          ...(await checkRefs(content, { checkActionExistence: input.checkActionExistence }, ctx)),
        );
      }

      // Layers 2 & 4 require a parseable document.
      if (doc && !fatal) {
        if (selected.has('schema')) findings.push(...checkSchema(doc));
        if (selected.has('semantic')) findings.push(...checkSemantic(doc, content, knowledge, input));
      }

      return { findings, filePath };
    },
  },

  plan: {
    buildPlan: (input, _knowledge, rules, confidence): WorkflowValidationPlan => {
      const { findings, filePath } = rules;
      return buildReport(findings, filePath, confidence, input);
    },
  },
});

// ─── Report assembly ──────────────────────────────────────────────────────────

function gradeFor(score: number): ValidationGrade {
  if (score >= 90) return 'A';
  if (score >= 80) return 'B';
  if (score >= 70) return 'C';
  if (score >= 60) return 'D';
  return 'F';
}

function buildReport(
  findings: WorkflowValidationIssue[],
  filePath: string,
  confidence: number,
  input: ValidateGithubWorkflowParams,
): WorkflowValidationPlan {
  let errors = 0;
  let warnings = 0;
  let info = 0;
  let penalty = 0;

  for (const f of findings) {
    // Carry the pattern confidence on Layer-4 (semantic) findings.
    if (f.layer === 'semantic') f.confidence = confidence;

    switch (f.metadata?.severity) {
      case ValidationSeverity.ERROR:
        errors++;
        penalty += PENALTY.ERROR;
        break;
      case ValidationSeverity.WARNING:
        warnings++;
        penalty += PENALTY.WARNING;
        break;
      default:
        info++;
        penalty += PENALTY.INFO;
        break;
    }
  }

  const score = Math.max(0, Math.min(100, 100 - penalty));
  const report: ValidationReport = {
    results: findings,
    score,
    grade: gradeFor(score),
    passed: 0,
    failed: findings.length,
    errors,
    warnings,
    info,
    timestamp: new Date().toISOString(),
  };

  const summary =
    errors > 0
      ? `🔧 ACTION REQUIRED: ${filePath} has ${pluralize(errors, 'required issue')} (score ${score}/100, grade ${report.grade}; ${pluralize(warnings, 'warning')}, ${info} info). Apply the fixes in nextAction, then re-run validate-github-workflow until it passes.`
      : `✅ ${filePath} passed all required checks (score ${score}/100, grade ${report.grade}; ${pluralize(warnings, 'warning')}, ${info} info). Safe to commit the workflow and push.`;

  const plan: WorkflowValidationPlan = {
    report,
    filePath,
    summary,
    attributionLabels: {
      annotations: {
        'com.azure.containerizationassist/version': PACKAGE_VERSION,
        'com.azure.containerizationassist/workflow-validator': 'validate-github-workflow',
      },
    },
    confidence,
  };

  if (errors > 0) {
    plan.nextAction = buildFixAction(findings, filePath, input);
  }

  return plan;
}

function buildFixAction(
  findings: WorkflowValidationIssue[],
  filePath: string,
  input: ValidateGithubWorkflowParams,
): NonNullable<WorkflowValidationPlan['nextAction']> {
  const errorFindings = findings.filter(
    (f) => f.metadata?.severity === ValidationSeverity.ERROR,
  );

  // For inline content there is no on-disk path; direct the fix loop at the
  // caller's workflowFileName (e.g. ci.yml) rather than a hardcoded default.
  const targetPath = filePath === '<inline>' ? workflowRelativePath(input) : filePath;

  const instruction = [
    `The GitHub Actions workflow at ${targetPath} has ${pluralize(errorFindings.length, 'required issue')} that must be fixed:`,
    '',
    ...errorFindings.map((f, i) => {
      const loc = f.metadata?.location ? ` (${f.metadata.location})` : '';
      const fix = f.suggestions?.[0] ? `\n     Fix: ${f.suggestions[0]}` : '';
      return `  ${i + 1}. [${f.ruleId ?? 'unknown'}]${loc} ${f.message ?? ''}${fix}`;
    }),
    '',
    'Edit the workflow to resolve each issue, then call validate-github-workflow again until it reports no required issues.',
  ].join('\n');

  return {
    action: 'fix-files',
    instruction,
    files: [
      {
        path: targetPath,
        purpose: 'GitHub Actions CI/CD workflow to fix so it passes validation',
      },
    ],
  };
}

// ─── Handler ──────────────────────────────────────────────────────────────────

async function handleValidateGithubWorkflow(
  input: ValidateGithubWorkflowParams,
  ctx: ToolContext,
): Promise<Result<WorkflowValidationPlan>> {
  return runPattern(input, ctx);
}

// ─── Tool export ──────────────────────────────────────────────────────────────

export default tool({
  name: TOOL_NAME.VALIDATE_GITHUB_WORKFLOW,
  description: validateGithubWorkflowToolDefinition.description,
  schema: validateGithubWorkflowSchema,
  metadata: { knowledgeEnhanced: true },
  handler: handleValidateGithubWorkflow,
  category: 'docker',
  version: '1.0.0',
  chainHints: validateGithubWorkflowToolDefinition.chainHints,
});
