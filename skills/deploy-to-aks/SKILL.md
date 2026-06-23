---
name: deploy-to-aks
description: End-to-end orchestrator that takes a source repository through analysis → Dockerfile → image build → security scan → AKS cluster prep → ACR push → Kubernetes manifests → deploy → verify on a real **Azure AKS** cluster. Trigger ONLY when the user explicitly names Azure, AKS, or ACR — e.g. "deploy to AKS", "deploy to Azure", "ship to Azure", "push to ACR", "containerize and deploy to Azure", "AKS dev loop", "run this on AKS", "get this on AKS". Do **NOT** trigger on generic deployment phrasing like "deploy this app", "deploy to kubernetes", "deploy to k8s", "deploy to the cloud", "deploy to minikube", "deploy to kind", "deploy locally", or any other non-Azure target — those are not this skill. If the target is ambiguous, ask the user whether they mean AKS before starting. Required inputs: container registry login server (e.g. `myregistry.azurecr.io`), resource group, AKS cluster name. Optional: namespace, image name.
---

# Deploy to AKS

You are driving a **full repository → running pod on AKS** iteration loop. This skill **orchestrates** other skills — read each one before you run its step, pass results forward, and don't duplicate their logic here.

## Inputs

Collect (ask the user once if any are missing):

- `registry` (**required**) — ACR login server, e.g. `myregistry.azurecr.io`
- `resourceGroup` (**required**) — Azure RG containing the AKS cluster
- `clusterName` (**required**) — AKS cluster name
- `namespace` (optional) — defaults to a unique name like `staging-<short-hash>`
- `imageName` (optional) — defaults to the repo directory name
- `repoPath` — the **current working directory** (confirm with user)
- `targetPlatform` — fixed `linux/amd64` (standard AKS node arch)
- `environment` — fixed `production`

After collection, **echo back the resolved values** in one block and ask the user to confirm before running Step 1.

## Workflow

Run the steps strictly in order. **Retry each step up to 2 times** before halting. Keep the user informed after every step.

### Step 1 — Select and validate the Azure tenant

1. `az account show --query "{tenantId:tenantId, tenantDisplayName:tenantDisplayName, user:user.name}" -o json` → current tenant.
2. `az account tenant list --query "[].{tenantId:tenantId, displayName:displayName, defaultDomain:defaultDomain}" -o json` (fall back to `az account list ... | dedupe by tenantId` if unavailable).
3. Present a picker (default = current tenant; additional = other tenants; free-text fallback). Use `vscode_askQuestions` if available.
4. If the chosen tenant differs from the active one, run `az login --tenant "<selectedTenantId>"` and re-check.
5. Validate `az account show --query tenantId -o tsv` == `<selectedTenantId>`.
6. Record `tenantId`. Confirm: "Using tenant **<displayName>** (`<tenantId>`)."

### Step 2 — Select and validate the Azure subscription

1. `az account show --query "{id:id, name:name, tenantId:tenantId}" -o json`.
2. `az account list --query "[?state=='Enabled' && tenantId=='<tenantId>'].{id:id, name:name}" -o json`.
3. Picker (default = current sub; additional = other enabled subs in tenant; free-text fallback).
4. Validate with `az account show --subscription "<id>" ...` — `tenantId` must match Step 1.
5. `az account set --subscription "<id>"`. Confirm: "Using subscription **<name>** (`<id>`)."

### Step 3 — Select and validate the AKS cluster (metadata only — no kubectl yet)

1. `az aks list -g <resourceGroup> --query "[].{name:name, sku:sku.name, fqdn:fqdn}" -o json`.
2. Picker (default = `clusterName` from input or first listed; additional = other clusters in RG; free-text fallback).
3. Validate: `az aks show -g <resourceGroup> -n "<clusterName>" --query "{name:name, fqdn:fqdn, sku:sku.name, azureRbac:aadProfile.enableAzureRbac, localAccountsDisabled:disableLocalAccounts}" -o json`.
4. **Record metadata:**
   - `isAutomatic` = `sku == "Automatic"`
   - `isAzureRbac` = `azureRbac == true`
   - `localAccountsDisabled` = `localAccountsDisabled == true`
