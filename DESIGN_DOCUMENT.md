# Containerization Assist MCP Server - Design Document

## Project Overview

**Containerization Assist MCP Server** is a comprehensive TypeScript-based MCP (Model Context Protocol) server designed for AI-powered containerization workflows. It provides intelligent Docker and Kubernetes support through a clean, modular architecture that emphasizes reliability, extensibility, and maintainability.

### Key Features
- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications  
- 🤖 **AI-Powered**: Intelligent Dockerfile generation and optimization
- 🔄 **Workflow Orchestration**: Complete containerization pipelines
- 📊 **Progress Tracking**: Real-time progress updates via MCP
- 🔒 **Security Scanning**: Built-in vulnerability scanning with Trivy

---

## Architecture Overview

### High-Level System Design

```
┌─────────────────────────────────────────┐
│            MCP Client (Claude)          │
└─────────────────┬───────────────────────┘
                  │ MCP Protocol
┌─────────────────▼───────────────────────┐
│          MCP Server Layer               │
│  ┌─────────────────────────────────┐    │
│  │     Tool Registry & Router      │    │
│  └─────────────────────────────────┘    │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         Application Layer               │
│  ┌──────────┐ ┌──────────┐ ┌────────┐  │
│  │  Tools   │ │Workflow  │ │Session │  │
│  └──────────┘ └──────────┘ └────────┘  │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         Infrastructure Layer            │
│  ┌──────┐ ┌──────┐ ┌─────┐ ┌────────┐  │
│  │Docker│ │ K8s  │ │ AI  │ │Session │  │
│  └──────┘ └──────┘ └─────┘ └────────┘  │
└─────────────────────────────────────────┘
```

### Architectural Principles

1. **Clean Architecture**: Clear separation between domain logic, application services, and infrastructure
2. **Result-Based Error Handling**: Consistent `Result<T>` pattern throughout the codebase
3. **Dependency Injection**: Centralized container for managing dependencies
4. **Path Aliases**: TypeScript path mapping for clean imports (@/app, @/mcp, @/tools, etc.)
5. **Tool Co-location**: Each tool has its own directory with schema, implementation, and exports

---

## Source Code Structure (`src/`)

### 📁 Root Level - Application Entry Point
**Purpose**: Main application interfaces and exports.

**Key Files**:
- `index.ts`: Main application exports and public API

**Responsibilities**:
- Public API definition
- Module exports coordination

### 📁 `/ai` - AI and Prompt Engine
**Purpose**: Complete prompt engine and AI integration with deterministic sampling.

**Key Files**:
- `prompt-engine.ts`: Core prompt building and message handling
- `prompt-templates.ts`: Template definitions
- `quality.ts`: Quality scoring for AI outputs

**Responsibilities**:
- AI prompt generation with knowledge pack integration
- Deterministic single-candidate sampling with quality scoring
- Message building for AI interactions
- Knowledge enhancement of prompts

### 📁 `/app` - Application Core
**Purpose**: Core application logic and orchestration.

**Key Files**:
- `index.ts`: Main application factory and entry point
- `orchestrator.ts`: Tool execution orchestration and routing
- `orchestrator-types.ts`: Orchestrator type definitions

**Responsibilities**:
- Application startup and configuration
- Tool execution coordination
- Policy enforcement and validation
- Session management integration

### 📁 `/cli` - Command Line Interface
**Purpose**: CLI entry points and server management.

**Key Files**:
- `cli.ts`: Main CLI entry point and argument parsing
- `server.ts`: MCP server startup and management

**Responsibilities**:
- Command-line interface
- Server lifecycle management
- Environment validation

### 📁 `/config` - Configuration Management
**Purpose**: Modular policy system and environment configuration.

**Key Files**:
- `environment.ts`: Unified environment configuration
- `policy-constraints.ts`: Policy constraint extraction
- `policy-eval.ts`: Rule evaluation and application logic
- `policy-io.ts`: Load, validate, migrate, and cache operations
- `policy-prompt.ts`: AI prompt constraint integration
- `policy-schemas.ts`: Zod schemas and TypeScript types

**Responsibilities**:
- Environment variable management
- Policy system with 5 specialized modules
- Type-safe configuration interfaces

### 📁 `/infra` - Infrastructure Clients
**Purpose**: External system clients and infrastructure services.

**Subdirectories**:

