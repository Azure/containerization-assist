# Development Setup Guide

This guide covers setting up the Containerization Assist MCP Server for development and contribution.

## Prerequisites for Development

- **Node.js** 20 or higher
- **Docker** 20.10 or higher  
- **kubectl** (optional, for Kubernetes deployments)
- **Git**

## Clone and Setup

```bash
git clone https://github.com/azure/containerization-assistt
cd containerization-assist
npm install
npm run build
```

## Development Workflow

### Local Development with Hot Reload

For development with hot reload, the project includes `.vscode/mcp.json`:

```json
{
  "servers": {
    "containerization-assist-dev": {
      "command": "npx",
      "args": ["tsx", "watch", "./src/cli/cli.ts"],
      "env": {
        "MCP_MODE": "true",
        "MCP_QUIET": "true",
        "NODE_ENV": "development"
      }
    }
  }
}
```

Simply restart VS Code to enable the development MCP server.

## Development Commands

### Build & Development
```bash
npm run build          # Full build (ESM + CJS)
npm run build:fast     # Fast development build
npm run build:watch    # Watch mode with auto-rebuild
npm run dev            # Development server with auto-reload
npm start              # Start production server
npm run clean          # Clean dist directory
```

### Code Quality
```bash
npm run lint           # ESLint code linting
npm run lint:fix       # Auto-fix ESLint issues
npm run typecheck      # TypeScript type checking
npm run format         # Prettier code formatting
npm run format:check   # Check formatting without changes
npm run validate       # Run lint + typecheck + test
npm run validate:pr    # Full PR validation with integration tests
npm run fix            # Auto-fix lint + format issues
```

### Quality Gates & Analysis
```bash
npm run quality:check  # Code quality metrics analysis
npm run quality:gates  # Quality gate validation
npm run deadcode      # Dead code analysis
```

### Testing
```bash
npm test                   # Run all tests
npm run test:unit          # Unit tests only
npm run test:integration   # Integration tests via MCP Inspector
npm run test:coverage      # Generate coverage report
npm run test:mcp           # MCP server integration tests
npm run validate:pr:fast   # Complete validation pipeline
```

### MCP Development Commands
```bash
npm run mcp:start      # Start MCP server
npm run mcp:inspect    # Start MCP inspector for testing
```

## Code Standards

- **Build System**: Ultra-fast tsdown (esbuild-based) - 10-100x faster than tsc
- **TypeScript**: Strict mode with ES2022 modules and native ESM support
- **Imports**: Path aliases supported (@app, @mcp, @tools, etc.) for clean imports
- **Architecture**: Clean layered separation with strict boundaries
- **Error Handling**: Result<T> monad pattern throughout
- **Quality Gates**: Automated lint ratcheting prevents regression
- **Testing**: Comprehensive unit and integration tests

### Import Rules
When writing or modifying code, you can use either approach:

```typescript
// ✅ OPTION 1 - Path aliases (Clean and preferred)
import { Config } from '@config/types'
import { Logger } from '@lib/logger'
import type { Result } from '@types'

// ✅ OPTION 2 - Relative imports (Also acceptable)
import { Config } from '../config/types'
import { Logger } from '../../lib/logger'
```

**Path Alias Mappings:**
- `@app/*` → `src/app/*`
- `@mcp/*` → `src/mcp/*`
- `@tools/*` → `src/tools/*`
- `@lib/*` → `src/lib/*`
- `@domain/*` → `src/domain/*`
- `@infrastructure/*` → `src/infrastructure/*`
- `@config/*` → `src/config/*`
- `@prompts/*` → `src/prompts/*`
- `@resources/*` → `src/resources/*`
- `@workflows/*` → `src/workflows/*`
- `@types` → `src/domain/types`

### Error Handling Pattern
All functions that can fail MUST return Result<T>:

```typescript
import { Result, Success, Failure } from '@domain/types'

// ✅ CORRECT - Always use Result<T>
export async function buildImage(config: BuildConfig): Promise<Result<BuildOutput>> {
  try {
    const output = await docker.build(config);
    return Success(output);
  } catch (error) {
    return Failure(`Build failed: ${error.message}`);
  }
}

// ❌ WRONG - Never throw or return naked types
export async function buildImage(config: BuildConfig): Promise<BuildOutput> {
  return await docker.build(config); // Could throw!
}
```

## Project Health Metrics

- **ESLint Warnings**: 700 (baseline enforced, 46% reduction from initial)
- **ESLint Errors**: 9 (must be fixed before PR)
- **TypeScript Errors**: 45 (work in progress)
- **Dead Code**: 234 unused exports (47% reduction)
- **Build Time**: < 1 second
- **Test Coverage**: > 70%

## File Organization

```
src/
├── cli/             # CLI entry points
├── config/          # Configuration management
├── core/            # Core utilities and types (Result, etc.)
├── lib/             # Libraries and adapters
├── mcp/             # MCP server implementation
├── tools/           # Tool implementations (co-located pattern)
│   ├── analyze-repo/
│   │   ├── tool.ts    # Tool implementation
│   │   ├── schema.ts  # Zod schema definition
│   │   └── index.ts   # Public exports
│   └── [tool-name]/   # Same structure for each tool
└── workflows/       # Workflow orchestration
```

