# ADR-006: Infrastructure Layer Organization

**Date:** 2025-10-17
**Status:** Accepted
**Deciders:** Development Team
**Context:** As the MCP server grew to support Docker, Kubernetes, and security scanning operations, we needed clear organizational boundaries to maintain code quality, testability, and separation of concerns.

## Decision

We organize the codebase into three distinct layers with clear responsibilities:

1. **`@infra/*`** - Infrastructure clients with external dependencies
2. **`@lib/*`** - Pure utilities without infrastructure dependencies
3. **`@tools/*`** - MCP tools that orchestrate infrastructure and utilities

Each layer has well-defined boundaries enforced through TypeScript path aliases and import conventions.

## Rationale

### Problem

The initial codebase mixed infrastructure clients, utilities, and business logic without clear boundaries:
- Infrastructure clients (Docker, Kubernetes) were scattered across multiple directories
- Utilities had hidden dependencies on infrastructure, making testing difficult
- Tools directly instantiated external dependencies, making them hard to mock
- No clear pattern for where to place new code

### Solution

**Infrastructure Layer (`@infra/*`):**
- Contains all code that interacts with external systems
- Examples: Docker client, Kubernetes client, security scanners, registry operations
- Located in `src/infra/`
- Subdirectories:
  - `docker/` - Docker client, registry, socket validation, error handling
  - `kubernetes/` - K8s client, idempotent apply, kubeconfig discovery
  - `security/` - Security scanning clients (Trivy, Grype)
  - `health/` - Health check clients for external services

**Utility Layer (`@lib/*`):**
- Contains pure functions without external dependencies
- Examples: error handling, logging, validation, file utilities
- Located in `src/lib/`
- Must NOT import from `@infra/*`
- Can be safely tested without mocking infrastructure
- Key files:
  - `error-guidance.ts` - Enhanced error messages with actionable guidance
  - `logger.ts` - Logging utilities
  - `file-utils.ts` - File system helpers
  - `validation.ts` - Input validation
  - `tool-helpers.ts` - Tool execution helpers

**Tool Layer (`@tools/*`):**
- Contains MCP tool implementations
- Orchestrates infrastructure and utilities to implement business logic
- Located in `src/tools/`
- Each tool is a self-contained directory with:
  - `tool.ts` - Implementation using `Tool<TSchema, TOut>` interface
  - `schema.ts` - Zod schema for input validation
  - `index.ts` - Public exports
- Tools import from both `@infra/*` and `@lib/*`

### Import Rules

```typescript
// ✅ Tools can import from both layers
import { createDockerClient } from '@infra/docker/client';
import { validatePathOrFail } from '@lib/validation-helpers';

// ✅ Infrastructure can import utilities
import { extractErrorMessage } from '@lib/errors';

// ❌ Utilities MUST NOT import infrastructure
import { createDockerClient } from '@infra/docker/client'; // Not allowed in @lib

// ✅ Use path aliases, not relative imports
import { createDockerClient } from '@infra/docker/client';

// ❌ Don't use relative imports for cross-layer access
import { createDockerClient } from '../../../infra/docker/client';
```

## Consequences

### Positive

1. **Clear Separation of Concerns**
   - Each layer has a single, well-defined responsibility
   - Easy to determine where new code belongs
   - Reduced cognitive load when navigating codebase

2. **Improved Testability**
   - Utilities in `@lib` can be tested without mocking infrastructure
   - Infrastructure clients in `@infra` are isolated and can be mocked
   - Tools in `@tools` can mock both infrastructure and utilities independently

3. **Better Maintainability**
   - Changes to infrastructure clients don't affect utilities
   - Utilities remain pure and predictable
   - Tools are decoupled from infrastructure implementation details

4. **Enhanced Code Reusability**
   - Utilities in `@lib` are infrastructure-agnostic and reusable
   - Infrastructure clients are tool-agnostic and reusable
   - Clean interfaces enable composition

5. **Simplified Dependency Management**
   - Heavy external dependencies (dockerode, @kubernetes/client-node) isolated in `@infra`
   - Utilities remain lightweight with minimal dependencies
   - Easier to tree-shake and bundle

6. **Improved Developer Experience**
   - TypeScript path aliases provide clear import semantics
   - IDE autocomplete works better with explicit paths
   - Circular dependency issues eliminated

### Negative

1. **More Boilerplate**
   - Need to create separate directories and files
   - Path aliases require tsconfig.json maintenance
   - Build system needs to resolve aliases (tsc-alias)

