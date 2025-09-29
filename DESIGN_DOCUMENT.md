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
- `container.ts`: Dependency injection container configuration

**Responsibilities**:
- Public API definition
- Dependency injection setup

### 📁 `/ai` - AI and Prompt Engine
**Purpose**: Complete prompt engine and AI integration.

**Key Files**:
- `prompt-engine.ts`: Core prompt building and message handling
- `prompt-registry.ts`: Prompt template management
- `prompt-templates.ts`: Template definitions

**Responsibilities**:
- AI prompt generation and management
- Message building for AI interactions
- Template system for consistent prompts

### 📁 `/app` - Application Core
**Purpose**: Core application logic and orchestration.

**Key Files**:
- `index.ts`: Main application entry point
- `kernel.ts`: Application kernel and lifecycle management

**Responsibilities**:
- Application startup and shutdown
- Core business logic coordination

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
**Purpose**: Knowledge base and matching system.

**Key Files**:
- `matcher.ts`: Knowledge matching logic

**Responsibilities**:
- Knowledge base integration
- Content matching and retrieval

### 📁 `/lib` - Pure Utilities
**Purpose**: Reusable utilities with no infrastructure dependencies.

**Key Files**:
- `docker.ts`: Docker utility functions
- `file-utils.ts`: File system utilities
- `regex-patterns.ts`: Common regex patterns
- `sampling.ts`: Sampling utilities

**Responsibilities**:
- Pure utility functions
- Helper libraries
- Common patterns and utilities

### 📁 `/mcp` - MCP Server Implementation
**Purpose**: Model Context Protocol server and adapters.

**Subdirectories**:

#### `/ai`
- `sampling-runner.ts`: AI sampling and execution

**Key Files**:
- Various MCP-specific implementations

**Responsibilities**:
- MCP protocol implementation
- Tool registration and routing
- MCP-specific AI integration

### 📁 `/session` - Session Management
**Purpose**: Unified session state management.

**Responsibilities**:
- Session lifecycle management
- Persistent state across tool executions

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

### 3. Dependency Injection Container
Centralized dependency management in `/app/container.ts`:

```typescript
export interface Deps {
  logger: Logger;
  dockerClient: DockerClient;
  sessionManager: SessionManager;
  // ... other dependencies
}

export function createContainer(overrides = {}): Deps {
  // Container configuration
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
npm run build           # Full build (ESM + CJS)
npm run build:fast      # Fast development build
npm run validate:pr:fast # Quick PR validation (30s)
npm run lint:fix        # Auto-fix linting issues
npm run test:unit       # Unit tests
npm run quality:gates   # Comprehensive quality analysis
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
2. Register in `src/ai/prompt-registry.ts`
3. Use via prompt engine: `buildMessages()` and related functions

### Infrastructure Extensions
1. Add new clients in `src/infra/`
2. Follow Result<T> pattern for error handling
3. Export via index files
4. Register in dependency container (`src/container.ts`)

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

## Conclusion

The Containerization Assist MCP Server represents a modern, well-architected approach to AI-powered containerization workflows. Its clean separation of concerns, Result-based error handling, and comprehensive tool ecosystem make it both reliable and extensible. The focus on developer experience through fast builds, clear documentation, and comprehensive testing ensures long-term maintainability and ease of contribution.