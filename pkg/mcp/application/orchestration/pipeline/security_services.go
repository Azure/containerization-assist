package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/logging"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// Context key types for security operations
type contextKey string

const (
	contextKeySessionID contextKey = "security_session_id"
	contextKeyOperation contextKey = "security_operation"
	contextKeyStartTime contextKey = "security_start_time"
)

// SecurityValidator handles security validation for operations
type SecurityValidator interface {
	// ValidateDockerOperation validates a Docker operation for security compliance
	ValidateDockerOperation(ctx context.Context, sessionID, operation string, args map[string]interface{}) error
}

// SecurityAuditor handles security event recording and audit logging
type SecurityAuditor interface {
	// RecordSecurityEvent records a security event
	RecordSecurityEvent(sessionID, operation, eventType, severity, description string, context map[string]interface{})

	// GetSecurityMetrics returns current security metrics
	GetSecurityMetrics() SecurityMetrics
}

// SecurityPolicyEnforcer handles policy enforcement and compliance
type SecurityPolicyEnforcer interface {
	// SecureOperationWrapper wraps operations with security validation
	SecureOperationWrapper(ctx context.Context, sessionID, operation string, args map[string]interface{}, operationFunc func() error) error
}

// SecurityRateLimiter handles rate limiting functionality
type SecurityRateLimiter interface {
	// CheckRateLimit checks if operation is within rate limits
	CheckRateLimit(sessionID string) error

	// CleanupRateLimiter removes old rate limit entries
	CleanupRateLimiter()
}

// SecurityServiceCombined combines all security capabilities
type SecurityServiceCombined interface {
	SecurityValidator
	SecurityAuditor
	SecurityPolicyEnforcer
	SecurityRateLimiter
}

// securityService implements SecurityService
type securityService struct {
	sessionManager session.SessionManager
	logger         logging.Standards
	config         SecurityConfig

	auditLog   []SecurityEvent
	auditMutex *sync.RWMutex

	rateLimiter map[string]*RateLimitEntry
	rateMutex   *sync.RWMutex
}

// NewSecurityServiceCombined creates a new combined security service
func NewSecurityServiceCombined(sessionManager session.SessionManager, config SecurityConfig, logger logging.Standards) SecurityServiceCombined {
	service := &securityService{
		sessionManager: sessionManager,
		logger:         logger.WithComponent("security_service"),
		config:         config,
		auditLog:       make([]SecurityEvent, 0),
		auditMutex:     &sync.RWMutex{},
		rateLimiter:    make(map[string]*RateLimitEntry),
		rateMutex:      &sync.RWMutex{},
	}

	go service.startSecurityMaintenance()

	return service
}

// SecurityValidator implementation

func (s *securityService) ValidateDockerOperation(_ context.Context, sessionID, operation string, args map[string]interface{}) error {
	if err := s.CheckRateLimit(sessionID); err != nil {
		s.RecordSecurityEvent(sessionID, operation, "RATE_LIMIT_EXCEEDED", "HIGH",
			fmt.Sprintf("Rate limit exceeded for session: %s", sessionID), args)
		return err
	}

	// Session validation
	if err := s.validateSession(sessionID); err != nil {
		s.RecordSecurityEvent(sessionID, operation, "INVALID_SESSION", "HIGH",
			fmt.Sprintf("Invalid session access attempt: %s", sessionID), args)
		return err
	}

	// Operation-specific validation
	switch operation {
	case "pull":
		return s.validatePullOperation(sessionID, args)
	case "push":
		return s.validatePushOperation(sessionID, args)
	case "tag":
		return s.validateTagOperation(sessionID, args)
	default:
		// For unknown operations, perform basic validation
		s.RecordSecurityEvent(sessionID, operation, "OPERATION_VALIDATED", "INFO",
			fmt.Sprintf("Generic operation validated: %s", operation), args)
		return nil
	}
}

// SecurityAuditor implementation

func (s *securityService) RecordSecurityEvent(sessionID, operation, eventType, severity, description string, context map[string]interface{}) {
	s.auditMutex.Lock()
	defer s.auditMutex.Unlock()

	event := SecurityEvent{
		ID:          s.generateEventID(),
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		Operation:   operation,
		EventType:   eventType,
		Severity:    severity,
		Description: description,
		Context:     context,
	}

	s.auditLog = append(s.auditLog, event)

	// Log high-severity events immediately
	if severity == "HIGH" {
		s.logger.Warn("Security event recorded",
			"event_id", event.ID,
			"session_id", sessionID,
			"operation", operation,
			"event_type", eventType,
			"description", description)
	}
}

