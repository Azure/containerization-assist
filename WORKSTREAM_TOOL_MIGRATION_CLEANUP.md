# WORKSTREAM: Tool Migration Cleanup & Auto-Registration Implementation

## 🎯 Mission
Complete the tool migration to the unified registry system by implementing proper auto-registration, removing temporary migration code, and ensuring all tools follow the new registration pattern.

## 📋 Context
- **Issue**: The file `pkg/mcp/application/registry/migrate_tools.go` contains temporary migration code with placeholder implementations
- **Goal**: Implement proper tool auto-registration and remove all migration artifacts
- **Dependencies**: BETA's unified registry work (95% complete)
- **Timeline**: 2-3 days

## 🔍 Current State Analysis

### Problems Identified:
1. **Temporary Migration File**: `migrate_tools.go` contains 628 lines of temporary code
2. **Placeholder Implementations**: Stub tools that return dummy data
3. **Manual Registration**: Tools are manually registered instead of auto-registering
4. **Global State**: Uses global migrator instance (anti-pattern)
5. **Missing Auto-Registration**: No init() functions in tool packages

### Existing Tools to Migrate:
Based on the codebase, these are the real tool implementations that need proper registration:
- `pkg/mcp/application/commands/analyze_consolidated.go`
- `pkg/mcp/application/commands/build_consolidated.go`
- `pkg/mcp/application/commands/deploy_consolidated.go`
- `pkg/mcp/application/commands/scan_consolidated.go`
- Session management tools (location TBD)

## 📝 Implementation Plan

### Phase 1: Implement Auto-Registration Pattern (Day 1)

#### Step 1.1: Create Auto-Registration Interface
```bash
# Create the auto-registration mechanism
cat > pkg/mcp/application/registry/auto_register.go << 'EOF'
package registry

import (
    "sync"
    "github.com/Azure/container-kit/pkg/mcp/application/api"
)

// autoRegistry holds tools that register themselves on import
var (
    autoRegistry = make(map[string]api.ToolCreator)
    autoMutex    sync.RWMutex
)

// RegisterTool allows tools to register themselves during init()
func RegisterTool(name string, creator api.ToolCreator) {
    autoMutex.Lock()
    defer autoMutex.Unlock()
    autoRegistry[name] = creator
}

// GetAutoRegisteredTools returns all auto-registered tools
func GetAutoRegisteredTools() map[string]api.ToolCreator {
    autoMutex.RLock()
    defer autoMutex.RUnlock()
    
    tools := make(map[string]api.ToolCreator)
    for k, v := range autoRegistry {
        tools[k] = v
    }
    return tools
}

// LoadAutoRegisteredTools loads all auto-registered tools into a registry
func LoadAutoRegisteredTools(registry api.Registry) error {
    tools := GetAutoRegisteredTools()
    for name, creator := range tools {
        tool, err := creator()
        if err != nil {
            return err
        }
        if err := registry.Register(tool); err != nil {
            return err
        }
    }
    return nil
}
EOF
```

#### Step 1.2: Update Tool Implementations with Auto-Registration
```bash
# Example for analyze tool
# Add this to the end of analyze_consolidated.go
cat >> pkg/mcp/application/commands/analyze_consolidated.go << 'EOF'

func init() {
    registry.RegisterTool("containerization_analyze", func() (api.Tool, error) {
        return NewAnalyzeTool(), nil
    })
}
EOF

# Repeat for other tools:
# - build_consolidated.go
# - deploy_consolidated.go  
# - scan_consolidated.go
```

### Phase 2: Migrate Real Tool Implementations (Day 1-2)

#### Step 2.1: Audit Existing Tools
```bash
# Find all tool implementations
echo "=== TOOL IMPLEMENTATION AUDIT ==="

# Find all files implementing Execute method
grep -r "func.*Execute.*context\.Context.*ToolInput.*ToolOutput" pkg/mcp --include="*.go" | grep -v migrate_tools.go > real_tools.txt

# List consolidated tools
ls -la pkg/mcp/application/commands/*_consolidated.go

# Check for session tools
find pkg/mcp -name "*session*.go" -type f | grep -v migrate_tools.go
```

