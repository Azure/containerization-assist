---
name: generate-k8s-manifests
description: Generate production-ready Kubernetes manifests (Deployment, Service, ConfigMap, Secret, ServiceAccount, optional HPA) for a containerized application. Use AFTER a Dockerfile exists and has been validated by fix-dockerfile. Triggers include "generate kubernetes manifests", "deploy this to k8s", "write k8s yaml", "create deployment yaml", or when running aks-loop and image is built. Writes manifests directly to the target directory.
argument-hint: <modulePath> [appName] [namespace=default] [environment=production|development]
---

# Generate Kubernetes Manifests

Produce a complete, production-ready set of Kubernetes manifests for **one
application** in **one module** and **write them to disk**. Deterministic —
every choice below comes from a fixed rule or table, not the model's
judgment. The artifacts are the YAML files on disk, not chat output.

## Inputs

| Field | Required | Description |
|---|---|---|
| `modulePath` | yes | Absolute path of the module to deploy (a module from `analyze-repo`). |
| `appName` | optional | Kubernetes resource name. Defaults to the directory basename of `modulePath`. Must be a valid DNS-1123 label (lowercase, digits, `-`, ≤ 63 chars). If the resolved value is invalid, normalize it (lowercase, replace invalid chars with `-`) and tell the user. |
| `image` | optional | Container image reference (`registry/repo:tag`). Defaults to `<appName>:dev` (caller must replace it before deploy). Used for `Deployment.spec.template.spec.containers[].image`. |
| `namespace` | optional | Defaults to `default`. |
| `environment` | optional | `production` (default) or `development`. Affects replicas, probes, resource defaults. |
| `language` | optional | From `analyze-repo`. Used to pick default container port if `ports` empty. |
| `framework` | optional | From `analyze-repo`. Same. |
| `ports` | optional | From `analyze-repo`. First port becomes `containerPort` + `Service.targetPort`. |
| `detectedEnvVars` | optional | From `analyze-repo`. Used to build `ConfigMap` (`config`/`database` types) and `Secret` (`secret` type). |
| `detectedDatabases` | optional | From `analyze-repo`. If any database is in the managed set (see below), emit a `ServiceAccount` with workload-identity annotations + reference it from the `Deployment`. |
| `trafficLevel` | optional | `high` \| `medium` \| `low`. Affects replica count + HPA. |
| `criticalityTier` | optional | `tier-1` (mission-critical) \| `tier-2` \| `tier-3`. Same. |
| `manifestType` | optional | `kubernetes` (default), `helm`, `kustomize`, `aca`. **This skill only handles `kubernetes`.** For `helm` / `kustomize` / `aca`, tell the user it's out of scope and STOP. |
| `outputDir` | optional | Where to write the YAML files. Default: `<modulePath>/k8s/`. |

If `modulePath` is missing → ask the user. STOP.

## Prerequisite: analyze-repo

This skill needs `language`, `framework`, `ports`, `detectedDatabases`,
and `detectedEnvVars` from the `analyze-repo` skill. If you don't
already have that output for `<modulePath>` from earlier in this
conversation, follow the `analyze-repo` skill against `<modulePath>`
first, then come back.

## Procedure

### Step 1 — Resolve defaults

**Container port** — if `ports[0]` is set, use it. Otherwise pick from the
language default:

| Language / framework | Default port |
|---|---|
| Spring Boot | 8080 |
| Java (other) | 8080 |
| ASP.NET Core | 8080 |
| Node / Express / Next / Fastify | 3000 |
| Python / Flask | 5000 |
| Python / Django | 8000 |
| Python / FastAPI / Uvicorn | 8000 |
| Go (gin / echo / fiber / chi) | 8080 |
| Rust (axum / actix / rocket) | 8080 |
| PHP-FPM | 9000 |
| PHP / Apache | 80 |
| Ruby on Rails | 3000 |
| Unknown | 8080 |

**Replica count** — start from this table, then override if both
`trafficLevel` and `criticalityTier` are given (lower row wins):

| Environment | Default `replicas` |
|---|---|
| `development` | 1 |
| `production` | 2 |

| Traffic × Criticality | `replicas` |
|---|---|
| `high` × `tier-1` | 5 |
| `high` × `tier-2` | 3 |
| `medium` × `tier-1` | 3 |
| `medium` × `tier-2` | 2 |
| `low` × any | 1 |
| `tier-3` (any traffic) | 1 |

**Resources** — fixed defaults (`requests` ≈ 50% of `limits`):

| Environment | requests.cpu | requests.memory | limits.cpu | limits.memory |
|---|---|---|---|---|
| `development` | `100m` | `128Mi` | `500m` | `512Mi` |
| `production` | `250m` | `256Mi` | `500m` | `512Mi` |

If the user supplied `k8sConfig.resourceDefaults`, use those instead.

**Probes** — pick a probe path:

| Framework | path |
|---|---|
| Spring Boot | `/actuator/health` (liveness), `/actuator/health/readiness` (readiness) |
| ASP.NET Core | `/health` |
| Express / Fastify with `health-checks` dependency | `/health` |
| FastAPI / Flask / Django | `/health` |
| Go web frameworks | `/health` |
| Others | `/` (TCP socket probe fallback — see below) |

