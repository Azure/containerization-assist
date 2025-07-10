package validation

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// validateSecurityEnhanced validates security contexts and policies - comprehensive version
func (v *KubernetesManifestValidator) validateSecurityEnhanced(manifest map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	kind, _ := manifest["kind"].(string)

	// Security validation is mainly applicable to Pod, Deployment, DaemonSet, StatefulSet
	if !v.hasSecurityContextEnhanced(kind) {
		return errs, warnings
	}

	spec, hasSpec := manifest["spec"]
	if !hasSpec {
		return errs, warnings
	}

	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return errs, warnings
	}

	// For Deployment, DaemonSet, StatefulSet, check template.spec
	if kind == "Deployment" || kind == "DaemonSet" || kind == "StatefulSet" {
		if template, ok := specMap["template"].(map[string]interface{}); ok {
			if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
				if secErrs, secWarnings := v.validatePodSecurityContextEnhanced(templateSpec); len(secErrs) > 0 {
					errs = append(errs, secErrs...)
				} else {
					warnings = append(warnings, secWarnings...)
				}
			}
		}
	} else if kind == "Pod" {
		// Direct Pod security validation
		if secErrs, secWarnings := v.validatePodSecurityContextEnhanced(specMap); len(secErrs) > 0 {
			errs = append(errs, secErrs...)
		} else {
			warnings = append(warnings, secWarnings...)
		}
	}

	return errs, warnings
}

// hasSecurityContextEnhanced checks if a resource kind can have security contexts
func (v *KubernetesManifestValidator) hasSecurityContextEnhanced(kind string) bool {
	securityContextKinds := map[string]bool{
		"Pod":         true,
		"Deployment":  true,
		"DaemonSet":   true,
		"StatefulSet": true,
		"Job":         true,
		"CronJob":     true,
	}
	return securityContextKinds[kind]
}

// validatePodSecurityContextEnhanced validates Pod-level and container-level security contexts with enhanced checks
func (v *KubernetesManifestValidator) validatePodSecurityContextEnhanced(podSpec map[string]interface{}) ([]error, []string) {
	var errs []error
	var warnings []string

	// Validate Pod-level security context
	if securityContext, ok := podSpec["securityContext"].(map[string]interface{}); ok {
		if secErrs, secWarnings := v.validateSecurityContextObject(securityContext, "spec.securityContext"); len(secErrs) > 0 {
			errs = append(errs, secErrs...)
		} else {
			warnings = append(warnings, secWarnings...)
		}
	} else {
		warnings = append(warnings, "No Pod-level security context specified - consider adding security constraints")
	}

	// Validate container security contexts
	if containers, ok := podSpec["containers"].([]interface{}); ok {
		for i, container := range containers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if securityContext, ok := containerMap["securityContext"].(map[string]interface{}); ok {
					path := fmt.Sprintf("spec.containers[%d].securityContext", i)
					if secErrs, secWarnings := v.validateContainerSecurityContext(securityContext, path); len(secErrs) > 0 {
						errs = append(errs, secErrs...)
					} else {
						warnings = append(warnings, secWarnings...)
					}
				}

				// Check for privileged containers
				if securityContext, ok := containerMap["securityContext"].(map[string]interface{}); ok {
					if privileged, ok := securityContext["privileged"].(bool); ok && privileged {
						errs = append(errs, errors.NewSecurityError(
							"privileged containers are not allowed",
							map[string]interface{}{
								"container": i,
								"policy":    "no_privileged_containers",
							},
						))
					}
				}
			}
		}
	}

	// Validate init containers security contexts
	if initContainers, ok := podSpec["initContainers"].([]interface{}); ok {
		for i, container := range initContainers {
			if containerMap, ok := container.(map[string]interface{}); ok {
				if securityContext, ok := containerMap["securityContext"].(map[string]interface{}); ok {
					path := fmt.Sprintf("spec.initContainers[%d].securityContext", i)
					if secErrs, secWarnings := v.validateContainerSecurityContext(securityContext, path); len(secErrs) > 0 {
						errs = append(errs, secErrs...)
					} else {
						warnings = append(warnings, secWarnings...)
					}
				}
			}
		}
	}

	// Validate host-level security settings
	if hostNetwork, ok := podSpec["hostNetwork"].(bool); ok && hostNetwork {
		warnings = append(warnings, "Pod uses host network - ensure this is necessary for security")
	}

	if hostPID, ok := podSpec["hostPID"].(bool); ok && hostPID {
		warnings = append(warnings, "Pod uses host PID namespace - ensure this is necessary for security")
	}

	if hostIPC, ok := podSpec["hostIPC"].(bool); ok && hostIPC {
		warnings = append(warnings, "Pod uses host IPC namespace - ensure this is necessary for security")
	}

	return errs, warnings
}

