// Package azure provides validation for Azure Container Apps manifests
package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// ValidateAzureManifests validates Bicep or ARM templates
func ValidateAzureManifests(
	ctx context.Context,
	manifestPath string,
	manifestType string, // "bicep" or "arm"
	strictMode bool,
	logger *slog.Logger,
) (*ValidationResult, error) {
	// Check if file exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("manifest file does not exist: %s", manifestPath)
	}

	// Read file content
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %v", err)
	}

	switch strings.ToLower(manifestType) {
	case "bicep":
		return validateBicepTemplate(ctx, manifestPath, string(content), strictMode, logger)
	case "arm":
		return validateARMTemplate(ctx, manifestPath, string(content), strictMode, logger)
	default:
		return nil, fmt.Errorf("unknown manifest type: %s", manifestType)
	}
}

// validateBicepTemplate validates a Bicep template
func validateBicepTemplate(ctx context.Context, path string, content string, strict bool, logger *slog.Logger) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Metadata: make(map[string]interface{}),
	}

	// Basic syntax validation
	if !strings.Contains(content, "resource") {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "No resource definitions found in Bicep template",
			Severity: "error",
			Rule:     "bicep-resource-required",
		})
	}

	if !strings.Contains(content, "Microsoft.App/containerApps") {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "No Container Apps resource found in Bicep template",
			Severity: "error",
			Rule:     "bicep-containerapp-required",
		})
	}

	// Check if Azure CLI is available for advanced validation
	if _, err := exec.LookPath("az"); err == nil {
		// Try to validate using Azure CLI
		cmd := exec.CommandContext(ctx, "az", "bicep", "build", "--file", path, "--stdout")
		output, err := cmd.CombinedOutput()

		if err != nil {
			// Parse Bicep errors from output
			errors := parseBicepErrors(string(output))
			for _, e := range errors {
				result.Valid = false
				result.Errors = append(result.Errors, e)
			}
		} else {
			if logger != nil {
				logger.Info("Bicep template validated successfully with Azure CLI")
			}
		}
	} else {
		// Azure CLI not available, add warning
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: "Azure CLI not available for advanced Bicep validation",
			Rule:    "azure-cli-unavailable",
		})
	}

	// Strict mode validation
	if strict {
		performStrictBicepValidation(content, result)
	}

	result.Metadata["templateType"] = "bicep"
	result.Metadata["filePath"] = path

	return result, nil
}

// validateARMTemplate validates an ARM template
func validateARMTemplate(ctx context.Context, path string, content string, strict bool, logger *slog.Logger) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Metadata: make(map[string]interface{}),
	}

	// Parse JSON structure
	var template map[string]interface{}
	if err := json.Unmarshal([]byte(content), &template); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  fmt.Sprintf("Invalid JSON in ARM template: %v", err),
			Severity: "critical",
			Rule:     "arm-json-invalid",
		})
		return result, nil
	}

	// Check required fields
	if _, ok := template["$schema"]; !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "Missing $schema in ARM template",
			Severity: "error",
			Rule:     "arm-schema-required",
		})
	}

	if _, ok := template["contentVersion"]; !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "Missing contentVersion in ARM template",
			Severity: "error",
			Rule:     "arm-version-required",
		})
	}

	resources, ok := template["resources"].([]interface{})
	if !ok || len(resources) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "Missing or empty resources section in ARM template",
			Severity: "error",
			Rule:     "arm-resources-required",
		})
	} else {
		// Check for Container Apps resource
		hasContainerApp := false
		for _, resource := range resources {
			if res, ok := resource.(map[string]interface{}); ok {
				if resType, ok := res["type"].(string); ok && resType == "Microsoft.App/containerApps" {
					hasContainerApp = true
					// Validate Container App resource structure
					validateContainerAppResource(res, result)
					break
				}
			}
		}

		if !hasContainerApp {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Message:  "No Container Apps resource found in ARM template",
				Severity: "error",
				Rule:     "arm-containerapp-required",
			})
		}
	}

	// Check parameters if present
	if params, ok := template["parameters"].(map[string]interface{}); ok {
		validateARMParameters(params, result)
	}

	// Strict mode validation
	if strict {
		performStrictARMValidation(template, result)
	}

	result.Metadata["templateType"] = "arm"
	result.Metadata["filePath"] = path

	return result, nil
}

// parseBicepErrors parses error output from Azure CLI bicep build
func parseBicepErrors(output string) []ValidationError {
	errors := []ValidationError{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Error") || strings.Contains(line, "ERROR") {
			// Try to extract line and column information
			var lineNum, colNum int
			if n, _ := fmt.Sscanf(line, "(%d,%d)", &lineNum, &colNum); n == 2 {
				errors = append(errors, ValidationError{
					Line:     lineNum,
					Column:   colNum,
					Message:  line,
					Severity: "error",
					Rule:     "bicep-syntax-error",
				})
			} else {
				errors = append(errors, ValidationError{
					Message:  line,
					Severity: "error",
					Rule:     "bicep-error",
				})
			}
		}
	}

	return errors
}

