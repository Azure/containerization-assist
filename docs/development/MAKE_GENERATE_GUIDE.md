# Make Generate Command Guide

This guide explains what the `make generate` command does and how it works in the Container Kit project.

## Overview

The `make generate` command is a comprehensive code generation tool that automates the creation of boilerplate code, dependency injection wiring, and pipeline templates. It's essential for maintaining consistency and reducing manual coding effort in the Container Kit project.

## What It Does

When you run `make generate`, it performs three main tasks:

1. **Schema Generation** - Builds the schema generator tool
2. **Pipeline Generation** - Creates pipeline implementations from templates
3. **Code Generation** - Generates boilerplate code using Go's `go generate` system

## Detailed Breakdown

### Step 1: Building the Schema Generator (`build-schemaGen`)

```bash
go build -o bin/schemaGen ./cmd/mcp-schema-gen
```

This compiles the schema generator tool from source code in `cmd/mcp-schema-gen/` and places the executable in `bin/schemaGen`. The schema generator is a custom tool that creates:

- Tool interface implementations
- Validation logic
- Test templates
- Domain-specific boilerplate code

### Step 2: Pipeline Generation (`generate-pipelines`)

```bash
go build -o tools/pipeline-generator/pipeline-generator ./tools/pipeline-generator
tools/pipeline-generator/pipeline-generator -template tools/pipeline-generator/templates/pipeline.go.tmpl -output pkg/mcp/application/pipeline/generated_pipeline.go -name ExamplePipeline -stages "ValidateInput,ProcessData,GenerateOutput"
```

This:
1. Builds the pipeline generator tool
2. Uses a Go template (`pipeline.go.tmpl`) to generate pipeline implementations
3. Creates `generated_pipeline.go` with predefined stages for workflow execution

**Example Output**: A complete pipeline implementation with methods for each stage (ValidateInput, ProcessData, GenerateOutput).

### Step 3: Go Generate (`go generate ./...`)

This runs all `//go:generate` directives found in the codebase. Currently, it processes:

#### Dependency Injection (Wire)
- **File**: `pkg/mcp/application/di/wire.go`
- **Command**: `//go:generate wire`
- **Purpose**: Generates dependency injection code using Google's Wire library
- **Output**: `wire_gen.go` with fully wired service containers

#### Schema Generation
- **Files**: 
  - `pkg/mcp/application/internal/conversation/canonical_tools.go`
  - `pkg/mcp/application/internal/conversation/chat_tool.go`
  - `test/example/tool_args.go`
- **Command**: `//go:generate ../../../../../bin/schemaGen -tool=<tool_name> -domain=<domain> -output=.`
- **Purpose**: Generates tool implementations, validation, and tests

## Generated Files

After running `make generate`, you'll see these new or updated files:

```
pkg/mcp/application/di/wire_gen.go                    # Dependency injection wiring
pkg/mcp/application/pipeline/generated_pipeline.go    # Pipeline implementation
pkg/mcp/application/internal/conversation/            # Generated conversation tools
test/example/                                         # Generated example tools
```

## Schema Generator Details

The schema generator (`bin/schemaGen`) accepts these flags:

- `-tool`: Tool name in format `domain_action_object` (e.g., `security_scan_container`)
- `-domain`: Domain category (e.g., `security`, `build`, `deploy`)
- `-output`: Output directory for generated files
- `-desc`: Tool description
- `-type`: Generation type (`boilerplate`, `compliance`, `migration`)
- `-v`: Verbose output

### Tool Naming Convention

Tools follow the pattern: `domain_action_object`

Examples:
- `security_scan_container` → SecurityScanContainer
- `build_create_dockerfile` → BuildCreateDockerfile
- `deploy_apply_manifest` → DeployApplyManifest

## Wire Dependency Injection

The Wire framework automatically generates dependency injection code based on provider functions defined in `pkg/mcp/application/di/providers.go`.

**Key Components**:
- `Container` struct: Holds all application services
- Provider functions: `NewToolRegistry()`, `NewSessionStore()`, etc.
- Wire directives: Define how dependencies are wired together

## When to Run `make generate`

Run this command when you:

1. **Add new tools** - Need boilerplate code generated
2. **Modify dependency injection** - Change provider functions or container structure
3. **Update pipeline templates** - Modify pipeline generation templates
4. **Add go:generate directives** - Add new code generation commands

## Troubleshooting

### Common Issues

1. **Wire errors**: Usually caused by conflicting dependencies or missing providers
2. **Template not found**: Schema generator can't find template files
3. **Permission errors**: Binary files don't have execute permissions

### Solutions

1. **Check provider functions** in `pkg/mcp/application/di/providers.go`
2. **Verify template paths** in schema generator configuration
3. **Ensure binary is built** with `make build-schemaGen`

## Example Workflow

```bash
# 1. Add a new tool specification
echo '//go:generate ../../bin/schemaGen -tool=security_scan_image -domain=security -output=.' >> pkg/security/tools.go

# 2. Generate all code
make generate

# 3. Review generated files
ls pkg/security/  # Check for new generated files

# 4. Implement business logic in generated Execute methods
# 5. Run tests to verify everything works
make test
```

## Best Practices

1. **Always run after dependency changes** - Ensures Wire generates correct injection code
2. **Review generated code** - Check that generated implementations match expectations
3. **Don't modify generated files directly** - Changes will be overwritten on next generation
4. **Use descriptive tool names** - Follow the `domain_action_object` convention
5. **Test generated code** - Run `make test` after generation to catch issues early

## Integration with Development Workflow

The `make generate` command integrates with other development commands:

```bash
make generate  # Generate all code
make fmt      # Format generated code
make lint     # Lint including generated files
make test     # Test generated implementations
make build    # Build with generated code
```

This ensures that generated code follows the same quality standards as hand-written code.