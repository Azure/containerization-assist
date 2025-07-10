# File Access Service Technical Specification

## Overview

The File Access Service provides secure, session-scoped file system access for MCP tools, enabling them to explore and analyze repositories similar to the pipeline stages.

## Architecture

### Service Interface

```go
package services

import (
    "context"
    "io/fs"
    "time"
)

// FileInfo represents file metadata
type FileInfo struct {
    Name        string    `json:"name"`
    Path        string    `json:"path"`
    Size        int64     `json:"size"`
    Mode        fs.FileMode `json:"mode"`
    ModTime     time.Time `json:"mod_time"`
    IsDir       bool      `json:"is_dir"`
    Permissions string    `json:"permissions"`
}

// FileAccessService provides secure file system operations
type FileAccessService interface {
    // ReadFile reads a file's contents within the session workspace
    ReadFile(ctx context.Context, sessionID, relativePath string) (string, error)

    // ReadFileWithLimit reads up to maxBytes from a file
    ReadFileWithLimit(ctx context.Context, sessionID, relativePath string, maxBytes int64) (string, bool, error)

    // ListDirectory lists files and directories
    ListDirectory(ctx context.Context, sessionID, relativePath string) ([]FileInfo, error)

    // FileExists checks if a file exists
    FileExists(ctx context.Context, sessionID, relativePath string) (bool, error)

    // GetFileInfo retrieves file metadata
    GetFileInfo(ctx context.Context, sessionID, relativePath string) (*FileInfo, error)

    // GetFileTree generates a tree representation of a directory
    GetFileTree(ctx context.Context, sessionID, rootPath string, maxDepth int) (string, error)

    // SearchFiles searches for files matching a pattern
    SearchFiles(ctx context.Context, sessionID, pattern string, searchPath string) ([]string, error)
}
```

### Implementation

```go
package infra

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "github.com/Azure/container-kit/pkg/mcp/application/services"
    "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

type fileAccessService struct {
    sessionStore services.SessionStore
    logger       *slog.Logger
    maxFileSize  int64
    allowedExts  map[string]bool
    blockedPaths []string
}

// NewFileAccessService creates a new file access service
func NewFileAccessService(sessionStore services.SessionStore, logger *slog.Logger) services.FileAccessService {
    return &fileAccessService{
        sessionStore: sessionStore,
        logger:       logger,
        maxFileSize:  10 * 1024 * 1024, // 10MB default
        allowedExts:  defaultAllowedExtensions(),
        blockedPaths: defaultBlockedPaths(),
    }
}
```

### Security Measures

#### Path Validation

```go
func (f *fileAccessService) validatePath(sessionID, relativePath string) (string, error) {
    // Get session workspace
    session, err := f.sessionStore.Get(context.Background(), sessionID)
    if err != nil {
        return "", errors.NewError().
            Code(errors.CodePermissionDenied).
            Message("invalid session").
            Build()
    }

    workspaceDir := session.WorkspaceDir
    if workspaceDir == "" {
        return "", errors.NewError().
            Code(errors.CodeInternalError).
            Message("session has no workspace").
            Build()
    }

    // Clean and resolve the path
    cleanPath := filepath.Clean(relativePath)

    // Prevent path traversal
    if strings.Contains(cleanPath, "..") {
        return "", errors.NewError().
            Code(errors.CodePermissionDenied).
            Message("path traversal not allowed").
            Build()
    }

    // Build absolute path
    absPath := filepath.Join(workspaceDir, cleanPath)

    // Verify the path is within workspace
    if !strings.HasPrefix(absPath, workspaceDir) {
        return "", errors.NewError().
            Code(errors.CodePermissionDenied).
            Message("path outside workspace").
            Build()
    }

    // Check blocked paths
    for _, blocked := range f.blockedPaths {
        if strings.Contains(cleanPath, blocked) {
            return "", errors.NewError().
                Code(errors.CodePermissionDenied).
                Message("access to this path is blocked").
                Build()
        }
    }

    return absPath, nil
}
```

#### File Type Restrictions

```go
func defaultAllowedExtensions() map[string]bool {
    return map[string]bool{
        // Source code
        ".go": true, ".js": true, ".ts": true, ".py": true, ".java": true,
        ".c": true, ".cpp": true, ".cs": true, ".rb": true, ".php": true,

        // Config files
        ".json": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true,
        ".xml": true, ".properties": true, ".env": true, ".config": true,

        // Build files
        ".dockerfile": true, "Dockerfile": true, ".dockerignore": true,
        "Makefile": true, ".makefile": true, ".gradle": true, ".maven": true,

        // Documentation
        ".md": true, ".txt": true, ".rst": true, ".adoc": true,

        // Package files
        "go.mod": true, "go.sum": true, "package.json": true, "package-lock.json": true,
        "requirements.txt": true, "pom.xml": true, "build.gradle": true,
    }
}

func defaultBlockedPaths() []string {
    return []string{
        ".git/objects",
        ".git/hooks",
        "node_modules",
        "__pycache__",
        ".env.local",
        ".env.production",
        "secrets",
        "credentials",
        ".ssh",
        ".gnupg",
    }
}
```

