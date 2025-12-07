# ADR-007: SDK Decoupling from MCP

**Date:** 2025-12-07
**Status:** Proposed
**Deciders:** Development Team
**Context:** A consumer requested the ability to use containerization tools as simple function calls in their VS Code extension for Copilot integration, without requiring MCP server infrastructure or pulling in `@modelcontextprotocol/sdk` as a dependency.

## Decision

We will create a **standalone SDK entry point** (`/sdk`) that exposes all 11 tools as simple async functions without any MCP dependencies. This requires:

1. **Core Layer Extraction**: Move `ToolContext` interface and `createToolContext` factory from `src/mcp/context.ts` to a new `src/core/context.ts` with zero MCP imports
2. **MCP Layer as Wrapper**: Refactor `src/mcp/context.ts` to re-export from core and add MCP-specific functionality (progress notifications via MCP protocol)
3. **SDK Entry Point**: Create `src/sdk/index.ts` with simplified function exports for all 11 tools
4. **Dependency Inversion**: Tools import `ToolContext` from core, not MCP; MCP layer builds on top of core

The architecture follows this layering:

```
┌─────────────────────────────────────────────┐
│  SDK Consumer          MCP Consumer         │
│  (VS Code Ext)         (Claude Desktop)     │
│       │                      │              │
│       ▼                      ▼              │
│   sdk/index.ts          app/index.ts        │
│       │                      │              │
│       │               mcp/context.ts        │
│       │                (re-exports +        │
│       │                 MCP helpers)        │
│       │                      │              │
│       └──────────┬───────────┘              │
│                  ▼                          │
│           core/context.ts                   │
│           (ToolContext)                     │
│                  ▲                          │
│                  │                          │
│           tools/*/tool.ts                   │
│                                             │
│      NO MCP DEPENDENCIES IN CORE            │
└─────────────────────────────────────────────┘
```

## Rationale

### Why Decouple from MCP?

1. **Consumer Demand**
   - VS Code extension developers integrating with GitHub Copilot want direct function calls
   - They have their own extension and don't want MCP server overhead
   - Pulling in `@modelcontextprotocol/sdk` adds unnecessary dependencies to their bundle

2. **Architectural Purity**
   - Tools themselves don't use MCP functionality - they just need a context with logger, signal, and progress callback
   - The current coupling is accidental, not essential
   - `ToolContext` being in `src/mcp/` is a naming/organizational issue, not a real dependency

3. **Flexibility**
   - SDK consumers can use tools in environments where MCP isn't available
   - Same tools, same behavior, different execution context
   - Opens up programmatic usage in Node.js applications, CI/CD pipelines, etc.

4. **Bundle Size**
   - SDK consumers avoid `@modelcontextprotocol/sdk` and its transitive dependencies
   - Lighter-weight integration for simple use cases

### Why Keep MCP as Primary Interface?

This decision complements, not replaces, ADR-005 (MCP Protocol Integration):

- **MCP remains the primary interface** for AI agent interaction (Claude Desktop, VS Code via MCP)
- **SDK is an alternative path** for direct programmatic access
- **Same tools, same behavior** - just different execution contexts
- **No feature regression** - MCP users get chain hints, policy enforcement, progress notifications as before

### Key Design Principles

1. **Single Source of Truth**: `ToolContext` interface defined once in `src/core/context.ts`
2. **No Code Duplication**: Tool handlers unchanged; only import paths shift
3. **Backward Compatible**: Existing MCP integration continues to work identically
4. **Clean Imports**: SDK consumers see no "mcp" in their import paths

## Consequences

### Positive

1. **New Consumer Segment**
   - VS Code extension developers can integrate directly
   - Copilot tool registration without MCP server
   - Programmatic access for scripts and automation

2. **Cleaner Architecture**
   - True dependency inversion - core doesn't depend on MCP
   - MCP becomes an optional integration layer, not a requirement
   - Easier to add other integration layers (REST, gRPC) in future

3. **Smaller Bundles for SDK Users**
   - No `@modelcontextprotocol/sdk` dependency for SDK consumers
   - Only pull in what you need

4. **Simpler Mental Model**
   - SDK: `import { buildImage } from 'pkg/sdk'; await buildImage({...})`
   - MCP: Full server with protocol, transport, notifications
   - Clear separation of concerns

5. **Testing Improvements**
   - Tools can be tested without MCP context
   - Core context is simpler to mock
   - Faster, more isolated unit tests

### Negative

