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
import generateGithubWorkflowTool from '@/tools/generate-github-workflow/tool';
import type { GithubWorkflowPlan } from '@/tools/generate-github-workflow/schema';
import { ACTION_PINS, pinnedUses } from '@/tools/shared/action-pins';
import { JOB_KEYS } from '@/tools/shared/workflow-contract';
import {
  validateGithubWorkflowSchema,
  type WorkflowValidationPlan,
} from '@/tools/validate-github-workflow/schema';
import { workflowRelativePath } from '@/tools/validate-github-workflow/checks/helpers';
import {
  extractUsesRefs,
  isPinnedSha,
  isVersionComment,
} from '@/tools/validate-github-workflow/checks/refs-check';

// ─── Helpers ────────────────────────────────────────────────────────────────

// Per-action pins, taken from the registry. Reusing one action's SHA for another would make
// a ref look correctly pinned while pointing at the wrong action — misleading fixture data
// that a future rule (or a real bug) could hide behind. Sourcing them here also means a pin
// refresh flows through every fixture automatically.
const CHECKOUT_SHA = ACTION_PINS.checkout.sha;
const AZURE_LOGIN_SHA = ACTION_PINS.azureLogin.sha;
const AKS_SET_CONTEXT_SHA = ACTION_PINS.aksSetContext.sha;
const K8S_BAKE_SHA = ACTION_PINS.k8sBake.sha;
const K8S_DEPLOY_SHA = ACTION_PINS.k8sDeploy.sha;

/** Valid SHA shape for an action deliberately absent from the pin registry. */
const UNKNOWN_ACTION_SHA = 'a1b2c3d4e5f60718293a4b5c6d7e8f9012345678';