### MCP Tool Implementations

#### read_file Tool

```go
package commands

type ReadFileTool struct {
    fileAccess services.FileAccessService
    logger     *slog.Logger
}

func (t *ReadFileTool) Name() string {
    return "read_file"
}

func (t *ReadFileTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Extract parameters
    filePath, ok := input.Data["path"].(string)
    if !ok || filePath == "" {
        return api.ToolOutput{
            Success: false,
            Error:   "path parameter is required",
        }, nil
    }

    // Read file
    content, err := t.fileAccess.ReadFile(ctx, input.SessionID, filePath)
    if err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, nil
    }

    return api.ToolOutput{
        Success: true,
        Data: map[string]interface{}{
            "path":    filePath,
            "content": content,
            "size":    len(content),
        },
    }, nil
}
```

#### list_directory Tool

```go
type ListDirectoryTool struct {
    fileAccess services.FileAccessService
    logger     *slog.Logger
}

func (t *ListDirectoryTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Extract parameters
    dirPath, _ := input.Data["path"].(string)
    if dirPath == "" {
        dirPath = "." // Default to workspace root
    }

    // List directory
    files, err := t.fileAccess.ListDirectory(ctx, input.SessionID, dirPath)
    if err != nil {
        return api.ToolOutput{
            Success: false,
            Error:   err.Error(),
        }, nil
    }

    // Convert to output format
    fileList := make([]map[string]interface{}, len(files))
    for i, f := range files {
        fileList[i] = map[string]interface{}{
            "name":     f.Name,
            "path":     f.Path,
            "size":     f.Size,
            "is_dir":   f.IsDir,
            "modified": f.ModTime,
        }
    }

    return api.ToolOutput{
        Success: true,
        Data: map[string]interface{}{
            "path":  dirPath,
            "files": fileList,
            "count": len(files),
        },
    }, nil
}
```

### Integration with Analyze Command

```go
// Updated analyze command using file access
func (cmd *ConsolidatedAnalyzeCommand) detectLanguageByExtension(ctx context.Context, workspaceDir string) (map[string]int, error) {
    languageMap := make(map[string]int)

    // Use file access service to scan directory
    files, err := cmd.fileAccess.SearchFiles(ctx, cmd.sessionID, "*", ".")
    if err != nil {
        return nil, err
    }

    // Count files by extension
    for _, file := range files {
        ext := strings.ToLower(filepath.Ext(file))

        // Map extensions to languages
        switch ext {
        case ".go":
            languageMap["go"]++
        case ".js", ".jsx", ".mjs":
            languageMap["javascript"]++
        case ".ts", ".tsx":
            languageMap["typescript"]++
        case ".py":
            languageMap["python"]++
        case ".java":
            languageMap["java"]++
        case ".cs":
            languageMap["csharp"]++
        case ".rb":
            languageMap["ruby"]++
        case ".php":
            languageMap["php"]++
        case ".rs":
            languageMap["rust"]++
        case ".cpp", ".cc", ".cxx":
            languageMap["cpp"]++
        case ".c":
            languageMap["c"]++
        }
    }

    return languageMap, nil
}

// Detect Go framework by examining go.mod and imports
func (cmd *ConsolidatedAnalyzeCommand) detectGoFramework(result *analyze.AnalysisResult, workspaceDir string) error {
    // Check if go.mod exists
    goModExists, err := cmd.fileAccess.FileExists(ctx, cmd.sessionID, "go.mod")
    if err != nil || !goModExists {
        return nil
    }

    // Read go.mod
    goModContent, err := cmd.fileAccess.ReadFile(ctx, cmd.sessionID, "go.mod")
    if err != nil {
        return err
    }

    // Check for common frameworks
    frameworks := map[string]string{
        "github.com/gin-gonic/gin":        "gin",
        "github.com/labstack/echo":        "echo",
        "github.com/gofiber/fiber":        "fiber",
        "github.com/gorilla/mux":          "gorilla",
        "github.com/go-chi/chi":           "chi",
        "github.com/kataras/iris":         "iris",
        "github.com/revel/revel":          "revel",
        "github.com/astaxie/beego":        "beego",
        "google.golang.org/grpc":          "grpc",
        "github.com/go-kit/kit":           "go-kit",
    }

    for pkg, framework := range frameworks {
        if strings.Contains(goModContent, pkg) {
            result.Framework = analyze.Framework{
                Name:       framework,
                Type:       analyze.FrameworkTypeWeb,
                Version:    extractVersion(goModContent, pkg),
                Confidence: analyze.ConfidenceHigh,
            }
            return nil
        }
    }

    // No framework detected, might be stdlib
    result.Framework = analyze.Framework{
        Name:       "stdlib",
        Type:       analyze.FrameworkTypeWeb,
        Confidence: analyze.ConfidenceMedium,
    }

    return nil
}
```

