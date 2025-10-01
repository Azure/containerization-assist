# Changelog: Transport Metadata & Persistence Hardening

**Release Date**: TBD
**PR**: [Link TBD]

## Overview

This release delivers three critical improvements to the MCP server:

1. **Transport Metadata Propagation** - Restores host-driven controls (cancellation, AI limits)
2. **Docker Build Log Capture** - Fixes missing build output visibility
3. **Session Persistence Hardening** - Prevents silent data loss on storage failures

All changes maintain backward compatibility while enhancing reliability and host integration capabilities.

---

## 1. Transport Metadata Propagation

### What Changed

The MCP server now properly extracts and propagates metadata from the transport layer (`RequestHandlerExtra`) to tool execution contexts:

- **AbortSignal** - Enables host-driven cancellation of long-running operations
- **SessionId** - Prefers transport-level session IDs over parameter-embedded values
- **AI Generation Limits** - Propagates `maxTokens` and `stopSequences` to constrain AI tool execution

### Technical Details

**Modified Files:**
- `src/mcp/mcp-server.ts` - Enhanced `prepareExecutionPayload()` to extract transport metadata
- `src/lib/zod-utils.ts` - Updated to preserve `sessionId` in sanitized params when available
- `src/tools/*/tool.ts` - Tools now receive `signal`, `maxTokens`, `stopSequences` via context

**Key Changes:**
```typescript
// Before: Only extracted sessionId from _meta params
const sessionId = extractSessionId(meta);

// After: Prefers transport-level sessionId
const sessionId = extra.sessionId || extractSessionId(meta);

// New: Propagates abort signal and AI limits
metadata: {
  signal: extra.signal,              // For cancellation
  maxTokens: extractMaxTokens(meta), // AI token ceiling
  stopSequences: extractStopSequences(meta), // AI stop tokens
  // ...
}
```

### Impact on Host Integrators

**Cancellation Support:**
Hosts can now cancel long-running operations by aborting the signal:
```typescript
const abortController = new AbortController();
const result = await client.callTool('build-image', params, {
  signal: abortController.signal
});

// Cancel operation
abortController.abort();
```

**AI Constraints:**
Hosts can limit AI token usage per request:
```typescript
await client.callTool('generate-dockerfile', params, {
  _meta: {
    maxTokens: 4096,
    stopSequences: ['###', 'STOP']
  }
});
```

### Testing

**New Tests:**
- `test/integration/metadata-propagation.test.ts` - Full coverage of signal, token limits, and sessionId propagation
- `test/unit/mcp/mcp-server.test.ts` - Unit tests for metadata extraction functions

**Test Coverage:**
- ✅ Cancellation via pre-aborted and runtime-aborted signals
- ✅ Token ceiling propagation to tool contexts
- ✅ Stop sequences propagation
- ✅ SessionId priority (transport > params)
- ✅ Graceful handling of missing/invalid metadata

---

## 2. Docker Build Log Capture

### What Changed

Docker build operations now capture and return complete build logs, including both stream output and auxiliary metadata.

### Technical Details

**Modified Files:**
- `src/infra/docker/client.ts` - Enhanced `buildImage()` to capture `stream` and `aux` events

**Key Changes:**
```typescript
// Before: Build logs were empty
buildLogs: []

// After: Captures all output
stream.on('data', (event) => {
  if (event.stream) {
    buildLogs.push(event.stream);  // User-facing output
  }
  if (event.aux) {
    buildLogs.push(`[aux] ${JSON.stringify(event.aux)}`);  // Metadata
  }
});
```

### Impact on Users

**Before:**
```json
{
  "imageId": "sha256:abc123...",
  "buildLogs": []  // ❌ Empty
}
```

**After:**
```json
{
  "imageId": "sha256:abc123...",
  "buildLogs": [
    "Step 1/5 : FROM node:20-alpine",
    " ---> abc123def456",
    "Step 2/5 : WORKDIR /app",
    " ---> Running in xyz789...",
    "[aux] {\"ID\":\"sha256:abc123...\"}"
  ]
}
```

### Testing

**New Tests:**
- `test/unit/infrastructure/docker/client.test.ts` - Verifies log capture from stream and aux events
- `test/unit/tools/build-image.test.ts` - Ensures build-image tool returns non-empty logs

**Test Coverage:**
- ✅ Stream output captured
- ✅ Auxiliary metadata captured
- ✅ Error messages captured
- ✅ Logs propagated to tool results

---

## 3. Session Persistence Hardening

### What Changed

Session storage failures are now properly detected and propagated as tool failures, preventing silent data loss.

### Technical Details

**Modified Files:**
- `src/lib/tool-helpers.ts` - `storeToolResults()` now checks session update results and returns failures

**Key Changes:**
```typescript
// Before: Ignored update failures
await ctx.sessionManager.update(sessionId, payload);
return { ok: true, value: undefined };  // Always succeeded

// After: Detects and propagates failures
const updateResult = await ctx.sessionManager.update(sessionId, payload);
if (!updateResult.ok) {
  const error = `Session update failed: ${updateResult.error}`;
  logger.error({ sessionId, toolName, error }, error);
  return { ok: false, error };  // Explicit failure
}
```

