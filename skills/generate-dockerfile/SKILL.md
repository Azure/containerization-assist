---
name: generate-dockerfile
description: Generate or enhance a Dockerfile for a containerized application using curated base-image recommendations and security best practices. Use AFTER analyze-repo (or after the user has told you the language/framework). Triggers include "generate a Dockerfile", "containerize this", "write Dockerfile for X", or when running aks-loop and no Dockerfile yet exists. If a Dockerfile already exists, this skill enhances it in place.
argument-hint: <modulePath> <language> [framework] [environment=production|development]
---

# Generate Dockerfile

Produce a production-quality, secure Dockerfile (or enhancement plan for an
existing one) for a single module. The output is a complete Dockerfile that
the user can write to disk — not a description of one.

## Inputs

| Field | Required | Description |
|---|---|---|
| `modulePath` | yes | Absolute path of the module to containerize (one of the modules from `analyze-repo`). |
| `language` | yes | `java` \| `dotnet` \| `javascript` \| `typescript` \| `python` \| `go` \| `rust` \| `php` \| `ruby`. |
| `languageVersion` | recommended | E.g. `21`, `8.0`, `20`, `3.11`. Substituted into the base image tag. |
| `framework` | optional | E.g. `spring-boot`, `express`, `next`, `django`, `flask`, `asp.net-core`, `gin`. |
| `environment` | optional | `production` (default) or `development`. |
| `targetPlatform` | optional | `linux/amd64` (default) or `linux/arm64`. |
| `detectedDependencies` | optional | Used to choose extra apt/apk packages (e.g. native modules). |
| `detectedEnvVars` | optional | List from `analyze-repo`. Used to add `ENV` instructions for `config`/`database` vars and to WARN about `secret`-classified ones (never bake secrets in). |
| `existingDockerfile` | optional | If a Dockerfile is already at `<modulePath>/Dockerfile`, read its content and follow the **Enhancement path** instead of the **Generation path**. |

## Prerequisite: analyze-repo

This skill needs `language`, `framework`, `detectedDependencies`, and
`detectedEnvVars` from the `analyze-repo` skill. If you don't already
have that output for `<modulePath>` from earlier in this conversation,
follow the `analyze-repo` skill against `<modulePath>` first, then come
back.

## Decision: generate or enhance?

1. Check if `<modulePath>/Dockerfile` exists.
2. If **no** → Generation path (Step G1–G4 below).
3. If **yes** → read the file, run **Dockerfile analysis** (Step E1), then
   Enhancement path (Step E2–E4).

---

## Generation path

### G1 — Choose build strategy

Use **multi-stage** if any of these:

- `language` ∈ {`java`, `go`, `rust`, `dotnet`}
- `language` is `typescript` AND build output is required (`tsc`, `next build`, `vite build`, etc.)
- `buildSystem.type` ∈ {`maven`, `gradle`}

Otherwise use **single-stage**.

The reason this matters: multi-stage drops compilers, SDKs, and source code
from the final image — typically 70–90% smaller and a smaller attack surface.

### G2 — Choose the base image(s)

Prefer the **first matching row**. Substitute the major version into the tag.

