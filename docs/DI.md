# Dependency Injection (DI) in the MCP Codebase

## Introduction to the Dependency Injection Pattern

Dependency Injection (DI) is a design pattern in which an object’s required dependencies are provided (or "injected") from the outside rather than the object creating them itself. In practice, this means instead of a component instantiating its own collaborators, those collaborators are passed in, often through the constructor (constructor injection) or via setters or interface methods. This approach **inverts the control** of dependency creation: a central piece of code (often called an *injector* or *container*) is responsible for building and supplying dependencies, rather than individual components building their own.

**Key benefits of the DI pattern include:**

- **Loose Coupling:** By depending on abstractions (interfaces) rather than concrete classes, components are decoupled from specific implementations. This makes it easy to swap out or modify components without affecting consumers.  
- **Enhanced Testability:** Because dependencies can be injected, it becomes straightforward to provide mock or stub implementations during testing. Components under test don’t need to construct real services (e.g. databases or network clients) – instead, you inject lightweight fakes.  
- **Modularity and Maintainability:** Systems built with DI encourage a clear separation of concerns. Components focus on their core logic, while configuration and assembly of dependencies are handled externally. This modular design makes it easier to extend and maintain code, as new features or implementations can be integrated by adjusting the wiring rather than rewriting core logic.  
- **Configurability and Reuse:** DI allows different configurations of a system (for example, using a different data store or logging mechanism) by changing how dependencies are wired. The same component can be reused in different contexts by injecting appropriate implementations.

In modern software engineering, DI is a cornerstone of many architectural patterns and frameworks. For example, frameworks in languages like Java or C# often provide *IoC (Inversion of Control) containers* that automatically instantiate and inject dependencies. Even in lightweight environments, developers apply DI principles by manually passing dependencies. The ultimate goal is to achieve code that is easier to reason about, swap out, and test in isolation.

## Dependency Injection in Go

Go does not have a built-in DI framework, but the language’s emphasis on simplicity and interfaces makes DI feasible through straightforward patterns:

- **Constructor Injection:** The most common approach in Go is to explicitly pass dependencies via function or method parameters (often through constructors or initializer functions). For example:
  ```go
  func NewA(logger Logger, db Database) *A { ... }
  ```
  ensures that when creating `A`, the appropriate `Logger` and `Database` implementations are provided.  
