#!/bin/bash

# Script to update remaining files from zerolog to internal logging package

# List of files to update (excluding the ones we've already done)
FILES=(
    "pkg/mcp/application/orchestration/pipeline/security_services.go"
    "pkg/mcp/application/internal/conversation/chat_tool.go"
    "pkg/mcp/application/internal/conversation/chat_tool_test.go"
    "pkg/mcp/application/orchestration/pipeline/cache_service.go"
    "pkg/core/security/secret_discovery.go"
    "pkg/core/security/secret_discovery_test.go"
    "pkg/mcp/application/internal/conversation/canonical_tools.go"
    "pkg/core/security/policy_engine_test.go"
    "pkg/core/security/health_monitor_test.go"
    "pkg/core/security/cve_database_test.go"
    "pkg/core/security/cve_database.go"
    "pkg/mcp/application/orchestration/pipeline/production_validation.go"
    "pkg/mcp/application/orchestration/pipeline/atomic_framework.go"
    "pkg/core/docker/trivy.go"
    "cmd/cmd.go"
    "cmd/mcp-server/main.go"
    "pkg/deps/updater.go"
    "pkg/core/security/health_monitor.go"
    "pkg/core/security/metrics.go"
    "pkg/core/security/sbom_vuln_integration.go"
    "pkg/core/security/health_checkers.go"
    "pkg/core/security/policy_engine.go"
    "pkg/core/security/sbom.go"
    "pkg/core/docker/grype.go"
    "pkg/core/docker/registry_health_test.go"
    "pkg/core/docker/registry_health.go"
    "pkg/core/docker/unified_scanner.go"
    "pkg/core/docker/trivy_test.go"
    "pkg/core/kubernetes/customizer.go"
    "pkg/core/kubernetes/logs.go"
    "pkg/core/kubernetes/secret_generator_test.go"
    "pkg/core/kubernetes/secret_generator.go"
    "pkg/core/kubernetes/health.go"
    "pkg/mcp/application/state/state_event_store.go"
    "pkg/mcp/application/internal/runtime/registration_helper.go"
    "pkg/mcp/application/internal/runtime/registry_test.go"
    "pkg/mcp/application/internal/runtime/registry_updates.go"
    "pkg/mcp/application/orchestration/pipeline/cache_types.go"
    "pkg/mcp/application/orchestration/pipeline/docker_optimizer.go"
    "pkg/mcp/application/orchestration/pipeline/monitoring_integration.go"
    "pkg/mcp/application/orchestration/pipeline/docker_optimizer_test.go"
    "pkg/logger/logger.go"
)

echo "Starting zerolog to internal logging migration..."

for file in "${FILES[@]}"; do
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

echo "Migration complete!"
echo ""
echo "Note: Some files may need manual review, especially:"
echo "1. Files that use zerolog-specific features like .Dur(), .Interface(), etc."
echo "2. Test files that mock zerolog"
echo "3. Files in pkg/core that may need their own logging adapter"
