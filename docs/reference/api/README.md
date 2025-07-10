# Container Kit API Reference

This directory contains the consolidated API reference documentation for Container Kit.

## Documentation

- **[interfaces.md](interfaces.md)** - Complete API reference with all interface definitions

## Migration from docs/api/

The previous API documentation in `docs/api/` has been consolidated into this single reference document. The consolidation addresses several issues:

### What Was Consolidated

1. **docs/api/README.md** - Directory overview (replaced by this file)
2. **docs/api/extracted-interfaces.md** - Generated interface definitions (outdated)
3. **docs/api/interfaces.md** - Manual interface documentation (outdated)
4. **docs/api/pipeline.md** - Pipeline system documentation (integrated)
5. **docs/api/tools.md** - Tool system documentation (integrated)

### Benefits

- **Single Source of Truth**: All interface definitions now reference the canonical source code
- **Current Information**: Documentation reflects the actual implemented interfaces
- **Comprehensive Coverage**: Complete API reference in one location
- **Consistent Structure**: Unified documentation structure and format
- **Reduced Maintenance**: Single document to maintain instead of multiple files

### Source Code References

All interface definitions are sourced from:
- **Primary**: `/pkg/mcp/application/api/interfaces.go` (1040 lines)
- **Services**: `/pkg/mcp/application/services/interfaces.go` 
- **Compatibility**: `/pkg/mcp/application/interfaces/interfaces.go`

## Quick Navigation

- [Core Interfaces](interfaces.md#core-interfaces)
- [Tool System](interfaces.md#tool-system)
- [Registry System](interfaces.md#registry-system)
- [Workflow System](interfaces.md#workflow-system)
- [Pipeline System](interfaces.md#pipeline-system)
- [Validation System](interfaces.md#validation-system)
- [Build System](interfaces.md#build-system-types)
- [Error Handling](interfaces.md#error-handling)
- [Monitoring](interfaces.md#monitoring-and-observability)

## Related Documentation

- [Architecture Overview](../../architecture/three-layer-architecture.md)
- [Service Container](../../architecture/service-container.md)
- [Developer Guides](../../guides/developer/)
- [Tool Standards](../tools/standards.md)