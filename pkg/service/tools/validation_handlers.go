package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"log/slog"

	"gopkg.in/yaml.v3"

	"github.com/Azure/containerization-assist/pkg/domain/validation"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterValidationTools registers all validator and apply tools
func RegisterValidationTools(mcpServer *server.MCPServer, deps ToolDependencies) error {
	tools := []ToolConfig{
		// Dockerfile tools
		{
			Name:           "validate_dockerfile",
			Description:    "Validate a Dockerfile draft produced by the host AI; returns findings and suggestions",
			Category:       CategoryWorkflow,
			RequiredParams: []string{"content"},
			NeedsLogger:    true,
			CustomHandler: func(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return createValidateDockerfileHandler(deps)
			},
			NextTool:    "apply_dockerfile",
			ChainReason: "Validation passed, apply the Dockerfile",
		},
		{
			Name:                "apply_dockerfile",
			Description:         "Write validated Dockerfile to repo and update workflow artifacts",
			Category:            CategoryWorkflow,
			RequiredParams:      []string{"session_id", "repo_path", "content"},
			OptionalParams:      map[string]interface{}{"path": "Dockerfile", "dry_run": false},
			NeedsLogger:         true,
			NeedsSessionManager: true,
			CustomHandler: func(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return createApplyDockerfileHandler(deps)
			},
			NextTool:    "build_image",
			ChainReason: "Dockerfile applied, build the container image",
		},

		// Kubernetes tools
		{
			Name:           "validate_k8s_manifests",
			Description:    "Validate Kubernetes manifest YAML produced by the host AI",
			Category:       CategoryWorkflow,
			RequiredParams: []string{"content"},
			NeedsLogger:    true,
			CustomHandler: func(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return createValidateManifestsHandler(deps)
			},
			NextTool:    "apply_k8s_manifests",
			ChainReason: "Validation passed, apply the manifests",
		},
		{
			Name:                "apply_k8s_manifests",
			Description:         "Write validated K8s manifests to repo and update workflow artifacts",
			Category:            CategoryWorkflow,
			RequiredParams:      []string{"session_id", "repo_path", "path", "content"},
			OptionalParams:      map[string]interface{}{"dry_run": false},
			NeedsLogger:         true,
			NeedsSessionManager: true,
			CustomHandler: func(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return createApplyManifestsHandler(deps)
			},
			NextTool:    "prepare_cluster",
			ChainReason: "Manifests applied, prepare the cluster",
		},
	}

	for _, cfg := range tools {
		if err := RegisterTool(mcpServer, cfg, deps); err != nil {
			return fmt.Errorf("register %s: %w", cfg.Name, err)
		}
	}

	return nil
}

// createValidateDockerfileHandler creates the Dockerfile validation handler
func createValidateDockerfileHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := deps.Logger
		if logger == nil {
			logger = slog.Default()
		}

		args := req.GetArguments()
		content, ok := args["content"].(string)
		if !ok || content == "" {
			err := fmt.Errorf("missing or empty 'content' parameter")
			result := createErrorResult(err)
			return &result, nil
		}

		logger.Info("validating Dockerfile",
			"content_length", len(content))

		// Build unified validation result
		result := validation.NewResult()
		result.Stats["lines"] = strings.Count(content, "\n") + 1
		result.Stats["size"] = len(content)

		// Perform validation checks
		validateDockerfileSyntax(content, result)
		validateDockerfileSecurity(content, result)
		validateDockerfileBestPractices(content, result)

		// Calculate quality score
		result.CalculateQualityScore()

		// Log validation metrics
		logger.Info("Dockerfile validation complete",
			"is_valid", result.IsValid,
			"errors", result.ErrorCount(),
			"warnings", result.WarningCount(),
			"quality_score", result.QualityScore)

		// Validation complete - prepare response

		// Set appropriate chain hint
		var chainHint *ChainHint
		if result.IsValid {
			chainHint = &ChainHint{
				NextTool: "apply_dockerfile",
				Reason:   "Validation passed. Apply the Dockerfile.",
			}
		} else {
			chainHint = &ChainHint{
				NextTool: "dockerfile-critique",
				Reason:   fmt.Sprintf("%d errors found. Use critique prompt to fix.", result.ErrorCount()),
			}
		}

		toolResult := createToolResult(result.IsValid, map[string]interface{}{
			"is_valid":      result.IsValid,
			"findings":      result.Findings,
			"quality_score": result.QualityScore,
			"stats":         result.Stats,
			"metadata":      result.Metadata,
		}, chainHint)
		return &toolResult, nil
	}
}

