package validation

import "reflect"

// CoreValidators represents the consolidated validation system
// This replaces 35+ scattered validators with 6 core functional areas
type CoreValidators struct {
	registry *CustomValidatorRegistry
	docker   *DockerValidators
	k8s      *KubernetesValidators
	security *SecurityValidators
	network  *NetworkValidators
	config   *ConfigValidators
}

// NewCoreValidators creates a new consolidated validator system
func NewCoreValidators() *CoreValidators {
	return &CoreValidators{
		registry: NewCustomValidatorRegistry(),
		docker:   NewDockerValidators(),
		k8s:      NewKubernetesValidators(),
		security: NewSecurityValidators(),
		network:  NewNetworkValidators(),
		config:   NewConfigValidators(),
	}
}

// Docker returns the Docker/Container validation subsystem
func (cv *CoreValidators) Docker() *DockerValidators {
	return cv.docker
}

// Kubernetes returns the Kubernetes/Deployment validation subsystem
func (cv *CoreValidators) Kubernetes() *KubernetesValidators {
	return cv.k8s
}

// Security returns the Security/Scan validation subsystem
func (cv *CoreValidators) Security() *SecurityValidators {
	return cv.security
}

// Network returns the Network/Infrastructure validation subsystem
func (cv *CoreValidators) Network() *NetworkValidators {
	return cv.network
}

// Config returns the Configuration validation subsystem
func (cv *CoreValidators) Config() *ConfigValidators {
	return cv.config
}

// Registry returns the custom validator registry for DSL-based validation
func (cv *CoreValidators) Registry() *CustomValidatorRegistry {
	return cv.registry
}

// ValidateStruct validates a struct using the tag-based DSL
func (cv *CoreValidators) ValidateStruct(s interface{}) error {
	parser := NewTagParser()
	structType := reflect.TypeOf(s)
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	_, err := parser.ParseStruct(structType)
	if err != nil {
		return err
	}
	return nil
}

// GlobalValidators provides the singleton instance for the consolidated validation system
var GlobalValidators = NewCoreValidators()
