# Logs Export Tool

The `get_logs` tool allows MCP clients to retrieve server logs with powerful filtering capabilities, eliminating the need for direct log file access on client computers.

## Overview

The MCP server captures logs in a ring buffer (circular buffer) with a configurable capacity. This allows clients to retrieve recent log entries through the MCP protocol without requiring file system access or log aggregation infrastructure.

## Features

- **Ring Buffer Storage**: Logs are stored in memory with automatic rotation
- **Multi-level Filtering**: Filter by log level, time range, and text patterns
- **Flexible Output**: JSON structured format or plain text
- **Performance**: Efficient in-memory storage with concurrent access support
- **Privacy**: Optional caller information redaction

## Tool Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `level` | string | `"info"` | Minimum log level to include (trace, debug, info, warn, error) |
| `time_range` | string | `""` | Time range filter (e.g., "5m", "1h", "24h") |
| `pattern` | string | `""` | Pattern to search for in log messages and fields |
| `limit` | int | `100` | Maximum number of log entries to return |
| `format` | string | `"json"` | Output format ("json" or "text") |
| `include_callers` | bool | `false` | Include source code location information |

## Usage Examples

### Get Recent Logs

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_logs",
    "arguments": {
      "format": "json",
      "limit": 50
    }
  },
  "id": 1
}
```

### Filter by Log Level

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_logs",
    "arguments": {
      "level": "warn",
      "format": "json"
    }
  },
  "id": 1
}
```

### Get Logs from Last Hour

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_logs",
    "arguments": {
      "time_range": "1h",
      "format": "json"
    }
  },
  "id": 1
}
```

### Search for Pattern

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_logs",
    "arguments": {
      "pattern": "error",
      "time_range": "30m",
      "format": "text"
    }
  },
  "id": 1
}
```

### Get Plain Text Logs with Caller Info

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_logs",
    "arguments": {
      "format": "text",
      "include_callers": true,
      "limit": 20
    }
  },
  "id": 1
}
```

## Response Format

### JSON Format Response

```json
{
  "logs": [
    {
      "timestamp": "2025-06-22T10:30:00Z",
      "level": "info",
      "message": "Server started successfully",
      "fields": {
        "port": 8080,
        "transport": "stdio"
      },
      "caller": "server.go:123"
    }
  ],
  "total_count": 1523,
  "filtered_count": 25,
  "time_range": "1h",
  "oldest_entry": "2025-06-22T09:30:00Z",
  "newest_entry": "2025-06-22T10:30:00Z",
  "format": "json"
}
```

### Text Format Response

```json
{
  "log_text": "[2025-06-22 10:30:00.123] INFO Server started successfully port=8080 transport=stdio\n[2025-06-22 10:30:01.456] DEBUG Processing request method=GET path=/health",
  "total_count": 1523,
  "filtered_count": 25,
  "time_range": "1h",
  "oldest_entry": "2025-06-22T09:30:00Z",
  "newest_entry": "2025-06-22T10:30:00Z",
  "format": "text"
}
```

## Log Entry Structure

Each log entry contains:

- `timestamp`: ISO 8601 formatted timestamp
- `level`: Log level (trace, debug, info, warn, error, fatal, panic)
- `message`: The log message
- `fields`: Structured fields as key-value pairs (optional)
- `caller`: Source code location (optional, only if include_callers=true)

## Log Level Filtering

When filtering by level, the tool returns all logs at or above the specified level:

- `trace`: All logs
- `debug`: debug, info, warn, error, fatal, panic
- `info`: info, warn, error, fatal, panic
- `warn`: warn, error, fatal, panic
- `error`: error, fatal, panic

## Pattern Matching

The pattern search is case-insensitive and searches in:
- Log messages
- String values in structured fields

## Error Handling

Errors are returned in the response:

```json
{
  "logs": null,
  "total_count": 0,
  "filtered_count": 0,
  "format": "json",
  "error": {
    "type": "INVALID_TIME_RANGE",
    "message": "Invalid time range format: time: invalid duration \"bad\"",
    "retryable": false,
    "timestamp": "2025-06-22T10:30:00Z"
  }
}
```

## Implementation Details

### Ring Buffer

The server uses a ring buffer with:
- Default capacity: 10,000 log entries
- Automatic rotation when full (oldest entries are overwritten)
- Thread-safe concurrent access
- Minimal memory overhead

### Performance Considerations

- Log filtering is performed in-memory
- Pattern matching uses simple substring search
- Time-based filtering leverages chronological ordering
- Results are limited to prevent large responses

### Privacy and Security

- Caller information is excluded by default
- No persistent storage of logs (memory only)
- Logs are cleared on server restart
- Pattern search doesn't support regex to prevent ReDoS

## Testing

To test the logs export tool:

1. Start the MCP server:
   ```bash
   ./container-kit-mcp
   ```

2. Generate some log activity by using other tools

3. Retrieve logs:
   ```bash
   echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_logs","arguments":{"format":"text","limit":10}},"id":1}' | ./container-kit-mcp
   ```

## Limitations

- Logs are stored in memory only (not persisted)
- Ring buffer has fixed capacity (older logs are lost)
- Pattern matching is substring-based (no regex support)
- No log aggregation across multiple server instances
