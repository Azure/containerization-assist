#!/bin/bash
# Track successful tool executions for containerization workflow
TOOL_NAME="$1"
TOOL_OUTPUT="$2"
TIMESTAMP="$(date '+%Y-%m-%d %H:%M:%S')"

# Only process containerization tools - ignore all others
case "$TOOL_NAME" in
  mcp_containerkit_*)
    # This is a container tool, continue processing
    ;;
  *)
    # Not a container tool, silently exit
    exit 0
    ;;
esac

# Parse tool output to check actual success status
SUCCESS_STATUS="unknown"
if [ -n "$TOOL_OUTPUT" ]; then
    # Try to extract success field from JSON output
    SUCCESS_STATUS=$(echo "$TOOL_OUTPUT" | jq -r '.success // "unknown"' 2>/dev/null || echo "unknown")
fi

if [ "$SUCCESS_STATUS" = "true" ]; then
    # Log successful tool executions
    echo "[$TIMESTAMP] ✅ Tool completed successfully: $TOOL_NAME" >> /tmp/workflow-hooks.log
elif [ "$SUCCESS_STATUS" = "false" ]; then
    # Log failed tool executions
    echo "[$TIMESTAMP] ❌ Tool failed: $TOOL_NAME" >> /tmp/workflow-hooks.log
    exit 0  # Don't track milestones for failed tools
else
    # Log when we can't determine success status
    echo "[$TIMESTAMP] ❓ Tool completed (status unknown): $TOOL_NAME" >> /tmp/workflow-hooks.log
    exit 0  # Don't track milestones for unknown status
fi

# Track specific containerization workflow milestones
case "$TOOL_NAME" in
  "mcp_containerkit_analyze_repository")
    echo "[$TIMESTAMP] 🔍 MILESTONE: Repository analysis completed" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_generate_dockerfile")
    echo "[$TIMESTAMP] 📝 MILESTONE: Dockerfile generation completed" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_build_image")
    echo "[$TIMESTAMP] 🏗️  MILESTONE: Container image build completed" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_scan_image")
    echo "[$TIMESTAMP] 🔐 MILESTONE: Security scan completed" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_tag_image")
    echo "[$TIMESTAMP] 🏷️  MILESTONE: Image tagging completed" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_push_image")
    echo "[$TIMESTAMP] 📤 MILESTONE: Image push completed" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_generate_k8s_manifests")
    echo "[$TIMESTAMP] ⚙️  MILESTONE: Kubernetes manifests generated" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_prepare_cluster")
    echo "[$TIMESTAMP] 🎯 MILESTONE: Kubernetes cluster prepared" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_deploy_application")
    echo "[$TIMESTAMP] 🚀 MILESTONE: Application deployment completed" >> /tmp/workflow-hooks.log
    ;;
  "mcp_containerkit_verify_deployment")
    echo "[$TIMESTAMP] ✅ MILESTONE: Deployment verification completed" >> /tmp/workflow-hooks.log
    echo "[$TIMESTAMP] 🎉 SUCCESS: Application fully containerized and deployed!" >> /tmp/workflow-hooks.log
    ;;
esac

exit 0
