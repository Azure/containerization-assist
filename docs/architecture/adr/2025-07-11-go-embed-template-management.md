# ADR-002: Go Embed Template Management

Date: 2025-07-11
Status: Accepted
Context: Container Kit needed a reliable way to distribute Dockerfile templates and configuration files with the binary. External template files created deployment complexity, version synchronization issues, and potential runtime failures when templates were missing or outdated. The system required embedded templates that would be available at runtime without external dependencies.

Decision: Use Go's `embed` package to embed all Dockerfile templates, configuration files, and static assets directly into the compiled binary. This eliminates external file dependencies, ensures version consistency, and simplifies deployment while maintaining the ability to customize templates.

## Architecture Details

### Go Embed Implementation
```go
// pkg/mcp/infrastructure/prompts/manager.go
//go:embed templates/*.yaml
var templates embed.FS

// Template access at runtime
func NewManager(logger *slog.Logger) *Manager {
    return &Manager{
        templates: templates,
        logger:    logger,
    }
}

// Template access at runtime
func GetDockerfileTemplate(language string, framework string) (string, error) {
    templatePath := fmt.Sprintf("templates/%s/%s/Dockerfile", language, framework)
    content, err := Templates.ReadFile(templatePath)
    if err != nil {
        return "", fmt.Errorf("template not found: %s", templatePath)
    }
    return string(content), nil
}
```

### Template Structure
```
pkg/mcp/infrastructure/prompts/templates/
├── analyze.yaml              # Repository analysis prompts
├── build.yaml               # Docker build prompts
├── deploy.yaml              # Kubernetes deployment prompts
├── scan.yaml                # Security scanning prompts
├── manifest.yaml            # Manifest generation prompts
├── tag.yaml                 # Image tagging prompts
├── push.yaml                # Registry push prompts
├── cluster.yaml             # Cluster setup prompts
├── verify.yaml              # Verification prompts
└── workflow.yaml            # General workflow prompts
```

### Template Selection Logic
```go
type TemplateSelector struct {
    templates embed.FS
    logger    *slog.Logger
}

func (ts *TemplateSelector) SelectTemplate(ctx context.Context, analysis *RepoAnalysis) (*TemplateInfo, error) {
    // Rule-based template selection
    language := analysis.PrimaryLanguage
    framework := analysis.DetectedFramework
    
    // Try specific framework template first
    if framework != "" {
        template, err := ts.getTemplate(language, framework)
        if err == nil {
            return &TemplateInfo{
                Path:      fmt.Sprintf("%s/%s", language, framework),
                Content:   template,
                Language:  language,
                Framework: framework,
            }, nil
        }
    }
    
    // Fallback to language standard template
    template, err := ts.getTemplate(language, "standard")
    if err != nil {
        // Final fallback to default template
        template, err = ts.getTemplate("default", "")
        if err != nil {
            return nil, fmt.Errorf("no suitable template found")
        }
    }
    
    return &TemplateInfo{
        Path:     language,
        Content:  template,
        Language: language,
    }, nil
}
```

## Previous Template Management Issues

### Before: External Template Files
- **Runtime Dependencies**: Templates required as external files
- **Version Synchronization**: Templates could be out of sync with binary
- **Deployment Complexity**: Need to ship templates alongside binary  
- **Missing File Errors**: Runtime failures when templates missing
- **Path Resolution**: Complex logic for finding template files
- **Distribution Issues**: Templates could be corrupted or modified

### Problems Addressed
- **Deployment Simplification**: Single binary contains everything needed
- **Version Consistency**: Templates always match the binary version
- **Reliability**: No runtime file missing errors
- **Security**: Templates cannot be modified after compilation
- **Portability**: Binary works anywhere without external dependencies

## Key Features

### Compile-Time Embedding
- **Build Integration**: Templates embedded during compilation
- **Version Locking**: Templates locked to specific binary version
- **Size Optimization**: Only used templates included in binary
- **Integrity**: Templates cannot be tampered with at runtime

### Template Categories
1. **Language Templates**: Base templates for each programming language
2. **Framework Templates**: Optimized templates for specific frameworks
3. **Multi-stage Templates**: Complex build patterns with optimization
4. **Security Templates**: Distroless and minimal base images
5. **Default Templates**: Generic fallback templates

### Template Features
- **Multi-stage Builds**: Optimized build and runtime stages
- **Security Hardening**: Non-root users, minimal attack surface
- **Layer Optimization**: Efficient Docker layer caching
- **Build Arguments**: Parameterized templates for customization
- **Health Checks**: Built-in container health monitoring

