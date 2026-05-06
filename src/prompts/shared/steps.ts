/**
 * Shared step helpers and prompt assembly for dev-loop prompts.
 *
 * Each helper returns a { heading, body } pair for one workflow step.
 * `assemblePrompt` numbers them dynamically and joins everything.
 */

import { TOOL_NAME } from '@/tools';

export interface Step {
  heading: string;
  body: string;
}

// ---------------------------------------------------------------------------
// Shared constants and snippet helpers (deduped across steps)
// ---------------------------------------------------------------------------

/** ARM scope template for an AKS managed cluster (used in remediation messages). */
const CLUSTER_SCOPE_TEMPLATE =
  '/subscriptions/<subscriptionId>/resourceGroups/<resourceGroup>/providers/Microsoft.ContainerService/managedClusters/<clusterName>';

/** ARM sub-scope template for a single namespace within a managed cluster. */
const NAMESPACE_SCOPE_TEMPLATE = `${CLUSTER_SCOPE_TEMPLATE}/namespaces/<namespace>`;

/** Note appended to remediation messages on AKS Automatic clusters. */
const LOCAL_ACCOUNTS_NOTE =
  'Note: AKS Automatic disables local accounts; --admin kubeconfig is unavailable.';

/** Identity / privilege roles that imply `Microsoft.Authorization/roleAssignments/write`. */
const ROLE_ASSIGNMENT_WRITER_ROLES =
  '`Owner`, `User Access Administrator`, or `Role Based Access Control Administrator`';

/**
 * Standard picker block used by tenant / subscription / cluster / ACR pickers.
 * Keeps wording consistent and centralizes the free-text-fallback rule.
 */
function pickerBlock(opts: {
  defaultLabel: string;
  additionalLabel: string;
  freeTextLabel: string;
}): string {
  return [
    'Present a picker to the user (use `vscode_askQuestions` if available, otherwise plain text):',
    `   - **Default option (pre-selected):** ${opts.defaultLabel}`,
    `   - **Additional options:** ${opts.additionalLabel}`,
    `   - **Free-text input:** ${opts.freeTextLabel}`,
  ].join('\n');
}

/**
 * Snippet describing how to determine whether the caller can grant role
 * assignments themselves. Used by the control-plane and data-plane failure
 * branches in `aksRbacPreDeployCheck`.
 */
function canSelfRemediateProbeSnippet(): string {
  return [
    '   - **Determine whether the caller can self-remediate** (sets/refreshes `canSelfRemediate`):',
    '     - Run: `az rest --method post --url "https://management.azure.com<clusterScope>/providers/Microsoft.Authorization/permissions?api-version=2022-04-01"` and inspect `actions` / `notActions` for `Microsoft.Authorization/roleAssignments/write`.',
    `     - If that command fails or is unavailable, fall back to inspecting \`az role assignment list --assignee <callerPrincipalId> --scope <clusterScope> --include-inherited -o json\` for any of: ${ROLE_ASSIGNMENT_WRITER_ROLES}. If that listing also fails with \`AuthorizationFailed\`, set \`canSelfRemediate = false\`.`,
  ].join('\n');
}

/**
 * Render a Case A / Case B remediation block. The two branches differ only
 * in whether the caller is told to run the command themselves or to ask
 * an admin, plus the identifier hand-off list — everything else (failed
 * probe summary, role list, `az role assignment create` commands) is shared.
 */
