# Codebase Simplification Implementation Plan

## Overview

This document outlines a detailed implementation plan to address over-engineering, premature abstraction, and non-idiomatic TypeScript patterns in the containerization-assist codebase, while preserving essential functionality like automatic tool dependency resolution.

## Issues Identified

### Over-Engineering
- Excessive tool router complexity (580-line file with complex dependency resolution system)
- Overly complex configuration system (71-line CONSTANTS object with deep nesting)
- Heavyweight session management with cleanup timers and TTL management

### Premature Abstraction
- Rigid tool pattern enforcement requiring co-located files for simple tools
- Result type overuse even for operations that rarely fail
- Complex AI integration patterns with multiple abstraction layers

### Non-Idiomatic TypeScript
- Inconsistent import organization (mix of relative imports and path aliases)
- Function overloading complexity in session management
- Type definitions scattered across multiple files
- Verbose error handling patterns

## Implementation Plan

### Phase 1: Configuration System Simplification

**Current Issues:**
- 71-line CONSTANTS object with excessive nesting
- Redundant configuration values (SESSION_TTL appears in multiple places)
- Over-engineered orchestrator configuration that may never be used

**Implementation Steps:**

1. **Flatten CONSTANTS object** (`src/config/app-config.ts:16-71`)
```typescript
// Replace nested CONSTANTS with flat defaults
const DEFAULT_CONFIG = {
  MCP_NAME: 'containerization-assist',
  SESSION_TTL: 86400, // seconds
  DOCKER_TIMEOUT: 60000,
  K8S_TIMEOUT: 30000,
  SCAN_TIMEOUT: 300000,
  MAX_FILE_SIZE: 10 * 1024 * 1024,
  MAX_SESSIONS: 100,
  // Remove orchestrator config unless proven necessary
} as const;
```

2. **Simplify schema definitions** (`src/config/app-config.ts:77-139`)
```typescript
const AppConfigSchema = z.object({
  server: z.object({
    nodeEnv: z.enum(['development', 'production', 'test']).default('development'),
    logLevel: z.enum(['error', 'warn', 'info', 'debug']).default('info'),
    port: z.coerce.number().default(3000),
  }),
  session: z.object({
    ttl: z.coerce.number().default(DEFAULT_CONFIG.SESSION_TTL),
    maxSessions: z.coerce.number().default(DEFAULT_CONFIG.MAX_SESSIONS),
  }),
  // Consolidate docker/k8s into single 'services' section
  services: z.object({
    dockerTimeout: z.coerce.number().default(DEFAULT_CONFIG.DOCKER_TIMEOUT),
    k8sTimeout: z.coerce.number().default(DEFAULT_CONFIG.K8S_TIMEOUT),
  })
});
```

3. **Remove unused configuration sections:**
   - `ORCHESTRATOR` object (lines 42-70) - move to tool-specific config if needed
   - `workspace.cleanupOnExit` - handle in process cleanup
   - `logging.format` - use single format

### Phase 2: Type Definitions Consolidation

**Current Issues:**
- Result type defined in multiple files (`src/types.ts`, `src/lib/result-utils.ts`, etc.)
- Interface definitions scattered across 39+ files
- Tool-specific result interfaces duplicating common patterns

**Implementation Steps:**

1. **Create central type definition file** `src/types/core.ts`
```typescript
// Consolidate all Result-related types
export type Result<T> = { ok: true; value: T } | { ok: false; error: string };
export const Success = <T>(value: T): Result<T> => ({ ok: true, value });
export const Failure = <T>(error: string): Result<T> => ({ ok: false, error });

// Common tool result patterns
export interface ToolExecutionResult {
  sessionId: string;
  executedAt: Date;
  duration: number;
}

export interface ValidationResult {
  valid: boolean;
  issues: Array<{ severity: 'error' | 'warning'; message: string }>;
}
```

2. **Eliminate duplicate type definitions:**
   - Remove Result type from `src/lib/result-utils.ts`
   - Update all imports to use `src/types/core.ts`
   - Consolidate tool-specific result interfaces into shared types