5. **Capture caller identity:**
   - `az account show -o json` → `subscriptionId`, `userName`, `userType`.
   - If `userType == "user"`: `az ad signed-in-user show --query id -o tsv` → `callerPrincipalId`.
   - If `userType == "servicePrincipal"`: `az ad sp show --id <userName> --query id -o tsv` → `callerPrincipalId`.
   - Else fall back to `userName` as `callerPrincipalId`.
6. **Do not** run any `kubectl` here. RBAC + kubeconfig fetch happen in Step 11.

### Step 4 — Select and validate the Azure Container Registry

1. `az acr list --query "[].{name:name, loginServer:loginServer}" -o json`.
2. Picker (default = the input `registry` matched by `loginServer`, or first ACR; free-text fallback).
3. Normalize so you retain **both** `<acrName>` and `<loginServer>`.
4. Validate: `az acr show --name "<acrName>" --query "{name:name, loginServer:loginServer}" -o json`.
5. **Validate AKS→ACR pull authorization:**
   - `az aks check-acr -g <resourceGroup> -n <clusterName> --acr <loginServer>`.
   - On failure, try `az aks update -g <resourceGroup> -n <clusterName> --attach-acr <acrName>`.
   - If attach also fails (e.g. `AuthorizationFailed`), tell the user:
     > "The AKS cluster cannot pull from this ACR. Attaching requires **Owner** or **User Access Administrator** on the ACR resource. Ask someone with that role to run: `az aks update -g <rg> -n <cluster> --attach-acr <acrName>`."
   - **Halt** until pull access is verified.
6. Confirm: "Using ACR **<loginServer>** (`<acrName>`). Cluster pull access verified."

### Step 5 — Analyze the repository

Follow the `analyze-repo` skill against the current working directory.

- Confirm the detected language, framework, modules, existing Dockerfiles, `detectedDatabases`, and `detectedEnvVars` with the user.
- For monorepos, list independently deployable modules and ask which one(s) to target.
- Forward the full analysis to every later step that needs it.

### Step 6 — Database dependency check

For each target module where `detectedDatabases` is non-empty, ask:

1. Do these exist as Azure PaaS services (Azure Database for PostgreSQL, Azure Cache for Redis, …)?
2. If yes, collect server hostname(s) and database name(s) per module.
3. Confirm the managed identity client ID for workload identity auth.

Skip if no module has detected databases.

### Step 7 — Environment variable check

For each target module where `detectedEnvVars` is non-empty, ask:

1. Confirm the `secret`/`database`/`config` classifications are correct.
2. For secret vars, confirm they will be injected at runtime (not baked into the image).
3. For config vars, confirm default values or ask for the correct ones.

Pass the confirmed `detectedEnvVars` to the Dockerfile and manifest generation steps.

Skip if no module has detected env vars.

### Step 8 — Generate or remediate the Dockerfile

For each target module:

- If **no Dockerfile exists**, follow the `generate-dockerfile` skill with the analysis context, environment `production`, and platform `linux/amd64`.
- If **a Dockerfile exists**, follow the `fix-dockerfile` skill against it; it writes the remediated file back to disk.

Retry up to 2 times if generation/remediation fails.

### Step 9 — Build the image

1. Call the **`build-image-context`** tool with:
   - `path` = repo (or module) path
   - `imageName` = `<imageName>:dev` (or an immutable tag like `<imageName>:git-<shortSha>`)
   - `platform` = `linux/amd64`
2. Execute the returned build command to produce the local Docker image.
3. On failure:
   - If the failure looks like a Dockerfile issue (syntax, missing files, base image not found, dependency error): follow the `fix-dockerfile` skill against the current Dockerfile, then retry.
   - Otherwise retry the build directly.
   - Maximum **2 retries** per failure mode.

### Step 10 — Scan the image

1. Call the **`scan-image`** tool with the built image ID.
2. Review vulnerabilities.
3. If any **critical** or **high** issues are reported: follow the `fix-dockerfile` skill using the scan results as context, then rebuild (Step 9) and rescan. Retry the fix/rebuild/scan loop **up to 2 times**.
4. Do not proceed past unresolved critical findings unless the user explicitly approves.

