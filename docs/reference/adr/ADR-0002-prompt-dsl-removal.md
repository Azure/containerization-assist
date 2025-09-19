# ADR-0002: Prompt DSL Removal (Variables-Only)

## Status
Accepted

## Context

The containerization assistant initially implemented a complex Domain-Specific Language (DSL) for prompt templates that included conditional logic, loops, and complex transformations. Over time, we discovered that this complexity was:

1. **Difficult to Debug**: Complex template logic made it hard to understand why prompts were generated incorrectly
2. **Maintenance Burden**: Template logic required specialized knowledge and tooling
3. **Performance Overhead**: Template parsing and execution added latency
4. **Limited AI Benefit**: AI models handle context better when given simpler, more direct prompts
5. **Reduced Transparency**: Complex transformations made it harder to understand what the AI was actually seeing

The original DSL supported:
- Conditional blocks (`{{#if condition}}...{{/if}}`)
- Loops (`{{#each items}}...{{/each}}`)
- Complex transformations (`{{transform data format="json"}}`)
- Nested template inheritance
- Dynamic section inclusion/exclusion

## Decision

We remove all complex DSL features and retain **only variable substitution** using Mustache-style syntax. The new simplified approach:

### Retained Features
- **Simple Variable Substitution**: `{{variableName}}`
- **Nested Object Access**: `{{object.property}}`
- **Array Index Access**: `{{array.0}}`
- **Safe Null Handling**: Missing variables render as empty strings

### Removed Features
- ❌ Conditional logic (`{{#if}}`, `{{#unless}}`)
- ❌ Iteration (`{{#each}}`, `{{#with}}`)
- ❌ Custom helpers and transformations
- ❌ Template inheritance and partials
- ❌ Dynamic section inclusion
- ❌ Complex formatting functions

### Migration Strategy

Complex logic is moved to:
1. **Data Preparation**: Transform data before passing to templates
2. **Multiple Templates**: Use different templates for different scenarios
3. **Code Logic**: Handle conditionals in TypeScript rather than templates

## Rationale

### Why Variables-Only?

1. **Simplicity**: Templates become readable documentation of what data the AI receives
2. **Debuggability**: Easy to inspect the exact prompt sent to the AI
3. **Performance**: No template parsing overhead beyond simple variable substitution
4. **Maintainability**: Anyone can understand and modify variable-only templates
5. **AI Effectiveness**: Cleaner prompts often produce better AI responses

### Why Not Keep Complex Features?

1. **Complexity vs Value**: Complex templating provided little benefit over data preprocessing
2. **Error Prone**: Template logic bugs were hard to track and fix
3. **Knowledge Barrier**: Required team members to learn template language
4. **Testing Difficulty**: Template logic required separate test infrastructure

### Why Mustache-Style Variables?

1. **Familiar Syntax**: Widely recognized `{{variable}}` pattern
2. **Tool Support**: Many editors provide syntax highlighting
3. **Safe Defaults**: Missing variables don't break template rendering
4. **Standard Behavior**: Well-defined variable substitution semantics

## Implementation Details

### Before (Complex DSL)

```yaml
# Complex template with conditionals and loops
template: |
  Analyze this {{technology}} project.

  {{#if hasDockerfile}}
  Existing Dockerfile found:
  ```dockerfile
  {{dockerfileContent}}
  ```
  {{else}}
  No existing Dockerfile detected.
  {{/if}}

  {{#each frameworks}}
  Framework: {{name}} ({{version}})
  {{#if config}}
  Configuration:
  {{#each config}}
  - {{key}}: {{value}}
  {{/each}}
  {{/if}}
  {{/each}}

  {{#unless isSimpleProject}}
  This appears to be a complex project requiring:
  {{transform requirements format="bullet"}}
  {{/unless}}
```

### After (Variables-Only)

```yaml
# Simple template with only variable substitution
template: |
  Analyze this {{technology}} project in {{projectPath}}.

  Project Structure:
  {{projectStructure}}

  Detected Frameworks:
  {{frameworkSummary}}

  {{dockerfileStatus}}

  {{existingDockerfileContent}}

  Requirements:
  {{projectRequirements}}
```

### Data Preparation Logic

