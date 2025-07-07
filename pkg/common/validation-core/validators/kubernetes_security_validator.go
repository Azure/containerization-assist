package validators

import (
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// Security Validation
// This file contains functions for validating Kubernetes security configurations

// performSecurityValidation performs security validation
func (k *KubernetesValidator) performSecurityValidation(manifest map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Check for privileged containers
	if kind, ok := manifest["kind"].(string); ok && strings.ToLower(kind) == "pod" {
		if spec, ok := manifest["spec"].(map[string]interface{}); ok {
			if containers, ok := spec["containers"].([]interface{}); ok {
				for i, container := range containers {
					if containerMap, ok := container.(map[string]interface{}); ok {
						k.validateContainerSecurity(containerMap, fmt.Sprintf("%s.containers[%d]", fieldPrefix, i), result)
					}
				}
			}
		}
	}

	// Check for deployment/daemonset/replicaset security contexts
	if kind, ok := manifest["kind"].(string); ok {
		switch strings.ToLower(kind) {
		case "deployment", "daemonset", "replicaset":
			k.validateWorkloadSecurity(manifest, fieldPrefix, result)
		}
	}
}

// validateContainerSecurity validates container security context
func (k *KubernetesValidator) validateContainerSecurity(container map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	if securityContext, exists := container["securityContext"]; exists {
		if secCtx, ok := securityContext.(map[string]interface{}); ok {
			// Check for privileged
			if privileged, exists := secCtx["privileged"]; exists {
				if privBool, ok := privileged.(bool); ok && privBool {
					result.AddWarning(&core.Warning{
						Error: &core.Error{
							Code:     "PRIVILEGED_CONTAINER",
							Message:  "Container is configured to run in privileged mode",
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityHigh,
							Field:    fieldPrefix + ".securityContext.privileged",
						},
					})
				}
			}

			// Check for runAsRoot
			if runAsUser, exists := secCtx["runAsUser"]; exists {
				if userID, ok := runAsUser.(int); ok && userID == 0 {
					result.AddWarning(&core.Warning{
						Error: &core.Error{
							Code:     "RUN_AS_ROOT",
							Message:  "Container is configured to run as root user",
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityMedium,
							Field:    fieldPrefix + ".securityContext.runAsUser",
						},
					})
				}
			}

			// Check for allowPrivilegeEscalation
			if allowPrivEsc, exists := secCtx["allowPrivilegeEscalation"]; exists {
				if allowBool, ok := allowPrivEsc.(bool); ok && allowBool {
					result.AddWarning(&core.Warning{
						Error: &core.Error{
							Code:     "ALLOW_PRIVILEGE_ESCALATION",
							Message:  "Container allows privilege escalation",
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityMedium,
							Field:    fieldPrefix + ".securityContext.allowPrivilegeEscalation",
						},
					})
				}
			}

			// Check for readOnlyRootFilesystem
			if readOnlyRoot, exists := secCtx["readOnlyRootFilesystem"]; exists {
				if readOnlyBool, ok := readOnlyRoot.(bool); ok && !readOnlyBool {
					result.AddWarning(&core.Warning{
						Error: &core.Error{
							Code:     "WRITABLE_ROOT_FILESYSTEM",
							Message:  "Container has writable root filesystem",
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityLow,
							Field:    fieldPrefix + ".securityContext.readOnlyRootFilesystem",
						},
					})
				}
			}

			// Check for capabilities
			if capabilities, exists := secCtx["capabilities"]; exists {
				if capMap, ok := capabilities.(map[string]interface{}); ok {
					k.validateSecurityCapabilities(capMap, fieldPrefix+".securityContext.capabilities", result)
				}
			}
		}
	}
}

// validateWorkloadSecurity validates security for workload resources
func (k *KubernetesValidator) validateWorkloadSecurity(manifest map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	if spec, ok := manifest["spec"].(map[string]interface{}); ok {
		if template, ok := spec["template"].(map[string]interface{}); ok {
			if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
				// Validate pod security context
				if securityContext, exists := templateSpec["securityContext"]; exists {
					if secCtx, ok := securityContext.(map[string]interface{}); ok {
						k.validatePodSecurityContext(secCtx, fieldPrefix+".template.spec.securityContext", result)
					}
				}

				// Validate container security contexts
				if containers, exists := templateSpec["containers"]; exists {
					if containersList, ok := containers.([]interface{}); ok {
						for i, container := range containersList {
							if containerMap, ok := container.(map[string]interface{}); ok {
								k.validateContainerSecurity(containerMap, fmt.Sprintf("%s.template.spec.containers[%d]", fieldPrefix, i), result)
							}
						}
					}
				}
			}
		}
	}
}

