# Progress Monitoring Guide

This guide explains how to monitor the progress of containerization workflows in Container Kit.

## Progress Monitoring Options

Container Kit provides multiple layers of progress reporting:

### 1. MCP Progress Tokens (Primary)
When using the MCP server with clients that support progress tokens, real-time progress updates are sent directly to the client.

### 2. Structured Progress Logs (Fallback)
For cases where progress tokens are not available, Container Kit emits structured log messages that can be easily monitored.

## Log-Based Progress Monitoring

### Log Format
Progress logs use a structured format with the keyword `PROGRESS`:

```
INFO 🔄 PROGRESS step=3 total=10 percentage=30 step_name="build_image" status="running" message="[30%] Building Docker image"
INFO ✅ PROGRESS step=3 total=10 percentage=30 step_name="build_image" status="completed" message="[30%] Building Docker image" duration="45.2s"
INFO ❌ PROGRESS step=3 total=10 percentage=30 step_name="build_image" status="failed" error="docker build failed" duration="12.1s"
```

### Progress Stages
1. 🚀 **workflow_start** (0%) - Workflow initialization
2. 🔄 **analyze_repository** (10%) - Repository analysis
3. 🔄 **generate_dockerfile** (20%) - Dockerfile generation
4. 🔄 **build_image** (30%) - Docker image build
5. 🔄 **scan_vulnerabilities** (40%) - Security scanning
6. 🔄 **tag_image** (50%) - Image tagging
7. 🔄 **push_image** (60%) - Push to registry
8. 🔄 **generate_manifests** (70%) - Kubernetes manifests
9. 🔄 **setup_cluster** (80%) - Cluster preparation
10. 🔄 **deploy_application** (90%) - Application deployment
11. 🔄 **verify_deployment** (100%) - Health verification
12. 🎉 **workflow_complete** (100%) - Final completion

### Monitoring Examples

#### Using grep to track progress:
```bash
# Monitor all progress events
container-kit-mcp 2>&1 | grep "PROGRESS"

# Monitor only completion events
container-kit-mcp 2>&1 | grep "✅ PROGRESS"

# Monitor failures
container-kit-mcp 2>&1 | grep "❌ PROGRESS"
```

#### Using jq for structured monitoring:
```bash
# Extract progress percentage
container-kit-mcp 2>&1 | grep "PROGRESS" | jq -r '.percentage'

# Show current step and message
container-kit-mcp 2>&1 | grep "PROGRESS" | jq -r '"\(.step)/\(.total): \(.message)"'
```

### Integration with Monitoring Systems

#### Prometheus/Grafana
Parse the structured logs to extract metrics:
- `containerkit_workflow_progress{step, total}` - Current progress
- `containerkit_step_duration_seconds{step_name}` - Step duration
- `containerkit_workflow_status{status}` - Workflow status

#### Custom Monitoring
The structured format makes it easy to build custom monitoring:

```go
type ProgressEvent struct {
    Step       int     `json:"step"`
    Total      int     `json:"total"`
    Percentage int     `json:"percentage"`
    StepName   string  `json:"step_name"`
    Status     string  `json:"status"`
    Message    string  `json:"message"`
    Duration   string  `json:"duration,omitempty"`
    Error      string  `json:"error,omitempty"`
}
```

## Troubleshooting

### No Progress Logs Appearing
- Check log level configuration (should be INFO or DEBUG)
- Verify the workflow is actually running
- Check for log filtering that might hide PROGRESS events

### Missing Step Updates
- Some steps may complete very quickly
- Check for ERROR logs that might indicate step failures
- Review the workflow execution flow

### Performance Impact
- Progress logging has minimal performance impact
- Structured logging is optimized for production use
- Consider log rotation for long-running deployments