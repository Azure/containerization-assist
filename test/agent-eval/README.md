# Agent Eval

A local test framework that compares **agent success rate** and **token usage**
across three CA delivery paths on the same legacy-app fixtures:

| Path     | What the agent gets                                                                 |
| -------- | ----------------------------------------------------------------------------------- |
| `bare`   | Generic "containerize and deploy this app to AKS" prompt. No CA help.               |
| `skills` | Same task + the CA `deploy-to-aks` SKILL bundle (markdown how-to + CA MCP tools).   |
| `mcp`    | Same task + the CA aks-loop MCP **prompt** + CA MCP tools.                          |

All three paths run against the same fixture, with the same model, against the
same live Azure infra (ACR + AKS). After each run we score the produced
artifacts with deterministic checks and compare tokens-out vs the `bare`
control.

## What it answers

- **Do skills move the needle?** `bare` → `skills` Δ on success checks and Δtokens.
- **Does the MCP prompt move it further?** `bare` → `mcp` Δ on the same.
- **Cost vs quality across models.** Sweep `azure:gpt-4.1,azure:gpt-4o,azure:gpt-4.1-mini`
  in one command; report renders one table per model plus a cross-model cost
  summary.

## Checks ([`checks.ts`](checks.ts))

| Check                  | What it asserts                                                                  |
| ---------------------- | -------------------------------------------------------------------------------- |
| `docker-builds`        | The produced Dockerfile actually builds (`docker build .`).                      |
| `requires-azure-base`  | Every `FROM` in the Dockerfile is from `mcr.microsoft.com/`.                     |
| `has-required-labels`  | Dockerfile carries `com.azure.containerizationassist.createdby`; K8s manifests carry `app.kubernetes.io/{name,managed-by}`. |

A run passes when every selected check passes.

## Local setup

The harness needs three things: Foundry creds, Docker, and an Azure
ACR + AKS the agent can push/deploy to.

### 1. `.env` (repo root)

```
AZURE_FOUNDRY_API_KEY=...
AZURE_FOUNDRY_ENDPOINT=https://<resource>.services.ai.azure.com/openai/v1
```

See [`docs/agent-eval-foundry-setup.md`](../../docs/agent-eval-foundry-setup.md)
for how to provision the Foundry resource.

### 2. Azure infra (one-time)

```sh
# Resource group + ACR + small AKS cluster + ACR attach.
# Replace names / region as needed.
RG=ca-eval-rg
LOC=eastus
ACR=caevalacr
AKS=ca-eval-aks

az group create -n $RG -l $LOC
az acr create  -n $ACR -g $RG --sku Basic
az aks create  -n $AKS -g $RG --node-count 1 --enable-managed-identity --attach-acr $ACR
az aks get-credentials -n $AKS -g $RG
kubectl create namespace eval-ns
```

### 3. Tell the harness where it is (optional — these are the defaults)

```sh
export AGENT_EVAL_REGISTRY=caevalacr.azurecr.io
export AGENT_EVAL_RESOURCE_GROUP=ca-eval-rg
export AGENT_EVAL_CLUSTER=ca-eval-aks
export AGENT_EVAL_NAMESPACE=eval-ns
export AGENT_EVAL_IMAGE=eval-image     # base name; sloid per model under the hood
```

### 4. Sanity check

```sh
npm run eval -- ping --model azure:gpt-4o-mini
```

## Run the gradient

`gradient` is the headline command. One invocation, one markdown report
covering every (path × fixture × model) combination.

```sh
npm run eval -- gradient \
  --fixtures test/fixtures/legacy-java/spring-boot-rest-api,test/fixtures/legacy-java/coolstore,test/fixtures/legacy-java/spring-mvc-war \
  --models azure:gpt-4.1,azure:gpt-4o,azure:gpt-4.1-mini \
  --out /tmp/gradient.json
```

Defaults:

- Models run **in parallel** (each gets its own ACR repo + Deployment name to
  avoid cross-talk). Pass `--sequential` to serialize.
- All three paths run unless you scope with `--paths bare,skills`.
- Per-run kubectl cleanup happens before **and** after each run, so a stuck
  Deployment never poisons the next path.

Output:

1. Definitions of `bare` / `skills` / `mcp`.
2. **One table per model.** Rows = paths. Columns grouped by fixture, with
   `docker-builds | requires-azure-base | has-required-labels | Δtok` per
   group. ✅ / ❌ for checks, `+24K` style deltas for tokens.
3. **Cross-model cost table.** Tokens-out per (path × model), summed across
   fixtures.
4. **Errors footer.** Any cell that errored is listed once with its message.

## Other commands

```
agent-eval ping     --model <spec>
agent-eval run      --fixture <dir> --mode <bare|skills|mcp> --model <spec>
agent-eval check    --dir <artifactDir> [--fixture <dir>] [--checks <names>]
agent-eval gradient --fixtures <dirs> --models <specs> [--paths bare,skills,mcp]
                    [--sequential] [--out results.json]
```

`run` is the single-shot version of one gradient cell — useful when iterating
on a prompt or skill and you want raw stdout, not a report.

## Adding a new app fixture

1. Drop the source under `test/fixtures/legacy-java/my-app/`.
2. Add it to the `--fixtures` list (or implement `--fixtures-dir` discovery if
   you want auto-pickup).

No code change required.

## Adding a new check

1. Implement a `Check` in [`checks.ts`](checks.ts): single async function
   returning `{ name, passed, message, details? }`.
2. Append it to `ALL_CHECKS` and `selectChecks` in the same file.
3. Reference it in [`gradient.ts`](gradient.ts)'s `headlineNames` if you want
   it surfaced as a column in the per-model table.

## Providers ([`providers.ts`](providers.ts))

Both `azure:` and `foundry:` prefixes hit the same Foundry resource via the
same OpenAI-compatible v1 endpoint. The model id is your **deployment name**:

| Prefix     | Used for                                                       |
| ---------- | -------------------------------------------------------------- |
| `azure:`   | OpenAI catalog deployments (`gpt-4o`, `gpt-4.1`, `gpt-4.1-mini`, …) |
| `foundry:` | Partner / community catalog deployments                        |

## CI

Not wired up yet. The plan after this demo is to trigger on PR pushes to
`skills/**` (and probably `knowledge/**`, `src/prompts/**`, `src/mcp/**`),
but only after we agree on (a) whether CI should hit live Azure on every PR
or run a cheaper subset, and (b) which signals should fail vs inform a PR.
