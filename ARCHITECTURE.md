# Container Kit Architecture

## 🧭 Overview

Container Kit is an AI-powered tool that automates application containerization and Kubernetes manifest generation. It provides two distinct modes of operation with different architectural approaches.

## 🏗️ Two-Mode Architecture

### 1. MCP Server (Primary) - Atomic + Conversational

The MCP (Model Context Protocol) server is the modern, recommended approach with enhanced AI integration:

```
┌─────────────────────────────────────────────────────────────┐
│                    MCP Server                               │
├─────────────────────────────────────────────────────────────┤
│  Transport Layer (stdio/http)                              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────────────────────┐   │
│  │  Atomic Tools   │  │    Conversation Mode           │   │
│  │                 │  │                                 │   │
│  │ • analyze_repo  │  │ • Chat Tool                     │   │
│  │ • generate_df   │  │ • Prompt Manager                │   │
│  │ • build_image   │  │ • Session State                 │   │
│  │ • push_image    │  │ • Preference Store              │   │
│  │ • gen_manifests │  │ • Telemetry                     │   │
│  │ • deploy_k8s    │  │                                 │   │
│  │ • check_health  │  └─────────────────────────────────┘   │
│  └─────────────────┘                                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐                  │
│  │ Session Manager │  │ Workspace Mgr   │                  │
│  │ (BoltDB)        │  │ (File System)   │                  │
│  └─────────────────┘  └─────────────────┘                  │
└─────────────────────────────────────────────────────────────┘
```

### 2. CLI Tool - Pipeline-Based

The original CLI uses a three-stage iterative pipeline:

```
┌─────────────────────────────────────────────────────────────┐
│                    CLI Pipeline                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐  │
│  │ Repository      │  │ Docker Stage    │  │ Manifest    │  │
│  │ Analysis        │  │                 │  │ Stage       │  │
│  │                 │  │ • Generate      │  │             │  │
│  │ • File scanning │  │ • Build → Fix   │  │ • Generate  │  │
│  │ • AI analysis   │  │ • Retry loop    │  │ • Deploy    │  │
│  │ • Template sel  │  │ • Push to reg   │  │ • Fix loop  │  │
│  └─────────────────┘  └─────────────────┘  └─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                     Azure OpenAI                           │
└─────────────────────────────────────────────────────────────┘
```

## 🎯 Design Principles

### MCP Server Principles

1. **No AI in Tools**: All atomic tools are deterministic, mechanical operations
2. **Rich AI Context**: Tools provide structured data following the [AI Integration Pattern](docs/AI_INTEGRATION_PATTERN.md)
3. **Stateless Conversation**: Conversation state managed by client (Claude)
4. **Session Persistence**: User sessions and preferences persist across interactions
5. **Composable Operations**: Tools can be used independently or in workflows
6. **Performance First**: <2s response time target with monitoring
7. **Iterative Fixing**: AI-driven build→fail→analyze→fix→retry loops built into atomic tools
8. **Security by Design**: Built-in security scanning, secret detection, and best practice validation
9. **Mechanical + Context = Success**: Reliable operations + comprehensive guidance for AI reasoning

### CLI Principles 

1. **Iterative Refinement**: Build → Fail → Analyze → Fix → Retry loops
2. **AI-Driven**: Azure OpenAI analyzes errors and suggests fixes
3. **Template-Based**: Uses Draft templates as starting points
4. **Snapshot Debugging**: Saves iteration snapshots for debugging

## 📦 Core Modules

### MCP Server Components

#### `/pkg/mcp/internal/core/`
**Main MCP server implementation**

- `server.go` - Core MCP server with transport handling
- `server_atomic.go` - Atomic tool registration and handling
- `server_conversation.go` - Conversation mode implementation
- `server_lifecycle.go` - Server startup, shutdown, and health management
- `tool_registry.go` - Tool registration and discovery
- `gomcp_manager.go` - GoMCP integration layer

#### `/pkg/mcp/internal/engine/`
**Core engine components**

- `conversation/` - Conversation mode and prompt management
- `orchestration/` - Tool orchestration and workflow management

#### `/pkg/mcp/internal/store/`
**Data persistence layer**

- `session/` - Session management with BoltDB
- `preference/` - User preference persistence

#### `/pkg/mcp/internal/ops/`
**Operations and monitoring**

- `telemetry_manager.go` - Prometheus metrics and monitoring
- `preflight_checker.go` - System validation
- `otel_middleware.go` - OpenTelemetry integration

#### `/pkg/mcp/internal/fixing/`
**AI-driven fixing capabilities**

- `iterative_fixer.go` - Core fixing framework
- `atomic_tool_mixin.go` - Fixing integration for atomic tools
- `analyzer_integration.go` - Error analysis integration

#### `/pkg/mcp/internal/tools/`
**Atomic tool implementations**

- `analyze_repository_atomic.go` - Repository structure analysis
- `build_image_atomic.go` - Docker image building with fixing
- `generate_dockerfile.go` - Dockerfile generation
- `push_image_atomic.go` - Registry push operations
- `pull_image_atomic.go` - Registry pull operations  
- `tag_image_atomic.go` - Image tagging operations
- `deploy_kubernetes_atomic.go` - K8s deployment with fixing
- `generate_manifests_atomic.go` - K8s manifest generation
- `check_health_atomic.go` - Deployment health verification
- `scan_image_security_atomic.go` - Security vulnerability scanning
- `scan_secrets_atomic.go` - Secret detection and remediation
- `validate_dockerfile_atomic.go` - Dockerfile validation
- `chat_tool.go` - Conversation mode tool