function rbacRemediationBlock(opts: {
  /** One-line summary of what failed (e.g. "Cannot fetch kubeconfig…"). */
  failureSummary: string;
  /** Optional 1+ extra "Failed probes:" lines to inject in the diagnostic. */
  failedProbesLines?: string[];
  /** Missing roles, in `Role Name @ <scope>` format. */
  missingRoles: Array<{ role: string; scope: string; note?: string }>;
  /** Whether to append the AKS Automatic local-accounts note. */
  includeLocalAccountsNote?: boolean;
  /** Identifiers to include in Case B's hand-off block. */
  handOffIdentifiers: Array<{ label: string; placeholder: string }>;
}): string {
  const failureBlock = [
    `     ⚠️ ${opts.failureSummary}`,
    '     Caller: <userName> (principalId=<callerPrincipalId>, type=<userType>)',
    ...(opts.failedProbesLines ?? []),
  ].join('\n');

  const roleBullets = opts.missingRoles
    .map((r) => `       • ${r.role}  @ ${r.scope}${r.note ? `   (${r.note})` : ''}`)
    .join('\n');

  const azCommands = opts.missingRoles
    .map((r) =>
      [
        '       az role assignment create \\',
        `         --role "${r.role}" \\`,
        '         --assignee <callerPrincipalId> \\',
        `         --scope "${r.scope}"`,
      ].join('\n'),
    )
    .join('\n\n');

  const handOff = opts.handOffIdentifiers
    .map((id) => `       ${id.label.padEnd(20)} ${id.placeholder}`)
    .join('\n');

  const localNote = opts.includeLocalAccountsNote ? `     ${LOCAL_ACCOUNTS_NOTE}` : '';

  return [
    '   - **Branch the remediation message based on `canSelfRemediate`:**',
    '',
    '     **Case A — `canSelfRemediate === true` (caller can grant the role(s) themselves):**',
    '     ```',
    failureBlock,
    '     Missing role(s) — you have permission to grant these yourself:',
    roleBullets,
    '     Run the appropriate command(s):',
    azCommands,
    ...(localNote ? [localNote] : []),
    '     ```',
    '     Offer to run the command(s) for the user (with confirmation), then poll.',
    '',
    '     **Case B — `canSelfRemediate === false` (caller cannot grant role assignments):**',
    '     ```',
    failureBlock,
    '     Missing role(s):',
    roleBullets,
    '',
    '     You do **not** have `Microsoft.Authorization/roleAssignments/write` on this cluster, so you cannot grant these roles yourself.',
    `     Please ask an **Owner**, **User Access Administrator**, or **Role Based Access Control Administrator** on the cluster (or the parent resource group / subscription) to run:`,
    '',
    azCommands,
    '',
    '     Identifiers to share with the admin:',
    handOff,
    ...(localNote ? [localNote] : []),
    '     ```',
    '     Do NOT attempt `az role assignment create` yourself in this case — it will fail with `AuthorizationFailed`.',
  ].join('\n');
}

// ---------------------------------------------------------------------------
// Azure pre-flight steps (used by aks-loop, available to kind-loop)
// ---------------------------------------------------------------------------

/**
 * Pre-flight step: Azure tenant (directory) selection and validation.
 *
 * Many users have access to multiple Entra ID tenants. The active tenant
 * determines which subscriptions `az account list` returns and which identity
 * is used for AKS Azure RBAC checks. Pin the tenant **before** the
 * subscription so the rest of the loop operates on a deterministic identity.
 */
export function azureTenantStep(): Step {
  return {
    heading: 'Select and validate the Azure tenant (directory)',
    body: [
      '1. Run `az account show --query "{tenantId:tenantId, tenantDisplayName:tenantDisplayName, user:user.name}" -o json` to detect the **current active tenant** and signed-in user.',
      '2. Run `az account tenant list --query "[].{tenantId:tenantId, displayName:displayName, defaultDomain:defaultDomain}" -o json` to list all tenants the caller can access. (If unavailable on the installed `az` version, fall back to `az account list --query "[].{tenantId:tenantId, name:name}" -o json` deduped by `tenantId`.)',
      `3. ${pickerBlock({
        defaultLabel: 'the current tenant (`<displayName> (<tenantId>)`)',
        additionalLabel: 'other tenants from the list (if any)',
        freeTextLabel: 'allow the user to type a tenant ID or domain manually',
      })}`,
      '4. **If the selected tenant differs from the current active tenant**, re-authenticate against it:',
      '   - Run `az login --tenant "<selectedTenantId>"` (interactive). For non-interactive / CI contexts, instruct the user to run this themselves and re-invoke the prompt.',
      '   - On success, `az account show` should now report `tenantId == <selectedTenantId>`.',
      '5. **Validate** the selected tenant is active:',
      '   - Run `az account show --query tenantId -o tsv` and confirm it equals `<selectedTenantId>`.',
      '   - If it does not match, re-prompt or abort with a clear error.',
      '6. Record the selection as `tenantId` (used implicitly by all subsequent `az` commands and by Azure RBAC scope construction).',
      '7. Confirm to the user: "Using tenant **<displayName>** (`<tenantId>`)."',
    ].join('\n'),
  };
}

