# SDK Decoupling from MCP - Implementation Plan

## Overview

This plan describes how to refactor the codebase to provide a standalone SDK that allows consumers to use all 11 tools as simple function calls without any MCP (Model Context Protocol) dependencies.

**See also**: `sdk-decoupling-from-mcp-detailed.md` for step-by-step implementation instructions.

## Problem Statement

**Consumer Request**: Provide agent tool support for VS Code Copilot integration. The consumer wants to call tools as simple functions without MCP dependencies.

**Scope**: All 11 tools will be exposed via SDK:
- `analyze-repo`, `generate-dockerfile`, `fix-dockerfile`
- `build-image`, `scan-image`, `tag-image`, `push-image`
- `generate-k8s-manifests`, `prepare-cluster`, `verify-deploy`
- `ops`

### Current Architecture Coupling

The codebase has MCP coupling at three layers:

1. **Tool Level** - Every tool imports `ToolContext` from `@/mcp/context`:
   ```typescript
   // src/tools/analyze-repo/tool.ts
   import type { ToolContext } from '@/mcp/context';
   ```

2. **Type Level** - The core `Tool` interface imports from MCP:
   ```typescript
   // src/types/tool.ts
   import type { ToolContext } from '@/mcp/context';
   ```

3. **App/Orchestrator Level** - Imports MCP SDK directly:
   ```typescript
   // src/app/index.ts
   import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
   import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
   ```

### Files Importing from `@/mcp/context`

15 files currently import from `@/mcp/context`:
- `src/types/tool.ts`
- `src/app/orchestrator.ts`
- `src/cli/policy-simulate.ts`
- All 11 tool files in `src/tools/*/tool.ts`
- `src/tools/shared/knowledge-tool-pattern.ts`

### Impact on Consumers

When a consumer imports tools today, they transitively pull in:
- `@modelcontextprotocol/sdk` package
- MCP-specific types and concepts
- Unnecessary bundle size for non-MCP use cases

## Design Principles

1. **Single Source of Truth**: `ToolContext` interface defined once, not duplicated
2. **Dependency Inversion**: Core should not depend on MCP; MCP should depend on Core
3. **Minimal Changes**: Existing MCP integration requires minimal/no breaking changes
4. **Clean Imports**: SDK consumers see no "mcp" in their import paths
5. **Zero Code Duplication**: Tool handlers remain unchanged, only imports shift

## Solution Architecture

### New Directory Structure

```
src/
├── core/                          # NEW - Zero MCP dependencies
│   ├── context.ts                 # ToolContext interface + createToolContext
│   ├── types.ts                   # Re-export core types needed by tools
│   └── index.ts                   # Core exports
│
├── sdk/                           # NEW - SDK entry point for non-MCP consumers
│   ├── index.ts                   # Simplified function exports
│   ├── executor.ts                # Direct tool execution (no orchestrator)
│   └── types.ts                   # SDK-specific types
│
├── mcp/                           # EXISTING - MCP integration layer
│   ├── context.ts                 # RE-EXPORTS from core + MCP-specific helpers
│   ├── context-helpers.ts         # MCP notification helpers (unchanged)
│   ├── mcp-server.ts              # MCP server (unchanged)
│   └── ...
│
├── tools/                         # EXISTING - Tool implementations
│   └── */tool.ts                  # UPDATE: import from @/core/context
│
├── types/                         # EXISTING - Type definitions
│   └── tool.ts                    # UPDATE: import from @/core/context
│
└── app/                           # EXISTING - App runtime (MCP-focused)
    └── ...                        # Unchanged
```

### Dependency Graph (After Refactoring)

