# API Refactoring Plan: Clean Migration to Idiomatic TypeScript

## Executive Summary

This document outlines a clean migration of the containerization-assist external API from Java enterprise patterns to idiomatic TypeScript patterns. This is a breaking change that eliminates unnecessary abstractions, class-based architectures, and complex state management in favor of modern TypeScript idioms.

## Current State Analysis

### Current Client Usage Pattern
```javascript
const { ContainerAssistServer, TOOL_NAMES } = require('@thgamble/containerization-assist-mcp');

const caServer = new ContainerAssistServer();
caServer.registerTools({ server }, {
  tools: [TOOL_NAMES.ANALYZE_REPO, TOOL_NAMES.BUILD_IMAGE],
  nameMapping: {
    [TOOL_NAMES.ANALYZE_REPO]: 'analyzeRepository',
    [TOOL_NAMES.BUILD_IMAGE]: 'buildImage'
  }
});
```

### Problems with Current Architecture

1. **Java Enterprise Anti-Patterns**
   - Unnecessary `IContainerAssistServer` interface abstraction
   - Class + Factory pattern (`ContainerAssistServer` + `createContainerAssistServer`)
   - Complex state object threading (`ContainerAssistServerState`)
   - Over-engineered dependency injection patterns

2. **Complex Type System**
   - Multiple server interface abstractions (`ServerRegisterTool`, `ServerAddTool`, `ServerMapTools`)
   - Runtime type guards instead of union types
   - Excessive interface layering for simple operations

3. **Non-Idiomatic TypeScript**
   - 400+ lines for simple tool registration
   - State management through parameter passing instead of closures
   - Method-based APIs instead of functional composition

## Proposed Idiomatic TypeScript Architecture

### 1. Function-First API Design

Replace class-based architecture with a single factory function:

```typescript
// New idiomatic API
export function createContainerAssist(options: {
  logger?: Logger;
} = {}) {
  // Implementation uses closures for state management
  return {
    bindToServer: (server: McpServer) => void,
    registerTools: (server: McpServer, config?: ToolConfig) => void,
    getAvailableTools: () => readonly string[],
  } as const;
}

export type ContainerAssist = ReturnType<typeof createContainerAssist>;
```

### 2. Simplified Tool Configuration

```typescript
export interface ToolConfig {
  tools?: readonly ToolName[];
  nameMapping?: Partial<Record<ToolName, string>>;
}

export const TOOLS = {
  ANALYZE_REPO: 'analyze_repo',
  BUILD_IMAGE: 'build_image',
  // ... other tools
} as const;

export type ToolName = typeof TOOLS[keyof typeof TOOLS];
```

### 3. Union Types Instead of Type Guards

```typescript
type McpServerLike =
  | { tool: (name: string, desc: string, schema: unknown, handler: Function) => void }
  | { addTool: (def: ToolDef, handler: Function) => void };

// No runtime type detection needed
function registerWithServer(server: McpServerLike, tool: Tool) {
  if ('tool' in server) {
    server.tool(tool.name, tool.description, tool.schema, tool.handler);
  } else {
    server.addTool(tool.definition, tool.handler);
  }
}
```

## Implementation Plan

### Phase 1: Core API Rewrite

**Timeline: 2-3 days**

1. **Replace Main Export File** (`src/index.ts`)
   ```typescript
   // Clean, functional exports only
   export { createContainerAssist as default } from './exports/container-assist.js';
   export { createContainerAssist, type ContainerAssist } from './exports/container-assist.js';
   export { TOOLS, type ToolName } from './exports/tools.js';
   export type { MCPTool, MCPToolResult } from './exports/types.js';
   ```

2. **Create New Factory Function** (`src/exports/container-assist.ts`)
   ```typescript
   export function createContainerAssist(options: { logger?: Logger } = {}) {
     const logger = options.logger || createLogger({ name: 'containerization-assist' });
     const sessionManager = createSessionManager(logger);
     const tools = loadAllTools();

     return {
       bindToServer: (server: McpServer) => bindAllTools(server, tools, logger),
       registerTools: (server: McpServer, config?: ToolConfig) =>
         registerSelectedTools(server, tools, config, logger),
       getAvailableTools: () => Object.keys(tools) as readonly ToolName[],
     } as const;
   }
   ```

3. **Delete Legacy Files**
   - Remove `src/exports/containerization-assist-server.ts`
   - Remove `src/exports/helpers.ts` (merge useful parts into new implementation)
   - Clean up unused interfaces and abstractions

4. **Simplify Tool Registration**
   - Replace type guards with union types
   - Direct server method calls (no abstraction layers)
   - Reduce total export code from ~400 lines to ~100 lines

### Phase 2: Update Package Metadata

**Timeline: 1 day**