/**
 * Pre-flight step: Azure subscription selection and validation.
 *
 * Runs **after** `azureTenantStep()` so subscription enumeration is scoped to
 * the chosen tenant.
 */
export function azureSubscriptionStep(): Step {
  return {
    heading: 'Select and validate the Azure subscription',
    body: [
      '1. Run `az account show --query "{id:id, name:name, tenantId:tenantId}" -o json` to detect the **current default subscription** within the previously selected tenant.',
      '2. Run `az account list --query "[?state==\'Enabled\' && tenantId==\'<tenantId>\'].{id:id, name:name}" -o json` to list all enabled subscriptions in the selected tenant.',
      `3. ${pickerBlock({
        defaultLabel: 'the current subscription (`<name> (<id>)`)',
        additionalLabel: 'other subscriptions from the list (if any)',
        freeTextLabel: 'allow the user to type a subscription ID or name manually',
      })}`,
      '4. **Validate** the selected subscription:',
      '   - Run `az account show --subscription "<selected-id>" --query "{id:id, tenantId:tenantId}" -o json`',
      '   - Confirm `tenantId` matches the tenant selected in the previous step. If not, re-prompt — the subscription belongs to a different tenant.',
      '   - If the command fails, inform the user the subscription was not found or is not accessible, and re-prompt.',
      '5. **Set** the subscription as the active context: `az account set --subscription "<selected-id>"`',
      '6. Confirm to the user: "Using subscription **<name>** (`<id>`) in tenant `<tenantId>`."',
    ].join('\n'),
  };
}

/**
 * Pre-flight step: AKS cluster selection, identity capture, and metadata recording.
 *
 * Lists clusters, picks one, fetches metadata, captures the caller's identity.
 * **No `kubectl` calls** here — kubeconfig fetch and all kubectl probes
 * (`auth can-i`, `get nodes`, `get namespace`) are deferred to
 * `aksRbacPreDeployCheck()` so missing-RBAC failures surface as actionable
 * role-assignment errors rather than opaque `Forbidden` responses.
 */
export function aksClusterValidationStep(): Step {
  return {
    heading: 'Select and validate the AKS cluster (metadata only — no kubectl yet)',
    body: [
      '1. Run `az aks list -g <resourceGroup> --query "[].{name:name, sku:sku.name, fqdn:fqdn}" -o json`.',
      `2. ${pickerBlock({
        defaultLabel: 'the cluster name from the prompt arguments (if any), or the first cluster in the list',
        additionalLabel: 'other clusters in the resource group',
        freeTextLabel: 'allow the user to type a cluster name manually',
      })}`,
      '3. **Validate** the selected cluster exists:',
      '   - Run `az aks show -g <resourceGroup> -n "<selectedCluster>" --query "{name:name, fqdn:fqdn, sku:sku.name, azureRbac:aadProfile.enableAzureRbac, localAccountsDisabled:disableLocalAccounts}" -o json`.',
      '   - If the command fails, inform the user and re-prompt.',
      '4. **Record cluster metadata** for downstream steps:',
      '   - `isAutomatic` = (`sku` == `"Automatic"`) — affects manifest generation and RBAC strategy',
      '   - `isAzureRbac` = (`azureRbac` == `true`) — affects permission checks and `command invoke` guidance',
      '   - `localAccountsDisabled` = (`localAccountsDisabled` == `true`) — suppresses `--admin` kubeconfig suggestions',
      '5. **Capture caller identity** (used by the next step to build remediation messages):',
      '   - Run `az account show -o json` and record `subscriptionId` (`id`), `userName` (`user.name`), and `userType` (`user.type`).',
      '   - If `userType` == `"user"`: run `az ad signed-in-user show --query id -o tsv` → record as `callerPrincipalId`.',
      '   - If `userType` == `"servicePrincipal"`: run `az ad sp show --id <userName> --query id -o tsv` → record as `callerPrincipalId`.',
      '   - If neither lookup succeeds, fall back to `userName` as `callerPrincipalId`.',
      '6. **No kubectl calls in this step** — see `Verify effective access` next.',
    ].join('\n'),
  };
}

