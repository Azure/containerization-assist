# Package Migration Mapping (86 → 10 packages)

## Target Structure
```
pkg/mcp/
├── api/          # Interface definitions (single source of truth)
├── core/         # Server & registry core  
├── tools/        # Container operations (analyze, build, deploy, scan)
├── session/      # Session management and persistence
├── workflow/     # Multi-step operation orchestration
├── transport/    # MCP protocol transports (stdio, HTTP)
├── storage/      # Persistence implementations (BoltDB)
├── security/     # Validation and security scanning
├── templates/    # Kubernetes manifest templates
└── internal/     # Implementation details and utilities
```

## Detailed Migration Mappings

### 1. API Package
- `application/api/*` → `api/`
- All interface definitions consolidated here

### 2. Core Package  
- `application/core/*` → `core/`
- `app/registry/*` → `core/registry/`
- `application/orchestration/registry/*` → `core/registry/`
- `services/registry/*` → `core/registry/`
- Server lifecycle and registry management

### 3. Tools Package
- `domain/containerization/analyze/*` → `tools/analyze/`
- `domain/containerization/build/*` → `tools/build/`
- `domain/containerization/deploy/*` → `tools/deploy/`
- `domain/containerization/scan/*` → `tools/scan/`
- `domain/containerization/database_detectors/*` → `tools/detectors/`
- All container operations consolidated

### 4. Session Package
- `domain/session/*` → `session/`
- `services/session/*` → `session/`
- `domain/containerization/session/*` → `session/`
- Session management unified

### 5. Workflow Package
- `application/orchestration/workflow/*` → `workflow/`
- `services/workflow/*` → `workflow/`
- `application/workflows/*` → `workflow/`
- Multi-step operations

### 6. Transport Package
- `infra/transport/*` → `transport/`
- MCP protocol implementations

### 7. Storage Package
- `infra/persistence/*` → `storage/`
- `infra/persistence/boltdb/*` → `storage/boltdb/`
- Persistence layer

### 8. Security Package
- `domain/security/*` → `security/`
- `domain/validation/*` → `security/validation/`
- `services/validation/*` → `security/validation/`
- `services/scanner/*` → `security/scanner/`
- Security and validation unified

### 9. Templates Package
- `infra/templates/*` → `templates/`
- All sub-directories preserved

### 10. Internal Package
- `application/internal/*` → `internal/`
- `domain/errors/*` → `internal/errors/`
- `services/errors/*` → `internal/errors/`
- `domain/types/*` → `internal/types/`
- `domain/utils/*` → `internal/utils/`
- `domain/common/*` → `internal/common/`
- `domain/retry/*` → `internal/retry/`
- `domain/logging/*` → `internal/logging/`
- `domain/processing/*` → `internal/processing/`
- Implementation details consolidated

## Packages to be Removed/Merged
- `application/orchestration/` (distributed across core, workflow)
- `application/services/` (distributed by function)
- `domain/` (entire layer flattened)
- `infra/` (distributed by function)
- `services/` (distributed by function)
- `app/` (merged into core)
- `container/` (merged into tools if needed)

## Import Path Changes
All imports will change from:
- `pkg/mcp/application/api/...` → `pkg/mcp/api/...`
- `pkg/mcp/domain/containerization/build/...` → `pkg/mcp/tools/build/...`
- `pkg/mcp/infra/transport/...` → `pkg/mcp/transport/...`
- Maximum 3 levels: `pkg/mcp/package/[subpackage]`