// validateDockerfileSyntax checks basic Dockerfile syntax
func validateDockerfileSyntax(content string, result *validation.Result) {
	lines := strings.Split(content, "\n")

	// Check for FROM instruction
	hasFrom := false
	hasWorkdir := false
	cmdCount := 0
	entrypointCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
			hasFrom = true

			// Check for :latest tag
			if strings.Contains(trimmed, ":latest") {
				result.AddWarning("DF002", fmt.Sprintf("Line %d", i+1), "Avoid using :latest tag for reproducible builds")
			}
		}

		// Check for WORKDIR instruction
		if strings.HasPrefix(strings.ToUpper(trimmed), "WORKDIR ") {
			hasWorkdir = true
		}

		// Count CMD instructions
		if strings.HasPrefix(strings.ToUpper(trimmed), "CMD ") {
			cmdCount++
		}

		// Count ENTRYPOINT instructions
		if strings.HasPrefix(strings.ToUpper(trimmed), "ENTRYPOINT ") {
			entrypointCount++
		}

		// Check for deprecated MAINTAINER
		if strings.HasPrefix(strings.ToUpper(trimmed), "MAINTAINER ") {
			result.AddWarning("DF055", fmt.Sprintf("Line %d", i+1), "MAINTAINER instruction is deprecated; use LABEL maintainer instead")
		}

		// Check EXPOSE ports
		if strings.HasPrefix(strings.ToUpper(trimmed), "EXPOSE ") {
			parts := strings.Fields(trimmed)
			for j := 1; j < len(parts); j++ {
				port := strings.Split(parts[j], "/")[0] // Remove protocol if present
				if portNum, err := strconv.Atoi(port); err != nil || portNum < 1 || portNum > 65535 {
					result.AddError("DF050", fmt.Sprintf("Line %d", i+1), fmt.Sprintf("Invalid port number: %s (must be 1-65535)", parts[j]))
				}
			}
		}

		// Check COPY/ADD source paths
		if strings.HasPrefix(strings.ToUpper(trimmed), "COPY ") || strings.HasPrefix(strings.ToUpper(trimmed), "ADD ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				srcPath := parts[1]
				if strings.HasPrefix(srcPath, "/") && !strings.HasPrefix(srcPath, "./") {
					result.AddInfo("DF052", fmt.Sprintf("Line %d", i+1), "Consider using relative paths or explicit current directory (./) for source paths")
				}
			}
		}
	}

	if !hasFrom {
		result.AddError("DF001", "Line 1", "Missing FROM instruction")
	}

	if !hasWorkdir {
		result.AddWarning("DF051", "Dockerfile", "Missing WORKDIR instruction; files will be created in filesystem root")
	}

	if cmdCount > 1 {
		result.AddError("DF053", "Dockerfile", fmt.Sprintf("Multiple CMD instructions found (%d); only the last one will take effect", cmdCount))
	}

	if entrypointCount > 1 {
		result.AddError("DF054", "Dockerfile", fmt.Sprintf("Multiple ENTRYPOINT instructions found (%d); only the last one will take effect", entrypointCount))
	}

	// Check for proper instruction format
	validInstructions := map[string]bool{
		"FROM": true, "RUN": true, "CMD": true, "ENTRYPOINT": true,
		"COPY": true, "ADD": true, "ENV": true, "EXPOSE": true,
		"WORKDIR": true, "USER": true, "VOLUME": true, "ARG": true,
		"HEALTHCHECK": true, "SHELL": true, "STOPSIGNAL": true,
		"LABEL": true, "MAINTAINER": true, "ONBUILD": true,
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.Fields(trimmed)
		if len(parts) > 0 {
			instruction := strings.ToUpper(parts[0])
			if !validInstructions[instruction] && !strings.HasPrefix(trimmed, "#") {
				result.AddWarning("DF003", fmt.Sprintf("Line %d", i+1), fmt.Sprintf("Unknown instruction: %s", parts[0]))
			}
		}
	}

	result.Stats["syntax_valid"] = !result.IsValid
}

// validateDockerfileSecurity checks security best practices
func validateDockerfileSecurity(content string, result *validation.Result) {
	upper := strings.ToUpper(content)
	lines := strings.Split(content, "\n")

	// Check for HEALTHCHECK
	if !strings.Contains(upper, "HEALTHCHECK") {
		result.AddWarning("DF010", "Dockerfile", "Missing HEALTHCHECK instruction for container health monitoring")
	}

	// Enhanced USER validation (DF101 - upgrade to error for root)
	hasUser := false
	isRoot := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upperLine := strings.ToUpper(trimmed)

		if strings.HasPrefix(upperLine, "USER ") {
			hasUser = true
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				user := parts[1]
				if user == "root" || user == "0" {
					result.AddError("DF101", fmt.Sprintf("Line %d", i+1), "Running as root user (UID 0) is a critical security risk")
					isRoot = true
				}
			}
		}
	}

	if !hasUser {
		result.AddError("DF012", "Dockerfile", "No USER instruction found; container will run as root by default")
		isRoot = true
	}

	// Enhanced secret detection (DF103)
	secretPatterns := []struct {
		pattern string
		name    string
	}{
		{"PASSWORD", "password"},
		{"TOKEN", "token"},
		{"SECRET", "secret"},
		{"API_KEY", "API key"},
		{"PRIVATE_KEY", "private key"},
		{"AUTH_TOKEN", "authentication token"},
		{"ACCESS_TOKEN", "access token"},
		{"JWT", "JWT token"},
		{"BEARER", "bearer token"},
		{"OAUTH", "OAuth token"},
		{"CERT", "certificate"},
		{"PEM", "PEM certificate"},
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upperLine := strings.ToUpper(trimmed)

		if strings.HasPrefix(upperLine, "ENV ") {
			for _, secret := range secretPatterns {
				if strings.Contains(upperLine, secret.pattern) {
					result.AddError("DF103", fmt.Sprintf("Line %d", i+1), fmt.Sprintf("Potential %s exposed in ENV instruction", secret.name))
				}
			}
		}

		// Check for insecure file permissions (DF106)
		if strings.Contains(upperLine, "CHMOD 777") || strings.Contains(upperLine, "CHMOD 0777") {
			result.AddError("DF106", fmt.Sprintf("Line %d", i+1), "chmod 777 creates world-writable files, which is insecure")
		}

		if strings.Contains(upperLine, "CHMOD 666") || strings.Contains(upperLine, "CHMOD 0666") {
			result.AddWarning("DF106", fmt.Sprintf("Line %d", i+1), "chmod 666 creates world-writable files, consider more restrictive permissions")
		}
	}

	// Check for dangerous port exposure (DF107)
	dangerousPorts := []int{22, 23, 135, 139, 445, 1433, 1521, 3306, 3389, 5432, 5984, 6379, 8086, 9200, 27017}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upperLine := strings.ToUpper(trimmed)

		if strings.HasPrefix(upperLine, "EXPOSE ") {
			parts := strings.Fields(trimmed)
			for j := 1; j < len(parts); j++ {
				port := strings.Split(parts[j], "/")[0]
				if portNum, err := strconv.Atoi(port); err == nil {
					for _, dangerousPort := range dangerousPorts {
						if portNum == dangerousPort {
							result.AddError("DF107", fmt.Sprintf("Line %d", i+1), fmt.Sprintf("Port %d is commonly targeted by attackers", portNum))
						}
					}
				}
			}
		}
	}

	// Check for package manager security (DF105)
	for i, line := range lines {
		upperLine := strings.ToUpper(line)
		if strings.Contains(upperLine, "APT-GET") && !strings.Contains(upperLine, "APT-GET UPGRADE") &&
			!strings.Contains(upperLine, "APT-GET DIST-UPGRADE") && strings.Contains(upperLine, "APT-GET INSTALL") {
			result.AddWarning("DF105", fmt.Sprintf("Line %d", i+1), "Consider running apt-get upgrade to install security updates")
		}
	}

	// Check for unnecessary packages in production (DF104)
	developmentPackages := []string{"curl", "wget", "vim", "nano", "git", "ssh", "telnet", "netcat", "nc"}
	if strings.Contains(upper, "FROM") && !strings.Contains(upper, "AS BUILDER") && !strings.Contains(upper, "AS BUILD") {
		for i, line := range lines {
			upperLine := strings.ToUpper(line)
			if strings.Contains(upperLine, "RUN") && (strings.Contains(upperLine, "APT-GET INSTALL") ||
				strings.Contains(upperLine, "YUM INSTALL") || strings.Contains(upperLine, "DNF INSTALL")) {

				for _, pkg := range developmentPackages {
					if strings.Contains(upperLine, " "+strings.ToUpper(pkg)+" ") ||
						strings.Contains(upperLine, " "+strings.ToUpper(pkg)) {
						result.AddWarning("DF104", fmt.Sprintf("Line %d", i+1), fmt.Sprintf("Package '%s' may not be needed in production image", pkg))
					}
				}
			}
		}
	}

	result.Stats["security_checks"] = map[string]bool{
		"has_healthcheck": strings.Contains(upper, "HEALTHCHECK"),
		"has_user":        hasUser,
		"is_root":         isRoot,
		"no_secrets":      result.ErrorCount() == 0,
	}
}

