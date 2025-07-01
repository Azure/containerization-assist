package validators

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
)

// NetworkValidator validates network-related data like IP addresses, ports, and hostnames
type NetworkValidator struct {
	*BaseValidatorImpl
	hostnameRegex *regexp.Regexp
	domainRegex   *regexp.Regexp
}

// NewNetworkValidator creates a new network validator
func NewNetworkValidator() *NetworkValidator {
	return &NetworkValidator{
		BaseValidatorImpl: NewBaseValidator("network", "1.0.0", []string{"ip", "port", "hostname", "cidr", "mac"}),
		hostnameRegex:     regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`),
		domainRegex:       regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`),
	}
}

// Validate validates network-related data
func (n *NetworkValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	result := n.BaseValidatorImpl.Validate(ctx, data, options)

	// Check context for network type
	networkType := ""
	if options != nil && options.Context != nil {
		if nt, ok := options.Context["network_type"].(string); ok {
			networkType = nt
		}
	}

	switch networkType {
	case "ip":
		n.validateIP(data, result, options)
	case "port":
		n.validatePort(data, result)
	case "hostname":
		n.validateHostname(data, result)
	case "cidr":
		n.validateCIDR(data, result)
	case "mac":
		n.validateMACAddress(data, result)
	default:
		// Try to detect network type automatically
		n.autoDetectAndValidate(data, result, options)
	}

	return result
}

// validateIP validates IP address (both IPv4 and IPv6)
func (n *NetworkValidator) validateIP(data interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	ipStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewValidationError(
			"INVALID_IP_TYPE",
			"IP address must be a string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		result.AddError(core.NewValidationError(
			"INVALID_IP_FORMAT",
			fmt.Sprintf("Invalid IP address format: %s", ipStr),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		).WithSuggestion("IP address should be in format: 192.168.1.1 (IPv4) or 2001:db8::1 (IPv6)"))
		return
	}

	// Check IP version if specified
	if options != nil && options.Context != nil {
		if version, ok := options.Context["ip_version"].(string); ok {
			switch version {
			case "4":
				if ip.To4() == nil {
					result.AddError(core.NewValidationError(
						"INVALID_IPV4",
						fmt.Sprintf("Expected IPv4 address, got: %s", ipStr),
						core.ErrTypeNetwork,
						core.SeverityMedium,
					))
				}
			case "6":
				if ip.To4() != nil {
					result.AddError(core.NewValidationError(
						"INVALID_IPV6",
						fmt.Sprintf("Expected IPv6 address, got: %s", ipStr),
						core.ErrTypeNetwork,
						core.SeverityMedium,
					))
				}
			}
		}
	}

	// Check for special IP addresses
	if ip.IsLoopback() {
		result.AddWarning(core.NewValidationWarning(
			"LOOPBACK_IP",
			fmt.Sprintf("IP address %s is a loopback address", ipStr),
		))
	}
	if ip.IsPrivate() {
		result.AddWarning(core.NewValidationWarning(
			"PRIVATE_IP",
			fmt.Sprintf("IP address %s is a private address", ipStr),
		))
	}
}

