# Contributing to Container Kit

Thank you for your interest in contributing to Container Kit! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)
- [Architecture Guidelines](#architecture-guidelines)

## Code of Conduct

This project adheres to the Microsoft Open Source Code of Conduct. By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Docker
- kubectl (for Kubernetes features)
- kind (for local testing)
- Git

### Development Setup

#### Option 1: Development Container (Recommended)

The fastest way to get started with a fully configured environment:

1. **Fork and Clone**
   ```bash
   git clone https://github.com/YOUR-USERNAME/container-copilot.git
   cd container-copilot
   ```

2. **Open in Dev Container**
   - Install [VS Code](https://code.visualstudio.com/) and the [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
   - Open the repository in VS Code: `code .`
   - Click "Reopen in Container" when prompted
   - Wait for automatic setup (3-5 minutes first time)

3. **Start Contributing**
   ```bash
   # All tools are pre-installed and ready to use
   make mcp          # Build MCP server
   make test         # Run tests
   make lint         # Run linting
   ```

See [`.devcontainer/README.md`](.devcontainer/README.md) for complete devcontainer documentation.

#### Option 2: Local Development

If you prefer to set up your local environment manually:

1. **Fork and Clone**
   ```bash
   git clone https://github.com/YOUR-USERNAME/container-copilot.git
   cd container-copilot
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   ```

3. **Build the Project**
   ```bash
   # Build CLI version
   go build -o container-kit .
   
   # Build MCP server
   go build -tags mcp -o container-kit-mcp ./cmd/mcp-server
   ```

4. **Run Tests**
   ```bash
   # Run all tests
   go test ./...
   
   # Run MCP-specific tests
   go test -tags mcp ./pkg/mcp/...
   
   # Run with race detection
   go test -race ./...
   ```

5. **Verify Installation**
   ```bash
   ./container-kit --version
   ./container-kit-mcp --version
   ```

## Project Structure

```
container-copilot/
├── cmd/                    # Main applications
│   ├── mcp-server/        # MCP server binary
│   └── root.go            # CLI root command
├── pkg/                   # Core packages
│   ├── mcp/               # MCP server implementation
│   │   ├── core/          # Server core functionality
│   │   ├── tools/         # Atomic tools
│   │   ├── engine/        # Conversation engine
│   │   └── transport/     # Communication protocols
│   ├── pipeline/          # Legacy CLI pipeline
│   ├── core/              # Shared core functionality
│   └── clients/           # External service clients
├── docs/                  # Documentation
├── scripts/               # Build and utility scripts
└── templates/             # Dockerfile and manifest templates
```

### Key Components

- **MCP Server** (`pkg/mcp/`) - Primary focus for new development
- **Atomic Tools** (`pkg/mcp/tools/`) - Containerization operations
- **Conversation Engine** (`pkg/mcp/engine/`) - Guided workflows
- **Legacy CLI** (`pkg/pipeline/`) - Original CLI implementation

## Making Changes

### Before You Start

1. **Check Existing Issues**: Look for existing issues or discussions
2. **Create an Issue**: For significant changes, create an issue first
3. **Assign Yourself**: Assign the issue to yourself to avoid duplicated work

### Development Workflow

1. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**
   - Follow the coding standards below
   - Add tests for new functionality
   - Update documentation as needed

3. **Validate Your Changes**
   ```bash
   # Format code
   go fmt ./...
   
   # Run static analysis
   go vet ./...
   
   # Run tests
   go test ./...
   
   # Check for race conditions
   go test -race ./pkg/mcp/...
   
   # Clean up dependencies
   go mod tidy
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "feat: add new atomic tool for X"
   ```

### Commit Message Guidelines

Use conventional commits format:
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `test:` - Test additions/changes
- `refactor:` - Code refactoring
- `style:` - Code style changes
- `chore:` - Maintenance tasks

Examples:
```
feat: add rollback_deployment_atomic tool
fix: resolve session persistence race condition
docs: update MCP setup instructions
test: add unit tests for conversation engine
```

## Testing

### Test Categories

1. **Unit Tests** - Test individual functions and methods
2. **Integration Tests** - Test component interactions
3. **End-to-End Tests** - Test complete workflows

### Writing Tests

```go
func TestNewTool(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    "test-input",
            expected: "expected-output",
            wantErr:  false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := NewTool(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewTool() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if result != tt.expected {
                t.Errorf("NewTool() = %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Test Requirements

- All new functionality must include tests
- Maintain >80% test coverage
- Use table-driven tests for multiple scenarios
- Mock external dependencies
- Test error conditions

## Submitting Changes

### Pull Request Process

1. **Push Your Branch**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create Pull Request**
   - Use the PR template
   - Link related issues
   - Provide clear description of changes
   - Include screenshots for UI changes

3. **PR Requirements**
   - All tests must pass
   - Code must be formatted (`go fmt`)
   - No linting errors (`go vet`)
   - Documentation updated
   - Reviewed by maintainer

### PR Template

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added for new functionality
```

## Code Style

### Go Guidelines

- Follow standard Go conventions
- Use descriptive variable names
- Add comments for exported functions
- Handle errors appropriately
- Use interfaces for testability

### Formatting

```bash
# Format all code
go fmt ./...

# Run linter
golangci-lint run

# Check for common issues
go vet ./...
```

### Documentation

- Add godoc comments for exported functions
- Update README files for new features
- Include examples in documentation
- Keep documentation current with code changes

## Architecture Guidelines

### MCP Server Development

1. **Atomic Tools**
   - Single responsibility principle
   - Stateless operations
   - Consistent error handling
   - Comprehensive logging

2. **Conversation Engine**
   - Stage-based workflow
   - User preference handling
   - Error recovery mechanisms
   - Session state management

3. **Transport Layer**
   - Protocol abstraction
   - Connection management
   - Error propagation
   - Health monitoring

### Adding New Atomic Tools

1. **Create Tool Structure Following Current Patterns**
   ```go
   type AtomicMyNewToolArgs struct {
       types.BaseToolArgs
       // Tool-specific parameters with JSON schema validation
       RequiredParam string `json:"required_param" jsonschema:"required"`
       OptionalParam int    `json:"optional_param,omitempty"`
   }
   
   type AtomicMyNewToolResult struct {
       types.BaseToolResponse
       BaseAIContextResult // Embed AI context methods
       
       // Tool-specific results
       Success bool `json:"success"`
       Data    interface{} `json:"data"`
       
       // Rich context for AI reasoning
       Context *MyToolContext `json:"context"`
   }
   
   type AtomicMyNewTool struct {
       pipelineAdapter PipelineAdapter
       sessionManager  SessionManager
       logger          zerolog.Logger
   }
   
   func (t *AtomicMyNewTool) Execute(ctx context.Context, args AtomicMyNewToolArgs) (*AtomicMyNewToolResult, error) {
       // Implementation following atomic tool patterns
   }
   ```

2. **Implement AI Integration Pattern**
   - Follow [docs/AI_INTEGRATION_PATTERN.md](docs/AI_INTEGRATION_PATTERN.md)
   - Provide rich context structures
   - Include failure analysis and remediation steps
   - Use structured data over free text

3. **Add Fixing Capabilities (if applicable)**
   ```go
   // Use AtomicToolFixingMixin for retry logic
   fixingMixin := fixing.NewAtomicToolFixingMixin(analyzer, "my_tool", logger)
   operation := fixing.NewOperationWrapper(/* ... */)
   err := fixingMixin.ExecuteWithRetry(ctx, sessionID, baseDir, operation)
   ```

4. **Register Tool in register_atomic_tools.go**
   ```go
   registry.RegisterTool("my_new_tool_atomic", func(adapter PipelineAdapter, sessionManager SessionManager, logger zerolog.Logger) interfaces.AtomicTool {
       return NewAtomicMyNewTool(adapter, sessionManager, logger)
   })
   ```

5. **Add Comprehensive Tests**
   ```go
   func TestAtomicMyNewTool_Execute(t *testing.T) {
       // Test success cases, error cases, and AI context generation
   }
   ```

### Error Handling

- Use structured errors with context
- Provide actionable error messages
- Log errors with appropriate levels
- Return user-friendly messages

```go
if err != nil {
    return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)
}
```

## Getting Help

- **GitHub Issues**: For bugs and feature requests
- **Discussions**: For questions and general discussion
- **Documentation**: Check existing docs first
- **Code Review**: Ask for feedback on complex changes

## Recognition

Contributors are recognized in:
- GitHub contributors list
- Release notes for significant contributions
- Documentation acknowledgments

Thank you for contributing to Container Kit!