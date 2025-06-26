# Unified MCP Server

The Unified MCP Server provides a comprehensive solution that combines both conversational chat capabilities and declarative workflow orchestration in a single, cohesive interface.

## Overview

The unified server architecture enables users to choose between three operational modes:

1. **Chat Mode**: Interactive conversational interface using the existing prompt manager
2. **Workflow Mode**: Declarative workflow execution with advanced orchestration
3. **Dual Mode**: Both chat and workflow capabilities available simultaneously

All modes share access to the same atomic tools, ensuring consistency and interoperability.

## Architecture

### Core Components

- **UnifiedMCPServer**: Main server that routes between chat and workflow modes
- **PromptManager**: Handles conversational interactions (from existing MCP system)
- **WorkflowOrchestrator**: Manages declarative workflow execution
- **ToolRegistry**: Centralized registry of all atomic tools
- **SessionManager**: Manages user sessions and state persistence
- **PreferenceStore**: Stores user preferences across sessions

### Key Features

#### 1. **Mode Selection**
Users can choose their preferred interaction mode:
- **Chat Mode**: Natural language interaction with AI guidance
- **Workflow Mode**: Declarative YAML/JSON workflow specifications
- **Dual Mode**: Switch between modes within the same session

#### 2. **Shared Atomic Tools**
All atomic tools are available in every mode:
- Repository analysis
- Docker operations (build, push, pull, tag)
- Security scanning
- Kubernetes operations
- Health checking

#### 3. **AI-Enhanced Capabilities**
Both modes maintain full AI capabilities:
- **Chat Mode**: Conversational AI with tool orchestration
- **Workflow Mode**: AI-driven error recovery and iterative fixing
- **Cross-Mode Context**: Shared session context between modes

#### 4. **Session Management**
Persistent sessions with:
- User preferences
- Conversation history
- Workflow state
- Resource tracking
- Checkpoint management

## Usage Examples

### Basic Server Setup

```go
package main

import (
    "github.com/Azure/container-kit/pkg/mcp/internal/server"
    "github.com/rs/zerolog"
    "go.etcd.io/bbolt"
)

func main() {
    logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

    // Open database
    db, err := bbolt.Open("/tmp/mcp.db", 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Create legacy orchestrator (from existing MCP system)
    legacyOrchestrator := createLegacyOrchestrator()

    // Create unified server in dual mode
    server, err := server.NewUnifiedMCPServer(
        db,
        legacyOrchestrator,
        logger,
        server.ModeDual,
    )
    if err != nil {
        panic(err)
    }

    // Server is ready to handle both chat and workflow requests
    startMCPServer(server)
}
```

### Chat Mode Usage

```go
ctx := context.Background()

// Interactive conversation
response, err := server.ExecuteTool(ctx, "chat", map[string]interface{}{
    "message": "I want to containerize my Node.js application",
    "session_id": "user-123",
})

// The chat tool will:
// 1. Analyze the request
// 2. Guide the user through the containerization process
// 3. Execute atomic tools as needed
// 4. Provide conversational feedback
```

### Workflow Mode Usage

```go
ctx := context.Background()

// Execute predefined workflow
response, err := server.ExecuteTool(ctx, "execute_workflow", map[string]interface{}{
    "workflow_name": "containerization-pipeline",
    "variables": map[string]string{
        "repo_url": "https://github.com/example/app",
        "registry": "myregistry.azurecr.io",
        "namespace": "production",
    },
    "options": map[string]interface{}{
        "dry_run": false,
        "checkpoints": true,
        "max_concurrency": 3,
    },
})

// Execute custom workflow
customWorkflow := map[string]interface{}{
    "apiVersion": "orchestration/v1",
    "kind": "Workflow",
    "metadata": map[string]interface{}{
        "name": "security-audit",
        "description": "Security-focused audit workflow",
    },
    "spec": map[string]interface{}{
        "stages": []map[string]interface{}{
            {
                "name": "security-scan",
                "tools": []string{"scan_image_security_atomic", "scan_secrets_atomic"},
                "parallel": true,
            },
        },
    },
}

response, err := server.ExecuteTool(ctx, "execute_workflow", map[string]interface{}{
    "workflow_spec": customWorkflow,
})
```

### Atomic Tool Direct Access

