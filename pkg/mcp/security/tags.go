// Package validation - Tag-based validation DSL for struct validation
package security

import (
	"regexp"
	"strings"
)

// Common validation tags for container-kit domain objects
const (
	// Basic validation tags
	TagRequired  = "required"
	TagOmitEmpty = "omitempty"
	TagMinLength = "min"
	TagMaxLength = "max"
	TagLength    = "len"
	TagRegex     = "regex"
	TagOneOf     = "oneof"

	// Infrastructure validation tags
	TagGitURL      = "git_url"
	TagImageName   = "image_name"
	TagDockerImage = "docker_image"
	TagDockerTag   = "docker_tag"
	TagPlatform    = "platform"
	TagRegistryURL = "registry_url"

	// Kubernetes validation tags
	TagK8sName      = "k8s_name"
	TagNamespace    = "namespace"
	TagResourceName = "resource_name"
	TagK8sLabel     = "k8s_label"
	TagK8sSelector  = "k8s_selector"

	// Security validation tags
	TagSessionID   = "session_id"
	TagNoSensitive = "no_sensitive"
	TagFilePath    = "file_path"
	TagSecurePath  = "secure_path"
	TagNoInjection = "no_injection"

	// Network validation tags
	TagURL       = "url"
	TagEndpoint  = "endpoint"
	TagPort      = "port"
	TagIPAddress = "ip"
	TagDomain    = "domain"

	// Collection validation tags
	TagDive      = "dive"
	TagKeys      = "keys"
	TagEndKeys   = "endkeys"
	TagValues    = "values"
	TagEndValues = "endvalues"
	TagMinItems  = "min_items"
	TagMaxItems  = "max_items"

	// Domain-specific validation tags
	TagGitBranch    = "git_branch"
	TagLanguage     = "language"
	TagFramework    = "framework"
	TagSeverity     = "severity"
	TagServiceType  = "service_type"
	TagStrategy     = "strategy"
	TagVulnType     = "vuln_type"
	TagFilePattern  = "file_pattern"
	TagResourceSpec = "resource_spec"

	// Conditional validation tags
	TagRequiredIf      = "required_if"
	TagRequiredUnless  = "required_unless"
	TagRequiredWith    = "required_with"
	TagRequiredWithout = "required_without"

	// Field comparison tags
	TagEqField  = "eqfield"
	TagNeField  = "nefield"
	TagGtField  = "gtfield"
	TagLtField  = "ltfield"
	TagGteField = "gtefield"
	TagLteField = "ltefield"
)

// ValidationTagDefinition defines validation rules for specific domain objects
type ValidationTagDefinition struct {
	Tag         string
	Description string
	Pattern     *regexp.Regexp
	Validator   func(value interface{}, fieldName string, params map[string]interface{}) error
}

