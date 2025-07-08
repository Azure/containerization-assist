package validation

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/errors/codes"
)

// NetworkValidators consolidates all Network/Infrastructure validation logic
// Replaces scattered network validation across multiple files
type NetworkValidators struct{}

// NewNetworkValidators creates a new Network validator
func NewNetworkValidators() *NetworkValidators {
	return &NetworkValidators{}
}

// ValidateIPAddress validates IPv4 and IPv6 addresses
func (nv *NetworkValidators) ValidateIPAddress(ip string) error {
	if ip == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("IP address cannot be empty").
			Build()
	}

	if net.ParseIP(ip) == nil {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid IP address format: %s", ip).
			Build()
	}

	return nil
}

// ValidatePort validates network port numbers
func (nv *NetworkValidators) ValidatePort(port interface{}) error {
	var portNum int

	switch p := port.(type) {
	case int:
		portNum = p
	case string:
		var err error
		portNum, err = strconv.Atoi(p)
		if err != nil {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid port format: %s", p).
				Build()
		}
	case float64:
		portNum = int(p)
	default:
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("port must be a number, got: %T", port).
			Build()
	}

	if portNum < 1 || portNum > 65535 {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("port number out of range (1-65535): %d", portNum).
			Build()
	}

	return nil
}

// ValidateURL validates URL format and structure
func (nv *NetworkValidators) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("URL cannot be empty").
			Build()
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid URL format: %s", urlStr).
			Build()
	}

	// Check for valid scheme
	validSchemes := map[string]bool{
		"http": true, "https": true, "ftp": true, "ftps": true,
	}

	if parsedURL.Scheme == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("URL missing scheme: %s", urlStr).
			Build()
	}

	if !validSchemes[strings.ToLower(parsedURL.Scheme)] {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("unsupported URL scheme: %s", parsedURL.Scheme).
			Build()
	}

	return nil
}

// ValidateHostname validates hostname format (RFC 1123)
func (nv *NetworkValidators) ValidateHostname(hostname string) error {
	if hostname == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("hostname cannot be empty").
			Build()
	}

	if len(hostname) > 253 {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("hostname too long (max 253 chars): %s", hostname).
			Build()
	}

	// RFC 1123 hostname validation
	validHostname := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	if !validHostname.MatchString(hostname) {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid hostname format: %s", hostname).
			Build()
	}

	return nil
}

// ValidateCIDR validates CIDR notation for network ranges
func (nv *NetworkValidators) ValidateCIDR(cidr string) error {
	if cidr == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("CIDR cannot be empty").
			Build()
	}

	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid CIDR format: %s", cidr).
			Build()
	}

	return nil
}

// ValidateLoadBalancerConfig validates load balancer configurations
func (nv *NetworkValidators) ValidateLoadBalancerConfig(config map[string]interface{}) error {
	// Validate required fields
	requiredFields := []string{"type", "ports"}
	for _, field := range requiredFields {
		if _, exists := config[field]; !exists {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("load balancer config missing required field: %s", field).
				Build()
		}
	}

	// Validate load balancer type
	if lbType, exists := config["type"]; exists {
		validTypes := []string{"internal", "external", "application", "network"}
		typeStr := strings.ToLower(lbType.(string))
		valid := false
		for _, validType := range validTypes {
			if typeStr == validType {
				valid = true
				break
			}
		}
		if !valid {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid load balancer type: %s", typeStr).
				Build()
		}
	}

	// Validate ports configuration
	if ports, exists := config["ports"]; exists {
		if portList, ok := ports.([]interface{}); ok {
			for i, port := range portList {
				if err := nv.ValidatePort(port); err != nil {
					return errors.NewError().
						Code(codes.VALIDATION_FAILED).
						Type(errors.ErrTypeValidation).
						Messagef("invalid port in load balancer config at index %d: %v", i, err).
						Build()
				}
			}
		}
	}

	return nil
}

// ValidateNetworkSecurity validates network security configurations
func (nv *NetworkValidators) ValidateNetworkSecurity(config map[string]interface{}) error {
	// Check for SSL/TLS configuration
	if tls, exists := config["tls"]; exists {
		if tlsMap, ok := tls.(map[string]interface{}); ok {
			if enabled, exists := tlsMap["enabled"]; exists {
				if tlsEnabled, ok := enabled.(bool); ok && tlsEnabled {
					// Validate certificate requirements
					requiredTLSFields := []string{"certificate", "privateKey"}
					for _, field := range requiredTLSFields {
						if _, exists := tlsMap[field]; !exists {
							return errors.NewError().
								Code(errors.CodeSecurity).
								Type(errors.ErrTypeSecurity).
								Messagef("TLS enabled but missing required field: %s", field).
								Build()
						}
					}
				}
			}
		}
	}

	// Validate firewall rules
	if firewall, exists := config["firewall"]; exists {
		if fwMap, ok := firewall.(map[string]interface{}); ok {
			if rules, exists := fwMap["rules"]; exists {
				if ruleList, ok := rules.([]interface{}); ok {
					for i, rule := range ruleList {
						if err := nv.validateFirewallRule(rule, i); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// validateFirewallRule validates individual firewall rules
func (nv *NetworkValidators) validateFirewallRule(rule interface{}, index int) error {
	ruleMap, ok := rule.(map[string]interface{})
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("firewall rule %d must be an object", index).
			Build()
	}

	// Validate required fields
	requiredFields := []string{"action", "protocol"}
	for _, field := range requiredFields {
		if _, exists := ruleMap[field]; !exists {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("firewall rule %d missing required field: %s", index, field).
				Build()
		}
	}

	// Validate action
	if action, exists := ruleMap["action"]; exists {
		validActions := []string{"allow", "deny", "drop"}
		actionStr := strings.ToLower(action.(string))
		valid := false
		for _, validAction := range validActions {
			if actionStr == validAction {
				valid = true
				break
			}
		}
		if !valid {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid firewall action in rule %d: %s", index, actionStr).
				Build()
		}
	}

	// Validate protocol
	if protocol, exists := ruleMap["protocol"]; exists {
		validProtocols := []string{"tcp", "udp", "icmp", "all"}
		protocolStr := strings.ToLower(protocol.(string))
		valid := false
		for _, validProtocol := range validProtocols {
			if protocolStr == validProtocol {
				valid = true
				break
			}
		}
		if !valid {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid protocol in firewall rule %d: %s", index, protocolStr).
				Build()
		}
	}

	return nil
}
