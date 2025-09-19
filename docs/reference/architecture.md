# Architecture

## Overview

The Containerization Assist MCP Server is a sophisticated, AI-powered MCP implementation that provides comprehensive containerization workflows with Docker and Kubernetes support.

## Single Entry Point

```bash
# Main CLI entry point with all features
containerization-assist-mcp

# Short alias
ca-mcp
```

## Core Architecture

```text
src/cli/cli.ts (Entry Point)
    ↓
MCP Server (src/mcp/server/)
    ├── Main Server (index.ts)
    ├── Progress Reporting (progress.ts)
    ├── Health Monitoring (health.ts)
    ├── Middleware (middleware.ts)
    └── Schema Definitions (schemas.ts, types.ts)
    │
    ├── Session Management (src/lib/session.ts)
    │   ├── State Tracking
    │   ├── Tool History
    │   └── Workflow Progress
    │
    ├── AI Services (src/lib/ai/)
    │   ├── AI Service Implementation (ai-service.ts)
    │   └── MCP Host AI Integration (mcp-host-ai.ts)
    │
    ├── Tool Registry (src/mcp/tools/)
    │   ├── Tool Registration (registry.ts)
    │   ├── Capabilities (capabilities.ts)
    │   └── Validation (validator.ts)
    │
    ├── Workflow Orchestrator (src/workflows/)
    │   ├── Intelligent Orchestration
    │   ├── Containerization Workflows
    │   └── Sampling-based Workflows
    │
    ├── Resource Management (src/resources/)
    │   ├── Resource Manager (manager.ts)
    │   ├── Caching (cache.ts, resource-cache.ts)
    │   └── URI Schemes (uri-schemes.ts)
    │
    └── Prompt Templates (src/prompts/prompt-registry.ts)
        ├── Template Management
        ├── Context Integration
        └── Dynamic Generation
```

## Key Components

### 1. MCP Server Core
- Implements Model Context Protocol specification
- Provides tool registration and routing
- Handles request/response lifecycle
- Manages tool execution context

### 2. Session Manager
- Tracks state across tool executions
- Stores analysis results, generated artifacts
- Maintains tool execution history
- Enables context-aware operations

### 3. Intelligent AI Service
- Validates and optimizes parameters
- Generates contextual guidance
- Analyzes execution results
- Provides next-step recommendations

### 4. Tool Implementations (17 total)
All tools follow the co-location pattern and provide:
- Zod schema validation
- Result-based error handling
- Session state integration
- Structured logging
- AI-powered enhancements where applicable

### 5. Workflow Orchestrator
Intelligent workflows that:
- Plan steps based on session state
- Support conditional execution
- Provide progress updates
- Generate AI recommendations

