# Documentation Review and Consolidation Plan

## Overview
This document categorizes all markdown files in the project and provides recommendations for consolidation, updates, or deletion.

## Categories

### 1. Core Project Documentation (Keep & Update)
These are essential project docs that should be maintained:

- **README.md** - Main project documentation ✓
- **CLAUDE.md** - AI assistant instructions ✓
- **CONTRIBUTING.md** - Contribution guidelines ✓
- **CODE_OF_CONDUCT.md** - Community standards ✓
- **SECURITY.md** - Security policies ✓
- **CHANGELOG.md** - Version history ✓
- **LICENSE** - Legal ✓

### 2. Architecture Documentation (Consolidate)
Multiple architecture docs exist that should be consolidated:

- **ARCHITECTURE.md** - Original architecture (OUTDATED)
- **ARCHITECTURE_NEW.md** - Updated architecture (OUTDATED)
- **docs/mcp-architecture.md** - Current MCP architecture ✓
- **INTERFACES.md** - Interface documentation (DUPLICATE with docs/interface-patterns.md)
- **docs/interface-patterns.md** - Current interface patterns ✓

**Action**: Merge ARCHITECTURE.md and ARCHITECTURE_NEW.md content into docs/mcp-architecture.md, then delete old files.

### 3. Reorganization Planning Docs (Archive or Delete)
These were temporary planning documents that are now complete:

- **REORG.md** - Reorganization plan (COMPLETE)
- **PARALLEL_WORKPLAN.md** - Parallel work plan (COMPLETE)
- **NEXTSTEPS.md** - Next steps planning (ACTIVE)
- **MIGRATION_SUMMARY.md** - Migration summary (COMPLETE)
- **All TEAM_*_PLAN.md files** - Team-specific plans (COMPLETE)
- **All TEAM_*_TASK*.md files** - Task tracking (COMPLETE)
- **team-c-*.md files** - Team C specific plans (COMPLETE)

**Action**: Move all completed team plans to an `archive/reorganization/` directory.

### 4. Developer Guides (Keep & Update)
Current developer documentation:

- **DEVELOPMENT_GUIDELINES.md** - Development practices ✓
- **docs/adding-new-tools.md** - Tool development guide ✓
- **docs/migration-guide.md** - v1 to v2 migration ✓
- **docs/breaking-changes.md** - Breaking changes list ✓
- **AUTO_REGISTRATION.md** - Auto-registration details (MERGE with adding-new-tools.md)

### 5. Technical Documentation (Keep)
Specific technical guides:

- **MCP_DOCUMENTATION.md** - MCP system documentation ✓
- **docs/AI_INTEGRATION_PATTERN.md** - AI integration patterns ✓
- **docs/ATOMIC_TOOL_STANDARDS.md** - Atomic tool standards ✓
- **docs/LINTING.md** - Linting configuration ✓
- **docs/quality-ci-cd.md** - CI/CD quality gates ✓

### 6. Tool-Specific Documentation (Review)
Documentation for specific tools:

- **docs/logs-export-tool.md** - Logs export tool ✓
- **docs/telemetry-export-tool.md** - Telemetry export tool ✓

### 7. Internal Documentation (Keep)
Documentation within code directories:

- **pkg/mcp/internal/orchestration/README.md** - Orchestration details ✓
- **pkg/mcp/internal/server/README.md** - Server implementation ✓
- **examples/README.md** - Examples overview ✓
- **examples/basic-tool/README.md** - Basic tool example ✓
- **.devcontainer/README.md** - Dev container setup ✓

### 8. Templates (Keep)
Conversation prompt templates (internal use):

- All files in **pkg/mcp/internal/runtime/conversation/prompts/templates/** ✓

### 9. Miscellaneous (Review)
Other documentation:

- **feedback.md** - Feedback collection (CHECK if still relevant)
- **scripts/resolve-arch-conflicts.md** - Conflict resolution guide ✓
- **test/integration/mcp/claude_desktop_test.md** - Test documentation ✓
- **docs/INTERFACE_CLEANUP.md** - Interface cleanup notes (OUTDATED)
- **docs/LEGACY_REFERENCES.md** - Legacy reference list (CHECK if complete)

### 10. Tool READMEs (Keep)
Tool-specific documentation:

- **tools/quality-dashboard/README.md** - Quality dashboard ✓
- **tools/validate-interfaces/README.md** - Interface validator ✓

## Recommendations

### Immediate Actions

1. **Create Archive Directory**
   ```bash
   mkdir -p archive/reorganization
   mkdir -p archive/old-architecture
   ```

2. **Move Completed Planning Docs**
   - Move all TEAM_*.md files to archive/reorganization/
   - Move REORG.md, PARALLEL_WORKPLAN.md, MIGRATION_SUMMARY.md to archive/reorganization/
   - Keep NEXTSTEPS.md in root (still active)

3. **Consolidate Architecture Docs**
   - Merge ARCHITECTURE.md and ARCHITECTURE_NEW.md into docs/mcp-architecture.md
   - Delete INTERFACES.md (duplicates docs/interface-patterns.md)
   - Move old architecture docs to archive/old-architecture/

4. **Merge Duplicate Content**
   - Merge AUTO_REGISTRATION.md content into docs/adding-new-tools.md
   - Update docs/INTERFACE_CLEANUP.md or delete if outdated

5. **Update Outdated Docs**
   - Review and update DEVELOPMENT_GUIDELINES.md for new patterns
   - Check feedback.md relevance
   - Verify docs/LEGACY_REFERENCES.md is complete

### Documentation Structure After Cleanup

```
/
├── README.md                    # Project overview
├── CLAUDE.md                    # AI assistant guide
├── CONTRIBUTING.md              # How to contribute
├── CODE_OF_CONDUCT.md          # Community standards
├── SECURITY.md                 # Security policies
├── CHANGELOG.md                # Version history
├── DEVELOPMENT_GUIDELINES.md   # Dev practices
├── NEXTSTEPS.md               # Current planning (temporary)
├── docs/
│   ├── mcp-architecture.md    # Complete architecture
│   ├── interface-patterns.md  # Interface design patterns
│   ├── adding-new-tools.md    # Tool development guide
│   ├── migration-guide.md     # v1 to v2 migration
│   ├── breaking-changes.md    # Breaking changes list
│   ├── AI_INTEGRATION_PATTERN.md
│   ├── ATOMIC_TOOL_STANDARDS.md
│   ├── LINTING.md
│   ├── quality-ci-cd.md
│   ├── logs-export-tool.md
│   └── telemetry-export-tool.md
├── examples/                   # Code examples
├── archive/
│   ├── reorganization/        # Completed reorg docs
│   └── old-architecture/      # Outdated architecture docs
└── [other project files]
```

## Next Steps

1. Execute the consolidation plan
2. Update cross-references in remaining docs
3. Add a documentation index to README.md
4. Ensure all docs reflect the unified interface patterns