// validateDockerfileBestPractices checks for best practices
func validateDockerfileBestPractices(content string, result *validation.Result) {
	upper := strings.ToUpper(content)
	lines := strings.Split(content, "\n")

	// Check for multi-stage build
	fromCount := strings.Count(upper, "FROM ")
	if fromCount > 1 {
		result.Stats["multi_stage"] = true
		result.AddInfo("DF020", "Dockerfile", "Good: Using multi-stage build for smaller image size")
	} else {
		result.AddInfo("DF021", "Dockerfile", "Consider using multi-stage build to reduce image size")
	}

	// Enhanced layer optimization analysis (DF201-DF206)
	runCount := 0
	copyCount := 0
	addCount := 0
	hasCache := false
	hasBuildDepsInFinal := false
	largeOperations := []string{}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upperLine := strings.ToUpper(trimmed)

		if strings.HasPrefix(upperLine, "RUN ") {
			runCount++

			// Check for inefficient layer ordering (DF201)
			if strings.Contains(upperLine, "APT-GET UPDATE") {
				// Look ahead to see if install is in the same RUN
				if !strings.Contains(upperLine, "APT-GET INSTALL") {
					result.AddWarning("DF201", fmt.Sprintf("Line %d", i+1), "Inefficient layer ordering: apt-get update without install in same RUN")
				}
			}

			// Check for large intermediate layers (DF203)
			if strings.Contains(upperLine, "INSTALL") && strings.Contains(upperLine, "BUILD-ESSENTIAL") {
				largeOperations = append(largeOperations, fmt.Sprintf("Line %d: build tools installation", i+1))
			}

			// Check for cache optimization
			if strings.Contains(upperLine, "--NO-CACHE") || strings.Contains(upperLine, "RM -RF /VAR/LIB/APT/LISTS/*") {
				hasCache = true
			}
		}

		if strings.HasPrefix(upperLine, "COPY ") {
			copyCount++

			// Analyze COPY efficiency (DF206)
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				if strings.Contains(parts[1], "*") && !strings.Contains(parts[len(parts)-1], "/") {
					result.AddWarning("DF206", fmt.Sprintf("Line %d", i+1), "Inefficient COPY: using wildcards with non-directory destination")
				}
			}
		}

		if strings.HasPrefix(upperLine, "ADD ") {
			addCount++
		}
	}

	// Layer count analysis
	if runCount > 5 {
		result.AddWarning("DF022", "Dockerfile", fmt.Sprintf("Found %d RUN instructions; consider combining them to reduce layers", runCount))
	}

	// Check for build dependencies in final stage (DF204)
	if fromCount == 1 { // Single-stage build
		buildDeps := []string{"build-essential", "gcc", "g++", "make", "cmake", "maven", "gradle"}
		for i, line := range lines {
			upperLine := strings.ToUpper(line)
			for _, dep := range buildDeps {
				if strings.Contains(upperLine, strings.ToUpper(dep)) && strings.Contains(upperLine, "INSTALL") {
					hasBuildDepsInFinal = true
					result.AddWarning("DF204", fmt.Sprintf("Line %d", i+1), fmt.Sprintf("Build dependency '%s' found in final stage; consider multi-stage build", dep))
				}
			}
		}
	}

	// Large intermediate layers warning (DF203)
	if len(largeOperations) > 0 {
		result.AddWarning("DF203", "Dockerfile", fmt.Sprintf("Large intermediate layers detected: %s", strings.Join(largeOperations, ", ")))
	}

	// .dockerignore impact analysis (DF205)
	hasDockerignoreRef := false
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), ".dockerignore") {
			hasDockerignoreRef = true
			break
		}
	}

	if !hasDockerignoreRef {
		result.AddWarning("DF205", "Dockerfile", "Missing .dockerignore reference; large build context may slow builds and increase layer sizes")
	}

	// COPY vs ADD optimization
	if addCount > 0 {
		result.AddInfo("DF023", "Dockerfile", fmt.Sprintf("Found %d ADD instructions; consider using COPY instead unless you need ADD's tar extraction features", addCount))
	}

	// apt-get best practices
	for i, line := range lines {
		if strings.Contains(line, "apt-get update") && !strings.Contains(line, "apt-get install") {
			result.AddWarning("DF024", fmt.Sprintf("Line %d", i+1), "apt-get update should be combined with apt-get install in the same RUN instruction")
		}

		if strings.Contains(line, "apt-get install") && !strings.Contains(line, "-y") {
			result.AddWarning("DF025", fmt.Sprintf("Line %d", i+1), "apt-get install should use -y flag for non-interactive installation")
		}
	}

	// .dockerignore suggestion
	if !strings.Contains(content, ".dockerignore") {
		result.AddInfo("DF026", "Dockerfile", "Consider using a .dockerignore file to exclude unnecessary files from the build context")
	}

	result.Stats["best_practices"] = map[string]interface{}{
		"multi_stage":             fromCount > 1,
		"run_commands":            runCount,
		"copy_commands":           copyCount,
		"add_commands":            addCount,
		"from_commands":           fromCount,
		"has_cache_optimization":  hasCache,
		"has_build_deps_in_final": hasBuildDepsInFinal,
		"large_operations_count":  len(largeOperations),
	}
}