/**
 * Pre-flight step: Effective-access check for AKS Automatic / Azure-RBAC clusters.
 *
 * Runs immediately after `aksClusterValidationStep()`. Strict order:
 *   1. Compute scopes
 *   2. Control-plane probe (`az aks get-credentials`) — must be first; without
 *      a kubeconfig no kubectl works.
 *   3. Structured RBAC inspection (`az role assignment list` + write-capability
 *      probe) — gives precise, actionable remediation when readable.
 *   4. Data-plane RBAC probes (`kubectl auth can-i`) — definitive truth source.
 *   5. Connectivity / inventory (`kubectl get nodes`, `kubectl get namespace`)
 *      — only after RBAC is confirmed, so failures here are network/cluster
 *      issues rather than authorization issues.
 *   6. Confirmation.
 *
 * Both control-plane and data-plane failure branches share a single
 * `rbacRemediationBlock` helper so Case A / Case B logic isn't duplicated.
 *
 * On AKS Standard *without* Azure RBAC, this step is a no-op.
 */
export function aksRbacPreDeployCheck(): Step {
  return {
    heading: 'Verify effective access (AKS Automatic / Azure RBAC)',
    body: [
      '**Skip this step entirely** if `isAutomatic !== true` AND `isAzureRbac !== true`.',
      'For all other AKS clusters (Automatic, or Standard with Azure RBAC enabled), perform the checks below.',
      '',
      '1. **Compute scopes** for diagnostics and remediation:',
      `   - \`clusterScope\` = \`${CLUSTER_SCOPE_TEMPLATE}\``,
      `   - \`namespaceScope\` = \`${NAMESPACE_SCOPE_TEMPLATE}\``,
      '',
      '2. **Control-plane probe** — fetch kubeconfig:',
      '   - Run `az aks get-credentials -g <resourceGroup> -n "<clusterName>" --overwrite-existing` (omit `--admin`; AKS Automatic disables local accounts).',
      '   - If this command **fails** (non-zero exit), the caller likely lacks the cluster-level role required to obtain a kubeconfig. **HALT** the workflow.',
      canSelfRemediateProbeSnippet(),
      rbacRemediationBlock({
        failureSummary: 'Cannot fetch kubeconfig for AKS [Automatic|Azure-RBAC] cluster <clusterName>.',
        missingRoles: [
          { role: 'Azure Kubernetes Service Cluster User Role', scope: '<clusterScope>' },
        ],
        includeLocalAccountsNote: true,
        handOffIdentifiers: [
          { label: 'Caller principal id:', placeholder: '<callerPrincipalId>' },
          { label: 'Caller type:', placeholder: '<userType>' },
          { label: 'Cluster scope:', placeholder: '<clusterScope>' },
        ],
      }),
      '',
      '   - In **either** case, **wait for the user** to confirm the role has been granted, then **poll** by re-running `az aks get-credentials` every **15 seconds for up to 5 minutes** until it succeeds.',
      '   - If polling exceeds the timeout, report the timeout to the user and stop.',
      '',
      '3. **Structured RBAC role-assignment inspection** (best-effort precision; data-plane probes in Step 4 are still the gate):',
      '   - List the caller\'s role assignments at both scopes:',
      '     - `az role assignment list --assignee <callerPrincipalId> --scope <clusterScope> --include-inherited -o json`',
      '     - `az role assignment list --assignee <callerPrincipalId> --scope <namespaceScope> --include-inherited -o json`',
      '   - Bucket the result as `rbacListReadable` / `rbacListUnauthorized` / `rbacListEmpty`.',
      '   - Refresh `canSelfRemediate` using the same probe defined in Step 2.',
      '   - **Sufficient roles** (any one is enough; proceed to Step 4 without halting):',
      '     - At `<clusterScope>`: `Azure Kubernetes Service RBAC Cluster Admin`, `Azure Kubernetes Service RBAC Admin`, OR (`Azure Kubernetes Service Cluster User Role` + `Azure Kubernetes Service RBAC Writer` at `<namespaceScope>`)',
      '     - At `<namespaceScope>`: `Azure Kubernetes Service RBAC Writer` OR `RBAC Admin`',
      '   - If `rbacListReadable` and the caller has none of the sufficient role combinations, surface the gap immediately using the Step 4 remediation block.',
      '   - If `rbacListUnauthorized`, log: "ℹ️ Cannot read your role assignments — falling back to effective-access probes."',
      '',
      '4. **Data-plane RBAC probes** via `kubectl auth can-i` (run BEFORE any read calls so missing permissions surface as actionable role errors, not opaque `Forbidden`):',
      '   - Probe the verbs needed for deployment (assume namespace may not exist yet — we have not listed it):',
      '     - `kubectl auth can-i create namespaces` (cluster-scoped)',
      '     - `kubectl auth can-i create deployments  -n <namespace>`',
      '     - `kubectl auth can-i create services     -n <namespace>`',
      '     - `kubectl auth can-i create configmaps   -n <namespace>`',
      '     - `kubectl auth can-i get    pods         -n <namespace>`',
      '   - Note: `kubectl auth can-i` against a non-existent namespace works — the API server answers hypothetically.',
      '   - If `can-i create namespaces` returns `no` AND all namespace-scoped probes return `yes`, that is acceptable **only if** an admin will pre-create the namespace. Surface this to the user and continue.',
      '   - If **any** namespace-scoped probe returns `no` (or Step 3 already detected missing roles), **HALT** the workflow.',
      rbacRemediationBlock({
        failureSummary:
          'Insufficient Kubernetes data-plane permissions on AKS [Automatic|Azure-RBAC] cluster <clusterName>.',
        failedProbesLines: [
          '     Failed probes: <list of failed `kubectl auth can-i` checks>',
          '     Current role assignments visible to you: <JSON from Step 3 if rbacListReadable, otherwise "(unknown)">',
        ],
        missingRoles: [
          {
            role: 'Azure Kubernetes Service RBAC Writer',
            scope: '<namespaceScope>',
            note: 'for deploy/services/configmaps in this namespace',
          },
          {
            role: 'Azure Kubernetes Service RBAC Admin',
            scope: '<clusterScope>',
            note: 'additionally, if the namespace must be created',
          },
        ],
        handOffIdentifiers: [
          { label: 'Caller principal id:', placeholder: '<callerPrincipalId>' },
          { label: 'Caller type:', placeholder: '<userType>' },
          { label: 'Cluster scope:', placeholder: '<clusterScope>' },
          { label: 'Namespace scope:', placeholder: '<namespaceScope>' },
        ],
      }),
      '',
      '   - **In both cases**, **wait for the user** to confirm the roles have been granted, then **poll** the failed `kubectl auth can-i` probes every **10 seconds for up to 2 minutes** until all return `yes`.',
      '   - If polling exceeds the timeout, report the timeout and stop. Do not proceed.',
      '',
      '5. **Verify cluster connectivity** (only after RBAC probes have all passed — this confirms the API server is actually reachable, not just authorized):',
      "   - Compare `fqdn` from the prior `az aks show` to the hostname portion of `kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}'` (parse the server URL first). Ignore expected `https://` and default `:443` differences; the hostnames should match.",
      '   - Run `kubectl get nodes --no-headers` to confirm API server reachability and that nodes are listed.',
      '   - Check whether the target namespace already exists: `kubectl get namespace <namespace> --no-headers --ignore-not-found`. Record `namespaceExists` for downstream steps.',
      '   - If `namespaceExists === false` and the earlier `can-i create namespaces` probe returned `no`, **HALT** here and ask the user (or an admin) to either pre-create the namespace or grant `RBAC Admin` at cluster scope; do not proceed.',
      '',
      '6. On all probes passing, confirm: "✅ Effective access verified (RBAC + control plane + data plane) for caller <userName> on cluster <clusterName>. Proceeding."',
    ].join('\n'),
  };
}