/** Load a workflow fixture from test/fixtures/workflows/{happy,sad}/. */
function readFixture(kind: 'happy' | 'sad', name: string): string {
  return readFileSync(join(__dirname, '../../fixtures/workflows', kind, `${name}.yml`), 'utf-8');
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

    it('treats an empty inline workflowContent as authoritative (does not silently read disk)', async () => {
      // A defined-but-blank workflowContent must fail as an empty source rather than
      // falling back to a possibly-stale file on disk.
      const plan = await validate({ repositoryPath: '/nonexistent', workflowContent: '   ' });
      expect(plan.report.errors).toBeGreaterThanOrEqual(1);
      expect(ruleIds(plan)).toContain('source/not-found');
      expect(plan.report.results.some((r) => /empty/i.test(r.message ?? ''))).toBe(true);
    });

    it('flags `with`/`secrets` on a normal (non-reusable) job as invalid job keys', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  build:',
        '    runs-on: ubuntu-latest',
        '    with: { foo: bar }',
        '    secrets: { token: abc }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['schema'] });
      const invalid = plan.report.results.filter((r) => r.ruleId === 'schema/invalid-job-key');
      expect(invalid.map((r) => r.message).join(' ')).toContain('with');
      expect(invalid.map((r) => r.message).join(' ')).toContain('secrets');
      expect(ruleIds(plan)).not.toContain('schema/unknown-job-key');
    });

    it('allows `with`/`secrets` on a reusable-workflow-call job (`uses:`)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  call:',
        '    uses: owner/repo/.github/workflows/reusable.yml@main',
        '    with: { foo: bar }',
        '    secrets: { token: abc }',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['schema'] });
      const ids = ruleIds(plan);
      expect(ids).not.toContain('schema/invalid-job-key');
      expect(ids).not.toContain('schema/unknown-job-key');
      expect(ids).not.toContain('schema/invalid-uses');
      expect(ids).not.toContain('schema/missing-runs-on');
    });

    it.each([
      ['numeric', '    uses: 123'],
      ['empty string', "    uses: ''"],
      ['whitespace-only string', "    uses: ' '"],
      ['null', '    uses: null'],
      ['empty mapping', '    uses: {}'],
    ])(
      'flags a malformed `uses` (%s) and still enforces the normal-job steps requirement',
      async (_label, usesLine) => {
        // `uses` present but not a non-empty string → not a valid reusable call. Provide
        // `runs-on` so `missing-steps` is unambiguously the expected normal-job requirement,
        // independent of whether the schema check ever gates it behind `runs-on`.
        const content = [
          'on: { push: { branches: [main] } }',
          'jobs:',
          '  build:',
          '    runs-on: ubuntu-latest',
          usesLine,
        ].join('\n');
        const plan = await validate({ workflowContent: content, layers: ['schema'] });
        const ids = ruleIds(plan);
        // Presence checks are decoupled from the severity mapping.
        expect(ids).toContain('schema/invalid-uses');
        // Treated as a normal job: with `runs-on` provided, the missing `steps` list is flagged
        // (and `missing-runs-on` is satisfied).
        expect(ids).toContain('schema/missing-steps');
        expect(ids).not.toContain('schema/missing-runs-on');
        // The malformed-`uses` finding is intentionally error-severity (`required`).
        const invalidUses = plan.report.results.find((r) => r.ruleId === 'schema/invalid-uses');
        expect(invalidUses?.metadata?.severity).toBe('error');
      },
    );
    // Runner labels are deliberately not validated: an allow-list goes stale every time
    // GitHub ships a runner, and ours already rejected larger and ARM runners as "unknown".
    it.each([
      'ubuntu-latest-8-cores',
      'ubuntu-24.04-arm',
      'windows-11-arm',
      'macos-latest-xlarge',
      'macos-26',
      'some-self-hosted-pool',
    ])('does not flag runner label %s', async (runner) => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  build:',
        `    runs-on: ${runner}`,
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['schema'] });
      expect(ruleIds(plan)).not.toContain('schema/unknown-runner');
      expect(ruleIds(plan)).toEqual([]);
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

    it('does not emit YAML-layer findings for a refs-only run on fatal YAML', async () => {
      // Layers are independently toggleable: a refs-only run falls back to the line scan
      // and must not surface yaml/parse findings the caller did not ask for.
      const plan = await validate({ workflowContent: sad('malformed'), layers: ['refs'] });
      expect(ruleIds(plan)).not.toContain('yaml/parse');
      expect(plan.report.results.every((r) => r.layer !== 'yaml')).toBe(true);
    });

    it('surfaces the fatal parse error when a doc-dependent layer (schema) is selected', async () => {
      const plan = await validate({ workflowContent: sad('malformed'), layers: ['schema'] });
      expect(errorRuleIds(plan)).toContain('yaml/parse');
    });

    // Pins the position convention we depend on: `yaml`'s linePos reports 1-based line AND
    // 1-based column (its own message for this input reads "at line 3, column 1"). If a yaml
    // upgrade ever switched either to 0-based, every reported location would silently shift.
    it('reports parse-error positions as 1-based line and column', async () => {
      const plan = await validate({
        workflowContent: 'jobs:\n  a: [1, 2\n',
        layers: ['yaml'],
      });
      const parse = plan.report.results.find((r) => r.ruleId === 'yaml/parse');
      expect(parse).toBeDefined();
      expect(parse?.line).toBe(3);
      expect(parse?.metadata?.location).toBe('column 1');
    });
  });

  // ── Layer 3: action refs / SHA pinning ────────────────────────────────────────

  describe('Layer 3 — action refs', () => {
    // Regression: a *non-fatal* parse error (a stray tab) used to force Layer 3 onto the
    // line scan, which cannot see inline-map steps — so an unpinned action vanished from
    // the report entirely and a mutable ref shipped unflagged. The partial AST is now
    // harvested regardless of parse errors, unioned with the scan.
    it('still finds inline-map refs when the parse is degraded but not fatal', async () => {
      const content = [
        'name: deploy',
        'on:',
        '  push:',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '\t\tenv:', // tab indent → parse error, tree still walkable
        '      FOO: bar',
        '    steps:',
        '      - uses: actions/checkout@v4', // block style: both extractors see it
        '      - { uses: azure/login@v2 }', // inline map: ONLY the AST sees it
        '',
      ].join('\n');

      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const flagged = plan.report.results
        .filter((r) => r.ruleId === 'refs/sha-pin')
        .map((r) => r.actionRef);

      expect(flagged).toContain('azure/login@v2');
      expect(flagged).toContain('actions/checkout@v4');
    });

    it('does not double-report a ref both extractors find on a degraded parse', async () => {
      const content = [
        'jobs:',
        '  a:',
        '\t\tenv: x',
        '    steps:',
        '      - uses: actions/checkout@v4',
        '',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const checkoutFindings = plan.report.results.filter(
        (r) => r.ruleId === 'refs/sha-pin' && r.actionRef === 'actions/checkout@v4',
      );
      expect(checkoutFindings).toHaveLength(1);
    });

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
        `uses: ${pinnedUses(ACTION_PINS.azureLogin)}`,
      ].join('\n');
      const refs = extractUsesRefs(content);
      expect(refs.map((r) => r.ownerRepo)).toEqual(['actions/checkout', 'azure/login']);
      expect(refs[1]?.comment).toBe(ACTION_PINS.azureLogin.version);
    });

    it('ignores commented-out `uses:` and `uses:` text inside run scripts', () => {
      const content = [
        '      # uses: actions/checkout@v4',
        '      - run: echo "uses: actions/checkout@v4"',
        '      - uses: actions/setup-node@v5',
      ].join('\n');
      const refs = extractUsesRefs(content);
      expect(refs.map((r) => r.raw)).toEqual(['actions/setup-node@v5']);
    });

    // The fallback scan (used when YAML is unparseable) builds its own line index rather
    // than rescanning the source per match, so its offset->line mapping needs its own cover.
    it('reports 1-based line numbers from the fallback line scan', () => {
      const content = [
        `      - uses: actions/checkout@${CHECKOUT_SHA}`, // 1
        '      - run: echo hi', //                          2
        '', //                                              3
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`, //   4
      ].join('\n');
      const refs = extractUsesRefs(content);
      expect(refs.map((r) => r.line)).toEqual([1, 4]);
    });

    // The line-anchored fallback regex cannot see these; the parsed path must, or an
    // unpinned action in inline-map style would silently pass the SHA-pin gate.
    it.each([
      ['inline map step', '      - { uses: actions/checkout@v4 }'],
      ['flow sequence', '    steps: [{ uses: actions/checkout@v4 }]'],
    ])('catches an unpinned action written as an %s', async (_label, stepLine) => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  build:',
        '    runs-on: ubuntu-latest',
        stepLine.startsWith('    steps:') ? stepLine : '    steps:',
        ...(stepLine.startsWith('    steps:') ? [] : [stepLine]),
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const shaPin = plan.report.results.find((r) => r.ruleId === 'refs/sha-pin');
      expect(shaPin).toBeDefined();
      expect(shaPin?.actionRef).toBe('actions/checkout@v4');
    });

    it('does not treat `uses:` inside a multiline run script as a real action (parsed nodes)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      - name: doc',
        '        run: |',
        '          echo "example workflow snippet:"',
        '          echo "  - uses: actions/checkout@v4"',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      // The only real action is the pinned checkout; the `uses:` inside the run block
      // must not raise a sha-pin finding.
      expect(ruleIds(plan)).not.toContain('refs/sha-pin');
    });

    it('preserves action subpaths in the sha-pin finding actionRef and message', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  build:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        '      - uses: owner/repo/path/to/action@v1',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const shaPin = plan.report.results.find((r) => r.ruleId === 'refs/sha-pin');
      expect(shaPin).toBeDefined();
      expect(shaPin?.actionRef).toBe('owner/repo/path/to/action@v1');
      expect(shaPin?.message).toContain('owner/repo/path/to/action@v1');
      expect(shaPin?.suggestions?.[0]).toContain('owner/repo/path/to/action@<40-char-sha>');
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

    it('validates refs without any network access', async () => {
      // Layer 3 is deliberately offline: upstream SHA existence is verified out-of-band by
      // scripts/refresh-action-pins.ts and scripts/validate-action-refs.ts, not per tool run.
      const originalFetch = global.fetch;
      global.fetch = jest.fn(() => {
        throw new Error('network access is not allowed from the refs layer');
      }) as any;
      try {
        const content = [
          'on: { push: { branches: [main] } }',
          'jobs:',
          '  buildImage:',
          '    runs-on: ubuntu-latest',
          '    steps:',
          `      - uses: actions/checkout@${CHECKOUT_SHA}`,
          `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
          `      - uses: azure/k8s-bake@${K8S_BAKE_SHA}`,
        ].join('\n');
        const plan = await validate({ workflowContent: content, layers: ['refs'] });
        expect(plan.report.errors).toBe(0);
        expect(global.fetch).not.toHaveBeenCalled();
      } finally {
        global.fetch = originalFetch;
      }
    });

    it('emits INFO findings with an empty warnings array (warnings reserved for WARNING severity)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`, // pinned SHA, no # version comment
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const vc = plan.report.results.find((r) => r.ruleId === 'refs/version-comment');
      expect(vc).toBeDefined();
      expect(vc?.metadata?.severity).toBe('info');
      expect(vc?.warnings).toEqual([]);
      expect(vc?.errors).toEqual([]);
    });

    it('isVersionComment accepts version-like comments and rejects others', () => {
      expect(isVersionComment('v6')).toBe(true);
      expect(isVersionComment('v6.0')).toBe(true);
      expect(isVersionComment('v6.0.3')).toBe(true);
      expect(isVersionComment('V6.0.3')).toBe(true); // case-insensitive
      expect(isVersionComment('  v6.0.3  ')).toBe(true); // surrounding whitespace is trimmed
      expect(isVersionComment('6.0.3')).toBe(false); // missing leading v
      expect(isVersionComment('v6.0.3 (LTS)')).toBe(false); // trailing note
      expect(isVersionComment('v6.0.3-beta')).toBe(false); // pre-release suffix
      expect(isVersionComment('pinned')).toBe(false);
      expect(isVersionComment('latest')).toBe(false);
      expect(isVersionComment(undefined)).toBe(false);
      expect(isVersionComment('')).toBe(false);
    });

    it('flags a pinned action whose trailing comment is not a version (e.g. `# pinned`)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA} # pinned`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const vc = plan.report.results.find((r) => r.ruleId === 'refs/version-comment');
      expect(vc).toBeDefined();
      expect(vc?.message).toContain('non-version');
    });

    it('treats a whitespace-only trailing comment as "no version comment"', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA} #${'   '}`, // trailing comment is only spaces
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const vc = plan.report.results.find((r) => r.ruleId === 'refs/version-comment');
      expect(vc).toBeDefined();
      expect(vc?.message).toContain('has no version comment');
      expect(vc?.message).not.toContain('non-version');
    });

    it('accepts a version-like trailing comment (`# v6.0.3`)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA} # v6.0.3`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      expect(ruleIds(plan)).not.toContain('refs/version-comment');
    });

    it('accepts a version-like trailing comment with surrounding whitespace (`#   v6.0.3  `)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA} #${'   '}v6.0.3${'  '}`, // padded on both sides
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      expect(ruleIds(plan)).not.toContain('refs/version-comment');
    });

    it('sanitizes an untrusted non-version comment (truncates + neutralizes backticks)', async () => {
      const longComment = `${'x'.repeat(80)} \`inject\``;
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA} # ${longComment}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['refs'] });
      const vc = plan.report.results.find((r) => r.ruleId === 'refs/version-comment');
      expect(vc).toBeDefined();
      const msg = vc?.message ?? '';
      // Backticks from the comment are neutralized and the embedded text is truncated.
      expect(msg).not.toContain('`inject`');
      expect(msg).toContain('\u2026');
      expect(msg).not.toContain('x'.repeat(80));
    });
  });

  // ── Layer 4: CA semantic invariants ───────────────────────────────────────────

  describe('Layer 4 — semantic invariants', () => {
    it('flags docker build and job-level environment as required issues', async () => {
      const plan = await validate({ workflowContent: sad('semantic') });
      expect(errorRuleIds(plan)).toContain('semantic/az-acr-build');
      expect(errorRuleIds(plan)).toContain('semantic/no-job-environment');
    });

    it('does not flag `docker build` that appears only in a comment or step name', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    permissions: { contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      # reminder: never use docker build here — use az acr build',
        '      - name: docker build (do NOT do this locally)',
        '        run: az acr build --image x .',
        '  deploy:',
        '    needs: [buildImage]',
        '    runs-on: ubuntu-latest',
        '    permissions: { actions: read, contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      // The only `docker build` text is a YAML comment + a step name — neither is a real
      // build step, so no az-acr-build finding (forbidden or missing) should be raised.
      expect(ruleIds(plan)).not.toContain('semantic/az-acr-build');
    });

    it('flags a `docker build` inside a multiline run block', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      - run: |',
        '          echo "building the image"',
        '          docker build -t myapp .',
        '  deploy:',
        '    needs: [buildImage]',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      expect(errorRuleIds(plan)).toContain('semantic/az-acr-build');
    });

    it('inspects the real aks-set-context step `with`, ignoring flags in other steps', async () => {
      // Correct admin/use-kubelogin appear on a DIFFERENT action's `with`, but the actual
      // azure/aks-set-context step omits them → must still be flagged.
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        `      - uses: some/other-action@${UNKNOWN_ACTION_SHA}`,
        "        with: { admin: 'false', use-kubelogin: 'true' }",
        `      - uses: azure/aks-set-context@${AKS_SET_CONTEXT_SHA}`,
        '        with: { resource-group: rg, cluster-name: c }',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      expect(ruleIds(plan)).toContain('semantic/aks-context-flags');
    });

    it('does not flag a correctly configured aks-set-context step', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: azure/aks-set-context@${AKS_SET_CONTEXT_SHA}`,
        "        with: { admin: 'false', use-kubelogin: 'true' }",
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      expect(ruleIds(plan)).not.toContain('semantic/aks-context-flags');
    });

    it('does not flag forbidden Azure actions that appear only in comments', async () => {
      // `az aks get-credentials` / `azure/setup-kubectl` mentioned only in comments must
      // not raise findings — only real step run/uses values should.
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      # reminder: never run az aks get-credentials',
        '      # reminder: never use azure/setup-kubectl',
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
        '      - run: az acr build --image x .',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const azureActionMsgs = plan.report.results
        .filter((r) => r.ruleId === 'semantic/azure-actions')
        .map((r) => r.message ?? '');
      expect(azureActionMsgs.some((m) => m.includes('get-credentials'))).toBe(false);
      expect(azureActionMsgs.some((m) => m.includes('setup-kubectl'))).toBe(false);
    });

    it('flags a missing azure/login even when `azure/login` appears only in a comment', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      # TODO: add azure/login here',
        '      - run: az acr build --image x .',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const msgs = plan.report.results
        .filter((r) => r.ruleId === 'semantic/azure-actions')
        .map((r) => r.message ?? '');
      // The comment-only `azure/login` must not satisfy the per-job deploy contract.
      expect(msgs.some((m) => /deploy.*must include a `azure\/login`/.test(m))).toBe(true);
    });

    it('accepts an azure/login step with different casing (case-insensitive)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: Azure/Login@${AZURE_LOGIN_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const msgs = plan.report.results
        .filter((r) => r.ruleId === 'semantic/azure-actions')
        .map((r) => r.message ?? '');
      // The cased login step satisfies the deploy login requirement (no login-missing finding).
      expect(msgs.some((m) => /must include a `azure\/login`/.test(m))).toBe(false);
    });

    it('enforces the per-job deployment contract (login+acr in buildImage; deploy actions in deploy)', async () => {
      // A structurally valid two-job workflow that omits every deployment action.
      const content = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    permissions: { contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '  deploy:',
        '    needs: [buildImage]',
        '    runs-on: ubuntu-latest',
        '    permissions: { actions: read, contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const msgs = plan.report.results
        .filter(
          (r) => r.ruleId === 'semantic/azure-actions' || r.ruleId === 'semantic/az-acr-build',
        )
        .map((r) => r.message ?? '');
      // buildImage: missing azure/login and az acr build.
      expect(msgs.some((m) => /buildImage.*azure\/login/.test(m))).toBe(true);
      expect(msgs.some((m) => /buildImage.*az acr build/.test(m))).toBe(true);
      // deploy: missing login, kubelogin, aks-set-context, k8s-deploy.
      expect(msgs.some((m) => /deploy.*azure\/login/.test(m))).toBe(true);
      expect(msgs.some((m) => /deploy.*azure\/use-kubelogin/.test(m))).toBe(true);
      expect(msgs.some((m) => /deploy.*azure\/aks-set-context/.test(m))).toBe(true);
      expect(msgs.some((m) => /deploy.*k8s-deploy/.test(m))).toBe(true);
    });

    it('flags concurrency when `cancel-in-progress: true` is only in a comment', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        '# concurrency here would set cancel-in-progress: true',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      expect(ruleIds(plan)).toContain('semantic/concurrency');
    });

    it('accepts a parsed concurrency mapping with cancel-in-progress: true', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      expect(ruleIds(plan)).not.toContain('semantic/concurrency');
    });

    it('flags a missing bake step even when `azure/k8s-bake` appears only in a comment', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      # remember to add azure/k8s-bake before deploy',
      ].join('\n');
      const plan = await validate({
        workflowContent: content,
        manifestFormat: 'helm',
        layers: ['semantic'],
      });
      expect(ruleIds(plan)).toContain('semantic/bake-step');
    });

    it('accepts a real azure/k8s-bake step for helm manifests', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: azure/k8s-bake@${K8S_BAKE_SHA}`,
      ].join('\n');
      const plan = await validate({
        workflowContent: content,
        manifestFormat: 'helm',
        layers: ['semantic'],
      });
      expect(ruleIds(plan)).not.toContain('semantic/bake-step');
    });

    it('flags a missing bake step when `azure/k8s-bake` is in buildImage, not deploy', async () => {
      // The bake step must run in the deploy job (its outputs feed Azure/k8s-deploy, and step
      // outputs are job-scoped) — a bake in buildImage cannot satisfy the deploy contract.
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: azure/k8s-bake@${K8S_BAKE_SHA}`,
        '  deploy:',
        '    needs: [buildImage]',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({
        workflowContent: content,
        manifestFormat: 'helm',
        layers: ['semantic'],
      });
      expect(ruleIds(plan)).toContain('semantic/bake-step');
    });

    // The generator states these as "⛔ CRITICAL RULES — MUST be followed exactly", so a
    // violation has to gate: fail the report and reach the fix loop, not sit in `warnings`
    // where the client can ignore it.
    it('gates on renamed job keys and routes them to the fix loop', async () => {
      const plan = await validate({ workflowContent: sad('semantic') });
      const jobKeys = plan.report.results.filter((r) => r.ruleId === 'semantic/job-keys');
      expect(jobKeys.length).toBeGreaterThan(0);
      expect(jobKeys.every((r) => r.metadata?.severity === 'error')).toBe(true);

      // ...and that failure actually reaches the agent as an actionable instruction.
      expect(plan.report.errors).toBeGreaterThan(0);
      expect(plan.nextAction?.action).toBe('fix-files');
      expect(plan.nextAction?.instruction).toContain('semantic/job-keys');
    });

    // Every CA-contract rule gates. A workflow missing the deployment contract must not be
    // reported as "passed all required checks" — that was the exact failure mode this
    // severity alignment fixes.
    it('gates on a missing deployment contract rather than passing with warnings', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  build-and-push:', //  renamed job keys
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content });

      const errored = errorRuleIds(plan);
      expect(errored).toContain('semantic/job-keys');
      expect(errored).toContain('semantic/required-secrets');
      expect(errored).toContain('semantic/concurrency');
      expect(plan.summary).toContain('ACTION REQUIRED');
      expect(plan.nextAction?.action).toBe('fix-files');
    });

    // The one deliberate exception: bake-step keys off the caller-supplied manifestFormat,
    // which can be wrong. Gating on it would push the agent to add a step the workflow does
    // not need, so it stays advisory.
    it('keeps the conditional bake-step advisory rather than gating', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({
        workflowContent: content,
        manifestFormat: 'helm',
        layers: ['semantic'],
      });
      const bake = plan.report.results.find((r) => r.ruleId === 'semantic/bake-step');
      expect(bake).toBeDefined();
      expect(bake?.metadata?.severity).toBe('info');
    });

    // The validator must stay fully functional with no knowledge base (offline, empty pack,
    // or a policy that filters every snippet). Layer 4 reads recommendation text by snippet
    // id from `knowledge.all` and falls back to hardcoded guidance when the id is absent.
    it('produces a complete report from hardcoded fallbacks when knowledge is empty', async () => {
      mockGetKnowledgeSnippets.mockResolvedValue([]);

      const plan = await validate({ workflowContent: sad('semantic') });

      // Findings are still produced and still carry actionable guidance.
      expect(ruleIds(plan)).toContain('semantic/job-keys');
      const jobKeys = plan.report.results.find((r) => r.ruleId === 'semantic/job-keys');
      expect(jobKeys?.suggestions?.[0]).toContain('Split the workflow into two jobs');

      const acr = plan.report.results.find((r) => r.ruleId === 'semantic/az-acr-build');
      expect(acr?.suggestions?.[0]).toContain('az acr build');
    });

    it('prefers knowledge-pack recommendation text over the hardcoded fallback', async () => {
      mockGetKnowledgeSnippets.mockResolvedValue([
        {
          id: 'workflow-two-job-structure',
          text: 'PACK TEXT: use buildImage and deploy job keys',
          category: 'cicd',
          tags: ['generate-github-workflow'],
          weight: 1.0,
        },
      ]);

      const plan = await validate({ workflowContent: sad('semantic') });

      const jobKeys = plan.report.results.find((r) => r.ruleId === 'semantic/job-keys');
      expect(jobKeys?.suggestions?.[0]).toBe('PACK TEXT: use buildImage and deploy job keys');
    });

    // Deterministic checks must not advertise a knowledge-derived confidence: the verdict is
    // the same whether or not the knowledge base returned anything.
    it('does not stamp per-finding confidence on semantic findings', async () => {
      const plan = await validate({ workflowContent: sad('semantic') });
      const semantic = plan.report.results.filter((r) => r.ruleId?.startsWith('semantic/'));
      expect(semantic.length).toBeGreaterThan(0);
      expect(semantic.every((r) => r.confidence === undefined)).toBe(true);
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
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
        `      - uses: azure/aks-set-context@${AKS_SET_CONTEXT_SHA}`,
        "        with: { admin: 'false', use-kubelogin: 'true' }",
        `      - uses: Azure/k8s-deploy@${K8S_DEPLOY_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: noSecrets });
      const secrets = plan.report.results.find((r) => r.ruleId === 'semantic/required-secrets');
      expect(secrets).toBeDefined();
      expect(secrets?.message).toContain('AZURE_CLIENT_ID');
    });

    it('does not accept a secret named only in a comment (requires a real ${{ secrets.X }} ref)', async () => {
      const content = [
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
        // Real refs for two secrets; the third appears only in a comment.
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
        '        with:',
        '          client-id: ${{ secrets.AZURE_CLIENT_ID }}',
        '          tenant-id: ${{ secrets.AZURE_TENANT_ID }}',
        '      # subscription-id: AZURE_SUBSCRIPTION_ID (configure this secret)',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const secrets = plan.report.results.find((r) => r.ruleId === 'semantic/required-secrets');
      expect(secrets).toBeDefined();
      expect(secrets?.message).toContain('AZURE_SUBSCRIPTION_ID');
      expect(secrets?.message).not.toContain('AZURE_CLIENT_ID');
    });

    it('flags a missing required secret when no azure/login step exists at all', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    permissions: { contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      - run: az acr build --image x .',
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const secrets = plan.report.results.filter((r) => r.ruleId === 'semantic/required-secrets');
      expect(secrets.length).toBe(1);
      expect(secrets[0]?.message).toContain('No `azure/login` step');
      expect(secrets[0]?.message).toContain('AZURE_CLIENT_ID');
      expect(secrets[0]?.message).toContain('AZURE_TENANT_ID');
      expect(secrets[0]?.message).toContain('AZURE_SUBSCRIPTION_ID');
    });

    it("flags a per-job incomplete login even when another job's login has all secrets", async () => {
      // buildImage's login has all three secrets, but deploy's login is missing one. A
      // global concatenation would wrongly pass; per-step validation must flag deploy.
      const content = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    permissions: { contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
        '        with:',
        '          client-id: ${{ secrets.AZURE_CLIENT_ID }}',
        '          tenant-id: ${{ secrets.AZURE_TENANT_ID }}',
        '          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}',
        '      - run: az acr build --image x .',
        '  deploy:',
        '    needs: [buildImage]',
        '    runs-on: ubuntu-latest',
        '    permissions: { actions: read, contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
        '        with:',
        '          client-id: ${{ secrets.AZURE_CLIENT_ID }}',
        '          tenant-id: ${{ secrets.AZURE_TENANT_ID }}',
        // deploy's login is missing subscription-id.
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const secrets = plan.report.results.filter((r) => r.ruleId === 'semantic/required-secrets');
      expect(secrets.length).toBe(1);
      expect(secrets[0]?.message).toContain('deploy');
      expect(secrets[0]?.message).toContain('AZURE_SUBSCRIPTION_ID');
      expect(secrets[0]?.message).not.toContain('AZURE_CLIENT_ID');
    });

    it('aggregates a single finding per job across multiple incomplete login steps', async () => {
      // Two azure/login steps in one job, each missing a different secret. The output must be
      // a single aggregated finding for the job (not one per login step).
      const content = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  deploy:',
        '    runs-on: ubuntu-latest',
        '    permissions: { actions: read, contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
        '        with:',
        '          client-id: ${{ secrets.AZURE_CLIENT_ID }}',
        '          tenant-id: ${{ secrets.AZURE_TENANT_ID }}',
        // first login missing subscription-id
        `      - uses: azure/login@${AZURE_LOGIN_SHA}`,
        '        with:',
        '          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}',
        // second login missing client-id and tenant-id
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const secrets = plan.report.results.filter((r) => r.ruleId === 'semantic/required-secrets');
      expect(secrets.length).toBe(1);
      // The single finding aggregates every secret missing from any login step in the job.
      expect(secrets[0]?.message).toContain('AZURE_CLIENT_ID');
      expect(secrets[0]?.message).toContain('AZURE_TENANT_ID');
      expect(secrets[0]?.message).toContain('AZURE_SUBSCRIPTION_ID');
    });

    it('flags the buildImage job when `contents: read` is missing (only id-token: write set)', async () => {
      const content = [
        'on: { push: { branches: [main] } }',
        'concurrency: { group: g, cancel-in-progress: true }',
        'jobs:',
        '  buildImage:',
        '    runs-on: ubuntu-latest',
        '    permissions: { id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
        '      - run: az acr build --image x .',
        '  deploy:',
        '    needs: [buildImage]',
        '    runs-on: ubuntu-latest',
        '    permissions: { actions: read, contents: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const perms = plan.report.results.filter((r) => r.ruleId === 'semantic/permissions');
      expect(perms.some((r) => r.metadata?.location === 'job "buildImage"')).toBe(true);
    });

    it('flags the deploy job when `contents: read` is missing (actions + id-token only)', async () => {
      const content = [
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
        '    permissions: { actions: read, id-token: write }',
        '    steps:',
        `      - uses: actions/checkout@${CHECKOUT_SHA}`,
      ].join('\n');
      const plan = await validate({ workflowContent: content, layers: ['semantic'] });
      const perms = plan.report.results.filter((r) => r.ruleId === 'semantic/permissions');
      expect(perms.some((r) => r.metadata?.location === 'job "deploy"')).toBe(true);
    });
  });

  // ── Generator ↔ validator lockstep ───────────────────────────────────────────
  // generate-github-workflow never emits a workflow file — like generate-k8s-manifests it
  // returns a nextAction instruction (prose + pinned YAML snippets) that the client LLM turns
  // into YAML. So lockstep is asserted in three parts rather than by diffing one artifact.

  describe('generator <-> validator lockstep', () => {
    /** Every pinned action, indexed by lower-cased `owner/repo`. */
    const PINS_BY_REF = new Map(Object.values(ACTION_PINS).map((p) => [p.ref.toLowerCase(), p]));

    // (1) The known-good fixture must stay derived from the pin registry. Without this,
    // scripts/refresh-action-pins.ts bumps a SHA on its weekly run and the fixture silently
    // keeps asserting a workflow CA no longer emits — the suite stays green either way.
    it('happy fixture pins are exactly the ACTION_PINS entries', () => {
      const refs = extractUsesRefs(happy('deploy'));
      expect(refs.length).toBeGreaterThan(0);

      for (const ref of refs) {
        const pin = PINS_BY_REF.get(ref.ownerRepo.toLowerCase());
        expect(pin).toBeDefined();
        // Compare the rendered `owner/repo@sha # vX.Y.Z` so a stale SHA *or* a stale version
        // comment both fail.
        expect(`${ref.raw} # ${ref.comment}`).toBe(pinnedUses(pin!));
      }
    });

    // (2) Assemble a workflow from the generator's own output and run it through the
    // validator. The four fenced snippets are the drift-prone parts the generator itself
    // marks "copy verbatim"; the scaffold below stands in for the client LLM.
    //
    // Extraction is structural, not byte-exact: line endings are normalized and trailing
    // whitespace after the ```yaml info string is tolerated. Otherwise a formatting-only
    // change to the generator (CRLF, a stray space) would fail a test that is meant to be
    // about the *contract*, and the failure would look like a real contract break.
    const normalizeEol = (s: string): string => s.replace(/\r\n?/g, '\n');

    const yamlBlocks = (instruction: string): string[] =>
      [...normalizeEol(instruction).matchAll(/```[ \t]*yaml[ \t]*\n([\s\S]*?)```/g)].map((m) =>
        m[1]!.replace(/\s+$/, ''),
      );

    /** Lines of an instruction section, up to the first blank line. */
    const section = (instruction: string, heading: string): string[] => {
      const lines = normalizeEol(instruction).split('\n');
      const start = lines.findIndex((l) => l.trim() === heading);
      if (start === -1) return [];
      const out: string[] = [];
      for (let i = start + 1; i < lines.length && lines[i]!.trim() !== ''; i++) out.push(lines[i]!);
      return out;
    };

    function assembleWorkflow(plan: GithubWorkflowPlan): string {
      const { instruction } = plan.nextAction;
      const blocks = yamlBlocks(instruction);
      expect(blocks).toHaveLength(4);
      const [acrBuild, aksContext, deployStep, annotate] = blocks as [
        string,
        string,
        string,
        string,
      ];

      // The checkout + login steps are described in prose, not YAML, so they are rebuilt here
      // from the same pin registry the generator renders them from.
      const login = [
        '      - name: Azure login',
        `        uses: ${pinnedUses(ACTION_PINS.azureLogin)}`,
        '        with:',
        '          client-id: ${{ secrets.AZURE_CLIENT_ID }}',
        '          tenant-id: ${{ secrets.AZURE_TENANT_ID }}',
        '          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}',
      ];

      return [
        'name: deploy',
        'on:',
        '  push:',
        '    branches: [main]',
        '  workflow_dispatch:',
        'concurrency:',
        '  group: ${{ github.workflow }}-${{ github.ref }}',
        '  cancel-in-progress: true',
        'env:',
        ...section(instruction, '## Workflow-level env variables'),
        'jobs:',
        `  ${JOB_KEYS.BUILD}:`,
        '    runs-on: ubuntu-latest',
        '    permissions:',
        '      contents: read',
        '      id-token: write',
        '    steps:',
        `      - uses: ${pinnedUses(ACTION_PINS.checkout)}`,
        ...login,
        acrBuild,
        `  ${JOB_KEYS.DEPLOY}:`,
        `    needs: [${JOB_KEYS.BUILD}]`,
        '    runs-on: ubuntu-latest',
        '    permissions:',
        '      actions: read',
        '      contents: read',
        '      id-token: write',
        '    steps:',
        `      - uses: ${pinnedUses(ACTION_PINS.checkout)}`,
        ...login,
        aksContext,
        deployStep,
        annotate,
      ].join('\n');
    }

    it.each(['k8s', 'helm'] as const)(
      'a workflow built from the generator output passes validation (%s)',
      async (manifestFormat) => {
        const generated = await generateGithubWorkflowTool.handler(
          {
            repositoryPath: '/home/user/myapp',
            registry: 'myregistry.azurecr.io',
            clusterName: 'my-aks',
            resourceGroup: 'my-rg',
            branches: ['main'],
            manifestFormat,
          } as any,
          createMockToolContext(),
        );
        expect(generated.ok).toBe(true);
        if (!generated.ok) return;

        const plan = await validate({
          workflowContent: assembleWorkflow(generated.value),
          manifestFormat,
        });

        // The generator must not produce anything its own validator objects to.
        // Asserting only `errors === 0` would be far too weak: most CA-contract rules are
        // `high` severity, which maps to WARNING, so a badly assembled workflow could still
        // report zero errors. Require the contract layers to be completely silent.
        expect(errorRuleIds(plan)).toEqual([]);
        expect(plan.report.errors).toBe(0);
        expect(plan.nextAction).toBeUndefined();

        const contractFindings = plan.report.results.filter(
          (r) => r.ruleId?.startsWith('semantic/') || r.ruleId?.startsWith('refs/'),
        );
        expect(contractFindings.map((r) => `${r.ruleId}: ${r.message}`)).toEqual([]);
        expect(plan.report.grade).toBe('A');
      },
    );

    // Guards the snippet extractor itself: a CRLF instruction must still yield the same four
    // blocks. Without this, the tolerance above is an untested claim, and a future generator
    // change to line endings would surface as a confusing "contract broken" failure.
    it('extracts the generator snippets regardless of line endings', async () => {
      const generated = await generateGithubWorkflowTool.handler(
        {
          repositoryPath: '/home/user/myapp',
          registry: 'myregistry.azurecr.io',
          clusterName: 'my-aks',
          resourceGroup: 'my-rg',
          branches: ['main'],
        } as any,
        createMockToolContext(),
      );
      expect(generated.ok).toBe(true);
      if (!generated.ok) return;

      const crlf: GithubWorkflowPlan = {
        ...generated.value,
        nextAction: {
          ...generated.value.nextAction,
          instruction: generated.value.nextAction.instruction.replace(/\n/g, '\r\n'),
        },
      };

      const plan = await validate({ workflowContent: assembleWorkflow(crlf) });
      expect(plan.report.errors).toBe(0);
      expect(plan.report.grade).toBe('A');
    });
  });

  // ── Finding positions ─────────────────────────────────────────────
  // Layers 2 and 4 reason over `doc.toJS()`, which discards node positions; they recover a
  // line by re-reading the parsed Document at the same path. These tests pin the mapping,
  // since an off-by-one would otherwise go unnoticed.

  describe('Finding positions', () => {
    /** Build a workflow where the 1-based array index equals the YAML line number. */
    const numbered = (lines: string[]): string => lines.join('\n');

    const lineOf = (plan: WorkflowValidationPlan, ruleId: string): number | undefined =>
      plan.report.results.find((r) => r.ruleId === ruleId)?.line;

    it('points semantic/no-job-environment at the `environment:` key', async () => {
      const plan = await validate({
        workflowContent: numbered([
          'on: { push: { branches: [main] } }', // 1
          'jobs:', //                              2
          '  buildImage:', //                      3
          '    runs-on: ubuntu-latest', //         4
          '    environment: production', //        5
          '    steps:', //                         6
          '      - run: az acr build .', //        7
        ]),
      });
      expect(lineOf(plan, 'semantic/no-job-environment')).toBe(5);
    });

    it('points a forbidden build method at the offending step', async () => {
      const plan = await validate({
        workflowContent: numbered([
          'on: { push: { branches: [main] } }', // 1
          'jobs:', //                              2
          '  buildImage:', //                      3
          '    runs-on: ubuntu-latest', //         4
          '    steps:', //                         5
          '      - run: az acr build .', //        6
          '      - name: Bad', //                  7
          '        run: docker build -t x .', //   8
        ]),
      });
      const acr = plan.report.results.find(
        (r) => r.ruleId === 'semantic/az-acr-build' && r.message?.includes('docker build'),
      );
      // The step node begins at its first key (`name:` on line 7), not at the `run:` line.
      expect(acr?.line).toBe(7);
      expect(acr?.metadata?.location).toBe('job "buildImage"');
    });

    it('points schema/unknown-job-key at the offending key', async () => {
      const plan = await validate({
        workflowContent: numbered([
          'on: { push: { branches: [main] } }', // 1
          'jobs:', //                              2
          '  buildImage:', //                      3
          '    runs-on: ubuntu-latest', //         4
          '    bogusKey: nope', //                 5
          '    steps:', //                         6
          '      - run: az acr build .', //        7
        ]),
      });
      expect(lineOf(plan, 'schema/unknown-job-key')).toBe(5);
    });

    // Boundary: the first line. An offset->line index that isn't seeded with the start of
    // line 1 reports 0 here rather than 1.
    it('reports line 1 for a finding on the first line', async () => {
      const plan = await validate({
        workflowContent: numbered([
          'bogusTopLevel: nope', //                1
          'on: { push: { branches: [main] } }', // 2
          'jobs:', //                              3
          '  buildImage:', //                      4
          '    runs-on: ubuntu-latest', //         5
          '    steps:', //                         6
          '      - run: az acr build .', //        7
        ]),
      });
      expect(lineOf(plan, 'schema/unknown-workflow-key')).toBe(1);
    });

    it('omits the line for findings about absent structure', async () => {
      const plan = await validate({
        workflowContent: numbered(['jobs:', '  buildImage:', '    runs-on: ubuntu-latest']),
      });
      // There is no `on:` block to point at, so the finding carries no position.
      const missingOn = plan.report.results.find((r) => r.ruleId === 'schema/missing-on');
      expect(missingOn).toBeDefined();
      expect(missingOn?.line).toBeUndefined();
    });

    it('surfaces the line in the fix-files instruction', async () => {
      const plan = await validate({
        workflowContent: numbered([
          'on: { push: { branches: [main] } }', // 1
          'jobs:', //                              2
          '  buildImage:', //                      3
          '    runs-on: ubuntu-latest', //         4
          '    environment: production', //        5
          '    steps:', //                         6
          '      - run: az acr build .', //        7
        ]),
      });
      expect(plan.nextAction?.instruction).toContain('line 5');
    });

    // `line` is the where, `metadata.location` the what. If a layer also encodes the line
    // into `location`, consumers that join the two render "(line 12, line 12)".
    it('never encodes a line number in metadata.location', async () => {
      const plan = await validate({
        workflowContent: [
          'on: { push: { branches: [main] } }',
          'jobs:',
          '  buildImage:',
          '    runs-on: ubuntu-latest',
          '    steps:',
          '      - uses: actions/checkout@v4', // unpinned -> refs/sha-pin (Layer 3)
          '      - run: docker build -t x .', // -> semantic/az-acr-build (Layer 4)
        ].join('\n'),
      });

      const located = plan.report.results.filter((r) => r.metadata?.location);
      expect(located.length).toBeGreaterThan(0);
      for (const r of located) {
        expect(r.metadata?.location).not.toMatch(/\bline\s+\d+/i);
      }

      // Layer 3 describes the subject instead, and the line survives on its own field.
      const shaPin = plan.report.results.find((r) => r.ruleId === 'refs/sha-pin');
      expect(shaPin?.metadata?.location).toBe('uses: actions/checkout');
      expect(shaPin?.line).toBe(6);

      // ...and the rendered instruction mentions that line exactly once.
      const rendered = plan.nextAction?.instruction ?? '';
      expect(rendered.match(/line 6\b/g)).toHaveLength(1);
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

    it('targets the caller workflowFileName in the fix-files path for inline content', async () => {
      const plan = await validate({
        workflowContent: sad('unpinned'),
        workflowFileName: 'ci.yml',
      });
      expect(plan.nextAction?.action).toBe('fix-files');
      expect(plan.nextAction?.files[0]?.path).toBe('.github/workflows/ci.yml');
      expect(plan.nextAction?.instruction).toContain('.github/workflows/ci.yml');
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