// createValidateManifestsHandler creates the K8s manifests validation handler
func createValidateManifestsHandler(deps ToolDependencies) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := deps.Logger
		if logger == nil {
			logger = slog.Default()
		}

		args := req.GetArguments()
		content, ok := args["content"].(string)
		if !ok || content == "" {
			err := fmt.Errorf("missing or empty 'content' parameter")
			result := createErrorResult(err)
			return &result, nil
		}

		logger.Info("validating Kubernetes manifests",
			"content_length", len(content))

		// Build unified validation result
		result := validation.NewResult()
		result.Stats["size"] = len(content)

		// Perform validation checks
		validateK8sManifestSyntax(content, result)
		validateK8sManifestSecurity(content, result)
		validateK8sManifestBestPractices(content, result)

		// Calculate quality score
		result.CalculateQualityScore()

		logger.Info("K8s manifests validation complete",
			"is_valid", result.IsValid,
			"errors", result.ErrorCount(),
			"warnings", result.WarningCount(),
			"quality_score", result.QualityScore)

		// Validation complete - prepare response

		// Set appropriate chain hint
		var chainHint *ChainHint
		if result.IsValid {
			chainHint = &ChainHint{
				NextTool: "apply_k8s_manifests",
				Reason:   "Validation passed. Apply the manifests.",
			}
		} else {
			chainHint = &ChainHint{
				NextTool: "k8s-manifests-critique",
				Reason:   fmt.Sprintf("%d errors found. Use critique prompt to fix.", result.ErrorCount()),
			}
		}

		toolResult := createToolResult(result.IsValid, map[string]interface{}{
			"is_valid":      result.IsValid,
			"findings":      result.Findings,
			"quality_score": result.QualityScore,
			"stats":         result.Stats,
			"metadata":      result.Metadata,
		}, chainHint)
		return &toolResult, nil
	}
}

// validateK8sManifestSyntax checks YAML syntax and required fields using proper parsing
func validateK8sManifestSyntax(content string, result *validation.Result) {
	// Parse YAML documents (K8S050)
	documents, err := parseYAMLDocuments(content)
	if err != nil {
		result.AddError("K8S050", "yaml", fmt.Sprintf("Invalid YAML syntax: %v", err))
		return
	}

	if len(documents) == 0 {
		result.AddError("K8S050", "manifest", "No YAML documents found")
		return
	}

	// Validate each document
	for i, doc := range documents {
		docPrefix := fmt.Sprintf("document %d", i+1)
		validateDocument(doc, docPrefix, result)
	}

	result.Stats["yaml_valid"] = result.ErrorCount() == 0
	result.Stats["document_count"] = len(documents)
}

// parseYAMLDocuments parses multiple YAML documents from content
func parseYAMLDocuments(content string) ([]map[string]interface{}, error) {
	var documents []map[string]interface{}

	// Split by document separator
	parts := strings.Split(content, "---")

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		var doc map[string]interface{}
		if err := yaml.Unmarshal([]byte(trimmed), &doc); err != nil {
			return nil, err
		}

		documents = append(documents, doc)
	}

	return documents, nil
}

// validateDocument validates a single K8s document
func validateDocument(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	// Check required fields
	apiVersion, hasAPIVersion := doc["apiVersion"].(string)
	if !hasAPIVersion || apiVersion == "" {
		result.AddError("K8S001", docPrefix, "Missing apiVersion field")
	}

	kind, hasKind := doc["kind"].(string)
	if !hasKind || kind == "" {
		result.AddError("K8S002", docPrefix, "Missing kind field")
	}

	metadata, hasMetadata := doc["metadata"].(map[string]interface{})
	if !hasMetadata {
		result.AddError("K8S003", docPrefix, "Missing metadata field")
	} else {
		name, hasName := metadata["name"].(string)
		if !hasName || name == "" {
			result.AddError("K8S004", docPrefix+".metadata", "Missing name in metadata")
		}
	}

	// Kind-specific validation
	if hasKind {
		switch kind {
		case "Deployment":
			validateDeployment(doc, docPrefix, result)
		case "Service":
			validateService(doc, docPrefix, result)
		case "ConfigMap":
			validateConfigMap(doc, docPrefix, result)
		case "Secret":
			validateSecret(doc, docPrefix, result)
		}
	}

	// Check for deprecated API versions (K8S053)
	if hasAPIVersion {
		checkDeprecatedAPIVersion(apiVersion, kind, docPrefix, result)
	}
}

// validateDeployment validates Deployment-specific fields
func validateDeployment(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	spec, hasSpec := doc["spec"].(map[string]interface{})
	if !hasSpec {
		result.AddError("K8S005", docPrefix, "Deployment missing spec field")
		return
	}

	if _, hasReplicas := spec["replicas"]; !hasReplicas {
		result.AddWarning("K8S006", docPrefix+".spec", "Deployment missing replicas field (will default to 1)")
	}

	if _, hasSelector := spec["selector"]; !hasSelector {
		result.AddError("K8S007", docPrefix+".spec", "Deployment missing selector field")
	}
}

// validateService validates Service-specific fields
func validateService(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	spec, hasSpec := doc["spec"].(map[string]interface{})
	if !hasSpec {
		result.AddError("K8S008", docPrefix, "Service missing spec field")
		return
	}

	if _, hasPorts := spec["ports"]; !hasPorts {
		result.AddError("K8S009", docPrefix+".spec", "Service missing ports field")
	}
}

// validateConfigMap validates ConfigMap-specific fields
func validateConfigMap(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	if _, hasData := doc["data"]; !hasData {
		if _, hasBinaryData := doc["binaryData"]; !hasBinaryData {
			result.AddWarning("K8S010", docPrefix, "ConfigMap has neither data nor binaryData")
		}
	}
}

// validateSecret validates Secret-specific fields
func validateSecret(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	if _, hasData := doc["data"]; !hasData {
		if _, hasStringData := doc["stringData"]; !hasStringData {
			result.AddWarning("K8S011", docPrefix, "Secret has neither data nor stringData")
		}
	}
}

