# Container Kit MCP Tools Inventory

## Current Implementation Status

Container Kit is in active development, transitioning from a legacy architecture to a clean three-layer design. **Most tools are currently in development** and only a basic set is implemented.

## Architecture Overview

### Three-Layer Architecture (ADR-001)

Following clean architecture principles with strict dependency rules:

```
pkg/mcp/
├── domain/              # Domain layer - business logic (no dependencies)
│   ├── config/         # Configuration entities and validation
│   ├── containerization/ # Container operations domain models
│   ├── errors/         # Rich error handling system (ADR-004)
│   ├── security/       # Security policies and validation (ADR-005)
│   ├── session/        # Session entities and rules
│   ├── types/          # Core domain types
│   └── internal/       # Shared utilities
├── application/         # Application layer - orchestration (depends on domain)
│   ├── api/            # Canonical interface definitions (single source of truth)
│   ├── commands/       # Command implementations (in development)
│   ├── core/           # Server lifecycle & registry management
│   ├── orchestration/  # Tool coordination & workflows
│   ├── services/       # Service interfaces for dependency injection (ADR-006)
│   └── internal/       # Internal implementations
└── infra/              # Infrastructure layer - external integrations
    ├── adapters/       # Interface adapters
    ├── persistence/    # BoltDB storage
    ├── transport/      # MCP protocol transports (stdio, HTTP)
    └── templates/      # YAML templates (ADR-002)
```

**Dependency Rules:**
- Domain → No dependencies
- Application → Domain only  
- Infrastructure → Domain and Application

## Currently Implemented Tools

### 1. Core Tools (4 implemented)

| Tool Name | Status | Purpose | Implementation |
|-----------|--------|---------|----------------|
| `analyze_repository` | ✅ **Basic** | Repository analysis with mock data | `pkg/mcp/application/core/server_impl.go:251` |
| `list_sessions` | ✅ **Working** | List active MCP sessions | `pkg/mcp/application/core/server_impl.go:310` |
| `ping` | ✅ **Working** | Connectivity testing | `pkg/mcp/application/core/server_impl.go:353` |
| `server_status` | ✅ **Working** | Server status information | `pkg/mcp/application/core/server_impl.go:367` |

### Implementation Details

#### `analyze_repository`
- **Status**: Mock implementation with basic validation
- **Parameters**: `repo_url` (required), `context`, `branch`, `language_hint`, `shallow`
- **Returns**: Success status, mock analysis data, sample Dockerfile
- **Location**: Inline function in server registration

#### `list_sessions`
- **Status**: Functional with no-op session manager
- **Parameters**: `limit` (optional)
- **Returns**: Empty sessions array (session manager not fully implemented)
- **Location**: Inline function in server registration

#### `ping` & `server_status`
- **Status**: Fully functional diagnostic tools
- **Purpose**: Testing connectivity and server health
- **Location**: Inline functions in server registration

## Planned Tool Architecture

### Service Container Pattern (ADR-006)

The planned architecture uses manual dependency injection with focused services:

```go
type ServiceContainer interface {
    SessionStore() SessionStore        // Session CRUD operations
    SessionState() SessionState        // State & checkpoint management
    BuildExecutor() BuildExecutor      // Container build operations
    ToolRegistry() ToolRegistry        // Tool registration & discovery
    WorkflowExecutor() WorkflowExecutor // Multi-step workflows
    Scanner() Scanner                  // Security scanning
    ConfigValidator() ConfigValidator  // Configuration validation
    ErrorReporter() ErrorReporter      // Unified error handling
}
```

### Planned Tool Domains

#### 1. Analysis Tools (Planned)
- **analyze_repository_consolidated** - Full repository analysis
- **validate_dockerfile** - Dockerfile validation with security checks
- **detect_databases** - Database detection in repositories

#### 2. Build Tools (Planned)  
- **build_image** - Docker image building with AI-powered error fixing
- **push_image** - Container registry operations
- **docker_operations** - Unified Docker operations

#### 3. Deploy Tools (Planned)
- **generate_manifests** - Kubernetes manifest generation
- **deploy_kubernetes** - Kubernetes deployment with health checks
- **validate_manifests** - Manifest validation

#### 4. Security Tools (Planned)
- **scan_image_security** - Vulnerability scanning with Trivy/Grype
- **scan_secrets** - Secret detection and remediation
- **security_audit** - Comprehensive security analysis

#### 5. Session Management (Planned)
- **delete_session** - Session deletion with workspace cleanup
- **manage_session_labels** - Label-based session organization
- **session_cleanup** - Automated cleanup operations

## Development Status

### Current Phase: Foundation
- ✅ Three-layer architecture established
- ✅ ADR decisions documented and implemented
- ✅ Basic MCP server infrastructure
- ✅ Service container interfaces defined
- 🔄 Command implementations in progress
- ⏳ Tool consolidation pending

### Next Phase: Tool Implementation
- ⏳ Session manager implementation
- ⏳ Pipeline operations framework
- ⏳ Service container wiring
- ⏳ Command constructor implementations
- ⏳ Full tool registration

### Implementation Files

#### Currently Active
- `pkg/mcp/application/core/server_impl.go` - Server and basic tool registration
- `pkg/mcp/application/commands/*.go` - Command implementations (partial)
- `pkg/mcp/domain/` - Domain models and entities

#### Commented Out (Waiting for Dependencies)
Most advanced tools are commented out in `server_impl.go` lines 206-248 until:
- Pipeline operations are implemented
- Session manager is completed  
- Service container is wired up
- Command constructors are built

## Migration from Legacy

The current state represents a transition from a legacy system. Many references to "consolidated tools" in documentation refer to a previous architecture that is being replaced with the current three-layer design.

### Legacy Cleanup Status
- ✅ Package structure simplified (ADR-001)
- ✅ Error system unified (ADR-004) 
- ✅ Validation DSL established (ADR-005)
- ✅ Service container design (ADR-006)
- 🔄 Tool implementations migrating to new architecture
- ⏳ Session management being rebuilt

## Contributing to Tool Development

When implementing new tools:

1. **Follow ADRs**: Use established patterns from architectural decisions
2. **Use Service Container**: Inject dependencies through the service container
3. **Rich Error Handling**: Use the unified error system from `domain/errors/rich.go`
4. **Validation DSL**: Use struct tags for parameter validation
5. **Session Integration**: Build on the session management framework

See [ADDING_NEW_TOOLS.md](./ADDING_NEW_TOOLS.md) for detailed implementation guidance.

## References

- [Three-Layer Architecture](./THREE_LAYER_ARCHITECTURE.md)
- [Architectural Decisions](./architecture/adr/)
- [Tool Development Guide](./ADDING_NEW_TOOLS.md)
- [Quality Standards](./QUALITY_STANDARDS.md)