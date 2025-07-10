# Getting Started with Container Kit

## Installation

### Prerequisites
- Go 1.24.1 or later
- Docker Engine
- Git

### Building from Source

```bash
# Clone the repository
git clone https://github.com/Azure/container-kit.git
cd container-kit

# Build the MCP server
make mcp

# Run tests
make test
```

## Basic Usage

### 1. Start the MCP Server

```bash
# Run in chat mode (interactive)
./bin/mcp-server --mode chat

# Run in workflow mode (automation)
./bin/mcp-server --mode workflow

# Run in dual mode (both)
./bin/mcp-server --mode dual
```

### 2. Analyze a Repository

```bash
# Using the CLI
mcp-cli analyze --repo /path/to/your/app

# Using the API
curl -X POST http://localhost:8080/tools/analyze \
  -H "Content-Type: application/json" \
  -d '{"repository": "/path/to/your/app"}'
```

### 3. Build a Container

```bash
# Build with automatic Dockerfile generation
mcp-cli build --repo /path/to/your/app --tag myapp:latest

# Build with existing Dockerfile
mcp-cli build --dockerfile /path/to/Dockerfile --tag myapp:latest
```

### 4. Security Scanning

```bash
# Scan for vulnerabilities
mcp-cli scan --image myapp:latest --severity HIGH,CRITICAL
```

### 5. Deploy to Kubernetes

```bash
# Generate manifests
mcp-cli deploy --image myapp:latest --type kubernetes

# Deploy directly
mcp-cli deploy --image myapp:latest --kubeconfig ~/.kube/config
```

## Configuration

### Environment Variables

```bash
# Set log level
export CONTAINER_KIT_LOG_LEVEL=debug

# Enable tracing
export CONTAINER_KIT_TRACING_ENABLED=true

# Set timeout
export CONTAINER_KIT_TIMEOUT=5m
```

### Configuration File

Create `~/.container-kit/config.yaml`:

```yaml
server:
  mode: dual
  port: 8080

tools:
  timeout: 30s
  retry:
    max_attempts: 3
    backoff: exponential

storage:
  type: boltdb
  path: ~/.container-kit/data

monitoring:
  metrics: true
  tracing: true
  prometheus_port: 9090
```

## Common Workflows

### Containerize a Node.js Application

```bash
# Analyze and generate Dockerfile
mcp-cli analyze --repo ./my-node-app --framework node

# Review generated Dockerfile
cat ./my-node-app/Dockerfile

# Build and scan
mcp-cli build --repo ./my-node-app --tag my-node-app:latest
mcp-cli scan --image my-node-app:latest

# Deploy
mcp-cli deploy --image my-node-app:latest --type kubernetes > k8s-manifests.yaml
kubectl apply -f k8s-manifests.yaml
```

### Multi-Stage Python Build

```bash
# Create a workflow file
cat > containerize.yaml << EOF
name: containerize-python
stages:
  - name: analyze
    tool: analyze
    args:
      repository: ./my-python-app
      framework: python
      
  - name: optimize
    tool: optimize
    args:
      dockerfile: ./my-python-app/Dockerfile
      target_size: minimal
      
  - name: build
    tool: build
    args:
      dockerfile: ./my-python-app/Dockerfile
      tag: my-python-app:latest
      platforms:
        - linux/amd64
        - linux/arm64
        
  - name: scan
    tool: scan
    args:
      image: my-python-app:latest
      fail_on: CRITICAL
EOF

# Execute workflow
mcp-cli workflow run containerize.yaml
```

## Troubleshooting

### Common Issues

1. **Build failures**: Check Docker daemon is running
2. **Permission denied**: Ensure proper file permissions
3. **Timeout errors**: Increase timeout values
4. **Memory issues**: Adjust resource limits

### Debug Mode

```bash
# Enable debug logging
export CONTAINER_KIT_LOG_LEVEL=debug

# Run with verbose output
mcp-cli --verbose analyze --repo ./myapp

# Check logs
tail -f ~/.container-kit/logs/mcp-server.log
```

## Next Steps

- [API Documentation](../api/README.md)
- [Examples](../examples/README.md)
- [Advanced Configuration](./advanced-config.md)
- [Custom Tool Development](./custom-tools.md)
