# Plan: `generate-github-workflow` Tool

## Problem Statement

After containerization-assist runs steps 1–7 and deploys the app to AKS, any future code change requires the entire loop to be re-run manually from `analyze-repo`. This does not scale for teams pushing code regularly.

## Solution

Add a `generate-github-workflow` tool at the end of the containerization-assist loop. This tool uses the knowledge-tool pattern (same as `generate-dockerfile`) to produce a `.github/workflows/deploy.yml` file. Once committed, GitHub Actions handles all future deployments automatically — no Copilot, no manual steps, no re-running the loop.

---

## What the Generated Workflow Looks Like

The tool instructs Copilot to generate a two-job GitHub Actions workflow that builds the
image with `az acr build` (in Azure, not on the runner) and deploys to AKS via kubelogin +
`azure/aks-set-context`. Non-sensitive config lives in the workflow-level `env:` block; only
the three OIDC values are GitHub secrets.

> ⛔ **Critical invariants the tool enforces:**
> - Job keys are literally `buildImage` and `deploy` — never `build-and-push`.
> - The image is built with `az acr build` ONLY — never `docker/build-push-action` or `docker build`.
> - **No `environment:` key on any job** — a job-level environment changes the OIDC subject
>   claim from `repo:OWNER/REPO:ref:refs/heads/BRANCH` to `repo:OWNER/REPO:environment:NAME`,
>   which breaks Azure federated-credential authentication.

```yaml
name: Build and Deploy to AKS

on:
  push:
    branches: [main]
  workflow_dispatch:

env:
  ACR_RESOURCE_GROUP: my-rg
  AZURE_CONTAINER_REGISTRY: myregistry
  CONTAINER_NAME: myapp
  CLUSTER_NAME: my-aks
  CLUSTER_RESOURCE_GROUP: my-rg
  DEPLOYMENT_MANIFEST_PATH: k8s/
  DOCKER_FILE: Dockerfile
  BUILD_CONTEXT_PATH: .
  NAMESPACE: default

jobs:
  buildImage:
    permissions:
      contents: read
      id-token: write
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Azure login
        uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Log into ACR
        run: |
          az acr login -n ${{ env.AZURE_CONTAINER_REGISTRY }}

      - name: Build and push image to ACR
        run: |
          az acr build --image ${{ env.AZURE_CONTAINER_REGISTRY }}.azurecr.io/${{ env.CONTAINER_NAME }}:${{ github.sha }} --registry ${{ env.AZURE_CONTAINER_REGISTRY }} -g ${{ env.ACR_RESOURCE_GROUP }} -f ${{ env.DOCKER_FILE }} ${{ env.BUILD_CONTEXT_PATH }}

  deploy:
    permissions:
      actions: read
      contents: read
      id-token: write
    runs-on: ubuntu-latest
    needs: [buildImage]
    steps:
      - uses: actions/checkout@v4

      - name: Azure login
        uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Set up kubelogin for non-interactive login
        uses: azure/use-kubelogin@v1
        with:
          kubelogin-version: "v0.0.25"

      - name: Get K8s context
        uses: azure/aks-set-context@v4
        with:
          resource-group: ${{ env.CLUSTER_RESOURCE_GROUP }}
          cluster-name: ${{ env.CLUSTER_NAME }}
          admin: "false"
          use-kubelogin: "true"

      - name: Deploys application
        uses: Azure/k8s-deploy@v5
        with:
          action: deploy
          manifests: ${{ env.DEPLOYMENT_MANIFEST_PATH }}
          images: |
            ${{ env.AZURE_CONTAINER_REGISTRY }}.azurecr.io/${{ env.CONTAINER_NAME }}:${{ github.sha }}
          namespace: ${{ env.NAMESPACE }}
```

> For Helm/Kustomize projects (`manifestFormat: helm | kustomize`) the deploy job adds an
> `azure/k8s-bake@v1` step before `Azure/k8s-deploy@v5`, passing `manifests: ${{ steps.bake.outputs.manifestsBundle }}`.

### Required GitHub Secrets

Only three secrets are required — everything else is in the workflow `env:` block.

| Type | Name | Value |
|---|---|---|
| Secret | `AZURE_CLIENT_ID` | App registration client ID (OIDC federated credential) |
| Secret | `AZURE_TENANT_ID` | Azure Entra ID tenant ID |
| Secret | `AZURE_SUBSCRIPTION_ID` | Azure subscription ID |

