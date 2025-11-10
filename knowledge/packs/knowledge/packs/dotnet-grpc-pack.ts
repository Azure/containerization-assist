export default [
  {
    "id": "grpc-http2-configuration",
    "category": "dockerfile",
    "pattern": "gRPC|Grpc\\.AspNetCore",
    "recommendation": "Configure HTTP/2 explicitly for gRPC services in containerized environments",
    "example": "FROM mcr.microsoft.com/dotnet/aspnet:8.0\nWORKDIR /app\nCOPY . .\nEXPOSE 80 443\n# Configure for HTTP/2\nENV ASPNETCORE_URLS=\"https://+:443;http://+:80\"\nENV ASPNETCORE_Kestrel__Protocols=\"Http2\"",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "http2",
      "kestrel",
      "microsoft",
      "protocols"
    ],
    "description": "gRPC requires HTTP/2 protocol configuration in container environments"
  },
  {
    "id": "grpc-tls-termination",
    "category": "dockerfile",
    "pattern": "gRPC.*TLS|grpc.*ssl",
    "recommendation": "Configure TLS termination properly for gRPC services behind load balancers",
    "example": "// In Program.cs\nbuilder.WebHost.ConfigureKestrel(options =>\n{\n    options.ListenAnyIP(80, listenOptions =>\n    {\n        listenOptions.Protocols = HttpProtocols.Http2;\n    });\n    options.ListenAnyIP(443, listenOptions =>\n    {\n        listenOptions.Protocols = HttpProtocols.Http2;\n        listenOptions.UseHttps();\n    });\n});",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "load-balancer",
      "ssl",
      "tls"
    ],
    "description": "gRPC TLS configuration is critical for secure communication in container deployments"
  },
  {
    "id": "grpc-health-checks",
    "category": "dockerfile",
    "pattern": "gRPC.*Health|Grpc\\.HealthCheck",
    "recommendation": "Implement gRPC health checks for container orchestration",
    "example": "// In Program.cs\nbuilder.Services.AddGrpcHealthChecks();\n\nvar app = builder.Build();\napp.MapGrpcHealthChecksService();\n\n// Docker health check using grpc_health_probe\nHEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \\\n  CMD /bin/grpc_health_probe -addr=:80 || exit 1",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "health-checks",
      "monitoring",
      "orchestration"
    ],
    "description": "gRPC health checks enable proper container lifecycle management"
  },
  {
    "id": "grpc-reflection-development",
    "category": "dockerfile",
    "pattern": "gRPC.*Reflection|AddGrpcReflection",
    "recommendation": "Enable gRPC reflection only in development environments",
    "example": "// Conditional reflection based on environment\nif (builder.Environment.IsDevelopment())\n{\n    builder.Services.AddGrpcReflection();\n}\n\nvar app = builder.Build();\n\nif (app.Environment.IsDevelopment())\n{\n    app.MapGrpcReflectionService();\n}",
    "severity": "medium",
    "tags": [
      "development",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "reflection",
      "security"
    ],
    "description": "gRPC reflection should be disabled in production containers for security"
  },
  {
    "id": "grpc-client-factory",
    "category": "dockerfile",
    "pattern": "GrpcClient|gRPC.*Client",
    "recommendation": "Use HttpClientFactory for gRPC clients in containerized services",
    "example": "// In Program.cs\nbuilder.Services.AddGrpcClient<MyServiceClient>(options =>\n{\n    options.Address = new Uri(builder.Configuration[\"MyService:Url\"]!);\n})\n.ConfigurePrimaryHttpMessageHandler(() => new SocketsHttpHandler\n{\n    PooledConnectionIdleTimeout = Timeout.InfiniteTimeSpan,\n    KeepAlivePingDelay = TimeSpan.FromSeconds(60),\n    KeepAlivePingTimeout = TimeSpan.FromSeconds(30),\n    EnableMultipleHttp2Connections = true\n});",
    "severity": "medium",
    "tags": [
      "client",
      "connection-pooling",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "httpclient"
    ],
    "description": "Proper gRPC client configuration improves performance and resource utilization"
  },
  {
    "id": "grpc-compression",
    "category": "dockerfile",
    "pattern": "gRPC.*Compression|CompressionLevel",
    "recommendation": "Configure compression for gRPC services to reduce bandwidth usage",
    "example": "// In service implementation\n[HttpPost]\npublic override async Task<Response> MyMethod(Request request, ServerCallContext context)\n{\n    context.ResponseHeaders.Add(\"grpc-encoding\", \"gzip\");\n    // Implementation\n}\n\n// Client configuration\nvar client = new MyServiceClient(channel);\nvar response = await client.MyMethodAsync(request, new CallOptions(\n    headers: new Metadata { { \"grpc-accept-encoding\", \"gzip\" } }\n));",
    "severity": "low",
    "tags": [
      "bandwidth",
      "compression",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "performance"
    ],
    "description": "gRPC compression reduces network usage in container-to-container communication"
  },
  {
    "id": "grpc-timeout-configuration",
    "category": "dockerfile",
    "pattern": "gRPC.*Timeout|CallOptions",
    "recommendation": "Configure appropriate timeouts for gRPC calls in container environments",
    "example": "// Global timeout configuration\nbuilder.Services.Configure<GrpcServiceOptions>(options =>\n{\n    options.DefaultTimeout = TimeSpan.FromSeconds(30);\n    options.MaxReceiveMessageSize = 4 * 1024 * 1024; // 4MB\n    options.MaxSendMessageSize = 4 * 1024 * 1024; // 4MB\n});\n\n// Per-call timeout\nvar response = await client.MyMethodAsync(request, \n    deadline: DateTime.UtcNow.AddSeconds(10));",
    "severity": "medium",
    "tags": [
      "configuration",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "resilience",
      "timeout"
    ],
    "description": "Proper timeout configuration prevents resource exhaustion in container environments"
  },
  {
    "id": "grpc-interceptors-logging",
    "category": "dockerfile",
    "pattern": "gRPC.*Interceptor|ServerInterceptor",
    "recommendation": "Use interceptors for logging and monitoring in gRPC services",
    "example": "public class LoggingInterceptor : Interceptor\n{\n    private readonly ILogger<LoggingInterceptor> _logger;\n\n    public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(\n        TRequest request,\n        ServerCallContext context,\n        UnaryServerMethod<TRequest, TResponse> continuation)\n    {\n        _logger.LogInformation(\"gRPC call {Method} started\", context.Method);\n        try\n        {\n            var response = await continuation(request, context);\n            _logger.LogInformation(\"gRPC call {Method} completed\", context.Method);\n            return response;\n        }\n        catch (Exception ex)\n        {\n            _logger.LogError(ex, \"gRPC call {Method} failed\", context.Method);\n            throw;\n        }\n    }\n}",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "interceptors",
      "logging",
      "monitoring"
    ],
    "description": "Interceptors provide consistent logging and monitoring across gRPC services"
  },
  {
    "id": "grpc-load-balancing",
    "category": "dockerfile",
    "pattern": "gRPC.*LoadBalanc|grpc.*lb",
    "recommendation": "Configure client-side load balancing for gRPC services in container clusters",
    "example": "// Client configuration with load balancing\nbuilder.Services.AddGrpcClient<MyServiceClient>(options =>\n{\n    options.Address = new Uri(\"dns:///myservice:80\");\n})\n.ConfigureChannel(options =>\n{\n    options.Credentials = ChannelCredentials.Insecure;\n    options.ServiceConfig = new ServiceConfig\n    {\n        LoadBalancingConfigs = { new RoundRobinConfig() }\n    };\n});",
    "severity": "medium",
    "tags": [
      "clustering",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "load-balancing",
      "scalability"
    ],
    "description": "Client-side load balancing improves reliability in multi-container gRPC deployments"
  },
  {
    "id": "grpc-streaming-backpressure",
    "category": "dockerfile",
    "pattern": "gRPC.*Stream|IAsyncStreamReader",
    "recommendation": "Implement backpressure handling for gRPC streaming services",
    "example": "public override async Task ServerStreamingMethod(\n    Request request,\n    IServerStreamWriter<Response> responseStream,\n    ServerCallContext context)\n{\n    var semaphore = new SemaphoreSlim(10); // Limit concurrent operations\n    \n    await foreach (var item in GetDataAsync(request))\n    {\n        await semaphore.WaitAsync(context.CancellationToken);\n        try\n        {\n            await responseStream.WriteAsync(new Response { Data = item });\n        }\n        finally\n        {\n            semaphore.Release();\n        }\n    }\n}",
    "severity": "medium",
    "tags": [
      "backpressure",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "performance",
      "streaming"
    ],
    "description": "Backpressure handling prevents memory issues in streaming gRPC services"
  },
  {
    "id": "grpc-authentication-jwt",
    "category": "security",
    "pattern": "gRPC.*Auth|Authentication",
    "recommendation": "Implement JWT authentication for gRPC services in container environments",
    "example": "// In Program.cs\nbuilder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)\n    .AddJwtBearer(options =>\n    {\n        options.TokenValidationParameters = new TokenValidationParameters\n        {\n            ValidateIssuer = true,\n            ValidateAudience = true,\n            ValidateLifetime = true,\n            ValidateIssuerSigningKey = true,\n            ValidIssuer = builder.Configuration[\"Jwt:Issuer\"],\n            ValidAudience = builder.Configuration[\"Jwt:Audience\"],\n            IssuerSigningKey = new SymmetricSecurityKey(\n                Encoding.UTF8.GetBytes(builder.Configuration[\"Jwt:SecretKey\"]!))\n        };\n    });\n\nbuilder.Services.AddAuthorization();",
    "severity": "high",
    "tags": [
      "authentication",
      "aws",
      "fix-dockerfile",
      "grpc",
      "jwt",
      "scan-image",
      "security"
    ],
    "description": "JWT authentication provides secure access control for containerized gRPC services"
  },
  {
    "id": "grpc-metrics-prometheus",
    "category": "dockerfile",
    "pattern": "gRPC.*Metrics|Prometheus",
    "recommendation": "Expose gRPC metrics for monitoring in container environments",
    "example": "// Add metrics packages\n// <PackageReference Include=\"Grpc.AspNetCore.Server.Reflection\" />\n// <PackageReference Include=\"prometheus-net.AspNetCore\" />\n\n// In Program.cs\nbuilder.Services.AddGrpc(options =>\n{\n    options.EnableDetailedErrors = false; // Production\n    options.Interceptors.Add<MetricsInterceptor>();\n});\n\nvar app = builder.Build();\napp.UseRouting();\napp.UseHttpMetrics(); // Prometheus metrics\napp.MapMetrics(); // /metrics endpoint",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "metrics",
      "monitoring",
      "prometheus"
    ],
    "description": "Metrics collection enables proper monitoring of gRPC services in container orchestration"
  },
  {
    "id": "grpc-deadlines-cancellation",
    "category": "dockerfile",
    "pattern": "gRPC.*Deadline|CancellationToken",
    "recommendation": "Properly handle deadlines and cancellation in gRPC services",
    "example": "public override async Task<Response> MyMethod(\n    Request request, \n    ServerCallContext context)\n{\n    // Check if call is already cancelled\n    context.CancellationToken.ThrowIfCancellationRequested();\n    \n    // Use cancellation token in async operations\n    var result = await SomeAsyncOperation(request, context.CancellationToken);\n    \n    // Check deadline\n    if (context.Deadline < DateTime.UtcNow.AddSeconds(1))\n    {\n        throw new RpcException(new Status(StatusCode.DeadlineExceeded, \"Operation too slow\"));\n    }\n    \n    return new Response { Result = result };\n}",
    "severity": "high",
    "tags": [
      "cancellation",
      "deadlines",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "resilience"
    ],
    "description": "Proper deadline and cancellation handling prevents resource leaks in container environments"
  },
  {
    "id": "grpc-error-handling",
    "category": "dockerfile",
    "pattern": "gRPC.*Error|RpcException",
    "recommendation": "Implement consistent error handling for gRPC services",
    "example": "public override async Task<Response> MyMethod(Request request, ServerCallContext context)\n{\n    try\n    {\n        var result = await ProcessRequest(request);\n        return new Response { Data = result };\n    }\n    catch (ArgumentException ex)\n    {\n        throw new RpcException(new Status(StatusCode.InvalidArgument, ex.Message));\n    }\n    catch (UnauthorizedAccessException)\n    {\n        throw new RpcException(new Status(StatusCode.Unauthenticated, \"Access denied\"));\n    }\n    catch (Exception ex)\n    {\n        _logger.LogError(ex, \"Unexpected error in MyMethod\");\n        throw new RpcException(new Status(StatusCode.Internal, \"Internal server error\"));\n    }\n}",
    "severity": "high",
    "tags": [
      "error-handling",
      "exceptions",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "status-codes"
    ],
    "description": "Consistent error handling improves client experience and service reliability"
  },
  {
    "id": "grpc-web-cors-configuration",
    "category": "dockerfile",
    "pattern": "gRPC.*Web|grpc-web",
    "recommendation": "Configure CORS properly for gRPC-Web in container deployments",
    "example": "// In Program.cs\nbuilder.Services.AddCors(options =>\n{\n    options.AddPolicy(\"AllowGrpcWeb\", policy =>\n    {\n        policy.AllowAnyOrigin()\n              .AllowAnyHeader()\n              .AllowAnyMethod()\n              .WithExposedHeaders(\"grpc-status\", \"grpc-message\", \"grpc-encoding\");\n    });\n});\n\nbuilder.Services.AddGrpc();\nbuilder.Services.AddGrpcWeb();\n\nvar app = builder.Build();\napp.UseCors(\"AllowGrpcWeb\");\napp.UseGrpcWeb();",
    "severity": "medium",
    "tags": [
      "browser",
      "cors",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "grpc-web"
    ],
    "description": "gRPC-Web requires specific CORS configuration for browser-based clients"
  },
  {
    "id": "grpc-connection-pooling",
    "category": "dockerfile",
    "pattern": "gRPC.*Channel|GrpcChannel",
    "recommendation": "Configure connection pooling for gRPC clients to optimize resource usage",
    "example": "// Singleton channel factory\nbuilder.Services.AddSingleton<IChannelFactory, ChannelFactory>();\n\npublic class ChannelFactory : IChannelFactory\n{\n    private readonly ConcurrentDictionary<string, GrpcChannel> _channels = new();\n    \n    public GrpcChannel GetChannel(string address)\n    {\n        return _channels.GetOrAdd(address, addr => GrpcChannel.ForAddress(addr, new GrpcChannelOptions\n        {\n            HttpHandler = new SocketsHttpHandler\n            {\n                PooledConnectionIdleTimeout = Timeout.InfiniteTimeSpan,\n                KeepAlivePingDelay = TimeSpan.FromSeconds(60),\n                KeepAlivePingTimeout = TimeSpan.FromSeconds(30)\n            }\n        }));\n    }\n}",
    "severity": "medium",
    "tags": [
      "connection-pooling",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "performance",
      "resources"
    ],
    "description": "Connection pooling reduces overhead in high-throughput gRPC applications"
  },
  {
    "id": "grpc-protobuf-optimization",
    "category": "dockerfile",
    "pattern": "\\.proto|protobuf",
    "recommendation": "Optimize Protocol Buffer definitions for container deployment size",
    "example": "// Optimize .proto files\nsyntax = \"proto3\";\n\npackage myservice.v1;\n\noption csharp_namespace = \"MyService.V1\";\n\n// Use appropriate field types\nmessage OptimizedMessage {\n  // Use int32 instead of int64 when possible\n  int32 id = 1;\n  \n  // Use repeated for collections\n  repeated string items = 2;\n  \n  // Use oneof for optional fields\n  oneof optional_field {\n    string text_value = 3;\n    int32 number_value = 4;\n  }\n}",
    "severity": "low",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "optimization",
      "protobuf",
      "serialization"
    ],
    "description": "Optimized Protocol Buffer definitions reduce serialization overhead and message size"
  },
  {
    "id": "grpc-retry-configuration",
    "category": "dockerfile",
    "pattern": "gRPC.*Retry|RetryPolicy",
    "recommendation": "Configure retry policies for gRPC clients in unreliable container networks",
    "example": "// Configure retry policy\nbuilder.Services.AddGrpcClient<MyServiceClient>(options =>\n{\n    options.Address = new Uri(\"https://myservice:443\");\n})\n.ConfigureChannel(options =>\n{\n    options.ServiceConfig = new ServiceConfig\n    {\n        MethodConfigs =\n        {\n            new MethodConfig\n            {\n                Names = { MethodName.Default },\n                RetryPolicy = new RetryPolicy\n                {\n                    MaxAttempts = 3,\n                    InitialBackoff = TimeSpan.FromSeconds(1),\n                    MaxBackoff = TimeSpan.FromSeconds(5),\n                    BackoffMultiplier = 2,\n                    RetryableStatusCodes = { StatusCode.Unavailable, StatusCode.DeadlineExceeded }\n                }\n            }\n        }\n    };\n});",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "networking",
      "resilience",
      "retry"
    ],
    "description": "Retry policies improve reliability in container network environments"
  },
  {
    "id": "grpc-service-discovery",
    "category": "dockerfile",
    "pattern": "gRPC.*Discovery|ServiceDiscovery",
    "recommendation": "Implement service discovery for gRPC services in container orchestration",
    "example": "// Using DNS-based service discovery\nbuilder.Services.AddGrpcClient<MyServiceClient>(options =>\n{\n    // Use Kubernetes DNS\n    options.Address = new Uri(\"dns:///myservice.default.svc.cluster.local:80\");\n})\n.ConfigureChannel(options =>\n{\n    options.Credentials = ChannelCredentials.Insecure;\n    options.ServiceConfig = new ServiceConfig\n    {\n        LoadBalancingConfigs = { new RoundRobinConfig() }\n    };\n});",
    "severity": "medium",
    "tags": [
      "dns",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "kubernetes",
      "service-discovery"
    ],
    "description": "Service discovery enables dynamic gRPC service location in container clusters"
  },
  {
    "id": "grpc-security-tls-mutual",
    "category": "security",
    "pattern": "gRPC.*mTLS|mutual.*tls",
    "recommendation": "Configure mutual TLS authentication for secure gRPC communication",
    "example": "// Server configuration\nbuilder.WebHost.ConfigureKestrel(options =>\n{\n    options.ListenAnyIP(443, listenOptions =>\n    {\n        listenOptions.Protocols = HttpProtocols.Http2;\n        listenOptions.UseHttps(httpsOptions =>\n        {\n            httpsOptions.ClientCertificateMode = ClientCertificateMode.RequireCertificate;\n            httpsOptions.ClientCertificateValidation = (cert, chain, errors) =>\n            {\n                // Custom certificate validation logic\n                return cert?.Issuer == \"CN=MyCA\";\n            };\n        });\n    });\n});",
    "severity": "high",
    "tags": [
      "fix-dockerfile",
      "grpc",
      "mtls",
      "mutual-tls",
      "scan-image",
      "security"
    ],
    "description": "Mutual TLS provides strong authentication for service-to-service gRPC communication"
  },
  {
    "id": "grpc-graceful-shutdown",
    "category": "dockerfile",
    "pattern": "gRPC.*Shutdown|GracefulShutdown",
    "recommendation": "Implement graceful shutdown for gRPC services in container environments",
    "example": "// In Program.cs\nvar app = builder.Build();\n\n// Configure graceful shutdown\nvar lifetime = app.Services.GetRequiredService<IHostApplicationLifetime>();\nlifetime.ApplicationStopping.Register(() =>\n{\n    // Give ongoing calls time to complete\n    Thread.Sleep(TimeSpan.FromSeconds(5));\n});\n\n// Configure shutdown timeout\nbuilder.Services.Configure<HostOptions>(options =>\n{\n    options.ShutdownTimeout = TimeSpan.FromSeconds(30);\n});",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "graceful-shutdown",
      "grpc",
      "lifecycle",
      "reliability"
    ],
    "description": "Graceful shutdown ensures ongoing gRPC calls complete before container termination"
  },
  {
    "id": "grpc-rate-limiting",
    "category": "dockerfile",
    "pattern": "gRPC.*RateLimit|throttle",
    "recommendation": "Implement rate limiting for gRPC services to prevent resource exhaustion",
    "example": "// Using AspNetCoreRateLimit or custom interceptor\npublic class RateLimitingInterceptor : Interceptor\n{\n    private readonly SemaphoreSlim _semaphore;\n    \n    public RateLimitingInterceptor(int maxConcurrentCalls)\n    {\n        _semaphore = new SemaphoreSlim(maxConcurrentCalls);\n    }\n    \n    public override async Task<TResponse> UnaryServerHandler<TRequest, TResponse>(\n        TRequest request,\n        ServerCallContext context,\n        UnaryServerMethod<TRequest, TResponse> continuation)\n    {\n        if (!await _semaphore.WaitAsync(TimeSpan.FromSeconds(1)))\n        {\n            throw new RpcException(new Status(StatusCode.ResourceExhausted, \"Too many requests\"));\n        }\n        \n        try\n        {\n            return await continuation(request, context);\n        }\n        finally\n        {\n            _semaphore.Release();\n        }\n    }\n}",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "rate-limiting",
      "resource-protection",
      "throttling"
    ],
    "description": "Rate limiting protects gRPC services from overload in container environments"
  },
  {
    "id": "grpc-binary-logging",
    "category": "dockerfile",
    "pattern": "gRPC.*BinaryLog|binary.*log",
    "recommendation": "Configure gRPC binary logging carefully to avoid performance impact",
    "example": "// Only enable binary logging in development or debugging\n#if DEBUG\nEnvironment.SetEnvironmentVariable(\"GRPC_TRACE\", \"api\");\nEnvironment.SetEnvironmentVariable(\"GRPC_VERBOSITY\", \"debug\");\n#endif\n\n// For production, use structured logging instead\nbuilder.Services.AddLogging(logging =>\n{\n    logging.AddJsonConsole(options =>\n    {\n        options.IncludeScopes = true;\n        options.TimestampFormat = \"yyyy-MM-dd HH:mm:ss \";\n    });\n});",
    "severity": "low",
    "tags": [
      "binary-logging",
      "debugging",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "performance"
    ],
    "description": "Binary logging should be used carefully as it can impact performance in production"
  },
  {
    "id": "grpc-keepalive-configuration",
    "category": "dockerfile",
    "pattern": "gRPC.*KeepAlive|keep.*alive",
    "recommendation": "Configure keep-alive settings for long-lived gRPC connections",
    "example": "// Server configuration\nbuilder.Services.AddGrpc(options =>\n{\n    options.EnableDetailedErrors = false;\n});\n\nbuilder.WebHost.ConfigureKestrel(options =>\n{\n    options.Limits.Http2.KeepAlivePingDelay = TimeSpan.FromSeconds(30);\n    options.Limits.Http2.KeepAlivePingTimeout = TimeSpan.FromSeconds(60);\n});\n\n// Client configuration\nvar channel = GrpcChannel.ForAddress(\"https://myservice:443\", new GrpcChannelOptions\n{\n    HttpHandler = new SocketsHttpHandler\n    {\n        KeepAlivePingDelay = TimeSpan.FromSeconds(60),\n        KeepAlivePingTimeout = TimeSpan.FromSeconds(30)\n    }\n});",
    "severity": "medium",
    "tags": [
      "connections",
      "fix-dockerfile",
      "generate-dockerfile",
      "grpc",
      "keepalive",
      "networking"
    ],
    "description": "Keep-alive configuration maintains connection health in container network environments"
  }
] as const;
