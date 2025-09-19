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
**Purpose**: Main application interfaces and dependency injection setup.

**Key Files**:
- `index.ts`: Main application exports and interfaces
- `container.ts`: Dependency injection container configuration and factory functions

**Responsibilities**:
- Application lifecycle management
- Dependency injection setup
- Container configuration for different environments

### 📁 `/cli` - Command Line Interface
**Purpose**: Command-line interface entry points and argument parsing.

**Key Files**:
- `cli.ts`: Main CLI entry point with Commander.js setup and validation logic
- `server.ts`: Server startup and configuration logic

**Responsibilities**:
- CLI argument parsing and validation
- Environment variable configuration
- Docker and Kubernetes connection validation
- Health checks and diagnostics
- Server initialization and shutdown handling

### 📁 `/config` - Configuration Management
**Purpose**: Centralized configuration management system replacing multiple config files.

**Key Files**:
- `index.ts`: Main configuration factory and environment variable mapping
- `types.ts`: TypeScript interfaces for configuration structures
- `defaults.ts`: Default values and constants
- `app-config.ts`: Application-specific configuration
- `tool-config.ts`: Tool-specific configuration settings

**Responsibilities**:
- Environment variable parsing and validation
- Configuration defaults and overrides
- Type-safe configuration interfaces
- Development/production configuration profiles

### 📁 `/types` - Type Definitions
**Purpose**: Centralized type definitions and domain models.

**Key Files**:
- `consolidated-types.ts`: Core type definitions including Result<T>, Tool interfaces, WorkflowState
- `validate-dockerfile.d.ts`: Dockerfile validation type definitions

**Responsibilities**:
- Result<T> type system for error handling
- Tool and workflow interfaces
- Session management types
- Domain model definitions

### 📁 `/services` - External System Services
**Purpose**: Service implementations for external systems (Docker, Kubernetes).

**Subdirectories**:

#### `/docker`
- Docker service implementations and utilities
- Mutex-based client management

#### `/kubernetes`  
- Kubernetes service implementations
- Idempotent apply operations

**Responsibilities**:
- External system integration
- Service layer abstractions
- Connection management
- Platform-specific implementations

### 📁 `/lib` - Shared Libraries and Utilities
**Purpose**: Reusable libraries and cross-cutting concerns.

**Key Files**:
- `cache.ts`: Caching strategies and implementations
- `logger.ts`: Structured logging with Pino  
- `mutex.ts`: Concurrency control utilities
- `security-scanner.ts`: Trivy integration for vulnerability scanning
- `error-context.ts`: Error handling utilities

**Subdirectories**:
- `/config`: Configuration-related utilities and scoring engines

**Responsibilities**:
- Session state management
- Caching and performance optimization
- Security scanning orchestration
- Shared utilities and helper functions

### 📁 `/mcp` - MCP Server Implementation
**Purpose**: Model Context Protocol server implementation and MCP-specific logic.

**Subdirectories**:

#### `/client`
- `mcp-client.ts`: MCP client implementation
- `mock-transport.ts`: Mock transport for testing
- `sdk-transport.ts`: SDK-based transport layer
- `transport.ts`: Transport abstraction layer

#### `/server`
- `index.ts`: Main MCP server implementation
- `types.ts`: MCP server types
- `schemas.ts`: MCP schema definitions

#### `/tools`
- `registrar.ts`: Tool registration and discovery
- Various utility files for tool management

#### `/context`
- Context management for tool execution

#### `/utils`
- Utility functions specific to MCP operations

**Responsibilities**:
- MCP protocol implementation
- Tool registration and routing
- Progress reporting
- Request/response handling
- Sampling and AI integration

### 📁 `/prompts` - Prompt Management
**Purpose**: AI prompt templates and prompt registry.

**Structure**: Organized by category (analysis, containerization, orchestration, sampling, security, validation)
- JSON files for prompt templates
- `loader.ts` and `registry.ts` for prompt management

**Responsibilities**:
- AI prompt templates
- Dynamic prompt generation
- Context-aware prompt selection

### 📁 `/resources` - Resource Management
**Purpose**: MCP resource management and caching.

**Key Files**:
- `cache.ts`: Resource caching implementation
- `manager.ts`: Resource lifecycle management
- `resource-cache.ts`: Specific resource caching strategies
- `types.ts`: Resource-related type definitions
- `uri-schemes.ts`: URI scheme handling for resources

