---
name: ca-release
description: 'Release workflow for containerization-assist (CA). Use when: cutting a new release, bumping patch/minor/major version, updating CHANGELOG.md for a release, preparing a release branch, syncing version across package.json and server.json, releasing CA, tagging a version, publishing a new version of containerization-assist.'
argument-hint: 'patch | minor | major'
---

# CA Release Workflow

## When to Use

- Cutting a new `patch`, `minor`, or `major` release of containerization-assist
- Updating `CHANGELOG.md` with changes since the last tag
- Bumping versions across `package.json`, `package-lock.json`, and `server.json`
- Preparing a release branch

---

## Procedure

### Step 1 — Create a release branch

Create a branch named `release-<next-version>` from `main`:

```sh
git checkout main && git pull
git checkout -b release-<next-version>
```

### Step 2 — Identify changes since the last release

Get all commits since the last version tag:

```sh
git log --oneline <last-tag>..HEAD
```

The tag format used in this repo is bare semver (e.g. `1.4.0`, not `v1.4.0`).

Group the commits into categories for the changelog:
- **Features** — `feat:` prefixed or meaningful capability additions
- **Fixes** — `fix:` prefixed commits
- **Dependency updates** — `build(deps):` / `build(deps-dev):` bumps — list PR numbers together on one line
- **Other** — chore, docs, ci, refactor, etc. (include if user-visible)

### Step 3 — Update CHANGELOG.md

Prepend a new `## [<next-version>]` section at the top of [CHANGELOG.md](../../../CHANGELOG.md), following the established format:

```markdown
## [1.4.1]

- feat: database detection + AKS workload identity support (#630)
- add AKS-loop pre-deployment checks (#664)
- fix: add static redirect pages for VS Code MCP install links (#668, #669)
- fix(scan): treat scanType "all" as vulnerability scan (#665)
- dependency updates (#657, #662, #663, #666)
```

Rules:
- One bullet per logical change; group dependency bumps into a single bullet with PR numbers
- Do not include internal/tooling-only commits that have no user impact
- Match the terse, imperative style used in existing entries

### Step 4 — Bump the version

Run `npm version` with `--no-git-tag-version` (so no commit or tag is created — those are managed manually). The `postversion` lifecycle hook automatically syncs `server.json`:

```sh
npm version patch --no-git-tag-version   # x.y.Z
npm version minor --no-git-tag-version   # x.Y.0
npm version major --no-git-tag-version   # X.0.0
```

This updates three files:
| File | Field(s) updated |
|------|-----------------|
| `package.json` | `version` |
| `package-lock.json` | `version` |
| `server.json` | `.version` and `.packages[0].version` (via `postversion` hook) |

### Step 5 — Verify

Confirm all three files reflect the new version:

```sh
node -e "const p=require('./package.json'); const s=require('./server.json'); console.log('pkg:', p.version, 'server:', s.version, s.packages[0].version)"
```

All three values must match.

### Step 6 — Commit and push

```sh
git add package.json package-lock.json server.json CHANGELOG.md
git commit -m "prepare for release <next-version>"
git push origin release-<next-version>
```

Then open a PR from the release branch into `main`.

---

## Files Involved

| File | Role |
|------|------|
| [scripts/postversion.ts](../../../scripts/postversion.ts) | `npm postversion` hook — syncs `server.json` after `npm version` runs |
| [CHANGELOG.md](../../../CHANGELOG.md) | Human-readable release notes |
| `package.json` | Canonical version source |
| `server.json` | MCP registry manifest — must stay in sync |

---

## Notes

- The repo uses bare semver tags (e.g. `1.4.0`, not `v1.4.0`) — use this form in git commands
- Always pass `--no-git-tag-version` to `npm version`; git commits and tags are managed manually
- The `postversion` npm lifecycle hook runs automatically and updates `server.json` — no separate step needed