func (s *securityService) GetSecurityMetrics() SecurityMetrics {
	s.auditMutex.RLock()
	defer s.auditMutex.RUnlock()

	metrics := SecurityMetrics{
		TotalEvents:        len(s.auditLog),
		SecurityViolations: 0,
		BlockedOperations:  0,
		RateLimitHits:      0,
		LastSecurityEvent:  time.Time{},
	}

	for _, event := range s.auditLog {
		if event.Severity == "HIGH" {
			metrics.SecurityViolations++
		}
		if strings.Contains(event.EventType, "BLOCKED") || strings.Contains(event.EventType, "RATE_LIMIT") {
			metrics.BlockedOperations++
		}
		if event.EventType == "RATE_LIMIT_EXCEEDED" {
			metrics.RateLimitHits++
		}
		if event.Timestamp.After(metrics.LastSecurityEvent) {
			metrics.LastSecurityEvent = event.Timestamp
		}
	}

	return metrics
}

// SecurityPolicyEnforcer implementation

func (s *securityService) SecureOperationWrapper(ctx context.Context, sessionID, operation string, args map[string]interface{}, operationFunc func() error) error {
	// Pre-operation security validation
	if err := s.ValidateDockerOperation(ctx, sessionID, operation, args); err != nil {
		return errors.NewError().Message("security validation failed").Cause(err).WithLocation().Build()
	}

	secureCtx := s.createSecureContext(ctx, sessionID, operation)

	// Execute operation with monitoring
	start := time.Now()
	err := operationFunc()
	duration := time.Since(start)

	// Post-operation security logging
	eventType := "OPERATION_SUCCESS"
	severity := "INFO"
	if err != nil {
		eventType = "OPERATION_FAILED"
		severity = "WARN"
	}

	s.RecordSecurityEvent(sessionID, operation, eventType, severity,
		fmt.Sprintf("Operation %s completed in %v", operation, duration),
		map[string]interface{}{
			"duration": duration,
			"error":    err,
			"args":     args,
		})

	// Check for suspicious patterns
	s.detectSuspiciousActivity(secureCtx, sessionID, operation, duration, err)

	return err
}

// SecurityRateLimiter implementation

func (s *securityService) CheckRateLimit(sessionID string) error {
	s.rateMutex.Lock()
	defer s.rateMutex.Unlock()

	now := time.Now()
	entry, exists := s.rateLimiter[sessionID]

	if !exists {
		s.rateLimiter[sessionID] = &RateLimitEntry{
			Count:     1,
			LastReset: now,
			Blocked:   false,
		}
		return nil
	}

	// Reset counter if minute has passed
	if now.Sub(entry.LastReset) > time.Minute {
		entry.Count = 1
		entry.LastReset = now
		entry.Blocked = false
		return nil
	}

	// Check rate limit (default: 60 operations per minute)
	if entry.Count >= 60 {
		entry.Blocked = true
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Messagef("rate limit exceeded for session: %s", sessionID).
			WithLocation().
			Build()
	}

	entry.Count++
	return nil
}

func (s *securityService) CleanupRateLimiter() {
	s.rateMutex.Lock()
	defer s.rateMutex.Unlock()

	// Remove old rate limit entries
	cutoff := time.Now().Add(-1 * time.Hour)
	for sessionID, entry := range s.rateLimiter {
		if entry.LastReset.Before(cutoff) {
			delete(s.rateLimiter, sessionID)
		}
	}
}

// Private helper methods

