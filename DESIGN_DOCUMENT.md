# Containerization Assist MCP Server - Design Document

## Project Overview

**Containerization Assist MCP Server** is a comprehensive TypeScript-based MCP (Model Context Protocol) server designed for AI-powered containerization workflows. It provides intelligent Docker and Kubernetes support through a clean, modular architecture that emphasizes reliability, extensibility, and maintainability.

## Architecture Overview

### High-Level System Design

```
┌─────────────────────────────────────────┐
│            MCP Client                   │
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
│  ┌──────────┐ ┌──────────┐             │
│  │  Tools   │ │Workflow  │             │
│  └──────────┘ └──────────┘             │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         Infrastructure Layer            │
│  ┌──────┐ ┌──────┐ ┌─────┐             │
│  │Docker│ │ K8s  │ │ AI  │             │
│  └──────┘ └──────┘ └─────┘             │
└─────────────────────────────────────────┘
```
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
**Purpose**: Core prompt building and template management.

**Key Files**:
- `prompt-engine.ts`: Core prompt building and message handling

**Responsibilities**:
- AI prompt generation
- Message building for AI interactions
- Template management

### 📁 `/mcp/ai` - MCP AI Integration
**Purpose**: MCP-specific AI enhancements and sampling.

**Key Files**:
- `knowledge-enhancement.ts`: Knowledge pack integration
- `sampling-runner.ts`: Deterministic sampling system
- `quality.ts`: Quality scoring for AI outputs
- `response-parser.ts`: AI response parsing
- `schemas.ts`: AI-related schemas

**Responsibilities**:
- Deterministic single-candidate sampling with quality scoring
- Knowledge enhancement of AI prompts
- AI response parsing and validation
- Quality scoring and metadata

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
**Purpose**: Policy system and environment configuration.

**Key Files**:
- `environment.ts`: Unified environment configuration
- `policy-eval.ts`: Rule evaluation and application logic
- `policy-io.ts`: Load, validate, and cache operations
- `policy-schemas.ts`: Zod schemas and TypeScript types
- `policy-data.ts`: Policy data structures

**Responsibilities**:
- Environment variable management
- Policy system for runtime constraints
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

### 📁 `/lib` - Shared Utilities
**Purpose**: Reusable utilities and helpers.

**Key Files**:
- `docker.ts`: Docker utility functions
- `file-utils.ts`: File system utilities
- `regex-patterns.ts`: Common regex patterns
- `security-scanner.ts`: Security scanning utilities
- `tool-helpers.ts`: Tool execution helpers

**Responsibilities**:
- Utility functions
- Helper libraries
- Common patterns

### 📁 `/mcp` - MCP Server Implementation
**Purpose**: Model Context Protocol server and adapters.

**Subdirectories**:

#### `/ai`
- `knowledge-enhancement.ts`: Knowledge pack integration for MCP tools
- `sampling-runner.ts`: Deterministic sampling system
- `quality.ts`: Quality scoring and validation
- `response-parser.ts`: AI response parsing

**Key Files**:
- `mcp-server.ts`: MCP protocol server implementation
- `context.ts`: Tool execution context management

**Responsibilities**:
- MCP protocol implementation
- Tool registration and routing
- MCP-specific AI integration with knowledge enhancement
- Context propagation for tool execution
- Deterministic sampling coordination

### 📁 `/tools` - Tool Implementations
**Purpose**: Individual MCP tool implementations using co-located pattern.

**Structure**: Each tool follows the same pattern:
```
/tool-name/
├── tool.ts     # Tool implementation
└── schema.ts   # Zod schema definition
```

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
- `ai-enhancement.ts`: AI-powered validation enhancement
- `ai-validator.ts`: AI validation logic
- `knowledge-helpers.ts`: Knowledge pack integration for validation

**Responsibilities**:
- Validation logic for containers and manifests
- Automated fixing and optimization
- AI-powered validation suggestions
- Report generation and merging
- Knowledge-enhanced validation

---