// CommonValidationTags returns the registry of common validation tags
func CommonValidationTags() map[string]ValidationTagDefinition {
	return map[string]ValidationTagDefinition{
		TagGitURL: {
			Tag:         TagGitURL,
			Description: "Validates Git repository URLs (https, ssh, file protocols)",
			Pattern:     regexp.MustCompile(`^(https?|ssh|git|file)://.*\.git$|^git@.*:.*\.git$`),
			Validator:   validateGitURL,
		},

		TagImageName: {
			Tag:         TagImageName,
			Description: "Validates Docker image names according to Docker naming conventions",
			Pattern:     regexp.MustCompile(`^([a-zA-Z0-9._-]+/)?[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*$`),
			Validator:   validateImageName,
		},

		TagDockerImage: {
			Tag:         TagDockerImage,
			Description: "Validates complete Docker image references including optional registry and tag",
			Pattern:     regexp.MustCompile(`^([a-zA-Z0-9._-]+/)?[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*(:([a-zA-Z0-9._-]+))?$`),
			Validator:   validateDockerImage,
		},

		TagDockerTag: {
			Tag:         TagDockerTag,
			Description: "Validates Docker image tags",
			Pattern:     regexp.MustCompile(`^[a-zA-Z0-9._-]+$`),
			Validator:   validateDockerTag,
		},

		TagPlatform: {
			Tag:         TagPlatform,
			Description: "Validates platform strings (linux/amd64, linux/arm64, etc.)",
			Pattern:     regexp.MustCompile(`^(linux|windows|darwin)/(amd64|arm64|arm|386|ppc64le|s390x)$`),
			Validator:   validatePlatform,
		},

		TagK8sName: {
			Tag:         TagK8sName,
			Description: "Validates Kubernetes resource names (RFC 1123 compliant)",
			Pattern:     regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`),
			Validator:   validateK8sName,
		},

		TagNamespace: {
			Tag:         TagNamespace,
			Description: "Validates Kubernetes namespace names",
			Pattern:     regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`),
			Validator:   validateNamespace,
		},

		TagSessionID: {
			Tag:         TagSessionID,
			Description: "Validates session IDs (UUID format)",
			Pattern:     regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
			Validator:   validateSessionID,
		},

		TagFilePath: {
			Tag:         TagFilePath,
			Description: "Validates file paths (no path traversal, reasonable length)",
			Pattern:     nil, // Custom validation logic
			Validator:   validateFilePath,
		},

		TagSecurePath: {
			Tag:         TagSecurePath,
			Description: "Validates paths for security (no ../,  no system directories)",
			Pattern:     nil, // Custom validation logic
			Validator:   validateSecurePath,
		},

		TagNoSensitive: {
			Tag:         TagNoSensitive,
			Description: "Validates that content doesn't contain sensitive data patterns",
			Pattern:     nil, // Custom validation logic
			Validator:   validateNoSensitive,
		},

		TagNoInjection: {
			Tag:         TagNoInjection,
			Description: "Validates input against injection attacks (SQL, command, etc.)",
			Pattern:     nil, // Custom validation logic
			Validator:   validateNoInjection,
		},

		TagPort: {
			Tag:         TagPort,
			Description: "Validates network port numbers (1-65535)",
			Pattern:     nil, // Numeric validation
			Validator:   validatePort,
		},

		TagEndpoint: {
			Tag:         TagEndpoint,
			Description: "Validates endpoint URLs (with port and path)",
			Pattern:     regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(:[0-9]+)?(/.*)?$`),
			Validator:   validateEndpoint,
		},

		TagGitBranch: {
			Tag:         TagGitBranch,
			Description: "Validates Git branch names",
			Pattern:     regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`),
			Validator:   validateGitBranch,
		},

		TagLanguage: {
			Tag:         TagLanguage,
			Description: "Validates programming language names",
			Pattern:     nil,
			Validator:   validateLanguage,
		},

		TagFramework: {
			Tag:         TagFramework,
			Description: "Validates framework names",
			Pattern:     nil,
			Validator:   validateFramework,
		},

		TagSeverity: {
			Tag:         TagSeverity,
			Description: "Validates security severity levels",
			Pattern:     nil,
			Validator:   validateSeverity,
		},

		TagServiceType: {
			Tag:         TagServiceType,
			Description: "Validates Kubernetes service types",
			Pattern:     nil,
			Validator:   validateServiceType,
		},

		TagFilePattern: {
			Tag:         TagFilePattern,
			Description: "Validates file glob patterns",
			Pattern:     nil,
			Validator:   validateFilePattern,
		},

		TagRegistryURL: {
			Tag:         TagRegistryURL,
			Description: "Validates container registry URLs",
			Pattern:     regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](:[0-9]+)?$`),
			Validator:   validateRegistryURL,
		},

		TagResourceSpec: {
			Tag:         TagResourceSpec,
			Description: "Validates Kubernetes resource specifications (CPU/memory)",
			Pattern:     nil,
			Validator:   validateResourceSpec,
		},

		TagDomain: {
			Tag:         TagDomain,
			Description: "Validates domain names",
			Pattern:     regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`),
			Validator:   validateDomain,
		},

		TagK8sSelector: {
			Tag:         TagK8sSelector,
			Description: "Validates Kubernetes label selectors",
			Pattern:     nil,
			Validator:   validateK8sSelector,
		},

		TagVulnType: {
			Tag:         TagVulnType,
			Description: "Validates vulnerability types for security scanning",
			Pattern:     nil,
			Validator:   validateVulnType,
		},
	}
}

