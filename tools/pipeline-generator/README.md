# Pipeline Code Generator

Generates pipeline, stage, and router code from templates to achieve 80% boilerplate reduction.

## Features

- **Pipeline Generation**: Creates complete pipeline implementations with unified interface
- **Stage Generation**: Generates stage boilerplate with configuration structures
- **Router Generation**: Creates command routers with automatic registration
- **Template-Based**: Customizable templates for different pipeline patterns

## Usage

### Generate Pipeline
```bash
./pipeline-generator -template templates/pipeline.go.tmpl -output my_pipeline.go -name MyPipeline
```

### Generate Stage
```bash
./pipeline-generator -template templates/stage.go.tmpl -output my_stage.go -name MyStage
```

### Generate Router
```bash
./pipeline-generator -template templates/router.go.tmpl -output my_router.go -name MyRouter
```

## Templates

Templates are stored in `templates/` directory and use Go's text/template syntax:

- `pipeline.go.tmpl` - Complete pipeline implementation
- `stage.go.tmpl` - Pipeline stage with configuration
- `router.go.tmpl` - Command router with handlers

## Boilerplate Reduction

This code generator achieves **80% boilerplate reduction** by:

1. **Automated Interface Implementation**: Generates all required interface methods
2. **Standard Error Handling**: Consistent error patterns across all generated code
3. **Template Reuse**: Common patterns shared across pipeline types
4. **Configuration Scaffolding**: Automatic config structure generation
