---
name: analyze-repo
description: Detect languages, frameworks, build systems, dependencies, databases, and environment variables in a repository **for the purpose of containerization or cloud deployment**. Use BEFORE generating a Dockerfile, fixing a Dockerfile, or generating Kubernetes manifests. Triggers require a containerization/deployment intent: "analyze my repo for containerization", "what do I need to containerize this", "detect my stack so I can deploy", "containerize this app", "prep this for AKS / Kubernetes / Docker". Do **NOT** trigger on generic analysis requests like "analyze my repo", "explain this codebase", "what does this project do", "review my code", or "summarize this repo" — those are not containerization tasks.
argument-hint: [optional absolute path to repository; defaults to workspace root]
---

# Analyze Repository

Produce a structured stack report for a repository by reading its manifest
files. This is a **deterministic, read-only** procedure — do not guess, do
not invoke language models against source code, and do not modify anything.
Use file-reading tools available in the chat environment (`read_file`, file
search, directory listing) to perform every step.

## Inputs

| Field | Required | Description |
|---|---|---|
| `repositoryPath` | yes | Absolute path. Default to the workspace root if the user did not specify one. Normalize Windows paths to forward slashes. |

If the user provided a `modules[]` array, skip detection and use it verbatim.
Otherwise, run the full procedure.

## Procedure

### Step 1 — Walk the repository

- Start at `repositoryPath`, recurse to **maximum depth 3**.
- **Skip these directory names** at any depth:
  `node_modules`, `.git`, `.vscode`, `.idea`, `dist`, `build`, `target`,
  `bin`, `obj`.
- **Cap total files scanned at 100.** Stop once you reach this.
- Record a flat directory tree (first 30 entries) for context.

### Step 2 — Identify config files

Match file basenames against this set (case-sensitive):

| Ecosystem | Files |
|---|---|
| Node | `package.json` |
| Java – Maven | `pom.xml` |
| Java – Gradle | `build.gradle`, `build.gradle.kts` |
| Python | `requirements.txt`, `pyproject.toml` |
| Rust | `Cargo.toml` |
| .NET | `*.csproj`, `*.fsproj`, `*.vbproj` |
| Go | `go.mod` |
| PHP | `composer.json` |
| Ruby | `Gemfile` |
| Container | `Dockerfile`, `docker-compose.yml`, `docker-compose.yaml` |
| Spring | `application.properties`, `application.yml`, `application.yaml` |
| Env templates | `.env.example`, `.env.sample`, `.env.template` |

Read each match, **truncate at 1000 characters**, and remember the (path,
content) pair.

### Step 3 — Parse each config

Per file type, extract the following fields. If a field is absent, leave it
unset — do not invent.

| File | Extract |
|---|---|
| `package.json` | `language` = `typescript` if `typescript` in devDeps or `tsconfig.json` exists, else `javascript`; `framework` from deps (`express`, `next`, `nestjs`, `fastify`, `koa`, `react`, etc.); `frameworkVersion` from dep range; `dependencies` = keys of `dependencies` + `devDependencies`; `languageVersion` from `engines.node`; `entryPoint` from `main` or `scripts.start`; `ports` from any literal port in `scripts.start`. `buildSystem.type` = `npm` / `yarn` / `pnpm` based on lockfile sibling. |
| `pom.xml` | `language` = `java`; `framework` from `<artifactId>` patterns (`spring-boot-starter*` → `Spring Boot`); `languageVersion` from `<maven.compiler.source>` or `<java.version>`; `dependencies` = `<groupId>:<artifactId>` list; `buildSystem.type` = `maven`. |
| `build.gradle[.kts]` | `language` = `java` (or `kotlin` if `.kts` + kotlin plugin); `framework` from `plugins { id 'org.springframework.boot' }` etc.; `languageVersion` from `sourceCompatibility` / `jvmToolchain`; `dependencies` = the strings inside `implementation '…'` / `api '…'`; `buildSystem.type` = `gradle`. |
| `requirements.txt` | `language` = `python`; `dependencies` = one per line stripped of version pin; `framework` from deps (`flask`, `django`, `fastapi`, `tornado`); `buildSystem.type` = `pip`. |
| `pyproject.toml` | `language` = `python`; read `[project] dependencies`, `[tool.poetry.dependencies]`, or `[tool.pdm]`; `framework` from deps; `languageVersion` from `requires-python`; `buildSystem.type` = `poetry` / `pdm` / `setuptools` based on `[build-system].build-backend`. |
| `Cargo.toml` | `language` = `rust`; `framework` from deps (`actix-web`, `axum`, `rocket`, `tonic`); `dependencies` = keys of `[dependencies]`; `entryPoint` = `[package].name`; `buildSystem.type` = `cargo`. |
| `*.csproj` | `language` = `dotnet`; `languageVersion` from `<TargetFramework>`; `framework` = `ASP.NET Core` if any `Microsoft.AspNetCore.*` package ref; `dependencies` = `PackageReference Include="…"` values; `buildSystem.type` = `dotnet`. |
| `go.mod` | `language` = `go`; `languageVersion` from `go <version>` line; `dependencies` = `require (...)` module list; `framework` from deps (`gin-gonic/gin`, `labstack/echo`, `fiber`, `chi`); `buildSystem.type` = `go-modules`. |
| `composer.json` | `language` = `other` (PHP); `dependencies` = keys of `require`; `framework` from deps (`laravel/framework`, `symfony/*`); `buildSystem.type` = `composer`. |
| `Gemfile` | `language` = `other` (Ruby); `dependencies` = `gem "…"` names; `framework` from deps (`rails`, `sinatra`); `buildSystem.type` = `bundler`. |
| `Dockerfile` | Note its existence — do NOT treat as a module config; affects "recommended next step". |