3. **Use composition and union types (idiomatic TypeScript):**
```typescript
// src/types/tools.ts - Composition over inheritance

// Core metadata that many tools share
type ToolMetadata = {
  sessionId: string;
  executedAt: Date;
  duration: number;
}

// Optional capabilities tools might have
type AnalysisCapability = {
  confidence: number;
  detectionMethod: 'signature' | 'extension' | 'ai-enhanced';
}

type BuildCapability = {
  artifacts: string[];
  size?: number;
}

type ValidationCapability = {
  issues: Array<{ severity: 'error' | 'warning'; message: string }>;
}

// Tool results using intersection types (composition)
export type AnalyzeRepoResult = Result<{
  language: string;
  framework?: string;
  // ... other fields
}> & ToolMetadata & AnalysisCapability;

export type BuildImageResult = Result<{
  imageId: string;
  tags: string[];
}> & ToolMetadata & BuildCapability;

// Union type for all possible tool results
export type AnyToolResult =
  | AnalyzeRepoResult
  | BuildImageResult
  | GenerateDockerfileResult
  | ScanResult;

// Utility types for extracting common patterns
export type WithAnalysis<T> = T & AnalysisCapability;
export type WithValidation<T> = T & ValidationCapability;
```

### Phase 3: Import Strategy Standardization

**Current Issues:**
- Mix of path aliases (`@lib/`, `@mcp/`) and relative imports (`../../`)
- Inconsistent import organization within files
- Some files use both strategies

**Implementation Decision: Standardize on Path Aliases**

**Implementation Steps:**

1. **Update tsconfig.json paths** (ensure complete coverage)
```json
{
  "compilerOptions": {
    "paths": {
      "@/*": ["src/*"],
      "@types": ["src/types"],
      "@lib/*": ["src/lib/*"],
      "@mcp/*": ["src/mcp/*"],
      "@tools/*": ["src/tools/*"],
      "@config/*": ["src/config/*"],
      "@services/*": ["src/services/*"]
    }
  }
}
```

