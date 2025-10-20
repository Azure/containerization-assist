# ADR-002: Unified Tool Interface

**Date:** 2025-10-17
**Status:** Accepted
**Deciders:** Engineering Team
**Context:** We needed a standardized way to define and implement MCP tools that ensures consistency, type safety, and seamless integration with the MCP protocol while keeping tool implementations simple and maintainable.

## Decision

All tools implement a unified `Tool<TSchema, TOut>` interface with co-located schema definitions and a standardized structure.

**Implementation:**

```typescript
// src/types/tool.ts
interface Tool<TSchema extends z.ZodType<any>, TOut> {
  name: string;
  description: string;
  version: string;
  schema: TSchema;
  run: (
    input: z.infer<TSchema>,
    ctx: ToolContext
  ) => Promise<Result<TOut>>;
}

// src/tools/build-image/tool.ts
const tool: Tool<typeof buildImageSchema, BuildImageResult> = {
  name: 'build-image',
  description: 'Build Docker image with progress tracking',
  version: '2.0.0',
  schema: buildImageSchema,
  run: async (input, ctx) => {
    // Implementation
    return Success(result);
  }
};

export default tool;
```

**Directory Structure:**

```
src/tools/
├── build-image/
│   ├── tool.ts          # Tool implementation
│   ├── schema.ts        # Zod schema
│   └── index.ts         # Re-export
├── scan-image/
│   ├── tool.ts
│   ├── schema.ts
│   └── index.ts
└── ...17 tools total
```

## Rationale

1. **Type Safety:** Generic `Tool<TSchema, TOut>` ensures schema and output types match
2. **Consistency:** All 17 tools follow identical structure, making them easy to understand
3. **Co-location:** Schema, implementation, and types live together for discoverability
4. **MCP Integration:** Interface maps directly to MCP protocol requirements
5. **Simplicity:** No adapters or wrappers needed; tools are used directly
6. **Validation:** Zod schemas provide runtime validation and TypeScript type inference

## Consequences

### Positive

- **17 tools with identical structure:** Consistent pattern across entire codebase
- **Easy to add new tools:** Copy existing tool, modify schema and implementation
- **Type-safe tool calls:** Input/output types automatically inferred
- **Automatic validation:** Zod validates inputs before tool execution
- **Self-documenting:** Schema describes parameters, making tools discoverable
- **No boilerplate:** Direct integration without adapters or wrappers
- **Excellent IDE support:** Full autocomplete for tool parameters and results

### Negative

- **Rigid structure:** All tools must follow the same pattern
- **Migration cost:** Existing tools needed conversion to new interface
- **Schema duplication:** Some tools have complex schemas that could be shared
- **Context dependency:** All tools require ToolContext even if not used

## Alternatives Considered

### Alternative 1: Individual Tool Signatures

- **Pros:**
  - Flexibility for each tool
  - No forced structure
  - Simpler initial implementation
- **Cons:**
  - Inconsistent patterns across tools
  - Harder to maintain
  - No type safety guarantees
  - Manual validation needed
- **Rejected because:** Lack of consistency makes codebase harder to navigate and maintain

### Alternative 2: Class-based Tools

```typescript
class BuildImageTool extends BaseTool<BuildImageInput, BuildImageResult> {
  async run(input: BuildImageInput): Promise<Result<BuildImageResult>> {
    // ...
  }
}
```

- **Pros:**
  - Inheritance for shared behavior
  - Familiar OOP pattern
  - Instance methods for internal logic
- **Cons:**
  - More boilerplate
  - Harder to test
  - Requires `new` keyword
  - Less functional composition
- **Rejected because:** Functional approach is simpler and more testable

### Alternative 3: MCP SDK Tool Decorators

- **Pros:**
  - Official SDK integration
  - Automatic registration
  - Metadata via decorators
- **Cons:**
  - Additional dependency
  - Decorator complexity
  - Less explicit
  - Runtime magic reduces clarity
- **Rejected because:** We wanted explicit, simple patterns without magic

## Related Decisions

- **ADR-001: Result<T> Error Handling Pattern** - All tools return Result<T> from their run function
- **ADR-005: MCP Protocol Integration** - Tool interface maps directly to MCP protocol requirements
- **ADR-006: Infrastructure Layer Organization** - Tools live in @tools/* layer and orchestrate infrastructure

## References

- Tool interface: `src/types/tool.ts`
- Tool context: `src/mcp/context.ts`
- Example implementations: `src/tools/*/tool.ts`
- Tool registration: `src/mcp/server/index.ts`