func (s *securityService) validatePullOperation(sessionID string, args map[string]interface{}) error {
	imageRef, ok := args["image_ref"].(string)
	if !ok || imageRef == "" {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid image reference").
			WithLocation().
			Build()
	}

	// Validate image reference format
	if !s.isValidImageReference(imageRef) {
		s.RecordSecurityEvent(sessionID, "pull", "INVALID_IMAGE_FORMAT", "HIGH",
			fmt.Sprintf("Invalid image reference format: %s", imageRef), args)
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Messagef("invalid image reference format: %s", imageRef).
			WithLocation().
			Build()
	}

	// Check against allowed registries
	if !s.isRegistryAllowed(imageRef) {
		s.RecordSecurityEvent(sessionID, "pull", "UNAUTHORIZED_REGISTRY", "HIGH",
			fmt.Sprintf("Unauthorized registry access: %s", imageRef), args)
		return errors.NewError().
			Code(errors.CodePermissionDenied).
			Type(errors.ErrTypePermission).
			Messagef("registry not allowed: %s", s.extractRegistry(imageRef)).
			WithLocation().
			Build()
	}

	// Check against blocked images
	if s.isImageBlocked(imageRef) {
		s.RecordSecurityEvent(sessionID, "pull", "BLOCKED_IMAGE", "HIGH",
			fmt.Sprintf("Blocked image access attempt: %s", imageRef), args)
		return errors.NewError().
			Code(errors.CodeSecurityViolation).
			Type(errors.ErrTypeSecurity).
			Messagef("image is blocked: %s", imageRef).
			WithLocation().
			Build()
	}

	// Log successful validation
	s.RecordSecurityEvent(sessionID, "pull", "OPERATION_VALIDATED", "INFO",
		fmt.Sprintf("Pull operation validated for image: %s", imageRef), args)

	return nil
}

func (s *securityService) validatePushOperation(sessionID string, args map[string]interface{}) error {
	imageRef, ok := args["image_ref"].(string)
	if !ok || imageRef == "" {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid image reference").
			WithLocation().
			Build()
	}

	// Validate image reference
	if !s.isValidImageReference(imageRef) {
		s.RecordSecurityEvent(sessionID, "push", "INVALID_IMAGE_FORMAT", "HIGH",
			fmt.Sprintf("Invalid image reference format: %s", imageRef), args)
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Messagef("invalid image reference format: %s", imageRef).
			WithLocation().
			Build()
	}

	// Check registry permissions for push
	if !s.canPushToRegistry(imageRef) {
		s.RecordSecurityEvent(sessionID, "push", "UNAUTHORIZED_PUSH", "HIGH",
			fmt.Sprintf("Unauthorized push attempt: %s", imageRef), args)
		return errors.NewError().
			Code(errors.CodePermissionDenied).
			Type(errors.ErrTypePermission).
			Messagef("push not allowed to registry: %s", s.extractRegistry(imageRef)).
			WithLocation().
			Build()
	}

	// Check for sensitive data in image name
	if s.containsSensitiveData(imageRef) {
		s.RecordSecurityEvent(sessionID, "push", "SENSITIVE_DATA_DETECTED", "HIGH",
			fmt.Sprintf("Sensitive data detected in image name: %s", imageRef), args)
		return errors.NewError().
			Code(errors.CodeSecurityViolation).
			Type(errors.ErrTypeSecurity).
			Message("sensitive data detected in image reference").
			WithLocation().
			Build()
	}

	s.RecordSecurityEvent(sessionID, "push", "OPERATION_VALIDATED", "INFO",
		fmt.Sprintf("Push operation validated for image: %s", imageRef), args)

	return nil
}

func (s *securityService) validateTagOperation(sessionID string, args map[string]interface{}) error {
	sourceRef, sourceOk := args["source_ref"].(string)
	targetRef, targetOk := args["target_ref"].(string)

	if !sourceOk || !targetOk || sourceRef == "" || targetRef == "" {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid source or target reference").
			WithLocation().
			Build()
	}

	// Validate both references
	if !s.isValidImageReference(sourceRef) || !s.isValidImageReference(targetRef) {
		s.RecordSecurityEvent(sessionID, "tag", "INVALID_IMAGE_FORMAT", "HIGH",
			"Invalid image reference format in tag operation", args)
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("invalid image reference format").
			WithLocation().
			Build()
	}

	// Prevent tag bombing (excessive tags)
	if s.isTagBombing(sessionID, sourceRef, targetRef) {
		s.RecordSecurityEvent(sessionID, "tag", "TAG_BOMBING_DETECTED", "HIGH",
			"Potential tag bombing detected", args)
		return errors.NewError().
			Code(errors.CodeSecurityViolation).
			Type(errors.ErrTypeSecurity).
			Message("excessive tagging detected").
			WithLocation().
			Build()
	}

	s.RecordSecurityEvent(sessionID, "tag", "OPERATION_VALIDATED", "INFO",
		fmt.Sprintf("Tag operation validated: %s -> %s", sourceRef, targetRef), args)

	return nil
}