```
┌─────────────────────────────────────────────────────────────────┐
│                        CONSUMER LAYER                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   SDK Consumer                         MCP Consumer              │
│   (VS Code Extension)                  (Claude Desktop)          │
│         │                                    │                   │
│         ▼                                    ▼                   │
│   ┌───────────┐                      ┌─────────────┐            │
│   │  sdk/     │                      │  app/       │            │
│   │  index.ts │                      │  index.ts   │            │
│   └─────┬─────┘                      └──────┬──────┘            │
│         │                                   │                    │
├─────────┼───────────────────────────────────┼────────────────────┤
│         │           INTEGRATION LAYER       │                    │
│         │                                   │                    │
│         │                            ┌──────▼──────┐            │
│         │                            │    mcp/     │            │
│         │                            │  context.ts │            │
│         │                            └──────┬──────┘            │
│         │                                   │                    │
│         │         ┌─────────────────────────┘                   │
│         │         │ (re-exports + MCP helpers)                  │
│         │         │                                              │
├─────────┼─────────┼──────────────────────────────────────────────┤
│         │         │            CORE LAYER                        │
│         │         │                                              │
│         ▼         ▼                                              │
│   ┌─────────────────────┐       ┌──────────────────┐            │
│   │     core/           │       │     tools/       │            │
│   │   context.ts        │◄──────│    */tool.ts     │            │
│   │   (ToolContext)     │       │                  │            │
│   └─────────────────────┘       └──────────────────┘            │
│                                                                  │
│   NO MCP DEPENDENCIES IN THIS LAYER                             │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Phases

### Phase 1: Create Core Layer

**Goal**: Extract `ToolContext` and related types to a new `src/core/` directory with zero MCP dependencies.

#### 1.1 Create `src/core/context.ts`

Move from `src/mcp/context.ts`:
- `ToolContext` interface
- `ProgressReporter` type
- `ContextOptions` interface (simplified, no MCP-specific options)
- `createToolContext()` function (simplified version)

```typescript
// src/core/context.ts

import type { Logger } from 'pino';
import type { RegoEvaluator } from '@/config/policy-rego';

/**
 * Progress reporting function for tool execution feedback
 */
export type ProgressReporter = (
  message: string,
  progress?: number,
  total?: number,
) => Promise<void>;

/**
 * Core tool execution context - no MCP dependencies
 */
export interface ToolContext {
  signal: AbortSignal | undefined;
  progress: ProgressReporter | undefined;
  logger: Logger;
  policy?: RegoEvaluator;
  queryConfig<T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null>;
}

/**
 * Options for creating a tool context
 */
export interface ContextOptions {
  signal?: AbortSignal;
  progress?: ProgressReporter;
  policy?: RegoEvaluator;
}

/**
 * Create a ToolContext for tool execution (core version, no MCP)
 */
export function createToolContext(logger: Logger, options: ContextOptions = {}): ToolContext {
  return {
    logger,
    signal: options.signal,
    progress: options.progress,
    ...(options.policy && { policy: options.policy }),
    queryConfig: async <T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
      if (!options.policy) {
        logger.debug({ packageName }, 'No policy configured, returning null for config query');
        return null;
      }
      return options.policy.queryConfig<T>(packageName, input);
    },
  };
}
```

#### 1.2 Create `src/core/index.ts`

```typescript
// src/core/index.ts

export type { ToolContext, ProgressReporter, ContextOptions } from './context.js';
export { createToolContext } from './context.js';
```

#### 1.3 Add Path Alias

Update `tsconfig.json`:
```json
{
  "compilerOptions": {
    "paths": {
      "@/core/*": ["src/core/*"],
      // ... existing aliases
    }
  }
}
```

### Phase 2: Update MCP Layer to Re-export from Core

**Goal**: Make `src/mcp/context.ts` a thin wrapper that re-exports from core and adds MCP-specific functionality.

#### 2.1 Refactor `src/mcp/context.ts`

```typescript
// src/mcp/context.ts

// Re-export everything from core
export type { ToolContext, ProgressReporter, ContextOptions } from '@/core/context';
export { createToolContext as createCoreToolContext } from '@/core/context';

// MCP-specific imports
import type { Logger } from 'pino';
import type { RegoEvaluator } from '@/config/policy-rego';
import type { ToolContext, ContextOptions as CoreContextOptions } from '@/core/context';
import { createToolContext as coreCreateToolContext } from '@/core/context';
import { extractProgressReporter } from './context-helpers.js';

// MCP-specific context options (extends core)
export interface MCPContextOptions extends CoreContextOptions {
  /** MCP notification callback for progress updates */
  sendNotification?: (notification: unknown) => Promise<void>;
}

