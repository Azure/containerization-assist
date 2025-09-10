import type { AITemplate } from './types';

export const JSON_REPAIR: AITemplate = {
  id: 'json-repair',
  name: 'JSON Repair',
  description: 'Fix malformed JSON responses with auto-repair capabilities',
  version: '1.0.0',
  system: `You are a JSON repair specialist. Your task is to fix malformed JSON and return only valid JSON.

Follow these repair strategies:
1. Fix syntax errors (missing commas, brackets, quotes)
2. Ensure proper string escaping
3. Remove markdown code fences if present
4. Fix trailing commas
5. Ensure proper number formatting
6. Fix boolean values (true/false, not True/False)
7. Handle null values properly

CRITICAL: Return ONLY the corrected JSON - no explanations, no markdown, no additional text.`,
  user: `The following JSON has errors:
{{malformed_json}}

Error: {{error_message}}

{{repair_instruction}}

Fix the JSON and return ONLY the corrected JSON.`,
  variables: [
    {
      name: 'malformed_json',
      description: 'The malformed JSON content to repair',
      required: true,
    },
    {
      name: 'error_message',
      description: 'The specific error message from JSON parsing',
      required: true,
    },
    {
      name: 'repair_instruction',
      description: 'Specific repair instructions based on error type',
      required: true,
    },
  ],
  outputFormat: 'json',
  examples: [
    {
      input: {
        malformed_json: '{\n  "language": "nodejs",\n  "framework": "express"\n',
        error_message: 'Unexpected end of JSON input',
        repair_instruction: 'Fix missing closing brace',
      },
      output: '{\n  "language": "nodejs",\n  "framework": "express"\n}\n',
    },
    {
      input: {
        malformed_json: '```json\n{\n  "ports": [3000, 8080,],\n  "secure": True\n}\n```\n',
        error_message: 'Unexpected token ] in JSON',
        repair_instruction: 'Fix trailing comma and boolean value',
      },
      output: '{\n  "ports": [3000, 8080],\n  "secure": true\n}\n',
    },
  ],
  tags: ['json', 'repair', 'reliability', 'error-recovery'],
} as const;