#### Step 2.2: Add Auto-Registration to Each Tool
```bash
# For each real tool implementation, add init() function
# Example script to add init functions
for file in pkg/mcp/application/commands/*_consolidated.go; do
    tool_name=$(basename $file | sed 's/_consolidated.go//')
    echo "Adding auto-registration to $file for tool: $tool_name"
    
    # Check if init already exists
    if ! grep -q "func init()" "$file"; then
        cat >> "$file" << EOF

func init() {
    registry.RegisterTool("containerization_${tool_name}", func() (api.Tool, error) {
        // TODO: Return actual tool instance
        return New${tool_name^}Tool(), nil
    })
}
EOF
    fi
done
```

### Phase 3: Remove Migration Code (Day 2)

#### Step 3.1: Verify All Tools Are Registered
```bash
# Create verification script
cat > verify_tool_registration.go << 'EOF'
package main

import (
    "fmt"
    "github.com/Azure/container-kit/pkg/mcp/application/registry"
)

func main() {
    tools := registry.GetAutoRegisteredTools()
    fmt.Printf("Auto-registered tools: %d\n", len(tools))
    for name := range tools {
        fmt.Printf("  - %s\n", name)
    }
    
    // Expected tools
    expected := []string{
        "containerization_analyze",
        "containerization_build",
        "containerization_deploy",
        "containerization_scan",
        "session_create",
        "session_manage",
    }
    
    for _, exp := range expected {
        if _, ok := tools[exp]; !ok {
            fmt.Printf("ERROR: Missing tool: %s\n", exp)
        }
    }
}
EOF

go run verify_tool_registration.go
```

#### Step 3.2: Remove Migration File
```bash
# First, ensure all tools are properly registered
echo "=== REMOVING MIGRATION CODE ==="

# Backup the file first (just in case)
cp pkg/mcp/application/registry/migrate_tools.go migrate_tools.go.backup

# Check for any imports of migrate_tools
grep -r "migrate_tools\|MigrateAllTools\|ToolMigrator" pkg/mcp --include="*.go" | grep -v migrate_tools.go

# If no dependencies found, remove the file
rm pkg/mcp/application/registry/migrate_tools.go

# Verify build still works
go build ./pkg/mcp/...
```

### Phase 4: Update Registry Integration (Day 2-3)

#### Step 4.1: Update Registry Initialization
```bash
# Find where registry is initialized
grep -r "NewUnifiedRegistry\|NewRegistry" pkg/mcp --include="*.go"

# Update initialization to load auto-registered tools
# Example update for server initialization
cat > pkg/mcp/application/core/registry_init.go << 'EOF'
package core

import (
    "github.com/Azure/container-kit/pkg/mcp/application/api"
    "github.com/Azure/container-kit/pkg/mcp/application/registry"
)

// InitializeRegistry creates and populates the registry with all tools
func InitializeRegistry() (api.Registry, error) {
    reg := registry.NewUnifiedRegistry()
    
    // Load all auto-registered tools
    if err := registry.LoadAutoRegisteredTools(reg); err != nil {
        return nil, err
    }
    
    return reg, nil
}
EOF
```

#### Step 4.2: Remove Manual Registration Code
```bash
# Find and remove manual tool registration
grep -r "Register.*Tool\|RegisterTool" pkg/mcp --include="*.go" | grep -v "auto_register.go"

# Remove manual registration blocks
# These typically look like:
# registry.Register(&AnalyzeTool{})
# registry.Register(&BuildTool{})
# etc.
```

### Phase 5: Testing & Validation (Day 3)

