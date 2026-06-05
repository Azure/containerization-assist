---
name: fix-dockerfile
description: Validate and remediate an existing Dockerfile against security, performance, and best-practice rules. Use AFTER a Dockerfile exists (either user-authored or produced by generate-dockerfile) and BEFORE building or pushing the image. Triggers include "fix my Dockerfile", "is this Dockerfile secure", "validate Dockerfile", "check Dockerfile for issues", "audit Dockerfile", or any container build flow that needs a sanity check.
argument-hint: <path to Dockerfile | inline Dockerfile content> [environment=production|development]
---

# Fix Dockerfile

Validate an existing Dockerfile against a fixed rule set, score it, and produce
a fully remediated version. Deterministic — every rule below is a regex or a
structural check, not a judgment call.

## Inputs

| Field | Required | Description |
|---|---|---|
| `dockerfile` *or* `path` | yes | Either the file contents OR an absolute path. If `path` is given, read it. |
| `environment` | optional | `production` (default) or `development`. Some rules downgrade to suggestions in dev. |
| `targetPlatform` | optional | `linux/amd64` (default) or `linux/arm64`. Used only in the **Next steps** section. |

If neither is provided → ask the user for the Dockerfile path. STOP.

## Procedure

### Step 1 — Read the file

If `path` is given, read the file content. Reject if it doesn't contain at
least one `FROM ` line (case-insensitive) — that's not a Dockerfile.

### Step 2 — Run every rule

Apply each rule below in order. For each match, record an **issue** with
`{ ruleId, category, severity, line, message }`.

`severity` ∈ `error` (must fix) | `warning` (should fix) | `info` (nice to fix).
`category` ∈ `security` | `performance` | `bestPractices`.

When a rule says "scan lines", split the file on `\n`, trim each line, ignore
empty lines and lines starting with `#`.

#### Security rules (highest priority)

| ruleId | Pattern | Severity | Message |
|---|---|---|---|
| `block-root-user` | Any line matches `^USER\s+(root\|0)\s*$` (case-insensitive multiline) | **error** | Running as root is forbidden. Replace with a non-root user. |
| `no-root-user` | No `USER` line at all, OR final `USER` line argument is `root`/`0` | **error** | Container must declare a non-root `USER` instruction. |
| `block-secrets-in-env` | Any `ENV` or `ARG` line matches `(?i)(PASSWORD\|PASSWD\|SECRET\|TOKEN\|API[_-]?KEY\|PRIVATE[_-]?KEY\|ACCESS[_-]?KEY\|CREDENTIAL\|DATABASE_URL\|CONNECTION_STRING\|REDIS_URL\|MONGODB?_URI\|_DSN)\b.*=\s*\S+` with a non-empty, non-placeholder value | **error** | Hardcoded secret in `ENV`/`ARG`. Inject via runtime (Kubernetes Secret). |
| `no-secrets` | Same as above but covers `RUN export FOO=…` and `RUN echo "FOO=…"` shapes | **error** | Hardcoded secret in `RUN`. |
| `avoid-sudo` | Any line matches `(?i)\bsudo\b` | warning | Drop `sudo`; run the entire stage as the required user instead. |
| `avoid-apt-upgrade` | Any line matches `(?i)apt-get\s+(upgrade\|dist-upgrade)` | warning | Avoid `apt-get upgrade`; pin package versions. |

#### Performance / optimization rules

| ruleId | Pattern | Severity | Message |
|---|---|---|---|
| `optimize-package-install` | `RUN` line with `apt-get install` but NO `rm -rf /var/lib/apt/lists/*` in the same `RUN` | warning | Combine update + install + cleanup in one `RUN`. |
| `apk-no-cache` | `apk add` without `--no-cache` | info | Add `--no-cache` to `apk add`. |
| `recommend-npm-ci` | `npm install` without `-g` | info | Use `npm ci` for reproducible builds. |
| `excessive-run-commands` | Count of `RUN` lines ≥ 6 | info | Combine `RUN` instructions with `&&` to reduce layers. |
| `dependency-order` | `COPY . .` (or `COPY . /<dir>`) appears **before** any `COPY package*.json` / `COPY pom.xml` / `COPY go.mod` / `COPY requirements*.txt` / `COPY Cargo.toml` | info | Copy dependency manifests before source for better layer caching. |
| `recommend-multistage` | Single `FROM` AND any of `mvn`, `gradle`, `npm run build`, `go build`, `cargo build`, `dotnet publish` appears in a `RUN` line | info | Use multi-stage build to drop build tools from the final image. |