/**
 * Pre-flight step: ACR selection and validation.
 *
 * Lists registries in the subscription, presents a picker, validates
 * the registry exists and the AKS cluster can pull from it.
 */
export function acrValidationStep(): Step {
  return {
    heading: 'Select and validate the Azure Container Registry',
    body: [
      '1. Run `az acr list --query "[].{name:name, loginServer:loginServer}" -o json` to list all ACRs in the subscription.',
      `2. ${pickerBlock({
        defaultLabel:
          'the registry from the prompt arguments (if any, matched by login server), or the first ACR in the list',
        additionalLabel: 'other ACRs in the subscription',
        freeTextLabel:
          'allow the user to type a registry login server manually (e.g., `myregistry.azurecr.io`)',
      })}`,
      '   - Normalize the selection so you always retain both values: `<acrName>` and `<loginServer>`.',
      '   - If the user selected an item from `az acr list`, use its `name` as `<acrName>` and its `loginServer` as `<loginServer>`.',
      '   - If the user typed a login server manually, derive `<acrName>` from it (for example, strip `.azurecr.io`) or look it up from the `az acr list` results by matching `loginServer`.',
      '3. **Validate** the selected ACR exists:',
      '   - Run `az acr show --name "<acrName>" --query "{name:name, loginServer:loginServer}" -o json`',
      '   - If the command fails, inform the user and re-prompt.',
      '4. **Validate AKS→ACR pull authorization**:',
      '   - Run `az aks check-acr -g <resourceGroup> -n <clusterName> --acr <loginServer>`',
      '   - If the check **fails**, attempt to attach: `az aks update -g <resourceGroup> -n <clusterName> --attach-acr <acrName>`',
      '   - If attach **also fails** (e.g., `AuthorizationFailed`), inform the user:',
      '     > "The AKS cluster cannot pull from this ACR. Attaching requires **Owner** or **User Access Administrator** role on the ACR resource (not just the cluster). Ask someone with that role to run: `az aks update -g <rg> -n <cluster> --attach-acr <acrName>`"',
      '   - Do NOT proceed past this step until ACR pull is confirmed.',
      '5. Confirm to the user: "Using ACR **<loginServer>** (`<acrName>`). Cluster pull access verified."',
    ].join('\n'),
  };
}