/**
 * Create a ToolContext with MCP progress notification support
 */
export function createToolContext(logger: Logger, options: MCPContextOptions = {}): ToolContext {
  // If MCP-specific options provided, use enhanced progress reporter
  if (options.sendNotification || (options.progress && typeof options.progress !== 'function')) {
    const progressReporter = extractProgressReporter(
      options.progress,
      logger,
      options.sendNotification,
    );

    return coreCreateToolContext(logger, {
      ...options,
      progress: progressReporter,
    });
  }

  // Otherwise, use core implementation
  return coreCreateToolContext(logger, options);
}

// Keep MCP-specific exports
export type { EnhancedProgressReporter } from './context-helpers.js';
export { extractProgressToken, createProgressReporter } from './context-helpers.js';
```

### Phase 3: Update All Tool Imports

**Goal**: Change all tools to import `ToolContext` from `@/core/context` instead of `@/mcp/context`.

#### Files to Update (15 total)

| File | Change |
|------|--------|
| `src/types/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/app/orchestrator.ts` | `@/mcp/context` → `@/core/context` |
| `src/cli/policy-simulate.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/analyze-repo/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/build-image/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/fix-dockerfile/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/generate-dockerfile/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/generate-k8s-manifests/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/ops/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/prepare-cluster/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/push-image/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/scan-image/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/tag-image/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/verify-deploy/tool.ts` | `@/mcp/context` → `@/core/context` |
| `src/tools/shared/knowledge-tool-pattern.ts` | `@/mcp/context` → `@/core/context` |

#### Automated Update Command

```bash
# Find and replace in all relevant files
find src -name "*.ts" -exec sed -i "s|from '@/mcp/context'|from '@/core/context'|g" {} \;
```

### Phase 4: Create SDK Entry Point

**Goal**: Provide a clean SDK interface for non-MCP consumers.

#### 4.1 Create `src/sdk/executor.ts`

```typescript
// src/sdk/executor.ts

import type { Logger } from 'pino';
import { createLogger } from '@/lib/logger';
import { createToolContext, type ToolContext, type ContextOptions } from '@/core/context';
import type { Tool } from '@/types/tool';
import type { Result } from '@/types';
import { loadKnowledgeBase } from '@/knowledge/loader';

export interface SDKOptions {
  /** Custom logger (defaults to silent logger) */
  logger?: Logger;
  /** Abort signal for cancellation */
  signal?: AbortSignal;
  /** Progress callback */
  onProgress?: (message: string, progress?: number, total?: number) => void;
}

// Singleton to track knowledge loading
let knowledgeLoaded = false;

/**
 * Ensure knowledge base is loaded (called once per process)
 */
function ensureKnowledgeLoaded(): void {
  if (!knowledgeLoaded) {
    loadKnowledgeBase();
    knowledgeLoaded = true;
  }
}

/**
 * Execute a tool directly without MCP orchestration
 */
export async function executeTool<TInput, TOutput>(
  tool: Tool<any, TOutput>,
  input: TInput,
  options: SDKOptions = {},
): Promise<Result<TOutput>> {
  // Ensure knowledge is loaded for tools that need it
  ensureKnowledgeLoaded();

  // Create logger (default to minimal logging)
  const logger = options.logger ?? createLogger({
    name: 'sdk',
    level: 'warn', // Quiet by default for SDK consumers
  });

  // Build context options
  const contextOptions: ContextOptions = {
    signal: options.signal,
    progress: options.onProgress
      ? async (msg, prog, total) => options.onProgress!(msg, prog, total)
      : undefined,
    // Note: SDK does not support policies in v1 - keep it simple
  };

  // Create context
  const ctx = createToolContext(logger, contextOptions);

  // Parse and validate input
  const parsedInput = tool.parse(input);

  // Execute tool handler
  return tool.handler(parsedInput, ctx);
}
```

#### 4.2 Create `src/sdk/index.ts`

```typescript
// src/sdk/index.ts
//
// SDK Entry Point - No MCP dependencies
//
// This module provides direct access to containerization tools without
// requiring the MCP protocol or server infrastructure.

