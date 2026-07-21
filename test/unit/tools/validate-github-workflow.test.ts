/**
 * Unit Tests: Validate GitHub Workflow Tool
 * Tests the validate-github-workflow tool schema, engine layers, and fix-loop output.
 */

import { jest } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { ToolContext } from '@/mcp/context';

// Mock the knowledge matcher the tool calls via the knowledge-tool-pattern.
const mockGetKnowledgeSnippets = jest.fn();
jest.mock('@/knowledge/matcher', () => ({
  getKnowledgeSnippets: mockGetKnowledgeSnippets,
}));

jest.mock('@/lib/logger', () => ({
  createLogger: jest.fn(() => createMockLogger()),
}));

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

function createMockToolContext(): ToolContext {
  return { logger: createMockLogger() } as any;
}

// Import after mocks
import validateGithubWorkflowTool from '@/tools/validate-github-workflow/tool';
import {
  validateGithubWorkflowSchema,
  type WorkflowValidationPlan,
} from '@/tools/validate-github-workflow/schema';
import { workflowRelativePath } from '@/tools/validate-github-workflow/checks/helpers';
import { extractUsesRefs, isPinnedSha } from '@/tools/validate-github-workflow/checks/refs-check';

// ─── Helpers ────────────────────────────────────────────────────────────────

const CHECKOUT_SHA = 'df4cb1c069e1874edd31b4311f1884172cec0e10';

/** Load a workflow fixture from test/fixtures/workflows/{happy,sad}/. */
function readFixture(kind: 'happy' | 'sad', name: string): string {
  return readFileSync(
    join(__dirname, '../../fixtures/workflows', kind, `${name}.yml`),
    'utf-8',
  );
}
const happy = (name: string): string => readFixture('happy', name);
const sad = (name: string): string => readFixture('sad', name);

async function validate(
  input: Partial<Parameters<typeof validateGithubWorkflowTool.handler>[0]> & {
    workflowContent?: string;
  },
): Promise<WorkflowValidationPlan> {
  const result = await validateGithubWorkflowTool.handler(
    {
      repositoryPath: '/tmp/repo',
      checkActionExistence: false,
      manifestFormat: 'k8s',
      ...input,
    } as any,
    createMockToolContext(),
  );
  expect(result.ok).toBe(true);
  if (!result.ok) throw new Error('handler failed');
  return result.value;
}

function ruleIds(plan: WorkflowValidationPlan): string[] {
  return plan.report.results.map((r) => r.ruleId ?? '');
}