// checkDeprecatedAPIVersion checks for deprecated API versions
func checkDeprecatedAPIVersion(apiVersion, kind, docPrefix string, result *validation.Result) {
	deprecatedAPIs := map[string]string{
		"extensions/v1beta1": "apps/v1",
		"apps/v1beta1":       "apps/v1",
		"apps/v1beta2":       "apps/v1",
	}

	if replacement, isDeprecated := deprecatedAPIs[apiVersion]; isDeprecated {
		result.AddWarning("K8S053", docPrefix, fmt.Sprintf("API version %s is deprecated for %s, use %s instead", apiVersion, kind, replacement))
	}
}

// validateK8sManifestSecurity checks security best practices with structured parsing
func validateK8sManifestSecurity(content string, result *validation.Result) {
	// Parse YAML documents for structured analysis
	documents, err := parseYAMLDocuments(content)
	if err != nil {
		// Fall back to string-based checks if parsing fails
		validateK8sSecurityStringBased(content, result)
		return
	}

	// Analyze each document
	for i, doc := range documents {
		docPrefix := fmt.Sprintf("document %d", i+1)
		validateDocumentSecurity(doc, docPrefix, result)
	}

	// Set security stats
	result.Stats["security_checks"] = map[string]bool{
		"has_security_context": strings.Contains(content, "securityContext:"),
		"has_resource_limits":  strings.Contains(content, "limits:"),
		"not_privileged":       !strings.Contains(content, "privileged: true"),
	}
}

// validateDocumentSecurity performs security validation on a single document
func validateDocumentSecurity(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	kind, hasKind := doc["kind"].(string)
	if !hasKind {
		return
	}

	switch kind {
	case "Deployment", "Pod", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		validateWorkloadSecurity(doc, docPrefix, result)
	case "Service":
		validateServiceSecurity(doc, docPrefix, result)
	case "Ingress":
		validateIngressSecurity(doc, docPrefix, result)
	}
}

// validateWorkloadSecurity validates security for workload resources
func validateWorkloadSecurity(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	spec, hasSpec := doc["spec"].(map[string]interface{})
	if !hasSpec {
		return
	}

	// Get pod template based on resource type
	var template map[string]interface{}
	if podSpec, exists := spec["template"].(map[string]interface{}); exists {
		if podSpecSpec, exists := podSpec["spec"].(map[string]interface{}); exists {
			template = podSpecSpec
		}
	} else if kind, _ := doc["kind"].(string); kind == "Pod" {
		template = spec
	}

	if template == nil {
		return
	}

	// Pod-level security context (K8S101)
	podSecurityContext := getPodSecurityContext(template)
	validatePodSecurityContext(podSecurityContext, docPrefix, result)

	// Container security validation
	if containers, exists := template["containers"].([]interface{}); exists {
		for i, container := range containers {
			containerMap, ok := container.(map[string]interface{})
			if !ok {
				continue
			}

			containerPrefix := fmt.Sprintf("%s.containers[%d]", docPrefix, i)
			validateContainerSecurity(containerMap, containerPrefix, result)
		}
	}

	// Service account security (K8S106)
	if serviceAccount, exists := template["serviceAccountName"].(string); exists {
		if serviceAccount == "default" {
			result.AddWarning("K8S106", docPrefix, "Using default service account; consider creating dedicated service account")
		}
	} else {
		result.AddInfo("K8S106", docPrefix, "No serviceAccountName specified; will use default service account")
	}
}

// validatePodSecurityContext validates pod-level security settings
func validatePodSecurityContext(securityContext map[string]interface{}, docPrefix string, result *validation.Result) {
	if len(securityContext) == 0 {
		result.AddWarning("K8S101", docPrefix, "Missing pod securityContext; consider adding security constraints")
		return
	}

	// Check runAsUser
	if runAsUser, exists := securityContext["runAsUser"]; exists {
		if user, ok := runAsUser.(int); ok && user == 0 {
			result.AddError("K8S015", docPrefix+".securityContext", "Pod running as root user (UID 0) is a security risk")
		}
	} else {
		result.AddWarning("K8S015", docPrefix+".securityContext", "No runAsUser specified; pod may run as root")
	}

	// Check runAsNonRoot
	if runAsNonRoot, exists := securityContext["runAsNonRoot"]; exists {
		if nonRoot, ok := runAsNonRoot.(bool); ok && !nonRoot {
			result.AddWarning("K8S015", docPrefix+".securityContext", "runAsNonRoot is false; consider setting to true")
		}
	} else {
		result.AddInfo("K8S015", docPrefix+".securityContext", "Consider setting runAsNonRoot: true for enhanced security")
	}
}

// validateContainerSecurity validates container-level security settings
func validateContainerSecurity(container map[string]interface{}, containerPrefix string, result *validation.Result) {
	// Check for container security context (K8S101)
	containerSecurityContext, hasContainerSC := container["securityContext"].(map[string]interface{})
	if !hasContainerSC || len(containerSecurityContext) == 0 {
		result.AddWarning("K8S101", containerPrefix, "Missing container securityContext")
	} else {
		validateContainerSecurityContext(containerSecurityContext, containerPrefix, result)
	}

	// Check for resource limits and requests (K8S011-K8S013)
	if resources, hasResources := container["resources"].(map[string]interface{}); hasResources {
		validateResourceLimits(resources, containerPrefix, result)
	} else {
		result.AddWarning("K8S011", containerPrefix, "Missing resource limits and requests")
	}

	// Check image tag (K8S024)
	if image, hasImage := container["image"].(string); hasImage {
		if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
			result.AddWarning("K8S024", containerPrefix, "Avoid using :latest tag or no tag for reproducible deployments")
		}
	}
}