## Configuration and Environment

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `DOCKER_SOCKET` | Docker daemon socket path | `/var/run/docker.sock` |
| `KUBECONFIG` | Kubernetes config path | `~/.kube/config` |
| `LOG_LEVEL` | Logging level | `info` |
| `K8S_NAMESPACE` | Default Kubernetes namespace | `default` |

### Configuration Architecture
The configuration system is centralized in `/config`:

```typescript
// Environment configuration via src/config/environment.ts
export const environment = {
  docker: { socketPath: '/var/run/docker.sock' },
  kubernetes: { namespace: 'default' },
  logging: { level: 'info' },
};

// Policy system modules:
// - policy-schemas.ts: Type definitions and Zod schemas
// - policy-io.ts: Load, validate, and cache operations
// - policy-eval.ts: Rule evaluation and application
// - policy-data.ts: Policy data structures
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
4. Register in `src/tools/index.ts`

### Adding New AI Prompts
1. Use the prompt engine: `buildMessages()` function from `src/ai/prompt-engine.ts`
2. Leverage knowledge packs from `knowledge/packs/` for context-aware prompts
3. For tool-specific prompts, use the knowledge-tool-pattern in `src/tools/shared/`

### Infrastructure Extensions
1. Add new clients in `src/infra/`
2. Follow Result<T> pattern for error handling
3. Export via index files
4. Integrate with application factory in `src/app/index.ts`

### Policy System Extensions
1. Extend schemas in `src/config/policy-schemas.ts`
2. Add evaluation logic in `src/config/policy-eval.ts`
3. Add policy data in `src/config/policy-data.ts`

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

**Knowledge Flow**:
```
Tool Invocation → ToolContext → buildMessages() → getKnowledgeSnippets() → AI Prompt
```

**Adding Knowledge**:
1. Create/update JSON files in `knowledge/packs/`
2. Follow the pack schema (tool-specific categories with `topic`, `environment`, `language` filters)
3. Knowledge is automatically loaded and matched by tool name and context
4. Use `maxChars` and `maxSnippets` to control knowledge budget

### Policy System

The policy system provides runtime configuration and constraint enforcement.

**Policy Architecture** (in `src/config/`):
- **`policy-schemas.ts`**: Zod schemas and TypeScript type definitions
- **`policy-io.ts`**: Load, validate, and cache policy files
- **`policy-eval.ts`**: Rule evaluation and application logic
- **`policy-data.ts`**: Policy data structures and examples

**Policy Enforcement Flow**:
1. Pass `policyPath` to `createApp()` or `createOrchestrator()`
2. `loadPolicy()` reads and validates YAML policy files at startup
3. During tool execution, `applyPolicy()` matches tool name and parameters against rules
4. Rules with `actions.block: true` prevent tool execution
5. Rules with `actions.warn: true` log warnings but allow execution
6. `src/app/orchestrator.ts` enforces policies before calling `tool.run()`

**Policy Features**:
- Type-safe with Zod schemas
- Stateless evaluation
- Environment-aware overrides
- Session-compatible

**Testing**:
- Unit tests validate policy application and loading
- Integration tests verify blocking behavior
- Policy violations return `Result.Failure` with rule IDs

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

1. **Identify the target tool** (e.g., `generate-dockerfile-plan`, `generate-k8s-manifests-plan`)

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

4. **Test**:
   ```bash
   npm run test:unit -- test/unit/knowledge/
   ```

**Knowledge Pack Schema**:
- `topic`: Matches against `TOPICS` enum (e.g., `TOPICS.DOCKERFILE_GENERATION`)
- `environment`: Optional filter (`production`, `development`, `test`)
- `language`: Optional filter (e.g., `node`, `python`, `go`)
- `content`: The actual guidance text (Markdown supported)
- `priority`: Higher priority snippets selected first (default: 0)

**Best Practices**:
- Keep snippets focused (200-500 characters)
- Use higher priority (10+) for critical guidance
- Include examples and specific recommendations

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

4. **Test**:
   ```bash
   npm run test:unit -- test/unit/config/policy-eval.test.ts
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
