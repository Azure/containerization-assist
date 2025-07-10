1  Pull the “quick wins” forward
The checklist in your doc (“Replace context.Background(), remove deprecated wrappers, stop ticker leak, pick one logging framework, etc.) can be landed independently of the big package moves and deliver immediate stability benefits .
Recommendation:

Task	Current slot	Move to
Single logging backend	Phase 2	Day 0 → guard‐rail for all refactors
Ticker‑leak fix	Phase 4 (implicit)	Alongside package flattening (Phase 1)
context propagation skeleton	Phase 4	Start in Phase 1 (even if only plumbing + linter rule)

Rationale: these items touch many files but in a mechanical way; doing them first avoids re‑touching code later and gives your new quality‑gate jobs a clean baseline.

2  Gate each phase with CI and lint rules
You already plan quality‑gates in CI , but make them phase‑aware:

Add a simple YAML matrix variable phase_target and fail PRs that break rules introduced in or before that phase.

Example: “max package depth ≤ 3” rule starts after Phase 1; “RichError everywhere” after Phase 2.

This prevents unfinished work in one branch from blocking unrelated stories while still ratcheting quality forward.

3  Re‑order the error‑handling consolidation
You put “RichError everywhere” in Phase 2 and “context propagation” in Phase 4. Because RichError builders already capture call‑sites, you’ll want real contexts attached before you flip the global lint rule, or you’ll immediately create thousands of context.TODO() violations.

Suggestion: move context propagation plumbing (creating the param and passing it through call‑stacks) to late Phase 1, before the RichError lint in Phase 2. The semantic use of time‑outs can still wait for Phase 4.

4  Merge Tool‑registry unification with DI introduction
Phase 2 merges registries, Phase 3 introduces DI. In practice the unified registry will need constructor injection to avoid hidden globals, so teams will touch the same files twice. Collapsing these into a single “Registry + DI” sprint reduces churn:

Phase 2: Package flattening, circular‑import removal.

Phase 3: Unified registry + lightweight DI container (wire, fx or your own) + delete reflection helpers.

That still leaves Pipeline consolidation and code‑gen in Phase 3 as you proposed.

5  Explicit migration scripts & freeze windows
The plan mentions tools/migrate and scripts/update-imports helpers . Add to the roadmap:

“Dry‑run week‑end” at the end of Phase 1: run the scripts in dry‑run against main, review diff size, update ignore lists.

Code‑freeze window (1–2 hours) when the real move lands, then immediately push the import‑rewriting commit so rebases stay tiny.

6  Coverage and benchmark ratchets per phase
You already track thresholds in .github/coverage‑thresholds.json . Tie them to the same phase_target so that:

Phase	Min global coverage	Key packages
1	keep baseline (≈15 %)	—
2	25 %	runtime, session, retry
3	40 %	all internal/*
4	55 % global, 80 % new code	

Do the same for benchmark P95 targets to catch perf regressions early.

7  Risk board for external integrations
Flattening and import‑path rewrites can break:

go:generate directives

Down‑stream repos or internal consumers

Existing release tags

Add a compatibility risk board per phase with an “owner”, readiness checklist and rollback plan (e.g. temporary re‑export stubs).

8  Document developer ergonomics
High‑churn refactors frustrate day‑to‑day work. Mitigations:

“Autofix” pre‑commit hook that runs update‑imports locally.

Short video / wiki page on the new package map.

Slack bot reminding contributors which phase’s rules are active.

9  Success metrics & exit criteria
Define quantitative DOD for every phase, e.g.:

Metric	Target	Measured by
Max import depth	≤ 3	quality‑gates job
Duplicate retry impls	0 copies	dupl linter
fmt.Errorf in MCP	< 10 (grandfathered)	grep + CI
Registry reflection	0 reflect.* calls	go vet –tags=registry

Exit criteria prevent scope creep and clarify when to roll forward.

10  Timeline realism
Eight weeks is feasible if a core team is dedicated, but consider:

“Unknown‑unknown” buffer: allocate ~20 % slack per phase.

Holidays / release freeze windows.

Parallel workstreams (CI improvements, docs) that don’t block code moves.

If the team is part‑time on this effort, stretch to 12 weeks.

Revised high‑level schedule (suggested)
Week	Major deliverables
1‑2	Quick‑wins patch‑set, context param plumbing, depth/ring lint in CI
3‑4	Package flattening, import rewrite, ticker leak fix, logging mono‑backend
5‑6	Unified registry + DI, delete deprecated wrappers, start RichError lint
7‑8	Pipeline consolidation, code‑gen tem`plates, context deadlines, perf tuning, docs & coverage targets

Bottom line
Your plan is solid; the changes above focus on front‑loading low‑risk wins, aligning phase gates with CI, and reducing double‑touch work. Integrating these tweaks should make the migration smoother for both contributors and downstream consumers.

Happy to iterate on any part of this or dig deeper into risk management for specific modules.