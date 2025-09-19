# Prompt-Backed Tools Architecture

## Overview

This document defines the unified architecture for all MCP tools in the containerization-assist project, implementing the AI-first vision where business logic lives in prompts and TypeScript handles only side effects.

## Core Principles

### 1. AI-First Design
- **Business Logic**: All strategy, analysis, and decision-making in AI prompts
- **TypeScript Role**: Only side effects (Docker API, K8s API, file I/O, HTTP calls)
- **Separation**: Clear boundary between AI intelligence and system integration

### 2. Explicit Dependencies
- No global dependency injection container usage in tools
- All dependencies passed explicitly through `ToolDeps` interface
- Enables better testing and reduces coupling

### 3. Consistent Interface
- All tools export identical structure for MCP registration
- Standardized execution signature across all tools
- Predictable error handling and session management

## Architecture Components

### 1. Prompt-Backed Tool Factory

```typescript
import { createPromptBackedTool } from '@mcp/tools/prompt-backed-tool';

const toolNameAI = createPromptBackedTool({
  name: 'tool-operation-name',           // Unique identifier for AI tool
  description: 'What this tool does',    // Human-readable description
  inputSchema: ToolInputSchema,          // Zod schema for input validation
  outputSchema: ToolOutputSchema,        // Zod schema for AI response
  promptId: 'prompt-file-name',          // References src/prompts/**/*.yaml
  knowledge: {                           // Knowledge base integration
    category: 'dockerfile|kubernetes|security',
    limit: 4,                           // Max knowledge entries
    textSelector: (params) => string,   // Optional: extract search text
  },
  policy: {                             // Policy-based parameter extraction
    tool: 'tool-name',
    extractor: (params) => object,     // Extract policy-relevant params
  },
});
```

### 2. TypeScript Execution Wrapper

```typescript
async function executeToolName(
  params: ToolParams,
  deps: ToolDeps,
  context: ToolContext,
): Promise<Result<ToolResult & { sessionId: string; ok: boolean }>> {
  const { logger } = deps;

  try {
    // 1. Validate input parameters
    const validated = toolInputSchema.parse(params);

    // 2. TypeScript side effects only
    const sideEffectData = await performSideEffects(validated, deps);

    // 3. Delegate intelligence to AI
    const aiResult = await toolNameAI.execute({
      ...validated,
      sideEffectData,
    }, deps, context);

    if (!aiResult.ok) {
      return aiResult;
    }

    // 4. Apply AI recommendations via TypeScript
    const finalResult = await applySideEffects(aiResult.value, deps);

    // 5. Session management
    const sessionId = params.sessionId || `${toolName}-${Date.now()}`;
    if (context.sessionManager) {
      await context.sessionManager.update(sessionId, {
        [toolName]: finalResult,
      });
    }

    return Success({
      ...finalResult,
      sessionId,
      ok: true,
    });
  } catch (error) {
    logger.error({ error: extractErrorMessage(error) }, 'Tool execution failed');
    return Failure(extractErrorMessage(error));
  }
}
```

### 3. MCP Tool Export

```typescript
export const tool = {
  name: 'tool_name',                    // MCP registration name (snake_case)
  description: 'Tool description',      // MCP tool description
  inputSchema: toolInputSchema,         // Zod schema for MCP validation
  execute: executeToolName,             // Execution function
};
```

## Tool Categories & Patterns

### Category 1: Analysis Tools
**Examples**: analyze-repo, scan
**Pattern**: Read data → AI analysis → Return insights

```typescript
// Side Effects: File system reading, external tool execution
const fileData = await readProjectFiles(repoPath);
const scanResults = await runSecurityScan(imageName);

// AI Intelligence: Analysis, categorization, recommendations
const analysis = await analyzeRepoAI.execute({
  ...params,
  files: fileData.files,
  scanResults,
}, deps, context);
```

### Category 2: Generation Tools
**Examples**: generate-dockerfile, generate-k8s-manifests
**Pattern**: Analyze context → AI generation → Write files

```typescript
// Side Effects: Read existing files, write generated content
const contextData = await gatherGenerationContext(params);

// AI Intelligence: Content generation, optimization strategies
const generated = await generateDockerfileAI.execute({
  ...params,
  contextData,
}, deps, context);

// Side Effects: Write files, update file system
await writeGeneratedFiles(generated.value.files);
```

### Category 3: Execution Tools
**Examples**: build-image, deploy, scan
**Pattern**: AI strategy → Execute operations → Report results

```typescript
// AI Intelligence: Strategy determination, parameter optimization
const strategy = await buildStrategyAI.execute(params, deps, context);

// Side Effects: Docker/K8s operations, external process execution
const buildResult = await executeBuildStrategy(strategy.value, deps);

// Combine AI strategy with execution results
return Success({ strategy: strategy.value, execution: buildResult });
```