### Step 11 — Verify effective access (AKS Automatic / Azure RBAC)

**Skip this step entirely** if `isAutomatic !== true` AND `isAzureRbac !== true`.

Otherwise:

1. **Compute scopes:**
   - `clusterScope` = `/subscriptions/<subscriptionId>/resourceGroups/<resourceGroup>/providers/Microsoft.ContainerService/managedClusters/<clusterName>`
   - `namespaceScope` = `<clusterScope>/namespaces/<namespace>`

2. **Control-plane probe — fetch kubeconfig:**
   - `az aks get-credentials -g <resourceGroup> -n "<clusterName>" --overwrite-existing` (omit `--admin`; AKS Automatic disables local accounts).
   - On failure: **halt**, run the `canSelfRemediate` probe below, emit the remediation block (Case A or B) for missing role `Azure Kubernetes Service Cluster User Role @ <clusterScope>`, wait for the user, then poll `az aks get-credentials` every **15 s for up to 5 min**.

3. **`canSelfRemediate` probe (used by Steps 2 and 4 remediation):**
   - `az rest --method post --url "https://management.azure.com<clusterScope>/providers/Microsoft.Authorization/permissions?api-version=2022-04-01"` — inspect `actions` / `notActions` for `Microsoft.Authorization/roleAssignments/write`.
   - Fallback: `az role assignment list --assignee <callerPrincipalId> --scope <clusterScope> --include-inherited -o json` for any of `Owner`, `User Access Administrator`, `Role Based Access Control Administrator`. If that also fails with `AuthorizationFailed`, set `canSelfRemediate = false`.

4. **Data-plane probes** (run before any read calls):
   - `kubectl auth can-i create namespaces` (cluster-scoped)
   - `kubectl auth can-i create deployments -n <namespace>`
   - `kubectl auth can-i create services -n <namespace>`
   - `kubectl auth can-i create configmaps -n <namespace>`
   - `kubectl auth can-i get pods -n <namespace>`
   - If `create namespaces == no` but **all** namespace-scoped probes pass, surface this and continue **only if** an admin will pre-create the namespace.
   - If **any** namespace-scoped probe returns `no`: **halt**, emit the remediation block for missing roles:
     - `Azure Kubernetes Service RBAC Writer @ <namespaceScope>` (for deploy/services/configmaps)
     - `Azure Kubernetes Service RBAC Admin @ <clusterScope>` (additionally, if namespace must be created)
   - Wait for the user, then poll the failed `kubectl auth can-i` checks every **10 s for up to 2 min**.

5. **Connectivity sanity:** compare `fqdn` from Step 3 to the hostname in `kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}'`. Run `kubectl get nodes --no-headers`. Check `kubectl get namespace <namespace> --no-headers --ignore-not-found` and record `namespaceExists`.

6. Confirm: "✅ Effective access verified for caller `<userName>` on cluster `<clusterName>`. Proceeding."

**Remediation block format** (use for any HALT in Steps 2 and 4):

- **Case A — `canSelfRemediate === true`:** present a `⚠️` summary, the missing role(s), and one `az role assignment create --role "<Role>" --assignee <callerPrincipalId> --scope "<scope>"` command per role. Offer to run it (with confirmation), then poll.
- **Case B — `canSelfRemediate === false`:** present the same summary + roles, but tell the user to ask an **Owner**, **User Access Administrator**, or **Role Based Access Control Administrator** to run the same `az role assignment create` commands. Include hand-off identifiers (`callerPrincipalId`, `userType`, `clusterScope`, `namespaceScope`). Do **NOT** attempt the role assignment yourself.
- Append "Note: AKS Automatic disables local accounts; `--admin` kubeconfig is unavailable." when `localAccountsDisabled` or `isAutomatic` is true.

### Step 12 — Prepare the cluster

1. Call the **`prepare-cluster`** tool with:
   - `clusterType: "generic"` (assumes the existing AKS cluster)
   - `namespace`: `<namespace>`
   - `targetPlatform: "linux/amd64"`
2. Retry up to 2 times.
3. If kubeconfig is missing, run `az aks get-credentials --resource-group <resourceGroup> --name <clusterName>` first.