// Validation function implementations

func validateGitURL(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "git URL cannot be empty")
	}

	// Allow various Git URL formats
	gitPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^https?://.*\.git$`),                   // HTTPS
		regexp.MustCompile(`^git@[^:]+:.*\.git$`),                  // SSH
		regexp.MustCompile(`^ssh://git@[^/]+/.*\.git$`),            // SSH with ssh://
		regexp.MustCompile(`^file://.*\.git$`),                     // Local file
		regexp.MustCompile(`^https?://github\.com/[^/]+/[^/]+/?$`), // GitHub without .git
		regexp.MustCompile(`^https?://gitlab\.com/[^/]+/[^/]+/?$`), // GitLab without .git
	}

	for _, pattern := range gitPatterns {
		if pattern.MatchString(str) {
			return nil
		}
	}

	return NewValidationError(fieldName, "must be a valid Git URL (https://repo.git, git@host:repo.git, etc.)")
}

func validateImageName(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "image name cannot be empty")
	}

	// Docker image name validation
	// - Can have optional registry prefix
	// - Must have repository name
	// - Can have optional tag
	parts := strings.Split(str, ":")
	imagePart := parts[0]

	// Validate image name part (without tag)
	if !regexp.MustCompile(`^([a-zA-Z0-9._-]+/)?[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*$`).MatchString(imagePart) {
		return NewValidationError(fieldName, "must be a valid Docker image name (alphanumeric, dots, hyphens, underscores, and slashes only)")
	}

	// Validate tag part if present
	if len(parts) > 2 {
		return NewValidationError(fieldName, "can only have one tag (one colon allowed)")
	}

	if len(parts) == 2 {
		tag := parts[1]
		// Allow empty tag (registry:port/image format)
		if tag != "" && !regexp.MustCompile(`^[a-zA-Z0-9._-]+$`).MatchString(tag) {
			return NewValidationError(fieldName, "tag must contain only alphanumeric characters, dots, hyphens, and underscores")
		}
	}

	return nil
}

func validateDockerImage(value interface{}, fieldName string, params map[string]interface{}) error {
	// DockerImage includes image name + optional tag validation
	return validateImageName(value, fieldName, params)
}

func validateDockerTag(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "Docker tag cannot be empty")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9._-]+$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid Docker tag (alphanumeric, dots, hyphens, underscores only)")
	}

	return nil
}

func validatePlatform(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "platform cannot be empty")
	}

	validPlatforms := []string{
		"linux/amd64", "linux/arm64", "linux/arm", "linux/386", "linux/ppc64le", "linux/s390x",
		"windows/amd64", "windows/386",
		"darwin/amd64", "darwin/arm64",
	}

	for _, platform := range validPlatforms {
		if str == platform {
			return nil
		}
	}

	return NewValidationError(fieldName, "must be a valid platform (e.g., linux/amd64, linux/arm64, windows/amd64)")
}

func validateK8sName(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "Kubernetes name cannot be empty")
	}

	// RFC 1123 compliant names
	if len(str) > 63 {
		return NewValidationError(fieldName, "Kubernetes name must be 63 characters or less")
	}

	if !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid Kubernetes name (lowercase alphanumeric and hyphens, start/end with alphanumeric)")
	}

	return nil
}

func validateNamespace(value interface{}, fieldName string, params map[string]interface{}) error {
	// Namespace validation is the same as K8s name validation
	return validateK8sName(value, fieldName, params)
}

