# Developer Guide

This guide provides a single entry point for contributors working on the Containerization Assist MCP server. It summarizes architecture, build tooling, testing strategy, AI behaviour, and release workflows, and links to detailed references where appropriate.

---

## 1. Project Overview

- **TypeScript MCP server** that orchestrates containerization tools (Docker, Kubernetes, scanning, etc.).
- **Single-operator workflow**: one session per user, sequential tools, deterministic AI outputs.
- **Public API/CLI**: MCP server executable (`containerization-assist-mcp`/`ca-mcp`) plus published TypeScript exports.
- **Currently Active Tools**: 4 tools enabled (see `src/tools/index.ts` → `ALL_TOOLS`)
  - `analyze-repo` (AI-enhanced)
  - `generate-dockerfile-plan` (Knowledge-enhanced planning)
  - `generate-manifest-plan` (Knowledge-enhanced planning)
  - `validate-dockerfile` (Utility)
- **In Development**: 17 additional tools (commented out in `ALL_TOOLS`)

Key directories:

```
src/
├── app/            # Runtime entry, orchestrator
├── cli/            # CLI wrappers for MCP server
├── tools/          # Tool implementations, schemas, metadata
├── mcp/            # MCP adapters (context, server, AI helpers)
├── ai/             # Prompt builders, knowledge enhancement
├── infra/          # Docker + Kubernetes clients
├── lib/            # Shared utilities (e.g. workflow hints)
├── config/         # Config loading, policies
└── validation/     # Validation helpers (AI-enhanced and static)
```

More detail: `README.md`, `CLAUDE.md`, and `docs/tool-capabilities.md`.

---

## 2. Build & Tooling

### 2.1 Builds

- **Dual output**: ESM (`dist/`) and CommonJS (`dist-cjs/`).
- **Commands**:
  - `npm run build` – clean build (runs `build:esm` then `build:cjs`).
  - `npm run build:esm` – `tsc -p tsconfig.json && tsc-alias -p tsconfig.json --resolve-full-paths`.
  - `npm run build:cjs` – `tsc -p tsconfig.cjs.json && tsc-alias -p tsconfig.cjs.json`.
  - `npm run build:watch` – watch-mode ESM build.

The package `exports` map relies on both directories; verify `npm run build` before publishing.

**Build Verification:**
- Both builds produce JavaScript files, TypeScript declarations (`.d.ts`), declaration maps (`.d.ts.map`), and source maps (`.js.map`).
- ESM files use ES module syntax (`import`/`export`), no `"use strict"` directive.
- CJS files use CommonJS syntax (`require`/`exports`), with `"use strict"` directive.
- Run `npm run test:unit -- test/unit/build-validation.test.ts` to verify all package exports exist and have correct module format.
- Run `npx tsx scripts/verify-build.ts` for a standalone verification script that checks build outputs against package.json exports.
- Build artifacts are validated in CI before publishing.

### 2.2 Scripts

- `npm run validate` – **Single validation entry point**: runs lint (`eslint`), type-check (`tsc --noEmit`), and unit tests (`jest --selectProjects unit`). This is the only validation command used both locally and in CI.
- `npm run fix` – **Auto-fix issues**: runs `lint:fix` + `format` to automatically fix linting issues and format code.
- `npm run test` / `npm run test:unit` – Jest with ESM support.
- `npm run lint` / `npm run lint:fix` – ESLint checking and fixing.
- `npm run format` / `npm run format:check` – Prettier formatting and checking.
- `npm run quality:gates` – optional in-depth quality suite (lint metrics, dead code, build timing). **Implemented as a TypeScript script** (`scripts/quality-gates.ts`) with no external dependencies. **Runs in reporting-only mode by default** (no file mutations). Use `UPDATE_BASELINES=true npm run quality:gates` to update baselines (CI-only). See `docs/quality-gates.md` for details.
- `npm run smoke:journey` – end-to-end workflow (requires Docker).

Husky pre-commit hooks currently run lint-staged; quality gates moved to CI and default to reporting-only mode locally.

### 2.3 Tooling Simplification Targets

See `plans/tooling-simplification-plan.md` for ongoing cleanups (script consolidation, workflow caching, etc.).

---

## 3. Architecture Highlights

### 3.1 Runtime & Orchestration

- `src/app/index.ts` exposes `createApp`. Public runtime provides stateless tool orchestration.
- `src/app/orchestrator.ts` coordinates tool execution with policy enforcement and parameter validation.
- Tools are pure functions that take input parameters and return results.
- Shared workflow hints helper (`src/lib/workflow-hints.ts`) generates next-step recommendations.

### 3.2 Tool Pattern

Each tool exports `Tool<typeof schema, ResultType>` with co-located schema/metadata. Example (`src/tools/generate-dockerfile/tool.ts`):

