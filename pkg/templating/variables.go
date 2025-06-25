// Package templating provides variable templating functionality for configuration files
package templating

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/rs/zerolog"
)

// VariableTemplater handles variable substitution in templates
type VariableTemplater struct {
	variables        map[string]interface{}
	envPrefix        string
	allowEnvFallback bool
	strictMode       bool
	logger           zerolog.Logger
}

// NewVariableTemplater creates a new variable templater
func NewVariableTemplater(logger zerolog.Logger) *VariableTemplater {
	return &VariableTemplater{
		variables:        make(map[string]interface{}),
		envPrefix:        "CONTAINER_KIT_",
		allowEnvFallback: true,
		strictMode:       false,
		logger:           logger.With().Str("component", "variable_templater").Logger(),
	}
}

// TemplateOptions configures templating behavior
type TemplateOptions struct {
	// StrictMode fails on undefined variables
	StrictMode bool
	// AllowEnvFallback allows falling back to environment variables
	AllowEnvFallback bool
	// EnvPrefix is the prefix for environment variable lookups
	EnvPrefix string
	// CustomFunctions adds custom template functions
	CustomFunctions template.FuncMap
}

// SetOptions configures the templater
func (vt *VariableTemplater) SetOptions(opts TemplateOptions) {
	vt.strictMode = opts.StrictMode
	vt.allowEnvFallback = opts.AllowEnvFallback
	if opts.EnvPrefix != "" {
		vt.envPrefix = opts.EnvPrefix
	}
}

// SetVariable sets a single variable
func (vt *VariableTemplater) SetVariable(key string, value interface{}) {
	vt.variables[key] = value
}

// SetVariables sets multiple variables
func (vt *VariableTemplater) SetVariables(vars map[string]interface{}) {
	for k, v := range vars {
		vt.variables[k] = v
	}
}

// LoadVariablesFromEnv loads variables from environment with prefix
func (vt *VariableTemplater) LoadVariablesFromEnv() {
	environ := os.Environ()
	for _, env := range environ {
		if strings.HasPrefix(env, vt.envPrefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], vt.envPrefix)
				key = strings.ToLower(key)
				vt.variables[key] = parts[1]
			}
		}
	}
}

// ProcessTemplate processes a template string with variable substitution
func (vt *VariableTemplater) ProcessTemplate(templateContent string) (string, error) {
	// Create template with sprig functions
	tmpl := template.New("template").Funcs(vt.getTemplateFunctions())

	// Parse template
	parsed, err := tmpl.Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, vt.getTemplateData()); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ProcessFile processes a template file