// ---------------------------------------------------------------------------
// Kind kubeconfig validation
// ---------------------------------------------------------------------------

export function kindContextValidationStep(): Step {
  return {
    heading: 'Validate kubeconfig context targets the Kind cluster',
    body: [
      '1. Run `kubectl config current-context` to get the active context name.',
      '2. The expected context for a containerization-assist Kind cluster is `kind-containerization-assist`.',
      '3. If the current context does **not** match:',
      '   - Run `kind get clusters` to check if the `containerization-assist` cluster exists.',
      '   - If it exists, switch context: `kubectl config use-context kind-containerization-assist`',
      '   - If it does not exist, note this — the prepare-cluster step will create it.',
      '4. If the context matches, verify connectivity: `kubectl get nodes --no-headers`.',
      '   - If this fails, the cluster may be stopped or deleted — the prepare-cluster step will handle recreation.',
    ].join('\n'),
  };
}

// ---------------------------------------------------------------------------
// Shared steps (identical across loops)
// ---------------------------------------------------------------------------

export function analyzeStep(): Step {
  return {
    heading: 'Analyze the repository',
    body: [
      `Call **${TOOL_NAME.ANALYZE_REPO}** using the current working directory as the repository path.`,
      '- Confirm the detected repository, language, framework, modules, and existing Dockerfiles with the user before proceeding.',
      '- If the repository is a monorepo, list all independently deployable modules and ask the user which ones to target.',
    ].join('\n'),
  };
}

export function generateDockerfileStep(): Step {
  return {
    heading: 'Generate Dockerfile (if missing)',
    body: [
      'If no Dockerfile exists for the target module(s):',
      `1. Call **${TOOL_NAME.GENERATE_DOCKERFILE}** with the repository path and analysis context.`,
      "2. Follow the tool's guidance to create the Dockerfile(s) on disk.",
      '3. Retry up to **2 times** if generation fails.',
    ].join('\n'),
  };
}