### 6. Enhanced Resources
- AI-augmented file resources
- Virtual session-based resources
- Custom URI schemes (repository://, dockerfile://, etc.)

### 7. Prompt Templates
- Context-aware prompt generation
- JSON-based templates organized by category
- Session integration
- Type-safe arguments

## Data Flow

```text
1. Client Request → MCP Server
                      ↓
2. Enhanced Handler (with ToolContext)
                      ↓
3. Session Manager (get/update state)
                      ↓
4. AI Service (validate/optimize)
                      ↓
5. Tool Execution (with progress)
                      ↓
6. Result Analysis (AI insights)
                      ↓
7. Session Update (store results)
                      ↓
8. Client Response (with recommendations)
```

## Tool Enhancement Process

```typescript
// Every tool automatically gets:
1. Pre-execution:
   - Parameter validation with AI
   - Session context loading
   - Parameter optimization

2. During execution:
   - Progress reporting
   - Cancellation checking
   - Logging with context

3. Post-execution:
   - Result analysis
   - Recommendation generation
   - Session state update
```

## Session State Structure

```typescript
{
  sessionId: string,
  analysis_result?: RepositoryAnalysis,
  generated_dockerfile?: string,
  k8s_manifests?: K8sManifests,
  scan_results?: ScanResults,
  workflow_state?: WorkflowState,
  completed_steps?: string[],
  tool_history?: ToolExecution[],
  subscriptions?: Subscription[]
}
```

## Tool Execution Flow

```typescript
// Tool execution pattern
1. Parameter validation using Zod schemas
2. Session state loading and management
3. Core tool logic execution
4. Result processing and formatting
5. Session state updates
6. Structured response return
```

## File Structure

```text
src/
├── cli/                         # CLI entry points
│   ├── cli.ts                   # Main CLI entry
│   └── server.ts                # Server utilities
│
├── mcp/                         # MCP server implementation
│   ├── server/                  # Core server components
│   │   ├── index.ts             # Main server
│   │   ├── progress.ts          # Progress reporting
│   │   ├── health.ts            # Health monitoring
│   │   └── middleware.ts        # Request middleware
│   ├── client/                  # MCP client implementation
│   ├── sampling/                # AI sampling services
│   ├── tools/                   # Tool registration
│   └── utils/                   # MCP utilities
│
├── lib/                         # Libraries and utilities
│   ├── ai/                      # AI services
│   ├── session.ts               # Session management
│   └── [other utilities]
│
├── tools/                       # Tool implementations (co-located)
│   ├── analyze-repo/
│   │   ├── tool.ts              # Implementation
│   │   ├── schema.ts            # Validation
│   │   └── index.ts             # Exports
│   └── [other tools]/           # Same structure
```

## Configuration

The server is configured with sensible defaults:

```typescript
// In server.ts constructor
constructor(logger?: Logger, options: MCPServerOptions = {}) {
  // ... initialization ...
  
  // Tool registration and setup
  this.setupTools();
}
```

## Benefits of Integrated Architecture

1. **Simplicity**: Single entry point, no configuration needed
2. **Consistency**: All tools enhanced uniformly
3. **Performance**: Shared session manager and AI service
4. **Maintainability**: Clear separation of concerns
5. **Extensibility**: Easy to add new tools or workflows
6. **Compatibility**: Backward compatible with standard MCP

## Testing

```bash
# Run all tests
npm test

# Run unit tests
npm run test:unit

# Run integration tests
npm run test:integration

# Test with coverage
npm run test:coverage
```

## AI Integration Architecture

The system implements sophisticated AI-powered automation through several layers:

### AI-Powered Tools (8 of 17 tools)

#### Primary AI Tools:
- **`generate-dockerfile`**: Multi-candidate generation with quality scoring and optimization
- **`generate-k8s-manifests`**: Resource estimation and security policy generation
- **`generate-aca-manifests`**: Azure-specific configuration optimization
- **`generate-helm-charts`**: Template structure optimization and dependency management
- **`convert-aca-to-k8s`**: Intelligent conversion between Azure and Kubernetes formats
- **`resolve-base-images`**: AI-powered base image recommendations
- **`build-image`**: AI-enhanced build process optimization
- **`scan`**: AI-augmented security analysis and recommendations

### Knowledge Base System

**30+ Knowledge Packs** organized by technology:
- Language packs (Node.js, Python, Java, Go, .NET, Ruby, Rust, PHP)
- Framework-specific packs (Blazor, gRPC, EF Core, SignalR)
- Platform packs (Kubernetes, Azure Container Apps, Database, Security)

**Knowledge Matching Algorithm**:
```typescript
function calculateKnowledgeScore(entry: KnowledgeEntry, context: AnalysisContext): number {
  let score = 0;

  // Pattern matching (highest weight)
  if (entry.pattern && entry._compiled?.pattern?.test(context.content)) {
    score += 30;
  }

  // Category, language, framework, and tag matching
  // ... scoring logic

  return score;
}
```

### Prompt System

**14 Prompt Templates** organized by category:
- Analysis prompts (repository enhancement)
- Containerization prompts (Docker generation/fixes)
- Orchestration prompts (Kubernetes/deployment manifests)
- Validation prompts (parameter suggestions)

**Template Structure**:
```json
{
  "id": "dockerfile-generation",
  "category": "containerization",
  "template": "Generate a Dockerfile for {{language}} project...",
  "variables": ["language", "framework", "dependencies"],
  "constraints": {"maxTokens": 2048, "format": "dockerfile"}
}
```

### Multi-Layered Fallback Strategy

1. **Tier 1**: AI-powered generation with knowledge enhancement
2. **Tier 2**: Rule-based generation with domain patterns
3. **Tier 3**: Template-based fallback with safe defaults

### Session-Aware Intelligence

Tools maintain context awareness through:
- Previous analysis results
- User preferences and patterns
- Tool execution history
- Cross-tool data dependencies

## Summary

The Containerization Assist MCP Server provides a comprehensive containerization solution with:
- 🚀 AI-powered Docker and Kubernetes workflows
- 🛠️ 17 specialized tools with co-location pattern
- 🔄 Session-aware state management
- 🎯 Intelligent workflow orchestration
- 📚 30+ knowledge packs with pattern matching
- 🤖 8 AI-enhanced tools with multi-tier fallback
- 📝 14 structured prompt templates
- ⚡ Result-based error handling throughout

The architecture provides a solid foundation for reliable, maintainable, and intelligent containerization automation.