If you cannot infer a meaningful HTTP endpoint, fall back to a **TCP
socket** probe on `containerPort`.

In **development** environment, omit probes entirely (keeps startup simple).

**Managed databases** — workload identity is required for any DB whose
`dbType` is in:

```
{ postgres, mysql, mongodb, redis, mssql, cosmosdb }
```

If `detectedDatabases` includes any of these → emit a `ServiceAccount`
called `<appName>-sa` with the Azure Workload Identity annotations (see
Step 2). Use this SA in the `Deployment.spec.template.spec.serviceAccountName`.

If no managed DB is detected → omit the ServiceAccount, the Deployment
uses the namespace's default SA.

### Step 2 — Build each manifest

Always required: **Deployment**, **Service**.
Conditional: **ConfigMap** (any `config` / `database` env var), **Secret**
(any `secret` env var — values **must be placeholders**, never real
secrets), **ServiceAccount** (managed DB detected),
**HorizontalPodAutoscaler** (production AND `trafficLevel ∈ {high, medium}`).

#### Required labels & annotations (on every resource's `metadata`)

```yaml
metadata:
  name: <appName>[-suffix]
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/managed-by: containerization-assist
  # Include the annotation only if the current `containerization-assist`
  # package version is readable from the environment; otherwise omit
  # the entire `annotations:` block. Never hard-code a stale version.
  annotations:
    com.azure.containerizationassist/version: <version>
```

Do NOT rename the annotation key to `containerization-assist/version`.

#### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: <appName>
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/managed-by: containerization-assist
spec:
  replicas: <from Step 1>
  selector:
    matchLabels:
      app.kubernetes.io/name: <appName>
  template:
    metadata:
      labels:
        app.kubernetes.io/name: <appName>
        app.kubernetes.io/managed-by: containerization-assist
      # If a managed DB is detected, add this annotations block (matches the ServiceAccount + serviceAccountName below).
      # Otherwise omit the entire `annotations:` key.
      annotations:
        azure.workload.identity/use: "true"
    spec:
      # Include `serviceAccountName` only if a managed DB is detected (matches the workload-identity annotation above).
      serviceAccountName: <appName>-sa
      securityContext:
        runAsNonRoot: true
        runAsUser: 10001
        fsGroup: 10001
        seccompProfile: { type: RuntimeDefault }
      containers:
        - name: <appName>
          image: <image>
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: <port>
              protocol: TCP
          env:                                  # one entry per env var (see below)
            - name: PORT
              value: "<port>"
          # Include `envFrom` only for the refs whose backing object you emit:
          # `configMapRef` only if a ConfigMap was emitted; `secretRef` only if a Secret was emitted.
          envFrom:
            - configMapRef:
                name: <appName>-config
            - secretRef:
                name: <appName>-secret
          resources:
            requests:
              cpu: <see Step 1>
              memory: <see Step 1>
            limits:
              cpu: <see Step 1>
              memory: <see Step 1>
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          # Include `livenessProbe` and `readinessProbe` only in production. Omit both in development.
          livenessProbe:
            httpGet:
              path: <probe path>
              port: http
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: <probe path>
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
```

**Env-var emission rules** (per entry in `detectedEnvVars`):

- `classification == "config"` or `"database"` with a `defaultValue` →
  becomes a key in the ConfigMap. Reference via `envFrom.configMapRef`.
- `classification == "config"` or `"database"` without a `defaultValue` →
  becomes a key in the ConfigMap with value `""` (user must fill in).
- `classification == "secret"` → key in the Secret with placeholder value
  `"REPLACE_ME"`. Reference via `envFrom.secretRef`. **NEVER include any
  real `defaultValue` for secret-classified entries.**

If `readOnlyRootFilesystem: true` would break the framework (Java apps
that write to `/tmp`, ASP.NET Core), add a `volumeMounts` + `emptyDir`
volume for `/tmp`:

```yaml
volumes:
  - name: tmp
    emptyDir: {}
# inside container:
volumeMounts:
  - name: tmp
    mountPath: /tmp
```

#### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: <appName>
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/managed-by: containerization-assist
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: <appName>
  ports:
    - name: http
      port: 80
      targetPort: http
      protocol: TCP
```

#### ConfigMap *(only if any non-secret env vars)*

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <appName>-config
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/managed-by: containerization-assist
data:
  <KEY>: "<defaultValue or empty string>"
  ...
```

#### Secret *(only if any secret env vars)*

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <appName>-secret
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/managed-by: containerization-assist
type: Opaque
stringData:
  <KEY>: "REPLACE_ME"
  ...
```

#### ServiceAccount *(only if managed DB detected)*

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: <appName>-sa
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/managed-by: containerization-assist
  annotations:
    azure.workload.identity/client-id: "<REPLACE_WITH_USER_ASSIGNED_IDENTITY_CLIENT_ID>"