2. **Learning Curve**
   - New contributors need to understand the three-layer architecture
   - Must learn import rules and path alias conventions
   - Requires discipline to maintain boundaries

3. **Migration Effort**
   - Existing code needed refactoring to fit new structure
   - Some code had to be split across layers
   - Import statements across codebase needed updates

4. **Indirect Dependencies**
   - Tools must explicitly import both infrastructure and utilities
   - More import statements per file
   - Cannot create convenience re-exports that cross layers

## Alternatives Considered

### Alternative 1: Flat Structure

**Approach:** Keep all code in `src/` without subdirectories.

```
src/
├── docker-client.ts
├── kubernetes-client.ts
├── build-image-tool.ts
├── logger.ts
└── validation.ts
```

**Pros:**
- Simple file structure
- No path aliases needed
- Easy to find files by name

**Cons:**
- Difficult to distinguish infrastructure from utilities
- 60+ files in a single directory becomes unwieldy
- No enforced boundaries between layers
- Harder to understand dependencies and relationships

**Rejected because:** Does not scale beyond small codebases. With 17 tools, multiple infrastructure clients, and dozens of utilities, a flat structure would create confusion and make dependencies unclear.

### Alternative 2: Monolithic Organization

**Approach:** Group all related code together without layer separation.

```
src/
├── docker/
│   ├── client.ts
│   ├── build-tool.ts
│   ├── scan-tool.ts
│   └── utils.ts
└── kubernetes/
    ├── client.ts
        └── utils.ts
```

**Pros:**
- Related code is physically close
- Easy to find all Docker-related code
- Natural grouping by technology

**Cons:**
- Tools are scattered across technology boundaries
- Shared utilities duplicated in each directory
- Cross-cutting concerns (logging, validation) have no clear home
- MCP tool discovery requires searching multiple directories

**Rejected because:** This project is a unified MCP server for containerization, not separate Docker and Kubernetes libraries. Tools often use both Docker and Kubernetes, so organizing by technology creates artificial boundaries that don't match the problem domain.

### Alternative 3: Feature-Based Organization

**Approach:** Group code by user-facing features.

```
src/
├── image-building/
│   ├── docker-client.ts
│   ├── generate-dockerfile.ts
│   ├── build-image.ts
│   └── scan-image.ts
└── deployment/
    ├── kubernetes-client.ts
    ├── generate-manifests.ts
    └── verify.ts
```

**Pros:**
- Aligns with user workflows
- All feature code in one place
- Easy to understand feature boundaries

**Cons:**
- Shared infrastructure (Docker client) duplicated across features
- Utilities scattered across feature directories
- Features share infrastructure but are organizationally separated
- Difficult to share code between features

**Rejected because:** The infrastructure clients (Docker, Kubernetes) are used across multiple features. This would lead to code duplication or complex dependency relationships between feature directories. The layered approach provides better code reuse.

### Alternative 4: Hexagonal/Ports and Adapters

**Approach:** Use hexagonal architecture with ports (interfaces) and adapters (implementations).

```
src/
├── domain/          # Core business logic
├── ports/           # Interface definitions
├── adapters/
│   ├── docker/      # Docker adapter
│   ├── kubernetes/  # K8s adapter
│   └── mcp/         # MCP adapter
└── tools/           # Tools as application services
```

**Pros:**
- Highly testable with clear adapter boundaries
- Core domain logic isolated from infrastructure
- Easy to swap implementations

**Cons:**
- Significant over-engineering for this use case
- Too much abstraction for a project with stable infrastructure (Docker, K8s)
- More complex for contributors to understand
- Adds interface boilerplate with little benefit

**Rejected because:** While hexagonal architecture works well for complex domains with multiple adapters, this project has stable infrastructure dependencies (Docker and Kubernetes are not expected to change). The three-layer architecture provides sufficient testability without excessive abstraction.

## Implementation Details

### Directory Structure