func validateSessionID(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "session ID cannot be empty")
	}

	// UUID format validation
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid UUID format")
	}

	return nil
}

func validateFilePath(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "file path cannot be empty")
	}

	// Basic path validation
	if len(str) > 4096 {
		return NewValidationError(fieldName, "file path too long (max 4096 characters)")
	}

	// Check for null bytes
	if strings.Contains(str, "\x00") {
		return NewValidationError(fieldName, "file path cannot contain null bytes")
	}

	return nil
}

func validateSecurePath(value interface{}, fieldName string, params map[string]interface{}) error {
	// First run basic file path validation
	if err := validateFilePath(value, fieldName, params); err != nil {
		return err
	}

	str := value.(string)

	// Security checks
	if strings.Contains(str, "..") {
		return NewValidationError(fieldName, "path cannot contain '..' (path traversal)")
	}

	// Check for system directories
	systemDirs := []string{"/etc/", "/sys/", "/proc/", "/dev/", "/root/"}
	for _, sysDir := range systemDirs {
		if strings.HasPrefix(str, sysDir) {
			return NewValidationError(fieldName, "path cannot access system directories")
		}
	}

	return nil
}

func validateNoSensitive(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	// Check for common sensitive data patterns
	sensitivePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password|passwd|pwd)[\s]*[=:]\s*['"']?[a-zA-Z0-9@#$%^&*()_+=-]+`),
		regexp.MustCompile(`(?i)(api[_-]?key|apikey)[\s]*[=:]\s*['"']?[a-zA-Z0-9@#$%^&*()_+=-]+`),
		regexp.MustCompile(`(?i)(secret|token)[\s]*[=:]\s*['"']?[a-zA-Z0-9@#$%^&*()_+=-]+`),
		regexp.MustCompile(`-----BEGIN [A-Z]+ PRIVATE KEY-----`),
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`), // AWS Access Key
	}

	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(str) {
			return NewValidationError(fieldName, "content appears to contain sensitive data")
		}
	}

	return nil
}

func validateNoInjection(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	// Check for injection patterns
	injectionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop|create|alter)\s+`), // SQL injection
		regexp.MustCompile(`[;&|` + "`" + `$(){}[\]\\]`),                                   // Command injection
		regexp.MustCompile(`<script[^>]*>.*?</script>`),                                    // XSS
		regexp.MustCompile(`javascript:`),                                                  // JavaScript injection
	}

	for _, pattern := range injectionPatterns {
		if pattern.MatchString(str) {
			return NewValidationError(fieldName, "content contains potential injection attack patterns")
		}
	}

	return nil
}

func validatePort(value interface{}, fieldName string, params map[string]interface{}) error {
	var port int

	switch v := value.(type) {
	case int:
		port = v
	case string:
		// Allow string representation of ports
		if v == "" {
			return NewValidationError(fieldName, "port cannot be empty")
		}
		// Try to parse as integer
		if p, err := regexp.MatchString(`^\d+$`, v); err == nil && p {
			// Convert to int for validation
			for i, r := range v {
				if i == 0 {
					port = int(r - '0')
				} else {
					port = port*10 + int(r-'0')
				}
			}
		} else {
			return NewValidationError(fieldName, "port must be a valid integer")
		}
	default:
		return NewValidationError(fieldName, "port must be an integer")
	}

	if port < 1 || port > 65535 {
		return NewValidationError(fieldName, "port must be between 1 and 65535")
	}

	return nil
}

func validateEndpoint(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "endpoint cannot be empty")
	}

	// Validate endpoint URL format
	if !regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(:[0-9]+)?(/.*)?$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid HTTP/HTTPS endpoint")
	}

	return nil
}