```ts
import type { Tool } from '@/types/tool';
import { generateDockerfileSchema } from './schema';

async function run(input, ctx) { /* ... */ }

const tool: Tool<typeof generateDockerfileSchema, AIResponse> = {
  name: 'generate-dockerfile',
  description: '...',
  schema: generateDockerfileSchema,
  metadata: { samplingStrategy: 'single', /* ... */ },
  run,
};

export default tool;
```

- Use path aliases (`@/…`) for internal modules.
- Tools operate as stateless functions, receiving all needed input as parameters.

### 3.3 AI & Deterministic Sampling

- `src/mcp/ai/sampling-runner.ts`: `sampleWithRerank` returns a single deterministic candidate with optional scoring metadata.
- Tool metadata sets `samplingStrategy: 'single'` for AI-driven tools, `'none'` for non-AI tools.
- `docs/ai-enhancement.md` covers enhancement architecture (knowledge enhancement, validation suggestions).

---

## 4. Testing Strategy

- **Unit tests**: Located under `test/unit/`. Use `jest` with `--experimental-vm-modules`.
- **Integration tests**: `test/integration/` includes MCP inspector suites, Docker/Kubernetes checks, workflow validation.
- **Smoke journey**: `npm run smoke:journey` orchestrates a full tool chain (requires Docker environment).
- **Quality gates**: Optional script for lint/unused exports metrics (`scripts/quality-gates.sh`). Primarily run in CI.

See `docs/quality-gates.md` for threshold details.

---

## 5. Documentation & Resources

- `docs/tool-capabilities.md` – detailed list of tools and enhancement capabilities.
- `docs/ai-enhancement.md` – architecture of AI generation/knowledge enhancement.
- `docs/error-guidance.md` – standard error structures and guidance patterns.
- `docs/examples/` – example MCP usage scenarios and prompts.

Use this guide for the big picture; dive into specific docs for in-depth topics.

---

## 6. Release & CI Workflows

### 6.1 GitHub Actions

- `test-pipeline.yml` – runs validation pipeline with build artifact sharing. The main test job builds once and uploads artifacts, which the security scan job reuses (eliminating duplicate npm install and builds). Includes npm dependency caching and build output caching.
- `release.yml` – tags trigger validation, build, npm publish, Docker image publish, and GitHub release. Build artifacts are created once in the validate-release job and reused by both publish-npm and create-github-release jobs. Docker builds use buildx with layer caching for faster subsequent builds.

### 6.2 Workflow Optimization Details

**Build Artifact Sharing:**
- Test pipeline: Single build in test job → uploaded as artifact → downloaded by security job
- Release pipeline: Single build in validate-release job → reused by publish-npm and create-github-release jobs
- Eliminates redundant `npm ci` and `npm run build` calls, reducing CI time significantly

**Caching Strategy:**
- **NPM dependencies**: `actions/cache` with npm cache dir (shared across workflow runs)
- **Build outputs**: Cache `dist/`, `dist-cjs/`, `.tsbuildinfo` based on source file hashes
- **Docker layers**: buildx with registry cache-from/cache-to for faster image builds
- **Artifacts**: Short-lived artifacts (1 day retention) for intra-workflow sharing

**Performance Impact:**
- Test pipeline: ~40% faster by eliminating duplicate install/build in security job
- Release pipeline: ~30% faster by reusing build artifacts across 3 jobs
- Docker builds: ~50% faster with layer caching on subsequent releases

### 6.3 Publishing Checklist

1. Ensure `npm run validate` and `npm run build` succeed locally.
2. **Verify build outputs**:
   - Run `npm run test:unit -- test/unit/build-validation.test.ts` to ensure all package exports exist and are correctly formatted.
   - Run `npx tsx scripts/verify-build.ts` for a comprehensive verification of all build artifacts.
3. Update changelog if necessary.
4. Push git tag `vX.Y.Z` or trigger `workflow_dispatch` with version.
5. Confirm release workflow completes (NPM package, GitHub release, container image).

---

## 8. Getting Started Checklist for Contributors

1. Clone repository & install dependencies (`npm install`).
2. Run `npm run validate` to ensure environment is healthy.
3. Run `npm run build` to generate ESM/CJS outputs (verify `dist/` + `dist-cjs/`).
4. **Verify build integrity**:
   - Run `npm run test:unit -- test/unit/build-validation.test.ts` to check that all outputs are correctly generated.
   - Run `npx tsx scripts/verify-build.ts` to verify build artifacts match package.json exports.
5. Review `src/tools/` for tool patterns and `docs/tool-capabilities.md` to understand existing tool metadata.
6. For AI-related changes, read `docs/ai-enhancement.md` and `src/mcp/ai/sampling-runner.ts`.
7. Before submitting PRs:
   - Run `npm run validate`.
   - Optionally run `npm run quality:gates` (CI will enforce thresholds).
   - For significant changes, run `npm run smoke:journey`.
8. Ensure documentation (`README`, relevant docs/*.md`) reflects new behaviour.

---

This guide should evolve alongside the codebase. If major architectural or tooling changes land, update this file (and link the relevant detailed docs) so new contributors have an accurate starting point.