// validateContainerSecurityContext validates container security context
func validateContainerSecurityContext(securityContext map[string]interface{}, containerPrefix string, result *validation.Result) {
	// Check privileged mode (K8S014)
	if privileged, exists := securityContext["privileged"]; exists {
		if isPrivileged, ok := privileged.(bool); ok && isPrivileged {
			result.AddError("K8S014", containerPrefix+".securityContext", "Container running in privileged mode is a critical security risk")
		}
	}

	// Check capabilities (K8S103)
	if capabilities, exists := securityContext["capabilities"]; exists {
		if capMap, ok := capabilities.(map[string]interface{}); ok {
			if add, hasAdd := capMap["add"]; hasAdd {
				if addList, ok := add.([]interface{}); ok {
					dangerousCaps := []string{"SYS_ADMIN", "NET_ADMIN", "SYS_PTRACE", "DAC_OVERRIDE"}
					for _, cap := range addList {
						if capStr, ok := cap.(string); ok {
							for _, dangerous := range dangerousCaps {
								if capStr == dangerous {
									result.AddError("K8S103", containerPrefix+".securityContext.capabilities", fmt.Sprintf("Dangerous capability %s granted", capStr))
								}
							}
						}
					}
				}
			}
		}
	}

	// Check readOnlyRootFilesystem (K8S107)
	if readOnly, exists := securityContext["readOnlyRootFilesystem"]; exists {
		if isReadOnly, ok := readOnly.(bool); ok && !isReadOnly {
			result.AddWarning("K8S107", containerPrefix+".securityContext", "Consider setting readOnlyRootFilesystem: true")
		}
	} else {
		result.AddInfo("K8S107", containerPrefix+".securityContext", "Consider adding readOnlyRootFilesystem: true for enhanced security")
	}

	// Check allowPrivilegeEscalation
	if allowEscalation, exists := securityContext["allowPrivilegeEscalation"]; exists {
		if allowEsc, ok := allowEscalation.(bool); ok && allowEsc {
			result.AddWarning("K8S103", containerPrefix+".securityContext", "allowPrivilegeEscalation is true; consider setting to false")
		}
	} else {
		result.AddInfo("K8S103", containerPrefix+".securityContext", "Consider setting allowPrivilegeEscalation: false")
	}
}

// validateResourceLimits validates resource limits and requests
func validateResourceLimits(resources map[string]interface{}, containerPrefix string, result *validation.Result) {
	hasLimits := false
	hasRequests := false

	if limits, hasLimitsField := resources["limits"].(map[string]interface{}); hasLimitsField && len(limits) > 0 {
		hasLimits = true
	}

	if requests, hasRequestsField := resources["requests"].(map[string]interface{}); hasRequestsField && len(requests) > 0 {
		hasRequests = true
	}

	if !hasLimits {
		result.AddWarning("K8S012", containerPrefix+".resources", "Missing resource limits")
	}

	if !hasRequests {
		result.AddWarning("K8S013", containerPrefix+".resources", "Missing resource requests")
	}
}

// validateServiceSecurity validates Service security
func validateServiceSecurity(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	spec, hasSpec := doc["spec"].(map[string]interface{})
	if !hasSpec {
		return
	}

	// Check service type
	if serviceType, exists := spec["type"]; exists {
		if svcType, ok := serviceType.(string); ok && svcType == "LoadBalancer" {
			result.AddWarning("K8S105", docPrefix, "LoadBalancer service type exposes service externally; ensure this is intended")
		}
	}
}

// validateIngressSecurity validates Ingress security
func validateIngressSecurity(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	spec, hasSpec := doc["spec"].(map[string]interface{})
	if !hasSpec {
		return
	}

	// Check for TLS
	if _, hasTLS := spec["tls"]; !hasTLS {
		result.AddWarning("K8S105", docPrefix, "Ingress without TLS configuration; consider adding HTTPS")
	}
}

// getPodSecurityContext extracts pod security context
func getPodSecurityContext(podSpec map[string]interface{}) map[string]interface{} {
	if securityContext, exists := podSpec["securityContext"].(map[string]interface{}); exists {
		return securityContext
	}
	return map[string]interface{}{}
}

// validateK8sSecurityStringBased is a fallback for string-based security validation
func validateK8sSecurityStringBased(content string, result *validation.Result) {
	// Legacy string-based checks as fallback
	if !strings.Contains(content, "securityContext:") {
		result.AddWarning("K8S101", "spec", "Missing securityContext for security hardening")
	}

	if !strings.Contains(content, "resources:") {
		result.AddWarning("K8S011", "spec.containers", "Missing resource limits and requests")
	} else {
		if !strings.Contains(content, "limits:") {
			result.AddWarning("K8S012", "spec.containers.resources", "Missing resource limits")
		}
		if !strings.Contains(content, "requests:") {
			result.AddWarning("K8S013", "spec.containers.resources", "Missing resource requests")
		}
	}

	if strings.Contains(content, "privileged: true") {
		result.AddError("K8S014", "securityContext", "Container running in privileged mode is a security risk")
	}

	if strings.Contains(content, "runAsUser: 0") {
		result.AddWarning("K8S015", "securityContext", "Container running as root user (UID 0)")
	}
}

// validateK8sManifestBestPractices checks for K8s best practices and consistency
func validateK8sManifestBestPractices(content string, result *validation.Result) {
	// Parse YAML documents for structured analysis
	documents, err := parseYAMLDocuments(content)
	if err != nil {
		// Fall back to string-based checks if parsing fails
		validateK8sBestPracticesStringBased(content, result)
		return
	}

	// Perform consistency checks across documents (K8S301-K8S306)
	validateResourceConsistency(documents, result)

	// Validate best practices for each document
	for i, doc := range documents {
		docPrefix := fmt.Sprintf("document %d", i+1)
		validateDocumentBestPractices(doc, docPrefix, result)
	}

	result.Stats["best_practices"] = map[string]bool{
		"has_labels":           strings.Contains(content, "labels:"),
		"has_liveness_probe":   strings.Contains(content, "livenessProbe:"),
		"has_readiness_probe":  strings.Contains(content, "readinessProbe:"),
		"no_latest_tag":        !strings.Contains(content, ":latest"),
		"consistent_selectors": true, // Will be set by consistency validation
	}
}