func validateGitBranch(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "Git branch name cannot be empty")
	}

	// Git branch name validation
	if !regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid Git branch name (alphanumeric, dots, hyphens, underscores, slashes)")
	}

	// Check for invalid sequences
	if strings.Contains(str, "..") || strings.HasPrefix(str, "/") || strings.HasSuffix(str, "/") {
		return NewValidationError(fieldName, "branch name cannot contain '..' or start/end with '/'")
	}

	return nil
}

func validateLanguage(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "language cannot be empty")
	}

	// Common programming languages
	validLanguages := []string{
		"go", "golang", "python", "javascript", "typescript", "java", "c", "cpp", "c++",
		"csharp", "c#", "rust", "php", "ruby", "kotlin", "swift", "dart", "scala",
		"clojure", "elixir", "erlang", "haskell", "lua", "perl", "r", "shell", "bash",
		"powershell", "yaml", "json", "xml", "html", "css", "sql", "dockerfile",
	}

	lowerStr := strings.ToLower(str)
	for _, lang := range validLanguages {
		if lowerStr == lang {
			return nil
		}
	}

	return NewValidationError(fieldName, "must be a supported programming language")
}

func validateFramework(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "framework cannot be empty")
	}

	// Common frameworks - keeping this flexible as there are many
	validFrameworks := []string{
		// JavaScript/TypeScript
		"express", "koa", "fastify", "nestjs", "react", "vue", "angular", "next", "nuxt",
		// Python
		"django", "flask", "fastapi", "tornado", "pyramid", "bottle",
		// Go
		"gin", "echo", "fiber", "chi", "gorilla", "beego",
		// Java
		"spring", "springboot", "quarkus", "micronaut", "dropwizard",
		// .NET
		"aspnet", "asp.net", "dotnet", ".net",
		// PHP
		"laravel", "symfony", "codeigniter", "slim",
		// Ruby
		"rails", "sinatra", "hanami",
		// Others
		"none", "custom", "unknown",
	}

	lowerStr := strings.ToLower(str)
	for _, framework := range validFrameworks {
		if lowerStr == framework {
			return nil
		}
	}

	return NewValidationError(fieldName, "must be a supported framework")
}

func validateSeverity(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "severity cannot be empty")
	}

	validSeverities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL", "INFO"}
	upperStr := strings.ToUpper(str)

	for _, severity := range validSeverities {
		if upperStr == severity {
			return nil
		}
	}

	return NewValidationError(fieldName, "must be one of: LOW, MEDIUM, HIGH, CRITICAL, INFO")
}

func validateServiceType(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "service type cannot be empty")
	}

	validServiceTypes := []string{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"}

	for _, serviceType := range validServiceTypes {
		if str == serviceType {
			return nil
		}
	}

	return NewValidationError(fieldName, "must be one of: ClusterIP, NodePort, LoadBalancer, ExternalName")
}

func validateFilePattern(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "file pattern cannot be empty")
	}

	// Basic glob pattern validation
	// Check for dangerous patterns
	if strings.Contains(str, "../") {
		return NewValidationError(fieldName, "file pattern cannot contain path traversal sequences")
	}

	// Validate that it's a reasonable glob pattern
	if len(str) > 255 {
		return NewValidationError(fieldName, "file pattern too long (max 255 characters)")
	}

	return nil
}

func validateRegistryURL(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "registry URL cannot be empty")
	}

	// Validate registry URL format
	// Must be hostname, optionally with port
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9](:[0-9]+)?$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid registry URL (hostname or hostname:port)")
	}

	// Check for reasonable hostname length
	parts := strings.Split(str, ":")
	hostname := parts[0]
	if len(hostname) > 253 {
		return NewValidationError(fieldName, "hostname too long (max 253 characters)")
	}

	// Check port if present
	if len(parts) == 2 {
		port := parts[1]
		if len(port) == 0 {
			return NewValidationError(fieldName, "port cannot be empty")
		}
		// Port validation (already handled by regex but double-check)
		for _, r := range port {
			if r < '0' || r > '9' {
				return NewValidationError(fieldName, "port must be numeric")
			}
		}
		// Convert to check range
		portNum := 0
		for _, r := range port {
			portNum = portNum*10 + int(r-'0')
		}
		if portNum < 1 || portNum > 65535 {
			return NewValidationError(fieldName, "port must be between 1 and 65535")
		}
	}

	return nil
}

