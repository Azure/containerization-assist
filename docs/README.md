# Containerization Assistant Documentation

Welcome to the Containerization Assistant MCP Server documentation.

## Quick Navigation

### User Guides
- **[Getting Started](./guides/getting-started.md)** - Installation, setup, usage, and development
- **[External Usage](./guides/external-usage.md)** - Using the MCP server in external applications
- **[Session Management](./guides/sessions.md)** - Understanding session state and tool interactions
- **[Usage Examples](./guides/examples/)** - Code examples for different integration scenarios

### Reference Documentation
- **[Architecture](./reference/architecture.md)** - System design, AI integration, and component overview
- **[Error Taxonomy](./reference/error-taxonomy.md)** - Error codes and troubleshooting reference
- **[Prompt Format Guide](./reference/prompt-format-guide.md)** - Standard prompt format and validation
- **[Architecture Decisions](./reference/adr/)** - Historical architecture decision records
- **[Prompt-Backed Tools](./reference/architecture/prompt-backed-tools.md)** - AI-first tool design patterns

### Development Documentation
- **[Language Framework Guide](./development/language-framework-guide.md)** - Adding new language and framework support
- **[Testing Patterns](./development/internal/testing-patterns-guide.md)** - Internal testing infrastructure guide
- **[Strategy & Flow Guide](./guides/strategy-flow-guide.md)** - Complete policy → strategy → prompt → knowledge flow

## Overview

The Containerization Assistant is a Model Context Protocol (MCP) server that provides AI-powered containerization workflows with Docker and Kubernetes support. It offers 17 tools for analyzing, building, scanning, and deploying containerized applications through natural language commands in VS Code and other MCP-compatible tools.

### Key Features

- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications
- 🤖 **AI-Powered**: Intelligent Dockerfile generation and optimization
- 🔄 **Workflow Orchestration**: Complete containerization pipelines
- 📊 **Progress Tracking**: Real-time progress updates via MCP
- 🔒 **Security Scanning**: Built-in vulnerability scanning with Trivy

### Quick Start

1. **Install**: `npm install -g @thgamble/containerization-assist-mcp`
2. **Configure VS Code**: Create `.vscode/mcp.json` (see [Getting Started](./guides/getting-started.md))
3. **Use**: Ask GitHub Copilot to "analyze my application for containerization"

## Document Organization

This documentation is organized into three main sections to serve different needs:

## Project Links

- **Main README**: [../README.md](../README.md) - Project overview and commands
- **CLAUDE.md**: [../CLAUDE.md](../CLAUDE.md) - Guidelines for Claude Code development
- **GitHub Repository**: [github.com/Azure/containerization-assist](https://github.com/Azure/containerization-assist)