The Azure federated credential must be configured for the **branch** subject
(`repo:OWNER/REPO:ref:refs/heads/main`). Do **not** add a job-level `environment:` unless a
matching environment-scoped federated credential exists — otherwise OIDC auth fails.

---

## How the Tool Fits into the Overall Design

```mermaid
flowchart TD
    subgraph ONCE["Run Once in Copilot (Containerization-Assist Loop)"]
        A[analyze-repo\nDetect language, framework, ports] 
        --> B[generate-dockerfile\nDockerfile plan + knowledge]
        --> C[build-image-context\ndocker build]
        --> D[scan-image\nTrivy vulnerability scan]
        --> E[tag-image\nTag for ACR]
        --> F[push-image\nPush to ACR]
        --> G[generate-k8s-manifests\nDeployment + Service YAML]
        --> H[prepare-cluster + deploy\nkubectl apply]
        --> I[verify-deploy\nCheck pods and endpoints]
        --> J[generate-github-workflow\n⭐ NEW TOOL\nProduce .github/workflows/deploy.yml]
    end

    subgraph COMMIT["Commit to Git"]
        K[Dockerfile\nk8s/ manifests\n.github/workflows/deploy.yml]
    end

    subgraph AUTO["Every Future git push — Fully Automated"]
        L[GitHub Actions triggers]
        --> M["azure/login<br/>OIDC — branch-scoped, no passwords"]
        --> N["az acr build<br/>Build image in Azure (ACR)<br/>Tag with commit SHA"]
        --> O["azure/use-kubelogin + aks-set-context<br/>kubeconfig via OIDC"]
        --> P["azure/k8s-bake (helm/kustomize only)<br/>Process manifests"]
        --> Q["azure/k8s-deploy<br/>Rolling update to AKS"]
    end

    J --> COMMIT
    COMMIT -->|git push| AUTO

    style J fill:#f90,color:#000
    style ONCE fill:#f0f7ff,stroke:#0078d4
    style AUTO fill:#f0fff4,stroke:#00a86b
```

---

## Implementation Phases

### Phase 1 — Knowledge Pack
**Create** `knowledge/packs/github-actions-pack.json`

This is the recipe book of GitHub Actions best practices the tool will query. Each entry is one piece of advice with an example. The pack is auto-embedded into the binary at the next `npm run build:knowledge` — no manual import required.

**Entries (12 total):**

| ID | Purpose | Severity |
|---|---|---|
| `github-oidc-permissions` | per-job `contents: read` + `id-token: write` (deploy also `actions: read`) | required |
| `azure-login-oidc` | `azure/login@v2` with the 3 OIDC secrets | required |
| `acr-docker-login` | `az acr login -n ${{ env.AZURE_CONTAINER_REGISTRY }}` after OIDC login | high |
| `docker-build-push-acr` | `az acr build` with `github.sha` tag (NOT docker build) | required |
| `aks-setup-kubectl` | `azure/use-kubelogin@v1` step | high |
| `aks-get-credentials` | `azure/aks-set-context@v4` (admin: false, use-kubelogin: true) | high |
| `k8s-bake-manifests` | `azure/k8s-bake@v1` for helm/kustomize | medium |
| `k8s-deploy-action` | `Azure/k8s-deploy@v5` + annotate step | required |
| `workflow-two-job-structure` | literal `buildImage` + `deploy` jobs, `needs: [buildImage]` | high |
| `workflow-concurrency` | `concurrency` block with `cancel-in-progress: true` | medium |
| `required-secrets-guidance` | only 3 OIDC secrets; rest in `env:` block | high |
| `no-job-environment-oidc` | NEVER add `environment:` to a job (breaks OIDC subject) | required |

**Entry format:**
```json
{
  "id": "azure-login-oidc",
  "category": "cicd",
  "pattern": "azure/login",
  "recommendation": "Use azure/login@v2 with OIDC federated credentials — never store passwords as secrets",
  "example": "- uses: azure/login@v2\n  with:\n    client-id: ${{ secrets.AZURE_CLIENT_ID }}\n    tenant-id: ${{ secrets.AZURE_TENANT_ID }}\n    subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}",
  "severity": "required",
  "tags": ["generate-github-workflow", "azure-oidc", "azure-login", "github-actions", "security"],
  "description": "OIDC federated credentials eliminate long-lived secrets entirely"
}
```

---

### Phase 2 — Type Extensions
**Files:** `src/types/topics.ts`, `src/knowledge/types.ts`, `src/knowledge/schemas.ts`

