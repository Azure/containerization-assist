# Telemetry Export Tool

The `get_telemetry_metrics` tool allows MCP clients to export Prometheus metrics from the MCP server without requiring HTTP access to the metrics endpoint.

## Overview

When the MCP server runs on client computers, accessing the Prometheus HTTP metrics endpoint can be challenging due to firewall restrictions or network configurations. This tool solves that problem by exposing metrics through the MCP protocol itself.

## Prerequisites

- MCP server must be started with telemetry enabled: `--enable-telemetry`
- Conversation mode must be enabled (telemetry is part of conversation components)

## Tool Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `format` | string | `"prometheus"` | Output format for metrics. Currently supports `"prometheus"` and `"json"` (JSON returns Prometheus text format) |
| `metric_names` | string[] | `[]` | Filter by specific metric names (e.g., `["mcp_tool_executions_total", "mcp_active_sessions"]`) |
| `include_help` | bool | `true` | Include metric HELP text in output |
| `include_empty` | bool | `false` | Include metrics with zero values |

## Usage Examples

### Export All Metrics

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_telemetry_metrics",
    "arguments": {
      "format": "prometheus"
    }
  },
  "id": 1
}
```

### Filter Specific Metrics

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_telemetry_metrics",
    "arguments": {
      "format": "prometheus",
      "metric_names": ["mcp_tool_executions_total", "llm_prompt_tokens_total"],
      "include_help": true
    }
  },
  "id": 1
}
```

### Exclude Empty Metrics

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_telemetry_metrics",
    "arguments": {
      "format": "prometheus",
      "include_empty": false
    }
  },
  "id": 1
}
```

## Response Format

```json
{
  "metrics": "# HELP mcp_tool_executions_total Total number of tool executions\n# TYPE mcp_tool_executions_total counter\nmcp_tool_executions_total{dry_run=\"false\",status=\"success\",tool=\"analyze_repository\"} 5\n...",
  "format": "prometheus",
  "metric_count": 15,
  "export_timestamp": "2025-06-22T10:30:00Z",
  "server_uptime": "2h30m15s"
}
```

## Available Metrics

The MCP server tracks the following metrics:

### Tool Execution Metrics
- `mcp_tool_executions_total`: Total number of tool executions (counter)
- `mcp_tool_duration_seconds`: Tool execution duration in seconds (histogram)
- `mcp_tool_errors_total`: Total number of tool errors (counter)

### Token Usage Metrics
- `llm_prompt_tokens_total`: Total prompt tokens used by model and tool (counter)
- `llm_completion_tokens_total`: Total completion tokens used by model and tool (counter)
- `mcp_tokens_used_total`: Total tokens used by tool (legacy counter)

### Session Metrics
- `mcp_active_sessions`: Number of currently active sessions (gauge)
- `mcp_session_duration_seconds`: Session duration in seconds (histogram)
- `mcp_stage_transitions_total`: Conversation stage transitions (counter)

### Pre-flight Check Metrics
- `mcp_preflight_results_total`: Pre-flight check results by check name and status (counter)

## Error Handling

If the telemetry export fails, the response will include an error field:

```json
{
  "metrics": "",
  "format": "prometheus",
  "metric_count": 0,
  "export_timestamp": "2025-06-22T10:30:00Z",
  "server_uptime": "0s",
  "error": {
    "type": "EXPORT_FAILED",
    "message": "Failed to export metrics: telemetry not initialized",
    "retryable": true,
    "timestamp": "2025-06-22T10:30:00Z"
  }
}
```

## Implementation Details

The tool:
1. Calls the `TelemetryManager.ExportMetrics()` method to get Prometheus-formatted metrics
2. Optionally filters metrics by name
3. Optionally removes metrics with zero values
4. Counts the number of metrics in the output
5. Includes server uptime calculated from tool creation time

## Testing

To test the telemetry export tool:

1. Start the MCP server with telemetry enabled:
   ```bash
   ./container-kit-mcp --enable-telemetry
   ```

2. Use an MCP client to call the tool:
   ```bash
   # Example using stdio transport
   echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_telemetry_metrics","arguments":{}},"id":1}' | ./container-kit-mcp --enable-telemetry
   ```

3. The response will include Prometheus-formatted metrics that can be parsed by monitoring tools.