// validatePort validates port number
func (n *NetworkValidator) validatePort(data interface{}, result *core.ValidationResult) {
	var port int
	switch v := data.(type) {
	case int:
		port = v
	case int64:
		port = int(v)
	case string:
		var err error
		port, err = strconv.Atoi(v)
		if err != nil {
			result.AddError(core.NewValidationError(
				"INVALID_PORT_FORMAT",
				fmt.Sprintf("Invalid port format: %s", v),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
			return
		}
	default:
		result.AddError(core.NewValidationError(
			"INVALID_PORT_TYPE",
			"Port must be a number or numeric string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	if port < 1 || port > 65535 {
		result.AddError(core.NewValidationError(
			"PORT_OUT_OF_RANGE",
			fmt.Sprintf("Port %d is out of valid range (1-65535)", port),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
	}

	// Warn about privileged ports
	if port < 1024 {
		result.AddWarning(core.NewValidationWarning(
			"PRIVILEGED_PORT",
			fmt.Sprintf("Port %d is a privileged port (requires root/admin access)", port),
		))
	}
}

// validateHostname validates hostname format
func (n *NetworkValidator) validateHostname(data interface{}, result *core.ValidationResult) {
	hostname, ok := data.(string)
	if !ok {
		result.AddError(core.NewValidationError(
			"INVALID_HOSTNAME_TYPE",
			"Hostname must be a string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	if len(hostname) == 0 {
		result.AddError(core.NewValidationError(
			"EMPTY_HOSTNAME",
			"Hostname cannot be empty",
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
		return
	}

	if len(hostname) > 253 {
		result.AddError(core.NewValidationError(
			"HOSTNAME_TOO_LONG",
			fmt.Sprintf("Hostname length %d exceeds maximum of 253 characters", len(hostname)),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
		return
	}

	// Check if it's an IP address (which is valid as a hostname)
	if net.ParseIP(hostname) != nil {
		return
	}

	// Check if it's a valid domain name
	if n.domainRegex.MatchString(hostname) {
		return
	}

	// Check each label in the hostname
	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if len(label) == 0 {
			result.AddError(core.NewValidationError(
				"EMPTY_HOSTNAME_LABEL",
				"Hostname contains empty label",
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
			continue
		}

		if len(label) > 63 {
			result.AddError(core.NewValidationError(
				"HOSTNAME_LABEL_TOO_LONG",
				fmt.Sprintf("Hostname label '%s' exceeds maximum of 63 characters", label),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
			continue
		}

		if !n.hostnameRegex.MatchString(label) {
			result.AddError(core.NewValidationError(
				"INVALID_HOSTNAME_LABEL",
				fmt.Sprintf("Invalid hostname label: %s", label),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			).WithSuggestion("Hostname labels must start and end with alphanumeric characters and contain only alphanumerics and hyphens"))
		}
	}
}

// validateCIDR validates CIDR notation
func (n *NetworkValidator) validateCIDR(data interface{}, result *core.ValidationResult) {
	cidrStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewValidationError(
			"INVALID_CIDR_TYPE",
			"CIDR must be a string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	ip, ipnet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		result.AddError(core.NewValidationError(
			"INVALID_CIDR_FORMAT",
			fmt.Sprintf("Invalid CIDR format: %v", err),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		).WithSuggestion("CIDR should be in format: 192.168.1.0/24 or 2001:db8::/32"))
		return
	}

	// Check if the IP is the network address
	if !ip.Equal(ipnet.IP) {
		result.AddWarning(core.NewValidationWarning(
			"CIDR_NOT_NETWORK_ADDRESS",
			fmt.Sprintf("CIDR IP %s is not the network address (expected %s)", ip, ipnet.IP),
		))
	}
}

// validateMACAddress validates MAC address format
func (n *NetworkValidator) validateMACAddress(data interface{}, result *core.ValidationResult) {
	macStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewValidationError(
			"INVALID_MAC_TYPE",
			"MAC address must be a string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	// Remove common separators and convert to lowercase
	cleaned := strings.ToLower(macStr)
	cleaned = strings.ReplaceAll(cleaned, ":", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")

	// Check length (should be 12 hex characters)
	if len(cleaned) != 12 {
		result.AddError(core.NewValidationError(
			"INVALID_MAC_LENGTH",
			fmt.Sprintf("MAC address has invalid length: %d (expected 12 hex characters)", len(cleaned)),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
		return
	}

	// Check if all characters are valid hex
	for _, c := range cleaned {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			result.AddError(core.NewValidationError(
				"INVALID_MAC_CHARACTER",
				fmt.Sprintf("MAC address contains invalid character: %c", c),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			).WithSuggestion("MAC address should contain only hexadecimal characters (0-9, a-f)"))
			return
		}
	}
}

// autoDetectAndValidate tries to detect the network type and validate accordingly
func (n *NetworkValidator) autoDetectAndValidate(data interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	str, ok := data.(string)
	if !ok {
		// For non-string data, try port validation if it's numeric
		switch data.(type) {
		case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
			n.validatePort(data, result)
		default:
			result.AddError(core.NewValidationError(
				"UNSUPPORTED_NETWORK_TYPE",
				fmt.Sprintf("Unsupported data type for network validation: %T", data),
				core.ErrTypeNetwork,
				core.SeverityHigh,
			))
		}
		return
	}

	str = strings.TrimSpace(str)

	// Check if it's a CIDR
	if strings.Contains(str, "/") {
		n.validateCIDR(str, result)
		return
	}

	// Check if it's a MAC address (contains colons or hyphens in specific pattern)
	if regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`).MatchString(str) {
		n.validateMACAddress(str, result)
		return
	}

	// Check if it's an IP address
	if net.ParseIP(str) != nil {
		n.validateIP(str, result, options)
		return
	}

	// Check if it's a port (pure numeric string)
	if _, err := strconv.Atoi(str); err == nil {
		n.validatePort(str, result)
		return
	}

	// Otherwise, validate as hostname
	n.validateHostname(str, result)
}
