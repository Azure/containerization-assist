# Session State Management Guide

## Overview

This guide documents the canonical session state structure used throughout the containerization-assist codebase. Following this structure ensures consistency and prevents data mismatches between tool executions.

## Canonical Session Structure

### Single Source of Truth

All tool results are stored in **one and only one location**:

```typescript
session.metadata.results[toolName] = toolResult;
```

**NEVER** use:
- ~~`session.results`~~ (top-level, deprecated, removed)
- ~~`session.metadata[toolName]`~~ (incorrect nesting)

### WorkflowState Interface

```typescript
interface WorkflowState {
  sessionId: string;                          // Unique session identifier
  metadata?: {
    results?: Record<string, unknown>;        // CANONICAL LOCATION for tool results
    [key: string]: unknown;                   // Other workflow metadata
  };
  completed_steps?: string[];                 // Array of completed tool names
  errors?: Record<string, unknown>;           // Errors encountered
  createdAt: Date;
  updatedAt: Date;
}
```

## Writing Tool Results

### Using the Canonical Helper

**Always use `updateSessionResults`** for writing tool results:

```typescript
import { updateSessionResults } from '@/lib/tool-helpers';

// In your tool implementation
const session = sessionResult.value;
updateSessionResults(session, 'analyze-repo', {
  language: 'Java',
  framework: 'Spring Boot',
  buildTool: 'Maven'
});

// Persist to session manager
await sessionManager.update(sessionId, session);
```

### Benefits of the Helper

1. **Consistency**: Ensures all writes go to `metadata.results`
2. **Validation**: Runtime checks for invalid session/toolName
3. **Timestamp**: Automatically updates `session.updatedAt`
4. **Initialization**: Creates `metadata.results` if missing

### Direct SessionFacade Usage

The `SessionFacade.storeResult` method internally uses `updateSessionResults`:

```typescript
// In tool code with ToolContext
ctx.session.storeResult('build-image', {
  imageId: 'sha256:abc123',
  tags: ['myapp:latest']
});
```

## Reading Tool Results

### Using SessionFacade.getResult

**Preferred approach** for reading cross-tool results:

```typescript
// Read analysis from previous tool
const analysis = ctx.session.getResult<RepositoryAnalysis>('analyze-repo');

if (analysis) {
  console.log(`Language: ${analysis.language}`);
  console.log(`Framework: ${analysis.framework}`);
}
```

### Reading Other Metadata

For non-result metadata (flags, paths, etc.):

```typescript
const appName = ctx.session.get<string>('appName');
const analyzedPath = ctx.session.get<string>('analyzedPath');
const isMonorepo = ctx.session.get<boolean>('isMonorepo');
```

## Common Patterns

### Tool Dependency Pattern

When a tool depends on results from a previous tool:

```typescript
async function run(input: ToolInput, ctx: ToolContext): Promise<Result<Output>> {
  // Try to get upstream results
  const analysis = ctx.session.getResult<RepositoryAnalysis>('analyze-repo');

  if (!analysis) {
    return Failure('Repository analysis not found. Please run analyze-repo first.');
  }

  // Use analysis data
  const dockerfile = generateDockerfile(analysis.language, analysis.framework);

  // Store this tool's results
  ctx.session.storeResult('generate-dockerfile', {
    content: dockerfile,
    path: input.outputPath
  });

  return Success({ content: dockerfile });
}
```

### Multi-Tool Workflow

```typescript
// Tool 1: Analyze repository
ctx.session.storeResult('analyze-repo', {
  language: 'Python',
  framework: 'FastAPI',
  dependencies: ['fastapi', 'uvicorn']
});

// Tool 2: Build image (reads analysis)
const analysis = ctx.session.getResult<Analysis>('analyze-repo');
const image = await buildImage(analysis);
ctx.session.storeResult('build-image', {
  imageId: image.id,
  tags: ['myapp:v1.0.0']
});

// Tool 3: Deploy (reads build results)
const buildResult = ctx.session.getResult<BuildResult>('build-image');
await deploy(buildResult.imageId);
ctx.session.storeResult('deploy', {
  status: 'deployed',
  endpoint: 'https://myapp.example.com'
});
```

