# Container Kit Quickstart Guide

Welcome to Container Kit! This guide will help you get started with containerizing your applications and generating Kubernetes manifests in minutes.

## Table of Contents

- [Installation](#installation)
  - [Quick Install (Recommended)](#quick-install-recommended)
  - [Manual Installation](#manual-installation)
  - [Installing Specific Versions](#installing-specific-versions)
- [First-Time Setup](#first-time-setup)
- [Basic Usage](#basic-usage)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)
- [Uninstalling](#uninstalling)

## Installation

### Quick Install (Recommended)

#### Linux/macOS

Open your terminal and run:

```bash
curl -sSL https://raw.githubusercontent.com/Azure/container-kit/main/scripts/install.sh | bash
```

Or with wget:

```bash
wget -qO- https://raw.githubusercontent.com/Azure/container-kit/main/scripts/install.sh | bash
```

#### Windows

Open PowerShell as Administrator and run:

```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force
Invoke-WebRequest -Uri https://raw.githubusercontent.com/Azure/container-kit/main/scripts/install.ps1 -OutFile install.ps1
./install.ps1
Remove-Item install.ps1
```

### Manual Installation

1. Visit the [releases page](https://github.com/Azure/container-kit/releases/latest)
2. Download the appropriate binary for your platform:
   - Linux: `container-kit_VERSION_linux_amd64.tar.gz` or `container-kit_VERSION_linux_arm64.tar.gz`
   - macOS: `container-kit_VERSION_darwin_amd64.tar.gz` or `container-kit_VERSION_darwin_arm64.tar.gz`
   - Windows: `container-kit_VERSION_windows_amd64.zip`
3. Extract the archive
4. Move the binary to a directory in your PATH (e.g., `/usr/local/bin` on Unix systems)
5. Make it executable (Unix only): `chmod +x container-kit`

### Installing Specific Versions

To install a specific version instead of the latest:

#### Linux/macOS
```bash
curl -sSL https://raw.githubusercontent.com/Azure/container-kit/main/scripts/install.sh | bash -s -- v1.2.3
```

#### Windows
```powershell
./install.ps1 -Version "v1.2.3"
```

### Verify Installation

After installation, verify that container-kit is working:

```bash
container-kit --version
```

## First-Time Setup

### Prerequisites

Container Kit requires the following tools to be installed:

1. **Docker** - For building container images
   - [Install Docker](https://docs.docker.com/get-docker/)
   - Verify: `docker --version`

2. **kubectl** - For deploying to Kubernetes
   - [Install kubectl](https://kubernetes.io/docs/tasks/tools/)
   - Verify: `kubectl version --client`

3. **Azure OpenAI Access** - For AI-powered analysis
   - Set up environment variables:
   ```bash
   export AZURE_OPENAI_KEY="your-api-key"
   export AZURE_OPENAI_ENDPOINT="your-endpoint"
   export AZURE_OPENAI_DEPLOYMENT_ID="your-deployment-id"
   ```

### Optional Tools

- **kind** - For local Kubernetes testing
  - [Install kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
  - Create a cluster: `kind create cluster`

- **Azure CLI** - For automatic Azure setup
  - [Install Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)

## Basic Usage

### Containerize an Application

The primary command is `generate`, which analyzes your application and creates a Dockerfile and Kubernetes manifests:

```bash
# Containerize the current directory
container-kit generate .

# Containerize a specific project
container-kit generate /path/to/your/project

# Use a custom registry
container-kit generate --registry myregistry.azurecr.io /path/to/project
```

### Using the Setup Wizard

For a guided experience, use the `setup` command:

```bash
container-kit setup --target-repo /path/to/your/project
```

This will:
1. Check all prerequisites
2. Guide you through configuration
3. Run the containerization process
4. Provide next steps

## Common Workflows

### 1. Containerizing a Node.js Application

```bash
cd ~/projects/my-node-app
container-kit generate .
```

Container Kit will:
- Detect Node.js and package.json
- Create an optimized multi-stage Dockerfile
- Generate Kubernetes deployment and service manifests
- Build and push the image (if registry is configured)

### 2. Containerizing a Python Application

```bash
container-kit generate --registry myacr.azurecr.io ~/projects/my-python-app
```

### 3. Testing Locally with Kind

```bash
# Create a kind cluster if you haven't already
kind create cluster

# Generate and deploy to kind
container-kit generate --deploy-to-kind ./my-app
```

### 4. Using MCP Server Mode

Container Kit also provides a Model Context Protocol (MCP) server for advanced integrations:

```bash
# Start the MCP server
container-kit-mcp

# Or with HTTP transport
container-kit-mcp --transport=http --port=8080
```

## Understanding the Output

After running `container-kit generate`, you'll find:

```
your-project/
├── .container-kit/           # Container Kit artifacts
│   ├── state.json           # Pipeline state
│   └── iterations/          # Snapshots of each iteration
├── Dockerfile               # Generated Dockerfile
├── k8s/                     # Kubernetes manifests
│   ├── deployment.yaml      # Deployment configuration
│   └── service.yaml         # Service configuration
└── ...
```

### What Happens During Generation

1. **Repository Analysis**: AI analyzes your codebase structure, dependencies, and configuration
2. **Dockerfile Generation**: Creates an optimized Dockerfile using best practices
3. **Build Attempt**: Tries to build the Docker image
4. **Iterative Fixes**: If build fails, AI analyzes errors and fixes the Dockerfile
5. **Manifest Generation**: Creates Kubernetes deployment and service manifests
6. **Validation**: Deploys to a test cluster (if available) and validates

## Troubleshooting

### Common Issues

#### 1. "Docker daemon not running"
```bash
# Start Docker
sudo systemctl start docker  # Linux
open -a Docker               # macOS
```

#### 2. "Azure OpenAI credentials not found"
Ensure environment variables are set:
```bash
echo $AZURE_OPENAI_KEY
echo $AZURE_OPENAI_ENDPOINT
echo $AZURE_OPENAI_DEPLOYMENT_ID
```

#### 3. "Permission denied" during installation
- On Linux/macOS: The installer will try to use sudo automatically
- If that fails, install to user directory: `~/bin`

#### 4. Build failures
Check the iteration logs:
```bash
ls -la .container-kit/iterations/
cat .container-kit/iterations/docker_stage_iteration_*/error.log
```

### Getting Help

- Run `container-kit --help` for command options
- Check the [documentation](https://github.com/Azure/container-kit/tree/main/docs)
- Report issues on [GitHub](https://github.com/Azure/container-kit/issues)

## Uninstalling

### Linux/macOS

```bash
# Remove the binary
sudo rm /usr/local/bin/container-kit
# or
rm ~/bin/container-kit
```

### Windows

Run PowerShell as Administrator:
```powershell
./install.ps1 -Uninstall
```

Or manually delete:
```powershell
Remove-Item "$env:PROGRAMFILES\container-kit" -Recurse -Force
```

## Next Steps

- Explore [advanced features](https://github.com/Azure/container-kit/tree/main/docs)
- Learn about [MCP integration](./MCP_DOCUMENTATION.md)
- Read about [AI integration patterns](./AI_INTEGRATION_PATTERN.md)

## Tips for Success

1. **Start Simple**: Begin with a small project to understand the workflow
2. **Review Generated Files**: Always review the generated Dockerfile and manifests
3. **Use Version Control**: Commit the generated files to track changes
4. **Iterate**: The AI improves with each iteration - let it fix issues automatically
5. **Provide Context**: Add a README or documentation to help the AI understand your project

Happy containerizing! 🚀
