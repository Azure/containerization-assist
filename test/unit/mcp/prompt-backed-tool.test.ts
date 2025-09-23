/**
 * Unit tests for prompt-backed tool functionality
 */

import { describe, it, expect } from '@jest/globals';
import { extractJSON } from '@/mcp/prompt-backed-tool';

describe('Prompt-Backed Tool', () => {
  describe('extractJSON', () => {
    it('should extract JSON from markdown code blocks', () => {
      const text = `Here is the result:
\`\`\`json
{
  "name": "test",
  "value": 123
}
\`\`\`
That's the JSON.`;

      const result = extractJSON(text);
      expect(result).toEqual({ name: 'test', value: 123 });
    });

    it('should extract JSON from code blocks without json marker', () => {
      const text = `Result:
\`\`\`
{
  "key": "value",
  "number": 42
}
\`\`\``;

      const result = extractJSON(text);
      expect(result).toEqual({ key: 'value', number: 42 });
    });

    it('should extract raw JSON without code blocks', () => {
      const text = `The response is: {"status": "success", "count": 5}`;

      const result = extractJSON(text);
      expect(result).toEqual({ status: 'success', count: 5 });
    });

    it('should handle multi-line raw JSON', () => {
      const text = `Response:
{
  "items": [
    {"id": 1, "name": "first"},
    {"id": 2, "name": "second"}
  ],
  "total": 2
}
End of response`;

      const result = extractJSON(text);
      expect(result).toEqual({
        items: [
          { id: 1, name: 'first' },
          { id: 2, name: 'second' },
        ],
        total: 2,
      });
    });

    it('should fix trailing commas in JSON', () => {
      const text = `\`\`\`json
{
  "field1": "value1",
  "field2": "value2",
  "field3": "value3",
}
\`\`\``;

      const result = extractJSON(text);
      expect(result).toEqual({
        field1: 'value1',
        field2: 'value2',
        field3: 'value3',
      });
    });

    it('should handle trailing commas in arrays', () => {
      const text = `{
        "items": [
          "item1",
          "item2",
          "item3",
        ],
        "count": 3,
      }`;

      const result = extractJSON(text);
      expect(result).toEqual({
        items: ['item1', 'item2', 'item3'],
        count: 3,
      });
    });

    it('should handle nested objects with trailing commas', () => {
      const text = `\`\`\`json
{
  "outer": {
    "inner": {
      "value": 1,
    },
  },
}
\`\`\``;

      const result = extractJSON(text);
      expect(result).toEqual({
        outer: {
          inner: {
            value: 1,
          },
        },
      });
    });

    it('should throw error when no JSON is found', () => {
      const text = 'This is just plain text with no JSON content';

      expect(() => extractJSON(text)).toThrow('No JSON found in response');
    });

    it('should throw error for invalid JSON that cannot be fixed', () => {
      const text = `{"broken": "json" "missing": "comma"}`;

      expect(() => extractJSON(text)).toThrow('JSON parsing failed');
    });

    it('should prefer code block JSON over raw JSON', () => {
      const text = `{"ignored": "json"}
\`\`\`json
{"preferred": "json"}
\`\`\`
{"also": "ignored"}`;

      const result = extractJSON(text);
      expect(result).toEqual({ preferred: 'json' });
    });

    it('should handle empty objects and arrays', () => {
      const text1 = '```json\n{}\n```';
      const result1 = extractJSON(text1);
      expect(result1).toEqual({});

      const text2 = '```json\n[]\n```';
      const result2 = extractJSON(text2);
      expect(result2).toEqual([]);
    });

    it('should handle JSON with special characters', () => {
      const text = `\`\`\`json
{
  "message": "Line 1\\nLine 2",
  "path": "C:\\\\Users\\\\test",
  "unicode": "\\u0048\\u0065\\u006c\\u006c\\u006f"
}
\`\`\``;

      const result = extractJSON(text);
      expect(result).toEqual({
        message: 'Line 1\nLine 2',
        path: 'C:\\Users\\test',
        unicode: 'Hello',
      });
    });

    it('should handle JSON with numbers and booleans', () => {
      const text = `{
        "integer": 42,
        "float": 3.14159,
        "scientific": 1.23e-4,
        "boolean_true": true,
        "boolean_false": false,
        "null_value": null
      }`;

      const result = extractJSON(text);
      expect(result).toEqual({
        integer: 42,
        float: 3.14159,
        scientific: 0.000123,
        boolean_true: true,
        boolean_false: false,
        null_value: null,
      });
    });
  });
});