function errorRuleIds(plan: WorkflowValidationPlan): string[] {
  return plan.report.results
    .filter((r) => r.metadata?.severity === 'error')
    .map((r) => r.ruleId ?? '');
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('validate-github-workflow', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockGetKnowledgeSnippets.mockResolvedValue([]);
  });

  // ── Schema ────────────────────────────────────────────────────────────────

  describe('Schema', () => {
    it('applies defaults for optional fields', () => {
      const result = validateGithubWorkflowSchema.safeParse({ repositoryPath: '/home/user/app' });
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.workflowFileName).toBe('deploy.yml');
        expect(result.data.manifestFormat).toBe('k8s');
        expect(result.data.checkActionExistence).toBe(false);
      }
    });

    it('rejects input missing repositoryPath', () => {
      const result = validateGithubWorkflowSchema.safeParse({ workflowContent: 'x' });
      expect(result.success).toBe(false);
    });

    it('rejects an invalid manifestFormat', () => {
      const result = validateGithubWorkflowSchema.safeParse({
        repositoryPath: '/x',
        manifestFormat: 'terraform',
      });
      expect(result.success).toBe(false);
    });

    it('rejects an invalid layer value', () => {
      const result = validateGithubWorkflowSchema.safeParse({
        repositoryPath: '/x',
        layers: ['yaml', 'nope'],
      });
      expect(result.success).toBe(false);
    });

    it('basename-sanitizes workflowFileName to prevent traversal', () => {
      expect(workflowRelativePath({ workflowFileName: '../../escape.yml' } as any)).toBe(
        '.github/workflows/escape.yml',
      );
      expect(workflowRelativePath({ workflowFileName: 'nested/path.yml' } as any)).toBe(
        '.github/workflows/path.yml',
      );
      expect(workflowRelativePath({ workflowFileName: '/abs/deploy.yml' } as any)).toBe(
        '.github/workflows/deploy.yml',
      );
    });

    it('workflowContent takes precedence over reading from disk', async () => {
      // repositoryPath points nowhere, but inline content is used, so filePath is '<inline>'
      const plan = await validate({
        repositoryPath: '/nonexistent',
        workflowContent: happy('deploy'),
      });
      expect(plan.filePath).toBe('<inline>');
    });
  });

  // ── Layer 1: YAML ───────────────────────────────────────────────────────────

  describe('Layer 1 — YAML', () => {
    it('flags malformed YAML as a required parse error and skips later layers', async () => {
      const plan = await validate({ workflowContent: sad('malformed') });
      expect(plan.report.errors).toBeGreaterThanOrEqual(1);
      expect(errorRuleIds(plan)).toContain('yaml/parse');
      // Fatal parse → only Layer-1 findings are produced.
      expect(plan.report.results.every((r) => r.layer === 'yaml')).toBe(true);
      const parseFinding = plan.report.results.find((r) => r.ruleId === 'yaml/parse');
      expect(parseFinding?.metadata?.location).toBeDefined();
    });
  });

  // ── Layer 3: action refs / SHA pinning ────────────────────────────────────────

  describe('Layer 3 — action refs', () => {
    it('isPinnedSha accepts 40/64-char SHAs and rejects tags/short shas', () => {
      expect(isPinnedSha(CHECKOUT_SHA)).toBe(true);
      expect(isPinnedSha('a'.repeat(64))).toBe(true);
      expect(isPinnedSha('v4')).toBe(false);
      expect(isPinnedSha('v4.2.2')).toBe(false);
      expect(isPinnedSha('main')).toBe(false);
      expect(isPinnedSha('abc1234')).toBe(false);
    });

    it('extractUsesRefs skips local (./) and docker:// references', () => {
      const content = [
        'uses: actions/checkout@v4',
        'uses: ./.github/actions/local',
        'uses: docker://ghcr.io/owner/img:tag',
        'uses: azure/login@532459ea530d8321f2fb9bb10d1e0bcf23869a43 # v3.0.0',
      ].join('\n');
      const refs = extractUsesRefs(content);
      expect(refs.map((r) => r.ownerRepo)).toEqual(['actions/checkout', 'azure/login']);
      expect(refs[1]?.comment).toBe('v3.0.0');
    });

    it('flags every tag-pinned action with a required sha-pin finding', async () => {
      const plan = await validate({ workflowContent: sad('unpinned') });
      const shaPin = plan.report.results.filter((r) => r.ruleId === 'refs/sha-pin');
      expect(shaPin.length).toBe(7);
      expect(shaPin.every((r) => r.metadata?.severity === 'error')).toBe(true);
      expect(shaPin[0]?.actionRef).toBeDefined();
    });

    it('does not raise sha-pin findings for fully SHA-pinned actions', async () => {
      const plan = await validate({ workflowContent: happy('deploy') });
      expect(ruleIds(plan)).not.toContain('refs/sha-pin');
    });

    it('offline existence probe is downgraded to info and never fails the tool', async () => {
      const originalFetch = global.fetch;
      global.fetch = jest.fn().mockRejectedValue(new Error('offline')) as any;
      try {
        const plan = await validate({
          workflowContent: happy('deploy'),
          checkActionExistence: true,
        });
        expect(plan.report.errors).toBe(0);
        expect(ruleIds(plan)).toContain('refs/sha-exists-skipped');
      } finally {
        global.fetch = originalFetch;
      }
    });
  });

  // ── Layer 4: CA semantic invariants ───────────────────────────────────────────

  describe('Layer 4 — semantic invariants', () => {
    it('flags docker build and job-level environment as required issues', async () => {
      const plan = await validate({ workflowContent: sad('semantic') });
      expect(errorRuleIds(plan)).toContain('semantic/az-acr-build');
      expect(errorRuleIds(plan)).toContain('semantic/no-job-environment');
    });

    it('flags renamed job keys as a warning-level job-keys finding', async () => {
      const plan = await validate({ workflowContent: sad('semantic') });
      const jobKeys = plan.report.results.filter((r) => r.ruleId === 'semantic/job-keys');
      expect(jobKeys.length).toBeGreaterThan(0);
      expect(jobKeys.every((r) => r.metadata?.severity === 'warning')).toBe(true);
    });

    it('flags a missing required secret', async () => {
      const noSecrets = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    permissions: { contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      - run: az acr build --image x .',
        '  deploy:',
        '    needs: [buildImage]',
        '    runs-on: ubuntu-latest',
        '    permissions: { actions: read, contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      - uses: azure/login@532459ea530d8321f2fb9bb10d1e0bcf23869a43',
        '      - uses: azure/aks-set-context@60623acbdcbbdcf799ad50a1adf8703874339f8b',
        "        with: { admin: 'false', use-kubelogin: 'true' }",
        '      - uses: Azure/k8s-deploy@c7ebd0d5f39477a23f1b5dea0f52e6db04adf28e',
      ].join('\n');
      const plan = await validate({ workflowContent: noSecrets });
      const secrets = plan.report.results.find((r) => r.ruleId === 'semantic/required-secrets');
      expect(secrets).toBeDefined();
      expect(secrets?.message).toContain('AZURE_CLIENT_ID');
    });
  });

  // ── Verdict + fix loop ────────────────────────────────────────────────────────

  describe('Verdict and fix loop', () => {
    it('a fully-correct SHA-pinned workflow passes with no nextAction', async () => {
      const plan = await validate({ workflowContent: happy('deploy') });
      expect(plan.report.errors).toBe(0);
      expect(plan.report.grade).toBe('A');
      expect(plan.nextAction).toBeUndefined();
      expect(plan.summary).toContain('passed all required checks');
      expect(typeof plan.confidence).toBe('number');
    });

    it('a failing workflow returns a fix-files nextAction enumerating each required issue', async () => {
      const plan = await validate({ workflowContent: sad('unpinned') });
      expect(plan.report.errors).toBeGreaterThan(0);
      expect(plan.nextAction).toBeDefined();
      expect(plan.nextAction?.action).toBe('fix-files');
      expect(plan.nextAction?.files[0]?.path).toBeDefined();
      // Every required finding is enumerated in the instruction.
      const requiredCount = plan.report.errors;
      expect(plan.nextAction?.instruction).toContain(`${requiredCount} required issue`);
      expect(plan.nextAction?.instruction).toContain('refs/sha-pin');
    });

    it('reports a not-found error when neither content nor file is available', async () => {
      const plan = await validate({ repositoryPath: '/definitely/not/here' });
      expect(plan.report.errors).toBeGreaterThanOrEqual(1);
      expect(ruleIds(plan)).toContain('source/not-found');
    });

    it('respects the layers filter (refs only skips semantic findings)', async () => {
      const plan = await validate({
        workflowContent: sad('semantic'),
        layers: ['refs'],
      });
      expect(ruleIds(plan).some((id) => id.startsWith('semantic/'))).toBe(false);
    });
  });

  // ── Tool metadata ──────────────────────────────────────────────────────────

  describe('Tool metadata', () => {
    it('has the correct tool name', () => {
      expect(validateGithubWorkflowTool.name).toBe('validate-github-workflow');
    });

    it('has knowledgeEnhanced metadata', () => {
      expect(validateGithubWorkflowTool.metadata.knowledgeEnhanced).toBe(true);
    });

    it('has chainHints', () => {
      expect(validateGithubWorkflowTool.chainHints?.success).toBeDefined();
      expect(validateGithubWorkflowTool.chainHints?.failure).toBeDefined();
    });
  });
});