// validateSecurityContextObject validates a security context object (Pod-level)
func (v *KubernetesManifestValidator) validateSecurityContextObject(securityContext map[string]interface{}, path string) ([]error, []string) {
	var errs []error
	var warnings []string

	// Validate runAsUser
	if runAsUser, ok := securityContext["runAsUser"]; ok {
		var uid int
		switch u := runAsUser.(type) {
		case int:
			uid = u
		case float64:
			uid = int(u)
		default:
			errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.runAsUser", path), "runAsUser must be a number"))
		}

		if uid == 0 {
			warnings = append(warnings, "Running as root user (UID 0) - consider using a non-root user for security")
		}
	}

	// Validate runAsGroup
	if runAsGroup, ok := securityContext["runAsGroup"]; ok {
		switch runAsGroup.(type) {
		case int, float64:
			// Valid
		default:
			errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.runAsGroup", path), "runAsGroup must be a number"))
		}
	}

	// Validate runAsNonRoot
	if runAsNonRoot, ok := securityContext["runAsNonRoot"].(bool); ok {
		if !runAsNonRoot {
			warnings = append(warnings, "runAsNonRoot is set to false - container may run as root")
		}
	} else {
		warnings = append(warnings, "runAsNonRoot not specified - consider setting to true for security")
	}

	// Validate readOnlyRootFilesystem
	if readOnlyRootFilesystem, ok := securityContext["readOnlyRootFilesystem"].(bool); ok {
		if !readOnlyRootFilesystem {
			warnings = append(warnings, "readOnlyRootFilesystem is set to false - consider making root filesystem read-only")
		}
	} else {
		warnings = append(warnings, "readOnlyRootFilesystem not specified - consider setting to true for security")
	}

	// Validate fsGroup
	if fsGroup, ok := securityContext["fsGroup"]; ok {
		switch fsGroup.(type) {
		case int, float64:
			// Valid
		default:
			errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.fsGroup", path), "fsGroup must be a number"))
		}
	}

	return errs, warnings
}

// validateContainerSecurityContext validates container-level security context
func (v *KubernetesManifestValidator) validateContainerSecurityContext(securityContext map[string]interface{}, path string) ([]error, []string) {
	var errs []error
	var warnings []string

	// Check for privileged containers (this is critical)
	if privileged, ok := securityContext["privileged"].(bool); ok && privileged {
		errs = append(errs, errors.NewSecurityError(
			"privileged containers are not allowed",
			map[string]interface{}{
				"path":   path,
				"policy": "no_privileged_containers",
			},
		))
	}

	// Validate allowPrivilegeEscalation
	if allowPrivilegeEscalation, ok := securityContext["allowPrivilegeEscalation"].(bool); ok && allowPrivilegeEscalation {
		warnings = append(warnings, "allowPrivilegeEscalation is set to true - consider setting to false for security")
	}

	// Validate capabilities
	if capabilities, ok := securityContext["capabilities"].(map[string]interface{}); ok {
		if capErrs, capWarnings := v.validateCapabilities(capabilities, path); len(capErrs) > 0 {
			errs = append(errs, capErrs...)
		} else {
			warnings = append(warnings, capWarnings...)
		}
	}

	// Validate runAsUser (container-level)
	if runAsUser, ok := securityContext["runAsUser"]; ok {
		var uid int
		switch u := runAsUser.(type) {
		case int:
			uid = u
		case float64:
			uid = int(u)
		default:
			errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.runAsUser", path), "runAsUser must be a number"))
		}

		if uid == 0 {
			warnings = append(warnings, "Container running as root user (UID 0) - consider using a non-root user")
		}
	}

	// Validate runAsNonRoot (container-level)
	if runAsNonRoot, ok := securityContext["runAsNonRoot"].(bool); ok {
		if !runAsNonRoot {
			warnings = append(warnings, "Container runAsNonRoot is set to false - container may run as root")
		}
	}

	// Validate readOnlyRootFilesystem (container-level)
	if readOnlyRootFilesystem, ok := securityContext["readOnlyRootFilesystem"].(bool); ok {
		if !readOnlyRootFilesystem {
			warnings = append(warnings, "Container readOnlyRootFilesystem is set to false - consider making root filesystem read-only")
		}
	}

	return errs, warnings
}

// validateCapabilities validates Linux capabilities
func (v *KubernetesManifestValidator) validateCapabilities(capabilities map[string]interface{}, path string) ([]error, []string) {
	var errs []error
	var warnings []string

	// Validate add capabilities
	if add, ok := capabilities["add"].([]interface{}); ok {
		for _, cap := range add {
			if capStr, ok := cap.(string); ok {
				if err := v.validateCapability(capStr, "add"); err != nil {
					errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.capabilities.add", path), err.Error()))
				} else if v.isDangerousCapability(capStr) {
					warnings = append(warnings, fmt.Sprintf("Adding dangerous capability '%s' - ensure this is necessary", capStr))
				}
			} else {
				errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.capabilities.add", path), "capability must be a string"))
			}
		}
	}

	// Validate drop capabilities
	if drop, ok := capabilities["drop"].([]interface{}); ok {
		for _, cap := range drop {
			if capStr, ok := cap.(string); ok {
				if err := v.validateCapability(capStr, "drop"); err != nil {
					errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.capabilities.drop", path), err.Error()))
				}
			} else {
				errs = append(errs, errors.NewValidationFailed(fmt.Sprintf("%s.capabilities.drop", path), "capability must be a string"))
			}
		}
	}

	return errs, warnings
}