## Template Examples

### Go Gin Framework Template
```dockerfile
# pkg/core/docker/templates/go/gin/Dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM gcr.io/distroless/static-debian12:latest

COPY --from=builder /app/main /app/main
EXPOSE 8080

USER 65532:65532
ENTRYPOINT ["/app/main"]
```

### Python FastAPI Template
```dockerfile
# pkg/core/docker/templates/python/fastapi/Dockerfile
FROM python:3.12-slim AS builder

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

FROM python:3.12-slim AS runtime

RUN groupadd -r appuser && useradd -r -g appuser appuser
WORKDIR /app

COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY . .

USER appuser
EXPOSE 8000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

### Template Customization
```go
func (ts *TemplateSelector) CustomizeTemplate(template string, config *CustomizationConfig) string {
    // Apply port customization
    if config.Port != 0 {
        template = strings.ReplaceAll(template, "EXPOSE 8080", fmt.Sprintf("EXPOSE %d", config.Port))
    }
    
    // Apply base image customization
    if config.BaseImage != "" {
        lines := strings.Split(template, "\n")
        for i, line := range lines {
            if strings.HasPrefix(line, "FROM ") && !strings.Contains(line, " AS ") {
                lines[i] = fmt.Sprintf("FROM %s", config.BaseImage)
                break
            }
        }
        template = strings.Join(lines, "\n")
    }
    
    // Apply environment variables
    if len(config.EnvVars) > 0 {
        var envLines []string
        for key, value := range config.EnvVars {
            envLines = append(envLines, fmt.Sprintf("ENV %s=%s", key, value))
        }
        
        // Insert ENV lines before EXPOSE
        template = strings.Replace(template, "EXPOSE", strings.Join(envLines, "\n")+"\n\nEXPOSE", 1)
    }
    
    return template
}
```

## Consequences

### Benefits
- **Single Binary Deployment**: No external template files needed
- **Version Consistency**: Templates always match binary version
- **Improved Reliability**: No runtime file missing errors
- **Security**: Templates cannot be modified after compilation
- **Simplified Distribution**: One file contains everything
- **Better Testing**: Templates included in test builds
- **Offline Operation**: Works without network or file system access

### Trade-offs
- **Binary Size**: Larger binary due to embedded content
- **Update Process**: Template changes require binary recompilation
- **Customization Limits**: Less flexibility than external templates
- **Build Time**: Slightly longer build times for embedding

### Performance Impact
- **Memory Usage**: Templates loaded into memory when accessed
- **Access Speed**: Faster access than file system reads
- **Startup Time**: No initial template loading delay
- **Cache Efficiency**: Templates cached in memory after first access

## Template Management

### Development Workflow
1. **Template Creation**: Add new templates to appropriate directories
2. **Testing**: Templates included automatically in test builds
3. **Validation**: Template syntax and best practices validation
4. **Documentation**: Template usage and customization docs

### Template Standards
- **Security First**: All templates use non-root users
- **Multi-stage**: Separate build and runtime stages
- **Layer Optimization**: Efficient Docker layer structure
- **Health Checks**: Built-in container health monitoring
- **Build Arguments**: Support for build-time customization

### Template Categories
1. **Production Ready**: Optimized for production deployment
2. **Development**: Enhanced debugging and development features
3. **Minimal**: Distroless and minimal attack surface
4. **Legacy**: Support for older framework versions

## Implementation Status
- ✅ Go embed integration for template storage
- ✅ Template selection logic based on language/framework detection
- ✅ Template customization system for ports, base images, env vars
- ✅ Multi-stage templates for production optimization
- ✅ Security-hardened templates with non-root users
- ✅ Framework-specific optimizations (Gin, FastAPI, Spring Boot, etc.)
- ✅ Fallback system (framework → language → default)
- ✅ Template validation and testing integration

## Template Governance
1. **Security Review**: All templates reviewed for security best practices
2. **Performance Testing**: Templates tested for build and runtime performance
3. **Framework Updates**: Regular updates for framework best practices
4. **Community Input**: Template improvements based on community feedback

## Related ADRs
- ADR-001: Single Workflow Tool Architecture (template integration in workflow)
- ADR-003: Manual Dependency Injection (template service dependencies)
- ADR-004: Unified Rich Error System (template error handling)
- ADR-006: Four-Layer MCP Architecture (templates in infrastructure layer)