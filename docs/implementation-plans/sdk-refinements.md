# SDK Refinements Implementation Plan

**Created:** 2025-12-07
**Based on:** Code review of commit 4cb98362
**Status:** Complete (all phases implemented)

## Overview

This plan addresses refinements identified in the SDK decoupling refactor. The architecture is sound; these are improvements to TypeScript idioms, type safety, and testability.

---

## Phase 1: Type Safety Improvements (High Priority)

### 1.1 Derive Input Types from Zod Schemas

**Problem:** SDK functions manually duplicate input types that already exist in Zod schemas.

**Files to modify:**
- `src/sdk/index.ts`

**Implementation:**

```typescript
// Before (duplicated types):
export async function buildImage(
  input: {
    path?: string;
    dockerfile?: string;
    // ... manually duplicated
  },
  options?: SDKOptions,
) { ... }

// After (derived from schema):
import type { z } from 'zod';
import { buildImageSchema } from '@/tools/build-image/schema';

type BuildImageInput = z.input<typeof buildImageSchema>;

export async function buildImage(
  input: BuildImageInput,
  options?: SDKOptions,
): Promise<Result<BuildImageResult>> {
  return executeTool(buildImageTool, input, options);
}
```

**Changes for each function:**

| Function | Schema Import | Input Type |
|----------|---------------|------------|
| `analyzeRepo` | `analyzeRepoSchema` | `z.input<typeof analyzeRepoSchema>` |
| `generateDockerfile` | `generateDockerfileSchema` | `z.input<typeof generateDockerfileSchema>` |
| `fixDockerfile` | `fixDockerfileSchema` | `z.input<typeof fixDockerfileSchema>` |
| `buildImage` | `buildImageSchema` | `z.input<typeof buildImageSchema>` |
| `scanImage` | `scanImageSchema` | `z.input<typeof scanImageSchema>` |
| `tagImage` | `tagImageSchema` | `z.input<typeof tagImageSchema>` |
| `pushImage` | `pushImageSchema` | `z.input<typeof pushImageSchema>` |
| `generateK8sManifests` | `generateK8sManifestsSchema` | `z.input<typeof generateK8sManifestsSchema>` |
| `prepareCluster` | `prepareClusterSchema` | `z.input<typeof prepareClusterSchema>` |
| `verifyDeploy` | `verifyDeploySchema` | `z.input<typeof verifyDeploySchema>` |
| `ops` | `opsSchema` | `z.input<typeof opsSchema>` |

**Validation:**
- Run `npm run typecheck` to ensure types are compatible
- Verify IDE autocomplete works correctly for consumers

---

### 1.2 Add Explicit Return Types

**Problem:** Return types are implicitly inferred, reducing documentation value.

**Files to modify:**
- `src/sdk/index.ts`

**Implementation:**

Add explicit `Promise<Result<T>>` return types to all exported functions:

```typescript
import type { Result } from '@/types/core';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';

export async function analyzeRepo(
  input: AnalyzeRepoInput,
  options?: SDKOptions,
): Promise<Result<RepositoryAnalysis>> {
  return executeTool(analyzeRepoTool, input, options);
}
```

**Return type mapping:**

| Function | Return Type |
|----------|-------------|
| `analyzeRepo` | `Promise<Result<RepositoryAnalysis>>` |
| `generateDockerfile` | `Promise<Result<DockerfilePlan>>` |
| `fixDockerfile` | `Promise<Result<DockerfileFixPlan>>` |
| `buildImage` | `Promise<Result<BuildImageResult>>` |
| `scanImage` | `Promise<Result<ScanImageResult>>` |
| `tagImage` | `Promise<Result<TagImageResult>>` |
| `pushImage` | `Promise<Result<PushImageResult>>` |
| `generateK8sManifests` | `Promise<Result<ManifestPlan>>` |
| `prepareCluster` | `Promise<Result<PrepareClusterResult>>` |
| `verifyDeploy` | `Promise<Result<VerifyDeploymentResult>>` |
| `ops` | `Promise<Result<OpsResult>>` |

