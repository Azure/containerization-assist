# MCP Tools Implementation Tasks

## Quick Wins (Week 1)
These can be implemented immediately with minimal risk:

### 1. Fix Simple Tool Implementations
**Priority: HIGH | Effort: LOW | Impact: HIGH**

#### 1.1 Fix push_image Tool
```go
// In tool_registration.go, update LazyPushTool.Execute()
// Replace mock implementation with:
- Get DockerClient from services
- Call actual Docker push API
- Handle authentication properly
- Return real results
```

#### 1.2 Fix list_sessions Tool
```go
// Query real SessionStore instead of returning mock data
sessions, err := t.sessionStore.List(ctx, filter)
```

#### 1.3 Fix ping/server_status Tools
```go
// Get actual metrics from service container
- Server uptime from start time
- Active sessions from SessionStore
- Tool count from ToolRegistry
- Memory/CPU from runtime
```

### 2. Implement Language Detection
**Priority: HIGH | Effort: MEDIUM | Impact: HIGH**

```go
// In analyze_consolidated.go
func (cmd *ConsolidatedAnalyzeCommand) detectLanguageByExtension(ctx context.Context, workspaceDir string) (map[string]int, error) {
    extensions := map[string]string{
        ".go": "go", ".js": "javascript", ".py": "python",
        ".java": "java", ".cs": "csharp", ".rb": "ruby",
    }

    counts := make(map[string]int)
    err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return nil
        }
        ext := filepath.Ext(path)
        if lang, ok := extensions[ext]; ok {
            counts[lang]++
        }
        return nil
    })

    return counts, err
}
```

## Core Functionality (Weeks 2-3)

### 3. Implement File Access Service
**Priority: HIGH | Effort: HIGH | Impact: CRITICAL**

See detailed specification in FILE_ACCESS_SERVICE_SPEC.md

Tasks:
- [ ] Create FileAccessService interface
- [ ] Implement security validation
- [ ] Create read_file MCP tool
- [ ] Create list_directory MCP tool
- [ ] Create file_exists MCP tool
- [ ] Add caching layer
- [ ] Write security tests

### 4. Framework Detection Implementation
**Priority: MEDIUM | Effort: MEDIUM | Impact: HIGH**

#### 4.1 Go Framework Detection
```go
func (cmd *ConsolidatedAnalyzeCommand) detectGoFramework(result *analyze.AnalysisResult, workspaceDir string) error {
    goModPath := filepath.Join(workspaceDir, "go.mod")
    content, err := os.ReadFile(goModPath)
    if err != nil {
        return nil // No go.mod, not a Go project
    }

    frameworks := map[string]string{
        "gin-gonic/gin": "gin",
        "labstack/echo": "echo",
        "gofiber/fiber": "fiber",
        "gorilla/mux": "gorilla",
    }

    for pattern, name := range frameworks {
        if strings.Contains(string(content), pattern) {
            result.Framework = analyze.Framework{
                Name: name,
                Type: analyze.FrameworkTypeWeb,
                Confidence: analyze.ConfidenceHigh,
            }
            return nil
        }
    }
    return nil
}
```

#### 4.2 JavaScript Framework Detection
```go
func (cmd *ConsolidatedAnalyzeCommand) detectJSFramework(result *analyze.AnalysisResult, workspaceDir string) error {
    packagePath := filepath.Join(workspaceDir, "package.json")
    content, err := os.ReadFile(packagePath)
    if err != nil {
        return nil
    }

    var pkg map[string]interface{}
    if err := json.Unmarshal(content, &pkg); err != nil {
        return err
    }

    // Check dependencies
    deps := make(map[string]bool)
    if d, ok := pkg["dependencies"].(map[string]interface{}); ok {
        for k := range d {
            deps[k] = true
        }
    }
    if d, ok := pkg["devDependencies"].(map[string]interface{}); ok {
        for k := range d {
            deps[k] = true
        }
    }

    // Detect framework
    switch {
    case deps["react"]:
        result.Framework.Name = "react"
    case deps["@angular/core"]:
        result.Framework.Name = "angular"
    case deps["vue"]:
        result.Framework.Name = "vue"
    case deps["express"]:
        result.Framework.Name = "express"
    case deps["next"]:
        result.Framework.Name = "nextjs"
    }

    return nil
}
```

### 5. Dependency Analysis
**Priority: MEDIUM | Effort: MEDIUM | Impact: MEDIUM**

#### 5.1 Go Dependencies
```go
func (cmd *ConsolidatedAnalyzeCommand) analyzeGoDependencies(workspaceDir string) ([]analyze.Dependency, error) {
    goModPath := filepath.Join(workspaceDir, "go.mod")
    content, err := os.ReadFile(goModPath)
    if err != nil {
        return nil, err
    }

    var deps []analyze.Dependency
    lines := strings.Split(string(content), "\n")
    inRequire := false

    for _, line := range lines {
        line = strings.TrimSpace(line)

        if line == "require (" {
            inRequire = true
            continue
        }
        if inRequire && line == ")" {
            break
        }

        if inRequire && line != "" {
            parts := strings.Fields(line)
            if len(parts) >= 2 {
                deps = append(deps, analyze.Dependency{
                    Name:    parts[0],
                    Version: parts[1],
                    Type:    analyze.DependencyTypeDirect,
                })
            }
        }
    }

    return deps, nil
}
```

### 6. Database Detection
**Priority: HIGH | Effort: HIGH | Impact: HIGH**

