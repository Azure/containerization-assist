# Containerization Assist MCP — Product Requirements Document (PRD)

**Version**: 0.1.0  
**Last Updated**: 2025-10-01  
**Owner**: Containerization Assist Core Team

---

## 1. Vision & North Star

Enable a single engineer, working inside GitHub Copilot, to migrate one legacy application into a container-ready deployment with confidence in less than one working session. The MCP server must provide:
- AI-guided generation of Docker/Kubernetes assets with deterministic, debuggable output;
- Operational tools (build, scan, tag, push, deploy, verify) that gracefully surface actionable remediation guidance; and
- A predictable, transcript-friendly experience that Copilot can replay, summarize, and build on.

Success means the engineer finishes migration without leaving Copilot and without guessing at infrastructure fixes.

---

## 2. Target Users & Scenarios

| User | Scenario | Key Outcomes |
|------|----------|--------------|
| Application Engineer | Migrates an existing web service into Docker/Kubernetes using Copilot prompts | Receives a deterministic Dockerfile and manifest, can build/scan/deploy, sees actionable guidance when operations fail |
| Platform Engineer | Validates tooling before wider rollout | Runs the scripted smoke test, inspects logs and error hints, configures policy controls |
| Maintainer | Extends toolchain with new generators or policies | Reuses standardized logging/error guidance patterns and leverages documentation/examples |

Constraints:
- Single-user, single-application workflows
- Offline-friendly except for Docker/Kubernetes operations.
- Copilot UI is the only user interface; logs must be readable as plain text transcripts.

---

## 3. Product Principles

1. **AI-First, Deterministic Output** — AI drives generation, but sampling must resolve to a single candidate with reproducible diagnostics.
2. **Actionable Remediation** — Every failure returns a `Result` with structured guidance (message, hint, resolution, details).
3. **Transcript Fidelity** — Logs use standardized "Starting …" / "Completed …" phrasing plus key metadata so Copilot conversations remain readable.
4. **Incremental Enhancements** — All changes land with tests or documented manual validation; no dormant toggles or hidden tools.
5. **Policy Safety Rails** — Static policy configuration (`policyPath`, `policyEnvironment`) validates orchestrated runs before execution.

---

## 4. Requirements by Area

### 4.1 AI Generation
- Single-candidate sampling (`count: 1`) across all generators, with scoring metadata captured for diagnostics.
- Content extraction utilities centralize parsing logic (Dockerfile, YAML, JSON).
- Regression tests assert deterministic sampling settings and failure fallback behavior.

### 4.2 Operational Tooling
- Tools rely on `createStandardizedToolTracker` to emit `Starting <tool>` / `Completed <tool>` logs with timing.
- Operational failures (build, push, tag, scan, deploy, verify) propagate structured guidance from infrastructure helpers.
- Session state retains latest successful outputs and workflow hints (e.g., next recommended tool).

### 4.3 Infrastructure Clients
- Docker & Kubernetes clients wrap errors via `extractDockerErrorGuidance` / `extractK8sErrorGuidance`.
- Fast-fail behavior for missing Docker daemon or kubeconfig (no silent timeouts).
- Unit and integration suites cover representative error codes and ensure hints/resolutions stay actionable.

### 4.4 Orchestrator & Policy
- `createOrchestrator` enforces single-session semantics and loads policies exactly once per run.
- Policy evaluation blocks unsafe runs with clear failure guidance; hot-reload is explicitly out of scope.
- `createApp` exposes only the simplified configuration surface (`policyPath`, `policyEnvironment`).
- Session state persists across tool executions and clears on server shutdown.

### 4.5 Logging & Telemetry
- Reusable helpers in `@/lib/runtime-logging` standardize startup/shutdown, tool execution, and failure messages.
- CLI leverages the same helpers to align terminal output with Copilot transcripts.
- Future telemetry hooks must integrate via these helpers to maintain formatting parity.

### 4.6 Documentation & Examples
- `README.md` and `docs/examples` demonstrate the sequential workflow (analyze → generate → fix → build → scan → tag → deploy → verify) using `npm run smoke:journey`.
- A smoke-test script orchestrates the real CLI path and is referenced by both CI and docs.
- Additional guidance lives in `docs/error-guidance.md`, `docs/ai-enhancement.md`, `docs/quality-gates.md`, and `docs/examples/*`.

---

## 5. Roadmap Alignment

The PRD aligns with the project's focus on:

- Single-session runtime with deterministic AI sampling
- Operator-focused enhancements (guidance, logging, Kubernetes ergonomics, documentation)
- Quality-gated development process

---

## 6. Release Criteria & Metrics

| Category | Metric / Gate |
|----------|----------------|
| Smoke Journey | `npm run smoke:journey` green locally and in CI |
| Error Guidance | All infrastructure-facing failures return `Result` with `guidance` (hint/resolution), verified by integration tests |
| Logging | Each tool logs standardized start/complete/failure lines (unit tests hook loggers to assert messages) |
| Docs | README + examples validated via scripted check (`npm run docs:verify`, TBD) |
| Quality | `npm run validate` (lint + typecheck + unit tests) passes; deterministic sampling tests enforce single-candidate behavior |

---

## 7. Open Questions & Risks

- **Kubernetes Reachability**: Need faster detection of unreachable clusters without lengthy timeouts.
- **Registry Authentication**: Additional tooling may be required for secure credential handling (future enhancement).
- **Examples vs. Real Workspaces**: Example scripts assume specific fixtures; formal verification commands must catch drift.
- **Policy Evolution**: Static policy files suffice for now; revisit dynamic or cloud-hosted policies when multi-team demands arise.


## 9. Appendices

- **Error Guidance Details**: `docs/error-guidance.md`
- **AI Enhancement System**: `docs/ai-enhancement.md`
- **Quality Gates**: `docs/quality-gates.md`
- **Session State Guide**: `docs/session-state-guide.md`
- **Tool Capabilities**: `docs/tool-capabilities.md`
- **Developer Guide**: `docs/developer-guide.md`
- **Example Usage**: `docs/examples/`