#### `/docker`
- Docker client implementations
- Registry and image management

#### `/kubernetes`
- Kubernetes API clients
- Idempotent apply operations

**Responsibilities**:
- External system integration
- Infrastructure service abstractions
- Connection and client management

### 📁 `/knowledge` - Knowledge Management
**Purpose**: Knowledge base and matching system that enhances AI prompts.

**Key Files**:
- `matcher.ts`: Knowledge matching logic
- `loader.ts`: Knowledge pack loading and validation

**Related Assets**:
- `knowledge/packs/`: Static JSON knowledge data (outside src/)

**Responsibilities**:
- Knowledge base integration for AI prompts
- Content matching and retrieval by tool and context
- Knowledge pack loading and validation
- Budget-aware knowledge selection for prompt enhancement

### 📁 `/lib` - Pure Utilities
**Purpose**: Reusable utilities with no infrastructure dependencies.

**Key Files**:
- `docker.ts`: Docker utility functions
- `file-utils.ts`: File system utilities
- `regex-patterns.ts`: Common regex patterns

**Responsibilities**:
- Pure utility functions
- Helper libraries
- Common patterns and utilities

### 📁 `/mcp` - MCP Server Implementation
**Purpose**: Model Context Protocol server and adapters.

**Subdirectories**:

#### `/ai`
- `knowledge-enhancement.ts`: Knowledge pack integration for MCP tools
- `quality.ts`: Quality scoring and validation

**Key Files**:
- `mcp-server.ts`: MCP protocol server implementation
- `context.ts`: Tool execution context management

**Responsibilities**:
- MCP protocol implementation
- Tool registration and routing
- MCP-specific AI integration with knowledge enhancement
- Context propagation for tool execution

### 📁 `/session` - Session Management
**Purpose**: Unified session state management for single-operator workflows.

**Key Files**:
- `core.ts`: Session state management and persistence

**Responsibilities**:
- Single active session lifecycle management
- Persistent state across tool executions within a workflow
- Session state cleared on server shutdown
- Tool result storage and retrieval

### 📁 `/tools` - Tool Implementations
**Purpose**: Individual MCP tool implementations using co-located pattern.

**Structure**: Each tool follows the same pattern:
```
/tool-name/
├── tool.ts     # Tool implementation
├── schema.ts   # Zod schema definition
└── index.ts    # Public exports
```

**Available Tools**:
- `analyze-repo`: Repository analysis and framework detection
- `build-image`: Docker image building with progress
- `convert-aca-to-k8s`: Convert ACA to Kubernetes
- `deploy`: Deploy applications to Kubernetes
- `fix-dockerfile`: Fix and optimize existing Dockerfiles
- `generate-aca-manifests`: Azure Container Apps manifests
- `generate-dockerfile`: AI-powered Dockerfile generation
- `generate-helm-charts`: Generate Helm charts
- `generate-k8s-manifests`: Kubernetes manifest generation
- `generate-kustomize`: Generate Kustomize configurations
- `inspect-session`: Session debugging
- `ops`: Operational utilities
- `prepare-cluster`: Kubernetes cluster preparation
- `push-image`: Push images to registry
- `resolve-base-images`: Base image recommendations
- `scan`: Security vulnerability scanning
- `tag-image`: Docker image tagging
- `verify-deployment`: Verify deployment status

**Shared Resources**:
- `shared/`: Common tool utilities and patterns

**Responsibilities**:
- Individual tool logic and implementation
- Parameter validation using Zod schemas
- Result-based error handling

### 📁 `/types` - Type Definitions
**Purpose**: Centralized type definitions and interfaces.

**Key Files**:
- `index.ts`: Core type definitions including Result<T> and Tool interfaces

**Responsibilities**:
- Result<T> type system for error handling
- Tool and application interfaces
- Domain model definitions

### 📁 `/validation` - Validation and Fixing
**Purpose**: Dockerfile and Kubernetes validation and repair.

**Key Files**:
- `dockerfile-fixer.ts`: Dockerfile fixing and optimization
- `dockerfile-validator.ts`: Dockerfile validation
- `dockerfilelint-adapter.ts`: Integration with dockerfilelint
- `k8s-normalizer.ts`: Kubernetes manifest normalization
- `k8s-schema-validator.ts`: Kubernetes schema validation
- `merge-reports.ts`: Report merging utilities

