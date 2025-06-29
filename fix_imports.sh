#!/bin/bash

echo "Fixing imports after interface cleanup..."

# Update import statements in pkg/mcp files to use core instead of the main mcp package
find pkg/mcp -name "*.go" -type f | while read file; do
    echo "Processing: $file"

    # Skip core package files to avoid circular imports
    if [[ "$file" == *"/core/"* ]]; then
        echo "  Skipping core package file"
        continue
    fi

    # Skip the main interfaces.go file we just cleaned
    if [[ "$file" == "pkg/mcp/interfaces.go" ]]; then
        echo "  Skipping main interfaces.go"
        continue
    fi

    # Create backup
    cp "$file" "$file.bak"

    # Update imports - replace bare mcp import with core import when needed
    sed -i 's|"github.com/Azure/container-kit/pkg/mcp"$|"github.com/Azure/container-kit/pkg/mcp/core"|g' "$file"

    # Update type references from mcp.Type to core.Type
    sed -i 's|mcp\.Tool|core.Tool|g' "$file"
    sed -i 's|mcp\.ToolMetadata|core.ToolMetadata|g' "$file"
    sed -i 's|mcp\.ToolExample|core.ToolExample|g' "$file"
    sed -i 's|mcp\.ProgressReporter|core.ProgressReporter|g' "$file"
    sed -i 's|mcp\.ProgressToken|core.ProgressToken|g' "$file"
    sed -i 's|mcp\.ProgressStage|core.ProgressStage|g' "$file"
    sed -i 's|mcp\.RepositoryAnalyzer|core.RepositoryAnalyzer|g' "$file"
    sed -i 's|mcp\.RepositoryInfo|core.RepositoryInfo|g' "$file"
    sed -i 's|mcp\.DockerfileInfo|core.DockerfileInfo|g' "$file"
    sed -i 's|mcp\.HealthCheckInfo|core.HealthCheckInfo|g' "$file"
    sed -i 's|mcp\.BuildRecommendations|core.BuildRecommendations|g' "$file"
    sed -i 's|mcp\.Transport|core.Transport|g' "$file"
    sed -i 's|mcp\.RequestHandler|core.RequestHandler|g' "$file"
    sed -i 's|mcp\.ToolRegistry|core.ToolRegistry|g' "$file"
    sed -i 's|mcp\.SessionManager|core.SessionManager|g' "$file"
    sed -i 's|mcp\.Session|core.Session|g' "$file"
    sed -i 's|mcp\.SessionState|core.SessionState|g' "$file"
    sed -i 's|mcp\.SecurityScanResult|core.SecurityScanResult|g' "$file"
    sed -i 's|mcp\.VulnerabilityCount|core.VulnerabilityCount|g' "$file"
    sed -i 's|mcp\.SecurityFinding|core.SecurityFinding|g' "$file"
    sed -i 's|mcp\.MCPRequest|core.MCPRequest|g' "$file"
    sed -i 's|mcp\.MCPResponse|core.MCPResponse|g' "$file"
    sed -i 's|mcp\.MCPError|core.MCPError|g' "$file"
    sed -i 's|mcp\.BaseToolResponse|core.BaseToolResponse|g' "$file"
    sed -i 's|mcp\.Server|core.Server|g' "$file"
    sed -i 's|mcp\.ServerConfig|core.ServerConfig|g' "$file"
    sed -i 's|mcp\.ConversationConfig|core.ConversationConfig|g' "$file"
    sed -i 's|mcp\.ServerStats|core.ServerStats|g' "$file"
    sed -i 's|mcp\.SessionManagerStats|core.SessionManagerStats|g' "$file"
    sed -i 's|mcp\.WorkspaceStats|core.WorkspaceStats|g' "$file"
    sed -i 's|mcp\.AlternativeStrategy|core.AlternativeStrategy|g' "$file"
    sed -i 's|mcp\.ConversationStage|core.ConversationStage|g' "$file"

    # Update constants
    sed -i 's|mcp\.ConversationStagePreFlight|core.ConversationStagePreFlight|g' "$file"
    sed -i 's|mcp\.ConversationStageAnalyze|core.ConversationStageAnalyze|g' "$file"
    sed -i 's|mcp\.ConversationStageDockerfile|core.ConversationStageDockerfile|g' "$file"
    sed -i 's|mcp\.ConversationStageBuild|core.ConversationStageBuild|g' "$file"
    sed -i 's|mcp\.ConversationStagePush|core.ConversationStagePush|g' "$file"
    sed -i 's|mcp\.ConversationStageManifests|core.ConversationStageManifests|g' "$file"
    sed -i 's|mcp\.ConversationStageDeploy|core.ConversationStageDeploy|g' "$file"
    sed -i 's|mcp\.ConversationStageScan|core.ConversationStageScan|g' "$file"
    sed -i 's|mcp\.ConversationStageCompleted|core.ConversationStageCompleted|g' "$file"
    sed -i 's|mcp\.ConversationStageError|core.ConversationStageError|g' "$file"

    echo "  Updated: $file"
done

echo "Import fixes complete!"
