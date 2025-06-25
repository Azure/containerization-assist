package orchestration

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/rs/zerolog"
)

// VariableResolver handles variable expansion and templating for workflows
type VariableResolver struct {
	logger zerolog.Logger
}

// NewVariableResolver creates a new variable resolver
func NewVariableResolver(logger zerolog.Logger) *VariableResolver {
	return &VariableResolver{
		logger: logger.With().Str("component", "variable_resolver").Logger(),
	}
}

// VariableContext represents the context for variable resolution
type VariableContext struct {
	WorkflowVars    map[string]string      // Workflow-level variables
	StageVars       map[string]string      // Stage-level variables
	SessionContext  map[string]interface{} // Runtime context
	EnvironmentVars map[string]string      // Environment variables
	Secrets         map[string]string      // Secret values (handled carefully)
}

// ResolveVariables expands variables in a string using ${var} syntax
func (vr *VariableResolver) ResolveVariables(input string, context *VariableContext) (string, error) {
	if input == "" {
		return input, nil
	}

	// Enhanced regex to support ${var}, ${var:-default}, and ${env:VAR} syntax
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	result := re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable expression (remove ${ and })
		expr := match[2 : len(match)-1]

		// Handle different variable types
		if strings.HasPrefix(expr, "env:") {
			// Environment variable: ${env:VAR_NAME}
			envKey := expr[4:]
			if value, exists := context.EnvironmentVars[envKey]; exists {
				return value
			}
			if value := os.Getenv(envKey); value != "" {
				return value
			}
			vr.logger.Warn().Str("env_var", envKey).Msg("Environment variable not found")
			return match // Return original if not found
		}

		if strings.HasPrefix(expr, "secret:") {
			// Secret reference: ${secret:SECRET_NAME}
			secretKey := expr[7:]
			if value, exists := context.Secrets[secretKey]; exists {
				return value
			}
			vr.logger.Warn().Str("secret", secretKey).Msg("Secret not found")
			return match
		}

		// Handle default values: ${var:-default}
		parts := strings.SplitN(expr, ":-", 2)
		varName := parts[0]
		defaultValue := ""
		if len(parts) > 1 {
			defaultValue = parts[1]
		}

		// Resolve variable with precedence: stage -> workflow -> session -> env
		if value := vr.resolveVariable(varName, context); value != "" {
			return value
		}

		// Return default value if provided
		if defaultValue != "" {
			return defaultValue
		}

		// Log warning for missing variable
		vr.logger.Warn().Str("variable", varName).Msg("Variable not found")
		return match // Return original if not found
	})

	return result, nil
}

// ResolveTemplate processes a string using Go template syntax with variable context
func (vr *VariableResolver) ResolveTemplate(templateStr string, context *VariableContext) (string, error) {
	if templateStr == "" {
		return templateStr, nil
	}

	// Create template with helper functions
	tmpl, err := template.New("workflow").Funcs(vr.getTemplateFunctions(context)).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare template data
	data := vr.buildTemplateData(context)

	// Execute template
	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return result.String(), nil
}

// ValidateVariables checks if all required variables are available
func (vr *VariableResolver) ValidateVariables(input string, context *VariableContext) []string {
	var missing []string

	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		if len(match) > 1 {
			expr := match[1]

			// Skip if it has a default value
			if strings.Contains(expr, ":-") {
				continue
			}

			// Skip environment variables (they can be optional)
			if strings.HasPrefix(expr, "env:") {
				continue
			}

			// Check if variable exists
			if vr.resolveVariable(expr, context) == "" {
				missing = append(missing, expr)
			}
		}
	}

	return missing
}

// Private helper methods

func (vr *VariableResolver) resolveVariable(name string, context *VariableContext) string {
	// Priority: stage variables -> workflow variables -> session context -> environment

	// 1. Stage-level variables (highest priority)
	if context.StageVars != nil {
		if value, exists := context.StageVars[name]; exists {
			return value
		}
	}

	// 2. Workflow-level variables
	if context.WorkflowVars != nil {
		if value, exists := context.WorkflowVars[name]; exists {
			return value
		}
	}

	// 3. Session context
	if context.SessionContext != nil {
		if value, exists := context.SessionContext[name]; exists {
			return fmt.Sprintf("%v", value)
		}
	}

	// 4. Environment variables (lowest priority)
	if context.EnvironmentVars != nil {
		if value, exists := context.EnvironmentVars[name]; exists {
			return value
		}
	}

	// 5. System environment variables
	if value := os.Getenv(name); value != "" {
		return value
	}

	return ""
}

func (vr *VariableResolver) getTemplateFunctions(context *VariableContext) template.FuncMap {
	return template.FuncMap{
		"var": func(name string) string {
			return vr.resolveVariable(name, context)
		},
		"varOrDefault": func(name, defaultValue string) string {
			if value := vr.resolveVariable(name, context); value != "" {
				return value
			}
			return defaultValue
		},
		"env": func(name string) string {
			if value := os.Getenv(name); value != "" {
				return value
			}
			return ""
		},
		"envOrDefault": func(name, defaultValue string) string {
			if value := os.Getenv(name); value != "" {
				return value
			}
			return defaultValue
		},
		"secret": func(name string) string {
			if context.Secrets != nil {
				if value, exists := context.Secrets[name]; exists {
					return value
				}
			}
			return ""
		},
		"required": func(name string) (string, error) {
			if value := vr.resolveVariable(name, context); value != "" {
				return value, nil
			}
			return "", fmt.Errorf("required variable '%s' not found", name)
		},
		"default": func(defaultValue, value string) string {
			if value != "" {
				return value
			}
			return defaultValue
		},
		"quote": func(s string) string {
			return fmt.Sprintf("\"%s\"", strings.ReplaceAll(s, "\"", "\\\""))
		},
	}
}

func (vr *VariableResolver) buildTemplateData(context *VariableContext) map[string]interface{} {
	data := make(map[string]interface{})

	// Add all variable contexts
	if context.WorkflowVars != nil {
		for k, v := range context.WorkflowVars {
			data[k] = v
		}
	}

	if context.StageVars != nil {
		for k, v := range context.StageVars {
			data[k] = v // Stage variables override workflow variables
		}
	}

	if context.SessionContext != nil {
		for k, v := range context.SessionContext {
			data[k] = v
		}
	}

	// Add helper contexts
	data["env"] = context.EnvironmentVars
	data["secrets"] = context.Secrets

	return data
}

// SecretRedactor ensures secrets are not logged
type SecretRedactor struct {
	secrets map[string]bool
}

// NewSecretRedactor creates a new secret redactor
func NewSecretRedactor(secretNames []string) *SecretRedactor {
	secrets := make(map[string]bool)
	for _, name := range secretNames {
		secrets[name] = true
	}
	return &SecretRedactor{secrets: secrets}
}

// RedactString replaces secret values with [REDACTED]
func (sr *SecretRedactor) RedactString(input string, secretValues map[string]string) string {
	result := input
	for _, value := range secretValues {
		if value != "" && len(value) > 0 {
			result = strings.ReplaceAll(result, value, "[REDACTED]")
		}
	}
	return result
}