---

### 1.3 Improve ZodError Handling

**Problem:** Structured Zod validation errors are discarded, losing debugging information.

**Files to modify:**
- `src/sdk/executor.ts`

**Implementation:**

```typescript
import { ZodError } from 'zod';

// In executeTool function, replace the catch block:

try {
  parsedInput = tool.parse(input);
} catch (error) {
  if (error instanceof ZodError) {
    const issues = error.issues
      .map((issue) => {
        const path = issue.path.length > 0 ? `${issue.path.join('.')}: ` : '';
        return `${path}${issue.message}`;
      })
      .join('; ');

    return Failure(`Validation failed: ${issues}`, {
      message: 'Input validation failed',
      hint: `Validation issues: ${issues}`,
      resolution: 'Review the tool schema for expected input format',
    });
  }

  const message = error instanceof Error ? error.message : 'Invalid input';
  return Failure(`Validation failed: ${message}`, {
    message: 'Input validation failed',
    hint: 'Check that all required parameters are provided with correct types',
    resolution: 'Review the tool documentation for expected input format',
  });
}
```

**Add test case:**

```typescript
// In test/unit/sdk/executor.test.ts

test('should include field path in validation error for nested fields', async () => {
  const nestedSchema = z.object({
    config: z.object({
      name: z.string(),
      port: z.number(),
    }),
  });

  const mockTool = createMockToolWithSchema(nestedSchema, async () => Success({}));

  const result = await executeTool(mockTool, {
    config: { name: 'test', port: 'not-a-number' }
  });

  expect(result.ok).toBe(false);
  if (!result.ok) {
    expect(result.error).toContain('config.port');
  }
});
```

---

## Phase 2: Testability Improvements (High Priority)

### 2.1 Replace Module-Level Mutable State

**Problem:** Module-level `let knowledgeLoaded = false` is difficult to test and reset.

**Files to modify:**
- `src/sdk/executor.ts`

**Implementation Option A: Injectable Factory (Recommended)**

```typescript
// Create a knowledge loader factory
interface KnowledgeLoader {
  ensureLoaded(logger: Logger): void;
  reset(): void;  // For testing
}

function createKnowledgeLoader(): KnowledgeLoader {
  let loaded = false;

  return {
    ensureLoaded(logger: Logger): void {
      if (loaded) return;

      try {
        loadKnowledgeBase();
        loaded = true;
        logger.debug('Knowledge base loaded for SDK');
      } catch (error) {
        logger.warn({ error }, 'Failed to load knowledge base');
      }
    },
    reset(): void {
      loaded = false;
    },
  };
}

// Default instance for production use
const defaultKnowledgeLoader = createKnowledgeLoader();

// Export for testing
export const _testing = {
  resetKnowledgeLoader: () => defaultKnowledgeLoader.reset(),
};

// Update executeTool to use the loader
export async function executeTool<TSchema extends ZodTypeAny, TOutput>(
  tool: Tool<TSchema, TOutput>,
  input: unknown,
  options: SDKOptions = {},
): Promise<Result<TOutput>> {
  const logger = options.logger ?? createSDKLogger();

  // Use the singleton loader
  defaultKnowledgeLoader.ensureLoaded(logger);

  // ... rest of implementation
}
```

**Implementation Option B: Lazy Singleton with Reset**

```typescript
// Simpler approach if injection isn't needed
const knowledgeState = {
  loaded: false,

  ensureLoaded(logger: Logger): void {
    if (this.loaded) return;

    try {
      loadKnowledgeBase();
      this.loaded = true;
      logger.debug('Knowledge base loaded for SDK');
    } catch (error) {
      logger.warn({ error }, 'Failed to load knowledge base');
    }
  },

  // For testing only
  reset(): void {
    this.loaded = false;
  },
};

// Export for testing
export const _testing = {
  resetKnowledgeState: () => knowledgeState.reset(),
};
```