| Language / Stack | Build stage | Runtime stage |
|---|---|---|
| `java` (Spring Boot, generic) | `mcr.microsoft.com/openjdk/jdk:<LV>-azurelinux` or `eclipse-temurin:<LV>-jdk` | `mcr.microsoft.com/openjdk/jdk:<LV>-distroless` or `eclipse-temurin:<LV>-jre-alpine` |
| `dotnet` (ASP.NET Core) | `mcr.microsoft.com/dotnet/sdk:<LV>-azurelinux` | `mcr.microsoft.com/dotnet/aspnet:<LV>-azurelinux` |
| `dotnet` (worker / CLI) | `mcr.microsoft.com/dotnet/sdk:<LV>-azurelinux` | `mcr.microsoft.com/dotnet/runtime:<LV>-azurelinux` |
| `javascript` / `typescript` | `mcr.microsoft.com/azurelinux/base/nodejs:<LV>` or `node:<LV>-alpine` | `mcr.microsoft.com/azurelinux/distroless/nodejs:<LV>` or `node:<LV>-alpine` |
| `python` | `mcr.microsoft.com/azurelinux/base/python:<LV>` or `python:<LV>-slim` | `mcr.microsoft.com/azurelinux/distroless/python:<LV>` or `python:<LV>-slim` |
| `go` | `golang:<LV>-alpine` | `gcr.io/distroless/static-debian12:nonroot` (CGO disabled) or `gcr.io/distroless/base-debian12:nonroot` (CGO enabled) |
| `rust` | `rust:<LV>-slim` | `gcr.io/distroless/cc-debian12:nonroot` |
| `php` (web) | `composer:2` | `php:<LV>-fpm-alpine` or `php:<LV>-apache` |
| `ruby` (rails) | `ruby:<LV>-slim` | `ruby:<LV>-slim` |

Defaults if `languageVersion` is missing: `java=21`, `dotnet=8.0`, `node=20`,
`python=3.11`, `go=1.22`, `rust=1.75`, `php=8.3`, `ruby=3.3`.

### G3 — Construct the Dockerfile

Apply every rule in this checklist:

**Layer/structure**
- One `WORKDIR` early (use `/app`); never use `cd` in `RUN`.
- Copy **dependency manifests first**, install deps, then copy source — to
  preserve layer cache.
- Use `RUN` chaining with `&&` to keep layers small. Aim for ≤ 6 `RUN`
  instructions.
- For multi-stage, give each stage a name (`AS build`, `AS runtime`).

**Security baseline (mandatory)**
- Final stage MUST end with a `USER` instruction set to a non-root user.
  - If the base image already provides one (e.g. `nonroot`, `appuser`), use
    it; otherwise create one with **two separate instructions** (`USER` is a
    Dockerfile instruction, not a shell command — it cannot be chained after
    `RUN ... &&`):
    - Alpine:
      ```
      RUN adduser -D -u 10001 appuser
      USER appuser
      ```
    - Debian / Azure Linux:
      ```
      RUN useradd -m -u 10001 appuser
      USER appuser
      ```
- **Never** include any of these in `ENV`:
  `PASSWORD`, `PASSWD`, `TOKEN`, `SECRET`, `CREDENTIAL`, `API_KEY`,
  `PRIVATE_KEY`, `ACCESS_KEY`, `*_DSN`, `DATABASE_URL`, `CONNECTION_STRING`,
  `REDIS_URL`, `MONGODB_URI`. These get injected at runtime via K8s Secrets.
- Do NOT run `apt-get upgrade` / `apt-get dist-upgrade`.
- If using `apt-get install`, clean up:
  `apt-get update && apt-get install -y --no-install-recommends <pkgs> && rm -rf /var/lib/apt/lists/*`.
- Pin **major** version tags (`:21`, `:8.0`, `:3.11`), never `:latest` and
  never floating patch versions like `21.0.3`.

**Quality baseline**
- Include `HEALTHCHECK` for long-running services (web servers, workers).
  Pick the simplest probe the runtime image actually supports:
  - Has `curl`: `HEALTHCHECK CMD curl -fsS http://localhost:<port>/health || exit 1`
  - Alpine / busybox (no curl, has wget): `HEALTHCHECK CMD wget --spider -q http://localhost:<port>/health || exit 1`
  - Distroless / no shell: skip HEALTHCHECK and add a comment `# HEALTHCHECK: skipped — distroless image has no shell. Use Kubernetes liveness/readiness probe.`
- `EXPOSE <port>` for the primary listener.
- `CMD` in **exec form** (`["binary","arg"]`), required for distroless.

**Env variables**
- For each entry in `detectedEnvVars` classified as `config` or `database`:
  if it has a `defaultValue`, emit `ENV NAME=<value>`; otherwise emit
  `# ENV NAME=<set at runtime>` as a comment.
