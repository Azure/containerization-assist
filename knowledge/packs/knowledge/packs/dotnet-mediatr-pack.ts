export default [
  {
    "id": "mediatr-base-setup",
    "category": "dockerfile",
    "pattern": "MediatR",
    "recommendation": "Use Alpine-based .NET images with proper MediatR CQRS configuration",
    "example": "FROM mcr.microsoft.com/dotnet/sdk:8.0-alpine AS build\nWORKDIR /app\nCOPY *.csproj ./\nRUN dotnet restore\nCOPY . ./\nRUN dotnet publish -c Release -o out\n\nFROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine\nWORKDIR /app\nCOPY --from=build /app/out .\nEXPOSE 8080\nENTRYPOINT [\"dotnet\", \"App.dll\"]",
    "severity": "high",
    "tags": [
      "alpine",
      "cqrs",
      "dotnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "mediatr",
      "microsoft"
    ],
    "description": "Basic MediatR CQRS application containerization with Alpine for reduced image size"
  },
  {
    "id": "mediatr-microservice-architecture",
    "category": "dockerfile",
    "pattern": "MediatR.*Microsoft\\.Extensions\\.ServiceDiscovery",
    "recommendation": "Implement microservice with MediatR, service discovery, and health checks",
    "example": "FROM mcr.microsoft.com/dotnet/sdk:8.0-alpine AS build\nWORKDIR /src\nCOPY *.sln ./\nCOPY */*.csproj ./\nRUN for file in $(ls *.csproj); do mkdir -p ${file%.*}/ && mv $file ${file%.*}/; done\nRUN dotnet restore\nCOPY . ./\nRUN dotnet publish -c Release -o /app/publish\n\nFROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine\nRUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup\nCOPY --from=build /app/publish .\nUSER appuser\nHEALTHCHECK --interval=30s --timeout=3s CMD curl -f http://localhost:8080/health || exit 1",
    "severity": "high",
    "tags": [
      "dotnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "health-check",
      "mediatr",
      "microservice",
      "microsoft",
      "service-discovery"
    ],
    "description": "Microservice architecture with MediatR, service discovery, and container health checks"
  },
  {
    "id": "mediatr-event-sourcing",
    "category": "dockerfile",
    "pattern": "MediatR.*EventStore",
    "recommendation": "Configure EventStore with MediatR for event sourcing patterns",
    "example": "ENV EventStore__ConnectionString=\"${EVENTSTORE_CONNECTION}\" \\\n    EventStore__DefaultCredentials__Username=\"${EVENTSTORE_USER}\" \\\n    EventStore__DefaultCredentials__Password=\"${EVENTSTORE_PASS}\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "event-sourcing",
      "eventstore",
      "fix-dockerfile",
      "generate-dockerfile",
      "mediatr"
    ],
    "description": "Event sourcing configuration with MediatR and EventStore using environment variables"
  },
  {
    "id": "mediatr-saga-orchestration",
    "category": "dockerfile",
    "pattern": "MediatR.*MassTransit.*Saga",
    "recommendation": "Configure saga orchestration with MediatR and MassTransit",
    "example": "FROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine\nRUN apk add --no-cache icu-libs\nENV DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=false \\\n    MessageBroker__Host=\"${RABBITMQ_HOST}\" \\\n    SagaState__Provider=\"Redis\" \\\n    SagaState__ConnectionString=\"${REDIS_CONNECTION}\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "masstransit",
      "mediatr",
      "microsoft",
      "orchestration",
      "saga"
    ],
    "description": "Saga orchestration with MediatR, MassTransit, and Redis state persistence"
  },
  {
    "id": "mediatr-cqrs-separation",
    "category": "configuration",
    "pattern": "ICommand.*IQuery.*ReadModel.*WriteModel",
    "recommendation": "Implement CQRS with separate read/write database connections",
    "example": "ENV ConnectionStrings__WriteDatabase=\"${WRITE_DB_CONNECTION}\" \\\n    ConnectionStrings__ReadDatabase=\"${READ_DB_CONNECTION}\" \\\n    CQRS__CommandTimeout=\"30\" \\\n    CQRS__QueryTimeout=\"10\" \\\n    CQRS__EnableQueryCaching=\"true\"",
    "severity": "high",
    "tags": [
      "dotnet",
      "mediatr",
      "cqrs",
      "database",
      "separation"
    ],
    "description": "CQRS pattern with read/write database separation and query caching"
  },
  {
    "id": "mediatr-performance-optimization",
    "category": "optimization",
    "pattern": "MediatR.*IPipelineBehavior",
    "recommendation": "Enable ReadyToRun and tiered compilation for MediatR performance",
    "example": "RUN dotnet publish -c Release -o /app/publish \\\n    --runtime linux-musl-x64 \\\n    -p:PublishReadyToRun=true\n\nENV DOTNET_TieredPGO=1 \\\n    DOTNET_TC_QuickJitForLoops=1 \\\n    DOTNET_ReadyToRun=1 \\\n    DOTNET_gcServer=1",
    "severity": "medium",
    "tags": [
      "dotnet",
      "generate-dockerfile",
      "mediatr",
      "optimization",
      "performance",
      "readytorun"
    ],
    "description": "Performance optimizations for MediatR with ReadyToRun and tiered compilation"
  },
  {
    "id": "mediatr-distributed-cache",
    "category": "caching",
    "pattern": "MediatR.*StackExchangeRedis",
    "recommendation": "Configure Redis distributed caching for MediatR queries",
    "example": "ENV ConnectionStrings__Redis=\"${REDIS_CONNECTION}\" \\\n    DistributedCache__DefaultExpiration=\"300\" \\\n    QueryCache__Enabled=\"true\" \\\n    QueryCache__MaxMemorySize=\"100MB\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "mediatr",
      "redis",
      "caching",
      "distributed"
    ],
    "description": "Distributed caching with Redis for MediatR query optimization"
  },
  {
    "id": "mediatr-validation-pipeline",
    "category": "validation",
    "pattern": "MediatR.*FluentValidation",
    "recommendation": "Implement FluentValidation pipeline behavior in MediatR",
    "example": "ENV Validation__StopOnFirstFailure=\"false\" \\\n    Validation__IncludeErrorDetails=\"true\" \\\n    Validation__ValidateChildProperties=\"true\" \\\n    Logging__LogLevel__Validation=\"Information\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "mediatr",
      "validation",
      "fluentvalidation",
      "pipeline"
    ],
    "description": "MediatR with FluentValidation pipeline for request validation"
  },
  {
    "id": "mediatr-outbox-pattern",
    "category": "reliability",
    "pattern": "MediatR.*OutboxPattern.*DomainEvent",
    "recommendation": "Implement outbox pattern for reliable messaging with MediatR",
    "example": "ENV Outbox__ProcessingInterval=\"5000\" \\\n    Outbox__BatchSize=\"100\" \\\n    Outbox__RetryCount=\"3\" \\\n    MessageBroker__Host=\"${RABBITMQ_HOST}\" \\\n    ConnectionStrings__DefaultConnection=\"${DB_CONNECTION}\"",
    "severity": "high",
    "tags": [
      "dotnet",
      "mediatr",
      "outbox",
      "reliability",
      "messaging"
    ],
    "description": "Outbox pattern implementation for reliable domain event publishing"
  },
  {
    "id": "mediatr-observability",
    "category": "generic",
    "pattern": "MediatR.*OpenTelemetry",
    "recommendation": "Configure OpenTelemetry observability for MediatR handlers",
    "example": "ENV OTEL_EXPORTER_OTLP_ENDPOINT=\"${JAEGER_ENDPOINT}\" \\\n    OTEL_SERVICE_NAME=\"mediatr-service\" \\\n    Metrics__Enabled=\"true\" \\\n    Metrics__CommandHandlerMetrics=\"true\" \\\n    Metrics__QueryHandlerMetrics=\"true\"",
    "severity": "high",
    "tags": [
      "dotnet",
      "mediatr",
      "observability",
      "opentelemetry",
      "monitoring"
    ],
    "description": "Comprehensive monitoring and observability for MediatR with OpenTelemetry"
  },
  {
    "id": "mediatr-multi-tenant",
    "category": "architecture",
    "pattern": "MediatR.*ITenantContext.*TenantResolver",
    "recommendation": "Configure multi-tenant isolation for MediatR applications",
    "example": "ENV MultiTenancy__Strategy=\"Subdomain\" \\\n    MultiTenancy__RequireTenant=\"true\" \\\n    ConnectionStrings__Template=\"Server=${DB_HOST};Database=App_{tenant};User Id=${DB_USER};Password=${DB_PASS}\" \\\n    TenantIsolation__DataIsolation=\"Database\"",
    "severity": "high",
    "tags": [
      "dotnet",
      "mediatr",
      "multi-tenant",
      "isolation",
      "architecture"
    ],
    "description": "Multi-tenant MediatR application with database-per-tenant isolation"
  },
  {
    "id": "mediatr-streaming-queries",
    "category": "streaming",
    "pattern": "MediatR.*IStreamRequest.*IAsyncEnumerable",
    "recommendation": "Configure streaming support for large dataset queries",
    "example": "ENV Streaming__BufferSize=\"1000\" \\\n    Streaming__MaxConcurrentStreams=\"10\" \\\n    DOTNET_gcServer=1 \\\n    DOTNET_GCHighMemPercent=75 \\\n    Kestrel__Limits__MaxRequestBodySize=\"2147483648\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "mediatr",
      "streaming",
      "async",
      "performance"
    ],
    "description": "Streaming query support for handling large datasets with MediatR"
  },
  {
    "id": "mediatr-security-scanning",
    "category": "security",
    "pattern": "MediatR",
    "recommendation": "Add security scanning to MediatR Docker builds",
    "example": "# Security scanning stage\nFROM build AS security\nRUN dotnet list package --vulnerable --include-transitive\nRUN dotnet tool install -g dotnet-outdated-tool && \\\n    ~/.dotnet/tools/dotnet-outdated\n\n# Scan final image with Trivy\nRUN apk add --no-cache curl && \\\n    curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin && \\\n    trivy fs --exit-code 1 --no-progress /app",
    "severity": "high",
    "tags": [
      "dotnet",
      "fix-dockerfile",
      "mediatr",
      "scan-image",
      "scanning",
      "security",
      "vulnerabilities"
    ],
    "description": "Security vulnerability scanning for MediatR applications"
  },
  {
    "id": "mediatr-dependency-injection",
    "category": "configuration",
    "pattern": "MediatR\\.Extensions\\.Microsoft\\.DependencyInjection",
    "recommendation": "Configure MediatR with proper service lifetime management",
    "example": "ENV MediatR__ServiceLifetime=\"Scoped\" \\\n    MediatR__RequestTimeout=\"30000\" \\\n    DI__ValidateScopes=\"true\" \\\n    DI__ValidateOnBuild=\"true\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "mediatr",
      "dependency-injection",
      "configuration"
    ],
    "description": "Proper dependency injection configuration for MediatR services"
  },
  {
    "id": "mediatr-retry-policies",
    "category": "resilience",
    "pattern": "MediatR.*Polly",
    "recommendation": "Implement retry and circuit breaker policies with Polly",
    "example": "ENV Resilience__RetryCount=\"3\" \\\n    Resilience__RetryDelay=\"1000\" \\\n    Resilience__CircuitBreakerThreshold=\"5\" \\\n    Resilience__CircuitBreakerDuration=\"30000\"",
    "severity": "high",
    "tags": [
      "dotnet",
      "mediatr",
      "resilience",
      "polly",
      "retry"
    ],
    "description": "Resilience patterns with Polly for MediatR handlers"
  },
  {
    "id": "mediatr-batch-processing",
    "category": "generic",
    "pattern": "MediatR.*Batch.*Processing",
    "recommendation": "Configure batch processing for MediatR commands",
    "example": "ENV BatchProcessing__MaxBatchSize=\"100\" \\\n    BatchProcessing__ProcessingInterval=\"5000\" \\\n    BatchProcessing__MaxConcurrency=\"5\" \\\n    BatchProcessing__EnableAutoRetry=\"true\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "mediatr",
      "batch",
      "performance",
      "processing"
    ],
    "description": "Batch processing configuration for high-throughput MediatR operations"
  },
  {
    "id": "mediatr-api-versioning",
    "category": "api",
    "pattern": "MediatR.*ApiVersion",
    "recommendation": "Implement API versioning with MediatR handlers",
    "example": "ENV ApiVersioning__DefaultVersion=\"1.0\" \\\n    ApiVersioning__AssumeDefaultVersionWhenUnspecified=\"true\" \\\n    ApiVersioning__ReportApiVersions=\"true\" \\\n    ApiVersioning__HeaderName=\"api-version\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "mediatr",
      "api",
      "versioning"
    ],
    "description": "API versioning strategy for MediatR-based APIs"
  },
  {
    "id": "mediatr-rate-limiting",
    "category": "security",
    "pattern": "MediatR.*RateLimiting",
    "recommendation": "Configure rate limiting for MediatR command handlers",
    "example": "ENV RateLimiting__PermitLimit=\"100\" \\\n    RateLimiting__Window=\"60\" \\\n    RateLimiting__QueueLimit=\"50\" \\\n    RateLimiting__SegmentsPerWindow=\"6\"",
    "severity": "high",
    "tags": [
      "dotnet",
      "fix-dockerfile",
      "mediatr",
      "rate-limiting",
      "scan-image",
      "security",
      "throttling"
    ],
    "description": "Rate limiting configuration for MediatR command handlers"
  },
  {
    "id": "mediatr-grpc-integration",
    "category": "generic",
    "pattern": "MediatR.*Grpc",
    "recommendation": "Integrate MediatR with gRPC services",
    "example": "ENV Kestrel__Endpoints__Grpc__Url=\"http://+:5000\" \\\n    Kestrel__Endpoints__Grpc__Protocols=\"Http2\" \\\n    Grpc__MaxReceiveMessageSize=\"4194304\" \\\n    Grpc__MaxSendMessageSize=\"4194304\"",
    "severity": "medium",
    "tags": [
      "dotnet",
      "mediatr",
      "grpc",
      "integration",
      "microservices"
    ],
    "description": "gRPC service integration with MediatR for inter-service communication"
  },
  {
    "id": "mediatr-feature-flags",
    "category": "features",
    "pattern": "MediatR.*FeatureManagement",
    "recommendation": "Implement feature flags with MediatR handlers",
    "example": "ENV FeatureManagement__NewFeature=\"true\" \\\n    FeatureManagement__BetaFeatures=\"false\" \\\n    FeatureManagement__RefreshInterval=\"60\" \\\n    FeatureManagement__Provider=\"AzureAppConfig\"",
    "severity": "low",
    "tags": [
      "azure",
      "configuration",
      "dotnet",
      "feature-flags",
      "mediatr"
    ],
    "description": "Feature flag management for MediatR handler behavior"
  }
] as const;