// validateResourceConsistency checks for consistency across multiple resources
func validateResourceConsistency(documents []map[string]interface{}, result *validation.Result) {
	var services []map[string]interface{}
	var deployments []map[string]interface{}
	var configMaps []string
	var secrets []string
	namespaces := make(map[string]bool)
	resourceNames := make(map[string]string) // name -> kind

	// Collect resources by type
	for _, doc := range documents {
		kind, _ := doc["kind"].(string)
		metadata, hasMetadata := doc["metadata"].(map[string]interface{})
		if !hasMetadata {
			continue
		}

		name, hasName := metadata["name"].(string)
		if !hasName {
			continue
		}

		// Check for duplicate resource names (K8S306)
		if existingKind, exists := resourceNames[name]; exists && existingKind != kind {
			result.AddError("K8S306", "metadata", fmt.Sprintf("Duplicate resource name '%s' found in %s and %s", name, existingKind, kind))
		} else {
			resourceNames[name] = kind
		}

		// Collect namespace info (K8S303)
		if namespace, hasNamespace := metadata["namespace"].(string); hasNamespace {
			namespaces[namespace] = true
		}

		switch kind {
		case "Service":
			services = append(services, doc)
		case "Deployment":
			deployments = append(deployments, doc)
		case "ConfigMap":
			configMaps = append(configMaps, name)
		case "Secret":
			secrets = append(secrets, name)
		}
	}

	// Namespace consistency check (K8S303)
	if len(namespaces) > 1 {
		var nsList []string
		for ns := range namespaces {
			nsList = append(nsList, ns)
		}
		result.AddWarning("K8S303", "manifests", fmt.Sprintf("Multiple namespaces detected: %v; ensure this is intended", nsList))
	}

	// Service/Deployment port mismatch detection (K8S301)
	validateServiceDeploymentConsistency(services, deployments, result)

	// Label selector consistency (K8S302)
	validateLabelSelectorConsistency(deployments, services, result)

	// ConfigMap/Secret reference validation (K8S304)
	validateConfigMapSecretReferences(documents, configMaps, secrets, result)
}

// validateServiceDeploymentConsistency checks if service ports match deployment container ports
func validateServiceDeploymentConsistency(services, deployments []map[string]interface{}, result *validation.Result) {
	for _, service := range services {
		serviceName := getResourceName(service)
		serviceSpec, hasSpec := service["spec"].(map[string]interface{})
		if !hasSpec {
			continue
		}

		servicePorts := getServicePorts(serviceSpec)

		// Find matching deployment
		for _, deployment := range deployments {
			deploymentName := getResourceName(deployment)
			if serviceName != deploymentName && !hasMatchingLabels(service, deployment) {
				continue
			}

			deploymentPorts := getDeploymentContainerPorts(deployment)

			// Check if service ports match container ports
			for _, servicePort := range servicePorts {
				found := false
				for _, containerPort := range deploymentPorts {
					if servicePort == containerPort {
						found = true
						break
					}
				}
				if !found {
					result.AddWarning("K8S301", "service/deployment", fmt.Sprintf("Service port %d not found in deployment container ports", servicePort))
				}
			}
		}
	}
}

// validateLabelSelectorConsistency checks label selector consistency
func validateLabelSelectorConsistency(deployments, services []map[string]interface{}, result *validation.Result) {
	for _, deployment := range deployments {
		deploymentName := getResourceName(deployment)
		deploymentLabels := getDeploymentPodLabels(deployment)

		// Find corresponding service
		for _, service := range services {
			serviceName := getResourceName(service)
			if serviceName != deploymentName {
				continue
			}

			serviceSelector := getServiceSelector(service)

			// Check if service selector matches deployment pod labels
			for selectorKey, selectorValue := range serviceSelector {
				if deploymentValue, exists := deploymentLabels[selectorKey]; !exists || deploymentValue != selectorValue {
					result.AddError("K8S302", "service/deployment", fmt.Sprintf("Service selector %s=%s does not match deployment pod label", selectorKey, selectorValue))
				}
			}
		}
	}
}

// validateConfigMapSecretReferences validates that referenced ConfigMaps and Secrets exist
func validateConfigMapSecretReferences(documents []map[string]interface{}, configMaps, secrets []string, result *validation.Result) {
	for _, doc := range documents {
		kind, _ := doc["kind"].(string)
		if kind != "Deployment" && kind != "Pod" && kind != "StatefulSet" && kind != "DaemonSet" {
			continue
		}

		// Check ConfigMap references
		cmRefs := extractConfigMapReferences(doc)
		for _, cmRef := range cmRefs {
			found := false
			for _, cm := range configMaps {
				if cm == cmRef {
					found = true
					break
				}
			}
			if !found {
				result.AddError("K8S304", "configMapRef", fmt.Sprintf("Referenced ConfigMap '%s' not found in manifests", cmRef))
			}
		}

		// Check Secret references
		secretRefs := extractSecretReferences(doc)
		for _, secretRef := range secretRefs {
			found := false
			for _, secret := range secrets {
				if secret == secretRef {
					found = true
					break
				}
			}
			if !found {
				result.AddError("K8S304", "secretRef", fmt.Sprintf("Referenced Secret '%s' not found in manifests", secretRef))
			}
		}

		// Environment variable validation (K8S305)
		validateEnvironmentVariables(doc, result)
	}
}

// validateDocumentBestPractices validates best practices for a single document
func validateDocumentBestPractices(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	metadata, hasMetadata := doc["metadata"].(map[string]interface{})
	if hasMetadata {
		// Check for labels
		if _, hasLabels := metadata["labels"]; !hasLabels {
			result.AddInfo("K8S020", docPrefix+".metadata", "Consider adding labels for better resource organization")
		}
	}

	kind, _ := doc["kind"].(string)
	if kind == "Deployment" || kind == "Pod" || kind == "StatefulSet" || kind == "DaemonSet" {
		validateWorkloadBestPractices(doc, docPrefix, result)
	}
}