func (vt *VariableTemplater) ProcessFile(inputPath, outputPath string) error {
	// Read input file
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Process template
	processed, err := vt.ProcessTemplate(string(content))
	if err != nil {
		return fmt.Errorf("failed to process template: %w", err)
	}

	// Write output file
	if err := os.WriteFile(outputPath, []byte(processed), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	vt.logger.Info().
		Str("input", inputPath).
		Str("output", outputPath).
		Msg("Successfully processed template file")

	return nil
}

// ProcessInPlace processes a file in place
func (vt *VariableTemplater) ProcessInPlace(filePath string) error {
	return vt.ProcessFile(filePath, filePath)
}

// getTemplateFunctions returns the template function map
func (vt *VariableTemplater) getTemplateFunctions() template.FuncMap {
	// Create function map with standard and custom functions
	funcs := template.FuncMap{
		// Variable resolution functions
		"var":          vt.getVariable,
		"varOrDefault": vt.getVariableOrDefault,
		"env":          vt.getEnvVariable,
		"envOrDefault": vt.getEnvVariableOrDefault,
		"required":     vt.requiredVariable,

		// Docker-specific functions
		"dockerBaseImage": vt.getDockerBaseImage,
		"dockerPort":      vt.getDockerPort,
		"dockerUser":      vt.getDockerUser,

		// Kubernetes-specific functions
		"k8sNamespace":       vt.getK8sNamespace,
		"k8sReplicas":        vt.getK8sReplicas,
		"k8sImagePullPolicy": vt.getK8sImagePullPolicy,

		// Utility functions
		"toYAML":  vt.toYAML,
		"toJSON":  vt.toJSON,
		"indent":  vt.indent,
		"include": vt.includeFile,
		"tpl":     vt.recursiveTemplate,

		// String functions
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"replace":    strings.ReplaceAll,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,

		// Logic functions
		"default": vt.defaultFunc,
		"empty":   vt.emptyFunc,
		"eq":      vt.eqFunc,
		"ne":      vt.neFunc,
		"lt":      vt.ltFunc,
		"le":      vt.leFunc,
		"gt":      vt.gtFunc,
		"ge":      vt.geFunc,
		"and":     vt.andFunc,
		"or":      vt.orFunc,
		"not":     vt.notFunc,
	}

	return funcs
}

// getTemplateData returns the data for template execution
func (vt *VariableTemplater) getTemplateData() interface{} {
	// Create a copy of variables to avoid mutations
	data := make(map[string]interface{})
	for k, v := range vt.variables {
		data[k] = v
	}

	// Add standard variables
	data["Variables"] = vt.variables
	data["Env"] = vt.getEnvMap()

	return data
}

// Variable resolution functions

func (vt *VariableTemplater) getVariable(key string) (interface{}, error) {
	if val, ok := vt.variables[key]; ok {
		return val, nil
	}

	// Try environment fallback
	if vt.allowEnvFallback {
		if envVal := os.Getenv(vt.envPrefix + strings.ToUpper(key)); envVal != "" {
			return envVal, nil
		}
	}

	if vt.strictMode {
		return nil, fmt.Errorf("variable not found: %s", key)
	}

	return "", nil
}

func (vt *VariableTemplater) getVariableOrDefault(key string, defaultValue interface{}) interface{} {
	val, err := vt.getVariable(key)
	if err != nil || val == "" {
		return defaultValue
	}
	return val
}

func (vt *VariableTemplater) getEnvVariable(key string) string {
	return os.Getenv(key)
}

func (vt *VariableTemplater) getEnvVariableOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func (vt *VariableTemplater) requiredVariable(key string) (interface{}, error) {
	val, err := vt.getVariable(key)
	if err != nil {
		return nil, fmt.Errorf("required variable not found: %s", key)
	}
	if val == "" {
		return nil, fmt.Errorf("required variable is empty: %s", key)
	}
	return val, nil
}

// Docker-specific functions

func (vt *VariableTemplater) getDockerBaseImage(language string) string {
	// Check for custom base image
	if img, err := vt.getVariable("docker_base_image"); err == nil && img != "" {
		return img.(string)
	}

	// Default base images by language
	defaults := map[string]string{
		"python": "python:3.11-slim",
		"node":   "node:18-alpine",
		"go":     "golang:1.21-alpine",
		"java":   "openjdk:17-slim",
		"dotnet": "mcr.microsoft.com/dotnet/sdk:7.0",
		"ruby":   "ruby:3.2-slim",
		"php":    "php:8.2-fpm-alpine",
	}

	if baseImage, ok := defaults[strings.ToLower(language)]; ok {
		return baseImage
	}

	return "ubuntu:22.04"
}

func (vt *VariableTemplater) getDockerPort() string {
	if port, err := vt.getVariable("port"); err == nil && port != "" {
		return fmt.Sprintf("%v", port)
	}
	return "8080"
}

func (vt *VariableTemplater) getDockerUser() string {
	if user, err := vt.getVariable("docker_user"); err == nil && user != "" {
		return user.(string)
	}
	return "appuser"
}

// Kubernetes-specific functions

func (vt *VariableTemplater) getK8sNamespace() string {
	if ns, err := vt.getVariable("namespace"); err == nil && ns != "" {
		return ns.(string)
	}
	return "default"
}

func (vt *VariableTemplater) getK8sReplicas() int {
	if replicas, err := vt.getVariable("replicas"); err == nil && replicas != "" {
		switch v := replicas.(type) {
		case int:
			return v
		case string:
			if r, err := fmt.Sscanf(v, "%d", &replicas); err == nil && r == 1 {
				return replicas.(int)
			}
		}
	}
	return 1
}

func (vt *VariableTemplater) getK8sImagePullPolicy() string {
	if policy, err := vt.getVariable("image_pull_policy"); err == nil && policy != "" {
		return policy.(string)
	}
	return "IfNotPresent"
}

// Utility functions

func (vt *VariableTemplater) toYAML(data interface{}) (string, error) {
	// Simple YAML conversion
	return fmt.Sprintf("%v", data), nil
}

func (vt *VariableTemplater) toJSON(data interface{}) (string, error) {
	// Simple JSON conversion
	return fmt.Sprintf("%v", data), nil
}

func (vt *VariableTemplater) indent(spaces int, text string) string {
	padding := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	for i := range lines {
		if lines[i] != "" {
			lines[i] = padding + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func (vt *VariableTemplater) includeFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to include file: %w", err)
	}
	return string(content), nil
}

func (vt *VariableTemplater) recursiveTemplate(templateStr string) (string, error) {
	// Process nested template
	return vt.ProcessTemplate(templateStr)
}

func (vt *VariableTemplater) getEnvMap() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// ValidateTemplate validates a template without executing it
func (vt *VariableTemplater) ValidateTemplate(templateContent string) error {
	tmpl := template.New("validate").Funcs(vt.getTemplateFunctions())
	_, err := tmpl.Parse(templateContent)
	return err
}

// ExtractVariables extracts all variable references from a template
func (vt *VariableTemplater) ExtractVariables(templateContent string) ([]string, error) {
	// Regular expressions to match different variable patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`{{\s*\.(\w+)\s*}}`),            // {{.varName}}
		regexp.MustCompile(`{{\s*var\s+"(\w+)"\s*}}`),      // {{var "varName"}}
		regexp.MustCompile(`{{\s*varOrDefault\s+"(\w+)"`),  // {{varOrDefault "varName"...
		regexp.MustCompile(`{{\s*required\s+"(\w+)"\s*}}`), // {{required "varName"}}
		regexp.MustCompile(`{{\s*env\s+"(\w+)"\s*}}`),      // {{env "varName"}}
		regexp.MustCompile(`{{\s*envOrDefault\s+"(\w+)"`),  // {{envOrDefault "varName"...
	}

	varMap := make(map[string]bool)

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(templateContent, -1)
		for _, match := range matches {
			if len(match) > 1 {
				varMap[match[1]] = true
			}
		}
	}

	// Convert map to slice
	vars := make([]string, 0, len(varMap))
	for v := range varMap {
		vars = append(vars, v)
	}

	return vars, nil
}

