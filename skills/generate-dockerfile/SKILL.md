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

**Hard rule: every `FROM` line MUST start with `mcr.microsoft.com/` unless
the stack appears in the explicit non-MCR list (G2.3) below.**

The Dockerfile is the contract for an Azure-hosted image. Public-registry
images (`eclipse-temurin`, `tomcat`, `node`, `python`, `golang`, `rust`)
are **not** "alternatives" — they are fallbacks for the few stacks where
MCR genuinely does not publish a base. Pick from the catalog in G2.1
first; only if no row matches, drop to G2.3.

#### G2.1 — MCR catalog (use these tags, do not invent others)

Use only the tags listed here. If your `<LV>` is not in the list, see
G2.2 (modernize) or G2.3 (documented fallback) — do NOT guess a tag.

**Java — `mcr.microsoft.com/openjdk/jdk`**

| `<LV>` | Build tag | Runtime tag |
|---|---|---|
| 8 | `8-azurelinux` | `8-distroless` |
| 11 | `11-azurelinux` | `11-distroless` |
| 17 | `17-azurelinux` | `17-distroless` |
| 21 | `21-azurelinux` | `21-distroless` |
| 25 | `25-azurelinux` | `25-distroless` |

These are the only Java major versions on MCR (verified against
`https://mcr.microsoft.com/v2/openjdk/jdk/tags/list`). The published tag
format is a **single digit major** — `8-azurelinux`, never `1.8-azurelinux`
or `8u292-jdk-slim` (those do not exist and fail with `image not found`).
MCR does **not** publish a separate `openjdk/jre` repository — for the
runtime stage of a JRE-only image, use the `-distroless` variant of the
same major (`8-distroless`, `17-distroless`, …) which ships only the JRE.

**.NET — `mcr.microsoft.com/dotnet/{sdk,aspnet,runtime}`**

| `<LV>` | SDK | ASP.NET runtime | Generic runtime |
|---|---|---|---|
| 8.0 | `sdk:8.0-azurelinux3.0` | `aspnet:8.0-azurelinux3.0` | `runtime:8.0-azurelinux3.0` |
| 9.0 | `sdk:9.0-azurelinux3.0` | `aspnet:9.0-azurelinux3.0` | `runtime:9.0-azurelinux3.0` |

**Node.js — `mcr.microsoft.com/azurelinux/{base,distroless}/nodejs`**

| `<LV>` | Build tag | Runtime tag |
|---|---|---|
| 18 | `base/nodejs:18` | `distroless/nodejs:18` |
| 20 | `base/nodejs:20` | `distroless/nodejs:20` |
| 22 | `base/nodejs:22` | `distroless/nodejs:22` |

**Python — `mcr.microsoft.com/azurelinux/{base,distroless}/python`**

| `<LV>` | Build tag | Runtime tag |
|---|---|---|
| 3.11 | `base/python:3.11` | `distroless/python:3.11` |
| 3.12 | `base/python:3.12` | `distroless/python:3.12` |

Defaults if `languageVersion` is missing: `java=21`, `dotnet=8.0`,
`node=20`, `python=3.12`.

#### G2.2 — Modernize before falling back

If the project targets a version not in G2.1 (typically Python 3.9/3.10,
Node 16, Java 7):

1. **Preferred:** bump the build target to the nearest LTS in G2.1
   (Python → 3.11, Node → 18). Update package metadata, keep MCR images.
2. Only if modernization is genuinely impossible, drop to G2.3 and
   document the reason in a `# WHY-NOT-MCR:` comment above the
   offending `FROM` line.

#### G2.3 — Allowed non-MCR fallbacks (closed list)

The following bases may appear when, and only when, no MCR row in G2.1
applies. Anything not in this list MUST NOT appear in a `FROM` line.