```go
// All atomic tools are available in any mode
response, err := server.ExecuteTool(ctx, "analyze_repository_atomic", map[string]interface{}{
    "session_id": "analysis-session",
    "repo_url": "https://github.com/example/python-app",
})

response, err := server.ExecuteTool(ctx, "build_image_atomic", map[string]interface{}{
    "session_id": "build-session",
    "image_name": "my-app",
    "tag": "v1.0.0",
})
```

## Server Modes

### Chat Mode (`ModeChat`)

**Available Tools:**
- `chat` - Interactive conversation interface
- `list_conversation_history` - View conversation history
- All atomic tools

**Use Cases:**
- Interactive exploration of containerization options
- Learning and discovery
- Complex decision-making with AI guidance
- Troubleshooting and debugging

**Benefits:**
- Natural language interaction
- AI-guided decision making
- Educational and exploratory
- Flexible and adaptive

### Workflow Mode (`ModeWorkflow`)

**Available Tools:**
- `execute_workflow` - Execute declarative workflows
- `list_workflows` - List available predefined workflows
- `get_workflow_status` - Check workflow execution status
- `pause_workflow` - Pause running workflows
- `resume_workflow` - Resume paused workflows
- `cancel_workflow` - Cancel running workflows
- All atomic tools

**Use Cases:**
- Production deployments
- CI/CD pipeline integration
- Automated operations
- Batch processing
- Standardized workflows

**Benefits:**
- Declarative configuration
- Reproducible results
- Parallel execution
- Advanced error handling
- Checkpoint/restore capabilities

### Dual Mode (`ModeDual`)

**Available Tools:**
- All chat mode tools
- All workflow mode tools
- All atomic tools

**Use Cases:**
- Development environments
- Training and experimentation
- Mixed interactive and automated workflows
- Transitioning between exploration and execution

**Benefits:**
- Maximum flexibility
- Seamless mode switching
- Shared session context
- Complete feature access

## Advanced Features

### 1. Server Capabilities Discovery

```go
capabilities := server.GetCapabilities()
// Returns:
// {
//   "chat_support": true,
//   "workflow_support": true,
//   "available_modes": ["chat", "workflow"],
//   "shared_tools": ["analyze_repository_atomic", "build_image_atomic", ...]
// }
```

### 2. Dynamic Tool Discovery

```go
tools := server.GetAvailableTools()
// Returns list of all tools available based on current server mode
```

### 3. Session Management

```go
// Get workflow status
status, err := server.ExecuteTool(ctx, "get_workflow_status", map[string]interface{}{
    "session_id": "workflow-session-123",
})

// List conversation history
history, err := server.ExecuteTool(ctx, "list_conversation_history", map[string]interface{}{
    "session_id": "chat-session-456",
    "limit": 10,
})
```

### 4. Workflow Management

```go
// List available workflows
workflows, err := server.ExecuteTool(ctx, "list_workflows", map[string]interface{}{
    "category": "security", // Optional filter
})

// Pause a running workflow
err := server.ExecuteTool(ctx, "pause_workflow", map[string]interface{}{
    "session_id": "workflow-session-789",
})

// Resume a paused workflow
result, err := server.ExecuteTool(ctx, "resume_workflow", map[string]interface{}{
    "session_id": "workflow-session-789",
})
```

## AI-Driven Features

### Iterative Fixing in Both Modes

#### Chat Mode AI Fixing
- Conversational error analysis
- Interactive solution guidance
- Step-by-step fix implementation
- Learning from user feedback

#### Workflow Mode AI Fixing
- Automatic error detection and analysis
- AI-driven recovery strategies
- Cross-stage context awareness
- Intelligent checkpoint placement

### Enhanced Context Sharing

The unified server provides richer context for AI operations:
- **Session Context**: Persistent state across interactions
- **Cross-Mode Context**: Share insights between chat and workflow modes
- **Tool Context**: Results from previous tool executions
- **User Preferences**: Personalized defaults and preferences

## Integration Patterns

### 1. Existing MCP Integration

The unified server integrates seamlessly with existing MCP components:

```go
// Use existing prompt manager and tool orchestrator
server, err := NewUnifiedMCPServer(
    db,
    existingToolOrchestrator, // From current MCP system
    logger,
    ModeDual,
)
```

