# Container Kit Architecture Diagrams

## Simplified Architecture Overview

```mermaid
graph TB
    %% External layer
    subgraph "External Clients"
        Claude[Claude Desktop]
        CLI[MCP CLI]
        HTTP[HTTP Clients]
    end

    %% Composition Root
    subgraph "Composition Root"
        Wire[Wire DI]
        Providers[Provider Functions]
    end

    %% 4-Layer Architecture
    subgraph "API Layer"
        Interfaces[Pure Interfaces<br/>No Implementation]
    end

    subgraph "Application Layer"
        Server[MCP Server]
        Handlers[Tool Handlers]
        Session[Session Service]
    end

    subgraph "Domain Layer"
        Workflow[Workflow Orchestrator]
        Events[Domain Events]
        ErrorCtx[Error Context]
        Progress[Progress Tracking]
    end

    subgraph "Infrastructure Layer"
        Steps[10 Workflow Steps]
        AI[AI/ML Integration]
        Docker[Docker/K8s]
        Storage[BoltDB Storage]
    end

    %% Connections
    Claude --> Server
    CLI --> Server
    HTTP --> Server
    
    Wire --> Server
    Wire --> Workflow
    
    Server --> Handlers
    Handlers --> Workflow
    Server --> Session
    
    Workflow --> Events
    Workflow --> ErrorCtx
    Workflow --> Progress
    
    Workflow --> Steps
    Steps --> AI
    Steps --> Docker
    Session --> Storage

    %% Styling
    classDef external fill:#f9f,stroke:#333,stroke-width:2px
    classDef composition fill:#fdd,stroke:#333,stroke-width:2px
    classDef api fill:#dfd,stroke:#333,stroke-width:2px
    classDef app fill:#ddf,stroke:#333,stroke-width:2px
    classDef domain fill:#fdf,stroke:#333,stroke-width:2px
    classDef infra fill:#ffd,stroke:#333,stroke-width:2px
    
    class Claude,CLI,HTTP external
    class Wire,Providers composition
    class Interfaces api
    class Server,Handlers,Session app
    class Workflow,Events,ErrorCtx,Progress domain
    class Steps,AI,Docker,Storage infra
```

## Workflow Execution Flow

```mermaid
graph LR
    %% Single Workflow Tool
    subgraph "containerize_and_deploy"
        Start[Start Workflow]
        S1[1. Analyze<br/>Repository]
        S2[2. Generate<br/>Dockerfile]
        S3[3. Build<br/>Image]
        S4[4. Scan<br/>Security]
        S5[5. Tag<br/>Image]
        S6[6. Push<br/>Registry]
        S7[7. Generate<br/>Manifests]
        S8[8. Setup<br/>Cluster]
        S9[9. Deploy<br/>App]
        S10[10. Verify<br/>Health]
        End[Complete]
    end

    Start --> S1
    S1 --> S2
    S2 --> S3
    S3 --> S4
    S4 --> S5
    S5 --> S6
    S6 --> S7
    S7 --> S8
    S8 --> S9
    S9 --> S10
    S10 --> End

    %% Progress tracking
    Progress[Progress: 1/10...10/10]
    S1 -.-> Progress
    S10 -.-> Progress

    %% Error recovery
    Error[Progressive Error Context<br/>AI-Assisted Recovery]
    S3 -.-> Error
    S9 -.-> Error
    Error -.-> S3
    Error -.-> S9

    %% Styling
    classDef step fill:#e3f2fd,stroke:#1976d2,stroke-width:2px
    classDef progress fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px
    classDef error fill:#ffebee,stroke:#c62828,stroke-width:2px
    
    class S1,S2,S3,S4,S5,S6,S7,S8,S9,S10 step
    class Progress progress
    class Error error
```

## Decorator Pattern for Orchestration

```mermaid
graph TB
    %% Base and decorators
    Base[Base Orchestrator<br/>Core Workflow Logic]
    Event[Event Decorator<br/>Publishes Domain Events]
    Saga[Saga Decorator<br/>Distributed Transactions]
    Metrics[Metrics Decorator<br/>Performance Tracking]
    Retry[Retry Decorator<br/>Error Recovery]
    Trace[Trace Decorator<br/>Distributed Tracing]
    
    %% Composition
    Base --> Event
    Event --> Saga
    Saga --> Metrics
    Metrics --> Retry
    Retry --> Trace
    
    %% Final orchestrator
    Final[Fully Decorated<br/>Orchestrator]
    Trace --> Final
    
    %% Usage
    Client[MCP Client]
    Client --> Final
    
    %% Styling
    classDef base fill:#e8f5e9,stroke:#4caf50,stroke-width:2px
    classDef decorator fill:#e3f2fd,stroke:#2196f3,stroke-width:2px
    classDef final fill:#f3e5f5,stroke:#9c27b0,stroke-width:2px
    
    class Base base
    class Event,Saga,Metrics,Retry,Trace decorator
    class Final final
```

## Error Recovery Flow

```mermaid
stateDiagram-v2
    [*] --> ExecuteStep
    ExecuteStep --> Success: No Error
    ExecuteStep --> ErrorOccurred: Error
    
    ErrorOccurred --> AddToContext: Record Error
    AddToContext --> CheckPattern: Analyze Pattern
    
    CheckPattern --> ShouldRetry: Retry Possible
    CheckPattern --> ShouldEscalate: Too Many Errors
    
    ShouldRetry --> AIAnalysis: Get AI Suggestion
    AIAnalysis --> ApplyFix: Apply Fix
    ApplyFix --> ExecuteStep: Retry
    
    ShouldEscalate --> HumanIntervention: Escalate
    HumanIntervention --> [*]
    
    Success --> [*]
    
    note right of AddToContext
        Progressive Error Context:
        - Error message
        - Step context
        - Attempt count
        - Previous fixes
    end note
    
    note right of AIAnalysis
        AI-Assisted Recovery:
        - Analyzes error pattern
        - Suggests fixes
        - Updates Dockerfile/config
    end note
```

## Key Architecture Patterns

### 1. Composition Root
- All dependency injection outside business layers
- Wire-based compile-time DI
- Clean separation of wiring from logic

### 2. Decorator Pattern
- Base orchestrator with pure business logic
- Decorators add cross-cutting concerns
- Composable and testable

### 3. Progressive Error Context
- Accumulates error history
- AI-assisted recovery suggestions
- Automatic escalation logic

### 4. Single Workflow Architecture
- One tool: `containerize_and_deploy`
- 10 structured steps with progress
- Built-in error recovery

### 5. Clean Architecture
- Strict layer boundaries
- Dependency rule: outer depends on inner
- Domain logic isolated from infrastructure

## File Structure

```
pkg/mcp/
├── composition/            # Composition root (outside layers)
│   ├── providers.go
│   ├── server.go
│   └── wire_gen.go
├── api/                   # Interface layer
│   └── interfaces.go
├── application/           # Application services
│   ├── server.go
│   ├── session/
│   └── providers.go
├── domain/                # Business logic
│   ├── workflow/
│   │   ├── orchestrator.go
│   │   ├── decorators.go
│   │   └── error_context.go
│   └── events/
└── infrastructure/        # Technical implementations
    ├── ai_ml/
    ├── orchestration/
    │   └── steps/         # 10 workflow steps
    └── persistence/
```