package sampling

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// SampleJSONWithSchema enforces schema validation with repair attempts
func (c *Client) SampleJSONWithSchema(
	ctx context.Context,
	req SamplingRequest,
	out interface{},
	schemaJSON string,
) (*SamplingResponse, error) {
	// Strengthen system prompt for JSON-only output
	systemPrompt := strings.TrimSpace(req.SystemPrompt + `
You MUST respond with ONLY valid JSON. No code fences, no markdown, no explanatory text.
Your entire response must be a single JSON object or array.`)
	req.SystemPrompt = systemPrompt

	// Add JSON instruction to the main prompt as well
	if !strings.Contains(strings.ToLower(req.Prompt), "json") {
		req.Prompt = req.Prompt + "\n\nRemember: Respond with ONLY valid JSON."
	}

	// First attempt
	resp, err := c.SampleInternal(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("sampling failed: %w", err)
	}

	// Try to unmarshal directly
	content := strings.TrimSpace(resp.Content)
	if err := json.Unmarshal([]byte(content), out); err == nil {
		// Validate against schema if provided
		if schemaJSON != "" {
			if err := validateJSONSchema(content, schemaJSON); err == nil {
				return resp, nil
			}
		} else {
			return resp, nil
		}
	}

	// Extract JSON from mixed content if needed
	candidate := extractJSONCandidate(content)
	if err := json.Unmarshal([]byte(candidate), out); err == nil {
		// Validate against schema if provided
		if schemaJSON != "" {
			if err := validateJSONSchema(candidate, schemaJSON); err == nil {
				resp.Content = candidate
				return resp, nil
			}
		} else {
			resp.Content = candidate
			return resp, nil
		}
	}

	// Self-healing repair attempts (up to 2)
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		c.logger.Debug("attempting JSON repair",
			"attempt", attempt,
			"last_error", lastErr)

		fixPrompt := buildJSONRepairPrompt(schemaJSON, candidate, lastErr)
		fixReq := SamplingRequest{
			Prompt:       fixPrompt,
			MaxTokens:    minNonZero(req.MaxTokens, 800),
			Temperature:  0.0,
			SystemPrompt: "Output ONLY valid JSON. No comments, no code fences, no markdown.",
		}

		fixed, err := c.SampleInternal(ctx, fixReq)
		if err != nil {
			lastErr = err
			continue
		}

		fixedText := stripCodeFences(strings.TrimSpace(fixed.Content))
		if err := json.Unmarshal([]byte(fixedText), out); err == nil {
			// Validate against schema if provided
			if schemaJSON != "" {
				if err := validateJSONSchema(fixedText, schemaJSON); err == nil {
					resp.Content = fixedText
					return resp, nil
				}
				lastErr = err
			} else {
				resp.Content = fixedText
				return resp, nil
			}
		}
		lastErr = err
	}

	return resp, fmt.Errorf("failed to parse valid JSON after %d repair attempts: %v", 2, lastErr)
}

// SampleJSON is a simpler version without schema validation
func (c *Client) SampleJSON(
	ctx context.Context,
	req SamplingRequest,
	out interface{},
) (*SamplingResponse, error) {
	return c.SampleJSONWithSchema(ctx, req, out, "")
}

// extractJSONCandidate attempts to extract JSON from mixed content
func extractJSONCandidate(s string) string {
	// First, strip any markdown code fences
	text := stripCodeFences(s)

	// Find JSON object or array boundaries
	start := -1
	var openDelim, closeDelim rune

	// Look for object start
	if idx := strings.Index(text, "{"); idx >= 0 {
		start = idx
		openDelim, closeDelim = '{', '}'
	} else if idx := strings.Index(text, "["); idx >= 0 {
		// Look for array start
		start = idx
		openDelim, closeDelim = '[', ']'
	}

	if start == -1 {
		return text // No JSON delimiters found, return as-is
	}

	// Balance braces/brackets to find the complete JSON
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(text); i++ {
		ch := rune(text[i])

		// Handle string literals
		if ch == '"' && !escaped {
			inString = !inString
		}

		// Track escape sequences
		if ch == '\\' && !escaped {
			escaped = true
			continue
		}
		escaped = false

		// Only count delimiters outside of strings
		if !inString {
			if ch == openDelim {
				depth++
			} else if ch == closeDelim {
				depth--
				if depth == 0 {
					return text[start : i+1]
				}
			}
		}
	}

	// If we didn't find a closing delimiter, return from start to end
	return text[start:]
}

// stripCodeFences removes markdown code fences and language tags
func stripCodeFences(s string) string {
	// Remove ```json or ```JSON blocks
	patterns := []string{
		"```json", "```JSON", "```Json",
		"```javascript", "```js",
		"```", // Generic code fence
	}

	result := s
	for _, pattern := range patterns {
		if strings.Contains(result, pattern) {
			// Find opening fence
			start := strings.Index(result, pattern)
			if start >= 0 {
				// Find content after the fence
				afterFence := start + len(pattern)

				// Skip to next line if there's a newline
				if afterFence < len(result) && result[afterFence] == '\n' {
					afterFence++
				}

				// Find closing fence
				closeIdx := strings.Index(result[afterFence:], "```")
				if closeIdx >= 0 {
					// Extract content between fences
					content := result[afterFence : afterFence+closeIdx]
					result = strings.TrimSpace(content)
				} else {
					// No closing fence, take everything after opening
					result = strings.TrimSpace(result[afterFence:])
				}
			}
		}
	}

	return result
}

// buildJSONRepairPrompt creates a prompt to fix invalid JSON
func buildJSONRepairPrompt(schema string, invalidJSON string, lastErr error) string {
	prompt := fmt.Sprintf(`The following text should be valid JSON but has an error:

%s

Error: %v`, invalidJSON, lastErr)

	if schema != "" {
		prompt += fmt.Sprintf(`

The JSON must conform to this schema:
%s`, schema)
	}

	prompt += `

Please output ONLY the corrected valid JSON. No explanations, no code fences, just the JSON.`

	return prompt
}

// validateJSONSchema validates JSON against a schema
func validateJSONSchema(jsonStr, schemaStr string) error {
	if schemaStr == "" {
		return nil
	}

	// Parse the schema
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaStr)); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("schema compilation failed: %w", err)
	}

	// Parse the JSON
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate
	if err := schema.Validate(data); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

// minNonZero returns the minimum non-zero value
func minNonZero(a, b int32) int32 {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}