## Error Handling

### Runtime Validation

The `updateSessionResults` helper validates inputs and throws descriptive errors:

```typescript
// ❌ Throws: "Cannot update session results: session is null"
updateSessionResults(null, 'tool-name', data);

// ❌ Throws: "Cannot update session results: session.sessionId is missing"
updateSessionResults({ metadata: {} }, 'tool-name', data);

// ❌ Throws: "Cannot update session results: toolName is invalid"
updateSessionResults(session, '', data);

// ✅ Succeeds
updateSessionResults(session, 'analyze-repo', data);
```

### Missing Dependencies

When a tool requires results from another tool:

```typescript
const upstreamResult = ctx.session.getResult<UpstreamData>('upstream-tool');

if (!upstreamResult) {
  return Failure(
    'Missing required data from upstream-tool. ' +
    'Please run upstream-tool before this tool.'
  );
}
```

## Testing

### Unit Tests

Mock SessionFacade for testing tool logic:

```typescript
const mockSession = {
  getResult: jest.fn((toolName: string) => {
    if (toolName === 'analyze-repo') {
      return { language: 'Java', framework: 'Spring Boot' };
    }
    return undefined;
  }),
  storeResult: jest.fn()
};

const ctx = { session: mockSession } as ToolContext;
```

### Integration Tests

Test full workflow with SessionManager:

```typescript
const sessionManager = new SessionManager(logger);
const createResult = await sessionManager.create();
const session = createResult.value;

// Store results
updateSessionResults(session, 'analyze-repo', analysisData);
await sessionManager.update(session.sessionId, session);

// Retrieve and verify
const retrieved = await sessionManager.get(session.sessionId);
const results = retrieved.value?.metadata?.results;
expect(results['analyze-repo']).toEqual(analysisData);
```

## Migration Notes

### Removed Patterns

These patterns are **no longer valid**:

```typescript
// ❌ REMOVED: Top-level results field
workflowState.results['tool-name'] = data;

// ❌ REMOVED: Direct metadata assignment
workflowState.metadata['tool-name'] = data;

// ❌ REMOVED: Fallback reads
const result = workflowState.results?.['tool-name'] ||
              workflowState.metadata?.results?.['tool-name'];
```

### New Patterns

Use these patterns instead:

```typescript
// ✅ Write via helper
updateSessionResults(session, 'tool-name', data);

// ✅ Write via SessionFacade
ctx.session.storeResult('tool-name', data);

// ✅ Read via SessionFacade
const result = ctx.session.getResult<T>('tool-name');
```

## Best Practices

1. **Always use helpers**: Never manually write to `session.metadata.results`
2. **Type your reads**: Use generics with `getResult<T>()` for type safety
3. **Check for undefined**: Always validate that upstream results exist
4. **Store complete data**: Include all relevant data in tool results
5. **Document dependencies**: Comment which tools your tool depends on
6. **Test workflows**: Add integration tests for multi-tool sequences

## Troubleshooting

### Results Not Found

If `getResult()` returns `undefined`:

1. Check that the upstream tool completed successfully
2. Verify the tool name matches exactly (case-sensitive)
3. Confirm the session ID is consistent across tools
4. Check that the session was persisted after storing results

### Type Mismatches

If results have unexpected structure:

1. Use TypeScript interfaces for tool output types
2. Add runtime validation of upstream results
3. Check that the upstream tool's output matches your expectations

### Example Debug Pattern

```typescript
const result = ctx.session.getResult<ExpectedType>('upstream-tool');

if (!result) {
  ctx.logger.error('Missing results from upstream-tool');
  return Failure('Upstream tool has not run');
}

if (!result.expectedField) {
  ctx.logger.warn({ result }, 'Upstream result missing expected field');
  return Failure('Upstream result is incomplete');
}
```

## Summary

- **Write**: Use `updateSessionResults()` or `ctx.session.storeResult()`
- **Read**: Use `ctx.session.getResult<T>()`
- **Location**: `session.metadata.results[toolName]`
- **Validate**: Check for `undefined` when reading upstream results
- **Test**: Add integration tests for tool workflows
