# .NET SDK for Visual Studio - Implementation Plan

## Executive Summary

This plan describes how to create a native .NET SDK that provides containerization functionality equivalent to the TypeScript SDK, designed specifically for Visual Studio integration. The SDK will enable .NET developers to use the 4 core tools (`analyze-repo`, `generate-dockerfile`, `build-image`, `scan-image`) as native C# function calls without any MCP or Node.js dependencies.

**Customer Request**: Provide SDK functionality that works natively in Visual Studio without MCP dependency, enabling direct integration into Visual Studio extensions and .NET tooling.

## Research Findings

### Available .NET Infrastructure Libraries

| Library | Purpose | NuGet Package | Maturity |
|---------|---------|---------------|----------|
| [Docker.DotNet](https://github.com/dotnet/Docker.DotNet) | Docker Remote API client | `Docker.DotNet 3.125.15` | Production (.NET Foundation) |
| [KubernetesClient](https://github.com/kubernetes-client/csharp) | Official K8s client | `KubernetesClient 18.0.13` | Production (kubernetes-client org) |
| [OneOf](https://github.com/mcintyre321/OneOf) | Discriminated unions / Result pattern | `OneOf` | Production |
| [Trivy](https://trivy.dev/) | Security scanning (CLI) | N/A (CLI tool) | Production |

### Visual Studio Extension Options

| Approach | Framework | Target | Best For |
|----------|-----------|--------|----------|
| [Community.VisualStudio.Toolkit](https://github.com/VsixCommunity/Community.VisualStudio.Toolkit) | VSIX | VS 2019/2022 | Traditional extensions |
| [VisualStudio.Extensibility](https://learn.microsoft.com/en-us/visualstudio/extensibility/visualstudio.extensibility/) | New SDK | VS 2022+ | Modern out-of-proc extensions |
| NuGet Package | Class Library | Any IDE | SDK-only consumption |

### Architecture Decision Matrix

| Approach | Dev Effort | Performance | Maintenance | VS Integration | Recommendation |
|----------|------------|-------------|-------------|----------------|----------------|
| Native .NET Port | High (3-4 months) | Excellent | Medium | Native | **Recommended** |
| gRPC Bridge | Medium (1-2 months) | Good | Low | Requires Node.js | Not recommended |
| CLI Wrapper | Low (2-3 weeks) | Poor | Low | Requires Node.js | Not recommended |
| Hybrid | Medium (2-3 months) | Good | High | Partial Native | Alternative |

**Recommendation**: Native .NET port provides the best developer experience for Visual Studio users, eliminates external runtime dependencies, and allows full use of C# language features and debugging.

## Architecture Design

### Layer Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         CONSUMER LAYER                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   Visual Studio Extension              CLI Application                   │
│   (VSIX / Extensibility SDK)           (dotnet tool)                     │
│              │                              │                            │
│              ▼                              ▼                            │
│   ┌─────────────────────────────────────────────────────────────┐       │
│   │              ContainerizationAssist.Sdk                      │       │
│   │                    (NuGet Package)                           │       │
│   └─────────────────────────────────────────────────────────────┘       │
│                              │                                           │
├──────────────────────────────┼───────────────────────────────────────────┤
│                              │           SDK LAYER                       │
│                              ▼                                           │
│   ┌─────────────────────────────────────────────────────────────┐       │
│   │                    SDK Public API                            │       │
│   │  • AnalyzeRepoAsync(path) → Result<RepositoryAnalysis>      │       │
│   │  • GenerateDockerfileAsync(params) → Result<DockerfilePlan> │       │
│   │  • BuildImageAsync(params) → Result<BuildResult>            │       │
│   │  • ScanImageAsync(imageId) → Result<ScanResult>             │       │
│   └──────────────────────────┬──────────────────────────────────┘       │
│                              │                                           │
├──────────────────────────────┼───────────────────────────────────────────┤
│                              │         CORE LAYER                        │
│              ┌───────────────┼───────────────┐                          │
│              ▼               ▼               ▼                          │
│   ┌───────────────┐  ┌──────────────┐  ┌──────────────┐                │
│   │   Analyzers   │  │   Builders   │  │   Scanners   │                │
│   │               │  │              │  │              │                │
│   │ • RepoAnalyzer│  │ • ImageBuilder│ │ • TrivyScanner│               │
│   │ • ConfigParser│  │ • Dockerfile │  │ • VulnMatcher│                │
│   │ • LangDetector│  │   Generator  │  │              │                │
│   └───────────────┘  └──────────────┘  └──────────────┘                │
│              │               │               │                          │
├──────────────┼───────────────┼───────────────┼──────────────────────────┤
│              │               │               │    INFRASTRUCTURE        │
│              ▼               ▼               ▼                          │
│   ┌─────────────────────────────────────────────────────────────┐       │
│   │                  Infrastructure Clients                      │       │
│   │                                                              │       │
│   │   ┌─────────────┐  ┌─────────────────┐  ┌───────────────┐  │       │
│   │   │ Docker.DotNet│  │ KubernetesClient│  │ Process Runner│  │       │
│   │   │ (Docker API) │  │  (K8s API)      │  │ (Trivy CLI)   │  │       │
│   │   └─────────────┘  └─────────────────┘  └───────────────┘  │       │
│   └─────────────────────────────────────────────────────────────┘       │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Project Structure

```
ContainerizationAssist.Sdk/
├── ContainerizationAssist.Sdk.sln
├── src/
│   ├── ContainerizationAssist.Sdk/           # Main SDK package
│   │   ├── ContainerizationAssist.Sdk.csproj
│   │   │
│   │   ├── Abstractions/                      # Public interfaces & types
│   │   │   ├── IContainerizationService.cs
│   │   │   ├── IRepositoryAnalyzer.cs
│   │   │   ├── IDockerfileGenerator.cs
│   │   │   ├── IImageBuilder.cs
│   │   │   └── ISecurityScanner.cs
│   │   │
│   │   ├── Models/                            # Domain models
│   │   │   ├── Results/
│   │   │   │   ├── Result.cs                  # Result<T> discriminated union
│   │   │   │   ├── Success.cs
│   │   │   │   └── Failure.cs
│   │   │   ├── Repository/
│   │   │   │   ├── RepositoryAnalysis.cs
│   │   │   │   ├── ModuleInfo.cs
│   │   │   │   └── FrameworkInfo.cs
│   │   │   ├── Dockerfile/
│   │   │   │   ├── DockerfilePlan.cs
│   │   │   │   ├── BaseImageRecommendation.cs
│   │   │   │   └── DockerfileRequirement.cs
│   │   │   ├── Build/
│   │   │   │   ├── BuildImageParams.cs
│   │   │   │   └── BuildImageResult.cs
│   │   │   └── Security/
│   │   │       ├── ScanImageParams.cs
│   │   │       ├── ScanImageResult.cs
│   │   │       └── Vulnerability.cs
│   │   │
│   │   ├── Services/                          # Core service implementations
│   │   │   ├── RepositoryAnalyzer.cs
│   │   │   ├── DockerfileGenerator.cs
│   │   │   ├── ImageBuilder.cs
│   │   │   └── SecurityScanner.cs
│   │   │
│   │   ├── Parsers/                           # Config file parsers
│   │   │   ├── IConfigParser.cs
│   │   │   ├── PackageJsonParser.cs
│   │   │   ├── CsprojParser.cs
│   │   │   ├── PomXmlParser.cs
│   │   │   ├── PyProjectParser.cs
│   │   │   ├── GoModParser.cs
│   │   │   └── CargoTomlParser.cs
│   │   │
│   │   ├── Infrastructure/                    # External integrations
│   │   │   ├── Docker/
│   │   │   │   ├── DockerClientWrapper.cs
│   │   │   │   └── DockerClientFactory.cs
│   │   │   ├── Kubernetes/
│   │   │   │   ├── KubernetesClientWrapper.cs
│   │   │   │   └── KubernetesClientFactory.cs
│   │   │   └── Security/
│   │   │       ├── TrivyRunner.cs
│   │   │       └── TrivyOutputParser.cs
│   │   │
│   │   ├── Knowledge/                         # Knowledge base (embedded)
│   │   │   ├── KnowledgeLoader.cs
│   │   │   ├── KnowledgeMatcher.cs
│   │   │   └── Data/                          # Embedded JSON resources
│   │   │       ├── base-images.json
│   │   │       ├── security.json
│   │   │       └── best-practices.json
│   │   │
│   │   └── Extensions/                        # Extension methods
│   │       ├── ServiceCollectionExtensions.cs # DI registration
│   │       └── ResultExtensions.cs
│   │
│   ├── ContainerizationAssist.Sdk.Abstractions/ # Abstractions-only package
│   │   └── (interfaces and models for DI scenarios)
│   │
│   └── ContainerizationAssist.VisualStudio/   # VS-specific integration
│       ├── ContainerizationAssist.VisualStudio.csproj
│       ├── source.extension.vsixmanifest
│       ├── Commands/
│       │   ├── AnalyzeProjectCommand.cs
│       │   ├── GenerateDockerfileCommand.cs
│       │   └── BuildImageCommand.cs
│       ├── ToolWindows/
│       │   ├── ContainerDashboardWindow.cs
│       │   └── ContainerDashboardControl.xaml
│       └── Options/
│           └── ContainerizationOptions.cs
│
├── tests/
│   ├── ContainerizationAssist.Sdk.Tests/
│   │   ├── Unit/
│   │   │   ├── Parsers/
│   │   │   ├── Services/
│   │   │   └── Knowledge/
│   │   └── Integration/
│   │       ├── DockerIntegrationTests.cs
│   │       └── TrivyIntegrationTests.cs
│   │
│   └── ContainerizationAssist.Sdk.TestUtils/
│       └── (test helpers, mocks, fixtures)
│
├── samples/
│   ├── ConsoleApp/
│   └── AspNetCoreApp/
│
└── docs/
    ├── getting-started.md
    ├── api-reference.md
    └── vs-extension-guide.md
```

## Detailed Component Design

### 1. Result Pattern Implementation

Use the `OneOf` library for a type-safe Result pattern matching the TypeScript SDK:

```csharp
// ContainerizationAssist.Sdk/Models/Results/Result.cs

using OneOf;

namespace ContainerizationAssist.Sdk.Models;

/// <summary>
/// Represents an operation result that is either successful with a value,
/// or failed with error details and guidance.
/// </summary>
public readonly struct Result<T>
{
    private readonly OneOf<Success<T>, Failure> _value;

    private Result(OneOf<Success<T>, Failure> value) => _value = value;

    public bool IsSuccess => _value.IsT0;
    public bool IsFailure => _value.IsT1;

    public T Value => _value.Match(
        success => success.Value,
        failure => throw new InvalidOperationException($"Cannot access Value on failed result: {failure.Message}")
    );

    public Failure Error => _value.Match(
        success => throw new InvalidOperationException("Cannot access Error on successful result"),
        failure => failure
    );

    public static Result<T> Ok(T value) => new(new Success<T>(value));
    public static Result<T> Fail(string message, ErrorGuidance? guidance = null) =>
        new(new Failure(message, guidance));

    public TResult Match<TResult>(
        Func<T, TResult> onSuccess,
        Func<Failure, TResult> onFailure) =>
        _value.Match(s => onSuccess(s.Value), onFailure);

    public async Task<TResult> MatchAsync<TResult>(
        Func<T, Task<TResult>> onSuccess,
        Func<Failure, Task<TResult>> onFailure) =>
        await _value.Match(
            async s => await onSuccess(s.Value),
            async f => await onFailure(f));
}

public readonly record struct Success<T>(T Value);

public readonly record struct Failure(string Message, ErrorGuidance? Guidance = null);

public record ErrorGuidance(
    string Message,
    string? Hint = null,
    string? Resolution = null
);
```

### 2. Core Service Interfaces

```csharp
// ContainerizationAssist.Sdk/Abstractions/IContainerizationService.cs

namespace ContainerizationAssist.Sdk.Abstractions;

/// <summary>
/// Main entry point for containerization operations.
/// Provides access to all SDK functionality through a unified interface.
/// </summary>
public interface IContainerizationService
{
    /// <summary>
    /// Analyze a repository to detect language, framework, and dependencies.
    /// </summary>
    Task<Result<RepositoryAnalysis>> AnalyzeRepositoryAsync(
        string repositoryPath,
        AnalyzeOptions? options = null,
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Generate Dockerfile recommendations for a repository.
    /// </summary>
    Task<Result<DockerfilePlan>> GenerateDockerfileAsync(
        GenerateDockerfileParams parameters,
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Build a Docker image from a Dockerfile.
    /// </summary>
    Task<Result<BuildImageResult>> BuildImageAsync(
        BuildImageParams parameters,
        IProgress<BuildProgress>? progress = null,
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Scan a Docker image for security vulnerabilities.
    /// </summary>
    Task<Result<ScanImageResult>> ScanImageAsync(
        ScanImageParams parameters,
        CancellationToken cancellationToken = default);
}

/// <summary>
/// Options for repository analysis.
/// </summary>
public record AnalyzeOptions
{
    public IReadOnlyList<ModuleInfo>? PreProvidedModules { get; init; }
    public int MaxDepth { get; init; } = 3;
    public bool IncludeDevDependencies { get; init; } = false;
}

/// <summary>
/// Progress information during image build.
/// </summary>
public record BuildProgress(
    string Message,
    int? Current = null,
    int? Total = null,
    string? Stage = null
);
```

### 3. Repository Analyzer Implementation

```csharp
// ContainerizationAssist.Sdk/Services/RepositoryAnalyzer.cs

using Microsoft.Extensions.Logging;

namespace ContainerizationAssist.Sdk.Services;

public class RepositoryAnalyzer : IRepositoryAnalyzer
{
    private readonly ILogger<RepositoryAnalyzer> _logger;
    private readonly IEnumerable<IConfigParser> _parsers;

    public RepositoryAnalyzer(
        ILogger<RepositoryAnalyzer> logger,
        IEnumerable<IConfigParser> parsers)
    {
        _logger = logger;
        _parsers = parsers;
    }

    public async Task<Result<RepositoryAnalysis>> AnalyzeAsync(
        string repositoryPath,
        AnalyzeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        // Validate path
        if (!Directory.Exists(repositoryPath))
        {
            return Result<RepositoryAnalysis>.Fail(
                $"Repository path does not exist: {repositoryPath}",
                new ErrorGuidance(
                    "The specified directory does not exist",
                    "Check that the path is correct",
                    "Provide a valid directory path containing a project"
                ));
        }

        var absolutePath = Path.GetFullPath(repositoryPath);
        _logger.LogInformation("Starting analysis of repository at {Path}", absolutePath);

        // If modules are pre-provided, use them
        if (options?.PreProvidedModules?.Count > 0)
        {
            return Result<RepositoryAnalysis>.Ok(new RepositoryAnalysis
            {
                Modules = options.PreProvidedModules.ToList(),
                IsMonorepo = options.PreProvidedModules.Count > 1,
                AnalyzedPath = absolutePath,
                Summary = $"Using {options.PreProvidedModules.Count} pre-provided module(s)"
            });
        }

        // Discover and parse config files
        var configFiles = await DiscoverConfigFilesAsync(absolutePath, options?.MaxDepth ?? 3, cancellationToken);
        var modules = new List<ModuleInfo>();

        foreach (var (filePath, content) in configFiles)
        {
            foreach (var parser in _parsers)
            {
                if (parser.CanParse(filePath))
                {
                    try
                    {
                        var parsed = await parser.ParseAsync(filePath, content, cancellationToken);
                        if (parsed != null)
                        {
                            modules.Add(CreateModuleFromParsedConfig(filePath, parsed));
                            break; // Only use first matching parser
                        }
                    }
                    catch (Exception ex)
                    {
                        _logger.LogWarning(ex, "Failed to parse {File}", filePath);
                    }
                }
            }
        }

        if (modules.Count == 0)
        {
            return Result<RepositoryAnalysis>.Fail(
                "No modules detected in repository",
                new ErrorGuidance(
                    "Could not identify any recognizable project files",
                    "Ensure the repository contains project files",
                    "Repository should contain package.json, *.csproj, pom.xml, requirements.txt, or similar"
                ));
        }

        var isMonorepo = modules.Count > 1;
        var languageList = string.Join(", ", modules.Select(m => m.Language).Distinct());

        return Result<RepositoryAnalysis>.Ok(new RepositoryAnalysis
        {
            Modules = modules,
            IsMonorepo = isMonorepo,
            AnalyzedPath = absolutePath,
            Summary = $"Analyzed repository. Detected {modules.Count} module(s) ({languageList}).{(isMonorepo ? " Monorepo structure identified." : "")}"
        });
    }

    private async Task<Dictionary<string, string>> DiscoverConfigFilesAsync(
        string rootPath,
        int maxDepth,
        CancellationToken cancellationToken)
    {
        var configFiles = new Dictionary<string, string>();
        var configPatterns = new[]
        {
            "package.json", "*.csproj", "*.fsproj", "*.vbproj",
            "pom.xml", "build.gradle", "build.gradle.kts",
            "requirements.txt", "pyproject.toml",
            "Cargo.toml", "go.mod", "Gemfile", "composer.json"
        };

        await ScanDirectoryAsync(rootPath, 0, maxDepth, configPatterns, configFiles, cancellationToken);
        return configFiles;
    }

    private async Task ScanDirectoryAsync(
        string directory,
        int currentDepth,
        int maxDepth,
        string[] patterns,
        Dictionary<string, string> results,
        CancellationToken cancellationToken)
    {
        if (currentDepth > maxDepth || cancellationToken.IsCancellationRequested)
            return;

        var dirName = Path.GetFileName(directory);
        if (IsIgnoredDirectory(dirName))
            return;

        foreach (var pattern in patterns)
        {
            foreach (var file in Directory.EnumerateFiles(directory, pattern))
            {
                try
                {
                    var content = await File.ReadAllTextAsync(file, cancellationToken);
                    // Truncate large files
                    if (content.Length > 10000)
                        content = content[..10000] + "\n...[truncated]";
                    results[file] = content;
                }
                catch { /* Skip unreadable files */ }
            }
        }

        foreach (var subDir in Directory.EnumerateDirectories(directory))
        {
            await ScanDirectoryAsync(subDir, currentDepth + 1, maxDepth, patterns, results, cancellationToken);
        }
    }

    private static bool IsIgnoredDirectory(string name) =>
        name is "node_modules" or ".git" or ".vs" or ".idea" or "bin" or "obj" or "dist" or "build" or "target";

    private ModuleInfo CreateModuleFromParsedConfig(string filePath, ParsedConfig config)
    {
        var directory = Path.GetDirectoryName(filePath)!;
        return new ModuleInfo
        {
            Name = Path.GetFileName(directory),
            ModulePath = directory,
            Language = config.Language,
            Frameworks = config.Framework != null
                ? new[] { new FrameworkInfo(config.Framework, config.FrameworkVersion) }
                : null,
            BuildSystems = config.BuildSystem != null
                ? new[] { new BuildSystemInfo(config.BuildSystem, config.LanguageVersion) }
                : null,
            Dependencies = config.Dependencies,
            EntryPoint = config.EntryPoint,
            Ports = config.Ports
        };
    }
}
```

### 4. Docker Integration (using Docker.DotNet)

```csharp
// ContainerizationAssist.Sdk/Infrastructure/Docker/DockerClientWrapper.cs

using Docker.DotNet;
using Docker.DotNet.Models;
using Microsoft.Extensions.Logging;

namespace ContainerizationAssist.Sdk.Infrastructure.Docker;

public class DockerClientWrapper : IDockerClient, IAsyncDisposable
{
    private readonly Docker.DotNet.DockerClient _client;
    private readonly ILogger<DockerClientWrapper> _logger;

    public DockerClientWrapper(ILogger<DockerClientWrapper> logger)
    {
        _logger = logger;

        // Auto-detect Docker socket based on platform
        var dockerUri = GetDockerUri();
        _client = new DockerClientConfiguration(dockerUri).CreateClient();
    }

    private static Uri GetDockerUri()
    {
        if (OperatingSystem.IsWindows())
        {
            return new Uri("npipe://./pipe/docker_engine");
        }
        else
        {
            // Linux/macOS
            return new Uri("unix:///var/run/docker.sock");
        }
    }

    public async Task<Result<BuildImageResult>> BuildImageAsync(
        BuildImageParams parameters,
        IProgress<BuildProgress>? progress = null,
        CancellationToken cancellationToken = default)
    {
        var contextPath = Path.GetFullPath(parameters.Path ?? ".");
        var dockerfilePath = parameters.Dockerfile ?? "Dockerfile";

        // Validate context exists
        if (!Directory.Exists(contextPath))
        {
            return Result<BuildImageResult>.Fail(
                $"Build context directory not found: {contextPath}",
                new ErrorGuidance(
                    "Build context directory does not exist",
                    "Verify the path is correct",
                    "Ensure the directory contains your application and Dockerfile"
                ));
        }

        // Validate Dockerfile exists
        var fullDockerfilePath = Path.Combine(contextPath, dockerfilePath);
        if (!File.Exists(fullDockerfilePath))
        {
            return Result<BuildImageResult>.Fail(
                $"Dockerfile not found: {fullDockerfilePath}",
                new ErrorGuidance(
                    "Dockerfile does not exist in the build context",
                    "Check that the Dockerfile path is correct",
                    "Use generate-dockerfile to create one, or specify the correct path"
                ));
        }

        _logger.LogInformation("Building image from {Context} using {Dockerfile}",
            contextPath, dockerfilePath);

        var startTime = DateTime.UtcNow;
        var logs = new List<string>();

        try
        {
            // Create tar archive of build context
            using var tarStream = CreateTarArchive(contextPath);

            var buildParams = new ImageBuildParameters
            {
                Dockerfile = dockerfilePath,
                Tags = parameters.Tags?.ToList() ?? new List<string> { "build:latest" },
                BuildArgs = parameters.BuildArgs?.ToDictionary(k => k.Key, v => v.Value),
                Platform = parameters.Platform,
                NoCache = parameters.NoCache,
            };

            string? imageId = null;

            await _client.Images.BuildImageFromDockerfileAsync(
                buildParams,
                tarStream,
                authConfigs: null,
                headers: null,
                progress: new Progress<JSONMessage>(msg =>
                {
                    if (!string.IsNullOrEmpty(msg.Stream))
                    {
                        var line = msg.Stream.TrimEnd('\n');
                        logs.Add(line);
                        progress?.Report(new BuildProgress(line));
                    }

                    if (!string.IsNullOrEmpty(msg.ID))
                    {
                        imageId = msg.ID;
                    }

                    if (!string.IsNullOrEmpty(msg.ErrorMessage))
                    {
                        throw new DockerBuildException(msg.ErrorMessage);
                    }
                }),
                cancellationToken);

            if (string.IsNullOrEmpty(imageId))
            {
                // Try to extract from logs
                imageId = ExtractImageIdFromLogs(logs);
            }

            if (string.IsNullOrEmpty(imageId))
            {
                return Result<BuildImageResult>.Fail(
                    "Build completed but image ID could not be determined",
                    new ErrorGuidance(
                        "Image was built but ID extraction failed",
                        "Check Docker daemon logs",
                        "Try running 'docker images' to find the built image"
                    ));
            }

            // Get image details
            var inspect = await _client.Images.InspectImageAsync(imageId, cancellationToken);
            var buildTime = (DateTime.UtcNow - startTime).TotalMilliseconds;

            _logger.LogInformation("Build completed. Image: {ImageId}, Size: {Size}",
                imageId, inspect.Size);

            return Result<BuildImageResult>.Ok(new BuildImageResult
            {
                Success = true,
                ImageId = imageId,
                RequestedTags = parameters.Tags?.ToList() ?? new List<string>(),
                CreatedTags = parameters.Tags?.ToList() ?? new List<string>(),
                Size = inspect.Size,
                Layers = inspect.RootFS?.Layers?.Count,
                BuildTime = (long)buildTime,
                Logs = logs,
                Summary = $"Built image successfully. Image: {parameters.Tags?.FirstOrDefault() ?? imageId} ({FormatSize(inspect.Size)}). Build completed in {FormatDuration(buildTime)}."
            });
        }
        catch (DockerApiException ex)
        {
            _logger.LogError(ex, "Docker build failed");
            return Result<BuildImageResult>.Fail(
                $"Docker build failed: {ex.Message}",
                new ErrorGuidance(
                    ex.Message,
                    "Check Docker daemon status and Dockerfile syntax",
                    "Ensure Docker Desktop is running and the Dockerfile is valid"
                ));
        }
    }

    public async Task<bool> IsDockerAvailableAsync(CancellationToken cancellationToken = default)
    {
        try
        {
            await _client.System.PingAsync(cancellationToken);
            return true;
        }
        catch
        {
            return false;
        }
    }

    public async ValueTask DisposeAsync()
    {
        _client.Dispose();
        GC.SuppressFinalize(this);
    }

    // Helper methods
    private static string FormatSize(long bytes) =>
        bytes switch
        {
            < 1024 => $"{bytes}B",
            < 1024 * 1024 => $"{bytes / 1024.0:F1}KB",
            < 1024 * 1024 * 1024 => $"{bytes / (1024.0 * 1024):F1}MB",
            _ => $"{bytes / (1024.0 * 1024 * 1024):F2}GB"
        };

    private static string FormatDuration(double ms) =>
        ms switch
        {
            < 1000 => $"{ms:F0}ms",
            < 60000 => $"{ms / 1000:F1}s",
            _ => $"{ms / 60000:F1}m"
        };

    private static Stream CreateTarArchive(string directory)
    {
        // Implementation using SharpZipLib or similar
        // Returns a tar.gz stream of the directory contents
        throw new NotImplementedException("Implement using SharpZipLib");
    }

    private static string? ExtractImageIdFromLogs(List<string> logs)
    {
        // Parse build output for "Successfully built <id>" pattern
        foreach (var log in logs.AsEnumerable().Reverse())
        {
            if (log.StartsWith("Successfully built "))
            {
                return log["Successfully built ".Length..].Trim();
            }
            if (log.Contains("sha256:"))
            {
                var match = System.Text.RegularExpressions.Regex.Match(log, @"sha256:([a-f0-9]+)");
                if (match.Success)
                    return match.Value;
            }
        }
        return null;
    }
}

public class DockerBuildException : Exception
{
    public DockerBuildException(string message) : base(message) { }
}
```

### 5. Security Scanner (Trivy Integration)

```csharp
// ContainerizationAssist.Sdk/Infrastructure/Security/TrivyRunner.cs

using System.Diagnostics;
using System.Text.Json;
using Microsoft.Extensions.Logging;

namespace ContainerizationAssist.Sdk.Infrastructure.Security;

public class TrivyRunner : ISecurityScanner
{
    private readonly ILogger<TrivyRunner> _logger;
    private readonly TrivyOutputParser _parser;

    public TrivyRunner(ILogger<TrivyRunner> logger)
    {
        _logger = logger;
        _parser = new TrivyOutputParser();
    }

    public async Task<Result<ScanImageResult>> ScanAsync(
        ScanImageParams parameters,
        CancellationToken cancellationToken = default)
    {
        // Check Trivy availability
        var trivyPath = await FindTrivyAsync();
        if (trivyPath == null)
        {
            return Result<ScanImageResult>.Fail(
                "Trivy scanner not found",
                new ErrorGuidance(
                    "The Trivy security scanner is not installed or not in PATH",
                    "Install Trivy from https://aquasecurity.github.io/trivy/",
                    "On Windows: scoop install trivy | On macOS: brew install trivy | On Linux: See docs"
                ));
        }

        _logger.LogInformation("Scanning image {Image} with Trivy", parameters.ImageId);

        try
        {
            var severity = parameters.Severity?.ToUpper() ?? "HIGH,CRITICAL";
            var args = $"image --format json --severity {severity} {parameters.ImageId}";

            var startInfo = new ProcessStartInfo
            {
                FileName = trivyPath,
                Arguments = args,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };

            using var process = new Process { StartInfo = startInfo };
            process.Start();

            var output = await process.StandardOutput.ReadToEndAsync(cancellationToken);
            var errorOutput = await process.StandardError.ReadToEndAsync(cancellationToken);

            await process.WaitForExitAsync(cancellationToken);

            if (process.ExitCode != 0 && string.IsNullOrEmpty(output))
            {
                return Result<ScanImageResult>.Fail(
                    $"Trivy scan failed: {errorOutput}",
                    new ErrorGuidance(
                        "Security scan encountered an error",
                        "Check that the image exists and is accessible",
                        "Try pulling the image first with 'docker pull'"
                    ));
            }

            var scanResult = _parser.Parse(output);

            // Determine pass/fail based on severity threshold
            var passed = DeterminePassStatus(scanResult, parameters.Severity ?? "high");

            var summary = BuildScanSummary(scanResult, passed);

            return Result<ScanImageResult>.Ok(new ScanImageResult
            {
                Success = true,
                Passed = passed,
                Vulnerabilities = scanResult.Summary,
                VulnerabilityDetails = scanResult.Vulnerabilities,
                ScanTime = DateTime.UtcNow.ToString("O"),
                Summary = summary
            });
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Trivy scan failed");
            return Result<ScanImageResult>.Fail(
                $"Security scan failed: {ex.Message}",
                new ErrorGuidance(
                    ex.Message,
                    "Check Trivy installation and Docker connectivity",
                    "Ensure Docker is running and Trivy has network access"
                ));
        }
    }

    private async Task<string?> FindTrivyAsync()
    {
        // Check common locations
        var candidates = new[]
        {
            "trivy",
            "trivy.exe",
            @"C:\Program Files\trivy\trivy.exe",
            "/usr/local/bin/trivy",
            "/usr/bin/trivy"
        };

        foreach (var candidate in candidates)
        {
            try
            {
                var startInfo = new ProcessStartInfo
                {
                    FileName = candidate,
                    Arguments = "--version",
                    RedirectStandardOutput = true,
                    UseShellExecute = false,
                    CreateNoWindow = true
                };

                using var process = Process.Start(startInfo);
                if (process != null)
                {
                    await process.WaitForExitAsync();
                    if (process.ExitCode == 0)
                        return candidate;
                }
            }
            catch { /* Not found, try next */ }
        }

        return null;
    }

    private static bool DeterminePassStatus(ParsedScanResult result, string severityThreshold)
    {
        return severityThreshold.ToLower() switch
        {
            "critical" => result.Summary.Critical == 0,
            "high" => result.Summary.Critical == 0 && result.Summary.High == 0,
            "medium" => result.Summary.Critical == 0 && result.Summary.High == 0 && result.Summary.Medium == 0,
            "low" => result.Summary.Total == 0,
            _ => result.Summary.Critical == 0 && result.Summary.High == 0
        };
    }

    private static string BuildScanSummary(ParsedScanResult result, bool passed)
    {
        var vulnText = $"{result.Summary.Total} vulnerabilities ({result.Summary.Critical} critical, {result.Summary.High} high, {result.Summary.Medium} medium)";

        return passed
            ? $"Security scan passed. {vulnText}."
            : $"Security scan failed. Found {vulnText}.";
    }
}
```

### 6. Dependency Injection Registration

```csharp
// ContainerizationAssist.Sdk/Extensions/ServiceCollectionExtensions.cs

using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;

namespace ContainerizationAssist.Sdk.Extensions;

public static class ServiceCollectionExtensions
{
    /// <summary>
    /// Adds ContainerizationAssist SDK services to the dependency injection container.
    /// </summary>
    public static IServiceCollection AddContainerizationAssist(
        this IServiceCollection services,
        Action<ContainerizationOptions>? configure = null)
    {
        var options = new ContainerizationOptions();
        configure?.Invoke(options);

        // Register options
        services.AddSingleton(options);

        // Register infrastructure
        services.TryAddSingleton<IDockerClient, DockerClientWrapper>();
        services.TryAddSingleton<ISecurityScanner, TrivyRunner>();

        // Register parsers
        services.TryAddEnumerable(ServiceDescriptor.Singleton<IConfigParser, PackageJsonParser>());
        services.TryAddEnumerable(ServiceDescriptor.Singleton<IConfigParser, CsprojParser>());
        services.TryAddEnumerable(ServiceDescriptor.Singleton<IConfigParser, PomXmlParser>());
        services.TryAddEnumerable(ServiceDescriptor.Singleton<IConfigParser, PyProjectParser>());
        services.TryAddEnumerable(ServiceDescriptor.Singleton<IConfigParser, GoModParser>());
        services.TryAddEnumerable(ServiceDescriptor.Singleton<IConfigParser, CargoTomlParser>());

        // Register knowledge base
        services.TryAddSingleton<IKnowledgeLoader, EmbeddedKnowledgeLoader>();
        services.TryAddSingleton<IKnowledgeMatcher, KnowledgeMatcher>();

        // Register core services
        services.TryAddSingleton<IRepositoryAnalyzer, RepositoryAnalyzer>();
        services.TryAddSingleton<IDockerfileGenerator, DockerfileGenerator>();
        services.TryAddSingleton<IImageBuilder, ImageBuilder>();

        // Register main service
        services.TryAddSingleton<IContainerizationService, ContainerizationService>();

        return services;
    }
}

public class ContainerizationOptions
{
    /// <summary>
    /// Path to custom knowledge base directory.
    /// </summary>
    public string? CustomKnowledgePath { get; set; }

    /// <summary>
    /// Default severity threshold for security scans.
    /// </summary>
    public string DefaultSeverityThreshold { get; set; } = "high";

    /// <summary>
    /// Enable verbose logging.
    /// </summary>
    public bool VerboseLogging { get; set; } = false;
}
```

### 7. SDK Entry Point (Simplified API)

```csharp
// ContainerizationAssist.Sdk/ContainerizationSdk.cs

namespace ContainerizationAssist.Sdk;

/// <summary>
/// Static entry point for quick SDK usage without dependency injection.
/// For more control, use IContainerizationService via DI.
/// </summary>
public static class ContainerizationSdk
{
    private static readonly Lazy<IContainerizationService> _service = new(() =>
    {
        var services = new ServiceCollection();
        services.AddLogging();
        services.AddContainerizationAssist();
        var provider = services.BuildServiceProvider();
        return provider.GetRequiredService<IContainerizationService>();
    });

    /// <summary>
    /// Analyze a repository to detect language, framework, and dependencies.
    /// </summary>
    /// <example>
    /// <code>
    /// var result = await ContainerizationSdk.AnalyzeRepositoryAsync("./my-app");
    /// result.Match(
    ///     analysis => Console.WriteLine($"Detected: {analysis.Modules[0].Language}"),
    ///     failure => Console.WriteLine($"Error: {failure.Message}")
    /// );
    /// </code>
    /// </example>
    public static Task<Result<RepositoryAnalysis>> AnalyzeRepositoryAsync(
        string repositoryPath,
        CancellationToken cancellationToken = default) =>
        _service.Value.AnalyzeRepositoryAsync(repositoryPath, cancellationToken: cancellationToken);

    /// <summary>
    /// Generate Dockerfile recommendations for a repository.
    /// </summary>
    public static Task<Result<DockerfilePlan>> GenerateDockerfileAsync(
        string repositoryPath,
        string targetPlatform = "linux/amd64",
        string? language = null,
        string? framework = null,
        CancellationToken cancellationToken = default) =>
        _service.Value.GenerateDockerfileAsync(new GenerateDockerfileParams
        {
            RepositoryPath = repositoryPath,
            TargetPlatform = targetPlatform,
            Language = language,
            Framework = framework
        }, cancellationToken);

    /// <summary>
    /// Build a Docker image from a Dockerfile.
    /// </summary>
    public static Task<Result<BuildImageResult>> BuildImageAsync(
        string path,
        string? imageName = null,
        IReadOnlyList<string>? tags = null,
        IProgress<BuildProgress>? progress = null,
        CancellationToken cancellationToken = default) =>
        _service.Value.BuildImageAsync(new BuildImageParams
        {
            Path = path,
            ImageName = imageName,
            Tags = tags
        }, progress, cancellationToken);

    /// <summary>
    /// Scan a Docker image for security vulnerabilities.
    /// </summary>
    public static Task<Result<ScanImageResult>> ScanImageAsync(
        string imageId,
        string severity = "high",
        CancellationToken cancellationToken = default) =>
        _service.Value.ScanImageAsync(new ScanImageParams
        {
            ImageId = imageId,
            Severity = severity
        }, cancellationToken);
}
```

## Visual Studio Extension Integration

### 8. VS Extension Command Example

```csharp
// ContainerizationAssist.VisualStudio/Commands/AnalyzeProjectCommand.cs

using Community.VisualStudio.Toolkit;
using Microsoft.VisualStudio.Shell;

namespace ContainerizationAssist.VisualStudio.Commands;

[Command(PackageIds.AnalyzeProjectCommand)]
internal sealed class AnalyzeProjectCommand : BaseCommand<AnalyzeProjectCommand>
{
    protected override async Task ExecuteAsync(OleMenuCmdEventArgs e)
    {
        await Package.JoinableTaskFactory.SwitchToMainThreadAsync();

        var project = await VS.Solutions.GetActiveProjectAsync();
        if (project == null)
        {
            await VS.MessageBox.ShowWarningAsync("No project selected", "Please select a project to analyze.");
            return;
        }

        var projectPath = Path.GetDirectoryName(project.FullPath);
        if (string.IsNullOrEmpty(projectPath))
        {
            await VS.MessageBox.ShowErrorAsync("Error", "Could not determine project path.");
            return;
        }

        // Show progress
        await VS.StatusBar.ShowMessageAsync("Analyzing project...");

        try
        {
            var result = await ContainerizationSdk.AnalyzeRepositoryAsync(projectPath);

            await result.MatchAsync(
                async analysis =>
                {
                    await VS.StatusBar.ShowMessageAsync($"Analysis complete: {analysis.Modules.Count} module(s) detected");

                    // Show results in output window
                    var outputWindow = await VS.Windows.GetOutputWindowPaneAsync(Community.VisualStudio.Toolkit.Windows.VSOutputWindowPane.General);
                    await outputWindow.WriteLineAsync($"\n=== Containerization Analysis ===");
                    await outputWindow.WriteLineAsync(analysis.Summary ?? "Analysis complete");

                    foreach (var module in analysis.Modules)
                    {
                        await outputWindow.WriteLineAsync($"\n- {module.Name}: {module.Language}");
                        if (module.Frameworks?.Any() == true)
                        {
                            await outputWindow.WriteLineAsync($"  Frameworks: {string.Join(", ", module.Frameworks.Select(f => f.Name))}");
                        }
                    }

                    return true;
                },
                async failure =>
                {
                    await VS.StatusBar.ShowMessageAsync("Analysis failed");
                    await VS.MessageBox.ShowErrorAsync("Analysis Failed", failure.Message);
                    return false;
                });
        }
        catch (Exception ex)
        {
            await VS.StatusBar.ShowMessageAsync("Analysis failed");
            await VS.MessageBox.ShowErrorAsync("Error", ex.Message);
        }
    }
}
```

### 9. Tool Window for Container Dashboard

```csharp
// ContainerizationAssist.VisualStudio/ToolWindows/ContainerDashboardWindow.cs

using Community.VisualStudio.Toolkit;
using Microsoft.VisualStudio.Shell;
using System.Runtime.InteropServices;
using System.Windows;

namespace ContainerizationAssist.VisualStudio.ToolWindows;

public class ContainerDashboardWindow : BaseToolWindow<ContainerDashboardWindow>
{
    public override string GetTitle(int toolWindowId) => "Container Dashboard";

    public override Type PaneType => typeof(Pane);

    public override Task<FrameworkElement> CreateAsync(int toolWindowId, CancellationToken cancellationToken)
    {
        return Task.FromResult<FrameworkElement>(new ContainerDashboardControl());
    }

    [Guid("YOUR-GUID-HERE")]
    internal class Pane : ToolWindowPane
    {
        public Pane()
        {
            BitmapImageMoniker = KnownMonikers.Docker;
        }
    }
}
```

## Implementation Phases

### Phase 1: Core SDK (4-6 weeks)

| Week | Focus | Deliverables |
|------|-------|--------------|
| 1-2 | Foundation | Project structure, Result pattern, base interfaces, config parsers |
| 3-4 | Repository Analysis | `RepositoryAnalyzer`, all config parsers (csproj, package.json, pom.xml, etc.) |
| 5-6 | Knowledge Base | Embedded JSON resources, `KnowledgeMatcher`, Dockerfile recommendation logic |

### Phase 2: Docker Integration (3-4 weeks)

| Week | Focus | Deliverables |
|------|-------|--------------|
| 7-8 | Docker Client | `DockerClientWrapper` using Docker.DotNet, build/tag operations |
| 9-10 | Security Scanning | `TrivyRunner`, output parsing, remediation guidance |

### Phase 3: Visual Studio Extension (2-3 weeks)

| Week | Focus | Deliverables |
|------|-------|--------------|
| 11 | Extension Setup | VSIX project, menu commands, basic integration |
| 12-13 | Tool Windows | Container dashboard, progress UI, output formatting |

### Phase 4: Testing & Documentation (2 weeks)

| Week | Focus | Deliverables |
|------|-------|--------------|
| 14 | Testing | Unit tests, integration tests, E2E scenarios |
| 15 | Documentation | API docs, samples, VS extension guide |

**Total Estimated Duration: 13-15 weeks**

## NuGet Package Structure

```xml
<!-- ContainerizationAssist.Sdk.csproj -->
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFrameworks>net8.0;net6.0;netstandard2.1</TargetFrameworks>
    <PackageId>ContainerizationAssist.Sdk</PackageId>
    <Version>1.0.0</Version>
    <Authors>Your Team</Authors>
    <Description>Native .NET SDK for AI-powered containerization</Description>
    <PackageTags>docker;kubernetes;containerization;devops</PackageTags>
    <GenerateDocumentationFile>true</GenerateDocumentationFile>
    <EmbedUntrackedSources>true</EmbedUntrackedSources>
  </PropertyGroup>

  <ItemGroup>
    <!-- Core dependencies -->
    <PackageReference Include="Docker.DotNet" Version="3.125.15" />
    <PackageReference Include="KubernetesClient" Version="18.0.13" />
    <PackageReference Include="OneOf" Version="3.0.271" />
    <PackageReference Include="Microsoft.Extensions.DependencyInjection.Abstractions" Version="8.0.0" />
    <PackageReference Include="Microsoft.Extensions.Logging.Abstractions" Version="8.0.0" />

    <!-- For TOML parsing (pyproject.toml, Cargo.toml) -->
    <PackageReference Include="Tomlyn" Version="0.17.0" />

    <!-- For XML parsing (pom.xml, csproj) -->
    <PackageReference Include="System.Xml.Linq" Version="4.3.0" />
  </ItemGroup>

  <ItemGroup>
    <!-- Embedded knowledge base resources -->
    <EmbeddedResource Include="Knowledge\Data\*.json" />
  </ItemGroup>
</Project>
```

## API Comparison: TypeScript vs .NET

| TypeScript SDK | .NET SDK |
|----------------|----------|
| `analyzeRepo({ repositoryPath })` | `ContainerizationSdk.AnalyzeRepositoryAsync(path)` |
| `generateDockerfile({ repositoryPath, targetPlatform })` | `ContainerizationSdk.GenerateDockerfileAsync(path, platform)` |
| `buildImage({ path, imageName, tags })` | `ContainerizationSdk.BuildImageAsync(path, imageName, tags)` |
| `scanImage({ imageId, severity })` | `ContainerizationSdk.ScanImageAsync(imageId, severity)` |
| `Result<T>` with `ok`/`value`/`error` | `Result<T>` with `IsSuccess`/`Value`/`Error` |
| `result.ok ? result.value : result.error` | `result.Match(onSuccess, onFailure)` |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Docker.DotNet API changes | Low | Medium | Pin specific version, abstract behind wrapper |
| Trivy CLI output format changes | Medium | Low | Robust parsing with fallbacks, version detection |
| Knowledge base sync with TypeScript | Medium | Medium | Automate sync from TypeScript JSON files |
| VS extension compatibility | Low | High | Test across VS 2019/2022 versions |
| .NET version fragmentation | Medium | Medium | Multi-target netstandard2.1, net6.0, net8.0 |

## Success Metrics

1. **Functional Parity**: All 4 tools produce equivalent results to TypeScript SDK
2. **Performance**: Analysis completes within 2x of TypeScript version
3. **Integration**: Works in VS 2019, VS 2022, and standalone .NET apps
4. **Adoption**: Customer successfully integrates into their VS extension

## Future Enhancements

1. **Policy System**: Port Rego/CEL policy evaluation to .NET
2. **Additional Tools**: Add remaining tools (fix-dockerfile, deploy, verify-deploy)
3. **Kubernetes Integration**: Full K8s manifest generation using KubernetesClient
4. **VS Code Extension**: Port to VS Code using C# Dev Kit patterns
5. **AI Integration**: Optional Azure OpenAI/local model integration for advanced recommendations

## Appendix: Key Resources

- [Docker.DotNet GitHub](https://github.com/dotnet/Docker.DotNet)
- [KubernetesClient GitHub](https://github.com/kubernetes-client/csharp)
- [OneOf Library](https://github.com/mcintyre321/OneOf)
- [Community.VisualStudio.Toolkit](https://github.com/VsixCommunity/Community.VisualStudio.Toolkit)
- [VS Extensibility Cookbook](https://www.vsixcookbook.com/)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [Microsoft Learn: VS Extension Development](https://learn.microsoft.com/en-us/visualstudio/extensibility/)