// validatePodSecurityContext validates pod-level security context
func (k *KubernetesValidator) validatePodSecurityContext(secCtx map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Check for runAsRoot at pod level
	if runAsUser, exists := secCtx["runAsUser"]; exists {
		if userID, ok := runAsUser.(int); ok && userID == 0 {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "POD_RUN_AS_ROOT",
					Message:  "Pod is configured to run as root user",
					Type:     core.ErrTypeSecurity,
					Severity: core.SeverityMedium,
					Field:    fieldPrefix + ".runAsUser",
				},
			})
		}
	}

	// Check for privileged at pod level
	if runAsNonRoot, exists := secCtx["runAsNonRoot"]; exists {
		if nonRootBool, ok := runAsNonRoot.(bool); ok && !nonRootBool {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "POD_ALLOWS_ROOT",
					Message:  "Pod explicitly allows running as root",
					Type:     core.ErrTypeSecurity,
					Severity: core.SeverityMedium,
					Field:    fieldPrefix + ".runAsNonRoot",
				},
			})
		}
	}

	// Check for fsGroup
	if fsGroup, exists := secCtx["fsGroup"]; exists {
		if fsGroupID, ok := fsGroup.(int); ok && fsGroupID == 0 {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "FS_GROUP_ROOT",
					Message:  "Pod uses root group for filesystem ownership",
					Type:     core.ErrTypeSecurity,
					Severity: core.SeverityLow,
					Field:    fieldPrefix + ".fsGroup",
				},
			})
		}
	}
}

// validateSecurityCapabilities validates Linux capabilities
func (k *KubernetesValidator) validateSecurityCapabilities(capabilities map[string]interface{}, fieldPrefix string, result *core.NonGenericResult) {
	// Check for dangerous capabilities being added
	if add, exists := capabilities["add"]; exists {
		if addList, ok := add.([]interface{}); ok {
			dangerousCaps := []string{"SYS_ADMIN", "NET_ADMIN", "SYS_TIME", "SYS_MODULE"}
			for _, cap := range addList {
				if capStr, ok := cap.(string); ok {
					for _, dangerousCap := range dangerousCaps {
						if strings.ToUpper(capStr) == dangerousCap {
							result.AddWarning(&core.Warning{
								Error: &core.Error{
									Code:     "DANGEROUS_CAPABILITY",
									Message:  fmt.Sprintf("Container adds dangerous capability: %s", capStr),
									Type:     core.ErrTypeSecurity,
									Severity: core.SeverityHigh,
									Field:    fieldPrefix + ".add",
								},
							})
						}
					}
				}
			}
		}
	}

	// Recommend dropping all capabilities if none are explicitly needed
	if _, hasAdd := capabilities["add"]; !hasAdd {
		if _, hasDrop := capabilities["drop"]; !hasDrop {
			result.AddWarning(&core.Warning{
				Error: &core.Error{
					Code:     "NO_CAPABILITIES_MANAGEMENT",
					Message:  "Consider explicitly dropping unnecessary capabilities",
					Type:     core.ErrTypeSecurity,
					Severity: core.SeverityLow,
					Field:    fieldPrefix,
				},
			})
		}
	}
}
