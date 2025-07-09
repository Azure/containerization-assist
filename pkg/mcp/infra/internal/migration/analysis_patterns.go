package migration

import (
	"regexp"
	"strings"
)

// initializePatterns sets up regex patterns for migration detection
func (md *Detector) initializePatterns() {
	md.patterns = map[string]*regexp.Regexp{
		// Interface patterns
		"duplicate_interface": regexp.MustCompile(`type\s+(\w+)\s+interface\s*{[^}]+}`),
		"empty_interface":     regexp.MustCompile(`type\s+(\w+)\s+interface\s*{\s*}`),
		"large_interface":     regexp.MustCompile(`type\s+(\w+)\s+interface\s*{(?:[^}]+\n){10,}}`),

		// Error handling patterns
		"error_ignore":      regexp.MustCompile(`_\s*(?:,\s*_\s*)?:?=.*(?:err|error)`),
		"panic_usage":       regexp.MustCompile(`panic\s*\(`),
		"bare_error_return": regexp.MustCompile(`return\s+(?:fmt\.)?Errorf?\s*\(`),

		// Type assertion patterns
		"unsafe_type_assertion": regexp.MustCompile(`(\w+)\s*:?=\s*(\w+)\.\([\w\.\*]+\)`),
		"interface_conversion":  regexp.MustCompile(`interface{}`),

		// Resource management patterns
		"missing_defer": regexp.MustCompile(`(?:Close|Unlock|Done)\s*\(\s*\)`),
		"defer_in_loop": regexp.MustCompile(`for\s+.*\{[^}]*defer[^}]*}`),

		// Concurrency patterns
		"unbuffered_channel": regexp.MustCompile(`make\s*\(\s*chan\s+[\w\.\*\[\]]+\s*\)`),
		"goroutine_leak":     regexp.MustCompile(`go\s+func\s*\(`),

		// Code smell patterns
		"long_function":   regexp.MustCompile(`func\s+(?:\([^)]+\)\s+)?(\w+)\s*\([^)]*\)[^{]*\{`),
		"deep_nesting":    regexp.MustCompile(`(?:\t{4,}|\s{16,})`),
		"magic_number":    regexp.MustCompile(`[^a-zA-Z0-9](\d{2,})[^a-zA-Z0-9]`),
		"global_variable": regexp.MustCompile(`var\s+[A-Z]\w*\s+`),
	}

	// Add custom patterns from config
	for name, pattern := range md.config.CustomPatterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			md.patterns[name] = compiled
		}
	}
}

// analyzePatterns searches for migration patterns in file content
func (md *Detector) analyzePatterns(content, filePath string) []Opportunity {
	var opportunities []Opportunity

	lines := strings.Split(content, "\n")

	for patternName, pattern := range md.patterns {
		matches := pattern.FindAllStringSubmatchIndex(content, -1)

		for _, match := range matches {
			lineNum := strings.Count(content[:match[0]], "\n") + 1

			opportunity := Opportunity{
				Type:            patternName,
				Priority:        md.getPriorityForPattern(patternName),
				Confidence:      md.getConfidenceForPattern(patternName),
				File:            filePath,
				Line:            lineNum,
				Column:          match[0] - strings.LastIndex(content[:match[0]], "\n") - 1,
				Description:     md.getDescriptionForPattern(patternName),
				Suggestion:      md.getSuggestionForPattern(patternName),
				EstimatedEffort: md.getEffortForPattern(patternName),
				Context: map[string]interface{}{
					"matched_text": content[match[0]:match[1]],
					"line_content": getLineContent(lines, lineNum),
				},
			}

			// Add examples for certain patterns
			if examples := md.getExamplesForPattern(patternName); len(examples) > 0 {
				opportunity.Examples = examples
			}

			opportunities = append(opportunities, opportunity)
		}
	}

	return opportunities
}

// Pattern metadata functions

func (md *Detector) getPriorityForPattern(pattern string) string {
	priorities := map[string]string{
		"duplicate_interface":   "HIGH",
		"empty_interface":       "MEDIUM",
		"large_interface":       "HIGH",
		"error_ignore":          "HIGH",
		"panic_usage":           "MEDIUM",
		"bare_error_return":     "LOW",
		"unsafe_type_assertion": "HIGH",
		"interface_conversion":  "MEDIUM",
		"missing_defer":         "HIGH",
		"defer_in_loop":         "HIGH",
		"unbuffered_channel":    "LOW",
		"goroutine_leak":        "HIGH",
		"long_function":         "MEDIUM",
		"deep_nesting":          "MEDIUM",
		"magic_number":          "LOW",
		"global_variable":       "MEDIUM",
	}

	if priority, exists := priorities[pattern]; exists {
		return priority
	}
	return "MEDIUM"
}

func (md *Detector) getConfidenceForPattern(pattern string) float64 {
	confidence := map[string]float64{
		"duplicate_interface":   0.9,
		"empty_interface":       1.0,
		"large_interface":       0.8,
		"error_ignore":          0.95,
		"panic_usage":           0.9,
		"bare_error_return":     0.7,
		"unsafe_type_assertion": 0.85,
		"interface_conversion":  0.8,
		"missing_defer":         0.7,
		"defer_in_loop":         0.95,
		"unbuffered_channel":    0.6,
		"goroutine_leak":        0.75,
		"long_function":         0.8,
		"deep_nesting":          0.7,
		"magic_number":          0.6,
		"global_variable":       0.85,
	}

	if conf, exists := confidence[pattern]; exists {
		return conf
	}
	return 0.5
}