**Responsibilities**:
- Validation logic for containers and manifests
- Automated fixing and optimization
- Report generation and merging

---

## Key Design Patterns

### 1. Result-Based Error Handling
All operations that can fail return a `Result<T>` type:

```typescript
export type Result<T> = { ok: true; value: T } | { ok: false; error: string };

// Usage
const result = await buildImage(config);
if (result.ok) {
  console.log('Image built:', result.value.imageId);
} else {
  console.error('Build failed:', result.error);
}
```

### 2. Tool Co-location Pattern
Each tool is self-contained with its own directory:

```typescript
// src/tools/build-image/
├── tool.ts     # Implementation
├── schema.ts   # Zod validation schema  
└── index.ts    # Public exports
```

### 3. Application Factory Pattern
Application initialization via factory function in `/app/index.ts`:

```typescript
export async function createApp(options: AppOptions) {
  // Initialize infrastructure clients
  // Load and validate policies
  // Create orchestrator with dependencies
  // Return configured application
}
```

### 4. Path Aliases for Clean Imports
TypeScript path mapping supports clean imports:

```typescript
// ✅ Path aliases (from tsconfig.json)
import { Config } from '@/config/types';
import { Logger } from '@/lib/logger';
import type { Result } from '@types';
import { analyzeRepo } from '@/tools/analyze-repo/tool';

// Available Path Aliases:
// @/*           → src/*
// @/ai/*        → src/ai/*
// @/mcp/*       → src/mcp/*
// @/tools/*     → src/tools/*
// @/lib/*       → src/lib/*
// @/infra/*     → src/infra/*
// @/session/*   → src/session/*
// @/config/*    → src/config/*
// @/resources/* → src/resources/*
// @/exports/*   → src/exports/*
// @/knowledge/* → src/knowledge/*
// @types        → src/types/index.ts
// @/container   → src/container
// @validation/* → src/validation

// ✅ Relative imports (also acceptable for local files)
import { Config } from '../config/types';
```

---

## Development Workflow

### Build System
- **Primary**: TypeScript compiler (`tsc`) with `tsc-alias` for path resolution
- **Target**: ES2022 with native ESM modules
- **Output**: `dist/` and `dist-cjs/` directories with TypeScript declarations

### Code Quality
- **TypeScript**: Strict mode with comprehensive type checking
- **ESLint**: ~700 warnings (baseline enforced, 46% reduction achieved)
- **Prettier**: Automatic code formatting
- **Quality Gates**: Automated lint ratcheting prevents regression

### Testing Strategy
- **Unit Tests**: Jest with ES module support
- **Integration Tests**: Docker and Kubernetes integration testing
- **MCP Tests**: Custom MCP inspector for protocol testing
- **Coverage**: >70% target with comprehensive tool testing

### Key Scripts
```bash
npm run build        # Clean build (ESM + CJS)
npm run build:esm    # Build ESM bundle only
npm run build:cjs    # Build CJS bundle only
npm run lint:fix     # Auto-fix linting issues
npm run test:unit    # Unit tests
npm run quality:gates # Comprehensive quality analysis
```

---

## Technology Stack

### Core Dependencies
- **@modelcontextprotocol/sdk**: MCP protocol implementation
- **dockerode**: Docker API client
- **@kubernetes/client-node**: Kubernetes API client
- **commander**: CLI argument parsing
- **pino**: Structured logging
- **zod**: Runtime type validation
- **execa**: Process execution
- **js-yaml**: YAML parsing for Kubernetes manifests

### Development Tools
- **TypeScript 5.3+**: Static typing and modern language features
- **tsc + tsc-alias**: TypeScript compiler with path alias resolution
- **Jest**: Testing framework with ES module support
- **ESLint**: Code linting with TypeScript support
- **Prettier**: Code formatting

---

## Configuration and Environment

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `DOCKER_SOCKET` | Docker daemon socket path | `/var/run/docker.sock` |
| `KUBECONFIG` | Kubernetes config path | `~/.kube/config` |
| `LOG_LEVEL` | Logging level | `info` |
| `SESSION_DIR` | Session storage directory | `~/.containerization-assist/sessions` |
| `K8S_NAMESPACE` | Default Kubernetes namespace | `default` |

### Configuration Architecture
The configuration system is centralized in `/config` with a modular policy system:

```typescript
// Environment configuration via src/config/environment.ts
export const environment = {
  docker: { socketPath: '/var/run/docker.sock' },
  kubernetes: { namespace: 'default' },
  logging: { level: 'info' },
  // ... other environment settings
};

// Policy system with 5 specialized modules:
// - policy-schemas.ts: Type definitions and Zod schemas
// - policy-io.ts: Load, validate, and cache operations
// - policy-eval.ts: Rule evaluation and application
// - policy-prompt.ts: AI prompt constraint integration
// - policy-constraints.ts: Data-driven constraint extraction
```

---

## Security and Best Practices

### Security Features
- **Vulnerability Scanning**: Built-in Trivy integration
- **Input Validation**: Zod schemas for all tool parameters
- **Resource Limits**: Configurable timeouts and size limits
- **Secure Defaults**: Conservative security settings

### Best Practices
- **No Secret Logging**: Structured logging avoids exposing sensitive data
- **Result-Based Errors**: No thrown exceptions, all errors handled explicitly
- **Immutable Configuration**: Configuration objects are read-only
- **Dependency Injection**: Testable architecture with clean separation

---

## Extension Points

### Adding New Tools
1. Create directory in `src/tools/new-tool/`
2. Implement `tool.ts` with unified Tool interface:
   ```typescript
   const tool: Tool<typeof schema, ResultType> = {
     name: 'new-tool',
     description: 'Tool description',
     version: '2.0.0',
     schema: newToolSchema,
     run: async (input, ctx) => { /* implementation */ }
   };
   export default tool;
   ```
3. Define `schema.ts` with Zod validation
4. Export via `index.ts`
5. Register in tool index

### Adding New AI Prompts
1. Add prompt templates in `src/ai/prompt-templates.ts`
2. Use via prompt engine: `buildMessages()` function
3. Optionally enhance with knowledge packs via `enhancePrompt()`
4. Apply policy constraints via `applyPolicyConstraints()`

### Infrastructure Extensions
1. Add new clients in `src/infra/`
2. Follow Result<T> pattern for error handling
3. Export via index files
4. Integrate with application factory in `src/app/index.ts`

### Policy System Extensions
1. Extend schemas in `src/config/policy-schemas.ts`
2. Add evaluation logic in `src/config/policy-eval.ts`
3. Update constraint extraction in `src/config/policy-constraints.ts`
4. Integrate with prompts via `src/config/policy-prompt.ts`

---

## Performance Considerations

### Build Performance
- **TypeScript Compilation**: Standard `tsc` compiler with `tsc-alias` for path resolution
- **Parallel Builds**: Smart build system with ~2.7s build time
- **Bundle Optimization**: ES2022 target with efficient module resolution
- **Development**: Fast incremental builds and watch mode

### Runtime Performance
- **Result<T> Pattern**: Eliminates exception overhead
- **Dependency Injection**: Efficient container-based dependency management
- **Session Management**: Persistent state reduces initialization overhead
- **Infrastructure Clients**: Connection pooling for Docker and Kubernetes

### Architecture Benefits
- **Modular Design**: Clean boundaries between layers reduce complexity
- **Type Safety**: Compile-time type checking prevents runtime errors
- **Path Aliases**: Clean imports improve build performance

---

## Knowledge Packs and Policy System

### Knowledge Pack Integration

Knowledge packs are a first-class feature that enhance AI prompts with domain-specific best practices and guidance. They are actively used throughout the containerization workflow and remain fully integrated post-simplification.

**How Knowledge Packs Work**:
1. **Static Data**: Knowledge stored as JSON files in `knowledge/packs/` (outside src/)
2. **Loading**: `src/knowledge/loader.ts` validates and loads knowledge packs
3. **Matching**: `src/knowledge/matcher.ts` selects relevant knowledge based on tool context
4. **Enhancement**: `src/ai/prompt-engine.ts` imports from `@/knowledge/matcher` and injects knowledge via `getKnowledgeSnippets()`
5. **Budget Control**: Knowledge selection respects character budgets via `maxChars` parameter
6. **Integration**: `buildMessages()` automatically calls `selectKnowledgeSnippets()` with topic, environment, and tool context

