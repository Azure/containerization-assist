/**
 * CA deploy-workflow contract — single source of truth.
 *
 * `generate-github-workflow` *states* these rules in the instruction it hands the client
 * LLM; `validate-github-workflow` *enforces* them in its Layer-4 semantic checks. Both
 * import from this module so the two cannot drift.
 *
 * Before this module existed the same facts were retyped in four places, and had already
 * diverged: the generator's CRITICAL RULE 2 listed `docker build` but omitted
 * `docker/login-action`, while its "Do NOT substitute" line 40 lines later did the exact
 * reverse. Anything both tools must agree on belongs here.
 *
 * Action *identities* are derived from `ACTION_PINS` rather than re-declared, so adding or
 * renaming a pinned action updates the generator and the validator together. This module is
 * the semantic counterpart to that registry: `ACTION_PINS` owns which SHA an action is
 * pinned to, this file owns which actions are required, forbidden, and how jobs are shaped.
 * Both live in `shared/` — neither the generator nor the validator owns them.
 */

import { visit, isScalar, type Document } from 'yaml';
import { ACTION_PINS } from './action-pins';

// ─── Job shape ────────────────────────────────────────────────────────────────

/**
 * The literal job keys the workflow must use. Renaming these breaks the CA contract —
 * downstream tooling and the validator both key off them.
 */
export const JOB_KEYS = {
  BUILD: 'buildImage',
  DEPLOY: 'deploy',
} as const;

/** GitHub repository secrets the OIDC login requires. */
export const REQUIRED_SECRETS = [
  'AZURE_CLIENT_ID',
  'AZURE_TENANT_ID',
  'AZURE_SUBSCRIPTION_ID',
] as const;

// ─── Image build ──────────────────────────────────────────────────────────────

/** The only sanctioned build method — runs the build in Azure, not on the runner. */
export const BUILD_COMMAND = 'az acr build';

/** Matches {@link BUILD_COMMAND} inside a step's `run:` script. */
export const BUILD_COMMAND_RE = /\baz\s+acr\s+build\b/;

/** Forbidden `uses:` action refs for building the image. */
export const FORBIDDEN_BUILD_ACTIONS = [
  'docker/build-push-action',
  'docker/setup-buildx-action',
  'docker/login-action',
] as const;

/**
 * Forbidden `run:` commands for building the image. `docker build` uses a negative
 * lookahead so it does not also match `docker buildx`, which is reported separately.
 */
export const FORBIDDEN_BUILD_COMMANDS = [
  { label: 'docker buildx', pattern: /\bdocker\s+buildx\b/ },
  { label: 'docker build', pattern: /\bdocker\s+build(?!x)/ },
] as const;

// ─── Deploy / AKS access ──────────────────────────────────────────────────────

/** Forbidden `uses:` action refs for obtaining cluster access. */
export const FORBIDDEN_DEPLOY_ACTIONS = ['azure/setup-kubectl'] as const;

/** Forbidden `run:` commands for obtaining cluster access. */
export const FORBIDDEN_DEPLOY_COMMANDS = [
  { label: 'az aks get-credentials', pattern: /az\s+aks\s+get-credentials/ },
] as const;

/** OIDC login action — required in both jobs. */
export const LOGIN_ACTION = ACTION_PINS.azureLogin.ref;

/** Configures kubelogin for non-interactive AAD auth; required before setting the context. */
export const USE_KUBELOGIN_ACTION = ACTION_PINS.useKubelogin.ref;

/** Sets the kubectl context; its `with:` flags are checked against {@link AKS_CONTEXT_FLAGS}. */
export const AKS_SET_CONTEXT_ACTION = ACTION_PINS.aksSetContext.ref;

/** Renders Helm/Kustomize manifests; required in the deploy job for those formats. */
export const BAKE_ACTION = ACTION_PINS.k8sBake.ref;

/**
 * Actions the `deploy` job must use, in order. Derived from the pin registry so a renamed
 * or newly pinned action flows through to the validator automatically.
 */
export const REQUIRED_DEPLOY_ACTIONS = [
  ACTION_PINS.azureLogin.ref,
  ACTION_PINS.useKubelogin.ref,
  ACTION_PINS.aksSetContext.ref,
  ACTION_PINS.k8sDeploy.ref,
] as const;

/**
 * Required `with:` flags on the aks-set-context step. Quoted strings in YAML, so the
 * validator compares with `String(...)` to accept both `false` and `'false'`.
 */
export const AKS_CONTEXT_FLAGS = {
  admin: 'false',
  'use-kubelogin': 'true',
} as const;

// ─── Derived label lists (for human-readable instruction text) ────────────────

/** Every forbidden build method, actions and commands alike. */
export const FORBIDDEN_BUILD_LABELS: readonly string[] = [
  ...FORBIDDEN_BUILD_ACTIONS,
  ...FORBIDDEN_BUILD_COMMANDS.map((c) => c.label),
];

/** Every forbidden cluster-access method, actions and commands alike. */
export const FORBIDDEN_DEPLOY_LABELS: readonly string[] = [
  ...FORBIDDEN_DEPLOY_ACTIONS,
  ...FORBIDDEN_DEPLOY_COMMANDS.map((c) => c.label),
];

