import { validateGithubWorkflowSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const validateGithubWorkflowToolDefinition = {
  name: TOOL_NAME.VALIDATE_GITHUB_WORKFLOW,
  description:
    'Deterministically validate a GitHub Actions deploy workflow (no AI calls). Checks YAML validity, structural schema (on/jobs/steps, needs graph), SHA-pinning of every `uses:` action, and the CA-specific invariants (buildImage/deploy job keys, az acr build, no job-level environment:, correct Azure actions, required secrets). Returns a ValidationReport; on failure it emits a fix-files nextAction enumerating each required issue.',
  category: 'docker' as const,
  version: '1.0.0',
  schema: validateGithubWorkflowSchema,
  metadata: {
    knowledgeEnhanced: true,
  },
  chainHints: {
    success:
      'Workflow valid — no required issues found. Safe to commit the workflow under .github/workflows/ and push. After committing, ensure AZURE_CLIENT_ID, AZURE_TENANT_ID, and AZURE_SUBSCRIPTION_ID are configured as GitHub repository secrets.',
    failure:
      'Validation found required issue(s). Apply the fixes listed in nextAction (edit the workflow file), then call validate-github-workflow again until it passes.',
  },
} satisfies IToolDefinition;