- **Interfaces for Decoupling:** Go’s interfaces are central to DI. By defining interfaces for the needed functionality (e.g. a `Database` interface with needed methods), and using those in function signatures, Go programs achieve dependency inversion. Higher-level code relies on interface types, and at runtime you inject a concrete type that implements that interface.  
- **Functional Options Pattern:** In Go, another DI-related technique is using **functional options** to configure components. This pattern uses variadic option functions to set up or override certain dependencies or settings. The MCP codebase uses this pattern for runtime overrides (as we'll see with `WithCoreServices` and related functions) to inject alternative service implementations at startup.  
- **DI Libraries and Code Generation:** While manual injection is common, there are tools to automate wiring in larger Go projects. One prominent tool is Google’s **Wire**, a compile-time dependency injection code generator. Wire allows developers to specify how to construct various types (via provider functions and structs), then automatically generates code that glues everything together. This avoids the need for a runtime container – instead, Wire produces static code to create and connect dependencies.

## DI in the MCP Codebase: Architecture and Patterns

The `pkg/mcp` module follows a layered architecture and uses Dependency Injection to connect these layers in a maintainable way. The design adheres to a **Clean Architecture** style with clear separation of concerns among layers: **Domain**, **Application**, **Infrastructure**, and a **Composition Root** for wiring. DI is the mechanism that binds these layers together while keeping them decoupled:

- **Domain Layer:**  
  Defines core business interfaces and types, completely independent of any infrastructure. Interfaces like `domainevents.Publisher` for event publishing or `domainsampling.UnifiedSampler` for sampling are pure abstractions. (See code in `pkg/mcp/domain/...`.)

- **Application Layer:**  
  Orchestrates high-level operations using domain interfaces.  
  - **GroupedDependencies:** Structs grouping related services into categories (Core, Workflow, Persistence, AI).  
  - **Service Interfaces:** `CoreServices`, `PersistenceServices`, `WorkflowServices`, `AIServices`, and `AllServices` (aggregates all).  
  - **NewServiceProvider:** Constructor that wraps a `*GroupedDependencies` into an `AllServices` interface.  
  - **Legacy Support:** Conversion functions between the new grouped approach and the old `Dependencies` struct.

  ```go
  type GroupedDependencies struct {
      Core        CoreDeps
      Workflow    WorkflowDeps
      Persistence PersistenceDeps
      AI          AIDeps
  }

  type AllServices interface {
      CoreServices
      PersistenceServices
      WorkflowServices
      AIServices
  }

  func NewServiceProvider(grouped *GroupedDependencies) AllServices {
      return &serviceProvider{grouped: grouped}
  }
  ```

- **Infrastructure Layer:**  
  Implements the domain interfaces with concrete types and defines **Wire provider sets**:  
  - `infrastructure/core.Providers`  
  - `infrastructure/messaging.Providers` with `wire.Bind` to bind to `domainevents.Publisher`  
  - `infrastructure/ai_ml.Providers` binding to ML interfaces  
  - Each subpackage exposes a `Providers` variable aggregating constructors and `wire.Bind` calls.

- **Composition Root (Wiring Layer):**  
  In `pkg/mcp/composition`:  
  - **Provider Sets Aggregation:** `InfrastructureProviders`, `ApplicationProviders`, and `AllProviders`.  
  - **Wire Injector Function:** `InitializeServer(logger, config) (api.MCPServer, error)` with `wire.Build(AllProviders)`.  
  - **Generated Code:** `wire_gen.go` calls provider functions in order, builds `*Dependencies`, then calls `application.ProvideServer`.

  ```go
  // InitializeServer is the entry point for Wire code generation.
  func InitializeServer(logger *slog.Logger, config workflow.ServerConfig) (api.MCPServer, error) {
      wire.Build(AllProviders)
      return nil, nil
  }
  ```

- **Example: ProvideWorkflowDeps**  
  Decorates orchestrator with events and saga:

  ```go
  func ProvideWorkflowDeps(
      orchestrator workflow.WorkflowOrchestrator,
      eventPublisher domainevents.Publisher,
      progressEmitterFactory workflow.ProgressEmitterFactory,
      sagaCoordinator *saga.SagaCoordinator,
      logger *slog.Logger,
  ) WorkflowDeps {
      // Wrap base orchestrator with event publishing
      eventAwareOrchestrator := workflow.WithEvents(orchestrator, eventPublisher)
      // Add saga management
      sagaAwareOrchestrator := workflow.WithSaga(eventAwareOrchestrator, sagaCoordinator, logger)

      return WorkflowDeps{
          Orchestrator:           orchestrator,
          EventAwareOrchestrator: eventAwareOrchestrator,
          SagaAwareOrchestrator:  sagaAwareOrchestrator,
          EventPublisher:         eventPublisher,
          ProgressEmitterFactory: progressEmitterFactory,
          SagaCoordinator:        sagaCoordinator,
      }
  }
  ```

- **Runtime Overrides with Functional Options:**  
  In `pkg/mcp/server_options.go`, functions like `WithCoreServices`, `WithPersistenceServices`, `WithWorkflowServices`, and `WithAIServices` allow runtime injection of custom implementations, useful for testing or environment-specific overrides.

## Conclusion: DI’s Value in the MCP Module

Dependency Injection in `pkg/mcp`:

- **Extensibility:** New services or implementations can be added without touching core logic—just add providers and bindings.  
- **Testability:** Components can be tested in isolation by injecting mocks via options or custom `AllServices`.  
- **Clean Architecture:** Strict layer boundaries—domain defines interfaces, application composes, infrastructure implements, composition root wires.  
- **Performance & Clarity:** Wire-generated DI occurs at compile time without runtime reflection, making dependency wiring explicit and easy to trace.

The DI pattern ensures each part of the system knows **what** it needs but not **how** it’s provided. The *how* is centralized in the composition root, leading to a codebase that is robust, maintainable, and easy for new developers to onboard.

## Sources

- MCP Codebase – `pkg/mcp/application` and `pkg/mcp/composition` packages (``, ``)  
- MCP Codebase – `pkg/mcp/server_options.go` (``, ``)  
- MCP Architecture Documentation (``, ``, ``)

