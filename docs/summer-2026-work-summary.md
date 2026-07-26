# Summer 2026 Work Summary — Benjamin Bamisile

A technical walkthrough of the work I landed this summer across two repos, why I
made the decisions I made, and what's still open. It's written for the team so you
can pick up any thread easily and understand the reasoning behind it. None of this
would have been possible without constantly reaching out to and brainstorming with
the team.

The work falls into four connected streams:

1. **Chat skills** — turning CA's MCP tools into VS Code Copilot skills.
2. **Agent evaluation framework** — a harness that measures how much CA actually
   lifts an agent's containerization quality.
3. **Agent-eval ADO pipeline** — an Azure DevOps pipeline that runs the eval
   end-to-end against real Azure infrastructure, on demand.
4. **AKS Tools integration** — shipping the CA skills inside the
   `vscode-aks-tools` extension, gated behind a preview flag.

All four share one theme: **we're migrating CA's capabilities from MCP tools to
skills, and we need to prove that skills change agent behavior at least as well as
the tools do — measurably and honestly.**

---

## 1. Chat skills for VS Code Copilot (PR #685, merged)

**What shipped:** five `SKILL.md` files under [skills/](https://github.com/Azure/containerization-assist/tree/main/skills), plus a
`skills-integrity` unit test.

| Skill | Purpose |
|---|---|
| `analyze-repo` | Detect language/framework/ports before generating anything |
| `generate-dockerfile` | Author an AKS-ready Dockerfile (MCR base, attribution labels) |
| `generate-k8s-manifests` | Author K8s manifests with required labels |
| `fix-dockerfile` | Repair a failing Dockerfile |
| `deploy-to-aks` | Orchestrate the full build → push → apply → verify flow |

**Decision — which tools became skills, and which stayed tools.** The five we
converted are all *generative* work — creating, mutating, or remediating
(authoring a Dockerfile, writing manifests, fixing a broken Dockerfile,
orchestrating a deploy). That kind of work is a natural fit for a skill: the model
is producing artifacts anyway, and markdown guidance shapes *how* it produces
them. We deliberately did **not** convert *validating* tools. Validation has to be
deterministic — an LLM can hallucinate a "pass" — so anything that checks or
verifies (e.g. the scoring/validation logic) stays a real, code-backed tool rather
than prose the model can talk itself around.

**Decision — skills as a re-expression of the MCP tools (a replacement in time,
not yet).** The MCP tools already encode this knowledge programmatically. Skills
re-express the same knowledge as markdown the model reads directly. The end goal
is for skills to *replace* the tools — but that only happens once we've shown they
hold up, which is exactly what the eval framework measures (how well skills match
the MCP path). For now they run alongside the tools. Re-expressing rather than
rewriting also means an agent gets the guidance even when it *isn't* calling our
tools — the gap the eval quantifies. `deploy-to-aks` deliberately lists a
"Tool/skill catalog" so the eval can tell which capabilities are skill-provided vs
tool-provided.

**Decision — write the skills as firm rules, not gentle suggestions.** Models
follow imperative "MUST / no exceptions" directives far more reliably than soft
"you may want to" advice, so `generate-dockerfile` is written that way. Two
concrete examples:

- **Base image (skill section G2):** *"every `FROM` line MUST start with
  `mcr.microsoft.com/` unless the stack appears in the explicit non-MCR list."*
  The skill then pins the exact allowed MCR tags (G2.1) and a closed list of
  sanctioned non-MCR fallbacks (G2.3), so the model picks from a menu instead of
  inventing tags.
- **Attribution label (skill section G3):** *"every Dockerfile MUST contain
  `LABEL com.azure.containerizationassist.createdby="containerization-assist"` …
  No exceptions. A Dockerfile without this label is rejected."*

Writing them this firmly is what turns them into a *measurable* signal. The
base-image rule (G2) is the piece of guidance the `bare` control never receives
(it only arrives via the CA layer), so the base-image check is the cleanest read
on what CA adds. The label requirement, by contrast, is handed to *every* level as
a functional deploy requirement (see the eval section), so it isn't a CA
differentiator on its own.

**Guardrail:** [test/unit/skills-integrity.test.ts](https://github.com/Azure/containerization-assist/blob/main/test/unit/skills-integrity.test.ts)
keeps the skill set from silently drifting.

---

## 2. Agent evaluation framework (PR #714, merged)

**The big one — and the reason the other streams exist.** We're migrating CA from
MCP tools to skills, and this framework is the evidence base for that decision. It
lives under [test/agent-eval/](https://github.com/Azure/containerization-assist/tree/main/test/agent-eval)
and answers one question: *for the same model on the same app, how does its
containerization compare with **no CA** vs **CA's MCP tools** vs **CA's skills**?*
If skills match or beat the tools, the migration is justified by data instead of
by assertion.

### How it's structured

It runs a **gradient** across three levels for each model × fixture cell — these
three levels ARE the comparison the migration hinges on:

- **bare** — a minimal, domain-neutral prompt with no CA tools and no skills. It's
  deliberately *not* given base-image (MCR) guidance — that's the CA layer's job —
  so it's the control for what a model picks unaided. (It *is* told the required
  labels; see "Why each level gets a different prompt".)
- **mcp** — the agent gets CA's MCP tools. What we have today.
- **skills** — the agent gets the CA skill bundle (plus the non-shadowed tools).
  Where we're trying to go.

Each cell is scored on exactly **three checks** ([checks.ts](https://github.com/Azure/containerization-assist/blob/main/test/agent-eval/checks.ts)):

1. `docker-builds` — does the agent's Dockerfile actually build?
2. `requires-azure-base` — did it pick a real, supported MCR base image?
3. `has-required-labels` — did it add the attribution + k8s labels?

Deploy success (`build → push → apply → verify` on a real AKS cluster) is tracked
as **informational for now** — though a clean end-to-end deploy is the real end
goal, so it's expected to graduate into a scored check.

### The most important design decision: the harness is stack-agnostic

The eval **does not build apps or contain any stack-specific logic.** It copies
the fixture in raw, hands it to the agent, and faithfully runs whatever the agent
produces (`docker buildx build … <workingDir>`). There is zero Java/Maven/Go
knowledge in the driver.

**Why this matters:** if a new stack fails end-to-end, the cause is that a CA
*skill or tool* couldn't author a working Dockerfile — not an eval limitation.
Surfacing that gap is the whole point. The one place stack knowledge lives is
*scoring* (`requires-azure-base` recognizes java/node/python/dotnet + more),
never *building*.

### Why each level gets a different prompt

The whole experiment only means something if the **only** thing that differs
between bare, mcp, and skills is the CA layer itself. So the prompts are designed
to isolate that one variable:

- **Labels are given to every level, because they're too specific to guess.** The
  `createdby` Dockerfile label and the `app.kubernetes.io/{name,managed-by}`
  manifest labels are exact strings downstream tooling depends on — `bare` would
  essentially never produce them unprompted, so withholding them wouldn't measure
  anything. They're a functional requirement, not a CA differentiator.
- **Base-image (MCR) guidance is withheld from the plain prompt.** Unlike the
  labels, a model *could* plausibly reach for `mcr.microsoft.com` on its own from a
  "deploy to AKS" prompt — so it's a fair thing to test. No level's task prompt
  mentions it; it only reaches a run through the CA layer (tool recommendations for
  `mcp`, skill hard-rules for `skills`). `bare` gets neither, which makes the
  base-image check the cleanest measure of CA lift.
- **What actually differs per level is the CA layer.** `bare` gets no tools and no
  skills; `mcp` gets the MCP tool bundle plus the AKS dev-loop prompt; `skills`
  gets the `deploy-to-aks` skill bundle loaded into its system prompt, plus the MCP
  tools that aren't already shadowed by those skills (`dropSkillShadowedTools`).
- **All three get the same neutral deploy nudge.** `requireDeploy` is set for every
  level, so when a model stops before deploying it gets a generic "finish
  deploying" reminder — the kind of thing a real user would say to any model.
  That's neutral scaffolding, not CA guidance, so giving it to `bare` too keeps
  deploy attempts comparable without polluting the containerization-quality signal.

### Scoring the Azure base image — and keeping the catalog fresh

One of the three scored checks is "did the agent pick a good base image"
([`requires-azure-base` in checks.ts](https://github.com/Azure/containerization-assist/blob/main/test/agent-eval/checks.ts)),
and it took the most iteration — scoring this *honestly* is genuinely tricky,
because **"use a Microsoft base image" is not a rule that applies to every app.**

**How the validation actually works.** For each `FROM` line in the agent's
Dockerfile (it parses every stage — `--platform`, digests, `AS` aliases, and
`host:port/repo` are all handled), the base is classified into one of these
outcomes:

- **It's already an `mcr.microsoft.com/*` image** → check the tag *exists* (see
  the catalog below). Real tag → **pass**; tag not found → **fail** as a likely
  hallucination (this is the important one — a made-up
  `mcr.microsoft.com/openjdk/jdk:17-azurelinux3.0` *looks* right but doesn't
  exist).
- **It's a public-registry base that MCR has an equivalent for** (e.g.
  `eclipse-temurin`, `node`, `python`, `dotnet/*` at a version MCR ships) →
  **fail** — it should have used the MCR image — with a suggested replacement.
- **It's a base MCR simply doesn't offer** → **pass.** This is the key caveat:
  MCR doesn't ship an image for everything, and we refuse to punish a legitimate
  base just because it isn't Microsoft's. That includes build tool images
  (Maven, Gradle, sbt), app servers (Tomcat, WildFly, Liberty, Jetty, Payara),
  web/proxy servers (nginx, httpd, Caddy, HAProxy), language toolchains with no
  MCR image (Go, Rust, PHP, Ruby, Elixir…), hardened minimals (distroless,
  Chainguard, `scratch`), and generic OS bases (Ubuntu, Debian, Alpine…).
- **Unrecognized base** → **strict fail by default**, because we can't *confirm*
  it's a genuine gap — unless it carries a justification (below).

**The `# WHY-NOT-MCR:` escape hatch.** A comment directly above a `FROM` line
lets an author justify a deliberate non-MCR choice, and it flips an
"MCR-equivalent-exists" or "unrecognized" failure into a pass. The one thing it
*cannot* excuse is a hallucinated MCR tag — you can't annotate your way out of an
image that doesn't exist. This is what makes multi-stage builds score correctly:
a `maven` builder stage passes on its own (no MCR equivalent), and only the final
runtime stage is held to the MCR bar.

**Version gaps are real too.** Even inside a "covered" stack, MCR doesn't publish
every version — e.g. `openjdk/jdk` has no Java 7, and Azure Linux Python is at
3.12 (no 3.9/3.10/3.11). A recognized stack at a version MCR doesn't ship is treated as
a genuine coverage gap and **passes** rather than being flagged, so legacy apps
aren't penalized for a gap they can't close.

**Where the "does this tag exist" data comes from — and how it stays fresh.** We
deliberately do **not** call the MCR registry API at scoring time — that would put
a flaky, rate-limited network dependency inside the scorer and make eval verdicts
nondeterministic. Instead:

- [scripts/refresh-mcr-catalog.ts](https://github.com/Azure/containerization-assist/blob/main/scripts/refresh-mcr-catalog.ts)
  queries live MCR `tags/list` for the tracked repos (openjdk/jdk, Azure Linux
  base/distroless nodejs + python, dotnet sdk/aspnet/runtime), **denoises** the
  raw dump (drops arch suffixes like `-amd64`/`-arm64`, nightly/preview/rc builds,
  and 3-part patch versions — shrinking a ~26k-line dump to ~650 lines), and writes
  [knowledge/catalogs/mcr-base-images.json](https://github.com/Azure/containerization-assist/blob/main/knowledge/catalogs/mcr-base-images.json).
- The check **reads that committed JSON offline.** Tag matching is
  canonical-family aware, so a real-but-more-specific tag (`dotnet/sdk:8.0.11`,
  `nodejs:20.14.0`) still matches, while genuine fakes don't. If the catalog is
  missing entirely, MCR-prefixed images are accepted (fail-open — don't flake a
  whole run over a missing file); the per-stack static version sets are the
  offline fallback.
- A **weekly scheduled workflow** (plus manual dispatch) re-runs the script and
  **opens a PR when the catalog drifts**, so freshness is a reviewable diff — you
  see exactly which tags appeared or disappeared — not a silent runtime lookup. A
  failed repo fetch keeps the previous entry, so a transient MCR outage can't
  corrupt the catalog.

**Why this shape:** scoring stays deterministic and offline-safe (runs anywhere,
repeatably), the registry API is touched *only* in CI, and the rule is strict
about hallucinations while being honest that not every app has — or should have —
an MCR base.

### Other deliberate choices

- **The report is a function, not a template** (`formatGradientHtml`) so it scales
  to any N models × M fixtures.
- **Fixtures are Java legacy apps** (spring-boot-rest-api, spring-mvc-war,
  ejb-ant-monolith) — a realistic modernization corpus. They're fully swappable
  runtime params.

---

## 3. The agent-eval ADO pipeline

With huge help from Suneha, I created an Azure DevOps (1ES) pipeline that runs the eval end-to-end against real
Azure infrastructure and real models, so the gradient can be run on demand without
anyone standing up a local environment. It's a single 1ES template pipeline:
[.azure-pipelines/agent-eval.yaml](https://github.com/Azure/containerization-assist/blob/main/.azure-pipelines/agent-eval.yaml).

**What it does, in order:** installs the toolchain → builds the project → runs the
gradient sweep (build/push/deploy against a real AKS cluster + ACR) → publishes the
HTML heatmap and JSON report as a build artifact.

### Why a pipeline, and the decisions behind it

- **Why a pipeline at all.** A recent corp-tenant change means we can no longer use
  federated credentials there, so the eval can't authenticate the way it used to.
  An Azure DevOps pipeline with a service connection is the sanctioned path
  forward, so that's what we moved to.

- **Manual trigger only, not a PR gate.** A run hits real Azure resources and paid
  LLM endpoints and can take hours, so it's deliberately `trigger: none` /
  `pr: none` — you launch it when you want a fresh fleet result, not on every push.
  This keeps it a measurement tool rather than a blocking check, and keeps cost
  under control.

- **Everything meaningful is a parameter.** Fixtures, models, reps, the
  bare/mcp/skills paths, and the timeout are all
  pipeline parameters with sensible defaults (full Java fixture set × three models
  × 3 reps). Running a different matrix never means editing YAML — you just change
  the parameters at queue time.

- **Auth via the service connection's managed identity.** The pipeline reaches AKS
  and ACR through the service connection's managed identity, and the Foundry
  credentials come from a variable group with a preflight step that fails fast with
  a clear message if they're missing — no half-started runs.

- **The build environment is locked down, so it bootstraps its own toolchain.** The
  1ES agents are non-root Mariner boxes that block the public npm registry, so the
  pipeline routes installs through the internal CFS proxy (restoring the lockfile
  afterward so the tree stays pristine) and installs the Azure CLI / kubectl into a
  user-writable location. This is why the eval "just runs" on a hardened agent.

---

## 4. Shipping the CA skills in the AKS Tools extension

Branch `feat/ca-skills-preview` in `vscode-aks-tools` contributes the CA skills
into the extension, **gated behind `aks.skillsEnabledPreview`** so it ships dark
until we choose to turn it on.

**Status:** the work is functionally complete. It isn't merged yet because of a
**pre-existing integration-test failure that blocks every PR in that repo, not
just this one** — it's unrelated to the skills change and needs to be investigated
and fixed at the repo level before anything can merge. Flagging it here because
it's a shared blocker, not a gap in this work.

### How the CA skills flow into the extension

The single source of truth is the CA repo — the extension never forks the skill
markdown. The path from CA to a shipped `.vsix` is:

1. **CA publishes the skills** as part of the `containerization-assist-mcp` npm
   package (its `skills/` folder).
2. **The extension depends on that package**, so on `npm install` the skills land
   in `node_modules/containerization-assist-mcp/skills`.
3. **At build time they're copied into `dist/skills`.** A VS Code extension's
   `.vsix` only contains what's under `dist/` (the entry point is
   `./dist/extension`), so anything that must ship has to land there. The
   production `webpack` build does this with a `CopyPlugin` pattern
   (`node_modules/containerization-assist-mcp/skills` → `dist/skills`), preceded by
   a small `CleanSkillsPlugin` that wipes `dist/skills` first so a stale copy can't
   linger. The test build (`test-compile`, which runs `tsc`, not webpack) does the
   same copy via `scripts/prepare-test-assets.js`. Both are defensive and fail
   loudly, so we never package an empty skills directory.
4. **The `.vsix` is built from `dist/`**, so `dist/skills` — sourced from the CA
   package — ships inside the extension.

### Why not just check the skills into the repo's `skills/` folder, like kickstart?

The kickstart skills are *authored and owned* in the AKS repo — that repo is their
single source, so living in `skills/` is correct for them. The CA skills are owned
by the **CA repo** and delivered as a **versioned npm dependency**. Copying them
into the AKS `skills/` folder would fork them: two copies of the same markdown,
guaranteed to drift, with a manual re-copy needed on every CA change.

Copying from `node_modules` at build time keeps CA as the single source instead —
bump the `containerization-assist-mcp` dependency and the extension automatically
picks up the new skills. The tradeoff is that the skills aren't visible in the repo
tree (they only exist after an install + build), which is exactly why the copy step
is defensive (clean + fail-loud): the repo can't eyeball them, so the build has to
guarantee they're there.

**Decision — consume, don't duplicate.**

---

## 5. What the eval runs revealed — findings & known failure modes

This section records what the pipeline runs surface right now, so whoever picks
this up next knows where the walls are. The headline: **the eval is sound — it
kicks off the agent, observes, and validates, exactly as designed. It is not
manufacturing the failures. What it surfaces are genuine tool-and-agent-behaviour
findings** (base-image selection, build-tool selection, Dockerfile design, label
stamping), which is the whole point.

### Where end-to-end deploy stands today

Full multi-fixture sweeps still finish with **no cell reaching a verified deploy** —
every run dies in the `dockerBuild` stage before it can push and deploy. The build
failures are not random; they fall into a small number of repeating buckets, and
almost all of them trace back to **restrictions inside the pipeline network
sandbox** rather than to the eval itself.

### The failure modes, in order of how much they hurt us

1. **External-registry pull timeouts — this is our number-one problem.** There is no
   Docker Hub / quay / gcr pull-through mirror in the pipeline, so any time a model
   reaches for `docker.io`, `quay.io`, or `gcr.io` the pull dies on a `192.0.2.x`
   (TEST-NET) i/o timeout. Only `mcr.microsoft.com` is reachable. This single
   restriction accounts for the largest share of build failures across every run.
   Model-side steering (the MCR catalog) reduces how often the agent reaches for
   Docker Hub, but as long as the sandbox blocks those registries it cannot be
   eliminated from the model side alone — the pipeline needs a mirror.

2. **In-build dependency resolution is network-blocked with no proxy.** Even when the
   base image comes from MCR, the build then fails trying to fetch dependencies:
   Maven Central, OS package repos (`apt`/`dnf`/`microdnf`), and Ant tarballs are all
   unreachable inside the build. npm is the one exception — it gets a CFS proxy in
   `agent-eval.yaml`; Maven and OS packages do not. So these failures also reflect
   the sandbox, not agent skill.

3. **`COPY target/…` against a missing artifact.** The agent tends to compile the JAR
   *outside* Docker (writing a `build.sh`) and then `COPY target/…` a pre-built
   artifact. The eval harness only runs `docker buildx build`; it never executes a
   model-written `build.sh`, so `target/` never exists and every rebuild fails
   identically (`lstat …/target: no such file or directory`). The agent loops
   rewriting `build.sh` instead of switching to an in-Dockerfile multi-stage build.

4. **Hallucinated MCR tags and wrong build tool (`exit code: 127`).** The agent
   sometimes invents an MCR tag that doesn't exist, or invokes a build tool the base
   image doesn't have. These are now the *smallest* buckets — the MCR-catalog work
   pushed the agent toward valid bases and correct tooling — but they still show up.

### Environment caveat (what's on the pipeline vs. the agent)

Base-image *selection* failures are a **fair** signal because a reachable option
(MCR) always exists — the agent choosing an unreachable registry is a real behaviour
finding. But buckets 1 and 2 above are **pipeline restrictions**: with the external
registries and dependency mirrors blocked, those cells cannot pass no matter how good
the agent is. Decide deliberately: either (a) keep the block as a realistic
air-gapped-enterprise constraint the agent must design around (multi-stage builds off
MCR bases, vendored deps), or (b) add Docker Hub and Maven/package pull-through
proxies the way npm already has one, so the eval measures agent skill rather than
connectivity.

### The `has-required-labels` gap: mcp does *worse* than bare

Counter-intuitively, the bare path (no CA tools) passes the label check while the
mcp path (with CA tools) fails it. Root cause is a **tool-guarantee gap**, not model
quality: `generate-dockerfile` hands the required `com.azure.containerizationassist.createdby`
label to the model as an *instruction* rather than stamping it, and
`generate-k8s-manifests` does not emit the required `app.kubernetes.io/name` /
`app.kubernetes.io/managed-by` labels at all. The mcp model trusts the tool output
(and drops the Dockerfile LABEL while rewriting during build-fix loops), so whatever
the tools don't enforce gets lost. Bare has no tool to defer to, so it follows the
prompt's label spec by hand; skills passes because the `deploy-to-aks` bundle drives
compliant output. **Fix:** have the CA generators emit these labels directly into the
artifacts (or add explicit label steps to the `aks-loop` mcp prompt).

### Smaller notes for whoever picks this up

- **`fix-dockerfile` is static-analysis only.** It is text-only (lint + policy →
  recommendations): it never sees the `docker build` error output or the build
  context, and it does not rewrite the Dockerfile, so it cannot recover a failing
  build. This is something being addressed on the skills path.
- **The `dockerBuild` retry cap is advisory only.** The prompt says "retry up to 3
  times" but nothing enforces it — cells were observed burning 8–24 build attempts.
  A hard cap would bound cost and turn budget.

The HTML reports for the multi-fixture runs referenced here are checked in under
[`docs/eval-runs/`](./eval-runs/) — see that folder's `README.md` for the
`run#` → date/model/fixture mapping.

---

## Where to look

| Area | Path |
|---|---|
| Skills | [skills/](https://github.com/Azure/containerization-assist/tree/main/skills) |
| Eval harness | [test/agent-eval/](https://github.com/Azure/containerization-assist/tree/main/test/agent-eval) |
| Eval scoring | [test/agent-eval/checks.ts](https://github.com/Azure/containerization-assist/blob/main/test/agent-eval/checks.ts) |
| MCR catalog + refresh | [knowledge/catalogs/](https://github.com/Azure/containerization-assist/tree/main/knowledge/catalogs), [scripts/refresh-mcr-catalog.ts](https://github.com/Azure/containerization-assist/blob/main/scripts/refresh-mcr-catalog.ts) |
| Pipeline | [.azure-pipelines/agent-eval.yaml](https://github.com/Azure/containerization-assist/blob/main/.azure-pipelines/agent-eval.yaml) |
| AKS integration | `vscode-aks-tools` → `feat/ca-skills-preview` (PR #2314) |