**Tools Using Knowledge Packs**:
- `generate-dockerfile`: Best practices for container images (multi-stage builds, security hardening)
- `scan`: Security scanning guidance and vulnerability remediation
- `generate-k8s-manifests`: Kubernetes deployment patterns and best practices
- `resolve-base-images`: Base image recommendations by language/framework
- `fix-dockerfile`: Dockerfile optimization techniques

**Knowledge Flow in Tool Execution**:
```
Tool Invocation → ToolContext → buildMessages() → getKnowledgeSnippets() → AI Prompt
```

**Verification**:
- Unit tests: `test/unit/ai/prompt-engine.test.ts` validates knowledge injection
- Integration tests: `test/integration/knowledge-policy-validation.test.ts` verifies end-to-end flow
- Knowledge snippets visible in AI prompt metadata (`knowledgeCount` field)

**Adding Knowledge**:
1. Create/update JSON files in `knowledge/packs/`
2. Follow the pack schema (tool-specific categories with `topic`, `environment`, `language` filters)
3. Knowledge is automatically loaded and matched by tool name and context
4. Use `maxChars` and `maxSnippets` to control knowledge budget

### Policy System

The policy system provides runtime configuration and constraint enforcement. It remains fully active post-cleanup.

**Policy Architecture** (5 specialized modules in `src/config/`):
1. **`policy-schemas.ts`**: Zod schemas and TypeScript type definitions
2. **`policy-io.ts`**: Load, validate, migrate, and cache policy files
3. **`policy-eval.ts`**: Rule evaluation and application logic
4. **`policy-prompt.ts`**: AI prompt constraint integration
5. **`policy-constraints.ts`**: Data-driven constraint extraction

**Policy Enforcement Flow**:
1. **Configuration**: Pass `policyPath` and optional `policyEnvironment` to `createApp()` or `createOrchestrator()`
2. **Load**: `loadPolicy()` reads and validates YAML policy files at startup
3. **Evaluation**: During tool execution, `applyPolicy()` matches tool name and parameters against policy rules
4. **Blocking**: Rules with `actions.block: true` prevent tool execution and return failure
5. **Warnings**: Rules with `actions.warn: true` log warnings but allow execution
6. **Integration**: `src/app/orchestrator.ts` enforces policies before calling `tool.run()` (lines 232-245)

**Policy Enforcement in Orchestrator**:
```typescript
if (policy) {
  const policyResults = applyPolicy(policy, {
    tool: tool.name,
    params: validatedParams
  });

  const blockers = policyResults
    .filter(r => r.matched && r.rule.actions.block)
    .map(r => r.rule.id);

  if (blockers.length > 0) {
    return Failure(ERROR_MESSAGES.POLICY_BLOCKED(blockers));
  }
}
```

**Policy Features**:
- **Type-safe**: Discriminated unions for compile-time safety (RegexMatcher vs FunctionMatcher)
- **Stateless**: Pure functions without global mutable caches
- **Environment-aware**: Policy rules can override per-environment
- **Session-compatible**: Policies apply across all tools in a session
- **Modular**: Each of 5 modules has single responsibility

**Verification**:
- Unit tests: `test/unit/app/orchestrator.test.ts` validates policy application
- Unit tests: `test/unit/config/policy-validation.test.ts` validates policy loading and evaluation
- Integration tests: `test/integration/knowledge-policy-validation.test.ts` verifies blocking behavior
- Runtime: Policy violations return `Result.Failure` with blocker rule IDs

**Example Policy** (YAML):
```yaml
version: "1.0"
rules:
  - id: block-production-deletion
    category: compliance
    priority: 100
    conditions:
      - kind: regex
        pattern: "production|prod"
      - kind: regex
        pattern: "delete|remove"
    actions:
      block: true
      message: "Cannot delete production resources"
```

**Adding/Updating Policies**:
1. Create YAML policy file with `version`, `rules`, and optional `environments`
2. Each rule has: `id`, `priority`, `conditions` (matchers), `actions`
3. Pass policy file path via `policyPath` config option
4. Override for specific environments using `environments` key in policy file

---

## Quickstart: Adding Knowledge Packs and Policies

### Adding a New Knowledge Snippet

Knowledge snippets enhance AI prompts with domain-specific guidance. Follow these steps:

1. **Identify the target tool** (e.g., `generate-dockerfile`, `generate-k8s-manifests`)

2. **Create or update pack file** in `knowledge/packs/`:
   ```bash
   # Example: knowledge/packs/dockerfile-security.json
   ```

