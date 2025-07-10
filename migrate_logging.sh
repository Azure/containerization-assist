#!/bin/bash

# Script to help migrate remaining files from zerolog to slog

echo "Files still using zerolog/logrus:"
echo "================================"

# Files from logging_audit.txt that haven't been processed yet
files=(
    "pkg/mcp/application/orchestration/pipeline/production_validation.go"
    "pkg/mcp/application/orchestration/pipeline/docker_optimizer_test.go"
    "pkg/mcp/application/orchestration/pipeline/cache_service.go"
    "pkg/mcp/application/orchestration/pipeline/basic_validator.go"
    "pkg/mcp/application/orchestration/pipeline/monitoring_integration.go"
    "pkg/mcp/application/orchestration/pipeline/atomic_framework.go"
    "pkg/mcp/application/orchestration/pipeline/docker_optimizer.go"
    "pkg/mcp/application/orchestration/pipeline/cache_types.go"
    "pkg/mcp/application/orchestration/pipeline/security_services.go"
    "pkg/mcp/application/clients.go"
    "pkg/mcp/application/api/interfaces.go"
    "pkg/mcp/application/internal/runtime/registry.go"
    "pkg/mcp/application/internal/runtime/registry_updates.go"
    "pkg/mcp/application/internal/runtime/registry_test.go"
    "pkg/mcp/application/internal/runtime/registration_helper.go"
    "pkg/mcp/application/internal/conversation/chat_tool.go"
    "pkg/mcp/application/internal/conversation/canonical_tools.go"
    "pkg/mcp/application/internal/conversation/chat_tool_test.go"
    "pkg/mcp/application/commands/scan_consolidated.go"
    "pkg/mcp/application/state/state_event_store.go"
    "pkg/mcp/application/core/server_impl.go"
)

for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        if grep -q "zerolog\|logrus" "$file"; then
            echo "$file"
        fi
    fi
done

echo ""
echo "Total files to process: ${#files[@]}"