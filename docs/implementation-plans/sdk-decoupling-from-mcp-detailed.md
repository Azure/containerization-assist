# SDK Decoupling from MCP - Detailed Implementation Plan

## Overview

This document provides step-by-step implementation details for decoupling core tool functionality from MCP dependencies, enabling a standalone SDK for VS Code Copilot integration.

**Target Consumer**: VS Code extension developers who want to call tools as simple functions without MCP dependencies.

**Scope**: All 11 tools will be exposed via SDK for architectural consistency:
- `analyze-repo` - Repository analysis
- `generate-dockerfile` - Dockerfile planning
- `fix-dockerfile` - Dockerfile fixes
- `build-image` - Docker builds
- `scan-image` - Security scanning
- `tag-image` - Image tagging
- `push-image` - Registry push
- `generate-k8s-manifests` - Kubernetes manifests
- `prepare-cluster` - Cluster setup
- `verify-deploy` - Deployment verification
- `ops` - Operational utilities

---

## PR Structure

| PR | Title | Est. Lines | Dependencies |
|----|-------|------------|--------------|
| 1 | Create core context layer | ~200 | None |
| 2 | Refactor MCP context to re-export from core | ~150 | PR 1 |
| 3 | Update tool imports to use core context | ~100 | PR 2 |
| 4 | Create SDK executor and entry point (all 11 tools) | ~600 | PR 3 |
| 5 | Add package exports and documentation | ~250 | PR 4 |
| 6 | Add SDK tests (all 11 tools) | ~400 | PR 5 |

**Total estimated lines**: ~1,700 (well under 2k per PR)

---

## PR 1: Create Core Context Layer

**Branch**: `feature/core-context-layer`

**Goal**: Create `src/core/` directory with `ToolContext` interface and `createToolContext` function that have zero MCP dependencies.

### Step 1.1: Update tsconfig.json

Add the `@/core/*` path alias.

**File**: `tsconfig.json`

**Action**: Add path alias in the `paths` object (alphabetically ordered with existing aliases).

```json
{
  "compilerOptions": {
    "paths": {
      "@/*": ["src/*"],
      "@/config/*": ["src/config/*"],
      "@/core/*": ["src/core/*"],
      "@/infra/*": ["src/infra/*"],
      "@/knowledge/*": ["src/knowledge/*"],
      "@/lib/*": ["src/lib/*"],
      "@/mcp/*": ["src/mcp/*"],
      "@/tools/*": ["src/tools/*"],
      "@/validation/*": ["src/validation/*"],
      "@types": ["src/types"]
    }
  }
}
```

### Step 1.2: Create src/core/context.ts

**File**: `src/core/context.ts` (NEW)

**Full Content**:

```typescript
/**
 * Core Tool Context
 *
 * Provides the foundational ToolContext interface and factory function
 * with zero MCP dependencies. This module is the single source of truth
 * for ToolContext - all other modules should import from here.
 *
 * Design Principle: Core layer has no dependencies on MCP or protocol-specific code.
 */

import type { Logger } from 'pino';
import type { RegoEvaluator } from '@/config/policy-rego';

// ===== TYPES =====

/**
 * Progress reporting function for tool execution feedback.
 *
 * Tools call this to report progress during long-running operations.
 * The implementation may forward these updates to various destinations
 * (console, MCP notifications, UI callbacks, etc.).
 *
 * @param message - Human-readable progress message
 * @param progress - Current progress value (optional)
 * @param total - Total progress value (optional)
 */
export type ProgressReporter = (
  message: string,
  progress?: number,
  total?: number,
) => Promise<void>;

/**
 * Core tool execution context.
 *
 * This interface defines what every tool receives during execution.
 * It provides access to logging, cancellation, progress reporting,
 * and optional policy evaluation.
 *
 * IMPORTANT: This interface has no MCP-specific types or dependencies.
 * MCP integration layers can extend or wrap this context as needed.
 */
export interface ToolContext {
  /**
   * Optional abort signal for cancellation support.
   * Tools should check this signal periodically for long-running operations.
   */
  signal: AbortSignal | undefined;

  /**
   * Optional progress reporting function for user feedback.
   * Should be called at regular intervals during long operations.
   */
  progress: ProgressReporter | undefined;

  /**
   * Logger for debugging and error tracking.
   * Required for all tools - use this for structured logging instead of console.log.
   */
  logger: Logger;

  /**
   * Optional Rego policy evaluator for tool self-validation.
   * Tools can use this to validate generated content against organizational policies.
   */
  policy?: RegoEvaluator;

  /**
   * Query policy for configuration data.
   *
   * Convenience method that wraps policy.queryConfig() with null-safety.
   * Returns null if no policy is configured.
   *
   * @param packageName - OPA package name to query (e.g., 'containerization.generation_config')
   * @param input - Input data for the query
   * @returns Configuration object from policy or null if no policy configured
   */
  queryConfig<T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null>;
}

// ===== CONTEXT OPTIONS =====

/**
 * Options for creating a tool context.
 *
 * These are the core options that don't depend on MCP.
 * MCP-specific options (like sendNotification) are handled
 * in the MCP layer which extends these options.
 */
export interface ContextOptions {
  /** Optional abort signal for cancellation */
  signal?: AbortSignal;

  /** Optional progress reporter function */
  progress?: ProgressReporter;

  /** Optional Rego policy evaluator */
  policy?: RegoEvaluator;
}

// ===== CONTEXT FACTORY =====

/**
 * Create a ToolContext for tool execution.
 *
 * This is the core factory function that creates a minimal ToolContext
 * with no MCP dependencies. For MCP-aware context creation with
 * notification support, use the factory from '@/mcp/context' instead.
 *
 * @param logger - Pino logger instance for debugging and error tracking
 * @param options - Optional configuration for signal, progress, and policy
 * @returns Configured ToolContext ready for tool execution
 *
 * @example
 * ```typescript
 * import { createToolContext } from '@/core/context';
 * import { createLogger } from '@/lib/logger';
 *
 * const logger = createLogger({ name: 'my-tool' });
 * const ctx = createToolContext(logger, {
 *   signal: abortController.signal,
 *   progress: async (msg) => console.log(msg),
 * });
 *
 * const result = await myTool.handler(input, ctx);
 * ```
 */
export function createToolContext(logger: Logger, options: ContextOptions = {}): ToolContext {
  const { signal, progress, policy } = options;

  return {
    logger,
    signal,
    progress,
    ...(policy && { policy }),
    queryConfig: async <T = unknown>(
      packageName: string,
      input: Record<string, unknown>,
    ): Promise<T | null> => {
      if (!policy) {
        logger.debug({ packageName }, 'No policy configured, returning null for config query');
        return null;
      }
      return policy.queryConfig<T>(packageName, input);
    },
  };
}
```