```

Also add the Deployment pod-template annotation:
`azure.workload.identity/use: "true"`.

#### HorizontalPodAutoscaler *(only if production AND trafficLevel ∈ {high, medium})*

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: <appName>
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: <appName>
    app.kubernetes.io/managed-by: containerization-assist
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: <appName>
  minReplicas: <replicas from Step 1>
  maxReplicas: <minReplicas * 3, min 5, max 20>
  metrics:
    - type: Resource
      resource:
        name: cpu
        target: { type: Utilization, averageUtilization: 70 }
```

### Step 3 — Self-check before writing

For each generated YAML, confirm:

- [ ] `apiVersion` and `kind` present
- [ ] `metadata.name` matches `<appName>` (or `<appName>-<suffix>`)
- [ ] `metadata.namespace` matches input
- [ ] Required labels (`app.kubernetes.io/name`, `app.kubernetes.io/managed-by`) present
- [ ] Deployment: `runAsNonRoot: true`, `runAsUser` ≥ 1000, no `privileged`, no `hostNetwork`, no `hostPID`
- [ ] Deployment: resources requests **and** limits both set
- [ ] No real secret values (only `REPLACE_ME` placeholders) in Secret
- [ ] If managed DB present: ServiceAccount exists AND Deployment references it AND `azure.workload.identity/use` annotation on pod template
- [ ] YAML parses cleanly (no tabs, consistent 2-space indent)

If any check fails, fix the YAML before writing. Never write a manifest
that fails the security checks.

### Step 4 — Write to disk

- Create `outputDir` (default `<modulePath>/k8s/`) if it doesn't exist.
- Write one file per resource, using these names (skip files for resources
  you did not emit):

  | File | Resource |
  |---|---|
  | `deployment.yaml` | Deployment |
  | `service.yaml` | Service |
  | `configmap.yaml` | ConfigMap |
  | `secret.yaml` | Secret |
  | `serviceaccount.yaml` | ServiceAccount |
  | `hpa.yaml` | HorizontalPodAutoscaler |

- Use the chat environment's file-edit/write capability. Do NOT ask the
  user to copy-paste. Do NOT print the YAML in chat unless the user asks.

### Step 5 — Output

Use this exact format (succinct — the YAML files are the artifact):

````md
**Kubernetes manifests** — `<appName>` in `<namespace>`

### Result
✅ Wrote N manifest(s) to `<outputDir>`:
- `deployment.yaml` — N replicas, image `<image>`
- `service.yaml` — ClusterIP on port 80 → containerPort <port>
- `configmap.yaml` — N keys                  *(omit line if not emitted)*
- `secret.yaml` — N keys (all `REPLACE_ME`)  *(omit line if not emitted)*
- `serviceaccount.yaml` — workload identity for <DB list>  *(omit line if not emitted)*
- `hpa.yaml` — min N, max M, target CPU 70%  *(omit line if not emitted)*

### Configuration summary
- **Replicas:** <N> (<reason — e.g. "production default", "trafficLevel=high × tier-1">)
- **Resources:** requests cpu=<v> mem=<v>, limits cpu=<v> mem=<v>
- **Probes:** <"liveness + readiness on <path>" or "disabled (development)" or "TCP fallback">
- **Security context:** runAsNonRoot=true, readOnlyRootFilesystem=true, capabilities dropped
- **Workload identity:** <"enabled — replace client-id annotation with your User-Assigned Identity ID" or "not required">

### Action required before deploy
- <list every `REPLACE_ME` value in secret.yaml and the workload identity client-id if SA was emitted>
- <"Replace placeholder image <appName>:dev" if image was defaulted>

### Next steps
1. `kubectl apply -n <namespace> -f <outputDir>/`
2. Run **verify-deploy** (tool) to confirm rollout succeeds.
3. Run **scan-image** (tool) on the image before promoting to production.
````

## Constraints

- The artifacts are the files in `<outputDir>`. Always write them to disk.
- NEVER print full YAML in chat unless the user explicitly asks ("show me",
  "print the deployment", etc.).
- NEVER put real secret values in a Secret. Always `REPLACE_ME`.
- NEVER omit `securityContext.runAsNonRoot: true` or `allowPrivilegeEscalation: false`.
- NEVER set `privileged: true`, `hostNetwork: true`, `hostPID: true`, or `hostIPC: true`.
- NEVER pick the `latest` tag for the image. If user gave one, warn and
  proceed but mark it in "Action required".
- NEVER emit `helm` or `kustomize` — out of scope for this skill.
- Manifest order in `kubectl apply -f` matters: this skill writes one file
  per kind, so kubectl handles ordering naturally.

## Failure modes

| Symptom | Action |
|---|---|
| `modulePath` doesn't exist | Echo path; ask for a valid one. STOP. |
| `appName` invalid for DNS-1123 | Normalize; tell user; continue with normalized name. |
| User asked for `helm` / `kustomize` | Decline politely; suggest manual chart authoring. STOP. |
| Cannot infer port AND no `ports` provided | Use `8080`; note in **Configuration summary** that the port is a guess. |
| `outputDir` is not writable | Surface the OS error; ask user for a different path. STOP. |
| User provided real-looking secret values in `detectedEnvVars` | Strip them; write `REPLACE_ME`; surface a warning. |
