# Agent Instructions

Instructions for AI coding agents working on this repository.

## Build Verification Before Pushing

This project produces **two build targets** (ESM and CJS) with different TypeScript configs.
The local `npm run typecheck` only checks the ESM build (`tsconfig.json`).
CI also builds CJS (`tsconfig.cjs.json` with `"module": "commonjs"`), which has different
constraints (e.g. `import.meta` is not allowed).

**Always verify both builds pass before pushing:**

```sh
npm run build
```

This runs: `clean` -> `build:version` -> `build:knowledge` -> `build:policies` -> `build:esm` -> `build:cjs`

If a full build is too slow, at minimum check both TypeScript compilations:

```sh
npm run build:version && npm run build:knowledge && npx tsc -p tsconfig.json --noEmit && npx tsc -p tsconfig.cjs.json --noEmit
```

### Common pitfalls

- **`import.meta`** compiles under ESM (`tsconfig.json`, `"module": "ES2022"`) but fails
  under CJS (`tsconfig.cjs.json`, `"module": "commonjs"`). Avoid it in `src/` code.
- **`__dirname` / `require()`** work in CJS but are not available at ESM runtime.
- **Generated files** (`src/lib/generated-version.ts`, `src/knowledge/embedded-packs.ts`)
  are gitignored and created by build steps. They must exist before typecheck or tests.
  Run `npm run build:version` to generate the version file.

## Running Tests

```sh
npm test                # unit + integration + e2e (auto-runs build:version)
npm run test:unit       # unit tests only
```

## Code Style

- TypeScript strict mode is enabled
- Formatting: Prettier (`npm run format:check`)
- Linting: ESLint (`npm run lint`)
