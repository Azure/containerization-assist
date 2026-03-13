---
layout: doc
---

# Manifest & Deployment Tools

Tools for generating Kubernetes manifests, preparing clusters, and verifying deployments.

## `generate-k8s-manifests` — Manifest Generation

Gathers insights and returns a structured plan with requirements for Kubernetes, Helm, ACA, or Kustomize manifest creation. The AI uses this plan to write the actual manifest files. Policies are evaluated during generation. Supports two modes: repository analysis (generate from source) and ACA conversion (convert Azure Container Apps YAML to Kubernetes).

| Input | Required | Description |
| --- | --- | --- |
| `repositoryPath` | One required | Absolute path to the repository root (for generation from source) |
| `acaManifest` | One required | Azure Container Apps manifest content to convert (YAML or JSON) |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `modulePath` | Conditional | Absolute path to module root — required when using `repositoryPath` |
| `name` | No | Module name (defaults to directory basename) |
| `manifestType` | No | Output format: `kubernetes`, `helm`, `aca`, or `kustomize` (default: `kubernetes`) |
| `language` | No | Primary language (e.g., `"java"`, `"python"`) |
| `languageVersion` | No | Language version |
| `framework` | No | Framework (e.g., `"spring"`, `"django"`) |
| `frameworks` | No | Array of `{ name, version }` framework objects |
| `buildSystem` | No | Build system info: `{ type, configFile }` |
| `environment` | No | Target environment (`production`, `development`, etc.) |
| `detectedDependencies` | No | Libraries/features from analysis (e.g., `["redis", "health-checks"]`) |
| `namespace` | No | Target Kubernetes namespace |
| `ports` | No | Ports to expose |
| `entryPoint` | No | Application entry point |
| `targetPlatform` | No | Docker platform (e.g., `linux/amd64`). Defaults to `linux/amd64` |
| `includeComments` | No | Add comments in output (default: `true`, primarily for ACA conversions) |
| `trafficLevel` | No | Expected traffic: `high`, `medium`, or `low` |
| `criticalityTier` | No | Criticality: `tier-1` to `tier-3` |

> Provide either `repositoryPath` + `modulePath` (generate from source) or `acaManifest` (convert ACA manifest), not both.

**Output**: Manifest plan with repository info, security considerations, resource management, best practices, knowledge matches, and policy validation results.

---

## `prepare-cluster` — Cluster Setup

Prepares a Kubernetes cluster for deployment. For Kind clusters, creates a local cluster with a Docker registry. For generic clusters (AKS, EKS, GKE, minikube), validates connectivity and namespace readiness.

| Input | Required | Description |
| --- | --- | --- |
| `clusterType` | No | `kind` (local cluster + registry) or `generic` (existing cluster). Inferred from environment if omitted |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `environment` | No | Target environment for knowledge filtering and policy context |
| `namespace` | No | Kubernetes namespace to prepare |
| `targetPlatform` | No | Platform for cluster validation (e.g., `linux/amd64`). Defaults to `linux/amd64` |
| `strictPlatformValidation` | No | Fail if cluster architecture doesn't match target platform (default: `true`) |

**Output**: Cluster status, namespace readiness, and for Kind clusters: `localRegistryUrl` for pushing images.

---

## `verify-deploy` — Deployment Verification

Verifies a Kubernetes deployment is healthy after applying manifests. Checks pod status, service endpoints, ingress configuration, and health probe responses.

| Input | Required | Description |
| --- | --- | --- |
| `deploymentName` | **Yes** | Deployment name to verify |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `namespace` | No | Kubernetes namespace (defaults to `default`) |
| `checks` | No | Checks to run: `pods`, `services`, `ingress`, `health` (defaults to all) |

**Output**: Status of each check — pod readiness, service endpoints, ingress rules, and health probe results.