```go
func (cmd *ConsolidatedAnalyzeCommand) detectDatabases(ctx context.Context, workspaceDir string) ([]analyze.Database, error) {
    var databases []analyze.Database
    patterns := map[string]string{
        "postgres://": "postgresql",
        "mysql://": "mysql",
        "mongodb://": "mongodb",
        "redis://": "redis",
        "DATABASE_URL": "unknown",
    }

    // Search for database connections in common files
    searchFiles := []string{
        ".env", ".env.example", "config.yaml", "config.json",
        "application.properties", "appsettings.json",
    }

    for _, file := range searchFiles {
        path := filepath.Join(workspaceDir, file)
        content, err := os.ReadFile(path)
        if err != nil {
            continue
        }

        contentStr := string(content)
        for pattern, dbType := range patterns {
            if strings.Contains(contentStr, pattern) {
                databases = append(databases, analyze.Database{
                    Type: dbType,
                    Detected: true,
                })
            }
        }
    }

    // Also check for ORM imports in code
    // ... implementation

    return databases, nil
}
```

## Advanced Features (Weeks 4-5)

### 7. Enhanced AutoFixHelper
**Priority: MEDIUM | Effort: LOW | Impact: MEDIUM**

Add new fix strategies:
```go
// In auto_fix_helper.go registerCommonFixes()

// Fix for port conflicts
h.fixes["port_conflict"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
    if strings.Contains(err.Error(), "port is already allocated") {
        // Try alternative ports: 8081, 8082, 3001, 5001
    }
}

// Fix for missing base image
h.fixes["base_image_not_found"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
    if strings.Contains(err.Error(), "pull access denied") {
        // Suggest alternative base images
    }
}

// Fix for invalid Kubernetes resources
h.fixes["k8s_resource_invalid"] = func(ctx context.Context, tool api.Tool, args interface{}, err error) (interface{}, error) {
    if strings.Contains(err.Error(), "Invalid CPU") {
        // Adjust resource limits
    }
}
```

### 8. Generate Dockerfile Implementation
**Priority: MEDIUM | Effort: MEDIUM | Impact: HIGH**

```go
// Create proper LazyGenerateDockerfileTool implementation
func (t *LazyGenerateDockerfileTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
    // Get language from input or detect it
    language := input.Data["language"].(string)
    if language == "" {
        // Run analysis first to detect language
    }

    // Get template service
    templateService := t.services.TemplateService()

    // Generate Dockerfile using templates
    dockerfile, err := templateService.GenerateDockerfile(language, input.Data)
    if err != nil {
        return api.ToolOutput{Success: false, Error: err.Error()}, nil
    }

    // Save to workspace
    workspaceDir := t.getWorkspaceDir(ctx, input.SessionID)
    dockerfilePath := filepath.Join(workspaceDir, "Dockerfile")

    if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
        return api.ToolOutput{Success: false, Error: err.Error()}, nil
    }

    return api.ToolOutput{
        Success: true,
        Data: map[string]interface{}{
            "dockerfile": dockerfile,
            "path": dockerfilePath,
            "language": language,
        },
    }, nil
}
```

## Testing & Documentation (Ongoing)

### 9. Unit Tests
**Priority: HIGH | Effort: MEDIUM | Impact: HIGH**

For each implementation:
- [ ] Test happy path
- [ ] Test error cases
- [ ] Test edge cases
- [ ] Test security boundaries
- [ ] Mock dependencies

### 10. Integration Tests
**Priority: HIGH | Effort: HIGH | Impact: HIGH**

```go
func TestAnalyzeBuildDeployWorkflow(t *testing.T) {
    // Create test project
    // Run analyze tool
    // Verify results saved to session
    // Run build tool using analysis results
    // Verify build succeeds
    // Run deploy tool
    // Verify manifests generated
}
```

### 11. Documentation Updates
**Priority: MEDIUM | Effort: LOW | Impact: MEDIUM**

- [ ] Update tool guides with new features
- [ ] Document file access security model
- [ ] Create troubleshooting guide
- [ ] Update API documentation
- [ ] Add example workflows

## Implementation Checklist

### Week 1: Quick Wins
- [ ] Fix push_image real implementation
- [ ] Fix list_sessions to query store
- [ ] Fix ping/server_status with real data
- [ ] Implement basic language detection
- [ ] Add unit tests for fixes

### Week 2: File Access
- [ ] Implement FileAccessService
- [ ] Create file access MCP tools
- [ ] Add security validation
- [ ] Integrate with analyze command
- [ ] Security testing

### Week 3: Analysis Features
- [ ] Framework detection (Go, JS, Python, Java)
- [ ] Dependency parsing
- [ ] Database detection
- [ ] Port detection
- [ ] Integration tests

### Week 4: Polish
- [ ] Enhanced AutoFixHelper strategies
- [ ] Proper Dockerfile generation
- [ ] Performance optimization
- [ ] Documentation updates
- [ ] Load testing

### Week 5: Deployment
- [ ] Feature flags for rollout
- [ ] Migration guide
- [ ] Performance benchmarks
- [ ] Security audit
- [ ] Production deployment

## Success Criteria

1. **All 9 tools have real implementations** (no mock data)
2. **File access tools integrated** and secure
3. **Analysis matches pipeline quality** (languages, frameworks, databases detected)
4. **AutoFix handles 80%+ of common errors**
5. **Performance <300μs P95** per tool
6. **80%+ test coverage** on new code
7. **Zero security vulnerabilities** in file access
8. **Documentation complete** and accurate

## Notes for Implementers

1. **Start with quick wins** - they provide immediate value
2. **Security first** for file access - this is critical
3. **Use existing patterns** - follow the consolidated command structure
4. **Test as you go** - don't leave testing until the end
5. **Update docs immediately** - keep documentation in sync
6. **Ask for reviews early** - get feedback on approach
7. **Monitor performance** - use benchmarks to catch regressions

This task list provides a clear path from the current state to a fully functional MCP tool implementation that matches the pipeline's capabilities while maintaining the architectural benefits of the MCP approach.
