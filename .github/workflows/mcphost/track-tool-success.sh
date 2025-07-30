#!/bin/bash
# Track successful tool executions for containerization workflow
TOOL_NAME="$1"
TOOL_OUTPUT="$2"
TIMESTAMP="$(date '+%Y-%m-%d %H:%M:%S')"

# Initialize log file if it doesn't exist
touch /tmp/workflow-hooks.log

# Only process containerization tools - ignore all others
case "$TOOL_NAME" in
  analyze_repository|generate_dockerfile|build_image|scan_image|tag_image|push_image|generate_k8s_manifests|prepare_cluster|deploy_application|verify_deployment)
    # This is a container tool, continue processing
    ;;
  *)
    # Not a container tool, echo name and exit
    echo "[$TIMESTAMP] ⏭️ Ignoring non-container tool: $TOOL_NAME" >> /tmp/workflow-hooks.log
    exit 0
    ;;
esac

# Clean the tool output by removing ANSI escape sequences and control characters
CLEAN_OUTPUT=""
if [ -n "$TOOL_OUTPUT" ]; then
    # Remove ANSI escape sequences, control chars, and other formatting
    CLEAN_OUTPUT=$(echo "$TOOL_OUTPUT" | sed 's/\x1b\[[0-9;]*m//g' | tr -d '\000-\010\013\014\016-\037' | tr -s ' ')
fi

# Parse tool output to check actual success status
SUCCESS_STATUS="unknown"
if [ -n "$CLEAN_OUTPUT" ]; then
    # Try to extract success field from JSON output
    SUCCESS_STATUS=$(echo "$CLEAN_OUTPUT" | jq -r '.success // "unknown"' 2>/dev/null || echo "unknown")
fi

# Log the tool execution with basic info
echo "[$TIMESTAMP] 🔧 Container tool executed: $TOOL_NAME" >> /tmp/workflow-hooks.log

if [ "$SUCCESS_STATUS" = "true" ]; then
    # Log successful tool executions
    echo "[$TIMESTAMP] ✅ Tool completed successfully: $TOOL_NAME" >> /tmp/workflow-hooks.log
elif [ "$SUCCESS_STATUS" = "false" ]; then
    # Log failed tool executions
    echo "[$TIMESTAMP] ❌ Tool failed: $TOOL_NAME" >> /tmp/workflow-hooks.log
    exit 0  # Don't track milestones for failed tools
else
    # Assume failure when we can't determine status, but allow for reruns
    echo "[$TIMESTAMP] ❌ Tool status unknown (assuming failure): $TOOL_NAME" >> /tmp/workflow-hooks.log
    exit 0  # Don't track milestones for unknown status
fi

# Track specific containerization workflow milestones
case "$TOOL_NAME" in
  "analyze_repository")
    echo "[$TIMESTAMP] 🔍 MILESTONE: Repository analysis completed" >> /tmp/workflow-hooks.log
    ;;
  "generate_dockerfile")
    echo "[$TIMESTAMP] 📝 MILESTONE: Dockerfile generation completed" >> /tmp/workflow-hooks.log
    ;;
  "build_image")
    echo "[$TIMESTAMP] 🏗️  MILESTONE: Container image build completed" >> /tmp/workflow-hooks.log
    ;;
  "scan_image")
    echo "[$TIMESTAMP] 🔐 MILESTONE: Security scan completed" >> /tmp/workflow-hooks.log
    ;;
  "tag_image")
    echo "[$TIMESTAMP] 🏷️  MILESTONE: Image tagging completed" >> /tmp/workflow-hooks.log
    ;;
  "push_image")
    echo "[$TIMESTAMP] 📤 MILESTONE: Image push completed" >> /tmp/workflow-hooks.log
    ;;
  "generate_k8s_manifests")
    echo "[$TIMESTAMP] ⚙️  MILESTONE: Kubernetes manifests generated" >> /tmp/workflow-hooks.log
    ;;
  "prepare_cluster")
    echo "[$TIMESTAMP] 🎯 MILESTONE: Kubernetes cluster prepared" >> /tmp/workflow-hooks.log
    ;;
  "deploy_application")
    echo "[$TIMESTAMP] 🚀 MILESTONE: Application deployment completed" >> /tmp/workflow-hooks.log
    ;;
  "verify_deployment")
    echo "[$TIMESTAMP] ✅ MILESTONE: Deployment verification completed" >> /tmp/workflow-hooks.log
    echo "[$TIMESTAMP] 🎉 SUCCESS: Application fully containerized and deployed!" >> /tmp/workflow-hooks.log
    ;;
esac

exit 0
