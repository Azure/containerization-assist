/**
 * SHA-pinned GitHub Action references — single source of truth.
 *
 * Lives in `shared/` because three consumers depend on it and none owns it:
 * `generate-github-workflow` renders `uses:` values from it, `workflow-contract.ts` derives
 * the required/forbidden action identities from it, and `scripts/refresh-action-pins.ts`
 * rewrites it. It previously sat under `generate-github-workflow/`, which forced
 * `shared/workflow-contract.ts` to import *upward* into a specific tool — an inverted
 * dependency that also risked a cycle.
 *
 * Every `uses:` the generate-github-workflow tool emits points at an immutable commit
 * SHA (validate-github-workflow enforces SHA-pinning as a `required` check) with a
 * trailing `# vX.Y.Z` comment for readability (an advisory `low`-severity check —
 * `refs/version-comment` — not required). Pinning to a reviewed SHA — rather than a
 * floating tag that can be repointed — is the recommended supply-chain posture, and
 * mirrors how prepare-cluster pins its `kind` version + node-image digest.
 *
 * These pins are kept fresh by `scripts/refresh-action-pins.ts`, which resolves the
 * latest commit SHA within each action's current major from GitHub and rewrites this
 * file (run periodically / in CI, landed via a reviewed PR). Do not hand-edit the SHAs
 * unless you have verified them (e.g. `gh api repos/<ref>/commits/<major> --jq .sha`).
 *
 * ⚠️ This module must stay dependency-free (no `@/` imports, no imports at all) so the
 * scripts that consume and rewrite it keep working under `tsx`, which does not resolve the
 * `@/` path alias. Adding an import here would also reopen the cycle risk above.
 */

export interface ActionPin {
  /** `owner/repo` of the action. */
  ref: string;
  /** Full 40-char commit SHA the action is pinned to. */
  sha: string;
  /** Human-readable version the SHA corresponds to (e.g. `v6.0.3`). */
  version: string;
}

export const ACTION_PINS = {
  checkout: {
    ref: 'actions/checkout',
    sha: 'df4cb1c069e1874edd31b4311f1884172cec0e10',
    version: 'v6.0.3',
  },
  azureLogin: {
    ref: 'azure/login',
    sha: '532459ea530d8321f2fb9bb10d1e0bcf23869a43',
    version: 'v3.0.0',
  },
  useKubelogin: {
    ref: 'azure/use-kubelogin',
    sha: '76597ae0fcbaace21b05e13a2cbf8daee2c6e820',
    version: 'v1.2',
  },
  aksSetContext: {
    ref: 'azure/aks-set-context',
    sha: '60623acbdcbbdcf799ad50a1adf8703874339f8b',
    version: 'v5.0.0',
  },
  k8sBake: {
    ref: 'azure/k8s-bake',
    sha: '0191a5ae5126cfe61885d9bd46511caa8e9a9550',
    version: 'v4.1.0',
  },
  k8sDeploy: {
    ref: 'Azure/k8s-deploy',
    sha: 'c7ebd0d5f39477a23f1b5dea0f52e6db04adf28e',
    version: 'v6.0.0',
  },
} as const satisfies Record<string, ActionPin>;

/** Render a SHA-pinned `uses:` value: `owner/repo@<sha> # vX.Y.Z`. */
export function pinnedUses(action: ActionPin): string {
  return `${action.ref}@${action.sha} # ${action.version}`;
}

/** Major tag (e.g. "v6") for an action version like "v6.0.3". */
export function actionMajor(version: string): string {
  return version.split('.')[0] ?? version;
}