// validateWorkloadBestPractices validates best practices for workload resources
func validateWorkloadBestPractices(doc map[string]interface{}, docPrefix string, result *validation.Result) {
	spec, hasSpec := doc["spec"].(map[string]interface{})
	if !hasSpec {
		return
	}

	// Get pod template
	var template map[string]interface{}
	if podSpec, exists := spec["template"].(map[string]interface{}); exists {
		if podSpecSpec, exists := podSpec["spec"].(map[string]interface{}); exists {
			template = podSpecSpec
		}
	} else if kind, _ := doc["kind"].(string); kind == "Pod" {
		template = spec
	}

	if template == nil {
		return
	}

	// Check containers for probes
	if containers, exists := template["containers"].([]interface{}); exists {
		for i, container := range containers {
			containerMap, ok := container.(map[string]interface{})
			if !ok {
				continue
			}

			containerPrefix := fmt.Sprintf("%s.containers[%d]", docPrefix, i)

			// Check for liveness probe
			if _, hasLiveness := containerMap["livenessProbe"]; !hasLiveness {
				result.AddInfo("K8S021", containerPrefix, "Consider adding livenessProbe for health checking")
			}

			// Check for readiness probe
			if _, hasReadiness := containerMap["readinessProbe"]; !hasReadiness {
				result.AddInfo("K8S022", containerPrefix, "Consider adding readinessProbe for traffic routing")
			}

			// Check image pull policy
			if imagePullPolicy, hasPullPolicy := containerMap["imagePullPolicy"].(string); hasPullPolicy && imagePullPolicy == "Always" {
				result.AddInfo("K8S023", containerPrefix, "imagePullPolicy: Always may cause unnecessary pulls")
			}
		}
	}
}

// Helper functions for consistency validation

func getResourceName(doc map[string]interface{}) string {
	if metadata, hasMetadata := doc["metadata"].(map[string]interface{}); hasMetadata {
		if name, hasName := metadata["name"].(string); hasName {
			return name
		}
	}
	return ""
}

func getServicePorts(serviceSpec map[string]interface{}) []int {
	var ports []int
	if portsList, hasPorts := serviceSpec["ports"].([]interface{}); hasPorts {
		for _, port := range portsList {
			if portMap, ok := port.(map[string]interface{}); ok {
				if targetPort, hasTargetPort := portMap["targetPort"]; hasTargetPort {
					if portNum, ok := targetPort.(int); ok {
						ports = append(ports, portNum)
					}
				} else if portNum, hasPort := portMap["port"].(int); hasPort {
					ports = append(ports, portNum)
				}
			}
		}
	}
	return ports
}

func getDeploymentContainerPorts(deployment map[string]interface{}) []int {
	var ports []int
	spec, hasSpec := deployment["spec"].(map[string]interface{})
	if !hasSpec {
		return ports
	}

	template, hasTemplate := spec["template"].(map[string]interface{})
	if !hasTemplate {
		return ports
	}

	podSpec, hasPodSpec := template["spec"].(map[string]interface{})
	if !hasPodSpec {
		return ports
	}

	if containers, hasContainers := podSpec["containers"].([]interface{}); hasContainers {
		for _, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if portsList, hasPorts := containerMap["ports"].([]interface{}); hasPorts {
					for _, port := range portsList {
						if portMap, ok := port.(map[string]interface{}); ok {
							if containerPort, hasPort := portMap["containerPort"].(int); hasPort {
								ports = append(ports, containerPort)
							}
						}
					}
				}
			}
		}
	}
	return ports
}

func getDeploymentPodLabels(deployment map[string]interface{}) map[string]string {
	labels := make(map[string]string)
	spec, hasSpec := deployment["spec"].(map[string]interface{})
	if !hasSpec {
		return labels
	}

	template, hasTemplate := spec["template"].(map[string]interface{})
	if !hasTemplate {
		return labels
	}

	metadata, hasMetadata := template["metadata"].(map[string]interface{})
	if !hasMetadata {
		return labels
	}

	if labelMap, hasLabels := metadata["labels"].(map[string]interface{}); hasLabels {
		for key, value := range labelMap {
			if strValue, ok := value.(string); ok {
				labels[key] = strValue
			}
		}
	}
	return labels
}

func getServiceSelector(service map[string]interface{}) map[string]string {
	selector := make(map[string]string)
	spec, hasSpec := service["spec"].(map[string]interface{})
	if !hasSpec {
		return selector
	}

	if selectorMap, hasSelector := spec["selector"].(map[string]interface{}); hasSelector {
		for key, value := range selectorMap {
			if strValue, ok := value.(string); ok {
				selector[key] = strValue
			}
		}
	}
	return selector
}

func hasMatchingLabels(service, deployment map[string]interface{}) bool {
	serviceSelector := getServiceSelector(service)
	deploymentLabels := getDeploymentPodLabels(deployment)

	for key, value := range serviceSelector {
		if deploymentValue, exists := deploymentLabels[key]; !exists || deploymentValue != value {
			return false
		}
	}
	return len(serviceSelector) > 0
}

func extractConfigMapReferences(doc map[string]interface{}) []string {
	var refs []string
	// This is a simplified extraction - would need deeper traversal for complete implementation
	content := fmt.Sprintf("%v", doc)
	// Look for configMapRef patterns
	if strings.Contains(content, "configMapRef") {
		refs = append(refs, "example-configmap") // Placeholder
	}
	return refs
}

func extractSecretReferences(doc map[string]interface{}) []string {
	var refs []string
	// This is a simplified extraction - would need deeper traversal for complete implementation
	content := fmt.Sprintf("%v", doc)
	// Look for secretRef patterns
	if strings.Contains(content, "secretRef") {
		refs = append(refs, "example-secret") // Placeholder
	}
	return refs
}

func validateEnvironmentVariables(doc map[string]interface{}, result *validation.Result) {
	// Environment variable validation (K8S305)
	// This would check for proper env var configuration
	// Simplified implementation for now
}

// validateK8sBestPracticesStringBased is a fallback for string-based validation
func validateK8sBestPracticesStringBased(content string, result *validation.Result) {
	// Legacy string-based checks
	if !strings.Contains(content, "labels:") {
		result.AddInfo("K8S020", "metadata", "Consider adding labels for better resource organization")
	}

	if !strings.Contains(content, "livenessProbe:") {
		result.AddInfo("K8S021", "spec.containers", "Consider adding livenessProbe for health checking")
	}

	if !strings.Contains(content, "readinessProbe:") {
		result.AddInfo("K8S022", "spec.containers", "Consider adding readinessProbe for traffic routing")
	}

	if strings.Contains(content, "imagePullPolicy: Always") {
		result.AddInfo("K8S023", "spec.containers", "imagePullPolicy: Always may cause unnecessary pulls")
	}

	if strings.Contains(content, ":latest") {
		result.AddWarning("K8S024", "spec.containers.image", "Avoid using :latest tag for reproducible deployments")
	}
}

// Apply handler implementations are in separate files:
// - apply_dockerfile_handler.go
// - apply_k8s_handler.go