### Step 1.3: Create src/core/index.ts

**File**: `src/core/index.ts` (NEW)

**Full Content**:

```typescript
/**
 * Core Module Exports
 *
 * This module exports the foundational types and utilities that have
 * no MCP dependencies. All tool implementations should import
 * ToolContext from this module (via '@/core/context').
 */

// Context types and factory
export type { ToolContext, ProgressReporter, ContextOptions } from './context.js';
export { createToolContext } from './context.js';
```

### Step 1.4: Verification

After creating these files, verify:

```bash
# Build should succeed
npm run build:esm

# No MCP imports in core
grep -r "@modelcontextprotocol" src/core/
# Should return nothing

grep -r "@/mcp" src/core/
# Should return nothing
```

### PR 1 Checklist

- [ ] Add `@/core/*` path alias to `tsconfig.json`
- [ ] Create `src/core/context.ts` with ToolContext interface and createToolContext
- [ ] Create `src/core/index.ts` with exports
- [ ] Verify build succeeds
- [ ] Verify no MCP imports in `src/core/`

---

## PR 2: Refactor MCP Context to Re-export from Core

**Branch**: `feature/mcp-context-reexport`

**Goal**: Update `src/mcp/context.ts` to re-export from core and add MCP-specific functionality on top.

### Step 2.1: Refactor src/mcp/context.ts

**File**: `src/mcp/context.ts`

**Action**: Replace entire file content with the following:

```typescript
/**
 * MCP Context - Tool execution environment with MCP protocol support
 *
 * This module re-exports the core ToolContext interface and adds
 * MCP-specific functionality for progress notifications via the
 * MCP protocol.
 *
 * Design: MCP layer builds ON TOP of core layer, not the other way around.
 *
 * Invariant: All tools receive consistent context interface
 * Trade-off: Abstraction overhead for tool isolation and testability
 */

import type { Logger } from 'pino';
import type { RegoEvaluator } from '@/config/policy-rego';

// ===== RE-EXPORTS FROM CORE =====
// These are the canonical exports - tools should import ToolContext from '@/core/context'
// but importing from here still works for backward compatibility

export type {
  ToolContext,
  ProgressReporter,
  ContextOptions as CoreContextOptions,
} from '@/core/context';

export { createToolContext as createCoreToolContext } from '@/core/context';

// ===== MCP-SPECIFIC IMPORTS =====

import { createToolContext as coreCreateToolContext } from '@/core/context';
import type { ToolContext, ContextOptions as CoreContextOptions } from '@/core/context';
import { extractProgressReporter } from './context-helpers.js';

// ===== MCP-SPECIFIC TYPES =====

/**
 * Extended context options with MCP notification support.
 *
 * Extends core options with MCP-specific functionality for
 * progress notifications via the MCP protocol.
 */
export interface ContextOptions extends CoreContextOptions {
  /**
   * MCP notification callback for progress updates.
   * When provided, progress updates are sent via MCP protocol.
   */
  sendNotification?: (notification: unknown) => Promise<void>;
}

// ===== MCP CONTEXT FACTORY =====

/**
 * Create a ToolContext with optional MCP progress notification support.
 *
 * This factory extends the core createToolContext with MCP-specific
 * functionality. If sendNotification is provided, progress updates
 * are forwarded through the MCP protocol.
 *
 * @param logger - Pino logger instance
 * @param options - Context options including MCP-specific sendNotification
 * @returns ToolContext configured for MCP or core usage
 *
 * @example
 * ```typescript
 * // MCP usage with notifications
 * const ctx = createToolContext(logger, {
 *   signal: request.signal,
 *   progress: request.params,
 *   sendNotification: server.sendNotification,
 * });
 *
 * // Simple usage (no MCP)
 * const ctx = createToolContext(logger);
 * ```
 */
export function createToolContext(logger: Logger, options: ContextOptions = {}): ToolContext {
  const { sendNotification, progress, ...coreOptions } = options;

  // If MCP notification callback provided, create enhanced progress reporter
  if (sendNotification) {
    const progressReporter = extractProgressReporter(progress, logger, sendNotification);

    return coreCreateToolContext(logger, {
      ...coreOptions,
      progress: progressReporter,
    });
  }

  // If progress is not a function (e.g., MCP request params), extract reporter
  if (progress !== undefined && typeof progress !== 'function') {
    const progressReporter = extractProgressReporter(progress, logger, sendNotification);

    return coreCreateToolContext(logger, {
      ...coreOptions,
      progress: progressReporter,
    });
  }

  // Otherwise, use core implementation directly
  return coreCreateToolContext(logger, {
    ...coreOptions,
    progress: progress as typeof coreOptions.progress,
  });
}

// ===== MCP-SPECIFIC EXPORTS =====

// Progress handling utilities specific to MCP protocol
export type { EnhancedProgressReporter } from './context-helpers.js';
export { extractProgressToken, createProgressReporter } from './context-helpers.js';
```

### Step 2.2: Verification

```bash
# Build should succeed
npm run build:esm

# Run existing tests to ensure no regressions
npm test
```

### PR 2 Checklist

- [ ] Refactor `src/mcp/context.ts` to re-export from `@/core/context`
- [ ] Keep MCP-specific `sendNotification` support
- [ ] Keep backward-compatible `createToolContext` function
- [ ] Verify build succeeds
- [ ] Verify existing tests pass

---

## PR 3: Update Tool Imports to Use Core Context

**Branch**: `feature/tools-use-core-context`

**Goal**: Update all 15 files that import from `@/mcp/context` to import from `@/core/context`.

### Step 3.1: Files to Update

The following files need their import updated from:
```typescript
import type { ToolContext } from '@/mcp/context';
```
to:
```typescript
import type { ToolContext } from '@/core/context';
```

**Files (15 total)**:

1. `src/types/tool.ts`
2. `src/app/orchestrator.ts`
3. `src/cli/policy-simulate.ts`
4. `src/tools/analyze-repo/tool.ts`
5. `src/tools/build-image/tool.ts`
6. `src/tools/fix-dockerfile/tool.ts`
7. `src/tools/generate-dockerfile/tool.ts`
8. `src/tools/generate-k8s-manifests/tool.ts`
9. `src/tools/ops/tool.ts`
10. `src/tools/prepare-cluster/tool.ts`
11. `src/tools/push-image/tool.ts`
12. `src/tools/scan-image/tool.ts`
13. `src/tools/tag-image/tool.ts`
14. `src/tools/verify-deploy/tool.ts`
15. `src/tools/shared/knowledge-tool-pattern.ts`

### Step 3.2: Detailed Changes for Each File

#### 3.2.1: src/types/tool.ts

**Current**:
```typescript
import type { ToolContext } from '@/mcp/context';
```

**Change to**:
```typescript
import type { ToolContext } from '@/core/context';
```

#### 3.2.2: src/app/orchestrator.ts

**Current**:
```typescript
import { createToolContext, type ToolContext } from '@/mcp/context';
```

**Change to**:
```typescript
import { createToolContext, type ToolContext } from '@/mcp/context';
```

**Note**: The orchestrator should KEEP importing from `@/mcp/context` because it needs the MCP-aware `createToolContext` that supports `sendNotification`. Only the `ToolContext` type could be from core, but for simplicity, keep the import as-is since mcp/context re-exports everything.

**Actually, reconsider**: The orchestrator needs MCP features, so it should import from `@/mcp/context`. Let's update the list:

**Revised Files to Update (14 total - excluding orchestrator)**:

1. `src/types/tool.ts` - Change to `@/core/context`
2. `src/cli/policy-simulate.ts` - Check if it needs MCP features
3. `src/tools/analyze-repo/tool.ts` - Change to `@/core/context`
4. `src/tools/build-image/tool.ts` - Change to `@/core/context`
5. `src/tools/fix-dockerfile/tool.ts` - Change to `@/core/context`
6. `src/tools/generate-dockerfile/tool.ts` - Change to `@/core/context`
7. `src/tools/generate-k8s-manifests/tool.ts` - Change to `@/core/context`
8. `src/tools/ops/tool.ts` - Change to `@/core/context`
9. `src/tools/prepare-cluster/tool.ts` - Change to `@/core/context`
10. `src/tools/push-image/tool.ts` - Change to `@/core/context`
11. `src/tools/scan-image/tool.ts` - Change to `@/core/context`
12. `src/tools/tag-image/tool.ts` - Change to `@/core/context`
13. `src/tools/verify-deploy/tool.ts` - Change to `@/core/context`
14. `src/tools/shared/knowledge-tool-pattern.ts` - Change to `@/core/context`

**Files that should KEEP importing from `@/mcp/context`**:
- `src/app/orchestrator.ts` - Needs MCP-aware createToolContext with sendNotification

### Step 3.3: Automated Update Script

Create a script or run manually:

```bash
# Update tool files and types (but NOT orchestrator)
for file in \
  src/types/tool.ts \
  src/cli/policy-simulate.ts \
  src/tools/analyze-repo/tool.ts \
  src/tools/build-image/tool.ts \
  src/tools/fix-dockerfile/tool.ts \
  src/tools/generate-dockerfile/tool.ts \
  src/tools/generate-k8s-manifests/tool.ts \
  src/tools/ops/tool.ts \
  src/tools/prepare-cluster/tool.ts \
  src/tools/push-image/tool.ts \
  src/tools/scan-image/tool.ts \
  src/tools/tag-image/tool.ts \
  src/tools/verify-deploy/tool.ts \
  src/tools/shared/knowledge-tool-pattern.ts
do
  sed -i "s|from '@/mcp/context'|from '@/core/context'|g" "$file"
done
```

### Step 3.4: Verification

```bash
# Build should succeed
npm run build:esm

# Run all tests
npm test

# Verify tools import from core
grep -l "@/core/context" src/tools/*/tool.ts
# Should list all tool files

# Verify orchestrator still uses mcp context
grep "@/mcp/context" src/app/orchestrator.ts
# Should show the import
```

### PR 3 Checklist

- [ ] Update `src/types/tool.ts` import
- [ ] Update `src/cli/policy-simulate.ts` import
- [ ] Update all 11 tool files in `src/tools/*/tool.ts`
- [ ] Update `src/tools/shared/knowledge-tool-pattern.ts`
- [ ] Keep `src/app/orchestrator.ts` importing from `@/mcp/context`
- [ ] Verify build succeeds
- [ ] Verify all tests pass

---

## PR 4: Create SDK Executor and Entry Point

**Branch**: `feature/sdk-entry-point`

**Goal**: Create the SDK module that provides simple function exports for non-MCP consumers.

### Step 4.1: Create src/sdk/executor.ts

**File**: `src/sdk/executor.ts` (NEW)

**Full Content**:

```typescript
/**
 * SDK Executor
 *
 * Provides direct tool execution without MCP orchestration overhead.
 * This is a lightweight execution path for SDK consumers who don't
 * need MCP protocol support, chain hints, or policy enforcement.
 *
 * Design: Minimal wrapper that creates context and calls tool handler.
 */

import type { Logger } from 'pino';
import { createLogger } from '@/lib/logger';
import { createToolContext } from '@/core/context';
import type { ContextOptions } from '@/core/context';
import type { Tool } from '@/types/tool';
import type { Result } from '@/types';
import { loadKnowledgeBase } from '@/knowledge/loader';
import type { ZodTypeAny } from 'zod';

// ===== SDK OPTIONS =====

/**
 * Options for SDK tool execution.
 *
 * These options allow SDK consumers to customize tool behavior
 * without needing to understand the full ToolContext interface.
 */
export interface SDKOptions {
  /**
   * Custom logger instance.
   * Defaults to a quiet logger (warn level) to minimize noise.
   */
  logger?: Logger;

  /**
   * Abort signal for cancellation support.
   * Pass an AbortController's signal to enable cancellation.
   */
  signal?: AbortSignal;

  /**
   * Progress callback for long-running operations.
   * Called with status updates during tool execution.
   */
  onProgress?: (message: string, progress?: number, total?: number) => void;
}

// ===== INTERNAL STATE =====

// Track whether knowledge base has been loaded (singleton pattern)
let knowledgeLoaded = false;

/**
 * Ensure knowledge base is loaded.
 *
 * Called once per process to load static knowledge packs.
 * Subsequent calls are no-ops.
 */
function ensureKnowledgeLoaded(logger: Logger): void {
  if (!knowledgeLoaded) {
    try {
      loadKnowledgeBase();
      knowledgeLoaded = true;
      logger.debug('Knowledge base loaded for SDK');
    } catch (error) {
      // Log but don't fail - tools can work without knowledge
      logger.warn({ error }, 'Failed to load knowledge base');
    }
  }
}

/**
 * Create a default logger for SDK usage.
 *
 * Returns a quiet logger that only shows warnings and errors.
 * SDK consumers typically don't want verbose logging.
 */
function createSDKLogger(): Logger {
  return createLogger({
    name: 'containerization-sdk',
    level: 'warn',
  });
}

// ===== EXECUTOR =====

/**
 * Execute a tool directly without MCP orchestration.
 *
 * This function provides the core SDK execution path:
 * 1. Ensures knowledge base is loaded (for tools that need it)
 * 2. Creates a minimal ToolContext
 * 3. Parses and validates input via Zod
 * 4. Calls the tool handler
 * 5. Returns the Result directly
 *
 * @param tool - The tool to execute
 * @param input - Tool input parameters (will be validated)
 * @param options - SDK options for customization
 * @returns Promise resolving to Result<TOutput>
 *
 * @example
 * ```typescript
 * import { executeTool } from '@/sdk/executor';
 * import analyzeRepoTool from '@/tools/analyze-repo/tool';
 *
 * const result = await executeTool(
 *   analyzeRepoTool,
 *   { repositoryPath: './my-app' },
 *   { onProgress: (msg) => console.log(msg) }
 * );
 *
 * if (result.ok) {
 *   console.log('Modules:', result.value.modules);
 * } else {
 *   console.error('Error:', result.error);
 * }
 * ```
 */
export async function executeTool<TSchema extends ZodTypeAny, TOutput>(
  tool: Tool<TSchema, TOutput>,
  input: unknown,
  options: SDKOptions = {},
): Promise<Result<TOutput>> {
  // Create or use provided logger
  const logger = options.logger ?? createSDKLogger();

  // Ensure knowledge is loaded (idempotent)
  ensureKnowledgeLoaded(logger);

  // Build context options
  const contextOptions: ContextOptions = {
    signal: options.signal,
    progress: options.onProgress
      ? async (msg: string, prog?: number, total?: number) => {
          options.onProgress!(msg, prog, total);
        }
      : undefined,
    // Note: SDK v1 does not support policies - keeps things simple
    // Policy support can be added in a future version if needed
  };

  // Create tool context
  const ctx = createToolContext(logger, contextOptions);

  // Parse and validate input using tool's Zod schema
  // This will throw if input is invalid
  let parsedInput;
  try {
    parsedInput = tool.parse(input);
  } catch (error) {
    // Return a Failure result for validation errors instead of throwing
    const { Failure } = await import('@/types/index.js');
    const message = error instanceof Error ? error.message : 'Invalid input';
    return Failure(`Validation failed: ${message}`, {
      message: 'Input validation failed',
      hint: 'Check that all required parameters are provided with correct types',
      resolution: 'Review the tool documentation for expected input format',
    });
  }

  // Execute the tool handler
  return tool.handler(parsedInput, ctx);
}
```

### Step 4.2: Create src/sdk/types.ts

**File**: `src/sdk/types.ts` (NEW)

**Full Content**:

```typescript
/**
 * SDK Type Exports
 *
 * Re-exports all types that SDK consumers might need.
 * This provides a single import location for type-only imports.
 */

// ===== CORE TYPES =====

export type { ToolContext, ProgressReporter } from '@/core/context';

// ===== RESULT TYPES =====

export type { Result, ErrorGuidance } from '@/types';
// Note: Success and Failure are values, exported from main sdk/index.ts

// ===== SDK-SPECIFIC TYPES =====

export type { SDKOptions } from './executor.js';

// ===== ANALYZE-REPO TYPES =====

export type {
  RepositoryAnalysis,
  ModuleInfo,
  FrameworkInfo,
  BuildSystemInfo,
} from '@/tools/analyze-repo/schema';

// ===== GENERATE-DOCKERFILE TYPES =====

export type {
  DockerfilePlan,
  GenerateDockerfileParams,
  BaseImageRecommendation,
  DockerfileRequirement,
  DockerfileAnalysis,
  EnhancementGuidance,
} from '@/tools/generate-dockerfile/schema';

// ===== FIX-DOCKERFILE TYPES =====

export type {
  DockerfileFixPlan,
  FixDockerfileParams,
} from '@/tools/fix-dockerfile/schema';

// ===== BUILD-IMAGE TYPES =====

export type { BuildImageParams } from '@/tools/build-image/schema';
export type { BuildImageResult } from '@/tools/build-image/tool';

// ===== SCAN-IMAGE TYPES =====

export type { ScanImageParams } from '@/tools/scan-image/schema';
export type { ScanImageResult } from '@/tools/scan-image/tool';

// ===== TAG-IMAGE TYPES =====

export type { TagImageParams } from '@/tools/tag-image/schema';
export type { TagImageResult } from '@/tools/tag-image/tool';

// ===== PUSH-IMAGE TYPES =====

export type { PushImageParams } from '@/tools/push-image/schema';
export type { PushImageResult } from '@/tools/push-image/tool';

// ===== GENERATE-K8S-MANIFESTS TYPES =====

export type {
  ManifestPlan,
  GenerateK8sManifestsParams,
} from '@/tools/generate-k8s-manifests/schema';

// ===== PREPARE-CLUSTER TYPES =====

export type { PrepareClusterParams } from '@/tools/prepare-cluster/schema';
export type { PrepareClusterResult } from '@/tools/prepare-cluster/tool';

// ===== VERIFY-DEPLOY TYPES =====

export type { VerifyDeployParams } from '@/tools/verify-deploy/schema';
export type { VerifyDeploymentResult } from '@/tools/verify-deploy/tool';

// ===== OPS TYPES =====

export type { OpsParams } from '@/tools/ops/schema';
export type { PingResult, ServerStatusResult } from '@/tools/ops/tool';
```

### Step 4.3: Create src/sdk/index.ts

**File**: `src/sdk/index.ts` (NEW)

**Full Content**:

```typescript
/**
 * Containerization Assist SDK
 *
 * Provides direct access to all 11 containerization tools without requiring
 * the MCP (Model Context Protocol) server infrastructure.
 *
 * This SDK is designed for:
 * - VS Code extension developers integrating with Copilot
 * - Direct programmatic usage without MCP overhead
 * - Lightweight tool execution in Node.js applications
 *
 * @example
 * ```typescript
 * import { analyzeRepo, buildImage, scanImage } from 'containerization-assist-mcp/sdk';
 *
 * // Full containerization workflow
 * const analysis = await analyzeRepo({ repositoryPath: './my-app' });
 * const build = await buildImage({ path: './my-app', imageName: 'myapp:v1' });
 * const scan = await scanImage({ imageId: 'myapp:v1' });
 * ```
 *
 * @packageDocumentation
 */

import { executeTool, type SDKOptions } from './executor.js';

// ===== TOOL IMPORTS (all 11 tools) =====

import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';
import buildImageTool from '@/tools/build-image/tool';
import scanImageTool from '@/tools/scan-image/tool';
import tagImageTool from '@/tools/tag-image/tool';
import pushImageTool from '@/tools/push-image/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import prepareClusterTool from '@/tools/prepare-cluster/tool';
import verifyDeployTool from '@/tools/verify-deploy/tool';
import opsTool from '@/tools/ops/tool';

// ===== TYPE RE-EXPORTS =====

// Result types
export type { Result, ErrorGuidance } from '@/types';
export { Success, Failure } from '@/types';

// SDK options
export type { SDKOptions } from './executor.js';

// Tool-specific types (most commonly needed)
export type { RepositoryAnalysis, ModuleInfo } from '@/tools/analyze-repo/schema';
export type { DockerfilePlan } from '@/tools/generate-dockerfile/schema';
export type { DockerfileFixPlan } from '@/tools/fix-dockerfile/schema';
export type { BuildImageResult } from '@/tools/build-image/tool';
export type { ScanImageResult } from '@/tools/scan-image/tool';
export type { TagImageResult } from '@/tools/tag-image/tool';
export type { PushImageResult } from '@/tools/push-image/tool';
export type { ManifestPlan } from '@/tools/generate-k8s-manifests/schema';
export type { PrepareClusterResult } from '@/tools/prepare-cluster/tool';
export type { VerifyDeploymentResult } from '@/tools/verify-deploy/tool';

// Full type exports available via sdk/types
// import type { ... } from 'containerization-assist-mcp/sdk/types';

// ===== SIMPLIFIED FUNCTION EXPORTS =====

// ----- Analysis Tools -----

/**
 * Analyze a repository to detect language, framework, and dependencies.
 */
export async function analyzeRepo(
  input: { repositoryPath: string },
  options?: SDKOptions,
) {
  return executeTool(analyzeRepoTool, input, options);
}

// ----- Dockerfile Tools -----

/**
 * Generate Dockerfile recommendations for a repository.
 */
export async function generateDockerfile(
  input: {
    repositoryPath: string;
    targetPlatform: string;
    modulePath?: string;
    language?: string;
    languageVersion?: string;
    framework?: string;
    environment?: string;
  },
  options?: SDKOptions,
) {
  return executeTool(generateDockerfileTool, input, options);
}

/**
 * Fix and optimize an existing Dockerfile.
 */
export async function fixDockerfile(
  input: {
    dockerfilePath: string;
    repositoryPath?: string;
    language?: string;
    framework?: string;
  },
  options?: SDKOptions,
) {
  return executeTool(fixDockerfileTool, input, options);
}

// ----- Image Tools -----

/**
 * Build a Docker image from a Dockerfile.
 * Requires Docker daemon to be running.
 */
export async function buildImage(
  input: {
    path?: string;
    dockerfile?: string;
    dockerfilePath?: string;
    imageName?: string;
    tags?: string[];
    buildArgs?: Record<string, string>;
    platform?: string;
  },
  options?: SDKOptions,
) {
  return executeTool(buildImageTool, input, options);
}

/**
 * Scan a Docker image for security vulnerabilities.
 * Requires Trivy to be installed for full functionality.
 */
export async function scanImage(
  input: {
    imageId: string;
    scanner?: string;
    severity?: string;
  },
  options?: SDKOptions,
) {
  return executeTool(scanImageTool, input, options);
}

/**
 * Tag a Docker image with additional tags.
 * Requires Docker daemon to be running.
 */
export async function tagImage(
  input: {
    sourceImage: string;
    targetImage: string;
  },
  options?: SDKOptions,
) {
  return executeTool(tagImageTool, input, options);
}

/**
 * Push a Docker image to a registry.
 * Requires Docker daemon and registry authentication.
 */
export async function pushImage(
  input: {
    imageId: string;
    registry?: string;
  },
  options?: SDKOptions,
) {
  return executeTool(pushImageTool, input, options);
}

// ----- Kubernetes Tools -----

/**
 * Generate Kubernetes manifests for deployment.
 */
export async function generateK8sManifests(
  input: {
    repositoryPath: string;
    imageName: string;
    namespace?: string;
    replicas?: number;
    port?: number;
    environment?: string;
  },
  options?: SDKOptions,
) {
  return executeTool(generateK8sManifestsTool, input, options);
}

/**
 * Prepare a Kubernetes cluster for deployment.
 * Requires kubectl configured with cluster access.
 */
export async function prepareCluster(
  input: {
    namespace: string;
    createNamespace?: boolean;
  },
  options?: SDKOptions,
) {
  return executeTool(prepareClusterTool, input, options);
}

/**
 * Verify a Kubernetes deployment status.
 * Requires kubectl configured with cluster access.
 */
export async function verifyDeploy(
  input: {
    namespace: string;
    deploymentName: string;
    timeout?: number;
  },
  options?: SDKOptions,
) {
  return executeTool(verifyDeployTool, input, options);
}

// ----- Operational Tools -----

/**
 * Operational utilities (ping, status).
 */
export async function ops(
  input: {
    action: 'ping' | 'status';
  },
  options?: SDKOptions,
) {
  return executeTool(opsTool, input, options);
}

// ===== ADVANCED: DIRECT TOOL ACCESS =====

/**
 * Direct access to all 11 tool objects for advanced use cases.
 *
 * Use this when you need:
 * - Access to tool schemas for validation
 * - Tool metadata and descriptions
 * - Custom execution patterns
 *
 * @example
 * ```typescript
 * import { tools } from 'containerization-assist-mcp/sdk';
 *
 * // Access tool schema
 * const schema = tools.analyzeRepo.schema;
 *
 * // Access tool metadata
 * console.log(tools.buildImage.description);
 * ```
 */
export const tools = {
  // Analysis
  analyzeRepo: analyzeRepoTool,
  // Dockerfile
  generateDockerfile: generateDockerfileTool,
  fixDockerfile: fixDockerfileTool,
  // Image
  buildImage: buildImageTool,
  scanImage: scanImageTool,
  tagImage: tagImageTool,
  pushImage: pushImageTool,
  // Kubernetes
  generateK8sManifests: generateK8sManifestsTool,
  prepareCluster: prepareClusterTool,
  verifyDeploy: verifyDeployTool,
  // Operations
  ops: opsTool,
} as const;

/**
 * Execute any tool directly with full control.
 *
 * Use this when you need to:
 * - Execute tools not exposed via simplified functions
 * - Pass custom options to the executor
 * - Handle tool objects dynamically
 *
 * @example
 * ```typescript
 * import { executeTool, tools } from 'containerization-assist-mcp/sdk';
 *
 * const result = await executeTool(
 *   tools.analyzeRepo,
 *   { repositoryPath: '.' },
 *   { signal: controller.signal }
 * );
 * ```
 */
export { executeTool } from './executor.js';
```