#### Step 5.1: Create Integration Test
```bash
cat > pkg/mcp/application/registry/auto_register_test.go << 'EOF'
package registry_test

import (
    "testing"
    "github.com/Azure/container-kit/pkg/mcp/application/registry"
    _ "github.com/Azure/container-kit/pkg/mcp/application/commands" // Import for side effects
)

func TestAutoRegistration(t *testing.T) {
    tools := registry.GetAutoRegisteredTools()
    
    expectedTools := []string{
        "containerization_analyze",
        "containerization_build",
        "containerization_deploy",
        "containerization_scan",
    }
    
    for _, expected := range expectedTools {
        if _, ok := tools[expected]; !ok {
            t.Errorf("Expected tool %s not found in auto-registry", expected)
        }
    }
    
    if len(tools) < len(expectedTools) {
        t.Errorf("Expected at least %d tools, got %d", len(expectedTools), len(tools))
    }
}

func TestLoadAutoRegisteredTools(t *testing.T) {
    reg := registry.NewUnifiedRegistry()
    
    err := registry.LoadAutoRegisteredTools(reg)
    if err != nil {
        t.Fatalf("Failed to load auto-registered tools: %v", err)
    }
    
    // Verify tools are accessible
    tools := reg.List()
    if len(tools) == 0 {
        t.Error("No tools loaded into registry")
    }
}
EOF

# Run the test
go test ./pkg/mcp/application/registry/...
```

#### Step 5.2: End-to-End Verification
```bash
# Build everything
make build

# Run all tests
make test-all

# Verify no migration code remains
if [ -f "pkg/mcp/application/registry/migrate_tools.go" ]; then
    echo "ERROR: Migration file still exists!"
else
    echo "✅ Migration file removed"
fi

# Check for clean imports
go mod tidy
go mod verify
```

## 🎯 Success Criteria

1. **Auto-Registration Working**
   - [ ] All tools use init() for self-registration
   - [ ] No manual tool registration code remains
   - [ ] Registry automatically loads all tools on startup

2. **Migration Code Removed**
   - [ ] `migrate_tools.go` deleted
   - [ ] No references to ToolMigrator remain
   - [ ] No placeholder/stub implementations

3. **Clean Architecture**
   - [ ] Each tool in its proper package
   - [ ] No global state for registration
   - [ ] Clear separation of concerns

4. **Tests Pass**
   - [ ] Auto-registration tests pass
   - [ ] All existing tests still pass
   - [ ] Integration tests verify tool availability

## 🚨 Risk Mitigation

1. **Before Removing Migration Code**:
   - Ensure ALL tools are properly registered
   - Run full test suite
   - Check for any runtime dependencies

2. **Backup Strategy**:
   - Keep backup of migrate_tools.go until verified
   - Commit changes incrementally
   - Test each phase thoroughly

3. **Rollback Plan**:
   - Git revert if issues found
   - Migration file backed up locally
   - Can re-add manual registration if needed

## 📊 Verification Commands

```bash
# Final verification checklist
echo "=== TOOL MIGRATION VERIFICATION ==="

# 1. Check auto-registration implementation
[ -f "pkg/mcp/application/registry/auto_register.go" ] && echo "✅ Auto-registration implemented" || echo "❌ Auto-registration missing"

# 2. Check migration file removed
[ ! -f "pkg/mcp/application/registry/migrate_tools.go" ] && echo "✅ Migration file removed" || echo "❌ Migration file still exists"

# 3. Count init() functions in tool files
init_count=$(grep -r "func init()" pkg/mcp/application/commands/*_consolidated.go | wc -l)
echo "Tools with init() functions: $init_count"

# 4. Test registry functionality
go test ./pkg/mcp/application/registry/... && echo "✅ Registry tests pass" || echo "❌ Registry tests fail"

# 5. Build verification
go build ./pkg/mcp/... && echo "✅ Build successful" || echo "❌ Build failed"
```

## 🏁 Definition of Done

- [ ] All tools use auto-registration pattern
- [ ] migrate_tools.go completely removed
- [ ] No manual tool registration code
- [ ] All tests pass
- [ ] Documentation updated
- [ ] No placeholder implementations
- [ ] Clean build with no warnings
- [ ] Registry loads all tools automatically

---

**Note**: This cleanup is essential for maintaining clean architecture and removing technical debt from the BETA workstream's registry unification effort.