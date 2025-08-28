# Containerization Assist User Guide for VS Code Copilot

*A Simple Guide to Set Up and Use the Containerization Assist MCP Server with VS Code Copilot*

This guide will help you set up Containerization Assist MCP Server to work with VS Code Copilot, even if you're not technical. Containerization Assist helps you automatically containerize your applications using AI assistance directly in your VS Code editor through GitHub Copilot Chat.

## Table of Contents

1. [What is Containerization Assist?](#what-is-containerization-assist)
2. [Prerequisites](#prerequisites)
3. [Step 1: Install Containerization Assist](#step-1-install-containerization-assist)
4. [Step 2: Set Up VS Code with Copilot](#step-2-set-up-vs-code-with-copilot)
5. [Step 3: Configure MCP Integration](#step-3-configure-mcp-integration)
6. [Step 4: Test Your Setup](#step-4-test-your-setup)
7. [How to Use Containerization Assist](#how-to-use-containerization-assist)
8. [Updating to New Releases](#updating-to-new-releases)
9. [Troubleshooting](#troubleshooting)
10. [Getting Help](#getting-help)

---

## What is Containerization Assist?

Containerization Assist is a tool that helps you:
- **Containerize your applications** - Turn your code into containers (like packaging your app)
- **Deploy to Kubernetes** - Put your app in the cloud  
- **Analyze your code** - Understand what technologies your project uses
- **Generate Docker files** - Create the files needed to containerize your app

It works as an MCP (Model Context Protocol) server that connects to VS Code Copilot, giving you containerization capabilities directly in your editor through AI-powered chat.

---

## Prerequisites

Before we start, make sure you have:

### Required Software
- **VS Code** - [Download here](https://code.visualstudio.com/)
- **GitHub Copilot** - [Get subscription](https://github.com/features/copilot)
- **Docker** - [Download here](https://www.docker.com/products/docker-desktop/)
- **Git** - [Download here](https://git-scm.com/downloads)

### System Requirements
- **Windows 10/11**, **macOS 10.15+**, or **Linux**
- **4GB RAM** minimum
- **2GB free disk space**

---

## Step 1: Install Containerization Assist

We've made installation super simple with our automated script.

### Option A: Easy Installation (Recommended)

**For Windows (PowerShell as Administrator):**
```powershell
# Run this command in PowerShell (right-click "Run as Administrator")
Set-ExecutionPolicy Bypass -Scope Process -Force; Invoke-WebRequest -Uri https://raw.githubusercontent.com/Azure/containerization-assist/main/scripts/setup-vscode.ps1 -OutFile setup-vscode.ps1; ./setup-vscode.ps1; Remove-Item setup-vscode.ps1
```

**For macOS/Linux:**
```bash
# Run this command in Terminal
curl -sSL https://raw.githubusercontent.com/Azure/containerization-assist/main/scripts/setup-vscode.sh | bash
```

This script will:
- ✅ Download Containerization Assist
- ✅ Install it in the right location
- ✅ Configure VS Code MCP integration
- ✅ Set up GitHub Copilot connection

*[Image placeholder: Screenshot of terminal showing successful installation]*

### Option B: Manual Installation

If the automated script doesn't work, follow these steps:

1. **Download Containerization Assist MCP Server**
   - Go to: https://github.com/Azure/containerization-assist/releases/latest
   - Download the archive matching your OS/architecture (examples):
     - Windows (x64): `containerization-assist-mcp_windows_amd64.zip`
     - Windows (ARM64): `containerization-assist-mcp_windows_arm64.zip`
     - macOS (Intel): `containerization-assist-mcp_darwin_amd64.tar.gz`
     - macOS (Apple Silicon): `containerization-assist-mcp_darwin_arm64.tar.gz`
     - Linux (x64): `containerization-assist-mcp_linux_amd64.tar.gz`
     - Linux (ARM64): `containerization-assist-mcp_linux_arm64.tar.gz`

2. **Extract the files**
   - Windows: Right-click the zip file → "Extract All"
   - macOS/Linux: Double-click the tar.gz file

3. **Move to the right location**
  - Windows: Move `containerization-assist-mcp.exe` to `C:\Program Files\ContainerizationAssist\` (or another directory on PATH)
  - macOS/Linux: Move `containerization-assist-mcp` to `/usr/local/bin/` (or `$HOME/bin` on PATH)

### Verify Installation

Open a terminal/command prompt and run:
```bash
containerization-assist-mcp --version
```

You should see something like:
```
Containerization Assist MCP Server v1.0.0
```

---

## Step 2: Set Up VS Code with Copilot

### Install Required Extensions

1. **Open VS Code**
2. **Click the Extensions icon** (looks like building blocks) in the left sidebar
3. **Search for and install these extensions:**
   - **GitHub Copilot** - AI pair programmer
   - **GitHub Copilot Chat** - Chat interface for Copilot
   - **MCP for VS Code** - Model Context Protocol support
   - **Docker** - For container support
   - **Kubernetes** - For deployment support (optional)

*[Image placeholder: VS Code Extensions marketplace showing required extensions]*

### Sign in to GitHub Copilot

1. **Open VS Code**
2. **Press** `Ctrl+Shift+P` (Windows/Linux) or `Cmd+Shift+P` (macOS)
3. **Type** "GitHub Copilot: Sign In"
4. **Follow the prompts** to authenticate with your GitHub account
5. **Make sure you have an active Copilot subscription**

*[Image placeholder: VS Code showing GitHub Copilot sign-in process]*

### Verify Copilot is Working

1. **Create a new file** (any programming language)
2. **Start typing a comment** like `// Function to add two numbers`
3. **You should see Copilot suggestions** appear
4. **If you see suggestions, Copilot is working!**

*[Image placeholder: VS Code showing Copilot suggestions in action]*

---

## Step 3: Configure MCP Integration

We use a user-level `mcp.json` file as the single source of truth (no edits to `settings.json`).

### Create or Edit mcp.json

Location by platform:
- **macOS:** `~/Library/Application Support/Code/User/mcp.json`
- **Linux:** `~/.config/Code/User/mcp.json`
- **Windows:** `%APPDATA%/Code/User/mcp.json`

If the file doesn't exist, create it with this minimal configuration:

```json
{
  "servers": {
    "containerization-assist": {
      "type": "stdio",
      "command": "containerization-assist-mcp",
      "args": []
    }
  }
}
```

Notes:
- Use just `containerization-assist-mcp` if it's on your PATH; otherwise supply the absolute path.
- Keep the server name `containerization-assist` (lowercase, hyphen) for consistency with prompts.
- Restart VS Code after saving to load changes.

### Optional: Enable Debug Logging Early

Add an `env` block to increase log verbosity:

```json
{
  "servers": {
    "containerization-assist": {
      "type": "stdio",
      "command": "containerization-assist-mcp",
      "args": [],
      "env": { "CONTAINERIZATION_ASSIST_LOG_LEVEL": "debug" }
    }
  }
}
```

*[Image placeholder: mcp.json open in VS Code]*

---

## Step 4: Test Your Setup

### Test Containerization Assist Directly

1. **Open a terminal/command prompt**
2. **Run this command:**

```bash
containerization-assist-mcp --version
```

You should see the version information.

*[Image placeholder: Terminal showing successful version command]*

### Test with VS Code Copilot

1. **Restart VS Code** (close and reopen it)
2. **Open the Copilot Chat panel** (click the chat icon in the sidebar or press `Ctrl+Alt+I`)
3. **Type:** "What Containerization Assist tools are available?"
4. **You should see** Copilot list the available Containerization Assist tools using MCP

*[Image placeholder: VS Code Copilot Chat showing Containerization Assist tools list]*

---

## How to Use Containerization Assist

Containerization Assist provides 13 different tools that work together to containerize your applications. Here's how to use them through VS Code Copilot:

### Quick Start: Complete Workflow

1. **Open your project** in VS Code
2. **Open Copilot Chat** (click the chat icon or press `Ctrl+Alt+I`)
3. **Ask Copilot:** "Use Containerization Assist to help me containerize this application"
4. **Copilot will use Containerization Assist** to analyze, containerize, and deploy your app
5. **Follow Copilot's guidance** through each step in your editor

*[Image placeholder: VS Code Copilot Chat conversation showing containerization workflow]*

### Individual Tools

You can also ask Copilot to use specific tools:

#### 1. Analyze Your Code
- **Ask:** "Use Containerization Assist to analyze this repository"
- **What it does:** Examines your code and detects the programming language, framework, and dependencies

#### 2. Create Dockerfile
- **Ask:** "Use Containerization Assist to generate a Dockerfile for this project"
- **What it does:** Creates a Dockerfile automatically based on your code

#### 3. Build Container
- **Ask:** "Use Containerization Assist to build a Docker image from this code"
- **What it does:** Builds a Docker container from your code

#### 4. Deploy to Kubernetes
- **Ask:** "Use Containerization Assist to deploy this application to Kubernetes"
- **What it does:** Creates and applies Kubernetes manifests

*[Image placeholder: VS Code Copilot Chat showing different Containerization Assist commands]*

### Example Conversations

**Complete Workflow:**
```
You: I have a Node.js app in this workspace. Can you help me containerize it?

Copilot: I'll help you containerize your Node.js application using Containerization Assist. Let me start by analyzing your repository...

[Copilot uses Containerization Assist tools to analyze, generate Dockerfile, build image, etc.]
```

**Specific Task:**
```
You: Can you use Containerization Assist to check what technologies this project uses?

Copilot: I'll analyze your project using Containerization Assist to identify the technologies and dependencies...

[Copilot uses the analyze_repository tool on your current workspace]
```

**Working with Files:**
```
You: Use Containerization Assist to create a Dockerfile, then show me the generated file

Copilot: I'll use Containerization Assist to generate a Dockerfile for your project and then display it...

[Copilot generates the Dockerfile and opens it in VS Code for you to review]
```

---

## Updating to New Releases

Containerization Assist releases updates regularly with new features and bug fixes.

### Automatic Update (Recommended)

Use our update script to get the latest version:

**Windows:**
```powershell
# Run in PowerShell as Administrator
Invoke-WebRequest -Uri https://raw.githubusercontent.com/Azure/containerization-assist/main/scripts/update-user.ps1 -OutFile update-user.ps1; ./update-user.ps1; Remove-Item update-user.ps1
```

**macOS/Linux:**
```bash
# Run in Terminal
curl -sSL https://raw.githubusercontent.com/Azure/containerization-assist/main/scripts/update-user.sh | bash
```

**After updating:**
1. **Restart VS Code** to use the new version
2. **Test the connection** by asking Copilot about Containerization Assist tools

*[Image placeholder: Terminal showing successful update process]*

### Manual Update

1. **Check your current version:**
   ```bash
   containerization-assist-mcp --version
   ```

2. **Check the latest version** at: https://github.com/Azure/containerization-assist/releases/latest

3. **If there's a newer version**, download and install it following the same steps as the initial installation

4. **Restart VS Code** after updating

---

## Troubleshooting

### Common Issues and Solutions

#### "Command not found: containerization-assist-mcp"

**Problem:** Containerization Assist isn't installed correctly or not in your PATH.

**Solutions:**
1. **Re-run the installation script**
2. **Check if the file exists:**
   - Windows: Look for `containerization-assist-mcp.exe` in `C:\Program Files\ContainerizationAssist\`
   - macOS/Linux: Look for `containerization-assist-mcp` in `/usr/local/bin/`
3. **Add to PATH manually** (ask your IT team for help)

#### Copilot doesn't see Containerization Assist tools

**Problem:** The MCP server isn't configured correctly in VS Code.

**Solutions:**
1. **Open your `mcp.json`** and verify the `servers.containerization-assist` block.
2. **Validate JSON syntax** (VS Code shows red squiggles on errors).
3. **Confirm MCP extension is installed & enabled**.
4. **Restart VS Code** after edits.
5. **Ensure `containerization-assist-mcp` resolves** (run `which containerization-assist-mcp` / `where containerization-assist-mcp`).

*[Image placeholder: VS Code showing MCP configuration and Copilot Chat connection status]*

#### "Docker not found"

**Problem:** Docker isn't installed or running.

**Solutions:**
1. **Install Docker Desktop** from [docker.com](https://www.docker.com/products/docker-desktop/)
2. **Start Docker Desktop** and wait for it to be running
3. **Test Docker** by running `docker --version` in terminal

#### "Permission denied" errors

**Problem:** Containerization Assist doesn't have the right permissions.

**Solutions:**
1. **Windows:** Run the installation script as Administrator
2. **macOS/Linux:** Use `sudo` with the installation commands
3. **Check file permissions** on the Containerization Assist executable

### Getting Debug Information

If you're having trouble, enable debug logging via `mcp.json`:

1. **Edit `mcp.json`** and add the env block:
```json
{
  "servers": {
    "containerization-assist": {
      "type": "stdio",
      "command": "containerization-assist-mcp",
      "args": [],
      "env": { "CONTAINERIZATION_ASSIST_LOG_LEVEL": "debug" }
    }
  }
}
```
2. **Restart VS Code**
3. **Retry the failing action**
4. **Open Output panel** (View → Output) and select the MCP-related channel (e.g. "MCP" or extension-specific).

*[Image placeholder: VS Code Output panel showing Containerization Assist debug logs]*

---

## Getting Help

### Documentation and Resources

- **Official Documentation:** [GitHub Repository](https://github.com/Azure/containerization-assist)
- **Examples:** (Coming soon)
- **MCP Documentation:** [Model Context Protocol](https://modelcontextprotocol.io/)

### Community Support

- **🐛 Bug Reports:** Use our [Bug Report Template](https://github.com/Azure/containerization-assist/issues/new?assignees=&labels=bug%2Cneeds-triage&projects=&template=bug-report.yml) 
- **✨ Feature Requests:** Use our [Feature Request Template](https://github.com/Azure/containerization-assist/issues/new?assignees=&labels=enhancement%2Cneeds-triage&projects=&template=feature-request.yml)
- **💬 Questions & Help:** Ask in [GitHub Discussions](https://github.com/Azure/containerization-assist/discussions)

### How to Report Issues

We have detailed templates to help you report bugs and request features effectively:

#### 🐛 Reporting Bugs

1. **Go to:** https://github.com/Azure/containerization-assist/issues/new?template=bug-report.yml
2. **Fill out the template** - it will guide you through providing all needed information
3. **Attach logs** (see [Gathering Logs](#gathering-logs) below)
4. **Remove sensitive data** like API keys or personal information

#### ✨ Requesting Features

1. **Go to:** https://github.com/Azure/containerization-assist/issues/new?template=feature-request.yml
2. **Describe your use case** and why the feature would be valuable
3. **Provide examples** of how you'd like it to work
4. **Consider alternatives** and mention any similar features you've seen

### Gathering Logs

When reporting issues, logs help us understand what went wrong. Here's how to gather them:

#### Method 1: Enable Debug Logging

1. **Update your `mcp.json`:**
```json
{
  "servers": {
    "containerization-assist": {
      "type": "stdio",
      "command": "containerization-assist-mcp",
      "args": [],
      "env": { "CONTAINERIZATION_ASSIST_LOG_LEVEL": "debug" }
    }
  }
}
```

2. **Restart VS Code**
3. **Try to reproduce the issue**
4. **Check VS Code Output panel:**
   - **Go to:** View → Output
   - **Select:** "MCP" or "Containerization Assist" from the dropdown
   - **Look for error messages** or debug information

#### Method 2: Run Containerization Assist Directly

1. **Open terminal/command prompt**
2. **Set debug level:**
   ```bash
   # Windows
   set CONTAINERIZATION_ASSIST_LOG_LEVEL=debug
   
   # macOS/Linux
   export CONTAINERIZATION_ASSIST_LOG_LEVEL=debug
   ```
3. **Run Containerization Assist:**
   ```bash
   containerization-assist-mcp
   ```
4. **Try to reproduce the issue in another terminal**
5. **Copy the log output from the first terminal**

#### Method 3: Use Log Files

Containerization Assist can write logs to files:

```bash
# Windows
set CONTAINERIZATION_ASSIST_LOG_FILE=containerization-assist.log
containerization-assist-mcp

# macOS/Linux
export CONTAINERIZATION_ASSIST_LOG_FILE=containerization-assist.log
containerization-assist-mcp
```

Then attach the `containerization-assist.log` file to your issue.

### What Information to Include

When reporting issues, always include:

1. **Containerization Assist version:** `containerization-assist-mcp --version`
2. **Operating system:** Windows 11, macOS 14.1, Ubuntu 22.04, etc.
3. **VS Code version:** Check Help → About in VS Code
4. **GitHub Copilot version:** Check Extensions panel in VS Code
5. **Steps to reproduce:** Exact steps you took
6. **Expected vs actual behavior:** What should happen vs what did happen
7. **Logs:** Debug logs from VS Code Output panel (sanitized of sensitive data)
8. **Configuration:** Your `mcp.json` server configuration (remove sensitive values)

#### Quick Info Script

Save this script to quickly gather system information:

**Windows (save as `gather-info.bat`):**
```batch
@echo off
echo === Containerization Assist Debug Information ===
echo.
echo Containerization Assist Version:
containerization-assist-mcp --version
echo.
echo Operating System:
systeminfo | findstr /B /C:"OS Name" /C:"OS Version"
echo.
echo Docker Version:
docker --version 2>nul || echo Docker not found
echo.
echo Current Directory:
cd
echo.
echo Environment Variables:
set | findstr CONTAINERIZATION_ASSIST
```

**macOS/Linux (save as `gather-info.sh`):**
```bash
#!/bin/bash
echo "=== Containerization Assist Debug Information ==="
echo
echo "Containerization Assist Version:"
containerization-assist-mcp --version
echo
echo "Operating System:"
uname -a
echo
echo "Docker Version:"
docker --version 2>/dev/null || echo "Docker not found"
echo
echo "Current Directory:"
pwd
echo
echo "Environment Variables:"
env | grep CONTAINERIZATION_ASSIST
```

Run the script and include its output when reporting issues.

*[Image placeholder: Screenshot showing debug log output with sensitive information redacted]*

---

## Available Containerization Assist Tools

Here are all the tools available in Containerization Assist:

### Workflow Tools (11 tools)
1. **analyze_repository** - Analyze your code and detect technologies
2. **resolve_base_images** - Get recommended base images for Dockerfile
3. **verify_dockerfile** - Verify Dockerfile for your project
4. **build_image** - Build a Docker container image
5. **scan_image** - Scan for security vulnerabilities
6. **tag_image** - Tag your image with version information
7. **push_image** - Push your image to a container registry
8. **verify_k8s_manifests** - Verify Kubernetes deployment files
9. **prepare_cluster** - Set up your Kubernetes cluster
10. **deploy_application** - Deploy your app to Kubernetes
11. **verify_deployment** - Check that your deployment is working

### Orchestration Tools (2 tools)
12. **start_workflow** - Run the complete containerization process
13. **workflow_status** - Check the progress of your workflow

### Utility Tools (1 tool)
14. **list_tools** - Show all available tools

*[Image placeholder: Diagram showing how the tools work together]*

---

## What's Next?

Congratulations! 🎉 You've successfully set up Containerization Assist with VS Code Copilot. Here are some things to try:

### Try These Examples

1. **Ask Copilot to containerize your current project**
2. **Have Copilot analyze your workspace repository**
3. **Get help generating Kubernetes manifests for your app**
4. **Use Copilot to explain Docker concepts while editing files**

### Learn More

- **Explore all 13 tools** by asking Copilot "What can Containerization Assist do?"
- **Read about Kubernetes** if you want to deploy to the cloud
- **Learn Docker basics** to understand what Containerization Assist is doing
- **Try the integrated workflow** - Copilot can edit files directly in your workspace

### Advanced Usage

- **Use Containerization Assist with Copilot Inline Chat** (`Ctrl+I`) for contextual help
- **Combine with other VS Code extensions** like Docker and Kubernetes
- **Set up workspace-specific configurations** for different projects

### Share Your Success

We'd love to hear about your experience! Share your success stories in our GitHub Discussions.

---

*Made with ❤️ by the Azure Containerization Assist team*

*Last updated: July 2025*