import type { AITemplate } from './types';

export const JSON_REPAIR: AITemplate = {
  id: 'json-repair',
  name: 'JSON Repair',
  description: 'Fix malformed JSON responses with auto-repair capabilities',
  version: '1.0.0',
  system:
    'You are a JSON repair specialist. Your task is to fix malformed JSON and return only valid JSON.\n\nFollow these repair strategies:\n1. Fix syntax errors (missing commas, brackets, quotes)\n2. Ensure proper string escaping\n3. Remove markdown code fences if present\n4. Fix trailing commas\n5. Ensure proper number formatting\n6. Fix boolean values (true/false, not True/False)\n7. Handle null values properly\n\nCRITICAL: Return ONLY the corrected JSON - no explanations, no markdown, no additional text.\n',
  user: 'The following JSON has errors:\n{{malformed_json}}\n\nError: {{error_message}}\n\n{{repair_instruction}}\n\nFix the JSON and return ONLY the corrected JSON.\n',
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