func validateResourceSpec(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "resource specification cannot be empty")
	}

	// Validate Kubernetes resource format (CPU: 100m, 1, 1.5; Memory: 128Mi, 1Gi, etc.)
	cpuPattern := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(m|[kMGT])?$`)
	memoryPattern := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?([kMGT]i?)?$`)

	// Check if it matches CPU pattern
	if cpuPattern.MatchString(str) {
		return nil
	}

	// Check if it matches memory pattern
	if memoryPattern.MatchString(str) {
		return nil
	}

	return NewValidationError(fieldName, "must be a valid Kubernetes resource specification (e.g., '100m', '1Gi', '0.5')")
}

func validateDomain(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "domain name cannot be empty")
	}

	// Basic domain validation
	if len(str) > 253 {
		return NewValidationError(fieldName, "domain name too long (max 253 characters)")
	}

	// Check domain format
	if !regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid domain name")
	}

	// Check for valid TLD (basic check)
	parts := strings.Split(str, ".")
	if len(parts) < 2 {
		return NewValidationError(fieldName, "domain must have at least one dot (e.g., example.com)")
	}

	// Check individual labels
	for _, part := range parts {
		if len(part) == 0 {
			return NewValidationError(fieldName, "domain parts cannot be empty")
		}
		if len(part) > 63 {
			return NewValidationError(fieldName, "domain parts must be 63 characters or less")
		}
	}

	return nil
}

func validateK8sSelector(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "Kubernetes selector cannot be empty")
	}

	// Simple validation - check basic format like "app=myapp,version=v1"
	if !regexp.MustCompile(`^[a-zA-Z0-9._/-]+=[a-zA-Z0-9._/-]*(,[a-zA-Z0-9._/-]+=[a-zA-Z0-9._/-]*)*$`).MatchString(str) {
		return NewValidationError(fieldName, "must be a valid Kubernetes label selector (e.g., 'app=myapp,version=v1')")
	}

	// Check individual selector parts
	parts := strings.Split(str, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, "=") {
			return NewValidationError(fieldName, "each selector must contain '=' (e.g., 'key=value')")
		}

		keyValue := strings.SplitN(part, "=", 2)
		key := keyValue[0]
		value := keyValue[1]

		// Validate key
		if !regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`).MatchString(key) {
			return NewValidationError(fieldName, "selector key must contain only alphanumeric characters, dots, hyphens, underscores, and slashes")
		}

		// Validate value
		if !regexp.MustCompile(`^[a-zA-Z0-9._/-]*$`).MatchString(value) {
			return NewValidationError(fieldName, "selector value must contain only alphanumeric characters, dots, hyphens, underscores, and slashes")
		}
	}

	return nil
}

func validateVulnType(value interface{}, fieldName string, params map[string]interface{}) error {
	str, ok := value.(string)
	if !ok {
		return NewValidationError(fieldName, "must be a string")
	}

	if str == "" {
		return NewValidationError(fieldName, "vulnerability type cannot be empty")
	}

	validVulnTypes := []string{"os", "library", "application", "config", "secret", "malware", "all"}
	lowerStr := strings.ToLower(str)

	for _, vulnType := range validVulnTypes {
		if lowerStr == vulnType {
			return nil
		}
	}

	return NewValidationError(fieldName, "must be one of: os, library, application, config, secret, malware, all")
}

// NewValidationError creates a validation error
func NewValidationError(field, message string) error {
	return &Error{
		Field:    field,
		Message:  message,
		Code:     "VALIDATION_FAILED",
		Severity: SeverityHigh,
		Context:  make(map[string]string),
	}
}