- For each entry classified as `secret`: emit `# SECRET: NAME — inject via
  Kubernetes Secret at runtime, do NOT bake into the image`.

**Required labels (attribution)**

Always emit:
```
LABEL com.azure.containerizationassist.createdby="containerization-assist"
```

If you can read the current `containerization-assist` package version
from the environment (e.g. its `package.json`), also emit:
```
LABEL com.azure.containerizationassist.version="<version>"
```
Otherwise omit the version label — never hard-code a stale version string.
Do NOT substitute either key for `org.opencontainers.image.*`.

**Framework-specific tweaks**

| Framework | Add |
|---|---|
| Spring Boot | `ENV JAVA_OPTS="-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0"`, copy fat JAR from `target/*.jar`, `ENTRYPOINT ["java","-jar","/app/app.jar"]` |
| Next.js | Build with `npm run build`; copy `.next/standalone`, `.next/static`, `public`; `CMD ["node","server.js"]` |
| Django | `RUN python manage.py collectstatic --noinput`; run with `gunicorn` |
| FastAPI | `CMD ["uvicorn","app.main:app","--host","0.0.0.0","--port","8000"]` |
| ASP.NET Core | `dotnet publish -c Release -o /out`; runtime stage runs `dotnet <App>.dll` |
| Go | `CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app`; copy single binary into distroless |
| Rust | `cargo build --release`; copy `target/release/<bin>` into distroless |

### G4 — Validate the result against this self-check

Before emitting, confirm:

- [ ] Single non-root `USER` in the final stage
- [ ] No hardcoded secrets in any `ENV`
- [ ] No `:latest` tag, no `apt-get upgrade`
- [ ] `WORKDIR` set, no `cd` in `RUN`
- [ ] Multi-stage chosen per rule G1
- [ ] `EXPOSE` matches detected listener port
- [ ] `LABEL com.azure.containerizationassist.createdby` is present
- [ ] `CMD`/`ENTRYPOINT` in exec form

If any item fails, fix the Dockerfile **before** showing it to the user.

---

## Enhancement path

### E1 — Analyze the existing Dockerfile

Read the file. Compute:

- `baseImages` = every line starting with `FROM ` (after `FROM `, take the
  first whitespace-delimited token).
- `isMultistage` = `baseImages.length > 1`.
- `hasHealthCheck` = any line starts with `HEALTHCHECK ` (case-insensitive).
- `hasNonRootUser` = any `USER` line whose argument is not `root` or `0`.
- `instructionCount` = number of lines whose first token is one of:
  `FROM, RUN, CMD, LABEL, EXPOSE, ENV, ADD, COPY, ENTRYPOINT, VOLUME,
  USER, WORKDIR, ARG, ONBUILD, STOPSIGNAL, HEALTHCHECK, SHELL`.
- `complexity` = `complex` if `instructionCount > 20` or `isMultistage`;
  else `moderate` if `> 10`; else `simple`.
- `securityPosture` = `good` if `hasNonRootUser && hasHealthCheck`; `poor`
  if `!hasNonRootUser && !hasHealthCheck`; else `needs-improvement`.

### E2 — Compute the diff plan

- **Preserve** (always keep): multi-stage structure if present, existing
  HEALTHCHECK if present, non-root USER if present, existing base image
  unless it's `:latest` or unsupported.
- **Improve** (rewrite in place): any rule from G3 that the current file
  violates (root user, hardcoded secrets, `apt-get upgrade`, missing
  cleanup, `cd` in `RUN`, floating tags, > 6 `RUN` instructions when
  combinable, missing labels).
- **Add missing**: non-root USER if absent, HEALTHCHECK if absent and the
  image has a shell, required `LABEL`s if absent, `WORKDIR` if absent.

### E3 — Pick the strategy