// validateContainerAppResource validates the structure of a Container App resource
func validateContainerAppResource(resource map[string]interface{}, result *ValidationResult) {
	// Check for required properties
	props, ok := resource["properties"].(map[string]interface{})
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "Container App resource missing properties",
			Severity: "error",
			Rule:     "containerapp-properties-required",
		})
		return
	}

	// Check for managedEnvironmentId
	if _, ok := props["managedEnvironmentId"]; !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "Container App missing managedEnvironmentId",
			Severity: "error",
			Rule:     "containerapp-environment-required",
		})
	}

	// Check template section
	if tmpl, ok := props["template"].(map[string]interface{}); ok {
		// Check containers
		if containers, ok := tmpl["containers"].([]interface{}); ok {
			if len(containers) == 0 {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Message:  "Container App template has no containers defined",
					Severity: "error",
					Rule:     "containerapp-containers-required",
				})
			} else {
				// Validate each container
				for i, container := range containers {
					if cont, ok := container.(map[string]interface{}); ok {
						validateContainer(cont, i, result)
					}
				}
			}
		} else {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Message:  "Container App template missing containers array",
				Severity: "error",
				Rule:     "containerapp-containers-required",
			})
		}

		// Check scale configuration
		if scale, ok := tmpl["scale"].(map[string]interface{}); ok {
			validateScaleConfiguration(scale, result)
		}
	} else {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  "Container App missing template section",
			Severity: "error",
			Rule:     "containerapp-template-required",
		})
	}

	// Check configuration if present
	if config, ok := props["configuration"].(map[string]interface{}); ok {
		validateConfiguration(config, result)
	}
}

// validateContainer validates a container definition
func validateContainer(container map[string]interface{}, index int, result *ValidationResult) {
	// Check required fields
	if _, ok := container["name"]; !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  fmt.Sprintf("Container %d missing name", index),
			Severity: "error",
			Rule:     "container-name-required",
		})
	}

	if _, ok := container["image"]; !ok {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:  fmt.Sprintf("Container %d missing image", index),
			Severity: "error",
			Rule:     "container-image-required",
		})
	}

	// Check resources
	if resources, ok := container["resources"].(map[string]interface{}); ok {
		validateContainerResources(resources, index, result)
	}
}

// validateContainerResources validates container resource specifications
func validateContainerResources(resources map[string]interface{}, containerIndex int, result *ValidationResult) {
	// Check CPU
	if cpu, ok := resources["cpu"]; ok {
		if cpuVal, ok := cpu.(float64); ok {
			if cpuVal <= 0 || cpuVal > 4 {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Message: fmt.Sprintf("Container %d CPU value %g may be outside typical range (0.25-4.0)", containerIndex, cpuVal),
					Rule:    "container-cpu-range",
				})
			}
		}
	}

	// Check memory
	if memory, ok := resources["memory"].(string); ok {
		if !strings.HasSuffix(memory, "Gi") && !strings.HasSuffix(memory, "Mi") {
			result.Errors = append(result.Errors, ValidationError{
				Message:  fmt.Sprintf("Container %d memory format invalid (should end with Gi or Mi)", containerIndex),
				Severity: "error",
				Rule:     "container-memory-format",
			})
		}
	}
}

// validateScaleConfiguration validates scale settings
func validateScaleConfiguration(scale map[string]interface{}, result *ValidationResult) {
	minReplicas := 0
	maxReplicas := 0

	if min, ok := scale["minReplicas"].(float64); ok {
		minReplicas = int(min)
		if minReplicas < 0 {
			result.Errors = append(result.Errors, ValidationError{
				Message:  "minReplicas cannot be negative",
				Severity: "error",
				Rule:     "scale-min-replicas-range",
			})
		}
	}

	if max, ok := scale["maxReplicas"].(float64); ok {
		maxReplicas = int(max)
		if maxReplicas < 1 {
			result.Errors = append(result.Errors, ValidationError{
				Message:  "maxReplicas must be at least 1",
				Severity: "error",
				Rule:     "scale-max-replicas-range",
			})
		}
		if maxReplicas > 300 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Message: "maxReplicas exceeds typical limit of 300",
				Rule:    "scale-max-replicas-limit",
			})
		}
	}

	if minReplicas > maxReplicas && maxReplicas > 0 {
		result.Errors = append(result.Errors, ValidationError{
			Message:  "minReplicas cannot be greater than maxReplicas",
			Severity: "error",
			Rule:     "scale-replicas-consistency",
		})
	}
}

