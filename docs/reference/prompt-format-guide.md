# Prompt Format Guide

## Overview

All prompts in the containerization-assist project follow a unified YAML format that ensures consistency and proper validation through the prompt registry system.

## Standard Format

Every prompt file must follow this structure:

```yaml
id: prompt-name
version: "1.0.0"
category: analysis|containerization|orchestration|security|validation|sampling
description: "Clear description of what this prompt does"
format: text|json|markdown|yaml
parameters:
  - name: parameter_name
    type: string|number|boolean|array|object
    required: true|false
    description: "What this parameter does"
    default: optional_default_value
template: |
  Your complete prompt content here with {{variables}} for substitution.
  The template should include all instructions and formatting requirements.

  Variables are substituted using Mustache syntax: {{parameter_name}}

  For JSON output format, include the expected structure in the template.

# Optional fields
ttl: 3600  # Cache TTL in seconds
source: "URL or reference"
previousVersion: "0.9.0"
deprecated: false
extends: "parent-prompt-id"
examples:  # Optional examples section
  - input:
      parameter1: value1
    output: |
      Expected output
tags:  # Optional tags for categorization
  - tag1
  - tag2
```

## Field Descriptions

### Required Fields

- **`id`**: Unique identifier for the prompt (kebab-case)
- **`version`**: Semantic version (must match format `X.Y.Z`)
- **`category`**: Category for organizing prompts
- **`description`**: Human-readable description
- **`format`**: Output format expected from the AI
- **`parameters`**: Array of parameter definitions
- **`template`**: The actual prompt template with variable placeholders

### Parameter Definition

Each parameter must include:
- **`name`**: Parameter identifier (snake_case)
- **`type`**: Data type (string, number, boolean, array, object)
- **`required`**: Whether the parameter is required
- **`description`**: Clear description of the parameter's purpose
- **`default`** (optional): Default value if not provided

### Optional Fields

- **`ttl`**: Cache time-to-live in seconds
- **`source`**: Reference URL or documentation
- **`previousVersion`**: Previous version for migration tracking
- **`deprecated`**: Mark prompt as deprecated
- **`extends`**: Inherit from another prompt
- **`examples`**: Input/output examples for documentation
- **`tags`**: Additional categorization tags

## Format Types

### text
Plain text output without specific formatting requirements.

### json
Structured JSON output. The template should specify the expected JSON schema.

### markdown
Markdown-formatted output for documentation or reports.

### yaml
YAML-formatted output, typically for configuration files.

## Variable Substitution

Variables are substituted using Mustache syntax:
- Basic: `{{variable_name}}`
- Nested: `{{object.property}}`
- Conditionals are handled by providing empty strings for missing optional parameters

## Best Practices

1. **Clear Instructions**: Include all necessary context and instructions in the template
2. **Explicit Output Format**: For JSON/YAML formats, include the expected structure
3. **Parameter Validation**: Mark parameters as required only when truly necessary
4. **Versioning**: Increment version when making breaking changes
5. **Documentation**: Include examples when the usage isn't obvious
6. **Naming**: Use descriptive, kebab-case IDs that reflect the prompt's purpose

## Migration from Old Format

The old format used separate `system`, `user`, and `variables` fields:

```yaml
# OLD FORMAT (deprecated)
system: "System instructions..."
user: "User prompt..."
variables:
  - name: var_name
    required: true
outputFormat: json
```

This has been replaced with the unified format where `system` and `user` are combined into `template`, `variables` becomes `parameters` with proper types, and `outputFormat` becomes `format`.

## Validation

All prompts are validated against a Zod schema on load. Validation errors will prevent the prompt from being loaded and will be logged with details about what failed validation.

The validation checks:
- Required fields are present
- Version follows semver format
- Format is one of the allowed values
- Parameters have valid type definitions
- Template is a non-empty string

## Examples

### Simple Text Prompt

```yaml
id: error-analysis
version: "2.1.0"
description: Analyze and fix containerization errors
category: validation
format: text
parameters:
  - name: language
    type: string
    description: Programming language
    required: true
  - name: error_message
    type: string
    description: The error to analyze
    required: true
template: |
  Analyze this {{language}} containerization error:

  {{error_message}}

  Provide a clear explanation and solution.
```

### JSON Output Prompt

```yaml
id: repository-analysis
version: "2.1.0"
description: Analyze repository structure
category: analysis
format: json
parameters:
  - name: file_list
    type: string
    description: List of repository files
    required: true
template: |
  Analyze this repository structure:

  {{file_list}}

  Return JSON in this format:
  {
    "primary_language": "detected language",
    "framework": "detected framework or null",
    "containerization_ready": true|false
  }
```

## Testing

To test that your prompt loads correctly:

1. Place it in the appropriate category folder under `src/prompts/`
2. Run the validation: `npm run validate`
3. Check the logs for any validation errors
4. Test with actual parameters to ensure proper rendering