#!/bin/bash

# Script to update MCP-specific files from zerolog to internal logging package

# List of MCP files to update
MCP_FILES=(
    "pkg/mcp/application/orchestration/pipeline/security_services.go"
    "pkg/mcp/application/internal/conversation/chat_tool.go"
    "pkg/mcp/application/internal/conversation/chat_tool_test.go"
    "pkg/mcp/application/orchestration/pipeline/cache_service.go"
    "pkg/mcp/application/internal/conversation/canonical_tools.go"
    "pkg/mcp/application/orchestration/pipeline/production_validation.go"
    "pkg/mcp/application/orchestration/pipeline/atomic_framework.go"
    "pkg/mcp/application/state/state_event_store.go"
    "pkg/mcp/application/internal/runtime/registration_helper.go"
    "pkg/mcp/application/internal/runtime/registry_test.go"
    "pkg/mcp/application/internal/runtime/registry_updates.go"
    "pkg/mcp/application/orchestration/pipeline/cache_types.go"
    "pkg/mcp/application/orchestration/pipeline/docker_optimizer.go"
    "pkg/mcp/application/orchestration/pipeline/monitoring_integration.go"
    "pkg/mcp/application/orchestration/pipeline/docker_optimizer_test.go"
)

echo "Starting MCP zerolog to internal logging migration..."

for file in "${MCP_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "Processing $file..."
        
        # Replace import
        sed -i 's|"github.com/rs/zerolog"|"github.com/Azure/container-kit/pkg/mcp/infra/internal/logging"|g' "$file"
        
        # Replace type declarations
        sed -i 's/zerolog\.Logger/logging.LoggingStandards/g' "$file"
        
        # Replace logger.With().Str("component", "xxx").Logger() pattern
        sed -i 's/logger\.With()\.Str("component", \("[^"]*"\)\)\.Logger()/logger.WithComponent(\1)/g' "$file"
        
        # Replace logger.With().Timestamp().Logger() pattern
        sed -i 's/logger\.With()\.Timestamp()\.Logger()/logger/g' "$file"
        
        # Replace zerolog.New(...) patterns
        sed -i 's/zerolog\.New([^)]*)/logging.NewLogger()/g' "$file"
        
        # Handle zerolog levels
        sed -i 's/zerolog\.InfoLevel/logging.LevelInfo/g' "$file"
        sed -i 's/zerolog\.DebugLevel/logging.LevelDebug/g' "$file"
        sed -i 's/zerolog\.WarnLevel/logging.LevelWarn/g' "$file"
        sed -i 's/zerolog\.ErrorLevel/logging.LevelError/g' "$file"
        
        echo "  ✓ Updated $file"
    else
        echo "  ⚠ File not found: $file"
    fi
done

echo "MCP migration complete!"