Small additions to the type system so the knowledge system knows about the new domain:

- **`src/types/topics.ts`** — add `GITHUB_WORKFLOW: 'github_workflow'` to the `TOPICS` constant. This is the search key the tool uses to query the knowledge base.
- **`src/knowledge/types.ts`** — add `CICD: 'cicd'` to the `CATEGORY` constant. Keeps GitHub Actions knowledge separate from dockerfile/kubernetes entries.
- **`src/knowledge/schemas.ts`** — add `'cicd'` to the `KnowledgeCategorySchema` Zod enum so validation passes.

---

### Phase 3 — Tool Files
**Create** `src/tools/generate-github-workflow/` with three files:

#### `types.ts` — Tool identity card
Exports `generateGithubWorkflowToolDefinition`:
- Name, description, category, version
- `metadata: { knowledgeEnhanced: true }` — signals the tool uses the knowledge base
- `chainHints.success` — tells Copilot what to say next: *"Commit `.github/workflows/deploy.yml` and configure AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID as GitHub repository secrets. Set up an OIDC federated credential in Azure for your GitHub repo."*

#### `schema.ts` — Input/output contract
**Inputs (Zod schema):**

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `repositoryPath` | `string` | ✅ | — | Path to the repo |
| `registry` | `string` | ✅ | — | ACR login server (e.g. `myregistry.azurecr.io`) |
| `clusterName` | `string` | ✅ | — | AKS cluster name |
| `resourceGroup` | `string` | ✅ | — | Azure resource group |
| `imageName` | `string` | ❌ | repo dir name | Docker image name |
| `namespace` | `string` | ❌ | `default` | Kubernetes namespace |
| `environment` | `enum` | ❌ | `production` | `development` / `staging` / `production` |
| `manifestFormat` | `enum` | ❌ | `k8s` | `k8s` / `helm` / `kustomize` |
| `branches` | `string[]` | ❌ | `['main']` | Branches that trigger the workflow |
| `manifestPath` | `string` | ❌ | — | Path to existing manifests directory |
| `language` | `string` | ❌ | — | From `analyze-repo` (for cache hints) |
| `framework` | `string` | ❌ | — | From `analyze-repo` (for cache hints) |

**Output types:**
- `GithubWorkflowPlan` — `nextAction`, `workflowJobs`, `secretsRequired`, `summary`, `attributionLabels`
- `WorkflowJobDescription` — `name`, `steps`, `runsOn`, `environment`

#### `tool.ts` — Core logic
Implements the `createKnowledgeTool` pattern (identical factory to `generate-dockerfile`):

**Step 1 — Query the knowledge base:**
- Topic: `TOPICS.GITHUB_WORKFLOW`
- Category: `CATEGORY.CICD`
- Max 15 snippets, 6000 chars
- Filters: `language`, `framework`, `environment`

**Step 2 — Categorise into 4 buckets:**
- `auth` — entries tagged `azure-oidc`, `azure-login`
- `build` — entries tagged `docker-build`, `acr`, `registry`
- `deploy` — entries tagged `aks`, `kubectl`, `k8s-deploy`, `k8s-bake`
- `bestPractices` — everything else (caching, environments, concurrency)

**Step 3 — Apply rules:**
- Include `k8s-bake` step if `manifestFormat === 'helm'` or `'kustomize'`
- Always include concurrency block
- Always set `runsOn: 'ubuntu-latest'`

**Step 4 — Build the plan:**
Returns `GithubWorkflowPlan` with:
- `nextAction.action: 'create-files'`
- `nextAction.files: [{ path: '.github/workflows/deploy.yml', purpose: 'CI/CD workflow' }]`
- `nextAction.instruction` — full prompt for Copilot to generate the YAML, incorporating all categorised knowledge snippets
- `secretsRequired: ['AZURE_CLIENT_ID', 'AZURE_TENANT_ID', 'AZURE_SUBSCRIPTION_ID']`
- Attribution label: `com.azure.containerizationassist/workflow-version: 1.4.0`

> ⚠️ **Constraint:** `tool.ts` must not use `import.meta` — the CJS build (`tsconfig.cjs.json`) forbids it. See `AGENTS.md`.

---

### Phase 4 — Registration & Wiring
**Files:** `src/tools/shared/toolDefinition.ts`, `src/tools/index.ts`, `src/app/chain-hints.ts`

Three small edits to plug the tool into the system:

1. **`toolDefinition.ts`** — Add `GENERATE_GITHUB_WORKFLOW: 'generate-github-workflow'` to the `TOOL_NAME` enum. This is the single source of truth for the tool name — prevents typos across the codebase.

2. **`src/tools/index.ts`** — Import and add `generateGithubWorkflowTool` to the `ALL_TOOLS` array. This is how the MCP server discovers and exposes the tool.

3. **`src/app/chain-hints.ts`** — Add the `'generate-github-workflow'` entry:
   - **success:** *"GitHub workflow generation complete. Commit `.github/workflows/deploy.yml` and configure AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID as GitHub repository secrets. Set up an OIDC federated credential in Azure Entra ID for your GitHub repository."*
   - **failure:** *"Workflow generation failed. Ensure registry, clusterName, and resourceGroup are provided."*

---

### Phase 5 — Prompt Step Helper
**File:** `src/prompts/shared/steps.ts`

Add a `generateGithubWorkflowStep(registry, clusterName, resourceGroup)` helper function. This mirrors the existing `generateDockerfileStep()` and `scanStep()` helpers — a reusable `{ heading, body }` object that any future workflow prompt can include in one line.

This makes it trivial to add the step to the AKS loop prompt in a future iteration.

---

### Phase 6 — Tests
**Create** `test/unit/tools/generate-github-workflow/`

Two test files following existing patterns:

- **`schema.test.ts`** — validates inputs: required fields fail without `registry`/`clusterName`/`resourceGroup`; defaults apply correctly (`namespace`, `environment`, `manifestFormat`, `branches`); optional fields accepted.

- **`tool.test.ts`** — mocks the knowledge base; verifies:
  - `nextAction.action === 'create-files'`
  - `nextAction.files` contains `.github/workflows/deploy.yml`
  - `secretsRequired` includes all three Azure secrets
  - Bake step is included when `manifestFormat === 'helm'` or `'kustomize'`
  - Bake step is excluded when `manifestFormat === 'k8s'`
  - Attribution label is present in output

---

## File Change Summary

| File | Action |
|---|---|
| `knowledge/packs/github-actions-pack.json` | **CREATE** |
| `src/types/topics.ts` | **MODIFY** — add `GITHUB_WORKFLOW` topic |
| `src/knowledge/types.ts` | **MODIFY** — add `CICD` category |
| `src/knowledge/schemas.ts` | **MODIFY** — add `'cicd'` to Zod enum |
| `src/tools/generate-github-workflow/types.ts` | **CREATE** |
| `src/tools/generate-github-workflow/schema.ts` | **CREATE** |
| `src/tools/generate-github-workflow/tool.ts` | **CREATE** |
| `src/tools/shared/toolDefinition.ts` | **MODIFY** — add to `TOOL_NAME` |
| `src/tools/index.ts` | **MODIFY** — add to `ALL_TOOLS` |
| `src/app/chain-hints.ts` | **MODIFY** — add chain hint entry |
| `src/prompts/shared/steps.ts` | **MODIFY** — add step helper |
| `test/unit/tools/generate-github-workflow/schema.test.ts` | **CREATE** |
| `test/unit/tools/generate-github-workflow/tool.test.ts` | **CREATE** |

---

## Execution Order

```
Phase 1 + Phase 2 (parallel — no dependencies)
    ↓
Phase 3 (depends on 1 + 2)
    ↓
Phase 4 + Phase 5 + Phase 6 (parallel — all depend on Phase 3)
    ↓
Verify:
  npm run build:knowledge   ← embeds new pack
  npx tsc -p tsconfig.json --noEmit       ← ESM passes
  npx tsc -p tsconfig.cjs.json --noEmit   ← CJS passes
  npm run test:unit                        ← new tests pass
```

---

## Reference Implementations

- [src/tools/generate-dockerfile/tool.ts](src/tools/generate-dockerfile/tool.ts) — primary pattern (createKnowledgeTool, categorisation, plan builder)
- [src/tools/generate-k8s-manifests/tool.ts](src/tools/generate-k8s-manifests/tool.ts) — multi-topic query pattern
- [src/prompts/shared/steps.ts](src/prompts/shared/steps.ts) — step helper pattern
- [knowledge/packs/kubernetes-pack.json](knowledge/packs/kubernetes-pack.json) — knowledge entry format

---

## Out of Scope (v1)

- PR check workflow (build + scan only, no deploy)
- Non-AKS targets (Azure Container Apps, etc.)
- Updating the `aks-loop` prompt to include this step automatically
- Multi-environment workflow (dev → staging → prod promotion gates)