### Step 13 — Tag the image for ACR

1. Call the **`tag-image`** tool to produce `<loginServer>/<imageName>:<tag>`.
2. Retry up to 2 times.

### Step 14 — Push the image to ACR

1. Call the **`push-image`** tool to push the tagged image to `<loginServer>`.
2. Retry up to 2 times.
3. On auth failure, prompt the user to run `az acr login --name <acrName>` and retry.

### Step 15 — Generate Kubernetes manifests (if missing)

If no manifests exist for the target module(s), follow the `generate-k8s-manifests` skill.

1. Parameters to pass to the skill:
   - `modulePath` = target module path
   - `namespace` = `<namespace>`
   - `environment` = `production`
   - `image` = `<loginServer>/<imageName>:<tag>`
   - `detectedDatabases` and `detectedEnvVars` from Step 5
   - `analysis` = the analyze-repo result
2. The skill writes one YAML file per resource to `<modulePath>/k8s/`.

**AKS Automatic hardening** — if `isAutomatic === true`, before applying, verify the generated manifests contain:

- Pod-level: `securityContext.runAsNonRoot: true` and `securityContext.seccompProfile.type: RuntimeDefault`.
- Container-level: `securityContext.allowPrivilegeEscalation: false` and `capabilities.drop: ["ALL"]`.
- Resource `requests` and `limits` on every container (e.g. `cpu: 500m / 1000m`, `memory: 512Mi / 1Gi`).

The generate-k8s-manifests skill already produces these for `environment == production`; only add what is missing.

### Step 16 — Deploy

1. Ensure the namespace exists (idempotent): `kubectl create namespace <namespace> --dry-run=client -o yaml | kubectl apply -f -`.
2. Apply manifests: `kubectl apply -f <modulePath>/k8s/ --namespace <namespace>`.
3. Retry up to 2 times.

### Step 17 — Verify

1. Call the **`verify-deploy`** tool with `<namespace>` to check pod status, readiness, and events.
2. On failure: inspect pod logs and events, fix the underlying issue (re-run an earlier step if needed), redeploy. Retry up to 2 times.
3. If a `Service` of type `LoadBalancer` or an `Ingress` is configured, report the external IP / hostname to the user.

## Tool / skill catalog used by this loop

| Step | Component | Kind |
| --- | --- | --- |
| 5 | `analyze-repo` | skill |
| 8 (gen) | `generate-dockerfile` | skill |
| 8 (fix), 9 (on-fail), 10 (on-vuln) | `fix-dockerfile` | skill |
| 9 | `build-image-context` | tool |
| 10 | `scan-image` | tool |
| 12 | `prepare-cluster` | tool |
| 13 | `tag-image` | tool |
| 14 | `push-image` | tool |
| 15 | `generate-k8s-manifests` | skill |
| 17 | `verify-deploy` | tool |

## Rules

- **Always retry a failed step up to 2 times** before halting and reporting.
- After every step, send a short status line to the user (`✅ Step N — <heading>`).
- If a tool or skill returns chain hints / next-step suggestions, follow them.
- Use `linux/amd64` for all builds (standard AKS node arch).
- For ACR auth issues, guide the user through `az acr login` before retrying push.
- If the RBAC check in Step 11 halts the workflow, follow its embedded instructions exactly: emit the missing-roles diagnostic, wait for the user, and poll until access propagates. Do **NOT** proceed to deploy or verify until probes pass.
- Do **NOT** suggest `az aks command invoke` as an RBAC bypass when `isAzureRbac == true` — it authenticates as the caller and hits the same `Forbidden`.
- Do **NOT** suggest `--admin` kubeconfig when `localAccountsDisabled == true` — AKS will reject it.
- When using `vscode_askQuestions` for pickers, always include a free-text input so the user can type a value manually.
- Do not invent Azure resources, role names, or commands. If something is unclear, ask.

## Final report

After Step 17 passes, summarize:

1. Cluster, namespace, image reference, and resource group.
2. Deployed resources (deployment, service, configmap, secret, serviceaccount, hpa as applicable).
3. External endpoint, if any.
4. Anything skipped (e.g. database setup deferred, vulnerabilities accepted by user).