### Category 4: Management Tools
**Examples**: verify-deploy, prepare-cluster
**Pattern**: Check state → AI assessment → Execute actions

```typescript
// Side Effects: Query external systems
const currentState = await checkSystemState(params, deps);

// AI Intelligence: Assessment, action planning
const assessment = await verifyDeploymentAI.execute({
  ...params,
  currentState,
}, deps, context);

// Side Effects: Execute recommended actions
const actions = await executeActions(assessment.value.actions, deps);
```

## Integration Points

### 1. Knowledge Base Integration
Tools automatically access relevant knowledge based on category:

```typescript
knowledge: {
  category: 'dockerfile',              // Matches src/knowledge/dockerfile/
  limit: 4,                          // Max entries in prompt context
  textSelector: (params) => {         // Extract search context
    return params.files?.join('\n');  // Use file list for matching
  },
}
```

### 2. Policy Framework
Extract environment-specific parameters for policy application:

```typescript
policy: {
  tool: 'build-image',
  extractor: (params) => ({
    platform: params.platform ?? 'linux/amd64',
    environment: params.environment ?? 'development',
  }),
}
```

### 3. Session Management
Automatic session state persistence:

```typescript
// Sessions store tool results for cross-tool workflows
await context.sessionManager.update(sessionId, {
  'tool-name': {
    timestamp: new Date().toISOString(),
    result: toolResult,
    metadata: { version: '1.0' },
  },
});

// Later tools can access previous results
const session = await context.sessionManager.get(sessionId);
const previousAnalysis = session.value?.metadata?.['analyze-repo'];
```

## Error Handling

### Result Pattern
All tools use the `Result<T>` pattern for consistent error handling:

```typescript
import { Success, Failure, type Result } from '@types';

// Success case
return Success({
  data: result,
  sessionId,
  ok: true,
});

// Error case
return Failure('Descriptive error message');
```

### Error Types
- **Validation Errors**: Invalid input parameters (Zod validation)
- **Side Effect Errors**: Docker/K8s/filesystem failures
- **AI Errors**: Prompt execution or parsing failures
- **Session Errors**: Session management failures

## Testing Strategy

### Unit Testing
```typescript
describe('executeToolName', () => {
  it('should handle valid inputs', async () => {
    const mockDeps = createMockDeps();
    const mockContext = createMockContext();

    const result = await executeToolName(validParams, mockDeps, mockContext);

    expect(result.ok).toBe(true);
    expect(result.value.sessionId).toBeDefined();
  });
});
```

### Integration Testing
- MCP server registration and execution
- End-to-end workflows with session state
- External system integration (Docker, K8s)

## Migration Checklist

For each tool being migrated to this architecture:

- [ ] Create prompt-backed AI tool with appropriate schema
- [ ] Extract business logic to prompts
- [ ] Implement TypeScript wrapper for side effects only
- [ ] Add knowledge base integration
- [ ] Add policy framework integration
- [ ] Implement session management
- [ ] Add comprehensive error handling
- [ ] Write unit and integration tests
- [ ] Update MCP registration
- [ ] Verify backward compatibility

## Performance Considerations

### AI Tool Caching
- Prompt-backed tools include built-in caching
- Identical inputs return cached responses
- Cache TTL configurable per tool

### Knowledge Base Optimization
- Limit knowledge entries to 4-6 for prompt efficiency
- Use textSelector to provide relevant context
- Category-based filtering reduces irrelevant matches

### Session State Management
- Store only essential data in sessions
- Use compression for large session data
- Implement session cleanup policies

## Future Enhancements

### 1. Tool Composition
Enable tools to call other tools for complex workflows:

```typescript
// Tool A generates analysis, Tool B uses it for deployment
const analysis = await tool.analyzeRepo.execute(params, deps, context);
const deployment = await tool.deploy.execute({
  ...deployParams,
  analysis: analysis.value,
}, deps, context);
```

### 2. Streaming Responses
For long-running operations, implement streaming:

```typescript
// Stream build progress to client
for await (const progress of buildImage.stream(params, deps, context)) {
  context.progress?.(progress);
}
```

### 3. Advanced Policies
Environment-specific behavior modification:

```typescript
// Different base images per environment
policy: {
  tool: 'generate-dockerfile',
  rules: {
    production: { baseImage: 'alpine:latest' },
    development: { baseImage: 'ubuntu:latest' },
  },
}
```

This architecture ensures all tools follow consistent patterns while maintaining the flexibility needed for diverse containerization and orchestration tasks.