### Step 4.4: Verification

```bash
# Build should succeed
npm run build:esm

# Verify no MCP imports in SDK
grep -r "@modelcontextprotocol" src/sdk/
# Should return nothing

grep -r "@/mcp" src/sdk/
# Should return nothing

# Check imports are correct
grep -r "from '@/core" src/sdk/
# Should show core imports
```

### PR 4 Checklist

- [ ] Create `src/sdk/executor.ts` with executeTool function
- [ ] Create `src/sdk/types.ts` with type re-exports for all 11 tools
- [ ] Create `src/sdk/index.ts` with simplified function exports for all 11 tools:
  - [ ] `analyzeRepo`
  - [ ] `generateDockerfile`
  - [ ] `fixDockerfile`
  - [ ] `buildImage`
  - [ ] `scanImage`
  - [ ] `tagImage`
  - [ ] `pushImage`
  - [ ] `generateK8sManifests`
  - [ ] `prepareCluster`
  - [ ] `verifyDeploy`
  - [ ] `ops`
- [ ] Export `tools` object with all 11 tool references
- [ ] Verify no MCP imports in `src/sdk/`
- [ ] Verify build succeeds

---

## PR 5: Add Package Exports and Documentation

**Branch**: `feature/sdk-package-exports`

**Goal**: Configure package.json exports and update documentation.

### Step 5.1: Update package.json

**File**: `package.json`

**Action**: Add SDK export to the `exports` field:

```json
{
  "exports": {
    ".": {
      "import": {
        "types": "./dist/src/index.d.ts",
        "default": "./dist/src/index.js"
      },
      "require": {
        "types": "./dist-cjs/src/index.d.ts",
        "default": "./dist-cjs/src/index.js"
      }
    },
    "./sdk": {
      "import": {
        "types": "./dist/src/sdk/index.d.ts",
        "default": "./dist/src/sdk/index.js"
      },
      "require": {
        "types": "./dist-cjs/src/sdk/index.d.ts",
        "default": "./dist-cjs/src/sdk/index.js"
      }
    },
    "./sdk/types": {
      "import": {
        "types": "./dist/src/sdk/types.d.ts",
        "default": "./dist/src/sdk/types.js"
      },
      "require": {
        "types": "./dist-cjs/src/sdk/types.d.ts",
        "default": "./dist-cjs/src/sdk/types.js"
      }
    }
  }
}
```

### Step 5.2: Update CLAUDE.md

**File**: `CLAUDE.md`

**Action**: Add SDK section after the "Quick Commands" section:

```markdown
## SDK Usage (Non-MCP)

For consumers who want to use tools directly without MCP server infrastructure (e.g., VS Code extensions, Copilot integrations):

```typescript
import {
  // Analysis
  analyzeRepo,
  // Dockerfile
  generateDockerfile,
  fixDockerfile,
  // Image
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  // Kubernetes
  generateK8sManifests,
  prepareCluster,
  verifyDeploy,
  // Operations
  ops,
} from 'containerization-assist-mcp/sdk';

// Simple function calls - no MCP server needed
const analysis = await analyzeRepo({ repositoryPath: './myapp' });

if (analysis.ok) {
  console.log('Modules:', analysis.value.modules);
}

// Full containerization workflow
const dockerfile = await generateDockerfile({
  repositoryPath: './myapp',
  targetPlatform: 'linux/amd64'
});
const build = await buildImage({ path: './myapp', imageName: 'myapp:v1' });
const scan = await scanImage({ imageId: 'myapp:v1' });
const manifests = await generateK8sManifests({
  repositoryPath: './myapp',
  imageName: 'myapp:v1'
});
```

### SDK vs MCP

| Feature | SDK (`/sdk`) | MCP (default) |
|---------|--------------|---------------|
| All 11 tools available | Yes | Yes |
| Function call interface | Yes | Via orchestrator |
| MCP protocol support | No | Yes |
| Chain hints | No | Yes |
| Policy enforcement | No | Yes |
| Progress via MCP notifications | No | Yes |
| Lightweight | Yes | Full featured |

### Available SDK Functions

| Category | Functions |
|----------|-----------|
| Analysis | `analyzeRepo` |
| Dockerfile | `generateDockerfile`, `fixDockerfile` |
| Image | `buildImage`, `scanImage`, `tagImage`, `pushImage` |
| Kubernetes | `generateK8sManifests`, `prepareCluster`, `verifyDeploy` |
| Operations | `ops` |

### SDK Options

```typescript
import { analyzeRepo } from 'containerization-assist-mcp/sdk';

const result = await analyzeRepo(
  { repositoryPath: '.' },
  {
    // Custom logger (default: quiet logger)
    logger: myPinoLogger,

    // Cancellation support
    signal: abortController.signal,

    // Progress callback
    onProgress: (message, progress, total) => {
      console.log(`${message} (${progress}/${total})`);
    },
  }
);
```

### Advanced SDK Usage

```typescript
import { tools, executeTool } from 'containerization-assist-mcp/sdk';

// Access tool metadata
console.log(tools.analyzeRepo.description);

// Access Zod schemas for validation
const schema = tools.buildImage.schema;

// Execute any tool directly
const result = await executeTool(tools.analyzeRepo, { repositoryPath: '.' });
```
```

### Step 5.3: Update README.md (if applicable)

**File**: `README.md`

**Action**: Add brief SDK mention in installation/usage section:

```markdown
### SDK Usage (Without MCP)

For direct tool usage without MCP protocol:

```typescript
import { analyzeRepo, buildImage } from 'containerization-assist-mcp/sdk';

const result = await analyzeRepo({ repositoryPath: './myapp' });
```

See [CLAUDE.md](./CLAUDE.md) for full SDK documentation.
```

### PR 5 Checklist

- [ ] Update `package.json` with SDK exports
- [ ] Update `CLAUDE.md` with SDK documentation
- [ ] Update `README.md` with SDK mention
- [ ] Verify `npm run build` succeeds
- [ ] Verify SDK can be imported: `node -e "import('containerization-assist-mcp/sdk')"`

---

## PR 6: Add SDK Tests

**Branch**: `feature/sdk-tests`

**Goal**: Add comprehensive tests for SDK functionality.

### Step 6.1: Create test/sdk/executor.test.ts

**File**: `test/sdk/executor.test.ts` (NEW)

**Full Content**:

```typescript
/**
 * SDK Executor Tests
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { executeTool } from '../../src/sdk/executor';
import { Success, Failure } from '../../src/types';
import { z } from 'zod';
import { tool } from '../../src/types/tool';

// Mock tool for testing
const mockSchema = z.object({
  input: z.string(),
  optional: z.number().optional(),
});

const createMockTool = (handler: (input: any, ctx: any) => Promise<any>) =>
  tool({
    name: 'analyze-repo' as any, // Use valid tool name
    description: 'Mock tool for testing',
    schema: mockSchema,
    metadata: { knowledgeEnhanced: false },
    handler,
  });

describe('executeTool', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('executes tool handler with valid input', async () => {
    const mockHandler = vi.fn().mockResolvedValue(Success({ result: 'success' }));
    const mockTool = createMockTool(mockHandler);

    const result = await executeTool(mockTool, { input: 'test' });

    expect(result.ok).toBe(true);
    expect(result.value).toEqual({ result: 'success' });
    expect(mockHandler).toHaveBeenCalledTimes(1);
  });

  it('returns Failure for invalid input', async () => {
    const mockHandler = vi.fn().mockResolvedValue(Success({ result: 'success' }));
    const mockTool = createMockTool(mockHandler);

    const result = await executeTool(mockTool, { wrong: 'field' });

    expect(result.ok).toBe(false);
    expect(result.error).toContain('Validation failed');
    expect(mockHandler).not.toHaveBeenCalled();
  });

  it('passes abort signal to context', async () => {
    const mockHandler = vi.fn().mockImplementation(async (input, ctx) => {
      expect(ctx.signal).toBeDefined();
      return Success({ result: 'success' });
    });
    const mockTool = createMockTool(mockHandler);
    const controller = new AbortController();

    await executeTool(mockTool, { input: 'test' }, { signal: controller.signal });

    expect(mockHandler).toHaveBeenCalled();
  });

  it('calls onProgress callback', async () => {
    const onProgress = vi.fn();
    const mockHandler = vi.fn().mockImplementation(async (input, ctx) => {
      if (ctx.progress) {
        await ctx.progress('Step 1', 1, 3);
        await ctx.progress('Step 2', 2, 3);
      }
      return Success({ result: 'success' });
    });
    const mockTool = createMockTool(mockHandler);

    await executeTool(mockTool, { input: 'test' }, { onProgress });

    expect(onProgress).toHaveBeenCalledTimes(2);
    expect(onProgress).toHaveBeenCalledWith('Step 1', 1, 3);
    expect(onProgress).toHaveBeenCalledWith('Step 2', 2, 3);
  });

  it('handles tool handler errors', async () => {
    const mockHandler = vi.fn().mockResolvedValue(
      Failure('Something went wrong', {
        message: 'Error occurred',
        hint: 'Try again',
        resolution: 'Check input',
      })
    );
    const mockTool = createMockTool(mockHandler);

    const result = await executeTool(mockTool, { input: 'test' });

    expect(result.ok).toBe(false);
    expect(result.error).toBe('Something went wrong');
  });

  it('uses custom logger when provided', async () => {
    const customLogger = {
      debug: vi.fn(),
      info: vi.fn(),
      warn: vi.fn(),
      error: vi.fn(),
      child: vi.fn().mockReturnThis(),
    } as any;

    const mockHandler = vi.fn().mockResolvedValue(Success({ result: 'success' }));
    const mockTool = createMockTool(mockHandler);

    await executeTool(mockTool, { input: 'test' }, { logger: customLogger });

    expect(mockHandler).toHaveBeenCalled();
    // Verify custom logger was passed to context
    const ctx = mockHandler.mock.calls[0][1];
    expect(ctx.logger).toBe(customLogger);
  });
});
```

### Step 6.2: Create test/sdk/index.test.ts

**File**: `test/sdk/index.test.ts` (NEW)

**Full Content**:

```typescript
/**
 * SDK Entry Point Tests
 */

import { describe, it, expect } from 'vitest';
import {
  // All 11 tool functions
  analyzeRepo,
  generateDockerfile,
  fixDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  generateK8sManifests,
  prepareCluster,
  verifyDeploy,
  ops,
  // Advanced
  tools,
  executeTool,
  Success,
  Failure,
} from '../../src/sdk';

describe('SDK exports', () => {
  describe('function exports (all 11 tools)', () => {
    it('exports analyzeRepo function', () => {
      expect(typeof analyzeRepo).toBe('function');
    });

    it('exports generateDockerfile function', () => {
      expect(typeof generateDockerfile).toBe('function');
    });

    it('exports fixDockerfile function', () => {
      expect(typeof fixDockerfile).toBe('function');
    });

    it('exports buildImage function', () => {
      expect(typeof buildImage).toBe('function');
    });

    it('exports scanImage function', () => {
      expect(typeof scanImage).toBe('function');
    });

    it('exports tagImage function', () => {
      expect(typeof tagImage).toBe('function');
    });

    it('exports pushImage function', () => {
      expect(typeof pushImage).toBe('function');
    });

    it('exports generateK8sManifests function', () => {
      expect(typeof generateK8sManifests).toBe('function');
    });

    it('exports prepareCluster function', () => {
      expect(typeof prepareCluster).toBe('function');
    });

    it('exports verifyDeploy function', () => {
      expect(typeof verifyDeploy).toBe('function');
    });

    it('exports ops function', () => {
      expect(typeof ops).toBe('function');
    });

    it('exports executeTool function', () => {
      expect(typeof executeTool).toBe('function');
    });
  });

  describe('tools object (all 11 tools)', () => {
    it('exports tools.analyzeRepo', () => {
      expect(tools.analyzeRepo).toBeDefined();
      expect(tools.analyzeRepo.name).toBe('analyze-repo');
    });

    it('exports tools.generateDockerfile', () => {
      expect(tools.generateDockerfile).toBeDefined();
      expect(tools.generateDockerfile.name).toBe('generate-dockerfile');
    });

    it('exports tools.fixDockerfile', () => {
      expect(tools.fixDockerfile).toBeDefined();
      expect(tools.fixDockerfile.name).toBe('fix-dockerfile');
    });

    it('exports tools.buildImage', () => {
      expect(tools.buildImage).toBeDefined();
      expect(tools.buildImage.name).toBe('build-image');
    });

    it('exports tools.scanImage', () => {
      expect(tools.scanImage).toBeDefined();
      expect(tools.scanImage.name).toBe('scan-image');
    });

    it('exports tools.tagImage', () => {
      expect(tools.tagImage).toBeDefined();
      expect(tools.tagImage.name).toBe('tag-image');
    });

    it('exports tools.pushImage', () => {
      expect(tools.pushImage).toBeDefined();
      expect(tools.pushImage.name).toBe('push-image');
    });

    it('exports tools.generateK8sManifests', () => {
      expect(tools.generateK8sManifests).toBeDefined();
      expect(tools.generateK8sManifests.name).toBe('generate-k8s-manifests');
    });

    it('exports tools.prepareCluster', () => {
      expect(tools.prepareCluster).toBeDefined();
      expect(tools.prepareCluster.name).toBe('prepare-cluster');
    });

    it('exports tools.verifyDeploy', () => {
      expect(tools.verifyDeploy).toBeDefined();
      expect(tools.verifyDeploy.name).toBe('verify-deploy');
    });

    it('exports tools.ops', () => {
      expect(tools.ops).toBeDefined();
      expect(tools.ops.name).toBe('ops');
    });

    it('exports exactly 11 tools', () => {
      expect(Object.keys(tools)).toHaveLength(11);
    });
  });

  describe('type exports', () => {
    it('exports Success constructor', () => {
      const result = Success({ value: 'test' });
      expect(result.ok).toBe(true);
    });

    it('exports Failure constructor', () => {
      const result = Failure('error');
      expect(result.ok).toBe(false);
    });
  });
});

describe('SDK integration', () => {
  it('analyzeRepo validates input', async () => {
    // @ts-expect-error - intentionally passing invalid input
    const result = await analyzeRepo({});

    expect(result.ok).toBe(false);
    expect(result.error).toContain('Validation failed');
  });

  it('analyzeRepo works with valid path', async () => {
    const result = await analyzeRepo({ repositoryPath: process.cwd() });

    // May succeed or fail depending on repo structure, but should not throw
    expect(typeof result.ok).toBe('boolean');
  });
});
```

### Step 6.3: Create test/sdk/no-mcp-imports.test.ts

**File**: `test/sdk/no-mcp-imports.test.ts` (NEW)

**Full Content**:

```typescript
/**
 * SDK No-MCP-Imports Tests
 *
 * Verifies that the SDK has no dependencies on MCP packages.
 * This is critical for consumers who want to avoid pulling in MCP deps.
 */

import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const SDK_DIR = join(process.cwd(), 'src/sdk');
const CORE_DIR = join(process.cwd(), 'src/core');

function getTypeScriptFiles(dir: string): string[] {
  try {
    return readdirSync(dir)
      .filter(f => f.endsWith('.ts'))
      .map(f => join(dir, f));
  } catch {
    return [];
  }
}

function checkFileForMCPImports(filePath: string): string[] {
  const content = readFileSync(filePath, 'utf-8');
  const violations: string[] = [];

  // Check for direct MCP SDK imports
  if (content.includes('@modelcontextprotocol')) {
    violations.push(`${filePath}: imports from @modelcontextprotocol`);
  }

  // Check for imports from @/mcp (internal MCP module)
  if (content.includes("from '@/mcp")) {
    violations.push(`${filePath}: imports from @/mcp`);
  }

  return violations;
}

describe('SDK MCP independence', () => {
  it('src/sdk/ has no MCP imports', () => {
    const files = getTypeScriptFiles(SDK_DIR);
    const violations = files.flatMap(checkFileForMCPImports);

    expect(violations).toEqual([]);
  });

  it('src/core/ has no MCP imports', () => {
    const files = getTypeScriptFiles(CORE_DIR);
    const violations = files.flatMap(checkFileForMCPImports);

    expect(violations).toEqual([]);
  });
});
```

### Step 6.4: Verification

```bash
# Run SDK tests
npm test -- --testPathPattern="sdk"

# Run all tests
npm test
```

### PR 6 Checklist

- [ ] Create `test/sdk/executor.test.ts`
- [ ] Create `test/sdk/index.test.ts`
- [ ] Create `test/sdk/no-mcp-imports.test.ts`
- [ ] All tests pass
- [ ] Test coverage for SDK is reasonable

---

## Verification Checklist (Final)

After all PRs are merged:

```bash
# 1. Build succeeds
npm run build

# 2. All tests pass
npm test

# 3. SDK has no MCP dependencies
grep -r "@modelcontextprotocol" src/sdk/ src/core/
# Should return nothing

# 4. SDK can be imported and used
node -e "
  import('containerization-assist-mcp/sdk').then(sdk => {
    console.log('SDK loaded');
    console.log('Functions:', Object.keys(sdk).filter(k => typeof sdk[k] === 'function'));
    console.log('Tools:', Object.keys(sdk.tools));
  });
"

# 5. MCP still works
npm run mcp:inspect
```

---

## Rollback Plan

If issues are discovered after merging:

1. **PR 1-3 issues**: Core/MCP layer issues
   - Re-export `ToolContext` from `@/mcp/context` to maintain backward compatibility
   - Tools can import from either location

2. **PR 4-6 issues**: SDK issues
   - SDK is additive, can be reverted without affecting MCP functionality
   - Remove SDK exports from package.json

---

## Future Enhancements

After initial implementation:

1. **SDK Policy Support**: Add optional policy loading
2. **Additional Tools**: Expose more tools via SDK
3. **Streaming**: Add streaming support for progress
4. **Separate Package**: Consider `containerization-assist-core` package