import { executeTool, type SDKOptions } from './executor.js';

// Import tools directly
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import buildImageTool from '@/tools/build-image/tool';
import scanImageTool from '@/tools/scan-image/tool';

// Re-export types from tool schemas
export type { RepositoryAnalysis, ModuleInfo } from '@/tools/analyze-repo/schema';
export type { DockerfilePlan, GenerateDockerfileParams } from '@/tools/generate-dockerfile/schema';
export type { BuildImageParams, BuildImageResult } from '@/tools/build-image/tool';
export type { ScanImageParams, ScanImageResult } from '@/tools/scan-image/tool';

// Re-export Result type for consumers
export type { Result, Success, Failure } from '@/types';
export { Success, Failure } from '@/types';

// Re-export SDK options
export type { SDKOptions } from './executor.js';

// ============================================================================
// Simplified Function Exports
// ============================================================================

/**
 * Analyze a repository to detect language, framework, and dependencies
 *
 * @example
 * ```typescript
 * import { analyzeRepo } from 'containerization-assist-mcp/sdk';
 *
 * const result = await analyzeRepo({ repositoryPath: './my-app' });
 * if (result.ok) {
 *   console.log('Detected modules:', result.value.modules);
 * }
 * ```
 */
export async function analyzeRepo(
  input: { repositoryPath: string },
  options?: SDKOptions,
) {
  return executeTool(analyzeRepoTool, input, options);
}

/**
 * Generate Dockerfile recommendations for a repository
 *
 * @example
 * ```typescript
 * import { generateDockerfile } from 'containerization-assist-mcp/sdk';
 *
 * const result = await generateDockerfile({
 *   repositoryPath: './my-app',
 *   targetPlatform: 'linux/amd64',
 * });
 * if (result.ok) {
 *   console.log('Recommendations:', result.value.recommendations);
 * }
 * ```
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
 * Build a Docker image from a Dockerfile
 *
 * @example
 * ```typescript
 * import { buildImage } from 'containerization-assist-mcp/sdk';
 *
 * const result = await buildImage({
 *   path: './my-app',
 *   imageName: 'myapp:v1.0.0',
 * });
 * if (result.ok) {
 *   console.log('Built image:', result.value.imageId);
 * }
 * ```
 */