**Hard rule — build images are build-only.** The images in the **Build
stage** column below (`maven:*`, `gradle:*`, `golang:*`, `rust:*`,
`composer:*`) are builders. They MUST NOT be the final / runtime stage. Using
a non-MCR build image obligates a **multi-stage** Dockerfile whose final
`FROM` is the matching **Runtime stage** image (an MCR `*-distroless` row from
G2.1 wherever one exists). A single-stage Dockerfile that ships a
`maven` / `gradle` / `golang` / `rust` builder as the runtime image is a policy
violation: it defeats the MCR-runtime goal and ships the full build toolchain
plus source in the running image — even though a lone `maven`/`gradle` `FROM`
looks "allowed" here.

| Stack / Reason | Build stage | Runtime stage |
|---|---|---|
| Maven builder (MCR ships no Maven-with-JDK image) | `maven:3.9-eclipse-temurin-<LV>` (where `<LV>` ∈ {8, 11, 17, 21}) | use the matching MCR `openjdk/jdk:<LV>-distroless` for runtime |
| Gradle builder (MCR ships no Gradle image) | `gradle:<ver>-jdk<LV>` | use the matching MCR `openjdk/jdk:<LV>-distroless` for runtime |
| Java EE / Jakarta EE app server (EJB, JSF, full profile — MCR ships no app server) | `maven:3.9-eclipse-temurin-<LV>` or MCR `openjdk/jdk:<LV>-azurelinux` | `jboss/wildfly:<ver>` (WildFly), `payara/server-full:<ver>` (Payara), `tomee:<ver>` (TomEE), or `tomcat:<ver>-jdk<LV>` (servlet-only WAR) |
| Servlet/JSP WAR with no EJB | MCR `openjdk/jdk:<LV>-azurelinux` | `tomcat:<ver>-jdk<LV>` |
| Go (no MCR base) | `golang:<LV>-alpine` | `gcr.io/distroless/static-debian12:nonroot` (CGO off) or `gcr.io/distroless/base-debian12:nonroot` (CGO on) |
| Rust (no MCR base) | `rust:<LV>-slim` | `gcr.io/distroless/cc-debian12:nonroot` |
| PHP (no MCR base) | `composer:2` | `php:<LV>-fpm-alpine` or `php:<LV>-apache` |
| Ruby (no MCR base) | `ruby:<LV>-slim` | `ruby:<LV>-slim` |

**Bare JDK/JRE images (`eclipse-temurin`, `openjdk`, `adoptopenjdk`,
`amazoncorretto`, `ibmjava`) are NOT on this list.** For Java versions
8, 11, 17, 21, or 25, the MCR `openjdk/jdk:<LV>-{azurelinux,distroless}`
row in G2.1 is mandatory — falling back to `eclipse-temurin:8-jre-alpine`
or `adoptopenjdk:8-jre-hotspot` when MCR has the same major is a policy
violation, even with a `# WHY-NOT-MCR:` comment.

**Tag-name precision for Docker Hub `maven` images:** the only published
`maven:3.9-eclipse-temurin-*` tags are `8`, `11`, `17`, `21` — there is
**no** `…-jdk` suffix on these. Tags such as `maven:3.9-eclipse-temurin-8-jdk`
or `maven:3.9-eclipse-temurin-17-jdk` do **not** exist and fail with
`not found`. Use `maven:3.9-eclipse-temurin-17`, not
`maven:3.9-eclipse-temurin-17-jdk`.

When you use any row from G2.3, prepend a `# WHY-NOT-MCR:` comment
above the `FROM` line explaining which condition triggered the
fallback (e.g. `# WHY-NOT-MCR: Java EE app server, no MCR equivalent`).

**Never invent a tag.** If you are tempted to write
`mcr.microsoft.com/<anything>:<LV>-<suffix>` and the exact tag is not
in G2.1, stop — either modernize per G2.2 or fall back per G2.3.

### G3 — Construct the Dockerfile

**Hard rule: every Dockerfile MUST contain
`LABEL com.azure.containerizationassist.createdby="containerization-assist"`,
emitted on the line immediately after the `FROM` of the runtime / final
stage. No exceptions. A Dockerfile without this label is rejected.**