**Update tests:**

```typescript
// In test/unit/sdk/executor.test.ts

import { _testing } from '../../../src/sdk/executor';

beforeEach(() => {
  jest.clearAllMocks();
  _testing.resetKnowledgeLoader(); // or resetKnowledgeState
});
```

---

### 2.2 Fix Test Type Assertions

**Problem:** Tests use `as never` type assertions that hide type mismatches.

**Files to modify:**
- `test/unit/sdk/executor.test.ts`

**Implementation:**

```typescript
import type { ToolContext } from '@/core/context';
import type { Result } from '@/types/core';

// Generic helper that properly types the mock
function createMockTool<TOut>(
  handler: (input: z.infer<typeof mockSchema>, ctx: ToolContext) => Promise<Result<TOut>>,
) {
  return tool({
    name: 'analyze-repo' as const,
    description: 'Mock tool for testing',
    schema: mockSchema,
    metadata: { knowledgeEnhanced: false },
    handler,
  });
}

// Usage in tests - now properly typed
test('should execute tool handler with valid input', async () => {
  const mockHandler = jest.fn<
    (input: { input: string; optional?: number }, ctx: ToolContext) => Promise<Result<{ result: string }>>
  >().mockResolvedValue(Success({ result: 'success' }));

  const mockTool = createMockTool(mockHandler);
  // ...
});
```

**Alternative: Create test utilities module:**

```typescript
// test/__support__/utilities/mock-tools.ts

import { z } from 'zod';
import type { ToolContext } from '@/core/context';
import type { Result } from '@/types/core';
import { tool } from '@/types/tool';

export function createTestTool<TSchema extends z.ZodTypeAny, TOut>(
  schema: TSchema,
  handler: (input: z.infer<TSchema>, ctx: ToolContext) => Promise<Result<TOut>>,
  options?: { name?: string; description?: string },
) {
  return tool({
    name: (options?.name ?? 'test-tool') as 'analyze-repo', // Cast needed for ToolName union
    description: options?.description ?? 'Test tool',
    schema,
    metadata: { knowledgeEnhanced: false },
    handler,
  });
}
```

---

### 2.3 Add Missing Test Scenarios

**Problem:** Missing tests for error edge cases.

**Files to modify:**
- `test/unit/sdk/executor.test.ts`

**Add these test cases:**

```typescript
describe('Error Handling', () => {
  test('should propagate unhandled exceptions from tool handler', async () => {
    const mockHandler = jest.fn().mockRejectedValue(new Error('Unexpected error'));
    const mockTool = createMockTool(mockHandler);

    await expect(executeTool(mockTool, { input: 'test' }))
      .rejects.toThrow('Unexpected error');
  });

  test('should handle non-Error thrown values', async () => {
    const mockHandler = jest.fn().mockRejectedValue('string error');
    const mockTool = createMockTool(mockHandler);

    await expect(executeTool(mockTool, { input: 'test' }))
      .rejects.toBe('string error');
  });
});

describe('Cancellation', () => {
  test('should pass aborted signal to tool', async () => {
    const controller = new AbortController();
    controller.abort();

    let receivedSignal: AbortSignal | undefined;
    const mockHandler = jest.fn().mockImplementation(async (_input, ctx) => {
      receivedSignal = ctx.signal;
      return Success({ result: 'success' });
    });
    const mockTool = createMockTool(mockHandler);

    await executeTool(mockTool, { input: 'test' }, { signal: controller.signal });

    expect(receivedSignal?.aborted).toBe(true);
  });
});

describe('Progress Callback', () => {
  test('should handle progress callback that throws', async () => {
    const onProgress = jest.fn().mockImplementation(() => {
      throw new Error('Progress callback error');
    });

    const mockHandler = jest.fn().mockImplementation(async (_input, ctx) => {
      // This should throw when progress callback fails
      await ctx.progress?.('Step 1', 1, 3);
      return Success({ result: 'success' });
    });
    const mockTool = createMockTool(mockHandler);

    await expect(executeTool(mockTool, { input: 'test' }, { onProgress }))
      .rejects.toThrow('Progress callback error');
  });
});
```