export async function buildImage(
  input: {
    path?: string;
    dockerfile?: string;
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
 * Scan a Docker image for security vulnerabilities
 *
 * @example
 * ```typescript
 * import { scanImage } from 'containerization-assist-mcp/sdk';
 *
 * const result = await scanImage({
 *   imageId: 'myapp:v1.0.0',
 *   severity: 'high',
 * });
 * if (result.ok) {
 *   console.log('Vulnerabilities:', result.value.vulnerabilities);
 * }
 * ```
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

// ============================================================================
// Advanced: Direct Tool Access
// ============================================================================

/**
 * Direct access to tool objects for advanced use cases
 * (e.g., accessing schemas, metadata, or custom execution)
 */
export const tools = {
  analyzeRepo: analyzeRepoTool,
  generateDockerfile: generateDockerfileTool,
  buildImage: buildImageTool,
  scanImage: scanImageTool,
} as const;

/**
 * Execute any tool directly with full control
 */
export { executeTool } from './executor.js';
```

#### 4.3 Create `src/sdk/types.ts`

```typescript
// src/sdk/types.ts

// Re-export all types SDK consumers might need
export type { ToolContext, ProgressReporter } from '@/core/context';
export type { Result, Success, Failure, ErrorGuidance } from '@/types';
export type { SDKOptions } from './executor.js';

// Tool-specific input types
export type { RepositoryAnalysis, ModuleInfo } from '@/tools/analyze-repo/schema';
export type {
  DockerfilePlan,
  GenerateDockerfileParams,
  BaseImageRecommendation,
  DockerfileRequirement,
} from '@/tools/generate-dockerfile/schema';
export type { BuildImageParams } from '@/tools/build-image/schema';
export type { BuildImageResult } from '@/tools/build-image/tool';
export type { ScanImageParams } from '@/tools/scan-image/schema';
export type { ScanImageResult } from '@/tools/scan-image/tool';
```

### Phase 5: Update Package Exports

**Goal**: Allow consumers to import from `containerization-assist-mcp/sdk`.

#### 5.1 Update `package.json`

```json
{
  "exports": {
    ".": {
      "import": "./dist/src/index.js",
      "require": "./dist-cjs/src/index.js",
      "types": "./dist/src/index.d.ts"
    },
    "./sdk": {
      "import": "./dist/src/sdk/index.js",
      "require": "./dist-cjs/src/sdk/index.js",
      "types": "./dist/src/sdk/index.d.ts"
    }
  }
}
```

#### 5.2 Verify No MCP Imports in SDK Path

After implementation, verify the SDK has no MCP dependencies:

```bash
# Should return no results
grep -r "@modelcontextprotocol" src/sdk/
grep -r "@/mcp" src/sdk/
grep -r "from.*mcp" src/core/
```

### Phase 6: Documentation and Testing

#### 6.1 Update CLAUDE.md

Add SDK usage section:

```markdown
## SDK Usage (Non-MCP)

For consumers who want to use tools without MCP:

\`\`\`typescript
import { analyzeRepo, buildImage, scanImage, generateDockerfile } from 'containerization-assist-mcp/sdk';

// Simple function calls - no MCP server needed
const analysis = await analyzeRepo({ repositoryPath: './myapp' });
const dockerfile = await generateDockerfile({
  repositoryPath: './myapp',
  targetPlatform: 'linux/amd64'
});
const build = await buildImage({ path: './myapp', imageName: 'myapp:v1' });
const scan = await scanImage({ imageId: 'myapp:v1' });
\`\`\`
```

#### 6.2 Add SDK Tests

Create `test/sdk/sdk.test.ts`:

```typescript
import { analyzeRepo, buildImage, tools } from '../../src/sdk';

describe('SDK', () => {
  it('exports simplified functions', () => {
    expect(typeof analyzeRepo).toBe('function');
    expect(typeof buildImage).toBe('function');
  });

  it('exports tool objects', () => {
    expect(tools.analyzeRepo.name).toBe('analyze-repo');
    expect(tools.buildImage.name).toBe('build-image');
  });

  it('analyzeRepo works without MCP', async () => {
    const result = await analyzeRepo({
      repositoryPath: process.cwd()
    });
    expect(result.ok).toBe(true);
  });
});
```

## Migration Guide for Existing Code

### For Tool Authors

No changes required - tools continue to work as before. The import path change is handled automatically.

### For MCP Consumers

No changes required - existing MCP integration continues to work identically.

### For New SDK Consumers

```typescript
// Before (required MCP knowledge)
import { createApp } from 'containerization-assist-mcp';
const app = createApp();
const result = await app.execute('analyze-repo', { repositoryPath: '.' });

// After (simple SDK)
import { analyzeRepo } from 'containerization-assist-mcp/sdk';
const result = await analyzeRepo({ repositoryPath: '.' });
```

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing imports | Low | High | Re-export from `@/mcp/context` maintains backward compatibility |
| Circular dependencies | Medium | Medium | Careful layering, core has no inward dependencies |
| Missing types for SDK consumers | Medium | Low | Comprehensive type exports in `sdk/types.ts` |
| Knowledge loading issues | Low | Medium | Singleton pattern ensures single load |

## Future Enhancements

1. **Policy Support for SDK**: Add optional policy loading to SDK executor
2. **Streaming Progress**: Add streaming support for long-running operations
3. **Separate Package**: Consider extracting to `containerization-assist-core` package
4. **Additional Tools**: Expand SDK to include more tools based on demand

## Summary

This refactoring:
- **Creates clean separation** between core tool functionality and MCP integration
- **Maintains backward compatibility** for existing MCP consumers
- **Provides simple SDK** for VS Code extension / Copilot integration
- **Eliminates code duplication** by having MCP layer build on core
- **Follows dependency inversion** - core knows nothing about MCP

Total estimated effort: **2-3 days** for implementation and testing.