#### `/pkg/mcp/utils/`
**Shared utilities**

- `common.go` - Common helper functions
- `errors.go` - Standardized error handling
- `tool_result.go` - Standardized tool results

### CLI Components 

#### Command Line Interface
- **`cmd/`**: CLI entrypoints (root, generate, test, setup)

#### Core Packages
- **`pkg/ai/`**: Azure OpenAI client wrapper
- **`pkg/clients/`**: External client aggregation (Docker, Kubectl, Kind)
- **`pkg/docker/`**: Dockerfile templating and build operations
- **`pkg/k8s/`**: Kubernetes manifest discovery and operations
- **`pkg/pipeline/`**: Core orchestration and iteration logic
- **`pkg/filetree/`**: Repository file structure analysis

## 🔄 Workflow Patterns

### MCP Server Workflows

#### Atomic Tool Usage
```
Client → MCP Server → Tool → Result → Client
```

#### Conversation Mode
```
Client → Chat Tool → Prompt Manager → Tool Orchestrator → Tools → Result → Client
        ↓
    Session State (BoltDB)
        ↓
    Preference Store
```

#### Conversation Stages
1. **PreFlight**: System validation (Docker, K8s, registry)
2. **Init**: Repository selection and setup
3. **Analysis**: Repository analysis and framework detection
4. **Dockerfile**: Generation and review
5. **Build**: Image building with progress
6. **Push**: Registry push (optional)
7. **Manifests**: K8s manifest generation
8. **Deployment**: Application deployment
9. **Completed**: Summary and next steps

### CLI Workflow (Legacy)

#### Pipeline Execution
```
Target Repo → Analysis → Template → Dockerfile → Build → Fix → Manifests → Deploy → Fix
                ↓           ↓          ↓         ↓      ↓        ↓         ↓
            AI Analysis  AI Select   AI Fix   Retry   AI Fix   Deploy   Retry
```

#### Iteration Loops
1. **Dockerfile Iteration**: Build → Fail → AI Fix → Retry (max 5x)
2. **Manifest Iteration**: Deploy → Fail → AI Fix → Retry (max 5x)
3. **Snapshot Creation**: Save state after each iteration

## 🗄️ Data Management

### MCP Server Storage

#### BoltDB (Sessions)
```
sessions/
├── session_id_1/
│   ├── metadata (created, updated, ttl)
│   ├── repo_analysis
│   ├── dockerfile
│   ├── build_results
│   └── k8s_manifests
└── session_id_2/...
```

#### BoltDB (Preferences)
```
preferences/
├── user_id_1/
│   ├── global_preferences
│   ├── language_defaults
│   └── recent_choices
└── user_id_2/...
```

#### File System (Workspaces)
```
workspaces/
├── session_id_1/
│   ├── repo/           # Cloned repository
│   ├── Dockerfile      # Generated Dockerfile
│   ├── manifests/      # K8s manifests
│   └── .snapshots/     # Iteration snapshots
└── session_id_2/...
```

### CLI Storage 

#### Snapshots
```
.container-kit-snapshots/
├── iteration_1/
│   ├── Dockerfile
│   ├── build_error.log
│   └── ai_response.json
├── iteration_2/...
└── final/
```

## 🔧 External Integrations

### Required Dependencies
- **Docker**: Image building and registry operations
- **kubectl**: Kubernetes operations (optional)
- **BoltDB**: Embedded database for sessions/preferences

### Optional Dependencies
- **Kind**: Local Kubernetes testing
- **Azure OpenAI**: AI features (CLI mode)
- **Prometheus**: Metrics collection (MCP telemetry)

## 🎛️ Configuration

### MCP Server Config
```go
ServerConfig{
    WorkspaceDir:      "/tmp/container-kit",
    MaxSessions:       100,
    SessionTTL:        24 * time.Hour,
    MaxDiskPerSession: 1024 * 1024 * 1024,
    TransportType:     "stdio", // or "http"
}

ConversationConfig{
    EnableTelemetry:   true,
    TelemetryPort:     9090,
    PreferencesDBPath: "/path/to/preferences.db",
}
```

### CLI Config 
```bash
AZURE_OPENAI_KEY=xxxxxxx
AZURE_OPENAI_ENDPOINT=xxxxxx
AZURE_OPENAI_DEPLOYMENT_ID=container-kit
```

## 🚀 Deployment Models

### MCP Server Deployment
- **Development**: Local stdio transport with Claude Desktop or VS Code devcontainer
- **Production**: HTTP transport with load balancing
- **Cloud**: Container deployment with persistent volumes
- **Instant Setup**: VS Code devcontainer with all tools pre-configured

### CLI Deployment =
- **Local**: Direct execution with local Docker/Kind
- **CI/CD**: Pipeline integration for automated containerization

## 📊 Observability

### MCP Server Metrics
- Tool execution duration and success rates
- Session lifecycle metrics
- Performance budget violations (>2s target)
- Active session and workspace usage

### CLI Metrics 
- Pipeline stage completion times
- AI iteration counts and success rates
- Build and deployment success rates

## 🔮 Future Architecture

### Planned Enhancements
- **Multi-language Support**: Plugin system for custom tools
- **Enhanced Telemetry**: Grafana dashboards and alerting
- **Distributed Sessions**: Multi-server session sharing
- **Advanced Security**: RBAC and audit logging

### Migration Path
- CLI → MCP Server migration utilities
- Backward compatibility layer
- Progressive feature migration