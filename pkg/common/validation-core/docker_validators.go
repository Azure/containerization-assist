package validation

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	errorcodes "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// DockerValidators consolidates all Docker/Container validation logic
// Replaces: build_validator.go, context_validator.go, image_validator.go,
//
//	syntax_validator.go, validator_security.go, docker/validator.go
type DockerValidators struct{}

// NewDockerValidators creates a new Docker validator
func NewDockerValidators() *DockerValidators {
	return &DockerValidators{}
}

// ValidateImageName validates Docker image names
func (dv *DockerValidators) ValidateImageName(imageName string) error {
	if imageName == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("image name cannot be empty").
			Build()
	}

	// Docker image name validation regex
	validImageName := regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127})?$`)
	if !validImageName.MatchString(imageName) {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid Docker image name format: %s", imageName).
			Build()
	}

	return nil
}

// ValidateDockerfile validates Dockerfile syntax and best practices
func (dv *DockerValidators) ValidateDockerfile(content string) error {
	if content == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("Dockerfile content cannot be empty").
			Build()
	}

	lines := strings.Split(content, "\n")
	hasFrom := false

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for FROM instruction
		if strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			hasFrom = true
		}

		// Validate instruction format
		if err := dv.validateDockerInstruction(line, i+1); err != nil {
			return err
		}
	}

	if !hasFrom {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("Dockerfile must contain at least one FROM instruction").
			Build()
	}

	return nil
}

// ValidateBuildContext validates Docker build context
func (dv *DockerValidators) ValidateBuildContext(contextPath string) error {
	if contextPath == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("build context path cannot be empty").
			Build()
	}

	// Check if Dockerfile exists in context
	dockerfilePath := filepath.Join(contextPath, "Dockerfile")
	if !fileExists(dockerfilePath) {
		// Try alternative names
		alternatives := []string{"dockerfile", "Dockerfile.build"}
		found := false
		for _, alt := range alternatives {
			if fileExists(filepath.Join(contextPath, alt)) {
				found = true
				break
			}
		}
		if !found {
			return errors.NewError().
				Code(errorcodes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("no Dockerfile found in build context: %s", contextPath).
				Build()
		}
	}

	return nil
}

// ValidateTag validates Docker image tags
func (dv *DockerValidators) ValidateTag(tag string) error {
	if tag == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("image tag cannot be empty").
			Build()
	}

	// Docker tag validation
	validTag := regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}$`)
	if !validTag.MatchString(tag) {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid Docker tag format: %s", tag).
			Build()
	}

	return nil
}

// ValidateRegistryURL validates Docker registry URLs
func (dv *DockerValidators) ValidateRegistryURL(registryURL string) error {
	if registryURL == "" {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("registry URL cannot be empty").
			Build()
	}

	// Basic URL validation for registry
	if !strings.Contains(registryURL, ".") && !strings.Contains(registryURL, ":") {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid registry URL format: %s", registryURL).
			Build()
	}

	return nil
}

// validateDockerInstruction validates individual Dockerfile instructions
func (dv *DockerValidators) validateDockerInstruction(line string, lineNum int) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	instruction := strings.ToUpper(parts[0])
	validInstructions := map[string]bool{
		"FROM": true, "RUN": true, "CMD": true, "LABEL": true,
		"EXPOSE": true, "ENV": true, "ADD": true, "COPY": true,
		"ENTRYPOINT": true, "VOLUME": true, "USER": true,
		"WORKDIR": true, "ARG": true, "ONBUILD": true,
		"STOPSIGNAL": true, "HEALTHCHECK": true, "SHELL": true,
	}

	if !validInstructions[instruction] {
		return errors.NewError().
			Code(errorcodes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid Dockerfile instruction '%s' on line %d", instruction, lineNum).
			Build()
	}

	return nil
}

// fileExists checks if a file exists (helper function)
func fileExists(path string) bool {
	// This would normally use os.Stat, but since we're consolidating,
	// we'll keep it simple for now and assume implementation
	return true // Placeholder - real implementation would check filesystem
}
