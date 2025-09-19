# Contributing to Containerization Assist

Thank you for your interest in contributing to Containerization Assist! This document provides guidelines for contributors to our AI-First TypeScript MCP server.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [AI-First Architecture](#ai-first-architecture)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)

## Code of Conduct

This project adheres to the Microsoft Open Source Code of Conduct. By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Node.js 20+
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
   npm run build
   ```

4. **Run Tests**
   ```bash
   npm test
   ```

5. **Start Development**
   ```bash
   npm run dev  # Development server with watch mode
   ```

## AI-First Architecture

This project implements an **AI-First Architecture** with specific patterns that all contributors should understand:

### Core Principles

1. **Business Logic in Prompts**: Decision-making logic lives in YAML prompts, not TypeScript
2. **TypeScript for Deterministic Operations**: Use TypeScript only for Docker calls, file I/O, and K8s operations
3. **Explicit Dependencies**: No global dependency injection; pass dependencies explicitly
4. **Unified Tool Pattern**: All tools follow the same execution pattern
5. **Session Management**: Context persists across tool executions

### Tool Development Pattern

All tools must follow this pattern:

```typescript
export const tool = {
  name: 'tool_name',
  description: 'Tool description',
  inputSchema: zodSchema,
  execute: async (params, deps, context) => {
    // 1. Use AI for decision-making via prompts
    const aiResult = await promptBackedTool.execute(params, deps, context);

    // 2. Perform deterministic side effects only
    await performDockerOperation(aiResult.value);

    // 3. Update session context
    await context.sessionManager.update(params.sessionId, result);

    return Success(result);
  }
};
```

### What Goes Where

**In TypeScript:**
- ✅ Docker client operations
- ✅ File system operations
- ✅ Kubernetes API calls
- ✅ Session management
- ✅ Result formatting

**In YAML Prompts:**
- 🤖 Analysis and decision-making
- 🤖 Content generation (Dockerfiles, manifests)
- 🤖 Heuristics and scoring
- 🤖 Best practice recommendations
- 🤖 Error analysis and suggestions

## Project Structure

```
src/
├── cli/                    # CLI entry points
├── mcp/                    # MCP server implementation
│   ├── server/             # MCP server core
│   ├── tools/              # Prompt-backed tool factory
│   └── ai/                 # AI integration utilities
├── tools/                  # Individual tools (co-located)
│   ├── analyze-repo/       # Tool implementation
│   │   ├── tool.ts         # Main tool logic
│   │   ├── schema.ts       # Input/output schemas
│   │   └── index.ts        # Public exports
│   └── ...                 # Other tools
├── lib/                    # Shared utilities
├── prompts/                # YAML prompt templates
├── knowledge/              # Knowledge base content
├── config/                 # Configuration management
└── types/                  # TypeScript type definitions

test/
├── unit/                   # Unit tests
├── integration/            # Integration tests
└── __support__/            # Test utilities and mocks
```

## Making Changes

### Before You Start

1. **Check Existing Issues**: Look for related issues or discussions
2. **Create an Issue**: For significant changes, create an issue first
3. **Understand the AI-First Pattern**: Review existing tools to understand the pattern

### Development Workflow

1. **Create a Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes Following Patterns**
   - Use the unified tool pattern for new tools
   - Put business logic in YAML prompts
   - Keep TypeScript code deterministic
   - Add comprehensive tests

3. **Validate Your Changes**
   ```bash
   npm run lint       # Check code style
   npm run typecheck  # Verify TypeScript
   npm test           # Run all tests
   npm run build      # Ensure it builds
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "feat: add new tool for X"
   ```

### Adding New Tools

1. **Create Tool Directory**
   ```bash
   mkdir src/tools/my-new-tool
   ```

2. **Implement Tool Pattern**
   ```typescript
   // src/tools/my-new-tool/tool.ts
   import { createPromptBackedTool } from '@mcp/tools/prompt-backed-tool';

   const myToolAI = createPromptBackedTool({
     name: 'my-new-tool',
     description: 'Tool description',
     inputSchema: MyToolSchema,
     outputSchema: MyToolResultSchema,
     promptId: 'my-tool-prompt',
     knowledge: {
       category: 'my-domain',
       topK: 4
     }
   });

   export const tool = {
     name: 'my_new_tool',
     description: 'Tool description',
     inputSchema: MyToolSchema,
     execute: async (params, deps, context) => {
       // AI decision-making
       const aiResult = await myToolAI.execute(params, deps, context);

       // Deterministic operations
       if (aiResult.ok) {
         await performSideEffects(aiResult.value, deps);
       }

       return aiResult;
     }
   };
   ```

3. **Create Prompt Template**
   ```yaml
   # src/prompts/my-domain/my-tool-prompt.yaml
   id: my-tool-prompt
   version: 1.0.0
   prompt: |
     Analyze the following input and provide recommendations:

     Input: {{input}}

     Provide your analysis in the following JSON format:
     {
       "analysis": "your analysis",
       "recommendations": ["rec1", "rec2"]
     }
   ```

4. **Add Tests**
   ```typescript
   // src/tools/my-new-tool/tool.test.ts
   import { tool } from './tool';
   import { mockLogger } from '../../test/__support__/mocks/mock-factories';

   describe('My New Tool', () => {
     it('should execute successfully', async () => {
       const deps = { logger: mockLogger() };
       const params = { /* test params */ };
       const context = { /* mock context */ };

       const result = await tool.execute(params, deps, context);

       expect(result.ok).toBe(true);
     });
   });
   ```

### Modifying Existing Tools

1. **Understand Current Implementation**: Read the tool's code and tests
2. **Check for Prompt Usage**: See if the tool uses `createPromptBackedTool`
3. **Update Prompts Not Code**: For business logic changes, modify YAML prompts
4. **Test Thoroughly**: Ensure changes don't break existing functionality

## Testing

### Test Requirements

- All new tools must have tests
- Maintain >80% test coverage
- Test both success and error cases
- Mock external dependencies
- Use the standard test patterns

### Test Patterns

```typescript
import { tool } from '@tools/my-tool/tool';
import { mockLogger, mockDocker } from '../__support__/mocks/mock-factories';

describe('My Tool', () => {
  let deps: ToolDeps;
  let context: ToolContext;

  beforeEach(() => {
    deps = {
      logger: mockLogger(),
      docker: mockDocker(),
    };
    context = {
      sessionManager: mockSessionManager(),
    };
  });

  it('should handle valid input', async () => {
    const params = { validParam: 'value' };

    const result = await tool.execute(params, deps, context);

    expect(result.ok).toBe(true);
    expect(result.value).toMatchObject({
      expectedField: expect.any(String)
    });
  });

  it('should handle errors gracefully', async () => {
    deps.docker.buildImage.mockRejectedValue(new Error('Docker error'));

    const result = await tool.execute(params, deps, context);

    expect(result.ok).toBe(false);
    expect(result.error).toContain('Docker error');
  });
});
```

### Running Tests

```bash
npm test                    # All tests
npm run test:unit          # Unit tests only
npm run test:integration   # Integration tests
npm run test:watch         # Watch mode
npm run test:coverage      # With coverage report
```

## Submitting Changes

### Pull Request Process

1. **Push Your Branch**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create Pull Request**
   - Use descriptive title and description
   - Link related issues
   - Include testing information
   - Add screenshots for UI changes

3. **PR Requirements**
   - All tests pass
   - Code follows style guidelines
   - No TypeScript errors
   - Documentation updated
   - Reviewed by maintainer

### Commit Message Guidelines

Use conventional commits:
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation
- `test:` - Tests
- `refactor:` - Code refactoring
- `style:` - Code style
- `chore:` - Maintenance

Examples:
```
feat: add Docker image scanning tool
fix: resolve session persistence issue
docs: update AI-First architecture guide
test: add unit tests for prompt-backed tools
```

## Code Style

### TypeScript Guidelines

- Use TypeScript strict mode
- Prefer explicit types over `any`
- Use path aliases (`@/`, `@tools/`, etc.)
- Follow import order conventions
- Add JSDoc comments for exported functions

### AI-First Guidelines

- **Keep TypeScript Simple**: Avoid complex business logic
- **Use Explicit Dependencies**: No global state or DI containers
- **Prompt-Driven Decisions**: Move heuristics to YAML prompts
- **Session Context**: Always use session for stateful operations
- **Error Handling**: Use Result pattern consistently

### Formatting

```bash
npm run lint      # Check and fix linting issues
npm run format    # Format code with Prettier
npm run typecheck # Check TypeScript types
```

### Code Quality

- Functions should be <50 lines
- Avoid deep nesting (max 3 levels)
- Use descriptive variable names
- Add error handling for all async operations
- Prefer composition over inheritance

## Architecture Guidelines

### AI Integration

- Use `createPromptBackedTool` for decision-making tools
- Structure prompts for consistent JSON output
- Include context and knowledge in prompts
- Track provenance for debugging

### Session Management

- Always update session state after operations
- Use typed session data structures
- Clean up sessions when complete
- Handle session recovery gracefully

### Error Handling

- Use Result pattern (`Success`/`Failure`)
- Provide actionable error messages
- Log errors with structured data
- Include troubleshooting hints

### Dependencies

- Pass dependencies explicitly to tools
- Use factory functions for dependency creation
- Mock dependencies in tests
- Avoid circular dependencies

## Getting Help

- **GitHub Issues**: Bug reports and feature requests
- **Discussions**: Questions and general discussion
- **Documentation**: Check existing docs first
- **Code Review**: Ask for feedback on complex changes

## Recognition

Contributors are recognized in:
- GitHub contributors list
- Release notes for significant contributions
- Documentation acknowledgments

Thank you for contributing to Containerization Assist!

## Contributor License Agreement

This project welcomes contributions and suggestions. Most contributions require you to agree to a Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us the rights to use your contribution.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide a CLA and decorate the PR appropriately. Simply follow the instructions provided by the bot.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.