# VS Code Extension Integration Guide

This guide explains how to integrate the `containerization-assist-mcp` SDK with a VS Code extension implementing the Language Model Tool API.

## Table of Contents

1. [Overview](#overview)
2. [Installation](#installation)
3. [Package.json Configuration](#packagejson-configuration)
4. [Tool Implementation Pattern](#tool-implementation-pattern)
5. [Extension Activation](#extension-activation)
6. [Complete Tool Examples](#complete-tool-examples)
7. [Error Handling](#error-handling)
8. [Testing](#testing)
9. [API Reference](#api-reference)

---

## Overview

The SDK provides everything needed to create VS Code Language Model Tools:

| Export | Purpose |
|--------|---------|
| `analyzeRepo()`, `buildImage()`, etc. | Core tool functions |
| `jsonSchemas` | JSON Schemas for package.json `inputSchema` |
| `toolMetadata` | Descriptions, icons, confirmations for tool config |
| `resultFormatters` | Convert results to LLM-friendly text |
| `createAbortSignalFromToken()` | Convert VS Code CancellationToken |
| `formatErrorForLLM()` | Format errors for LLM consumption |

---

## Installation

```bash
npm install containerization-assist-mcp
```

The SDK is available via the `/sdk` export:

```typescript
import {
  analyzeRepo,
  jsonSchemas,
  toolMetadata,
  resultFormatters,
  createAbortSignalFromToken,
} from 'containerization-assist-mcp/sdk';
```

---

## Package.json Configuration

Use `jsonSchemas` and `toolMetadata` to configure your extension's `package.json`:

```json
{
  "contributes": {
    "languageModelTools": [
      {
        "name": "analyze_repo",
        "displayName": "Analyze Repository",
        "toolReferenceName": "containerization-analyze",
        "modelDescription": "Analyzes a repository to detect programming languages...",
        "userDescription": "Detect languages, frameworks, and dependencies",
        "icon": "$(search)",
        "canBeReferencedInPrompt": true,
        "inputSchema": {
          "type": "object",
          "properties": {
            "repositoryPath": {
              "type": "string",
              "description": "Absolute path to the repository"
            }
          },
          "required": ["repositoryPath"]
        }
      }
    ]
  }
}
```

### Generating package.json Entries

You can generate these entries programmatically:

```typescript
// scripts/generate-tool-config.ts
import { jsonSchemas, toolMetadata } from 'containerization-assist-mcp/sdk';
import { writeFileSync } from 'fs';

const tools = Object.entries(toolMetadata).map(([key, meta]) => ({
  name: meta.name,
  displayName: meta.displayName,
  toolReferenceName: meta.toolReferenceName,
  modelDescription: meta.modelDescription,
  userDescription: meta.userDescription,
  icon: meta.icon,
  canBeReferencedInPrompt: meta.canBeReferencedInPrompt,
  inputSchema: jsonSchemas[key as keyof typeof jsonSchemas],
}));

console.log(JSON.stringify({ languageModelTools: tools }, null, 2));
```

---

## Tool Implementation Pattern

Each tool follows this pattern:

```typescript
import * as vscode from 'vscode';
import {
  analyzeRepo,
  toolMetadata,
  resultFormatters,
  createAbortSignalFromToken,
  formatErrorForLLM,
  type AnalyzeRepoInput,
  type RepositoryAnalysis,
} from 'containerization-assist-mcp/sdk';

export class AnalyzeRepoTool
  implements vscode.LanguageModelTool<AnalyzeRepoInput>
{
  /**
   * Execute the tool.
   */
  async invoke(
    options: vscode.LanguageModelToolInvocationOptions<AnalyzeRepoInput>,
    token: vscode.CancellationToken
  ): Promise<vscode.LanguageModelToolResult> {
    // 1. Convert cancellation token
    const { signal, dispose } = createAbortSignalFromToken(token);

    try {
      // 2. Call SDK function
      const result = await analyzeRepo(options.input, { signal });

      // 3. Handle failure
      if (!result.ok) {
        throw new Error(formatErrorForLLM(result.error, result.guidance));
      }

      // 4. Format successful result
      const formatted = resultFormatters.analyzeRepo(result.value);

      return new vscode.LanguageModelToolResult([
        new vscode.LanguageModelTextPart(formatted),
      ]);
    } finally {
      dispose();
    }
  }

  /**
   * Prepare confirmation dialog.
   */
  async prepareInvocation(
    options: vscode.LanguageModelToolInvocationPrepareOptions<AnalyzeRepoInput>,
    _token: vscode.CancellationToken
  ): Promise<vscode.PreparedToolInvocation> {
    const meta = toolMetadata.analyzeRepo;
    const { repositoryPath } = options.input;

    // Substitute template variables
    const message = meta.confirmation.messageTemplate
      .replace('{{repositoryPath}}', repositoryPath);

    return {
      invocationMessage: `Analyzing ${repositoryPath}`,
      confirmationMessages: {
        title: meta.confirmation.title,
        message: new vscode.MarkdownString(
          message + (meta.confirmation.isReadOnly
            ? '\n\n*This is a read-only operation.*'
            : '')
        ),
      },
    };
  }
}
```

---

## Extension Activation

Register all tools in your extension's `activate` function:

```typescript
// src/extension.ts
import * as vscode from 'vscode';
import { AnalyzeRepoTool } from './tools/analyze-repo';
import { BuildImageTool } from './tools/build-image';
import { GenerateDockerfileTool } from './tools/generate-dockerfile';
import { FixDockerfileTool } from './tools/fix-dockerfile';
import { ScanImageTool } from './tools/scan-image';
import { TagImageTool } from './tools/tag-image';
import { PushImageTool } from './tools/push-image';
import { GenerateK8sManifestsTool } from './tools/generate-k8s-manifests';
import { PrepareClusterTool } from './tools/prepare-cluster';
import { VerifyDeployTool } from './tools/verify-deploy';
import { OpsTool } from './tools/ops';

export function activate(context: vscode.ExtensionContext) {
  // Register all containerization tools
  const tools = [
    ['analyze_repo', new AnalyzeRepoTool()],
    ['build_image', new BuildImageTool()],
    ['generate_dockerfile', new GenerateDockerfileTool()],
    ['fix_dockerfile', new FixDockerfileTool()],
    ['scan_image', new ScanImageTool()],
    ['tag_image', new TagImageTool()],
    ['push_image', new PushImageTool()],
    ['generate_k8s_manifests', new GenerateK8sManifestsTool()],
    ['prepare_cluster', new PrepareClusterTool()],
    ['verify_deploy', new VerifyDeployTool()],
    ['ops', new OpsTool()],
  ] as const;

  for (const [name, tool] of tools) {
    context.subscriptions.push(vscode.lm.registerTool(name, tool));
  }

  console.log('Containerization tools registered');
}

export function deactivate() {}
```

---

## Complete Tool Examples

### Build Image Tool (Mutating Operation)

```typescript
import * as vscode from 'vscode';
import {
  buildImage,
  toolMetadata,
  resultFormatters,
  createAbortSignalFromToken,
  formatErrorForLLM,
  resolveWorkspacePath,
  type BuildImageInput,
} from 'containerization-assist-mcp/sdk';

export class BuildImageTool
  implements vscode.LanguageModelTool<BuildImageInput>
{
  async invoke(
    options: vscode.LanguageModelToolInvocationOptions<BuildImageInput>,
    token: vscode.CancellationToken
  ): Promise<vscode.LanguageModelToolResult> {
    const { signal, dispose } = createAbortSignalFromToken(token);

    try {
      // Resolve relative paths
      const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
      const input = { ...options.input };

      if (workspaceRoot && input.path) {
        input.path = resolveWorkspacePath(input.path, workspaceRoot);
      }

      // Show progress (VS Code doesn't have built-in progress for tools)
      const result = await buildImage(input, {
        signal,
        onProgress: (message, progress, total) => {
          // Could show in output channel if needed
          console.log(`Build: ${message} (${progress}/${total})`);
        },
      });

      if (!result.ok) {
        throw new Error(formatErrorForLLM(result.error, result.guidance));
      }

      return new vscode.LanguageModelToolResult([
        new vscode.LanguageModelTextPart(
          resultFormatters.buildImage(result.value)
        ),
      ]);
    } finally {
      dispose();
    }
  }

  async prepareInvocation(
    options: vscode.LanguageModelToolInvocationPrepareOptions<BuildImageInput>,
    _token: vscode.CancellationToken
  ): Promise<vscode.PreparedToolInvocation> {
    const meta = toolMetadata.buildImage;
    const { path, imageName } = options.input;

    return {
      invocationMessage: `Building image ${imageName || 'unnamed'}`,
      confirmationMessages: {
        title: meta.confirmation.title,
        message: new vscode.MarkdownString(
          `Build Docker image:\n\n` +
          `**Context**: \`${path || '.'}\`\n` +
          `**Image**: \`${imageName || 'auto'}\`\n\n` +
          `${meta.confirmation.warning}`
        ),
      },
    };
  }
}
```

### Scan Image Tool (With Vulnerability Summary)

```typescript
import * as vscode from 'vscode';
import {
  scanImage,
  toolMetadata,
  resultFormatters,
  createAbortSignalFromToken,
  formatErrorForLLM,
  type ScanImageInput,
} from 'containerization-assist-mcp/sdk';

export class ScanImageTool
  implements vscode.LanguageModelTool<ScanImageInput>
{
  async invoke(
    options: vscode.LanguageModelToolInvocationOptions<ScanImageInput>,
    token: vscode.CancellationToken
  ): Promise<vscode.LanguageModelToolResult> {
    const { signal, dispose } = createAbortSignalFromToken(token);

    try {
      const result = await scanImage(options.input, { signal });

      if (!result.ok) {
        throw new Error(formatErrorForLLM(result.error, result.guidance));
      }

      // Use formatter with custom options
      const formatted = resultFormatters.scanImage(result.value, {
        includeSuggestedNext: true,
        maxFieldLength: 2000, // Allow longer vuln lists
      });

      return new vscode.LanguageModelToolResult([
        new vscode.LanguageModelTextPart(formatted),
      ]);
    } finally {
      dispose();
    }
  }

  async prepareInvocation(
    options: vscode.LanguageModelToolInvocationPrepareOptions<ScanImageInput>,
    _token: vscode.CancellationToken
  ): Promise<vscode.PreparedToolInvocation> {
    const meta = toolMetadata.scanImage;

    return {
      invocationMessage: `Scanning ${options.input.imageId} for vulnerabilities`,
      confirmationMessages: {
        title: meta.confirmation.title,
        message: new vscode.MarkdownString(
          `Scan image for security vulnerabilities:\n\n` +
          `\`${options.input.imageId}\`\n\n` +
          `*This is a read-only operation.*`
        ),
      },
    };
  }
}
```

---

## Error Handling

### Standard Error Pattern

```typescript
if (!result.ok) {
  // formatErrorForLLM creates a message the LLM can use to help the user
  throw new Error(formatErrorForLLM(result.error, result.guidance));
}
```

### Custom Error Handling

```typescript
if (!result.ok) {
  // Check for specific error types
  if (result.error.includes('Docker daemon')) {
    throw new Error(
      'Docker is not running. Please start Docker Desktop and try again.'
    );
  }

  if (result.error.includes('not found')) {
    throw new Error(
      `Resource not found: ${result.error}. ` +
      `Make sure the path exists and is accessible.`
    );
  }

  // Default
  throw new Error(formatErrorForLLM(result.error, result.guidance));
}
```

---

## Testing

### Unit Testing Tools

```typescript
import { describe, it, expect, vi } from 'vitest';
import { AnalyzeRepoTool } from './analyze-repo';

// Mock vscode
vi.mock('vscode', () => ({
  LanguageModelToolResult: class {
    constructor(public parts: any[]) {}
  },
  LanguageModelTextPart: class {
    constructor(public text: string) {}
  },
  MarkdownString: class {
    constructor(public value: string) {}
  },
}));

describe('AnalyzeRepoTool', () => {
  it('returns formatted result on success', async () => {
    const tool = new AnalyzeRepoTool();

    const mockToken = {
      isCancellationRequested: false,
      onCancellationRequested: () => ({ dispose: () => {} }),
    };

    const result = await tool.invoke(
      { input: { repositoryPath: process.cwd() } } as any,
      mockToken as any
    );

    expect(result.parts).toHaveLength(1);
    expect(result.parts[0].text).toContain('Analysis');
  });
});
```

---

## API Reference

### SDK Functions

| Function | Description |
|----------|-------------|
| `analyzeRepo(input, options?)` | Analyze repository structure |
| `generateDockerfile(input, options?)` | Generate Dockerfile plan |
| `fixDockerfile(input, options?)` | Fix existing Dockerfile |
| `buildImage(input, options?)` | Build Docker image |
| `scanImage(input, options?)` | Scan for vulnerabilities |
| `tagImage(input, options?)` | Tag Docker image |
| `pushImage(input, options?)` | Push to registry |
| `generateK8sManifests(input, options?)` | Generate K8s manifests |
| `prepareCluster(input, options?)` | Prepare namespace |
| `verifyDeploy(input, options?)` | Verify deployment |
| `ops(input, options?)` | Operational utilities |

### JSON Schemas

```typescript
import { jsonSchemas } from 'containerization-assist-mcp/sdk';

// Access individual schemas
jsonSchemas.analyzeRepo      // JSON Schema for analyze-repo input
jsonSchemas.buildImage       // JSON Schema for build-image input
// ... etc
```

### Tool Metadata

```typescript
import { toolMetadata, type ToolMetadata } from 'containerization-assist-mcp/sdk';

// Access metadata
const meta: ToolMetadata = toolMetadata.analyzeRepo;

meta.name                  // 'analyze_repo'
meta.displayName           // 'Analyze Repository'
meta.modelDescription      // Detailed LLM-facing description
meta.userDescription       // Brief user-facing description
meta.toolReferenceName     // 'containerization-analyze'
meta.icon                  // '$(search)'
meta.canBeReferencedInPrompt // true
meta.confirmation          // { title, messageTemplate, isReadOnly, warning? }
meta.suggestedNextTools    // ['generate_dockerfile']
meta.category              // 'analysis'
meta.requiresExternalDeps  // []
```

### Result Formatters

```typescript
import { resultFormatters, type FormatterOptions } from 'containerization-assist-mcp/sdk';

const options: FormatterOptions = {
  includeSuggestedNext: true,  // Include "Suggested Next Step" section
  maxFieldLength: 1000,        // Truncate long fields
  asJson: false,               // Output as prose (true = JSON)
};

// Format results
const text = resultFormatters.analyzeRepo(result.value, options);
```

### VS Code Utilities

```typescript
import {
  createAbortSignalFromToken,
  formatErrorForLLM,
  resolveWorkspacePath,
} from 'containerization-assist-mcp/sdk';

// Convert CancellationToken to AbortSignal
const { signal, dispose } = createAbortSignalFromToken(token);

// Format error for LLM
const message = formatErrorForLLM(error, { hint, resolution });

// Resolve workspace-relative path
const absPath = resolveWorkspacePath('./src', '/workspace/myapp');
```

---

## Workflow Guidance

The SDK exports a standard workflow order:

```typescript
import { standardWorkflow, toolMetadata } from 'containerization-assist-mcp/sdk';

// standardWorkflow = [
//   'analyzeRepo',
//   'generateDockerfile',
//   'buildImage',
//   'scanImage',
//   'tagImage',
//   'pushImage',
//   'generateK8sManifests',
//   'prepareCluster',
//   'verifyDeploy',
// ]

// Each tool's suggestedNextTools points to the next step
toolMetadata.analyzeRepo.suggestedNextTools // ['generate_dockerfile']
toolMetadata.buildImage.suggestedNextTools  // ['scan_image', 'tag_image']
```

---

## Troubleshooting

### Common Issues

1. **"Docker daemon not running"**
   - Start Docker Desktop
   - Check `docker ps` works in terminal

2. **"Trivy not found"**
   - Install Trivy: `brew install trivy` or equivalent
   - Scan will return limited results without it

3. **"kubectl not configured"**
   - Run `kubectl cluster-info` to verify access
   - Check kubeconfig is set correctly

4. **"Input validation failed"**
   - Check required fields in `jsonSchemas`
   - Verify types match schema definitions

### Debug Logging

Enable debug logging by setting environment variables:

```typescript
// In your extension's activate function
process.env.LOG_LEVEL = 'debug';
```

### Cancellation Handling

Ensure proper cleanup when operations are cancelled:

```typescript
const { signal, dispose } = createAbortSignalFromToken(token);

try {
  const result = await buildImage(input, { signal });
  // ...
} catch (error) {
  if (signal.aborted) {
    // User cancelled the operation
    throw new Error('Operation cancelled by user');
  }
  throw error;
} finally {
  // Always dispose to clean up event listeners
  dispose();
}
```