### 2. Progressive Migration

Organizations can migrate gradually:

1. **Start with Chat Mode**: Keep existing conversational interface
2. **Add Workflow Capabilities**: Enable workflow mode for automation
3. **Unified Experience**: Switch to dual mode for maximum flexibility

### 3. API Integration

The unified server can be exposed through various interfaces:
- MCP protocol for AI model integration
- REST API for external systems
- WebSocket for real-time interactions
- CLI for command-line usage

## Configuration Options

### Server Configuration

```go
type ServerConfig struct {
    Mode            ServerMode        // Chat, Workflow, or Dual
    DatabasePath    string           // Path to BoltDB database
    SessionConfig   SessionConfig    // Session management settings
    WorkflowConfig  WorkflowConfig   // Workflow execution settings
    SecurityConfig  SecurityConfig   // Security and authentication
    Logger          zerolog.Logger   // Logging configuration
}
```

### Session Configuration

```go
type SessionConfig struct {
    WorkspaceDir      string        // Session workspace directory
    MaxSessions       int           // Maximum concurrent sessions
    SessionTTL        time.Duration // Session time-to-live
    MaxDiskPerSession int64         // Disk quota per session
    TotalDiskLimit    int64         // Total disk usage limit
}
```

### Workflow Configuration

```go
type WorkflowConfig struct {
    MaxConcurrency    int           // Maximum parallel stage execution
    DefaultTimeout    time.Duration // Default stage timeout
    CheckpointEnabled bool          // Enable automatic checkpoints
    ErrorRetryLimit   int           // Maximum error retry attempts
}
```

## Best Practices

### 1. Mode Selection Guidelines

**Choose Chat Mode when:**
- Users need guidance and exploration
- Requirements are unclear or evolving
- Learning and experimentation are priorities
- Interactive decision-making is required

**Choose Workflow Mode when:**
- Requirements are well-defined
- Automation and repeatability are critical
- Production deployments are the goal
- Integration with CI/CD systems is needed

**Choose Dual Mode when:**
- Development and experimentation environments
- Mixed use cases within the same organization
- Training and onboarding scenarios
- Maximum flexibility is required

### 2. Session Management

- Use meaningful session IDs for tracking
- Implement session cleanup for resource management
- Leverage persistent preferences for user experience
- Monitor session metrics for optimization

### 3. Error Handling

- Implement comprehensive error logging
- Use structured error responses
- Leverage AI-driven error recovery
- Provide clear user feedback

### 4. Performance Optimization

- Use parallel execution in workflow mode
- Implement connection pooling for databases
- Cache frequently used data
- Monitor resource usage

## Monitoring and Metrics

The unified server provides comprehensive metrics:

### Server Metrics
- Active sessions
- Tool execution counts
- Error rates
- Response times

### Chat Mode Metrics
- Conversation turns
- Tool invocations
- User satisfaction
- Resolution rates

### Workflow Mode Metrics
- Workflow executions
- Success rates
- Stage performance
- Resource utilization

### Atomic Tool Metrics
- Individual tool performance
- Error patterns
- Usage statistics
- Resource consumption

## Security Considerations

### 1. Session Security
- Encrypted session storage
- Secure session ID generation
- Session timeout management
- Access control validation

### 2. Tool Security
- Input validation for all tools
- Resource quota enforcement
- Sandboxed execution environments
- Audit logging

### 3. Data Protection
- Encrypted sensitive data storage
- Secure inter-component communication
- Privacy-compliant logging
- Data retention policies

## Future Enhancements

### 1. Advanced AI Features
- Multi-modal AI interactions
- Predictive workflow suggestions
- Automated optimization recommendations
- Learning from usage patterns

### 2. Enhanced Integration
- GraphQL API support
- Webhook integrations
- Event-driven architectures
- Cloud-native deployments

### 3. Enterprise Features
- Multi-tenancy support
- Role-based access control
- Enterprise SSO integration
- Compliance reporting

### 4. Performance Improvements
- Distributed execution
- Advanced caching strategies
- Real-time optimization
- Resource elasticity

The Unified MCP Server represents a significant evolution in containerization automation, providing users with the flexibility to choose between conversational AI guidance and declarative workflow automation while maintaining all the powerful capabilities of the underlying atomic tools.