**Additional Improvements:**
- Changed log level from `warn` to `error` for visibility
- Enhanced error messages with context (sessionId, toolName)
- Fixed metadata structure to properly nest results under `metadata.results`

### Impact on Operators

**Before:**
- Session storage failures were silently ignored
- Tool reported success even when data wasn't persisted
- Workflow state could be incomplete without warning

**After:**
- Storage failures immediately fail the tool execution
- Operators receive clear error messages
- Workflow integrity is guaranteed or explicitly failed

**Example Error:**
```json
{
  "ok": false,
  "error": "Session update failed: Write conflict: session was modified by another process"
}
```

### Testing

**New Tests:**
- `test/unit/lib/tool-helpers.test.ts` - Comprehensive unit tests for success/failure modes
- `test/integration/session-persistence-failure.test.ts` - End-to-end workflow failure propagation

**Test Coverage:**
- ✅ Session not found
- ✅ Session lookup failure
- ✅ Session update failure (Result pattern)
- ✅ Session update exception
- ✅ Result merging without data loss
- ✅ Metadata preservation
- ✅ Graceful handling of missing context

---

## Breaking Changes

**None.** All changes are backward compatible:

- Metadata fields are optional and gracefully handled when missing
- Existing hosts without cancellation support continue to work
- Session persistence behavior only changes from "silent failure" to "explicit failure"

---

## Migration Guide

### For Host Implementers

**Cancellation Support (Optional):**
```typescript
// Add signal support to enable cancellation
import { AbortController } from 'node:abort-controller';

const controller = new AbortController();
const result = await mcpClient.callTool('build-image', params, {
  signal: controller.signal
});

// Cancel if needed
controller.abort();
```

**AI Token Limits (Optional):**
```typescript
// Constrain AI generation in tools
await mcpClient.callTool('generate-dockerfile', params, {
  _meta: {
    maxTokens: 4096,
    stopSequences: ['###']
  }
});
```

### For Tool Implementers

**Respecting Cancellation:**
```typescript
async function run(input: ToolInput, ctx: ToolContext) {
  // Check signal periodically during long operations
  if (ctx.signal?.aborted) {
    return Failure('Operation cancelled');
  }

  // Listen for abort events
  ctx.signal?.addEventListener('abort', () => {
    // Clean up resources
  });

  // Your tool logic...
}
```

**Build Log Visibility:**
No changes required. Tools using `build-image` automatically receive populated logs.

**Session Persistence:**
No changes required. Tools using `storeToolResults()` automatically get failure detection.

---

## Validation Steps

### Manual Testing

1. **Cancellation:**
   ```bash
   # Start a long-running build and cancel it
   # Verify tool returns cancellation error
   ```

2. **Build Logs:**
   ```bash
   npm run mcp:inspect
   # Call build-image tool
   # Verify non-empty buildLogs in response
   ```

3. **Session Failures:**
   ```bash
   # Simulate disk full / connection loss
   # Verify tool fails with clear error message
   ```

### Automated Testing

```bash
# Run full test suite
npm run validate

# Run specific test suites
NODE_OPTIONS='--experimental-vm-modules' npx jest test/integration/metadata-propagation.test.ts --no-coverage
NODE_OPTIONS='--experimental-vm-modules' npx jest test/integration/session-persistence-failure.test.ts --no-coverage
NODE_OPTIONS='--experimental-vm-modules' npx jest test/unit/lib/tool-helpers.test.ts --no-coverage

# Run smoke journey (requires Docker)
npm run smoke:journey
```

---

## File Inventory

### Modified Files (12)
```
src/infra/docker/client.ts                     (10 lines)
src/lib/tool-helpers.ts                        (31 lines)
src/lib/zod-utils.ts                           (4 lines)
src/mcp/mcp-server.ts                          (32 lines)
src/tools/analyze-repo/tool.ts                 (7 lines)
src/tools/build-image/tool.ts                  (11 lines)
src/tools/deploy/tool.ts                       (8 lines)
src/tools/generate-k8s-manifests/tool.ts       (7 lines)
test/unit/infrastructure/docker/client.test.ts (36 lines)
test/unit/mcp/mcp-server.test.ts               (243 lines)
test/unit/tools/build-image.test.ts            (54 lines)
test/unit/tools/deploy.test.ts                 (1 line)
```

### New Files (3)
```
test/integration/metadata-propagation.test.ts
test/integration/session-persistence-failure.test.ts
test/unit/lib/tool-helpers.test.ts
```

### Total Diff
```
12 files changed, 409 insertions(+), 35 deletions
```

---

## References

- **Sprint Plan:** `plans/sprint-plan-metadata-delivery.md`
- **Architecture:** `docs/developer-guide.md`
- **Tool Capabilities:** `docs/tool-capabilities.md`
- **MCP Protocol:** https://modelcontextprotocol.io/specification

---

## Credits

Implemented as part of Sprint: Transport Metadata & Persistence Hardening
- Workstream 1: Transport Metadata Propagation (T1.1-T1.4)
- Workstream 2: Docker Build Log Capture (T2.1-T2.3)
- Workstream 3: Session Persistence Hardening (T3.1-T3.3)
- Workstream 4: PR Hygiene & Wrap-Up (T4.1-T4.3)