**Responsibilities**:
- Resource discovery and management
- Caching strategies for expensive operations
- URI-based resource access
- Resource lifecycle management

### 📁 `/tools` - Tool Implementations
**Purpose**: Individual tool implementations using co-located pattern.

**Structure**: Each tool follows the same pattern:
```
/tool-name/
├── tool.ts     # Tool implementation
├── schema.ts   # Zod schema definition
└── index.ts    # Public exports
```

**Available Tools** (using co-location pattern):
- `analyze-repo`: Repository structure and framework detection
- `build-image`: Docker image building with progress tracking
- `deploy`: Application deployment to Kubernetes
- `fix-dockerfile`: Dockerfile optimization and fixes
- `generate-dockerfile`: AI-powered Dockerfile generation
- `generate-k8s-manifests`: Kubernetes manifest generation
- `ops`: Operational tools (ping, health checks)
- `prepare-cluster`: Kubernetes cluster preparation
- `push-image`: Image registry operations
- `resolve-base-images`: Base image recommendations
- `scan`: Security vulnerability scanning
- `tag-image`: Docker image tagging
- `verify-deploy`: Deployment verification and health checks
- `workflow`: Workflow orchestration tools

**Additional Files**:
- `types.ts`: Common tool types and interfaces
- `session-types.ts`: Session-related type definitions
- `analysis-perspectives.ts`: Multi-perspective analysis strategies

**Responsibilities**:
- Individual tool logic and implementation
- Parameter validation using Zod schemas
- Result-based error handling
- Progress reporting integration

### 📁 `/workflows` - Workflow Orchestration
**Purpose**: Complex workflow orchestration and pipeline management.

**Key Files**:
- `containerization.ts`: Main containerization pipeline
- `deployment.ts`: Deployment workflow orchestration
- `intelligent-orchestration.ts`: AI-driven workflow decisions
- `workflow-config.ts`: Workflow configuration utilities
- `types.ts`: Workflow type definitions

**Responsibilities**:
- Multi-step workflow orchestration
- AI-driven workflow decisions
- Workflow state management
- Error recovery and retry logic

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
import type { Result } from '@/types';

// ✅ Relative imports (also acceptable)
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

### Configuration Structure
Configuration is centralized in `/config` with type-safe interfaces:

```typescript
export const config = {
  mcp: { name: 'containerization-assist', version: '1.0.0' },
  server: { logLevel: 'info', port: 3000 },
  workspace: { workspaceDir: process.cwd() },
  docker: { socketPath: '/var/run/docker.sock' },
  kubernetes: { namespace: 'default' },
  // ... other configuration sections
};
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
2. Implement `tool.ts` with execute function
3. Define `schema.ts` with Zod validation
4. Export via `index.ts`
5. Register in tool registry

### Adding New Workflows
1. Create workflow file in `src/workflows/`
2. Implement using existing tool composition
3. Add workflow registration
4. Include progress reporting

### Infrastructure Extensions
1. Add new clients in `src/infrastructure/`
2. Follow Result<T> pattern for error handling
3. Export via index files
4. Register in dependency container

---

## Performance Considerations

### Build Performance
- **TypeScript Compilation**: Standard tsc with fast incremental builds
- **Alias Resolution**: tsc-alias for clean path resolution
- **Bundle Optimization**: Tree shaking and minification in production builds

### Runtime Performance
- **Caching**: Multi-layer caching for expensive operations
- **Connection Pooling**: Efficient Docker and Kubernetes connection management
- **Progress Streaming**: Real-time progress updates without blocking

### Quality Metrics
- **ESLint Warnings**: 700 (46% reduction from initial baseline)
- **TypeScript Errors**: 45 (ongoing reduction effort)
- **Dead Code**: 234 unused exports (47% reduction)
- **Test Coverage**: >70% with comprehensive integration testing

---

## Conclusion

The Containerization Assist MCP Server represents a modern, well-architected approach to AI-powered containerization workflows. Its clean separation of concerns, Result-based error handling, and comprehensive tool ecosystem make it both reliable and extensible. The focus on developer experience through fast builds, clear documentation, and comprehensive testing ensures long-term maintainability and ease of contribution.