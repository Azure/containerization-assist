# Azure Container Apps ➜ Dockerfile & AKS Manifest Migration

This document describes **how to extend Container Copilot so it can migrate an existing [Azure Container Apps](https://learn.microsoft.com/azure/container-apps/) deployment to a repeatable Dockerfile + AKS (Kubernetes) manifest setup.**

> The design plugs straight into the existing LLM-driven pipeline without disrupting current behaviour.

---

## 1  Problem Statement

Given **an existing Container App** (identified by its ARM resource ID *or* an exported YAML template), generate:

1. A deterministic **Dockerfile** that reproduces the running container image and its build-time requirements.
2. A set of **AKS-ready Kubernetes manifests** (Deployment, Service, ConfigMap, Secret, HPA, Ingress) that express the same runtime configuration, scaling and networking rules.

## 2  High-level Approach

1. **Introduce a new pipeline stage** `containerappsstage` that runs *after* repository analysis and *before* the existing `dockerstage` / `manifeststage`.
2. **Accept a new CLI flag** on `generate`:
   ```bash
   container-kit generate --container-app \
     "subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.App/containerApps/myApp" \
     <target-repo>
   ```
   or
   ```bash
   container-kit generate --container-app ./myApp-export.yaml <target-repo>
   ```
3. Fetch/parse the Container App spec, translate it to an initial Dockerfile + K8s YAML.
4. Hand the artefacts to downstream stages; they will iterate & fix as usual.

## 3  Pipeline Integration

| Stage | Purpose | Notes |
|-------|---------|-------|
| `analysis` | Understand source repo (unchanged) |  |
| **`containerapps`** | Translate CA spec → Dockerfile & manifests | *NEW* |
| `docker` | Build/fix Dockerfile | Re-uses output of previous stage |
| `manifest` | Deploy/fix manifests on Kind |  |

`cmd/generate.go` snippet:
```go
runner := pipeline.NewRunner([]*pipeline.StageConfig{
  { Id: "analysis",  ... },
  { Id: "containerapps", Path: caInputPath, Stage: &containerappsstage.ContainerAppsStage{ ... } },
  { Id: "docker",      ... },
  { Id: "manifest",    ... },
}, os.Stdout)
```

## 4  `containerappsstage` Details

### 4.1  Initialize
* Parse the provided ARM/YAML into an internal `ContainerAppSpec` struct (only fields we care about: containers, env, ingress, scale, secrets).

### 4.2  Generate
* Deterministic mapping (MVP):
  * `properties.template.containers[0].image` → `Deployment.spec.template.spec.containers[0].image`
  * `properties.ingress.targetPort`           → `Service` & `container.port`
  * `env` & `secrets`                         → `ConfigMap` / `Secret`
  * `scale.minReplicas` / `maxReplicas`       → `replicas:` or an `HPA`
* Optionally call LLM (`AzOpenAIClient`) with the CA spec + repo info to refine Dockerfile / YAML – keep existing `<DOCKERFILE>` & `<MANIFEST>` tag protocol.

### 4.3  Run
* Attempt a **dry-run deploy** to Kind (reuse `clients.DeployAndVerifySingleManifest`).
* On failure loop with LLM for fixes, just like `manifeststage`.

### 4.4  Deploy
* No-op – actual push handled by downstream stages.

### 4.5  State Handoff
* `state.Dockerfile.Content` – filled with generated Dockerfile.
* `state.K8sObjects` – populated via `k8s.ReadK8sObjects` so later stages see manifests as *existing*.

## 5  CLI Changes

```go
// cmd/generate.go
var containerApp string

generateCmd.PersistentFlags().StringVarP(&containerApp, "container-app", "a", "", "Azure Container App resource ID or exported YAML to migrate")
```

If `containerApp != ""` the path/ID is stored in `PipelineState.Metadata[pipeline.ContainerAppKey]` for the stage to consume.

## 6  Azure API Helper (optional)

Add `pkg/clients/azureapps.go`:
```go
type ContainerAppsClient struct{ azAuth *azidentity.DefaultAzureCredential }
func (c *ContainerAppsClient) Export(ctx context.Context, resourceID string) ([]byte, error) {
    // call `az containerapp export --id <resourceID> --output json`
}
```

## 7  Testing Strategy

* **Unit tests**: feed a minimal CA YAML into `containerappsstage.Generate` and assert Dockerfile & YAML contents.
* **e2e**: new GitHub Action matrix job that runs `generate --container-app sample.yaml` and expects `kubectl get pods` → Running.

## 8  Future Enhancements

* Map **Dapr**, **KEDA** and **Revisioned rollouts** to equivalent AKS resources.
* Produce a Helm/Bicep scaffold from the same spec.
* Add flag `--helm` to output a chart instead of raw YAML.

## 9  Concrete Task Checklist (merged & expanded)

The list below merges the original tasks with the detailed implementation steps you added in *mystuff.md*.

### 9.1  Module & Helper Code

- [ ] Create `pkg/azureaca/types.go` with `ACAConfig` (Name, Image, Env, CPU, Memory, Replicas, Port, Ingress, LivenessPath, ReadinessPath).
- [ ] Implement `pkg/azureaca/parser.go` → `ParseACAJSON(path string) (*ACAConfig, error)`.
  - Log skipped or unsupported ACA features (e.g., `authSettings`, `revisions`).
- [ ] Implement `pkg/azureaca/transform.go` → `GenerateK8sObjects(cfg *ACAConfig)` returning Deployment, Service, ConfigMap (and Secret when needed).
- [ ] Enhance `pkg/k8s` (or new sub-pkg) with:
  - `NewDeployment`, `NewService`, `NewConfigMap`, `NewSecret` helpers.
  - `WriteK8sObjectsToFiles(objs, dir)` to dump YAMLs.

### 9.2  Pipeline Stage

- [ ] Add `pkg/pipeline/acatransformstage/acatransformstage.go` implementing `pipeline.PipelineStage`.
  - `Generate` uses ACA parser + transformer.
  - `WriteSuccessfulFiles` calls `WriteK8sObjectsToFiles`.
  - `Run` / `Deploy` remain no-ops (manifeststage handles deploy/fix).

### 9.3  CLI & Runner Wiring

- [ ] Add `--aca-config <path>` flag to `generate`.
- [ ] Store flag path at `PipelineState.Metadata[pipeline.UserACAConfigPathKey]`.
- [ ] Build stage list dynamically:
  1. `analysis`
  2. If `UserACAConfigPathKey` present → `acatransform`
  3. `docker` (skipped when ACA stage present & Dockerfile not required)
  4. `manifest`
- [ ] Log a clear banner when ACA migration path is engaged.

### 9.4  LLM Prompt & Metadata Integration

- [ ] After ACA parsing, create `ACAAnalysisSummary` (skipped items, assumptions) and attach to `state.Metadata` so downstream LLM repair loops can reference it.
- [ ] Optionally annotate generated YAML with comments indicating inferred values (helps LLM context & human review).

### 9.5  Error Handling & Logging

- [ ] Parse function must error if `Image` is empty (Dockerfile generation is not part of this flow).
- [ ] Warn when env vars are secret-refs or dynamic expressions that aren't handled automatically.
- [ ] Provide guidance when non-ACA JSON/YAML is supplied ("Did you run `az containerapp export`?").

### 9.6  Tests & CI

- [ ] Unit tests for parser & transformer.
- [ ] Stage tests similar to `dockerstage_test.go`.
- [ ] A GitHub Action e2e job running `container-kit generate --aca-config sample.json .` and verifying pods reach `Running`.

### 9.7  Acceptance Criteria

- [ ] `container-kit generate --aca-config ./myApp.json .` outputs manifests under `./manifests/` with no Dockerfile changes.
- [ ] Incorrect ACA file → descriptive error.
- [ ] Logs clearly indicate inferred vs missing configuration.
- [ ] Downstream `manifeststage` can iterate/refine the produced YAML just like hand-written manifests.

---

## 10  Choosing ACA vs Dockerfile/Manifest flow automatically

The project's command should decide **at runtime** whether to activate the migration stage:

*If* `--aca-config` (or corresponding env-var/config) is supplied ➜ insert `ACATransformStage` **before** Docker & Manifest stages.

```go
if metadata[pipeline.UserACAConfigPathKey] != nil {
    // We skip template generation: ACA already defines the runtime image.
    // Docker/Manifest stages still run so LLM can repair or refine.
}
```

This keeps a single entry-point (`generate`) while still allowing a future dedicated `migrate` command that simply forwards the flag.

## 11  Prototype Status

* `mystuff.md` contains the first pass of `ACATransformStage` **plus** a markdown brainstorm. The code compiles but references helper functions (`k8s.NewDeployment`, etc.) that still need to be added.
* After the extraction & helper additions outlined above, the stage will fully participate in snapshots, retries, and Kind deployments like every other stage.

---

### Status

This design is **not yet implemented** – the document serves as implementation guidance and project discussion reference. 