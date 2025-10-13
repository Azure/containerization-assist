# Contributing to Containerization Assist

Thank you for your interest in contributing to Containerization Assist! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)
- [Architecture Guidelines](#architecture-guidelines)

## Code of Conduct

This project adheres to the Microsoft Open Source Code of Conduct. By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Node.js 20 or later
- npm (comes with Node.js)
- Docker
- kubectl (for Kubernetes features)
- Git

### Development Setup

1. **Fork and Clone**
   ```bash
   git clone https://github.com/YOUR-USERNAME/containerization-assist.git
   cd containerization-assist
   ```

2. **Install Dependencies**
   ```bash
   npm install
   ```

3. **Build the Project**
   ```bash
   # Full build (ESM + CJS)
   npm run build

   # ESM only
   npm run build:esm

   # CJS only
   npm run build:cjs

   # Watch mode for development
   npm run build:watch
   ```

4. **Run Tests**
   ```bash
   # All tests
   npm test

   # Unit tests only
   npm run test:unit

   # With coverage
   npm run test:coverage
   ```

5. **Verify Installation**
   ```bash
   # Run validation (lint + typecheck + tests)
   npm run validate
   ```

## Project Structure

```
containerization-assist/
├── src/                   # TypeScript source code
│   ├── app/              # Application core and orchestrator
│   ├── cli/              # CLI entry points
│   ├── tools/            # Tool implementations
│   ├── mcp/              # MCP server implementation
│   ├── ai/               # Prompt engine and AI integration
│   ├── session/          # Session management
│   ├── infra/            # Infrastructure clients (Docker, K8s)
│   ├── lib/              # Shared utilities
│   ├── config/           # Configuration and policy system
│   ├── validation/       # Validation and fixing logic
│   ├── knowledge/        # Knowledge pack system
│   └── types/            # Type definitions
├── docs/                 # Documentation
├── test/                 # Test files
├── scripts/              # Build and utility scripts
└── knowledge/            # Knowledge packs (JSON)
```

### Key Components

- **MCP Server** (`src/mcp/`) - MCP protocol implementation
- **Tools** (`src/tools/`) - Containerization tools with AI enhancement
- **AI System** (`src/ai/`) - Prompt engine and knowledge enhancement
- **Orchestrator** (`src/app/orchestrator.ts`) - Tool execution coordination
- **Session Manager** (`src/session/`) - Single-session state management

## Making Changes

### Before You Start

1. **Check Existing Issues**: Look for existing issues or discussions
2. **Create an Issue**: For significant changes, create an issue first
3. **Assign Yourself**: Assign the issue to yourself to avoid duplicated work

### Development Workflow

1. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**
   - Follow the coding standards below
   - Add tests for new functionality
   - Update documentation as needed

3. **Validate Your Changes**
   ```bash
   # Run validation (lint + typecheck + tests)
   npm run validate

   # Auto-fix linting and formatting issues
   npm run fix

   # Run tests only
   npm test

   # Build the project
   npm run build
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "feat: add new tool for X"
   ```

### Commit Message Guidelines

Use conventional commits format:
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `test:` - Test additions/changes
- `refactor:` - Code refactoring
- `style:` - Code style changes
- `chore:` - Maintenance tasks

Examples:
```
feat: add scan tool with AI suggestions
fix: resolve session persistence issue
docs: update MCP setup instructions
test: add unit tests for prompt engine
```

## Testing

### Test Categories

1. **Unit Tests** - Test individual functions and methods
2. **Integration Tests** - Test component interactions
3. **End-to-End Tests** - Test complete workflows

### Writing Tests

```typescript
import { describe, it, expect } from '@jest/globals';

describe('MyTool', () => {
  it('should process valid input', async () => {
    const result = await myTool.run({ input: 'test' }, ctx);
    
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value).toBeDefined();
    }
  });

  it('should handle errors gracefully', async () => {
    const result = await myTool.run({ input: '' }, ctx);
    
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toContain('Invalid input');
    }
  });
});
```

### Test Requirements

- All new functionality must include tests
- Maintain >70% test coverage
- Test error conditions
- Mock external dependencies (Docker, Kubernetes)
- Use Jest with ES module support

## Submitting Changes

### Pull Request Process

1. **Push Your Branch**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create Pull Request**
   - Use the PR template
   - Link related issues
   - Provide clear description of changes
   - Include screenshots for UI changes (if applicable)

3. **PR Requirements**
   - All tests must pass
   - Code must pass linting (`npm run lint`)
   - Type checking must pass (`npm run typecheck`)
   - Documentation updated
   - Reviewed by maintainer

### PR Template

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added for new functionality
```

## Code Style

### TypeScript Guidelines

- Follow standard TypeScript conventions
- Use descriptive variable names
- Add JSDoc comments for exported functions
- Handle errors using `Result<T>` pattern
- Use strict TypeScript mode
- Avoid `any` type - use proper types or `unknown`
- Keep functions focused and under 50 lines when possible
- Use path aliases (`@/`) for imports

### Code Quality

All code must:
- Pass ESLint checks (`npm run lint`)
- Pass TypeScript type checking (`npm run typecheck`)
- Follow Prettier formatting (`npm run format`)
- Include appropriate tests
- Use the Result<T> pattern for error handling

Example:
```typescript
import type { Result } from '@/types';

async function myFunction(input: string): Promise<Result<Output>> {
  if (!input) {
    return { ok: false, error: 'Input is required' };
  }

  try {
    const output = await processInput(input);
    return { ok: true, value: output };
  } catch (err) {
    return { ok: false, error: `Processing failed: ${err}` };
  }
}
```

### Pre-commit Hooks

We use Husky and lint-staged for pre-commit hooks:

```bash
# Automatically installed with npm install
# Runs ESLint and Prettier on staged files
```

### Documentation

- Add JSDoc comments for exported functions and classes
- Update README files for new features
- Include examples in documentation
- Keep documentation current with code changes

## Architecture Guidelines

### Tool Development

1. **Tool Structure**
   - Each tool in `src/tools/[tool-name]/`
   - Required files: `tool.ts`, `schema.ts`, `index.ts`
   - Use unified Tool interface
   - Include metadata for AI enhancement

2. **Tool Template**
   ```typescript
   import type { Tool } from '@/types';
   import { z } from 'zod';

   const myToolSchema = z.object({
     // Define parameters
   });

   const tool: Tool<typeof myToolSchema, ResultType> = {
     name: 'my-tool',
     description: 'Description of what this tool does',
     version: '1.0.0',
     schema: myToolSchema,
     metadata: {
       knowledgeEnhanced: false,
       samplingStrategy: 'none',
       enhancementCapabilities: [],
     },
     run: async (input, ctx) => {
       // Implementation
     },
   };

   export default tool;
   ```

3. **AI Integration**
   - Set `samplingStrategy: 'single'` for AI-driven tools
   - Set `knowledgeEnhanced: true` for tools using knowledge packs
   - Use `ctx.ai.sampleWithRerank()` for AI generation
   - Add to `enhancementCapabilities` array

4. **Session Management**
   - Access prior results via `ctx.session.getResult('tool-name')`
   - Session state automatically persisted by orchestrator
   - Use session for workflow continuity

### Error Handling

All functions that can fail should return `Result<T>`:

```typescript
type Result<T> = 
  | { ok: true; value: T }
  | { ok: false; error: string };

// Usage
const result = await myFunction();
if (result.ok) {
  console.log(result.value);
} else {
  console.error(result.error);
}
```

## Getting Help

- **GitHub Issues**: For bugs and feature requests
- **Discussions**: For questions and general discussion
- **Documentation**: Check existing docs first
- **Code Review**: Ask for feedback on complex changes

## Recognition

Contributors are recognized in:
- GitHub contributors list
- Release notes for significant contributions
- Documentation acknowledgments

Thank you for contributing to Containerization Assist!

# Contributor License Agreement

This project welcomes contributions and suggestions. Most contributions require you to
agree to a Contributor License Agreement (CLA) declaring that you have the right to,
and actually do, grant us the rights to use your contribution. For details, visit
https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need
to provide a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the
instructions provided by the bot. You will only need to do this once across all repositories using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/)
or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