### Performance Optimizations

#### Caching Layer

```go
type cachedFileAccessService struct {
    services.FileAccessService
    cache    *lru.Cache
    cacheTTL time.Duration
}

func NewCachedFileAccessService(base services.FileAccessService) services.FileAccessService {
    cache, _ := lru.New(1000) // Cache up to 1000 file reads
    return &cachedFileAccessService{
        FileAccessService: base,
        cache:            cache,
        cacheTTL:         5 * time.Minute,
    }
}
```

#### Concurrent Directory Scanning

```go
func (f *fileAccessService) GetFileTree(ctx context.Context, sessionID, rootPath string, maxDepth int) (string, error) {
    // Use worker pool for concurrent scanning
    workerPool := make(chan struct{}, 10)
    var wg sync.WaitGroup
    var mu sync.Mutex
    var result strings.Builder

    // Scan directories concurrently
    var scanDir func(path string, depth int)
    scanDir = func(path string, depth int) {
        if depth > maxDepth {
            return
        }

        workerPool <- struct{}{}
        wg.Add(1)

        go func() {
            defer wg.Done()
            defer func() { <-workerPool }()

            files, err := f.ListDirectory(ctx, sessionID, path)
            if err != nil {
                return
            }

            mu.Lock()
            for _, file := range files {
                indent := strings.Repeat("  ", depth)
                if file.IsDir {
                    result.WriteString(fmt.Sprintf("%s[%s/]\n", indent, file.Name))
                } else {
                    result.WriteString(fmt.Sprintf("%s%s (%d bytes)\n", indent, file.Name, file.Size))
                }
            }
            mu.Unlock()

            // Recursively scan subdirectories
            for _, file := range files {
                if file.IsDir {
                    scanDir(filepath.Join(path, file.Name), depth+1)
                }
            }
        }()
    }

    scanDir(rootPath, 0)
    wg.Wait()

    return result.String(), nil
}
```

## Testing Strategy

### Unit Tests

```go
func TestFileAccessService_Security(t *testing.T) {
    tests := []struct {
        name        string
        path        string
        shouldError bool
        errorCode   string
    }{
        {
            name:        "path traversal attempt",
            path:        "../../../etc/passwd",
            shouldError: true,
            errorCode:   errors.CodePermissionDenied,
        },
        {
            name:        "absolute path attempt",
            path:        "/etc/passwd",
            shouldError: true,
            errorCode:   errors.CodePermissionDenied,
        },
        {
            name:        "blocked path",
            path:        ".git/objects/abc123",
            shouldError: true,
            errorCode:   errors.CodePermissionDenied,
        },
        {
            name:        "valid path",
            path:        "src/main.go",
            shouldError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

```go
func TestFileAccessIntegration(t *testing.T) {
    // Create test workspace
    tmpDir := t.TempDir()

    // Create test files
    testFiles := map[string]string{
        "main.go":           "package main\n\nfunc main() {}",
        "go.mod":            "module test\n\ngo 1.21",
        "config/app.yaml":   "port: 8080",
        ".env":              "DATABASE_URL=postgres://localhost",
    }

    for path, content := range testFiles {
        fullPath := filepath.Join(tmpDir, path)
        os.MkdirAll(filepath.Dir(fullPath), 0755)
        os.WriteFile(fullPath, []byte(content), 0644)
    }

    // Test file operations
    // ...
}
```

## Deployment Considerations

### Configuration

```yaml
fileAccess:
  maxFileSize: 10485760  # 10MB
  cacheTTL: 5m
  workerPoolSize: 10
  blockedPaths:
    - .git/objects
    - node_modules
    - __pycache__
  allowedExtensions:
    - .go
    - .js
    - .py
    # ... etc
```

### Monitoring

- File access metrics (reads/sec, cache hit rate)
- Security violations (blocked attempts)
- Performance metrics (P95 latency)
- Error rates by operation type

## Migration Path

1. Deploy file access service with feature flag
2. Update analyze tool to use file access
3. Monitor performance and security
4. Gradually enable for other tools
5. Deprecate direct file system access

This specification provides a secure, performant foundation for file access in MCP tools while maintaining session isolation and security boundaries.