#### Best-practice / quality rules

| ruleId | Pattern | Severity | Message |
|---|---|---|---|
| `specific-base-image` | Any `FROM` whose tag is `latest` or missing (no `:`) | warning | Pin a specific major version tag, never `:latest`. |
| `require-healthcheck` | No line starts with `HEALTHCHECK ` | info | Add `HEALTHCHECK` (or note why if image is distroless / shell-less). |
| `require-workdir` | No `WORKDIR` line, OR any `RUN cd /` pattern | info | Use `WORKDIR` instead of `cd`. |
| `recommend-expose` | Application code references a port (`PORT=`, `LISTEN`, `:8080`, `:3000`, `:5000`) but no `EXPOSE` line | info | Add `EXPOSE <port>` to document the listener. |
| `missing-attribution-labels` | No `LABEL com.azure.containerizationassist.createdby` | info | Add the attribution `createdby` label (and `version` if a version is available) for traceability. |

### Step 3 — Score

Start at **100**. Subtract **10** per issue (any severity). Floor at 0.

Grade band:

| Score | Grade |
|---|---|
| 90–100 | A |
| 75–89 | B |
| 60–74 | C |
| 40–59 | D |
| 0–39 | F |

### Step 4 — Compute overall priority

- `high` if any security issue has severity `error`, OR ≥ 3 security issues total.
- `medium` if any security/performance/best-practice issue exists.
- `low` if no issues found.

### Step 5 — Decide remediation strategy

| Conditions | Strategy |
|---|---|
| Grade = F OR ≥ 1 `error` severity issue | `rewrite` — emit a fully restructured Dockerfile. |
| Grade ∈ {C, D} | `refactor` — emit the file with targeted edits per issue. |
| Grade ∈ {A, B} | `tweak` — emit the file with minimal edits; if no issues, return unchanged. |

### Step 6 — Produce **and apply** the fixed Dockerfile

Walk every recorded issue and apply the corresponding fix. The fix table:

| ruleId | Fix |
|---|---|
| `block-root-user` / `no-root-user` | Append (or replace existing) before final `CMD`/`ENTRYPOINT`: `RUN adduser -D -u 10001 appuser` (Alpine) or `RUN useradd -m -u 10001 appuser` (Debian/Azure Linux), then `USER appuser`. Choose flavor from base image. |
| `block-secrets-in-env` / `no-secrets` | Delete the offending `ENV`/`ARG`/`RUN` line. Add a comment `# SECRET: <NAME> — inject via Kubernetes Secret at runtime`. |
| `avoid-sudo` | Drop `sudo ` prefix. If the action requires root, move it before the final `USER` directive. |
| `avoid-apt-upgrade` | Replace with `apt-get install -y --no-install-recommends <explicit packages>`. |
| `optimize-package-install` | Combine into: `RUN apt-get update && apt-get install -y --no-install-recommends <pkgs> && rm -rf /var/lib/apt/lists/*`. |
| `apk-no-cache` | Add `--no-cache` flag. |
| `recommend-npm-ci` | Replace `npm install` with `npm ci`. |
| `excessive-run-commands` | Chain consecutive `RUN` lines with `&&`. |
| `dependency-order` | Move the manifest `COPY` lines above `COPY . .` and add the dependency install step between them. |
| `recommend-multistage` | Restructure into two stages: `FROM <build-image> AS build` … `FROM <runtime-image>` … `COPY --from=build …`. |
| `specific-base-image` | Replace `:latest` (or no tag) with a pinned major version from the curated table in `generate-dockerfile` skill, matching detected language. |
| `require-healthcheck` | Add a HEALTHCHECK matching the base image's available tools: `curl -fsS http://localhost:<port>/health \|\| exit 1` if curl is present; `wget --spider -q http://localhost:<port>/health \|\| exit 1` on Alpine/busybox; for distroless add a comment `# HEALTHCHECK: skipped — distroless image has no shell. Use Kubernetes liveness/readiness probe.` |
| `require-workdir` | Add `WORKDIR /app` after `FROM`. Replace any `RUN cd /<dir>` with `WORKDIR /<dir>`. |
| `recommend-expose` | Add `EXPOSE <port>` near the bottom. Infer port from the code reference. |
| `missing-attribution-labels` | Add `LABEL com.azure.containerizationassist.createdby="containerization-assist"`. If you can read the current `containerization-assist` package version from the environment, also add `LABEL com.azure.containerizationassist.version="<version>"`; otherwise omit the version label — never hard-code a stale version string. Do NOT use `org.opencontainers.image.*` keys. |

