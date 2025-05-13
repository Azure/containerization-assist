# Container Copilot Modules

This document provides an overview of the key modules and their responsibilities within the Container Copilot project.

## Command Line Interface

- **`cmd/`**: CLI entrypoints and command registration (root, generate, test, setup)

## Core Packages

### AI Integration

- **`pkg/ai/`**: Azure OpenAI client wrapper (AzOpenAIClient)

### Client Management

- **`pkg/clients/`**: Aggregates external-facing clients:
  - AzOpenAIClient
  - DockerClient (build/push)
  - KubeRunner (kubectl operations)
  - KindRunner (kind cluster & registry management)

### Docker Support

- **`pkg/docker/`**: Dockerfile templating (`GetDockerfileTemplateName`, `WriteDockerfileFromTemplate`), Draft template integration, build loop

### File Management

- **`pkg/filetree/`**: Scans target repo to build file tree representation for LLM context

### Kubernetes Integration

- **`pkg/k8s/`**: Kubernetes manifest discovery & lightweight parsing (`FindK8sObjects`, `ReadK8sObjects`), Draft manifest template integration

### Pipeline Orchestration

- **`pkg/pipeline/`**: Core orchestration:
  - Dockerfile iteration (analysis ↔ build ↔ fix)
  - Kubernetes manifest iteration (analysis ↔ apply ↔ fix)
  - Snapshot management in `.container-copilot-snapshots`

### Execution

- **`pkg/runner/`**: Abstraction over OS command execution (`CommandRunner`)

## Resources

- **`templates/`**: Embedded Dockerfile templates for multiple languages
- **`hack/`**: Shell scripts (`env.example`, `run-container-copilot.sh`)
- **`utils/`**: Helper functions (`GrabContentBetweenTags`) to extract LLM responses

## Component Interactions

### CLI Execution Flow

- **CLI** (`cmd/generate.go`):
  - Calls `initClients()` to initialize `Clients`
  - Invokes `generate()` passing `Clients`

### Generate Flow (`cmd/generate.go`)

1. Ensure Kind cluster via `Clients.Kind`
2. Generate or write initial Dockerfile (`pkg/docker`)
3. Generate raw k8s manifests (`pkg/k8s`)
4. Build `PipelineState`, load file tree

### Dockerfile Iteration Process

- **`pipeline.IterateDockerfileBuild`**:
  - LLM analysis (`Clients.AzOpenAIClient`)
  - Build via `Clients.Docker`
  - Push image (`Clients.Docker.Push`)

### Manifest Iteration Process

- **`pipeline.IterateMultipleManifestsDeploy`**:
  - LLM analysis on manifests
  - Apply via `Clients.Kube`
  - On success, write final artifacts to disk