### Step 4 — Group configs into modules

- Group parsed configs by **directory of the config file**.
- Each directory with ≥ 1 parsed config becomes one `ModuleInfo` with:
  - `name` = basename of directory
  - `modulePath` = absolute directory path (forward slashes)
  - `language` ∈ {`java`, `dotnet`, `javascript`, `typescript`, `python`,
    `rust`, `go`, `other`}
  - `frameworks` = `[{ name, version? }]`
  - `buildSystems` = list of `{ type, languageVersion? }` (one per parsed
    config in the dir)
  - `dependencies` = combined dependency list (deduped)
  - `ports`, `entryPoint`
- `isMonorepo` = `modules.length > 1`.

### Step 5 — Detect databases

Walk every module's `dependencies[]`. Lowercase each, then match:

**Exact-name table** (lowercased):

| Database | Dependency names |
|---|---|
| `postgres` | `pg`, `pg-pool`, `pg-native`, `postgres`, `postgresql`, `psycopg2`, `psycopg2-binary`, `asyncpg`, `npgsql`, `tokio-postgres` |
| `mysql` | `mysql`, `mysql2`, `pymysql`, `mysqlclient`, `mysql-connector-java`, `mysql-connector-python` |
| `mongodb` | `mongodb`, `mongoose`, `pymongo`, `motor`, `mongodb.driver`, `mongo-go-driver` |
| `redis` | `redis`, `ioredis`, `redis-py`, `go-redis`, `stackexchange.redis` |
| `mssql` | `mssql`, `tedious`, `pymssql` |
| `sqlite` | `sqlite3`, `better-sqlite3`, `aiosqlite` |
| `cosmosdb` | `@azure/cosmos`, `microsoft.azure.cosmos` |
| `elasticsearch` | `@elastic/elasticsearch`, `elasticsearch`, `elasticsearch-py`, `olivere/elastic` |

**Pattern matches** (regex on lowercased dep):

| Pattern | Database |
|---|---|
| `(?:^|:)postgresql(?:$|:)` or `^org\.postgresql:` or `jackc/pgx(?:\/|$)` or `(?:^|\/)lib\/pq(?:$|\/)` | `postgres` |
| `(?:^|:)mysql[_-]connector` or `^go-sql-driver/mysql` | `mysql` |
| `^go\.mongodb\.org/mongo-driver` | `mongodb` |

Each detected DB: `{ dbType, dependencies: [<names that matched>] }`.

### Step 6 — Detect environment variables

Walk this list for each module (using only config files in the module's
directory or in the repo root for non-root modules):

**a) `.env.example` / `.env.sample` / `.env.template`** — line-based, safe
even if truncated:
- Match `^([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?$`. Skip blank lines and `#`.
- `required` = no `=` or empty value after `=`. Strip surrounding quotes
  from the value.

**b) `docker-compose.{yml,yaml}`** — parse as YAML; **skip if truncated**
(truncation breaks YAML). For each service, read `environment`:
- List format `- VAR=val` or `- VAR`
- Map format `VAR: val`
- Same `required` rule.

**c) `application.properties` / `application.yml`** — extract `${VAR}` /
`${VAR:default}` placeholders via regex. Each becomes one entry; `defaultValue`
from the `:default` suffix if present.

**d) Framework inference** — append these based on `(language, framework)`:

| Stack | Inferred vars |
|---|---|
| node + express | `PORT`, `NODE_ENV` |
| node + next | `PORT`, `NODE_ENV`, `NEXT_PUBLIC_*` (hint only) |
| python + flask | `FLASK_ENV`, `FLASK_APP`, `PORT` |
| python + django | `DJANGO_SETTINGS_MODULE`, `SECRET_KEY`, `DEBUG`, `ALLOWED_HOSTS` |
| python + fastapi | `PORT`, `LOG_LEVEL` |
| java + spring-boot | `SERVER_PORT`, `SPRING_PROFILES_ACTIVE`, `JAVA_OPTS` |
| dotnet + asp.net core | `ASPNETCORE_ENVIRONMENT`, `ASPNETCORE_URLS` |
| go + gin/echo/fiber | `PORT`, `GIN_MODE` (gin only) |

**Classify each var** by name (uppercased):

- **secret** — matches any of: `DATABASE_URL$`, `CONNECTION_STRING$`, equals
  `REDIS_URL` / `MONGODB_URI` / `MONGO_URI`; ends with `_DSN` or equals `DSN`;
  contains `PASSWORD|PASSWD|TOKEN|SECRET|CREDENTIAL|API_KEY|PRIVATE_KEY|ACCESS_KEY`;
  contains `_AUTH_` / starts `AUTH_` / ends `_AUTH`.
- **database** — starts with `DB_`, `REDIS_`, `MONGO_`, `MYSQL_`, `PG_`,
  `POSTGRES_`, `MSSQL_`, or `DATABASE_`.
- **config** — everything else.

**Redact** `defaultValue` for any var classified as `secret` — never surface
credential-shaped defaults.

**Deduplicate** by `name` — keep the first occurrence (priority: explicit
config files over framework inference).

### Step 7 — Build the summary string

Template (mirror this exactly):

```
✅ Analyzed repository at <repositoryPath>. Detected <module description>.<monorepo clause><database clause><env clause> Ready for Dockerfile generation.
```

- `<module description>` — for 1 module: `<language> project`. For >1:
  `N modules (<lang1>, <lang2>, ...)`.
- `<monorepo clause>` — ` Monorepo structure identified.` if `isMonorepo`.
- `<database clause>` — ` Databases: <comma-separated dbTypes>.` if any.
- `<env clause>` — ` N env var(s) detected.` if any.

### Step 8 — Output

Use the exact format in the next section.

## Output format

Use these exact headings:

````md
**Repository analysis** — `<repositoryPath>`

> <summary line from Step 7>

### Stack
- **Language:** <language> <languageVersion if present>
- **Framework:** <framework name + version, or "none detected">
- **Build system:** <buildSystem.type> (<languageVersion>)
- **Entry point:** <entryPoint or "not detected">

### Dependencies of note
- <up to 5 bullets — focus on items that affect the container:
  database drivers, native modules, gRPC/protobuf, web frameworks. Skip
  generic libraries like lodash, chalk, requests.>

### Databases detected
- <one bullet per detected DB: `<dbType>` (from `<dep1>, <dep2>`)> or "none"

### Environment variables to surface
- <up to 10 bullets: `<NAME>` *<classification>* (from `<source>`, required: <bool>)>
- ...and N more — <if list was truncated>

### Ports
- <comma-separated list, or "not detected">

### Structure
- <"Single project" OR "Monorepo with N modules:" then one bullet per module:
  `- <name> (<language>) — <modulePath>`>

### Recommended next step
<choose one:
 - "Run **generate-dockerfile** on `<modulePath>`."
 - "An existing Dockerfile was detected — run **fix-dockerfile** instead."
 - "This is a monorepo. Tell me which module to containerize first: <list>."
 - "No buildable project found — verify the path and retry.">
````

## Constraints

- NEVER dump raw config file contents.
- NEVER invent fields. If a parser step yields nothing, write "not detected".
- NEVER recurse deeper than 3 levels or scan more than 100 files.
- NEVER report a `defaultValue` for a secret-classified env var.
- For a monorepo, list **every** module — never abbreviate.
- Target ≤ 25 lines of output for a single-module repo, ≤ 40 for a monorepo.
- If a section has no content, keep the header and write "none" / "not
  detected" — never omit a section.

## Failure modes

| Symptom | Action |
|---|---|
| `repositoryPath` doesn't exist or isn't a directory | Echo the path, ask user to provide a valid absolute path. STOP. |
| Walked the tree but found zero config files | Tell the user "No recognizable project files (package.json, pom.xml, requirements.txt, Cargo.toml, go.mod, *.csproj, etc.) under `<path>`." STOP. |
| Every module ends up with `language: "other"` | Note this in the report; ask the user which language so downstream skills can proceed. |
| A config file parses with no usable fields | Skip it silently — do not fail the whole analysis. |
| `docker-compose.yml` was truncated | Skip the env-var extraction for it; still include it in the dependency picture. |