export function scanStep(): Step {
  return {
    heading: 'Scan the image',
    body: [
      `1. Call **${TOOL_NAME.SCAN_IMAGE}** with the built image ID.`,
      `2. Review vulnerabilities. If critical/high issues are found, call **${TOOL_NAME.FIX_DOCKERFILE}** and rebuild. Retry up to **2 times**.`,
    ].join('\n'),
  };
}

export function deployStep(target: string): Step {
  return {
    heading: `Deploy to ${target}`,
    body: [
      '1. Ensure the namespace exists (idempotent): `kubectl create namespace <namespace> --dry-run=client -o yaml | kubectl apply -f -`',
      '2. Apply the generated manifests: `kubectl apply -f <manifest-folder> --namespace <namespace>`.',
      '3. Retry up to **2 times** on failure.',
    ].join('\n'),
  };
}

export function verifyStep(extraLines?: string[]): Step {
  const lines = [
    `1. Call **${TOOL_NAME.VERIFY_DEPLOY}** with the namespace to check pod status, readiness, and events.`,
    '2. If verification fails, inspect pod logs and events, fix issues, and re-deploy. Retry up to **2 times**.',
  ];
  if (extraLines) {
    lines.push(...extraLines);
  }
  return {
    heading: 'Verify the deployment',
    body: lines.join('\n'),
  };
}

export function databaseCheckStep(): Step {
  return {
    heading: 'Check for database dependencies',
    body: [
      'Review the `modules[].detectedDatabases` field in the analyze-repo results for each module.',
      '- For each module where `detectedDatabases` is non-empty, ask the user:',
      '  1. Do these databases exist as Azure PaaS services (e.g., Azure Database for PostgreSQL, Azure Cache for Redis)?',
      '  2. If yes, collect the server hostname(s) and database name(s) for the affected module(s).',
      '  3. Confirm the managed identity client ID to use for workload identity authentication.',
      '- If no modules have any detected databases, skip this step.',
    ].join('\n'),
  };
}

export function envVarCheckStep(): Step {
  return {
    heading: 'Check for detected environment variables',
    body: [
      'Review the `modules[].detectedEnvVars` field in the analyze-repo results for each module.',
      '- For each module where `detectedEnvVars` is non-empty, ask the user:',
      '  1. Confirm the classifications (secret, database, config) are correct.',
      '  2. For secret-classified vars, confirm they will be injected at runtime (not baked into the image).',
      '  3. For config-classified vars, confirm default values or ask for correct values.',
      `- Pass the confirmed \`detectedEnvVars\` to downstream tools (**${TOOL_NAME.GENERATE_DOCKERFILE}**, **${TOOL_NAME.GENERATE_K8S_MANIFESTS}**).`,
      '- If no modules have any detected environment variables, skip this step.',
    ].join('\n'),
  };
}

// ---------------------------------------------------------------------------
// Shared rules
// ---------------------------------------------------------------------------

export const sharedRules = [
  '- **Retry failed steps at least 2 times** before reporting failure.',
  '- **Follow the chain hints** returned by each tool to determine next steps.',
  '- If a tool suggests calling another tool, follow that suggestion.',
  '- Keep the user informed of progress at each step.',
  '- If all retry attempts for a step are exhausted, report the failure clearly with diagnostic details.',
];

// ---------------------------------------------------------------------------
// Prompt assembly
// ---------------------------------------------------------------------------

/**
 * Assemble a complete loop prompt with dynamically numbered steps.
 */
export function assemblePrompt(
  title: string,
  contextLines: string[],
  steps: Step[],
  rules: string[],
): string {
  const numberedSteps = steps
    .map((s, i) => `### Step ${i + 1} — ${s.heading}\n${s.body}`)
    .join('\n\n');

  return `You are driving a **${title}** using the containerization-assist MCP server tools.

## Context
${contextLines.join('\n')}

## Workflow — follow each step in order

${numberedSteps}

## Important rules
${rules.join('\n')}`;
}