After applying all fixes, run **Step 2 again** as a self-check on the new
content. If any rule still triggers, fix it before writing. Never write a
Dockerfile that still violates a security `error` rule.

### Step 7 — Apply the change

- If `path` was provided **and** the file scored below A (any issues found):
  **write the remediated content to that file immediately**, using whatever
  file-editing capability the environment exposes. Do NOT ask the user to
  copy-paste. Do NOT defer this to a “Next steps” list.
- If `path` was provided **and** the file already scored A with 0 issues:
  do nothing — the file is fine as-is.
- If only inline `dockerfile` content was provided (no `path`): you cannot
  write to disk — say so and include the fixed content inline.
- If the user explicitly asks to see the fixed content (“show me the file”,
  “print the Dockerfile”, etc.), include it under an extra **Fixed
  Dockerfile** section. Otherwise omit it — the file on disk is the
  artifact, not the chat output.

## Output format

Use these exact sections in order:

````md
**Dockerfile validation** — `<path or "(inline)">`

### Score
- **Grade:** <A|B|C|D|F> (<score>/100)
- **Priority:** <high|medium|low>
- **Strategy:** <rewrite|refactor|tweak>

### Issues (<N>)

#### Security (<count>)
- *<severity>* `<ruleId>` (line <line>) — <message>
- ...

#### Performance (<count>)
- *<severity>* `<ruleId>` (line <line>) — <message>
- ...

#### Best practices (<count>)
- *<severity>* `<ruleId>` (line <line>) — <message>
- ...

> If a category has 0 issues, write `- none`.

### Result
✅ Wrote remediated Dockerfile to `<path>`. <N> issue(s) fixed.

> Replace this with `✅ Dockerfile already meets the baseline. No changes
> required.` if the input scored A with 0 issues, OR with `⚠️ Inline
> content — no path provided. Fixed content below.` when only inline
> content was supplied.

### Diff summary
- **Added:** <bullet list of new instructions>
- **Removed:** <bullet list of dropped/replaced instructions>
- **Modified:** <bullet list of in-place edits>

> Omit this section entirely if no changes were applied.

### Fixed Dockerfile *(only when no path was given, or when the user asked to see it)*

```dockerfile
<full remediated Dockerfile content>
```

### Next steps
1. Rebuild: `docker buildx build --platform=<targetPlatform> -t <name>:dev <build-context>`.
2. Re-run **fix-dockerfile** to confirm score is **A** (90+).
3. Once clean, proceed to `generate-k8s-manifests`.
````

If the input scores **A with 0 issues**, the output collapses to: the
**Score** section, the **Issues** section (all “none”), and the **Result**
block — nothing else.

## Constraints

- The artifact is the **file on disk**, not the chat output. Always write
  the remediated content to `path` immediately after Step 6 when changes
  are needed.
- NEVER write a Dockerfile that still has any `error`-severity issue.
- NEVER print the full Dockerfile in chat unless (a) no `path` was given,
  or (b) the user explicitly asks to see it.
- NEVER keep a value next to any secret-pattern env var name. Even if the
  user wrote it themselves, surface the violation and remove it.
- NEVER replace `:latest` with another floating tag; always a specific major
  version.
- NEVER add a `HEALTHCHECK` that uses tools not present in the base image
  (e.g. `curl` on distroless). Use the distroless fallback comment.
- Preserve the user's choice of base image **family** (don't switch
  `node:20-alpine` → `mcr.microsoft.com/...` unless `:latest` forced a
  retag).
- Line numbers in the issue list must reference the **original** file.

## Failure modes

| Symptom | Action |
|---|---|
| Neither `dockerfile` nor `path` provided | Ask for one. STOP. |
| `path` doesn't exist or isn't readable | Echo path, ask user to verify. STOP. **Do NOT generate a new Dockerfile** — if the user wants one created, point them at the `generate-dockerfile` skill. |
| File has no `FROM ` line | Reject: "This doesn't look like a Dockerfile." STOP. **Do NOT generate one from scratch** — use `generate-dockerfile` for that. |
| Parse failure (unbalanced quotes, etc.) | Surface a single `parse-error` issue (severity `error`), skip remaining rules, set Grade = F, Strategy = `rewrite`. |
| User insists on keeping a secret in `ENV` | Refuse. Explain that K8s Secrets are the only acceptable path. |
