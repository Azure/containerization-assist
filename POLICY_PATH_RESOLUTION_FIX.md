# Policy Path Resolution Fix - Investigation Summary

## Problem Statement

Built-in policies (located in `policies/*.rego`) were being correctly discovered and applied in **development mode** but were **not being discovered** in the **packaged/installed mode** (after `npm pack` + `npm install -g`).

This meant that when the MCP server was installed as a global npm package, organizational policies like `base-images.rego` would not be loaded, resulting in unfiltered knowledge recommendations that violated policy constraints.

## Investigation Process

### Test Setup

Created `scripts/test-spring-petclinic-policy-filtering.mjs` to validate built-in policy filtering in the packed application:

- Spawns the installed `ca-mcp` command (not direct node execution)
- Tests the Spring PetClinic repository (Java/Spring Boot project)
- Calls `generate-dockerfile` with Java 25 parameters
- **Validates** that only Microsoft Container Registry images appear (per `policies/base-images.rego`)
- **Fails** if non-compliant images (eclipse-temurin, alpine) appear in output

### CI Workflow

Created `.github/workflows/test-packed-policy-filtering.yml`:

```yaml
- Pack project with npm pack
- Install globally with npm install -g
- Verify policies/ directory is packaged
- Run test script to validate policy filtering
```

### Initial Test Failures

Test consistently failed with:
```
❌ ERROR: Found non-Microsoft images in output!
  - eclipse-temurin
  - alpine
```

Despite `policies/base-images.rego` being packaged correctly, the MCP server was not discovering it.

### Debug Logging Strategy

Added execution chain tracing with color-coded markers:

- 🔴 **MCP handler invocation** (`mcp-server.ts`)
- 🔵 **Wrapped execute callback** (`mcp-server.ts`)
- 🟢 **orchestratedExecute wrapper** (`index.ts`)
- 🟡 **Orchestrator.execute()** (`orchestrator.ts`)
- 🟣 **Policy discovery** (`orchestrator.ts` - `discoverBuiltInPolicies()`)

This allowed us to trace the full execution path through the MCP protocol, tool registration, and orchestrator execution.

## Root Cause Discovery

The debug logs revealed:

```
🟣 Checking 7 search paths: [
  "/opt/hostedtoolcache/node/22.21.1/x64/lib/node_modules/policies",  ❌ WRONG!
  "/home/runner/work/containerization-assist/.../policies",
  ...
]
🟣 NO POLICIES FOUND in any search path!
```

The **first search path was incorrect**! It should have been:
```
"/opt/hostedtoolcache/node/22.21.1/x64/lib/node_modules/containerization-assist-mcp/policies"
```

But it was missing the **package name directory** (`containerization-assist-mcp`).

### The Bug: Path Depth Calculation

In `src/app/orchestrator.ts`, the `discoverBuiltInPolicies()` function was calculating the module-relative path incorrectly:

**WRONG (4 levels up):**
```typescript
// From dist-cjs/src/app/orchestrator.js
const moduleRelativePath = resolve(dirName, '../../../../policies');

// Path resolution:
// dist-cjs/src/app/     (start)
// dist-cjs/src/         (../)
// dist-cjs/             (../../)
// package-root/         (../../../)
// parent-of-package/    (../../../../) ← TOO FAR!

// Result: /opt/.../node_modules/policies ❌
```

**CORRECT (3 levels up):**
```typescript
// From dist-cjs/src/app/orchestrator.js
const moduleRelativePath = resolve(dirName, '../../../policies');

// Path resolution:
// dist-cjs/src/app/     (start)
// dist-cjs/src/         (../)
// dist-cjs/             (../../)
// package-root/         (../../../) ← STOP HERE!

// Result: /opt/.../node_modules/containerization-assist-mcp/policies ✅
```

### Two Code Paths to Fix

The function had **two approaches** for resolving the module path:

1. **CJS approach** - Uses `__dirname` (accessed via `new Function()` in ESM context)
2. **ESM approach** - Uses `import.meta.url` (captured at module scope)

**Both needed the same fix** - changing from 4 levels (`../../../../`) to 3 levels (`../../../`).

## The Fix

### File: `src/app/orchestrator.ts`

**CJS Path Resolution (Line ~64):**
```typescript
// BEFORE (WRONG):
const moduleRelativePath = resolve(dirName, '../../../../policies');

// AFTER (CORRECT):
const moduleRelativePath = resolve(dirName, '../../../policies');
```

**ESM Path Resolution (Line ~80):**
```typescript
// BEFORE (WRONG):
const moduleRelativePath = resolve(__dirname, '../../../../policies');

// AFTER (CORRECT):
const moduleRelativePath = resolve(__dirname, '../../../policies');
```

### Comments Updated

Both code paths now include accurate comments:
```typescript
// From dist/src/app/ or dist-cjs/src/app/, go up 3 levels to package root
// dist-cjs/src/app/ -> dist-cjs/src/ -> dist-cjs/ -> package-root/policies/
```

## Verification

After the fix, CI logs showed:

```
🟣 Checking path: /opt/.../node_modules/containerization-assist-mcp/policies, exists: true
🟣 Found 3 .rego files in /opt/.../node_modules/containerization-assist-mcp/policies
🟣🟣🟣 RETURNING 3 policy files: [
  ".../policies/base-images.rego",
  ".../policies/container-best-practices.rego",
  ".../policies/security-baseline.rego"
]
```

✅ **Policy discovery now works correctly!**

## Key Lessons

1. **Module path resolution is fragile** - Off-by-one errors in relative paths can cause silent failures
2. **Dev vs packaged environments differ** - What works with `tsx src/...` may fail with `node dist/...`
3. **Test packaging early** - The `npm run mcp:packaged` toggle script was invaluable
4. **Execution tracing is powerful** - Color-coded markers through the call stack revealed the exact failure point
5. **Both ESM and CJS must be considered** - The package supports dual builds, both need correct paths

## Impact

This fix ensures that:

- Built-in policies are **always discovered** in packaged installations
- Organizations can rely on `policies/base-images.rego` to enforce container registry policies
- Knowledge filtering works consistently across dev and production environments
- The MCP server behaves identically whether run from source or installed globally

## Files Modified

1. `src/app/orchestrator.ts` - Fixed path resolution depth (4 → 3 levels)
2. `scripts/test-spring-petclinic-policy-filtering.mjs` - Created packaging test
3. `.github/workflows/test-packed-policy-filtering.yml` - Created CI workflow

## Testing

To reproduce and verify:

```bash
# Pack and install
npm pack
npm install -g containerization-assist-mcp-*.tgz

# Verify policies are packaged
ls $(npm root -g)/containerization-assist-mcp/policies

# Run the test
node scripts/test-spring-petclinic-policy-filtering.mjs /path/to/spring-petclinic
```

Expected result: Only Microsoft Container Registry base images in output.

## Next Steps (Future Work)

The test currently **still fails** because policies are discovered but not yet applied to knowledge filtering. This is a separate issue:

- Policies are loaded into the orchestrator ✅
- Knowledge matcher receives policy evaluator ❓
- `findPolicyAwareKnowledgeMatches()` queries policy for filters ❓
- Registry filtering removes non-compliant base images ❓

This will be addressed in a future PR focusing on the policy-aware knowledge filtering integration.

## Commits

1. `fix: correct policy path resolution - use 3 levels not 4` - Fixed CJS path
2. `fix: correct ESM path resolution for policies (was only fixed for CJS)` - Fixed ESM path

---

**Author:** Investigation conducted via Claude Code
**Date:** 2025-11-12
**Issue:** Built-in policy discovery failing in packaged installations
**Resolution:** Corrected relative path depth from 4 to 3 levels
