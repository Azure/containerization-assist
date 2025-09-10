# Containerization Assistant Documentation

Welcome to the Containerization Assistant MCP Server documentation.

## Quick Navigation

### User Guides
- **[Getting Started](./getting-started.md)** - Installation, setup, and first containerization
- **[External Usage](./external-usage.md)** - Using the MCP server in various environments

### Developer Guides
- **[Development Setup](./development-setup.md)** - Local development environment, testing, and contribution guidelines
- **[Architecture](./architecture.md)** - System design, MCP features, and API reference
- **[Internal Docs](./internal/)** - Technical documentation for maintainers

### Examples
- **[Usage Examples](./examples/)** - Code examples for different integration scenarios

## Overview

The Containerization Assistant is a Model Context Protocol (MCP) server that provides AI-powered containerization workflows with Docker and Kubernetes support. It offers 14 tools for analyzing, building, scanning, and deploying containerized applications through natural language commands in VS Code and other MCP-compatible tools.

### Key Features

- 🐳 **Docker Integration**: Build, scan, and deploy container images
- ☸️ **Kubernetes Support**: Generate manifests and deploy applications
- 🤖 **AI-Powered**: Intelligent Dockerfile generation and optimization
- 🔄 **Workflow Orchestration**: Complete containerization pipelines
- 📊 **Progress Tracking**: Real-time progress updates via MCP
- 🔒 **Security Scanning**: Built-in vulnerability scanning with Trivy

### Quick Start

1. **Install**: `npm install -g @thgamble/containerization-assist-mcp`
2. **Configure VS Code**: Create `.vscode/mcp.json` (see [Getting Started](./getting-started.md))
3. **Use**: Ask GitHub Copilot to "analyze my application for containerization"

## Document Organization

This documentation is organized into three main sections to serve different needs:

## Project Links

- **Main README**: [../README.md](../README.md) - Project overview and commands
- **CLAUDE.md**: [../CLAUDE.md](../CLAUDE.md) - Guidelines for Claude Code development
- **GitHub Repository**: [github.com/azure/container-assistttps://github.com/azure/concontainer-assist