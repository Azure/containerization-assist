# MCP Host Containerization Test Script

## Overview

The `test-mcphost.sh` script provides a comprehensive way to test the containerization functionality of this project using the MCP (Model Context Protocol) Host tool and Azure OpenAI integration.

## Prerequisites

### Required Tools
1. **Go** (version 1.22+) - For building the MCP server binary
2. **mcphost** - The MCP host CLI tool
3. **Azure OpenAI credentials** - Set up in a `.secrets` file

### Installing mcphost

Choose one of the following installation methods:

#### Option 1: Download Pre-built Binary
```bash
# Download and install mcphost
VERSION="v0.29.0"
curl -L -o mcphost.tar.gz "https://github.com/mark3labs/mcphost/releases/download/${VERSION}/mcphost_$(uname)_$(uname -m).tar.gz"
tar -xzf mcphost.tar.gz
sudo mv mcphost /usr/local/bin/
rm mcphost.tar.gz

# Verify installation
mcphost --version
```

#### Option 2: Build from Source
```bash
# Install Go if not already installed, then:
go install github.com/mark3labs/mcphost@latest
```

#### Option 3: Using Homebrew (macOS)
```bash
brew install mark3labs/tap/mcphost
```

## Setup

### 1. Azure OpenAI Configuration

Create a `.secrets` file in the project root with your Azure OpenAI credentials:

```bash
AZURE_OPENAI_DEPLOYMENT_ID=your-deployment-id
AZURE_OPENAI_KEY=your-api-key
AZURE_OPENAI_ENDPOINT=your-endpoint
```

**Example:**
```bash
AZURE_OPENAI_DEPLOYMENT_ID=gpt-4o
AZURE_OPENAI_KEY=F7Dz7atg3q7r8pExrEBBqrxTA7q6DSVLRWz5TXhxzU9JTNjbe1ClJQQJ99BDACHYHv6XJ3w3AAABACOGExrp
AZURE_OPENAI_ENDPOINT=https://containercopilotai-testservice.openai.azure.com/
```

### 2. Verify Go Installation

```bash
go version
# Should show Go 1.22 or higher
```

## Usage

### Basic Usage
```bash
# Test with default repository (konveyor-ecosystem/coolstore)
./test-mcphost.sh

# Test with a specific repository
./test-mcphost.sh https://github.com/your-org/your-repo
```

### What the Script Does

1. **Pre-flight Checks**
   - Verifies `mcphost` and `go` are installed
   - Checks for `.secrets` file with Azure OpenAI credentials

2. **Environment Setup**
   - Loads Azure OpenAI environment variables
   - Creates workspace directories
   - Sets up proper file permissions

3. **MCP Server Setup**
   - Builds the custom containerization-assist MCP server binary (if needed)
   - Creates MCP configuration with filesystem, bash, and containerization tools
   - Tests the MCP connection with a simple calculation

4. **Containerization Process**
   - Runs full containerization of the specified repository
   - Monitors progress with timeout and stagnation detection
   - Handles auto-continuation if the process stalls

5. **Artifact Validation**
   - Searches for generated Dockerfiles
   - Validates Kubernetes manifests
   - Shows content of all generated files
   - Provides comprehensive summary

### Expected Output

The script will generate:
- **Dockerfile(s)** - Optimized container images for the application
- **Kubernetes Manifests** - deployment.yaml, service.yaml, etc.
- **Configuration Files** - ConfigMaps, Secrets as needed
- **Detailed Logs** - Complete process logs for debugging

### Output Locations

- **Workspace**: `./mcp-test-workspace/`
- **Generated Files**: `./mcp-test-workspace/containerization-output/`
- **Logs**: `./mcp-test-workspace/mcp-containerization.log`

## Example Successful Run

```bash
$ ./test-mcphost.sh

=== MCP Host Containerization Test ===
Repository: https://github.com/konveyor-ecosystem/coolstore
Workspace: ./mcp-test-workspace

[STEP] Running pre-flight checks...
[SUCCESS] All pre-flight checks passed

[STEP] Setting up environment...
[INFO] Azure OpenAI Deployment: gpt-4o
[INFO] Azure OpenAI Endpoint: https://containercopilotai-testservice.openai.azure.com/
[SUCCESS] Environment setup complete

[STEP] Checking MCP server binary...
[INFO] Using existing MCP server binary
[SUCCESS] MCP server binary is ready

[STEP] Creating MCP configuration...
[SUCCESS] MCP configuration created

[STEP] Testing MCP connection...
Testing with simple calculation: 2 + 3
[SUCCESS] MCP connection test passed

[STEP] Starting containerization process...
[INFO] Repository: https://github.com/konveyor-ecosystem/coolstore
[INFO] Output directory: /path/to/output
[INFO] Timeout: 5 minutes
Starting containerization process...
Monitoring containerization progress...
[INFO] Progress detected (log size: 1245 bytes, elapsed: 15s)
[INFO] Progress detected (log size: 2890 bytes, elapsed: 30s)
[SUCCESS] Containerization process completed

[STEP] Validating generated artifacts...
[SUCCESS] Found Dockerfile(s): ./Dockerfile
[SUCCESS] Found Kubernetes manifest(s): ./deployment.yaml, ./service.yaml

📊 Artifacts generated:
   - Dockerfiles: 1
   - YAML files: 2
[SUCCESS] CONTAINERIZATION SUCCESS: Generated both Dockerfile and Kubernetes manifests

🎉 Test completed successfully!
📁 Output directory: ./mcp-test-workspace/containerization-output
📄 Logs available in: ./mcp-test-workspace/mcp-containerization.log
```

## Troubleshooting

### Common Issues

1. **mcphost not found**
   ```bash
   [ERROR] mcphost is not installed or not in PATH
   ```
   **Solution**: Install mcphost using one of the methods above

2. **Missing Azure OpenAI credentials**
   ```bash
   [ERROR] .secrets file not found
   ```
   **Solution**: Create `.secrets` file with your Azure OpenAI credentials

3. **Go not installed**
   ```bash
   [ERROR] Go is not installed
   ```
   **Solution**: Install Go from https://golang.org/

4. **MCP connection failed**
   ```bash
   [ERROR] MCP connection test failed
   ```
   **Solution**: Check Azure OpenAI credentials and network connectivity

### Debug Mode

For more detailed output, you can modify the script to run in debug mode:

```bash
# Add debug flag at the top of the script
set -x

# Or run with bash debug
bash -x ./test-mcphost.sh
```

## Integration with CI/CD

The script can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions step
- name: Test Containerization
  run: |
    # Install mcphost
    curl -L -o mcphost.tar.gz "https://github.com/mark3labs/mcphost/releases/download/v0.29.0/mcphost_Linux_x86_64.tar.gz"
    tar -xzf mcphost.tar.gz
    sudo mv mcphost /usr/local/bin/
    
    # Run test
    ./test-mcphost.sh
  env:
    AZURE_OPENAI_DEPLOYMENT_ID: ${{ secrets.AZURE_OPENAI_DEPLOYMENT_ID }}
    AZURE_OPENAI_KEY: ${{ secrets.AZURE_OPENAI_KEY }}
    AZURE_OPENAI_ENDPOINT: ${{ secrets.AZURE_OPENAI_ENDPOINT }}
```

## Next Steps

1. Install mcphost using one of the methods above
2. Set up your `.secrets` file with Azure OpenAI credentials
3. Run the script with `./test-mcphost.sh`
4. Review generated containerization artifacts
5. Use the generated Dockerfile and Kubernetes manifests in your deployment pipeline