```
src/
├── infra/                    # Infrastructure clients
│   ├── docker/
│   │   ├── client.ts        # Docker client factory
│   │   ├── registry.ts      # Registry operations
│   │   ├── errors.ts        # Docker error handling
│   │   └── socket-validation.ts  # Socket detection
│   ├── kubernetes/
│   │   ├── client.ts        # K8s client factory
│   │   ├── idempotent-apply.ts   # Safe apply logic
│   │   ├── kubeconfig-discovery.ts
│   │   └── resource-operations.ts
│   ├── security/
│   │   └── scanner.ts       # Security scanning clients
│   └── health/
│       └── checks.ts        # Health check clients
├── lib/                     # Pure utilities
│   ├── error-guidance.ts    # Enhanced error messages
│   ├── errors.ts            # Error extraction
│   ├── logger.ts            # Logging utilities
│   ├── file-utils.ts        # File operations
│   ├── validation.ts        # Input validation
│   ├── validation-helpers.ts
│   ├── tool-helpers.ts      # Tool execution helpers
│   └── platform.ts          # Platform detection
├── tools/                   # MCP tools
│   ├── build-image/
│   │   ├── tool.ts          # Uses @infra/docker + @lib/*
│   │   ├── schema.ts
│   │   └── index.ts
│   └── ...
├── config/                  # Configuration (policies, environment)
├── mcp/                     # MCP server adapter
└── types/                   # Shared TypeScript types
```

### Path Alias Configuration

**tsconfig.json:**
```json
{
  "compilerOptions": {
    "paths": {
      "@/*": ["src/*"],
      "@infra/*": ["src/infra/*"],
      "@lib/*": ["src/lib/*"],
      "@tools/*": ["src/tools/*"],
      "@config/*": ["src/config/*"],
      "@mcp/*": ["src/mcp/*"],
      "@types": ["src/types"]
    }
  }
}
```

### Example: Build Image Tool

**src/tools/build-image/tool.ts:**
```typescript
// Infrastructure imports
import { createDockerClient, type DockerBuildOptions } from '@infra/docker/client';

// Utility imports
import { normalizePath } from '@lib/platform';
import { setupToolContext } from '@lib/tool-context-helpers';
import { validatePathOrFail } from '@lib/validation-helpers';
import { extractErrorMessage } from '@lib/errors';

// Type imports
import type { ToolContext } from '@mcp/context';
import { type Result, Success, Failure } from '@types';

// Local imports
import { type BuildImageParams, buildImageSchema } from './schema';

async function run(
  input: BuildImageParams,
  ctx: ToolContext
): Promise<Result<BuildImageResult>> {
  // 1. Use @lib utilities for validation
  const pathValidation = await validatePathOrFail(input.path);
  if (!pathValidation.ok) return pathValidation;

  // 2. Use @infra for infrastructure operations
  const dockerResult = await createDockerClient();
  if (!dockerResult.ok) return dockerResult;

  const docker = dockerResult.value;

  // 3. Orchestrate infrastructure and utilities
  const buildResult = await docker.buildImage({ ... });

  return Success({ ... });
}
```

### Enforcement

The architecture is enforced through:

1. **Import Rules in CLAUDE.md:**
   - Documented path alias conventions
   - Explicit prohibition of @lib importing @infra
   - ESLint import ordering rules

2. **Code Review Guidelines:**
   - Reviewers check imports follow layer boundaries
   - New code placed in appropriate layer
   - Utilities remain pure

3. **Automated Linting:**
   - ESLint enforces import ordering
   - TypeScript strict mode prevents circular dependencies
   - Build fails if path aliases are incorrect

## Migration Path

1. **Phase 1: Create Layer Structure** (Completed)
   - Created `src/infra/` and moved infrastructure clients
   - Ensured `src/lib/` contains only pure utilities
   - Added path aliases to tsconfig.json

2. **Phase 2: Update Imports** (Completed)
   - Replaced relative imports with path aliases
   - Updated all tools to import from @infra and @lib
   - Removed re-exports that crossed layers

3. **Phase 3: Documentation** (In Progress)
   - Document architecture in CLAUDE.md
   - Create this ADR
   - Add code comments explaining layer boundaries

## References

- [CLAUDE.md](../../CLAUDE.md) - Path alias conventions and import guidelines
- [src/infra/docker/client.ts](../../src/infra/docker/client.ts) - Example infrastructure client
- [src/lib/validation-helpers.ts](../../src/lib/validation-helpers.ts) - Example pure utility
- [src/tools/build-image/tool.ts](../../src/tools/build-image/tool.ts) - Example tool orchestration
- [ADR-002: Unified Tool Interface](./002-tool-interface.md) - Related decision on tool structure

## Related Decisions

- **ADR-002: Unified Tool Interface** - Defines how tools are structured within `@tools/*`
- **ADR-001: Result<T> Error Handling** - Error handling pattern used across all layers
- **ADR-005: MCP Integration** - MCP server uses the infrastructure and tool layers
