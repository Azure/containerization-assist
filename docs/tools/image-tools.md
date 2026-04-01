---
layout: doc
---

# Image Tools

Tools for analyzing repositories, generating Dockerfiles, building images, and managing container artifacts.

## `analyze-repo` — Repository Analysis

Analyzes repository structure and detects technologies by parsing config files. Identifies languages, frameworks, build systems, dependencies, and monorepo modules. Use this as the first step before generating Dockerfiles or manifests.

| Input | Required | Description |
| --- | --- | --- |
| `repositoryPath` | **Yes** | Absolute path to the repository root |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `modules` | No | Pre-analyzed modules — skip auto-detection if provided |

**Output**: Detected modules with language, framework, build system, dependencies, ports, and entry point for each.

---

## `generate-dockerfile` — Dockerfile Generation

Gathers insights from the knowledge base and returns a structured plan with requirements for Dockerfile creation. The AI uses this plan to write the actual Dockerfile. Policies are evaluated during generation — violations and warnings appear in the output.

| Input | Required | Description |
| --- | --- | --- |
| `repositoryPath` | **Yes** | Absolute path to the repository root |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `modulePath` | No | Module path for monorepo projects |
| `language` | No | Primary language (e.g., `"java"`, `"python"`) |
| `languageVersion` | No | Language version (e.g., `"17"`, `"3.11"`) |
| `framework` | No | Framework (e.g., `"spring"`, `"django"`) |
| `environment` | No | Target environment (`production`, `development`, etc.) |
| `detectedDependencies` | No | Libraries/features from analysis (e.g., `["redis", "mongodb"]`) |
| `targetPlatform` | No | Docker platform (e.g., `linux/amd64`, `linux/arm64`). Defaults to `linux/amd64` |
| `trafficLevel` | No | Expected traffic: `high`, `medium`, or `low` |
| `criticalityTier` | No | Criticality: `tier-1` (mission-critical) to `tier-3` (low-priority) |

**Output**: Build strategy, base image recommendations, security considerations, optimizations, best practices, and policy validation results.

---

## `fix-dockerfile` — Dockerfile Analysis & Fixes

Analyzes an existing Dockerfile for security, performance, and best practice issues. Returns categorized findings with knowledge-based fix recommendations and a validation score. Policies are evaluated — organizational violations appear alongside technical issues.

| Input | Required | Description |
| --- | --- | --- |
| `dockerfile` | One required | Dockerfile content to analyze |
| `path` | One required | Path to Dockerfile to analyze |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `environment` | No | Target environment (`production`, `development`, etc.) |
| `targetPlatform` | No | Platform for context (e.g., `linux/amd64`) |
| `policyPath` | No | Path to a specific policy file (defaults to all discovered policies) |

> Provide either `dockerfile` (content) or `path` (file location), not both.

**Output**: Categorized issues (security, performance, best practices), fix recommendations, validation score (A–F grade), and policy validation results.

---

## `build-image-context` — Build Context Preparation

Prepares Docker build context with security analysis and returns ready-to-execute build commands. Validates the Dockerfile, analyzes BuildKit features, computes tags, and checks for security issues — all before the build runs.

| Input | Required | Description |
| --- | --- | --- |
| `path` | No | Build context directory path |
| `dockerfile` | No | Dockerfile name (relative to context) |
| `dockerfilePath` | No | Absolute path to Dockerfile |
| `imageName` | No | Image name for tagging |
| `tags` | No | Tags to apply to the image |
| `buildArgs` | No | Build arguments (`key=value` pairs) |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `platform` | No | Target platform (e.g., `linux/amd64`) |

**Output**: Validated paths, security analysis with risk level, build configuration, BuildKit feature analysis, Dockerfile analysis, and a build command ready to execute.

---

## `scan-image` — Vulnerability Scanning

Scans Docker images for security vulnerabilities with AI-powered remediation guidance. Supports multiple scanners with configurable severity thresholds and focus areas.

| Input | Required | Description |
| --- | --- | --- |
| `imageId` | **Yes** | Docker image ID or name to scan |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `severity` | No | Minimum severity to report: `LOW`, `MEDIUM`, `HIGH`, `CRITICAL` |
| `scanType` | No | Scan type: `vulnerability`, `config`, or `all` (default: `vulnerability`) |
| `scanner` | No | Scanner engine: `trivy`, `snyk`, `grype`, or `osv` (default: `osv`) |
| `enableAISuggestions` | No | Enable AI-powered remediation suggestions (default: `true`) |
| `aiEnhancementOptions` | No | AI config: `mode`, `focus`, `confidence`, `maxSuggestions`, `includeExamples` |

**Output**: Vulnerability findings with severity, affected packages, and AI-generated remediation guidance.

---

## `tag-image` — Image Tagging

Tags a Docker image with a new version or registry tag. Simple utility for applying semantic versions, build numbers, or registry-qualified names.

| Input | Required | Description |
| --- | --- | --- |
| `imageId` | **Yes** | Docker image ID to tag |
| `tag` | **Yes** | New tag to apply |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |

---

## `push-image` — Registry Push

Pushes a Docker image to a container registry. Supports credential-based auth and Docker credential helpers. Use after tagging with the target registry.

| Input | Required | Description |
| --- | --- | --- |
| `imageId` | **Yes** | Docker image ID or name to push |
| `registry` | **Yes** | Target registry hostname (e.g., `myregistry.azurecr.io`) |
| `workspacePath` | No | Current workspace path (for project-scoped policy discovery) |
| `platform` | No | Target platform (e.g., `linux/amd64`) |
| `credentials` | No | Registry credentials (`username`, `password`). Falls back to Docker credential helpers |
