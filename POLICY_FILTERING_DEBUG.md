# Policy Filtering Debug Guide

## Problem Statement

Built-in policies (specifically `policies/base-images.rego`) are **not being applied** in the packed/installed version of the application, even though they work correctly in development mode.

**Expected behavior:** When `base-images.rego` is loaded, only Microsoft Container Registry images should be recommended.

**Actual behavior:** Non-compliant images (eclipse-temurin, alpine, etc.) are still appearing in recommendations.

## Changes Made

### 1. Fixed `import.meta.url` Access (src/app/orchestrator.ts)

**Problem:** `import.meta.url` was being accessed incorrectly using `new Function()`, which doesn't have access to module-scope bindings.

**Fix:** Capture `import.meta.url` at module scope:
```typescript
// Line 26
const MODULE_URL = typeof import.meta !== 'undefined' && import.meta.url ? import.meta.url : undefined;

// Line 68-79
if (!modulePathResolved && MODULE_URL) {
  const __filename = fileURLToPath(MODULE_URL);
  const __dirname = dirname(__filename);
  const moduleRelativePath = resolve(__dirname, '../../../policies');
  searchPaths.push(moduleRelativePath);
}
```

### 2. Enhanced Logging

Added comprehensive logging to track:
- Policy discovery attempts (CJS vs ESM)
- Module path resolution
- Policy paths discovered
- Policy loading success/failure
- Whether policy is passed to tool context

**Log locations:**
- `discoverBuiltInPolicies()` - Lines 61, 74, 82
- `discoverPolicies()` - Lines 308-314, 320-327
- `createContextForTool()` - Lines 253-259

### 3. Created Test Infrastructure

**Test script:** `scripts/test-spring-petclinic-policy-filtering.mjs`
- Tests the **actual packed/installed version** (via `npm pack` + `npm install -g`)
- Uses `ca-mcp` command (not direct node execution)
- Validates built-in policy auto-discovery
- **FAILS** if non-Microsoft images appear in output

**CI workflow:** `.github/workflows/test-packed-policy-filtering.yml`
- Packs and installs the application globally
- Verifies `policies/` directory exists in installed package
- Runs the test script
- Reports detailed results

## What to Look for in CI Logs

When the test runs in CI, look for these log entries (they go to stderr):

### 1. Module Path Resolution
```json
{"level":30,"msg":"Resolved module path for policy discovery","method":"ESM import.meta.url","moduleRelativePath":"/usr/local/lib/node_modules/containerization-assist-mcp/policies","MODULE_URL":"file:///usr/local/lib/node_modules/containerization-assist-mcp/dist/src/app/orchestrator.js"}
```

**What to check:**
- Is `METHOD` "ESM import.meta.url" or "CJS __dirname"?
- Is `moduleRelativePath` pointing to the correct `policies/` directory?
- Is `MODULE_URL` defined?

### 2. Policy Discovery
```json
{"level":30,"msg":"Discovered policies, loading...","policyPaths":["/usr/local/lib/node_modules/containerization-assist-mcp/policies/base-images.rego",...]}
```

**What to check:**
- How many policies were discovered?
- Does the list include `base-images.rego`?
- Are the paths absolute and pointing to the installed package?

### 3. Policy Loading Success
```json
{"level":30,"msg":"Policies loaded successfully for orchestrator","total":3,"policyPaths":[...]}
```

**What to check:**
- Did policy loading succeed?
- How many policies were loaded?

### 4. Tool Context Creation
```json
{"level":20,"msg":"Creating tool context","hasPolicy":true,"toolName":"generate-dockerfile"}
```

**What to check:**
- Is `hasPolicy` true or false?
- If false, the policy didn't load or isn't being passed to tools

## Possible Issues & Solutions

### Issue 1: MODULE_URL is undefined
**Symptom:** Log shows `"Could not resolve module path for built-in policies"`

**Cause:** `import.meta.url` isn't available in the module scope

**Solution:** Check that the package is using ESM (`"type": "module"` in package.json)

### Issue 2: Policies directory not found
**Symptom:** Log shows `"Built-in policies directory not found in any search path"`

**Cause:** The `policies/` directory isn't in the packaged tarball

**Solution:** Verify `package.json` `files` field includes `"policies/**/*.rego"`

### Issue 3: Policy loads but isn't applied
**Symptom:**
- Logs show "Policies loaded successfully"
- But `hasPolicy: false` in tool context

**Cause:** Policy isn't being passed from orchestrator to tools

**Solution:** Check `executeWithOrchestration()` in orchestrator.ts

### Issue 4: Policy loads but knowledge isn't filtered
**Symptom:**
- Logs show "Policies loaded successfully"
- `hasPolicy: true` in tool context
- But non-compliant images still appear

**Cause:** Knowledge pack matcher isn't using the policy

**Solution:** Check `getPolicyAwareKnowledgeSnippets()` in knowledge-tool-pattern.ts

## Testing Locally

To test the packed version locally:

```bash
# Build and pack
npm run build
npm pack

# Install globally
npm install -g ./containerization-assist-mcp-*.tgz

# Clone test repo
git clone --depth 1 https://github.com/spring-projects/spring-petclinic.git
cd spring-petclinic

# Run test
node ../scripts/test-spring-petclinic-policy-filtering.mjs $(pwd)

# Cleanup
npm uninstall -g containerization-assist-mcp
cd ..
rm -rf spring-petclinic
```

## Expected Test Result

**When fixed, the test should PASS with:**
```
✅ Found 2 Microsoft Container Registry image(s) in output.
  - mcr.microsoft.com/openjdk/jdk:25-azurelinux
  - mcr.microsoft.com/openjdk/jdk:25-distroless
✅ No non-Microsoft images found (built-in policy correctly filtered them out).
✅ azurelinux images found in output
✅ distroless images found in output

=== ALL BUILT-IN POLICY TESTS PASSED ===
```

**Currently failing with:**
```
❌ ERROR: Found non-Microsoft images in output!
Built-in base-images.rego policy should have filtered these out:
  - eclipse-temurin
  - alpine
```

## Next Steps

1. **Run CI** and examine the logs for the entries described above
2. **Identify which step is failing** (discovery, loading, context passing, or filtering)
3. **Apply the appropriate solution** based on the failure mode
4. **Repeat** until the test passes