// validateCapability validates a single Linux capability
func (v *KubernetesManifestValidator) validateCapability(capability, _ string) error {
	// Common Linux capabilities
	validCapabilities := map[string]bool{
		"CAP_CHOWN":            true,
		"CAP_DAC_OVERRIDE":     true,
		"CAP_DAC_READ_SEARCH":  true,
		"CAP_FOWNER":           true,
		"CAP_FSETID":           true,
		"CAP_KILL":             true,
		"CAP_SETGID":           true,
		"CAP_SETUID":           true,
		"CAP_SETPCAP":          true,
		"CAP_LINUX_IMMUTABLE":  true,
		"CAP_NET_BIND_SERVICE": true,
		"CAP_NET_BROADCAST":    true,
		"CAP_NET_ADMIN":        true,
		"CAP_NET_RAW":          true,
		"CAP_IPC_LOCK":         true,
		"CAP_IPC_OWNER":        true,
		"CAP_SYS_MODULE":       true,
		"CAP_SYS_RAWIO":        true,
		"CAP_SYS_CHROOT":       true,
		"CAP_SYS_PTRACE":       true,
		"CAP_SYS_PACCT":        true,
		"CAP_SYS_ADMIN":        true,
		"CAP_SYS_BOOT":         true,
		"CAP_SYS_NICE":         true,
		"CAP_SYS_RESOURCE":     true,
		"CAP_SYS_TIME":         true,
		"CAP_SYS_TTY_CONFIG":   true,
		"CAP_MKNOD":            true,
		"CAP_LEASE":            true,
		"CAP_AUDIT_WRITE":      true,
		"CAP_AUDIT_CONTROL":    true,
		"CAP_SETFCAP":          true,
		"CAP_MAC_OVERRIDE":     true,
		"CAP_MAC_ADMIN":        true,
		"CAP_SYSLOG":           true,
		"CAP_WAKE_ALARM":       true,
		"CAP_BLOCK_SUSPEND":    true,
		"ALL":                  true, // Special case for dropping all capabilities
	}

	if !validCapabilities[capability] {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Messagef("unknown capability '%s'", capability).
			WithLocation().
			Build()
	}

	return nil
}

// isDangerousCapability checks if a capability is considered dangerous
func (v *KubernetesManifestValidator) isDangerousCapability(capability string) bool {
	dangerousCapabilities := map[string]bool{
		"CAP_SYS_ADMIN":    true,
		"CAP_SYS_MODULE":   true,
		"CAP_SYS_RAWIO":    true,
		"CAP_SYS_PTRACE":   true,
		"CAP_SYS_BOOT":     true,
		"CAP_NET_ADMIN":    true,
		"CAP_DAC_OVERRIDE": true,
	}

	return dangerousCapabilities[capability]
}

// KubernetesSecurityValidator provides security-focused validation for Kubernetes resources
type KubernetesSecurityValidator struct {
	name                     string
	allowPrivileged          bool
	allowHostNetwork         bool
	allowedCapabilities      []string
	requiredDropCapabilities []string
}

// NewKubernetesSecurityValidator creates a security-focused Kubernetes validator
func NewKubernetesSecurityValidator() *KubernetesSecurityValidator {
	return &KubernetesSecurityValidator{
		name:             "KubernetesSecurityValidator",
		allowPrivileged:  false,
		allowHostNetwork: false,
		allowedCapabilities: []string{
			"CAP_NET_BIND_SERVICE", // Common capability for binding to privileged ports
		},
		requiredDropCapabilities: []string{
			"CAP_NET_RAW", // Often dropped for security
		},
	}
}

func (v *KubernetesSecurityValidator) Validate(_ context.Context, value interface{}) ValidationResult {
	manifest, ok := value.(map[string]interface{})
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []error{errors.NewValidationFailed("input", "expected map[string]interface{} for Kubernetes manifest")},
		}
	}

	// Reuse the comprehensive security validation from KubernetesManifestValidator
	manifestValidator := NewKubernetesManifestValidator()
	errs, warnings := manifestValidator.validateSecurityEnhanced(manifest)

	return ValidationResult{
		Valid:    len(errs) == 0,
		Errors:   errs,
		Warnings: warnings,
	}
}

func (v *KubernetesSecurityValidator) Name() string {
	return v.name
}

func (v *KubernetesSecurityValidator) Domain() string {
	return "security"
}

func (v *KubernetesSecurityValidator) Category() string {
	return "policy"
}

func (v *KubernetesSecurityValidator) Priority() int {
	return 200 // Highest priority - security is critical
}

func (v *KubernetesSecurityValidator) Dependencies() []string {
	return []string{"KubernetesManifestValidator"} // Depends on basic manifest validation
}