func (md *Detector) getDescriptionForPattern(pattern string) string {
	descriptions := map[string]string{
		"duplicate_interface":   "Duplicate interface definition detected",
		"empty_interface":       "Empty interface definition found",
		"large_interface":       "Interface with too many methods (>10)",
		"error_ignore":          "Error is being ignored or discarded",
		"panic_usage":           "Direct panic usage found - consider returning error",
		"bare_error_return":     "Error returned without wrapping context",
		"unsafe_type_assertion": "Type assertion without checking success",
		"interface_conversion":  "Usage of interface{} - consider using specific types",
		"missing_defer":         "Resource cleanup without defer statement",
		"defer_in_loop":         "Defer statement inside loop - may cause resource leak",
		"unbuffered_channel":    "Unbuffered channel creation - may cause goroutine blocking",
		"goroutine_leak":        "Goroutine creation without proper lifecycle management",
		"long_function":         "Function exceeds recommended length",
		"deep_nesting":          "Deep nesting detected - consider refactoring",
		"magic_number":          "Magic number detected - use named constant",
		"global_variable":       "Global variable detected - consider dependency injection",
	}

	if desc, exists := descriptions[pattern]; exists {
		return desc
	}
	return "Migration opportunity detected"
}

func (md *Detector) getSuggestionForPattern(pattern string) string {
	suggestions := map[string]string{
		"duplicate_interface":   "Consolidate duplicate interfaces into a single definition",
		"empty_interface":       "Remove empty interface or add meaningful methods",
		"large_interface":       "Split interface into smaller, focused interfaces (ISP)",
		"error_ignore":          "Handle errors appropriately or explicitly document why ignored",
		"panic_usage":           "Return error instead of panic for recoverable errors",
		"bare_error_return":     "Wrap errors with context using fmt.Errorf or errors.Wrap",
		"unsafe_type_assertion": "Use comma-ok idiom: value, ok := x.(Type)",
		"interface_conversion":  "Use specific types instead of interface{}",
		"missing_defer":         "Add defer statement immediately after resource acquisition",
		"defer_in_loop":         "Move defer outside loop or use explicit cleanup",
		"unbuffered_channel":    "Consider using buffered channel if appropriate",
		"goroutine_leak":        "Ensure goroutine has proper termination condition",
		"long_function":         "Break down into smaller, focused functions",
		"deep_nesting":          "Extract nested logic into separate functions",
		"magic_number":          "Define as named constant with meaningful name",
		"global_variable":       "Pass as parameter or use dependency injection",
	}

	if suggestion, exists := suggestions[pattern]; exists {
		return suggestion
	}
	return "Consider refactoring this code"
}

func (md *Detector) getEffortForPattern(pattern string) string {
	efforts := map[string]string{
		"duplicate_interface":   "MINOR",
		"empty_interface":       "TRIVIAL",
		"large_interface":       "MAJOR",
		"error_ignore":          "MINOR",
		"panic_usage":           "MINOR",
		"bare_error_return":     "TRIVIAL",
		"unsafe_type_assertion": "TRIVIAL",
		"interface_conversion":  "MAJOR",
		"missing_defer":         "TRIVIAL",
		"defer_in_loop":         "MINOR",
		"unbuffered_channel":    "TRIVIAL",
		"goroutine_leak":        "MAJOR",
		"long_function":         "MAJOR",
		"deep_nesting":          "MAJOR",
		"magic_number":          "TRIVIAL",
		"global_variable":       "MINOR",
	}

	if effort, exists := efforts[pattern]; exists {
		return effort
	}
	return "MINOR"
}

func (md *Detector) getExamplesForPattern(pattern string) []CodeExample {
	examples := map[string][]CodeExample{
		"unsafe_type_assertion": {
			{
				Title:  "Safe type assertion",
				Before: `value := data.(string)`,
				After: `value, ok := data.(string)
if !ok {
    return fmt.Errorf("expected string, got %T", data)
}`,
			},
		},
		"error_ignore": {
			{
				Title:  "Proper error handling",
				Before: `result, _ := someFunction()`,
				After: `result, err := someFunction()
if err != nil {
    return fmt.Errorf("failed to call someFunction: %w", err)
}`,
			},
		},
		"missing_defer": {
			{
				Title: "Using defer for cleanup",
				Before: `file, err := os.Open("file.txt")
// ... code ...
file.Close()`,
				After: `file, err := os.Open("file.txt")
if err != nil {
    return err
}
defer file.Close()`,
			},
		},
	}

	if exs, exists := examples[pattern]; exists {
		return exs
	}
	return nil
}

// Helper function to get line content
func getLineContent(lines []string, lineNum int) string {
	if lineNum > 0 && lineNum <= len(lines) {
		return strings.TrimSpace(lines[lineNum-1])
	}
	return ""
}
