# Refactoring Plan: Type-Safe Ports & MCP Registration

## Principles
- **Use MCP SDK directly** - No custom registration layers, use SDK's built-in capabilities
- **Remove legacy code** - Delete unused SDKToolRegistry and dual registration patterns
- **Minimal abstractions** - Simple ports/adapters, no enterprise patterns
- **Clean implementation** - No backwards compatibility code

## Implementation Steps

### 1. Add Result Helper and MCP Types
**Files to create:**
- `src/types/result.ts` - Lightweight Result helpers (Ok, Err constructors)
- `src/mcp/types.ts` - MCP-specific types (TextContent, ToolContext, MCPTool, MCPResponse)

**Note:** Keep existing `src/domain/types.ts` Result<T> pattern, just add helpers

### 2. Create Type-Safe MCP Registration Helper
**File to create:**
- `src/exports/helpers.ts` - Minimal helper to register tools with MCP SDK server

**Key decisions:**
- Use MCP SDK's native `server.tool()` method directly
- Helper just wraps with type safety, no custom registry
- Use `zod-to-json-schema` for schema conversion

### 3. Create Docker Port and Adapter
**Files to create:**
- `src/ports/docker.ts` - Simple interface with pushImage, tagImage, buildImage
- `src/lib/docker-adapter.ts` - Adapt existing DockerClient to port interface

**Key decisions:**
- Port is just an interface, no base classes
- Adapter is a simple factory function, not a class
- Reuse existing DockerClient implementation

### 4. Remove SDK Tool Registry
**Files to delete:**
- `src/mcp/tools/registry.ts` - Remove completely
- `src/mcp/tools/sdk-native.ts` - Remove if exists

**Files to update:**
- `src/app/container.ts` - Remove SDKToolRegistry instantiation
- `src/mcp/server/index.ts` - Keep direct tool registration, enhance with helper

### 5. Refactor Push-Image Tool
**Files to update:**
- `src/tools/push-image/schema.ts` - Already good, keep as is
- `src/tools/push-image/tool.ts` - Rewrite to use DockerPort injection

**Key changes:**
- Factory function `makePushImage(docker: DockerPort)`
- No Docker client creation inside tool
- Use injected port for all Docker operations

### 6. Remove 'as any' Usage
**Files to fix:**
- `src/app/container.ts` - Line 143: Remove null as any
- `src/infrastructure/docker/registry.ts` - Line 48: Type API response
- `src/mcp/server/schemas.ts` - Line 267: Type JSON schema properly
- Other instances as needed

### 7. Write Tests
**File to create:**
- `test/unit/tools/push-image.test.ts` - Unit tests with fake DockerPort

**Test cases:**
- Success path with digest
- Tag failure handling
- Push failure handling
- Missing imageId error

### 8. Update Linting Rules
**Files to update:**
- `.eslintrc.js` - Add no-explicit-any rules
- `tsconfig.json` - Consider stricter options

### 9. Clean Up Legacy Code
**Verify and remove:**
- Unused tool registry code
- Duplicate registration patterns
- Any backwards compatibility shims
- Commented out code

## Commit Structure
1. `feat(types): add Result helpers and MCP types`
2. `feat(mcp): add type-safe registration helper using SDK`
3. `feat(ports): introduce DockerPort and adapter`
4. `refactor(container): remove SDKToolRegistry and clean up wiring`
5. `refactor(push-image): use DockerPort injection`
6. `fix: remove all 'as any' usage`
7. `test: add push-image unit tests`
8. `chore: tighten eslint and tsconfig rules`
9. `chore: remove legacy code and unused files`

## Success Criteria
- [ ] No `as any` in modified files
- [ ] Push-image uses only DockerPort
- [ ] SDKToolRegistry completely removed
- [ ] Single tool registration pattern via MCP SDK
- [ ] Tests pass without Docker
- [ ] No legacy/compatibility code remains
- [ ] Lint and typecheck pass