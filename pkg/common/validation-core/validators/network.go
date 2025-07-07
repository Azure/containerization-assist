package validators

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
func (n *NetworkValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
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
func (n *NetworkValidator) validateIP(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	ipStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewError(
			"INVALID_IP_TYPE",
			"IP address must be a string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		result.AddError(core.NewError(
			"INVALID_IP_FORMAT",
			fmt.Sprintf("Invalid IP address format: %s", ipStr),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
		return
	}

	// Check IP version if specified
	if options != nil && options.Context != nil {
		if version, ok := options.Context["ip_version"].(string); ok {
			switch version {
			case "4":
				if ip.To4() == nil {
					result.AddError(core.NewError(
						"INVALID_IPV4",
						fmt.Sprintf("Expected IPv4 address, got: %s", ipStr),
						core.ErrTypeNetwork,
						core.SeverityMedium,
					))
				}
			case "6":
				if ip.To4() != nil {
					result.AddError(core.NewError(
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
		result.AddWarning(core.NewWarning(
			"LOOPBACK_IP",
			fmt.Sprintf("IP address %s is a loopback address", ipStr),
		))
	}
	if ip.IsPrivate() {
		result.AddWarning(core.NewWarning(
			"PRIVATE_IP",
			fmt.Sprintf("IP address %s is a private address", ipStr),
		))
	}
}

// validatePort validates port number
func (n *NetworkValidator) validatePort(data interface{}, result *core.NonGenericResult) {
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
			result.AddError(core.NewError(
				"INVALID_PORT_FORMAT",
				fmt.Sprintf("Invalid port format: %s", v),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
			return
		}
	default:
		result.AddError(core.NewError(
			"INVALID_PORT_TYPE",
			"Port must be a number or numeric string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	if port < 1 || port > 65535 {
		result.AddError(core.NewError(
			"PORT_OUT_OF_RANGE",
			fmt.Sprintf("Port %d is out of valid range (1-65535)", port),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
	}

	// Warn about privileged ports
	if port < 1024 {
		result.AddWarning(core.NewWarning(
			"PRIVILEGED_PORT",
			fmt.Sprintf("Port %d is a privileged port (requires root/admin access)", port),
		))
	}
}

// validateHostname validates hostname format
func (n *NetworkValidator) validateHostname(data interface{}, result *core.NonGenericResult) {
	hostname, ok := data.(string)
	if !ok {
		result.AddError(core.NewError(
			"INVALID_HOSTNAME_TYPE",
			"Hostname must be a string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	if len(hostname) == 0 {
		result.AddError(core.NewError(
			"EMPTY_HOSTNAME",
			"Hostname cannot be empty",
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
		return
	}

	if len(hostname) > 253 {
		result.AddError(core.NewError(
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
			result.AddError(core.NewError(
				"EMPTY_HOSTNAME_LABEL",
				"Hostname contains empty label",
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
			continue
		}

		if len(label) > 63 {
			result.AddError(core.NewError(
				"HOSTNAME_LABEL_TOO_LONG",
				fmt.Sprintf("Hostname label '%s' exceeds maximum of 63 characters", label),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
			continue
		}

		if !n.hostnameRegex.MatchString(label) {
			result.AddError(core.NewError(
				"INVALID_HOSTNAME_LABEL",
				fmt.Sprintf("Invalid hostname label: %s", label),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
		}
	}
}

// validateCIDR validates CIDR notation
func (n *NetworkValidator) validateCIDR(data interface{}, result *core.NonGenericResult) {
	cidrStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewError(
			"INVALID_CIDR_TYPE",
			"CIDR must be a string",
			core.ErrTypeNetwork,
			core.SeverityHigh,
		))
		return
	}

	ip, ipnet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		result.AddError(core.NewError(
			"INVALID_CIDR_FORMAT",
			fmt.Sprintf("Invalid CIDR format: %v", err),
			core.ErrTypeNetwork,
			core.SeverityMedium,
		))
		return
	}

	// Check if the IP is the network address
	if !ip.Equal(ipnet.IP) {
		result.AddWarning(core.NewWarning(
			"CIDR_NOT_NETWORK_ADDRESS",
			fmt.Sprintf("CIDR IP %s is not the network address (expected %s)", ip, ipnet.IP),
		))
	}
}

// validateMACAddress validates MAC address format
func (n *NetworkValidator) validateMACAddress(data interface{}, result *core.NonGenericResult) {
	macStr, ok := data.(string)
	if !ok {
		result.AddError(core.NewError(
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
		result.AddError(core.NewError(
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
			result.AddError(core.NewError(
				"INVALID_MAC_CHARACTER",
				fmt.Sprintf("MAC address contains invalid character: %c", c),
				core.ErrTypeNetwork,
				core.SeverityMedium,
			))
			return
		}
	}
}

// autoDetectAndValidate tries to detect the network type and validate accordingly
func (n *NetworkValidator) autoDetectAndValidate(data interface{}, result *core.NonGenericResult, options *core.ValidationOptions) {
	str, ok := data.(string)
	if !ok {
		// For non-string data, try port validation if it's numeric
		switch data.(type) {
		case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
			n.validatePort(data, result)
		default:
			result.AddError(core.NewError(
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

// ============================================================================
// Type-Safe Validation Methods (applying Kubernetes validator pattern)
// ============================================================================

// NetworkData represents type-safe network validation data
type NetworkData struct {
	Type     string      `json:"type"`               // ip, port, hostname, cidr, mac
	Value    string      `json:"value"`              // The network value to validate
	Version  string      `json:"version,omitempty"`  // For IP: "4" or "6"
	Protocol string      `json:"protocol,omitempty"` // For ports: "tcp", "udp"
	Context  interface{} `json:"context,omitempty"`  // Additional context
	Raw      interface{} `json:"raw,omitempty"`      // Keep raw data for backward compatibility
}

// ValidateTyped validates network data with type safety
func (n *NetworkValidator) ValidateTyped(ctx context.Context, networkData NetworkData, options *core.ValidationOptions) *core.NonGenericResult {
	result := n.BaseValidatorImpl.Validate(ctx, networkData, options)

	// Use the typed data for validation
	n.validateNetworkData(networkData, result, options)

	return result
}

// ConvertToNetworkData converts interface{} to typed NetworkData
func ConvertToNetworkData(data interface{}) (NetworkData, error) {
	networkData := NetworkData{
		Raw: data, // Keep raw data for backward compatibility
	}

	switch v := data.(type) {
	case string:
		networkData.Value = v
		networkData.Type = detectNetworkType(v)
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		networkData.Value = fmt.Sprintf("%v", v)
		networkData.Type = "port"
	case map[string]interface{}:
		if value, ok := v["value"].(string); ok {
			networkData.Value = value
		}
		if netType, ok := v["type"].(string); ok {
			networkData.Type = netType
		} else {
			networkData.Type = detectNetworkType(networkData.Value)
		}
		if version, ok := v["version"].(string); ok {
			networkData.Version = version
		}
		if protocol, ok := v["protocol"].(string); ok {
			networkData.Protocol = protocol
		}
		if context, ok := v["context"]; ok {
			networkData.Context = context
		}
	case NetworkData:
		return v, nil
	default:
		return networkData, errors.NewError().Messagef("unsupported data type for network validation: %T", data).Build()
	}

	return networkData, nil
}

// detectNetworkType automatically detects the type of network data
func detectNetworkType(value string) string {
	value = strings.TrimSpace(value)

	// Check if it's a CIDR
	if strings.Contains(value, "/") {
		return "cidr"
	}

	// Check if it's a MAC address
	if regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`).MatchString(value) {
		return "mac"
	}

	// Check if it's an IP address
	if net.ParseIP(value) != nil {
		return "ip"
	}

	// Check if it's a port (pure numeric string)
	if _, err := strconv.Atoi(value); err == nil {
		return "port"
	}

	// Default to hostname
	return "hostname"
}

// validateNetworkData validates typed network data
func (n *NetworkValidator) validateNetworkData(data NetworkData, result *core.NonGenericResult, options *core.ValidationOptions) {
	// Set up options context with type information
	if options == nil {
		options = core.NewValidationOptions()
	}
	if options.Context == nil {
		options.Context = make(map[string]interface{})
	}

	// Add type information to context
	options.Context["network_type"] = data.Type
	if data.Version != "" {
		options.Context["ip_version"] = data.Version
	}
	if data.Protocol != "" {
		options.Context["protocol"] = data.Protocol
	}

	// Validate based on type
	switch data.Type {
	case "ip":
		n.validateIP(data.Value, result, options)
	case "port":
		n.validatePort(data.Value, result)
	case "hostname":
		n.validateHostname(data.Value, result)
	case "cidr":
		n.validateCIDR(data.Value, result)
	case "mac":
		n.validateMACAddress(data.Value, result)
	default:
		// Try auto-detection
		n.autoDetectAndValidate(data.Value, result, options)
	}

	// Add additional validations based on context
	if data.Type == "port" && data.Protocol != "" {
		n.validatePortProtocol(data.Value, data.Protocol, result)
	}
}

// validatePortProtocol validates port with specific protocol context
func (n *NetworkValidator) validatePortProtocol(portStr, protocol string, result *core.NonGenericResult) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return // Already validated in validatePort
	}

	// Check for well-known ports for specific protocols
	wellKnownPorts := map[string]map[int]string{
		"tcp": {
			22:   "SSH",
			80:   "HTTP",
			443:  "HTTPS",
			3306: "MySQL",
			5432: "PostgreSQL",
		},
		"udp": {
			53:  "DNS",
			67:  "DHCP Server",
			68:  "DHCP Client",
			123: "NTP",
		},
	}

	if services, ok := wellKnownPorts[strings.ToLower(protocol)]; ok {
		if service, exists := services[port]; exists {
			result.AddWarning(core.NewWarning(
				"WELL_KNOWN_PORT",
				fmt.Sprintf("Port %d is commonly used for %s service", port, service),
			))
		}
	}
}