---

## Phase 3: Code Quality Improvements (Medium Priority)

### 3.1 Standardize Import Extensions

**Problem:** Inconsistent use of `.ts` vs `.js` extensions in imports.

**Decision:** Use `.ts` extensions consistently (as per user preference).

**Files to modify:**
- `src/sdk/index.ts`
- `src/sdk/types.ts`
- `src/mcp/context.ts`

**Changes:**

```typescript
// src/sdk/index.ts
// Before:
import { executeTool, type SDKOptions } from './executor.js';
export { executeTool } from './executor.js';

// After:
import { executeTool, type SDKOptions } from './executor.ts';
export { executeTool } from './executor.ts';
```

**Note:** This requires ensuring the build system (`tsc-alias`) handles `.ts` → `.js` conversion properly. Verify with:
```bash
npm run build
grep -r "from './executor" dist/src/sdk/
# Should show .js extensions in built output
```

**Alternative (if build issues arise):** Remove extensions entirely and let TypeScript resolve:
```typescript
import { executeTool, type SDKOptions } from './executor';
```

---

### 3.2 Simplify Conditional Spread Pattern

**Problem:** Verbose conditional spreads with redundant checks.

**Files to modify:**
- `src/sdk/executor.ts`
- `src/mcp/context.ts`

**Implementation:**

```typescript
// src/sdk/executor.ts - Before:
const ctx = createToolContext(logger, {
  ...(options.signal !== undefined && { signal: options.signal }),
  ...(options.onProgress !== undefined && {
    progress: async (msg: string, prog?: number, total?: number) => {
      options.onProgress!(msg, prog, total);
    },
  }),
});

// After:
const ctx = createToolContext(logger, {
  signal: options.signal,
  progress: options.onProgress
    ? async (msg: string, prog?: number, total?: number) => {
        options.onProgress(msg, prog, total);
      }
    : undefined,
});
```

**Or extract helper:**

```typescript
function wrapProgressCallback(
  onProgress: SDKOptions['onProgress'],
): ProgressReporter | undefined {
  if (!onProgress) return undefined;

  return async (message, progress, total) => {
    onProgress(message, progress, total);
  };
}

// Usage:
const ctx = createToolContext(logger, {
  signal: options.signal,
  progress: wrapProgressCallback(options.onProgress),
});
```

---

### 3.3 Clean Up Re-export Aliases

**Problem:** Confusing re-export patterns with inconsistent aliases.

**Files to modify:**
- `src/mcp/context.ts`

**Implementation:**

```typescript
// src/mcp/context.ts - Reorganized

/**
 * MCP Context - Tool execution environment with MCP protocol support
 */

import type { Logger } from 'pino';
import type { RegoEvaluator } from '@/config/policy-rego';

// ===== CORE RE-EXPORTS =====
// Canonical types - import from here for backward compatibility
// New code should import directly from '@/core/context'

export type { ToolContext, ProgressReporter } from '@/core/context';
export type { ContextOptions as CoreContextOptions } from '@/core/context';
export { createToolContext as createCoreToolContext } from '@/core/context';

// ===== INTERNAL IMPORTS =====

import {
  createToolContext as createCoreContext,
  type ToolContext,
  type ProgressReporter,
} from '@/core/context';
import { extractProgressReporter } from './context-helpers.ts';

// ===== MCP-SPECIFIC TYPES =====

export interface MCPContextOptions {
  signal?: AbortSignal;
  progress?: ProgressReporter | unknown;
  sendNotification?: (notification: unknown) => Promise<void>;
  policy?: RegoEvaluator;
}

// ===== MCP CONTEXT FACTORY =====

export function createToolContext(
  logger: Logger,
  options: MCPContextOptions = {},
): ToolContext {
  const { sendNotification, progress, signal, policy } = options;

  const progressReporter = extractProgressReporter(progress, logger, sendNotification);

  return createCoreContext(logger, {
    signal,
    policy,
    progress: progressReporter,
  });
}

// ===== MCP-SPECIFIC EXPORTS =====

export type { EnhancedProgressReporter } from './context-helpers.ts';
export { extractProgressToken, createProgressReporter } from './context-helpers.ts';
```