func (s *securityService) validateSession(sessionID string) error {
	// Get session state for expiration check
	sessionState, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSession).
			Messagef("failed to get session state: %w", err).
			WithLocation().
			Build()
	}

	// Check session expiration
	if time.Now().After(sessionState.ExpiresAt) {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSession).
			Messagef("session expired: %s", sessionID).
			WithLocation().
			Build()
	}

	// Check maximum session duration
	if s.config.MaxSessionDuration > 0 && time.Since(sessionState.CreatedAt) > s.config.MaxSessionDuration {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeSession).
			Message("session duration exceeded maximum allowed time").
			WithLocation().
			Build()
	}

	return nil
}

func (s *securityService) isValidImageReference(imageRef string) bool {
	// Enhanced validation with security considerations
	if len(imageRef) > 255 {
		return false
	}

	// Basic format validation
	imageRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*[a-zA-Z0-9]$`)
	if !imageRegex.MatchString(imageRef) {
		return false
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"../", "\\", "<script", "javascript:", "data:",
		"cmd.exe", "/bin/sh", "powershell",
	}

	lowercaseRef := strings.ToLower(imageRef)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowercaseRef, pattern) {
			return false
		}
	}

	return true
}

func (s *securityService) isRegistryAllowed(imageRef string) bool {
	if len(s.config.AllowedRegistries) == 0 {
		return true // No restrictions if list is empty
	}

	registry := s.extractRegistry(imageRef)
	for _, allowed := range s.config.AllowedRegistries {
		if registry == allowed || strings.HasSuffix(registry, "."+allowed) {
			return true
		}
	}

	return false
}

func (s *securityService) isImageBlocked(imageRef string) bool {
	for _, blocked := range s.config.BlockedImages {
		if strings.Contains(imageRef, blocked) {
			return true
		}
	}
	return false
}

func (s *securityService) extractRegistry(imageRef string) string {
	parts := strings.Split(imageRef, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		return parts[0]
	}
	return "docker.io" // Default registry
}

func (s *securityService) canPushToRegistry(imageRef string) bool {
	registry := s.extractRegistry(imageRef)

	// Implement registry-specific push permissions
	// For now, allow push to allowed registries
	return s.isRegistryAllowed(imageRef) && !strings.Contains(registry, "public")
}

func (s *securityService) containsSensitiveData(imageRef string) bool {
	sensitivePatterns := []string{
		"password", "secret", "key", "token", "credential",
		"api_key", "private", "confidential", "internal",
	}

	lowercaseRef := strings.ToLower(imageRef)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowercaseRef, pattern) {
			return true
		}
	}

	return false
}

func (s *securityService) isTagBombing(_, _, _ string) bool {
	// Simple heuristic: check if too many tags from same source in short time
	// This would be expanded in production
	return false
}

func (s *securityService) createSecureContext(ctx context.Context, sessionID, operation string) context.Context {
	// Add security metadata to context
	secureCtx := context.WithValue(ctx, contextKeySessionID, sessionID)
	secureCtx = context.WithValue(secureCtx, contextKeyOperation, operation)
	secureCtx = context.WithValue(secureCtx, contextKeyStartTime, time.Now())
	return secureCtx
}

func (s *securityService) detectSuspiciousActivity(_ context.Context, sessionID, operation string, duration time.Duration, _ error) {
	// Detect anomalous patterns
	if duration > 30*time.Minute {
		s.RecordSecurityEvent(sessionID, operation, "LONG_RUNNING_OPERATION", "WARN",
			fmt.Sprintf("Unusually long operation duration: %v", duration), nil)
	}

	// Add more detection logic here
}

func (s *securityService) generateEventID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// fallback to timestamp-based ID if crypto random fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func (s *securityService) startSecurityMaintenance() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupAuditLog()
		s.CleanupRateLimiter()
	}
}

func (s *securityService) cleanupAuditLog() {
	s.auditMutex.Lock()
	defer s.auditMutex.Unlock()

	// Keep only last 24 hours of events
	cutoff := time.Now().Add(-24 * time.Hour)
	var filteredLog []SecurityEvent

	for _, event := range s.auditLog {
		if event.Timestamp.After(cutoff) {
			filteredLog = append(filteredLog, event)
		}
	}

	s.auditLog = filteredLog
}