// TemplateResult represents the result of template processing
type TemplateResult struct {
	Content          string
	Variables        map[string]interface{}
	UsedVariables    []string
	MissingVariables []string
	Warnings         []string
}

// ProcessWithValidation processes a template with validation and reporting
func (vt *VariableTemplater) ProcessWithValidation(templateContent string) (*TemplateResult, error) {
	result := &TemplateResult{
		Variables:        vt.variables,
		UsedVariables:    []string{},
		MissingVariables: []string{},
		Warnings:         []string{},
	}

	// Extract variables
	vars, err := vt.ExtractVariables(templateContent)
	if err != nil {
		return result, fmt.Errorf("failed to extract variables: %w", err)
	}

	// Check for missing variables
	for _, v := range vars {
		if _, ok := vt.variables[v]; !ok {
			if vt.allowEnvFallback {
				if envVal := os.Getenv(vt.envPrefix + strings.ToUpper(v)); envVal == "" {
					result.MissingVariables = append(result.MissingVariables, v)
					result.Warnings = append(result.Warnings, fmt.Sprintf("Variable '%s' not found, using empty string", v))
				}
			} else {
				result.MissingVariables = append(result.MissingVariables, v)
			}
		} else {
			result.UsedVariables = append(result.UsedVariables, v)
		}
	}

	// Process template
	processed, err := vt.ProcessTemplate(templateContent)
	if err != nil {
		return result, err
	}

	result.Content = processed
	return result, nil
}

// Logic helper functions

func (vt *VariableTemplater) defaultFunc(defaultVal, val interface{}) interface{} {
	if vt.emptyFunc(val) {
		return defaultVal
	}
	return val
}

func (vt *VariableTemplater) emptyFunc(val interface{}) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return v == ""
	case int, int64, float64:
		return v == 0
	case bool:
		return !v
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

func (vt *VariableTemplater) eqFunc(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func (vt *VariableTemplater) neFunc(a, b interface{}) bool {
	return !vt.eqFunc(a, b)
}

func (vt *VariableTemplater) ltFunc(a, b interface{}) bool {
	aFloat, aErr := vt.toFloat64(a)
	bFloat, bErr := vt.toFloat64(b)
	if aErr == nil && bErr == nil {
		return aFloat < bFloat
	}
	return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
}

func (vt *VariableTemplater) leFunc(a, b interface{}) bool {
	return vt.ltFunc(a, b) || vt.eqFunc(a, b)
}

func (vt *VariableTemplater) gtFunc(a, b interface{}) bool {
	return !vt.leFunc(a, b)
}

func (vt *VariableTemplater) geFunc(a, b interface{}) bool {
	return !vt.ltFunc(a, b)
}

func (vt *VariableTemplater) andFunc(vals ...interface{}) bool {
	for _, val := range vals {
		if vt.emptyFunc(val) {
			return false
		}
		if b, ok := val.(bool); ok && !b {
			return false
		}
	}
	return true
}

func (vt *VariableTemplater) orFunc(vals ...interface{}) bool {
	for _, val := range vals {
		if !vt.emptyFunc(val) {
			if b, ok := val.(bool); !ok || b {
				return true
			}
		}
	}
	return false
}

func (vt *VariableTemplater) notFunc(val interface{}) bool {
	if b, ok := val.(bool); ok {
		return !b
	}
	return vt.emptyFunc(val)
}

func (vt *VariableTemplater) toFloat64(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		var f float64
		_, err := fmt.Sscanf(v, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", val)
	}
}