---

## Phase 4: Documentation Improvements (Low Priority)

### 4.1 Add JSDoc to Tools Object Properties

**Files to modify:**
- `src/sdk/index.ts`

**Implementation:**

```typescript
/**
 * Direct access to all 11 tool objects for advanced use cases.
 */
export const tools = {
  // ===== Analysis =====
  /** Analyze repository structure, detect languages, frameworks, and dependencies */
  analyzeRepo: analyzeRepoTool,

  // ===== Dockerfile =====
  /** Generate optimized Dockerfile with security best practices */
  generateDockerfile: generateDockerfileTool,
  /** Fix and optimize existing Dockerfile issues */
  fixDockerfile: fixDockerfileTool,

  // ===== Image Operations =====
  /** Build Docker image from Dockerfile (requires Docker daemon) */
  buildImage: buildImageTool,
  /** Scan image for security vulnerabilities (requires Trivy) */
  scanImage: scanImageTool,
  /** Tag Docker image with additional tags */
  tagImage: tagImageTool,
  /** Push image to container registry */
  pushImage: pushImageTool,

  // ===== Kubernetes =====
  /** Generate Kubernetes deployment manifests */
  generateK8sManifests: generateK8sManifestsTool,
  /** Prepare Kubernetes cluster namespace and prerequisites */
  prepareCluster: prepareClusterTool,
  /** Verify Kubernetes deployment status and health */
  verifyDeploy: verifyDeployTool,

  // ===== Operations =====
  /** Operational utilities (ping, status checks) */
  ops: opsTool,
} as const;
```

---

## Implementation Order

```
Phase 1: Type Safety (High Priority)
├── 1.1 Derive input types from Zod schemas
├── 1.2 Add explicit return types
└── 1.3 Improve ZodError handling

Phase 2: Testability (High Priority)
├── 2.1 Replace module-level mutable state
├── 2.2 Fix test type assertions
└── 2.3 Add missing test scenarios

Phase 3: Code Quality (Medium Priority)
├── 3.1 Standardize import extensions (.ts)
├── 3.2 Simplify conditional spread pattern
└── 3.3 Clean up re-export aliases

Phase 4: Documentation (Low Priority)
└── 4.1 Add JSDoc to tools object properties
```

---

## Validation Checklist

After implementation, verify:

- [x] `npm run typecheck` passes
- [x] `npm run lint` passes
- [x] `npm test` passes (all existing + new tests)
- [x] `npm run build` succeeds
- [x] Built output has correct `.js` extensions
- [x] SDK can be imported and used in isolation
- [x] IDE autocomplete works for SDK functions
- [x] No MCP imports in `src/sdk/` or `src/core/`

---

## Estimated Scope

| Phase | Files Modified | New Tests | Complexity |
|-------|----------------|-----------|------------|
| 1.1-1.2 | 1 | 0 | Low |
| 1.3 | 1 | 2 | Low |
| 2.1 | 1 | 1 | Medium |
| 2.2-2.3 | 1 | 5 | Low |
| 3.1-3.3 | 3 | 0 | Low |
| 4.1 | 1 | 0 | Low |

**Total:** ~6 files modified, ~8 new test cases