// validateConfiguration validates the configuration section
func validateConfiguration(config map[string]interface{}, result *ValidationResult) {
	// Check ingress if present
	if ingress, ok := config["ingress"].(map[string]interface{}); ok {
		if targetPort, ok := ingress["targetPort"]; ok {
			if port, ok := targetPort.(float64); ok {
				if port <= 0 || port > 65535 {
					result.Errors = append(result.Errors, ValidationError{
						Message:  fmt.Sprintf("Invalid target port: %g", port),
						Severity: "error",
						Rule:     "ingress-port-range",
					})
				}
			}
		}
	}

	// Check Dapr if present
	if dapr, ok := config["dapr"].(map[string]interface{}); ok {
		if enabled, ok := dapr["enabled"].(bool); ok && enabled {
			if _, hasAppId := dapr["appId"]; !hasAppId {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Message: "Dapr enabled but no appId specified",
					Rule:    "dapr-appid-recommended",
				})
			}
		}
	}
}

// validateARMParameters validates ARM template parameters
func validateARMParameters(params map[string]interface{}, result *ValidationResult) {
	for name, param := range params {
		if p, ok := param.(map[string]interface{}); ok {
			// Check for type
			if _, hasType := p["type"]; !hasType {
				result.Errors = append(result.Errors, ValidationError{
					Message:  fmt.Sprintf("Parameter '%s' missing type", name),
					Severity: "error",
					Rule:     "parameter-type-required",
				})
			}
		}
	}
}

// performStrictBicepValidation performs additional strict validation for Bicep
func performStrictBicepValidation(content string, result *ValidationResult) {
	// Check for required tags
	if !strings.Contains(content, "tags:") && !strings.Contains(content, "tags =") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: "No tags defined in Bicep template (recommended for resource management)",
			Rule:    "strict-tags-recommended",
		})
	}

	// Check for outputs
	if !strings.Contains(content, "output ") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: "No outputs defined in Bicep template",
			Rule:    "strict-outputs-recommended",
		})
	}

	// Check for proper parameter usage
	if strings.Contains(content, "'eastus'") || strings.Contains(content, "'westus'") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: "Hardcoded location found - consider using parameters",
			Rule:    "strict-no-hardcoded-location",
		})
	}
}

// performStrictARMValidation performs additional strict validation for ARM templates
func performStrictARMValidation(template map[string]interface{}, result *ValidationResult) {
	// Check for tags on resources
	if resources, ok := template["resources"].([]interface{}); ok {
		for _, resource := range resources {
			if res, ok := resource.(map[string]interface{}); ok {
				if _, hasTags := res["tags"]; !hasTags {
					resType, _ := res["type"].(string)
					resName, _ := res["name"].(string)
					result.Warnings = append(result.Warnings, ValidationWarning{
						Message: fmt.Sprintf("Resource '%s' (%s) has no tags", resName, resType),
						Rule:    "strict-resource-tags-recommended",
					})
				}
			}
		}
	}

	// Check for outputs
	if _, hasOutputs := template["outputs"]; !hasOutputs {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Message: "No outputs defined in ARM template",
			Rule:    "strict-outputs-recommended",
		})
	}

	// Check API versions
	if resources, ok := template["resources"].([]interface{}); ok {
		for _, resource := range resources {
			if res, ok := resource.(map[string]interface{}); ok {
				if apiVersion, ok := res["apiVersion"].(string); ok {
					// Check if using old API version - be specific about versions
					// For Container Apps, versions before 2023-05-01 are considered old
					isOldVersion := false
					switch {
					case strings.HasPrefix(apiVersion, "Microsoft.App/"):
						// Container Apps specific versions
						if apiVersion == "Microsoft.App/containerApps/2022-03-01" ||
							apiVersion == "Microsoft.App/containerApps/2022-11-01-preview" ||
							apiVersion == "Microsoft.App/managedEnvironments/2022-03-01" ||
							apiVersion == "Microsoft.App/managedEnvironments/2022-11-01-preview" ||
							apiVersion == "Microsoft.App/containerApps/2023-01-01-preview" {
							isOldVersion = true
						}
					case strings.Contains(apiVersion, "/2022-"):
						// Any 2022 version for other resources
						isOldVersion = true
					case apiVersion == "2023-01-01" || apiVersion == "2023-01-01-preview":
						// Specific old 2023 versions
						isOldVersion = true
					}

					if isOldVersion {
						result.Warnings = append(result.Warnings, ValidationWarning{
							Message: fmt.Sprintf("Resource using older API version: %s. Consider updating to latest stable version.", apiVersion),
							Rule:    "strict-api-version-latest",
						})
					}
				}
			}
		}
	}
}
