# Agent Eval

A local harness that compares **check pass-rate** and **token cost** across
three CA delivery paths on the same legacy-app fixtures — same model, same live
Azure infra (ACR + AKS):

| Path     | What the agent gets |
| -------- | ------------------- |
| `bare`   | Generic "containerize and deploy this app to AKS" prompt. The control. |
| `mcp`    | Same task + the CA `aks-loop` MCP prompt + CA MCP tools. |
| `skills` | Same task + the CA `deploy-to-aks` SKILL bundle + CA MCP tools. |

After each run, deterministic checks score the produced artifacts and the report
shows each path's lift over the `bare` control, swept across every model in one
command.

## Checks ([`checks.ts`](checks.ts))

| Check                  | What it asserts                                                                  |
| ---------------------- | -------------------------------------------------------------------------------- |
| `docker-builds`        | The produced Dockerfile actually builds (`docker build .`).                      |
| `requires-azure-base`  | Strict allowlist: every `FROM` must be on `mcr.microsoft.com/`, or a recognized stack with no MCR equivalent (Maven/Tomcat/WildFly/Go/…), or carry a `# WHY-NOT-MCR:` annotation. Public bases with a known MCR equivalent (Node/JDK/Python/.NET) and unrecognized/unparseable bases **fail**. |
| `has-required-labels`  | Dockerfile carries `com.azure.containerizationassist.createdby`; K8s manifests carry `app.kubernetes.io/{name,managed-by}`. |

A run passes when every selected check passes.

## Setup

Needs Foundry creds, Docker, and an Azure ACR + AKS the agent can push/deploy to.

**1. `.env` (repo root)** — create it with your Foundry resource's key and
endpoint:

```
AZURE_FOUNDRY_API_KEY=...
AZURE_FOUNDRY_ENDPOINT=https://<resource>.services.ai.azure.com/openai/v1
```

**2. Azure infra (one-time):**

```sh
RG=ca-test-suite; LOC=eastus2; ACR=caevalacr; AKS=ca-eval-aks
az group create -n $RG -l $LOC
az acr create  -n $ACR -g $RG --sku Basic
az aks create  -n $AKS -g $RG --node-count 1 --enable-managed-identity --attach-acr $ACR
az aks get-credentials -n $AKS -g $RG
kubectl create namespace eval-ns
```

**3. Point the harness at it** (optional — these are the defaults):

```sh
export AGENT_EVAL_REGISTRY=caevalacr.azurecr.io
export AGENT_EVAL_RESOURCE_GROUP=ca-test-suite
export AGENT_EVAL_CLUSTER=ca-eval-aks
export AGENT_EVAL_NAMESPACE=eval-ns
export AGENT_EVAL_IMAGE=eval-image
```

**4. Sanity check:** `npm run eval -- ping --model azure:gpt-4o-mini`

## Run the gradient

The headline command — one invocation covers every (path × fixture × model) cell:

```sh
npm run eval -- gradient \
  --fixtures test/fixtures/legacy-java/spring-boot-rest-api,test/fixtures/legacy-java/spring-mvc-war \
  --models azure:gpt-4.1,azure:gpt-4o,azure:gpt-4.1-mini \
  --out /tmp/gradient.json
```

- All three paths run unless scoped with `--paths bare,skills`.
- Models run **in parallel** when more than one is passed (each lane gets its
  own ACR repo + Deployment). Pass `--sequential` to force serial execution,
  or `--max-concurrent-models <n>` to cap concurrency for provider rate limits.
- `--reps <n>` repeats each cell; `--fixtures-dir <dir>` auto-discovers fixtures
  from a parent directory instead of listing them with `--fixtures`.
- kubectl cleanup runs before **and** after each cell, so a stuck Deployment
  can't poison the next path.

`--out results.json` also writes a companion self-contained `results.html`
(the heatmap report) alongside it. The report shows per-path scores, the
quality heatmap, a cost-vs-effectiveness scatter, and an errors footer.

## Other commands

```
agent-eval ping     --model <spec>
agent-eval run      --fixture <dir> --mode <baseline|skills|mcp> --model <spec>
agent-eval check    --dir <artifactDir> [--checks <names>]
agent-eval gradient (--fixtures <dirs> | --fixtures-dir <dir>) --models <specs>
                    [--paths ...] [--sequential | --parallel] [--reps <n>] [--out results.json]
```

`run` is the single-shot version of one gradient cell — raw stdout, no report.

## Extending

- **New fixture:** drop source under `test/fixtures/legacy-java/my-app/` and add
  it to `--fixtures` (or point `--fixtures-dir` at its parent). No code change.
- **New check:** add a `Check` in [`checks.ts`](checks.ts) (async, returns
  `{ name, passed, message, details? }`), append it to `ALL_CHECKS` /
  `selectChecks`, and add its name to `HEADLINE_CHECK_NAMES` in
  [`gradient.ts`](gradient.ts) to surface it as a report column.

## Providers ([`providers.ts`](providers.ts))

`azure:` and `foundry:` both hit the same Foundry v1 endpoint; the model id is
your **deployment name**. Use `azure:` for OpenAI-catalog deployments (`gpt-4o`,
`gpt-4.1`, …) and `foundry:` for partner/community deployments.

## CI

[`azure-pipelines.yml`](../../azure-pipelines.yml) runs the gradient sweep in
Azure DevOps on merges to `main` that touch `skills/**` and on manual runs
(PRs do not trigger it — the sweep is too slow to gate PRs on). It
authenticates to Azure through a **service connection** (no stored Azure
secrets) and reads the Foundry credentials from the `agent-eval-foundry`
variable group. Every run uses the full matrix — all three paths (`bare`,
`mcp`, `skills`) across every fixture and model — so each run shows the skills
path's lift over the `bare` control and `mcp`. Merges to `main` run 3 reps for
a robust comparison; manual runs honor the `reps` parameter. The JSON and HTML
reports are published as a pipeline artifact. See the pipeline header for the
one-time service-connection + variable-group setup.