Apply every rule in this checklist:

**Layer/structure**
- One `WORKDIR` early (use `/app`); never use `cd` in `RUN`.
- Copy **dependency manifests first**, install deps, then copy source — to
  preserve layer cache.
- **Source MUST be copied before any build/compile command.** Every build
  stage that runs `mvn` / `./mvnw` / `gradle` / `./gradlew` / `npm run build`
  / `tsc` / `dotnet publish` / `go build` / `cargo build` MUST be preceded
  by a `COPY src ./src` (or equivalent — `COPY . .` for languages without
  a conventional `src/` layout). Skipping the source copy produces an empty
  build that fails with errors like "Unable to find main class" or
  "no main module" at runtime.
- Use `RUN` chaining with `&&` to keep layers small. Aim for ≤ 6 `RUN`
  instructions.
- For multi-stage, give each stage a name (`AS build`, `AS runtime`).

**Build output & dependencies (derive, never assume)**

Where a build writes its artifact, and which dependencies it needs to run,
are determined by the project's own config — not by framework convention.
Two recurring build failures come from guessing instead of reading:

- **Derive the build-output directory; never hardcode it.** Before you
  `COPY` build output into the runtime stage, find where the build tool
  actually writes it:
  1. Read the build tool's config for a configured output path — e.g.
     `build.outDir` in `vite.config.*`, `compilerOptions.outDir` in
     `tsconfig.json`, `outputPath` in `angular.json`, `distDir` in
     `next.config.*`, `output.path` in `webpack.config.*`, `buildDir` in
     `nuxt.config.*`.
  2. If none is set, use that tool's documented default.
  3. If still unsure, run the build in the build stage and `COPY` the
     directory it actually produced.
  `build/` is only Create React App's default. Vite, Vue CLI, Angular and
  most bundlers emit to `dist/`; Next.js to `.next/`. Hardcoding
  `COPY build/ ...` fails with `"/app/build": not found` on any of them.

- **Install full dependencies in the build stage; prune only for runtime.**
  Compilers and bundlers (`tsc`, `vite`, `webpack`, `rollup`, `esbuild`)
  almost always live in `devDependencies`. In the build stage run a full
  install (`npm ci` / `npm install` with **no** `--omit=dev` /
  `--production`). Running `npm ci --omit=dev` before `npm run build`
  removes the compiler and fails with `tsc: not found` / exit 127. Only the
  **runtime** stage may drop dev deps — or, better, copy just the built
  artifact into a clean runtime image and install nothing there.

These two rules hold for any compiled or bundled stack — present or future
— so they replace per-framework "copy from X" recipes rather than adding to
them.

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
- **Package manager per base image** — using the wrong one fails with `command not found`:

  | Base image family | Package manager | Install one-liner |
  |---|---|---|
  | `mcr.microsoft.com/openjdk/jdk:*-azurelinux`, `mcr.microsoft.com/azurelinux/base/*`, `mcr.microsoft.com/dotnet/*-azurelinux*`, `*-mariner*` | **`tdnf`** | `tdnf install -y <pkg> && tdnf clean all` |
  | `mcr.microsoft.com/*-distroless`, `gcr.io/distroless/*` | **none** — no shell, no package manager. Install in the build stage instead and copy artifacts in. |
  | `eclipse-temurin:*-jre-alpine`, `node:*-alpine`, `python:*-alpine` | `apk` | `apk add --no-cache <pkg>` |
  | `eclipse-temurin:*-jdk`, `python:*-slim`, `node:*` (Debian-based) | `apt-get` | as above |

  **`microdnf` does NOT exist on Azure Linux / Mariner.** It ships on RHEL/UBI
  only. Using `microdnf install …` on any `*-azurelinux` or `*-mariner`
  base image fails with `microdnf: command not found`. Use `tdnf` instead
  — same syntax, same flags. Do NOT use `apt-get` on Azure Linux either.