### Tool Co-location Pattern
Tools are organized with co-located schemas for better modularity:
- Each tool has its own folder under `src/tools/`
- Schema definitions are co-located with implementations
- `index.ts` provides clean exports for both tool and schema
- Tools can be deleted by simply removing their folder

## Development Workflow Best Practices

### Development Workflow
```bash
# Start development with watch mode
npm run dev

# In another terminal, run tests in watch mode
npm run test:watch

# Before committing
npm run validate:pr:fast
```

### Before Making Changes
1. **Check TypeScript compilation**: `npm run typecheck`
2. **Run linting**: `npm run lint`
3. **Ensure tests pass**: `npm test`

### After Making Changes
1. **Run quick validation**: `npm run validate`
2. **Fix any issues**: `npm run fix`
3. **Full PR validation**: `npm run validate:pr`

## Testing

### Test Structure

```text
test/
├── __tests__/
│   ├── unit/        # Unit tests
│   ├── integration/ # Integration tests
│   └── e2e/        # End-to-end tests
├── __fixtures__/    # Test fixtures
└── __mocks__/      # Mock implementations
```

### Writing Tests

```typescript
import { describe, it, expect, jest } from '@jest/globals';
import { analyzeRepository } from '../tools/analyze-repository';

describe('analyzeRepository', () => {
  it('should detect Node.js projects', async () => {
    const result = await analyzeRepository({
      repoPath: './test-fixtures/node-app',
      sessionId: 'test-session'
    });
    
    expect(result.isSuccess()).toBe(true);
    expect(result.value.language).toBe('javascript');
    expect(result.value.framework).toBe('express');
  });

  it('should handle missing repositories', async () => {
    const result = await analyzeRepository({
      repoPath: './non-existent',
      sessionId: 'test-session'
    });
    
    expect(result.isFailure()).toBe(true);
    expect(result.error).toContain('does not exist');
  });
});
```

## Debugging

### VS Code Configuration

The project includes `.vscode/launch.json` for debugging:

```json
{
  "configurations": [
    {
      "type": "node",
      "request": "launch",
      "name": "Debug MCP Server",
      "program": "${workspaceFolder}/src/cli/cli.ts",
      "runtimeArgs": ["-r", "tsx/cjs"],
      "env": {
        "MCP_MODE": "true",
        "LOG_LEVEL": "debug"
      }
    }
  ]
}
```

### Debug Logging

```bash
# Enable debug logging
LOG_LEVEL=debug npm run dev

# Debug specific module
DEBUG=mcp:* npm run dev
```

### Script Management Guidelines

#### Script Philosophy
- **Minimize permanent scripts**: Only create scripts that will be used repeatedly
- **Prefer inline commands**: Use simple npm script one-liners over separate shell files
- **Consolidate similar tasks**: Avoid script proliferation and variations
- **Delete migration scripts**: Remove one-time setup/migration scripts after use

#### Approved Script Categories
1. **Core Build/Test**: `build`, `test`, `lint`, `typecheck`
2. **Quality Gates**: `quality` (consolidated validation)
3. **MCP Server**: `mcp:start`, `mcp:inspect`
4. **Release**: `validate`, `prepublishOnly`, `release`

## Testing with MCP Inspector

For development testing, use the MCP Inspector:

```bash
# Using local development build
npx @modelcontextprotocol/inspector npx tsx src/cli/cli.ts

# Using built version
npm run build
npx @modelcontextprotocol/inspector node dist/cli.js
```

## Debugging

### Enable Debug Logging

```bash
# Set environment variable
export LOG_LEVEL=debug

# Or inline
LOG_LEVEL=debug npm run mcp:start
```

### Common Development Issues

#### TypeScript Compilation Errors
```bash
# Clean and rebuild
npm run clean
npm run build

# Check specific errors
npm run typecheck
```

#### ESLint Issues
```bash
# Auto-fix what's possible
npm run lint:fix

# Check baseline
npm run baseline:report
```

#### Test Failures
```bash
# Run specific test suite
npm test -- --testNamePattern="Docker"

# Update snapshots if needed
npm test -- -u
```

## Contributing Guidelines

### Process
1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes following coding standards
4. Add tests for new functionality
5. Run validation: `npm run validate:pr:fast`
6. Commit with descriptive message
7. Push and create pull request

### Commit Message Format

```text
type(scope): description

- Detail 1
- Detail 2

Fixes #123
```

Types: feat, fix, docs, style, refactor, test, chore

## Troubleshooting

### Common Issues

**TypeScript errors after adding new files:**
```bash
npm run clean && npm run build
```

**ESLint not recognizing new rules:**
```bash
npm run lint:fix
npm run baseline:lint  # If improvements made
```

**Tests failing with module resolution:**
```bash
# Check jest.config.js moduleNameMapper
# Ensure using relative imports, not aliases
```

**Docker connection issues in tests:**
```bash
# Ensure Docker is running
docker ps

# Check mock mode
USE_MOCK_DOCKER=true npm test
```

## Related Documentation

- **[Main README](../README.md)** - User-focused documentation
- **[Getting Started](./getting-started.md)** - Installation and first use
- **[Architecture Guide](./architecture.md)** - System design and components
- **[Internal Documentation](./internal/)** - Technical documentation for maintainers
- **[Claude Code Guidelines](../CLAUDE.md)** - Guidelines for Claude Code