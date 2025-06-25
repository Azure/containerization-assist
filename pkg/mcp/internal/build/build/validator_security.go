package build

import (
	"fmt"
	"strings"
)

// Additional security validation methods

func (v *BuildValidatorImpl) validateUserInstruction(parts []string, lineNum int, result *ValidationResult) {
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: "USER instruction requires a username or UID",
			Rule:    "user-syntax",
		})
		result.Valid = false
		return
	}

	user := parts[1]
	if user == "root" || user == "0" {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    lineNum,
			Message: "Running as root user is not recommended",
			Rule:    "no-root-user",
		})
	}
}

func (v *BuildValidatorImpl) validateExposeInstruction(parts []string, lineNum int, result *ValidationResult) {
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: "EXPOSE instruction requires at least one port",
			Rule:    "expose-syntax",
		})
		result.Valid = false
		return
	}

	for i := 1; i < len(parts); i++ {
		port := parts[i]
		// Remove protocol suffix if present
		port = strings.TrimSuffix(port, "/tcp")
		port = strings.TrimSuffix(port, "/udp")

		// Validate port is numeric
		// In a real implementation, we'd parse and validate the port number
		result.Info = append(result.Info, fmt.Sprintf("Exposing port: %s", parts[i]))
	}
}

func (v *BuildValidatorImpl) validateEnvArgInstruction(parts []string, lineNum int, result *ValidationResult, instruction string) {
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: fmt.Sprintf("%s instruction requires a name and optional value", instruction),
			Rule:    "env-arg-syntax",
		})
		result.Valid = false
		return
	}

	// Check for sensitive variable names
	varName := parts[1]
	if strings.Contains(varName, "=") {
		varName = strings.Split(varName, "=")[0]
	}

	sensitiveVars := []string{
		"PASSWORD", "TOKEN", "SECRET", "KEY", "CERT",
	}

	for _, sensitive := range sensitiveVars {
		if strings.Contains(strings.ToUpper(varName), sensitive) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    lineNum,
				Message: fmt.Sprintf("Potential sensitive data in %s: %s", instruction, varName),
				Rule:    "sensitive-env",
			})
		}
	}
}

func (v *BuildValidatorImpl) validateWorkdirInstruction(parts []string, lineNum int, result *ValidationResult) {
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: "WORKDIR instruction requires a path",
			Rule:    "workdir-syntax",
		})
		result.Valid = false
		return
	}

	workdir := parts[1]
	if !strings.HasPrefix(workdir, "/") && !strings.HasPrefix(workdir, "$") {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    lineNum,
			Message: "WORKDIR should use absolute paths",
			Rule:    "workdir-absolute",
		})
	}
}

func (v *BuildValidatorImpl) validateCmdEntrypointInstruction(parts []string, lineNum int, result *ValidationResult, instruction string) {
	if len(parts) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineNum,
			Message: fmt.Sprintf("%s instruction requires a command", instruction),
			Rule:    "cmd-entrypoint-syntax",
		})
		result.Valid = false
		return
	}

	// Check for shell form vs exec form
	if !strings.HasPrefix(parts[1], "[") {
		result.Info = append(result.Info, fmt.Sprintf("%s uses shell form, consider exec form for better signal handling", instruction))
	}
}

func (v *BuildValidatorImpl) checkNetworkExposure(lines []string, result *SecurityValidationResult) {
	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Check for EXPOSE instruction
		if strings.HasPrefix(strings.ToUpper(line), "EXPOSE") {
			parts := strings.Fields(line)
			for j := 1; j < len(parts); j++ {
				port := strings.TrimSuffix(parts[j], "/tcp")
				port = strings.TrimSuffix(port, "/udp")

				// Check for privileged ports
				if port == "22" || port == "23" || port == "21" {
					result.MediumIssues = append(result.MediumIssues, SecurityIssue{
						Severity:    "MEDIUM",
						Type:        "privileged-port",
						Message:     fmt.Sprintf("Exposing potentially dangerous port: %s", port),
						Line:        i + 1,
						Remediation: "Consider if this port really needs to be exposed",
					})
				}
			}
		}
	}
}

func (v *BuildValidatorImpl) checkPackageManagement(lines []string, result *SecurityValidationResult) {
	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Check RUN instructions
		if strings.HasPrefix(strings.ToUpper(line), "RUN") {
			runCmd := strings.TrimPrefix(strings.ToUpper(line), "RUN")
			runCmd = strings.TrimSpace(runCmd)

			// Check for package updates
			if strings.Contains(line, "apt-get upgrade") || strings.Contains(line, "yum upgrade") {
				result.LowIssues = append(result.LowIssues, SecurityIssue{
					Severity:    "LOW",
					Type:        "package-upgrade",
					Message:     "Avoid running upgrade in containers, use updated base images instead",
					Line:        i + 1,
					Remediation: "Update the base image version instead of upgrading packages",
				})
			}

			// Check for package verification
			if strings.Contains(line, "curl") || strings.Contains(line, "wget") {
				if !strings.Contains(line, "--verify") && !strings.Contains(line, "gpg") {
					result.MediumIssues = append(result.MediumIssues, SecurityIssue{
						Severity:    "MEDIUM",
						Type:        "unverified-download",
						Message:     "Downloading files without verification",
						Line:        i + 1,
						Remediation: "Verify checksums or signatures of downloaded files",
					})
				}
			}

			// Check for clean up
			if strings.Contains(line, "apt-get install") && !strings.Contains(line, "rm -rf /var/lib/apt/lists") {
				result.BestPractices = append(result.BestPractices, "Consider cleaning package manager cache to reduce image size")
			}
		}
	}
}