| Conditions | Strategy |
|---|---|
| `securityPosture == "poor"` OR (`improve.length + addMissing.length > 5`) | `major-overhaul` |
| `securityPosture == "needs-improvement"` OR (`improve.length + addMissing.length > 2`) | `moderate-refactor` |
| Otherwise | `minor-tweaks` |

### E4 — Produce the enhanced Dockerfile

Compose the **full** updated Dockerfile (do not show a diff). Apply every
relevant item from G3 to bring it to baseline. Preserve the listed items
verbatim. Re-run the G4 self-check before writing.

---

## Writing the file (mandatory)

As soon as the Dockerfile passes the G4 self-check, **write it to
`<modulePath>/Dockerfile`**. Do not ask the user for confirmation first;
do not stage it in the response. If a Dockerfile already exists, overwrite
it (the Enhancement path already incorporates anything worth preserving).

Do **NOT** print the full Dockerfile content in the chat response. The
user can open the file on disk. Only include excerpts when:

- the user explicitly asks to see it ("show me the Dockerfile", "print it"),
- you need to highlight a specific change (≤ 10 lines of context), or
- the write failed and you need to surface the content for manual recovery.

## Output format

Use exactly these sections, in order. Keep the response short — the file
on disk is the artifact.

````md
**Dockerfile <generation|enhancement>** — written to `<modulePath>/Dockerfile`

### Plan
- **Strategy:** <multi-stage build | single-stage build | minor-tweaks | moderate-refactor | major-overhaul>
- **Base image (build):** `<image>` <only if multi-stage>
- **Base image (runtime):** `<image>`
- **User:** `appuser` (UID 10001) <or "preserved from existing">
- **Healthcheck:** <yes/no — reason if no>

### Env vars
- `<NAME>` *<config|database>* → emitted as `ENV`
- `<NAME>` *secret* → **runtime injection only** (Kubernetes Secret)
- ...

### Next steps
1. Build locally: `docker buildx build --platform=<targetPlatform> -t <module>:dev <modulePath>`
2. Run **fix-dockerfile** to lint and remediate the file against the built-in security and best-practice rules.
3. Once validated, scan the built image and proceed to `generate-k8s-manifests`.
````

If the result was an **enhancement**, also include this section between
**Plan** and **Env vars**:

````md
### Changes from existing
- **Preserve:** <bullet list>
- **Improve:** <bullet list>
- **Add:** <bullet list>
````

## Constraints

- The Dockerfile written to disk must be **complete and runnable as-is**
  (no placeholders, no `<TODO>` markers, no truncation).
- NEVER use `FROM <image>:latest`.
- NEVER set any of the secret-pattern env vars in the file. If the user
  insists, refuse and explain why.
- NEVER include `apt-get upgrade` or `apt-get dist-upgrade`.
- NEVER end the final stage as `root`.
- NEVER omit the `com.azure.containerizationassist.createdby` label, and NEVER replace either attribution label key with `org.opencontainers.image.*`.
- NEVER print the full Dockerfile in the chat response by default — write
  it to disk and reference the path. Print only when the user asks or to
  show a short excerpt.
- The Dockerfile must pass the G4 self-check before being written. If a
  constraint conflicts with a user instruction, surface the conflict
  explicitly and ask before proceeding.

## Failure modes

| Symptom | Action |
|---|---|
| `modulePath` doesn't exist | Echo the path, ask user to provide a valid one. STOP. |
| No `language` provided and no manifest file at `modulePath` | Ask the user for the language, OR suggest running `analyze-repo` first. STOP. |
| Existing Dockerfile is empty or unreadable | Treat as Generation path, note this in **Plan**. |
| Existing Dockerfile already passes the G4 self-check | Strategy = `minor-tweaks`; report "no changes required"; leave the file untouched. |
| Write to `<modulePath>/Dockerfile` fails (permissions, read-only FS) | Surface the error and the Dockerfile content for manual save. |
| User has a custom registry / non-standard base image preference | Use the user's choice as base image, still apply all G3 baseline rules. |