3. **Define knowledge structure**:
   ```json
   {
     "generate-dockerfile": [
       {
         "topic": "dockerfile-generation",
         "environment": "production",
         "language": "node",
         "content": "For Node.js applications, use multi-stage builds...",
         "priority": 10
       }
     ]
   }
   ```

4. **Test knowledge integration**:
   ```bash
   npm run test:unit -- test/unit/knowledge/
   npm run test:integration -- test/integration/knowledge-policy-validation.test.ts
   ```

5. **Verify in prompts**:
   - Knowledge snippets are automatically included based on `topic`, `environment`, and `tool`
   - Check `metadata.knowledgeCount` in prompt results
   - Use `maxChars` parameter to control budget

**Knowledge Pack Schema**:
- `topic`: Matches against `TOPICS` enum (e.g., `TOPICS.DOCKERFILE_GENERATION`)
- `environment`: Optional filter (`production`, `development`, `test`)
- `language`: Optional filter (e.g., `node`, `python`, `go`)
- `content`: The actual guidance text (Markdown supported)
- `priority`: Higher priority snippets selected first (default: 0)

**Best Practices**:
- Keep snippets focused and actionable (200-500 characters)
- Use higher priority (10+) for critical security/correctness guidance
- Test with different budget constraints to ensure snippet is used
- Include examples and specific recommendations, not general advice

### Adding a New Policy Rule

Policy rules enforce constraints and governance during tool execution.

1. **Create policy file** (or extend existing):
   ```bash
   # Example: policies/my-team-policy.yaml
   ```

2. **Define policy rule**:
   ```yaml
   version: "1.0"
   rules:
     - id: require-version-tags
       category: quality
       priority: 50
       conditions:
         - kind: regex
           pattern: "tag-image"
           field: tool
         - kind: regex
           pattern: ":latest$"
           field: params.imageName
       actions:
         block: true
         warn: true
         message: "Image tags must use semantic versions, not :latest"
       description: "Enforce semantic versioning for image tags"
   ```

3. **Apply policy** via configuration:
   ```typescript
   const app = await createApp({
     policyPath: './policies/my-team-policy.yaml',
     policyEnvironment: 'production'
   });
   ```

4. **Test policy enforcement**:
   ```bash
   npm run test:unit -- test/unit/config/policy-eval.test.ts
   npm run test:integration -- test/integration/knowledge-policy-validation.test.ts
   ```

**Policy Rule Components**:
- `id`: Unique rule identifier
- `category`: Rule category (`security`, `quality`, `compliance`)
- `priority`: Higher priority rules evaluated first (0-100)
- `conditions`: Array of matchers (all must match for rule to trigger)
  - `kind: regex`: Pattern matching against tool name or params
  - `field`: Optional field selector (e.g., `params.imageName`, `tool`)
- `actions`: What happens when rule matches
  - `block`: Prevent tool execution (returns error)
  - `warn`: Log warning but allow execution
  - `message`: User-facing explanation

**Testing Policy Rules**:
```typescript
import { loadPolicy } from '@config/policy-io';
import { applyPolicy } from '@config/policy-eval';

const result = loadPolicy('./policies/my-team-policy.yaml');
if (result.ok) {
  const policyResults = applyPolicy(result.value, {
    tool: 'tag-image',
    params: { imageName: 'myapp:latest' }
  });

  const blocked = policyResults.find(r => r.matched && r.rule.actions.block);
  console.log('Policy blocked:', blocked !== undefined);
}
```

**Environment-Specific Policies**:
```yaml
version: "1.0"
rules:
  - id: strict-in-production
    # ... rule definition ...

environments:
  production:
    rules:
      - id: strict-in-production
        actions:
          block: true  # Block in production
  development:
    rules:
      - id: strict-in-production
        actions:
          warn: true   # Only warn in development
```

---

## Conclusion

The Containerization Assist MCP Server represents a modern, well-architected approach to AI-powered containerization workflows. Its clean separation of concerns, Result-based error handling, and comprehensive tool ecosystem make it both reliable and extensible. The integration of knowledge packs for AI enhancement and the modular policy system for runtime configuration ensure that the platform can adapt to diverse requirements while maintaining deterministic, high-quality outputs. The focus on developer experience through fast builds, clear documentation, and comprehensive testing ensures long-term maintainability and ease of contribution.