1. **SDK Lacks MCP Features**
   - No chain hints (workflow guidance) in SDK path
   - No policy enforcement in SDK v1 (can be added later)
   - No MCP progress notifications (but has callback-based progress)

2. **Two Entry Points to Maintain**
   - `src/index.ts` (MCP) and `src/sdk/index.ts` (SDK)
   - Must ensure feature parity where appropriate
   - Documentation needs to cover both paths

3. **Migration Effort**
   - 15 files need import path updates
   - New `src/core/` directory to create and maintain
   - Additional package.json exports configuration

4. **Potential for Divergence**
   - SDK and MCP paths could drift apart over time
   - Need discipline to keep them synchronized
   - Testing must cover both paths

## Alternatives Considered

### Alternative 1: Document Current Usage

**Approach**: Document that consumers can use tools directly via `createToolContext` without starting MCP server.

**Pros**:
- Zero code changes
- Immediate solution
- No new entry points to maintain

**Cons**:
- Consumers still pull in `@modelcontextprotocol/sdk` transitively
- `ToolContext` import path contains "mcp" (confusing)
- Requires understanding of internal architecture
- Not a clean public API

**Rejected because**: Doesn't solve the core problem of MCP dependency in consumer bundles.

### Alternative 2: Separate Package

**Approach**: Extract core tools into `containerization-assist-core` package, have MCP package depend on it.

**Pros**:
- Complete separation at package level
- Clear dependency boundaries
- Independent versioning

**Cons**:
- Monorepo or multi-repo complexity
- Publishing and versioning overhead
- More complex CI/CD
- Overkill for current needs

**Rejected because**: Over-engineered for the use case. Subpath exports (`/sdk`) achieve the same goal with less complexity.

### Alternative 3: Conditional Exports

**Approach**: Use package.json conditional exports to provide MCP-free bundle.

**Pros**:
- No source code changes
- Build-time tree-shaking

**Cons**:
- MCP types still referenced in source
- Complex build configuration
- Doesn't address architectural coupling
- Hard to maintain and debug

**Rejected because**: Treats symptoms, not cause. Architectural coupling remains.

### Alternative 4: Runtime MCP Detection

**Approach**: Lazy-load MCP dependencies only when needed.

**Pros**:
- Single entry point
- No new directories or files
- Automatic optimization

**Cons**:
- Complex dynamic imports
- Type safety challenges
- Doesn't remove MCP from tool imports
- Runtime overhead

**Rejected because**: Adds complexity without addressing the architectural coupling.

## Implementation

The implementation is split into 6 PRs (each under 2k lines):

| PR | Title | Scope |
|----|-------|-------|
| 1 | Create core context layer | New `src/core/` with ToolContext |
| 2 | Refactor MCP context | Re-export from core + MCP helpers |
| 3 | Update tool imports | Change 14 files from `@/mcp` to `@/core` |
| 4 | Create SDK entry point | `src/sdk/` with all 11 tool functions |
| 5 | Package exports + docs | package.json exports, CLAUDE.md |
| 6 | Add SDK tests | Test coverage for SDK path |

See `docs/implementation-plans/sdk-decoupling-from-mcp-detailed.md` for step-by-step instructions.

## SDK API Design

```typescript
// Consumer usage
import {
  analyzeRepo,
  generateDockerfile,
  buildImage,
  scanImage,
  // ... all 11 tools
} from 'containerization-assist-mcp/sdk';

// Simple function calls
const analysis = await analyzeRepo({ repositoryPath: './myapp' });
const build = await buildImage({ path: './myapp', imageName: 'myapp:v1' });

// With options
const result = await scanImage(
  { imageId: 'myapp:v1' },
  {
    signal: abortController.signal,
    onProgress: (msg, progress, total) => console.log(msg),
  }
);

// Advanced: direct tool access
import { tools, executeTool } from 'containerization-assist-mcp/sdk';
const result = await executeTool(tools.buildImage, params, options);
```

## Related Decisions

- **ADR-002: Unified Tool Interface** - Tools follow unified interface, enabling both MCP and SDK execution
- **ADR-005: MCP Protocol Integration** - MCP remains primary AI agent interface; SDK is complementary
- **ADR-006: Infrastructure Organization** - SDK adds new layer without changing infra organization

## References

- Consumer Request: VS Code Copilot agent tool integration
- Current MCP Implementation: `src/mcp/mcp-server.ts`
- Tool Interface: `src/types/tool.ts`