// ─── `uses:` grammar ──────────────────────────────────────────────────────
// Shared by the validator's Layer 3 and `scripts/validate-action-refs.ts`, so both agree on
// what an action reference is. Prefer {@link extractActionRefsFromDoc}; the regex below is a
// fallback for sources that cannot be parsed.

/**
 * Every pinnable `uses:` reference in a parsed workflow, with the source offset of each.
 *
 * Walks real mapping pairs, so it is agnostic to YAML style — block steps, inline maps
 * (`- { uses: x@sha }`) and flow sequences (`steps: [{ uses: x@sha }]`) are all found — and
 * `uses:` appearing inside a comment or a `run:` script is never matched. This is what a
 * SHA-pin check needs: a missed reference is a silent supply-chain hole, whereas the regex
 * fallback can only see line-anchored `uses:` keys.
 */
export function extractActionRefsFromDoc(doc: Document): DocActionRef[] {
  const refs: DocActionRef[] = [];
  visit(doc, {
    Pair(_, pair) {
      const key = pair.key;
      const value = pair.value;
      if (!isScalar(key) || key.value !== 'uses') return;
      if (!isScalar(value) || typeof value.value !== 'string') return;
      const offset = Array.isArray(value.range) ? value.range[0] : 0;
      const comment = typeof value.comment === 'string' ? value.comment.trim() : undefined;
      const parsed = parseActionRef(value.value, comment);
      if (parsed) refs.push({ ...parsed, offset });
    },
  });
  return refs;
}

/**
 * Fallback matcher for sources that could not be parsed.
 *
 * Anchored to the start of a YAML line (optional indentation, optional list `-`) so
 * commented-out lines (`# uses: ...`) and `uses:` text inside a `run:` script are ignored.
 * Group 1 = `owner/repo[/sub]@ref`; group 2 = the trailing `# comment`.
 *
 * **Known limitation:** being line-anchored, it does not match inline-map or flow-sequence
 * steps (`- { uses: x@sha }`). That is acceptable only as a fallback — callers that can parse
 * the document must use {@link extractActionRefsFromDoc} instead, or they will silently skip
 * those references.
 *
 * Stateful (`g` flag): build a fresh `new RegExp(USES_RE)` per scan rather than sharing
 * this instance's `lastIndex` across callers.
 */
export const USES_RE = /^[ \t]*-?[ \t]*uses:\s*([^#\s]+@[^#\s]+)\s*(?:#\s*(.+))?/gm;

/** The parts of a pinnable `uses:` reference. */
export interface ParsedActionRef {
  /** The raw `owner/repo[/sub]@ref` string, trimmed. */
  raw: string;
  /** `owner/repo` — the first two path segments, with any subpath dropped. */
  ownerRepo: string;
  /** The ref portion, after the last `@`. */
  ref: string;
  /** Trailing `# ...` comment, if any. */
  comment?: string;
}

/** A {@link ParsedActionRef} located in a parsed document. */
export interface DocActionRef extends ParsedActionRef {
  /** Character offset of the `uses:` value node, for resolving a line number. */
  offset: number;
}

/**
 * Parse a raw `uses:` value into its parts. Returns null for references that cannot be
 * SHA-pinned — local actions (`./...`) and container actions (`docker://...`) — and for
 * shapes that are not `owner/repo@ref` at all.
 */
export function parseActionRef(raw: string, comment?: string): ParsedActionRef | null {
  const trimmed = raw.trim();
  if (!trimmed || trimmed.startsWith('./') || trimmed.startsWith('docker://')) return null;

  const at = trimmed.lastIndexOf('@');
  if (at <= 0) return null;
  const actionPath = trimmed.slice(0, at);
  const ref = trimmed.slice(at + 1);

  const parts = actionPath.split('/');
  if (parts.length < 2) return null;

  return {
    raw: trimmed,
    ownerRepo: `${parts[0]}/${parts[1]}`,
    ref,
    ...(comment ? { comment } : {}),
  };
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

/**
 * Case-insensitive matcher for an action ref. Owner casing is inconsistent upstream
 * (`Azure/k8s-deploy` vs `azure/login`), so identity comparisons must ignore it.
 */
export function actionRefPattern(ref: string): RegExp {
  return new RegExp(ref.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i');
}

/**
 * Join items as prose — `a`, `a or b`, `a, b, or c`. Deriving messages from the full list
 * rather than indexing a known position keeps them correct if a list gains or loses entries.
 * Returns an empty string for an empty list.
 */
export function joinWithConjunction(
  items: readonly string[],
  conjunction: 'or' | 'and' = 'or',
): string {
  if (items.length <= 1) return items[0] ?? '';
  if (items.length === 2) return `${items[0]} ${conjunction} ${items[1]}`;
  return `${items.slice(0, -1).join(', ')}, ${conjunction} ${items[items.length - 1]}`;
}

/**
 * Render a list as backtick-quoted inline code for instruction prose:
 * `` `a`, `b`, or `c` ``.
 */
export function formatList(items: readonly string[], conjunction: 'or' | 'and' = 'or'): string {
  return joinWithConjunction(
    items.map((i) => `\`${i}\``),
    conjunction,
  );
}