2. **Convert all relative imports in tools/**
   - `../../types` → `@types`
   - `../../lib/` → `@lib/`
   - `../../mcp/` → `@mcp/`

3. **Establish import ordering convention:**
```typescript
// 1. Node.js built-ins
import { readFileSync } from 'fs';

// 2. External packages
import { z } from 'zod';

// 3. Internal path aliases (alphabetical)
import type { ToolContext } from '@mcp/context';
import { Success, Failure } from '@types';
import { createLogger } from '@lib/logger';

// 4. Relative imports (if any - should be rare)
import { localHelper } from './helpers';
```

4. **Create ESLint rule** to enforce import strategy:
```json
{
  "rules": {
    "import/no-relative-parent-imports": "error",
    "import/order": ["error", {
      "groups": ["builtin", "external", "internal", "relative"],
      "pathGroups": [
        { "pattern": "@/**", "group": "internal" }
      ]
    }]
  }
}
```

### Phase 4: Session Management Simplification

**Current Issues:**
- Dual interfaces (`InternalSession` + `WorkflowState`)
- Complex cleanup timer management
- Over-engineered FIFO eviction and TTL handling

**Implementation Steps:**

1. **Simplify session storage** (`src/lib/session.ts:21-35`)
```typescript
// Replace complex InternalSession with simple approach
interface Session extends WorkflowState {
  createdAt: Date;
  lastAccessedAt: Date;
}

class SimpleSessionStore {
  private sessions = new Map<string, Session>();
  private cleanupInterval: NodeJS.Timeout;

  constructor(private logger: Logger, private maxSessions = 1000, private ttlMs = 86400000) {
    // Simple cleanup every 5 minutes
    this.cleanupInterval = setInterval(() => this.cleanup(), 300000);
    this.cleanupInterval.unref();
  }

  private cleanup(): void {
    const now = Date.now();
    for (const [id, session] of this.sessions) {
      if (now - session.lastAccessedAt.getTime() > this.ttlMs) {
        this.sessions.delete(id);
      }
    }
  }
}
```

2. **Remove redundant interfaces:**
   - Eliminate `InternalSession` interface
   - Combine `SessionManager` interface methods into single class
   - Remove separate factory function pattern

3. **Simplify error handling:**
```typescript
// Replace complex error creation with simple approach
async create(id?: string): Promise<Session> {
  if (this.sessions.size >= this.maxSessions) {
    this.cleanup();
    if (this.sessions.size >= this.maxSessions) {
      throw new Error(`Maximum sessions (${this.maxSessions}) reached`);
    }
  }
  // ... simple implementation
}
```

### Phase 5: AI Abstraction Layer Reduction

**Current AI Files Analysis:**
- `src/mcp/ai/` (4 files) - AI prompt and assistance infrastructure
- `src/mcp/tool-ai-helpers.ts` - AI generation utilities
- `src/mcp/tool-ai-generation.ts` - AI content generation
- `src/mcp/sampling-cache.ts` - AI response caching

**Reduction Strategy:**

1. **Consolidate AI utilities** into single file `src/lib/ai-utils.ts`
```typescript
// Combine tool-ai-helpers.ts + tool-ai-generation.ts + parameter suggestion
export interface AIContext {
  sampling?: (request: any) => Promise<any>;
  getPrompt?: (name: string) => Promise<string>;
}

export async function generateWithAI(
  logger: Logger,
  context: AIContext,
  options: {
    promptName: string;
    promptArgs: Record<string, unknown>;
    maxTokens?: number;
  }
): Promise<Result<string>> {
  // Simplified implementation combining both helpers
}

// Keep parameter suggestion - essential for UX
export async function suggestMissingParameters(
  toolName: string,
  currentParams: Record<string, unknown>,
  schema: z.ZodSchema,
  sessionContext: Record<string, unknown>,
  aiContext: AIContext
): Promise<Result<Record<string, unknown>>> {
  // Simplified but preserved implementation
}
```

2. **Remove over-engineered features:**
   - `sampling-cache.ts` - Replace with simple Map-based cache
   - `prompt-builder.ts` - Inline template substitution
   - `default-suggestions.ts` - Move to tool-specific defaults

3. **Simplify host AI assistant** (`src/mcp/ai/host-ai-assist.ts`)
```typescript
// Keep the functionality but simplify the interface
export async function fillMissingParams(
  params: Record<string, unknown>,
  schema: z.ZodSchema,
  sessionContext: Record<string, unknown>,
  aiContext: AIContext
): Promise<Record<string, unknown>> {
  // Preserve parameter suggestion logic but reduce abstraction layers
  // This is essential UX functionality for reducing user input burden
}
```

4. **Keep essential AI integration with simplified structure:**
   - Single AI utility file with parameter suggestion preserved
   - Direct integration in tools that need it
   - Maintain AI parameter suggestion (essential for UX)
   - Remove only redundant abstraction layers

## Implementation Timeline and Priorities

### Priority 1 (High Impact, Low Risk) - Week 1-2:

1. **Import Strategy Standardization**
   - Risk: Low (automated refactoring)
   - Impact: High (consistent codebase)
   - Effort: 2-3 days
   - Files: ~50 files to update

2. **Type Definitions Consolidation**
   - Risk: Low (compile-time safety)
   - Impact: High (reduced duplication)
   - Effort: 3-4 days
   - Files: `src/types/`, tool schemas

### Priority 2 (Medium Impact, Medium Risk) - Week 3-4:

3. **Configuration System Simplification**
   - Risk: Medium (runtime behavior changes)
   - Impact: Medium (easier configuration)
   - Effort: 4-5 days
   - Files: `src/config/app-config.ts`, dependent files

4. **Session Management Simplification**
   - Risk: Medium (session behavior changes)
   - Impact: Medium (simpler maintenance)
   - Effort: 3-4 days
   - Files: `src/lib/session.ts`, tests

### Priority 3 (Variable Impact, Higher Risk) - Week 5-6:

5. **AI Abstraction Layer Reduction**
   - Risk: High (functionality changes)
   - Impact: Variable (depends on AI usage)
   - Effort: 5-7 days
   - Files: `src/mcp/ai/`, AI-related utilities

## Implementation Checklist

### Pre-Implementation:
- [ ] Run comprehensive test suite baseline
- [ ] Document current API surface area

### Phase Implementation:
- [ ] Update TypeScript configurations
- [ ] Run automated refactoring tools where possible
- [ ] Update tests incrementally
- [ ] Validate no functionality changes
- [ ] Update documentation
## Risk Mitigation

- Implement phases independently to enable rollback
- Maintain 100% test coverage throughout
- Document all breaking changes clearly

## Expected Benefits

1. **Reduced Complexity**: Simpler codebase easier to understand and maintain
2. **Better TypeScript Practices**: Consistent imports and type organization
3. **Improved Developer Experience**: Less cognitive overhead when working with code
4. **Maintainability**: Fewer abstraction layers to navigate
5. **Reduced Memory Usage**: From simplified session management

## Preservation of Essential Features

- **Tool Router Dependency Resolution**: Maintained as-is (essential functionality)
- **Result Type Pattern**: Kept but simplified and consolidated
- **MCP Protocol Compliance**: All protocol requirements preserved
- **Tool Execution Pipeline**: Core functionality unchanged

This plan focuses on simplification while preserving the essential architectural decisions that provide real value to the system.