```typescript
// Complex logic moved to data preparation
function preparePromptData(analysis: RepoAnalysis): PromptData {
  // Handle Dockerfile logic
  const dockerfileStatus = analysis.hasDockerfile
    ? 'Existing Dockerfile found:'
    : 'No existing Dockerfile detected.';

  const existingDockerfileContent = analysis.hasDockerfile
    ? `\`\`\`dockerfile\n${analysis.dockerfileContent}\n\`\`\``
    : '';

  // Handle frameworks formatting
  const frameworkSummary = analysis.frameworks
    .map(f => `- ${f.name} (${f.version})`)
    .join('\n');

  // Handle requirements formatting
  const projectRequirements = analysis.isSimpleProject
    ? 'Standard containerization approach'
    : analysis.requirements.map(r => `- ${r}`).join('\n');

  return {
    technology: analysis.technology,
    projectPath: analysis.path,
    projectStructure: analysis.structure,
    frameworkSummary,
    dockerfileStatus,
    existingDockerfileContent,
    projectRequirements,
  };
}
```

### New Template Processing

```typescript
interface TemplateProcessor {
  /**
   * Process template with variable substitution only
   */
  process(template: string, data: Record<string, unknown>): string;
}

class SimpleTemplateProcessor implements TemplateProcessor {
  process(template: string, data: Record<string, unknown>): string {
    return template.replace(/\{\{([^}]+)\}\}/g, (match, path) => {
      const value = this.getValue(data, path.trim());
      return value != null ? String(value) : '';
    });
  }

  private getValue(obj: any, path: string): unknown {
    return path.split('.').reduce((current, key) => {
      return current?.[key];
    }, obj);
  }
}
```

## Consequences

### Positive

1. **Improved Debuggability**: Easy to see exactly what prompt is sent to AI
2. **Better Performance**: No template parsing overhead
3. **Lower Maintenance**: Simple templates require minimal maintenance
4. **Easier Testing**: Data preparation logic is easily unit tested
5. **Better AI Results**: Cleaner prompts often produce better responses
6. **Lower Learning Curve**: New team members can immediately understand templates

### Negative

1. **More Data Preparation Code**: Logic moves from templates to TypeScript
2. **Template Duplication**: May need separate templates for different scenarios
3. **Loss of Template Reuse**: Cannot share complex template logic
4. **Verbose Data Preparation**: Some formatting logic becomes more verbose

### Mitigation Strategies

1. **Shared Data Formatters**: Create reusable formatting functions for common patterns
2. **Template Validation**: Ensure all referenced variables are provided
3. **Type Safety**: Use TypeScript interfaces for template data contracts
4. **Documentation**: Clear examples of data preparation patterns

## Migration Guide

### Step 1: Identify Complex Templates

Find templates using:
- `{{#if}}` conditions
- `{{#each}}` loops
- `{{#unless}}` negative conditions
- Custom helpers

### Step 2: Extract Logic to Data Preparation

Move template logic to TypeScript functions that prepare data before templating.

### Step 3: Simplify Templates

Replace complex template logic with simple variable references.

### Step 4: Update Tests

Change tests to verify data preparation logic rather than template processing.

## Examples

### Example 1: Conditional Content

**Before:**
```yaml
template: |
  {{#if hasTests}}
  Test files found: {{testCount}}
  {{else}}
  No tests detected. Consider adding tests.
  {{/if}}
```

**After:**
```yaml
template: |
  {{testStatus}}
```

```typescript
// Data preparation
const testStatus = analysis.hasTests
  ? `Test files found: ${analysis.testCount}`
  : 'No tests detected. Consider adding tests.';
```

### Example 2: List Formatting

**Before:**
```yaml
template: |
  Dependencies:
  {{#each dependencies}}
  - {{name}}: {{version}}
  {{/each}}
```

**After:**
```yaml
template: |
  Dependencies:
  {{dependencyList}}
```

```typescript
// Data preparation
const dependencyList = dependencies
  .map(dep => `- ${dep.name}: ${dep.version}`)
  .join('\n');
```

## Related Decisions

- ADR-0001: Effective Config & Policy Precedence
- ADR-0003: Router Architecture Split

## References

- [Mustache Template Specification](https://mustache.github.io/mustache.5.html)
- [Template Processing Implementation](../prompts/unified-registry.ts)
- [Data Preparation Patterns](../lib/prompt-formatters.ts)