**Distroless final-stage rules (hard):** when the runtime stage `FROM`
ends in `-distroless` (or starts with `gcr.io/distroless/`), the image
has no shell, no package manager, no `useradd`/`adduser`, no `chmod`,
no `mkdir`. Any `RUN` in the distroless stage will fail at build time
with `exec: "/bin/sh": stat /bin/sh: no such file or directory`. Rules:

- **No `RUN` instructions in the distroless stage.** Do all setup
  (chmod, package install, user creation) in the build stage and `COPY`
  the prepared artifacts in.
- **Do not try to `RUN useradd` / `RUN adduser` on distroless** (no shell).
  Set the user with a numeric `USER` instead — which non-root user the base
  provides depends on the image:
  - **Node.js / Python** (`mcr.microsoft.com/azurelinux/distroless/{nodejs,python}`)
    predefine a non-root user `app` at UID 1000 — emit `USER 1000` (preferred)
    or `USER app`.
  - **Java** (`mcr.microsoft.com/openjdk/jdk:<LV>-distroless`) does **NOT**
    predefine a non-root user — its default is root, and there is no `app`
    user. Emit the numeric `USER 1000` (do **not** write `USER app`: that name
    does not exist on the Java distroless image, and a Kubernetes
    `runAsNonRoot` securityContext then fails at deploy with
    `CreateContainerConfigError: ... non-numeric user`).
  Always prefer the **numeric** `USER 1000` on distroless: a numeric UID
  satisfies `runAsNonRoot` without needing an `/etc/passwd` entry.
- **Do not try to `RUN chmod` / `RUN chown` on distroless.** Set the
  mode/owner with `COPY --chmod=…` / `COPY --chown=app:app …` instead
  (BuildKit syntax, supported by default).
- **Skip `HEALTHCHECK`** on distroless — there is no shell to run the
  probe command. Document with `# HEALTHCHECK: skipped — distroless,
  use Kubernetes liveness/readiness probe`.
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

`LABEL` is a Dockerfile instruction that only takes effect inside a build
stage — it MUST appear after a `FROM` line or `docker build` errors out
with `no build stage in current context`. Emit attribution labels
**immediately after the `FROM` line of the final / runtime stage** (and
not before the first `FROM`, not in the build stage).

Always emit, in the runtime stage:
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

**Framework-specific tweaks (illustrative, not an allowlist)**

These rows are worked examples of the general rules above — not an
exhaustive list. For a framework not listed here, apply the same approach:
derive the build-output directory and the run command from the project's
own config and the framework's documented defaults. Never skip
containerizing a stack, or fall back to a guess, just because it lacks a
row in this table.

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

- [ ] **Required label present.** The runtime stage contains `LABEL com.azure.containerizationassist.createdby="containerization-assist"` immediately after its `FROM` line. (If missing, add it now — do not skip.)
- [ ] **Azure base.** Every `FROM` line either starts with `mcr.microsoft.com/` (using a tag from the G2.1 catalog) **or** matches a row in the G2.3 fallback list with a `# WHY-NOT-MCR:` comment above it. No invented MCR tags.
- [ ] **No builder as runtime.** If any `FROM` uses a G2.3 *Build stage* image (`maven`/`gradle`/`golang`/`rust`/`composer`), the Dockerfile is multi-stage and its **final** `FROM` is the matching G2.3 *Runtime stage* image (or an MCR G2.1 image) — never the builder itself.
- [ ] **Source copied before build.** Every `RUN` that invokes a build tool (`mvn`/`mvnw`, `gradle`/`gradlew`, `npm run build`, `tsc`, `dotnet publish`, `go build`, `cargo build`) is preceded by a `COPY` of the source tree.
- [ ] Single non-root `USER` in the final stage
- [ ] No hardcoded secrets in any `ENV`
- [ ] No `:latest` tag, no `apt-get upgrade`
- [ ] `WORKDIR` set, no `cd` in `RUN`
- [ ] Multi-stage chosen per rule G1
- [ ] `EXPOSE` matches detected listener port
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