1. **Bump Major Version** (breaking change)
2. **Update README** with new API examples only
3. **Create Migration Guide** in separate doc
4. **Update JSDoc** with new patterns

## New Client Usage Pattern

### Before (Java Enterprise Style)
```javascript
const { ContainerAssistServer, TOOL_NAMES } = require('@thgamble/containerization-assist-mcp');

const caServer = new ContainerAssistServer();
caServer.registerTools({ server }, {
  tools: [TOOL_NAMES.ANALYZE_REPO, TOOL_NAMES.BUILD_IMAGE],
  nameMapping: {
    [TOOL_NAMES.ANALYZE_REPO]: 'analyzeRepository',
    [TOOL_NAMES.BUILD_IMAGE]: 'buildImage'
  }
});
```

### After (Idiomatic TypeScript)
```javascript
const { createContainerAssist, TOOLS } = require('@thgamble/containerization-assist-mcp');

const containerAssist = createContainerAssist();
containerAssist.registerTools(server, {
  tools: [TOOLS.ANALYZE_REPO, TOOLS.BUILD_IMAGE],
  nameMapping: {
    [TOOLS.ANALYZE_REPO]: 'analyzeRepository',
    [TOOLS.BUILD_IMAGE]: 'buildImage'
  }
});

// Or for all tools
containerAssist.bindToServer(server);
```

### Alternative Functional Style
```javascript
const { createContainerAssist, TOOLS } = require('@thgamble/containerization-assist-mcp');

// One-liner for simple cases
createContainerAssist().bindToServer(server);

// Or with configuration
const assist = createContainerAssist({ logger: customLogger });
assist.registerTools(server, {
  tools: [TOOLS.ANALYZE_REPO, TOOLS.BUILD_IMAGE],
  nameMapping: { [TOOLS.ANALYZE_REPO]: 'analyzeRepository' }
});
```

## Benefits of New Architecture

### 1. **Reduced Complexity**
- ~150 lines instead of 400+ for core functionality
- Single factory function instead of class + interface + factory
- Direct closures instead of state object threading

### 2. **Better TypeScript Idioms**
- Functional composition over inheritance
- Union types over runtime type detection
- Immutable return types with `as const`
- Proper use of TypeScript's structural typing

### 3. **Improved Developer Experience**
- Simpler import patterns
- More discoverable API surface
- Better IDE autocomplete and type checking
- Clearer function signatures

### 4. **Maintainability**
- Fewer abstraction layers
- More predictable code flow
- Easier testing with pure functions
- Better separation of concerns

## Implementation Checklist

### Core Refactoring
- [ ] Create `src/exports/container-assist.ts` with new factory function
- [ ] Implement simplified tool registration without type guards
- [ ] Update union types for server compatibility
- [ ] Create closure-based state management
- [ ] Add comprehensive TypeScript types

### Breaking Changes
- [ ] Remove `ContainerAssistServer` class completely
- [ ] Remove `IContainerAssistServer` interface
- [ ] Remove `createContainerAssistServer` factory function
- [ ] Update `src/index.ts` to export only new API
- [ ] Change `TOOL_NAMES` constant to `TOOLS`

### Testing & Validation
- [ ] Unit tests for new API functions
- [ ] Integration tests with actual MCP servers
- [ ] Update existing tests to use new API
- [ ] Performance validation (should be better due to less overhead)

### Documentation
- [ ] Update README with new examples only
- [ ] Create migration guide document
- [ ] Update JSDoc comments
- [ ] Add TypeScript usage examples

## Risk Assessment

### Medium Risk (Breaking Changes)
- Existing client code will need updates
- API surface changes significantly
- Import paths and usage patterns change

### Mitigation Strategies
- Clear migration documentation with before/after examples
- Comprehensive test coverage for new API
- Semantic versioning with major version bump
- Simple, intuitive new API that's easy to adopt

### Client Migration Required
Your client code needs these changes:
```javascript
// Before
const { ContainerAssistServer, TOOL_NAMES } = require('@thgamble/containerization-assist-mcp');
const caServer = new ContainerAssistServer();
caServer.registerTools({ server }, { tools: [TOOL_NAMES.ANALYZE_REPO] });

// After
const { createContainerAssist, TOOLS } = require('@thgamble/containerization-assist-mcp');
const containerAssist = createContainerAssist();
containerAssist.registerTools(server, { tools: [TOOLS.ANALYZE_REPO] });
```

## Timeline

- **Week 1**: Core API implementation, delete legacy code, update tests
- **Week 2**: Documentation, migration guide, package version bump
- **Week 3**: Release new major version with clean API

This refactoring will modernize the codebase, improve developer experience, and align with TypeScript best practices through a clean break from legacy patterns.