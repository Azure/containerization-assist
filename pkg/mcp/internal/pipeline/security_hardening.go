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

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// SecurityManager provides security hardening for Docker operations
type SecurityManager struct {
	sessionManager *session.SessionManager
	logger         zerolog.Logger
	
	// Security policies
	allowedRegistries     []string
	blockedImages         []string
	maxImageSize          int64
	maxSessionDuration    time.Duration
	requireAuthentication bool
	
	// Security audit trail
	auditLog    []SecurityEvent
	auditMutex  *sync.RWMutex
	
	// Rate limiting
	rateLimiter map[string]*RateLimitEntry
	rateMutex   *sync.RWMutex
}

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	SessionID   string                 `json:"session_id"`
	Operation   string                 `json:"operation"`
	EventType   string                 `json:"event_type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Context     map[string]interface{} `json:"context"`
	RemoteAddr  string                 `json:"remote_addr,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
}

// RateLimitEntry tracks rate limiting for sessions/IPs
type RateLimitEntry struct {
	Count     int       `json:"count"`
	LastReset time.Time `json:"last_reset"`
	Blocked   bool      `json:"blocked"`
}

// SecurityConfig configures security policies
type SecurityConfig struct {
	AllowedRegistries     []string      `json:"allowed_registries"`
	BlockedImages         []string      `json:"blocked_images"`
	MaxImageSize          int64         `json:"max_image_size"`
	MaxSessionDuration    time.Duration `json:"max_session_duration"`
	RequireAuthentication bool          `json:"require_authentication"`
	RateLimitPerMinute    int           `json:"rate_limit_per_minute"`
	EnableAuditLogging    bool          `json:"enable_audit_logging"`
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(sessionManager *session.SessionManager, config SecurityConfig, logger zerolog.Logger) *SecurityManager {
	sm := &SecurityManager{
		sessionManager:        sessionManager,
		logger:                logger.With().Str("component", "security_manager").Logger(),
		allowedRegistries:     config.AllowedRegistries,
		blockedImages:         config.BlockedImages,
		maxImageSize:          config.MaxImageSize,
		maxSessionDuration:    config.MaxSessionDuration,
		requireAuthentication: config.RequireAuthentication,
		auditLog:              make([]SecurityEvent, 0),
		auditMutex:            &sync.RWMutex{},
		rateLimiter:           make(map[string]*RateLimitEntry),
		rateMutex:             &sync.RWMutex{},
	}
	
	// Start background cleanup
	go sm.startSecurityMaintenance()
	
	return sm
}

// ValidateDockerOperation validates a Docker operation for security compliance
func (sm *SecurityManager) ValidateDockerOperation(ctx context.Context, sessionID, operation string, args map[string]interface{}) error {
	// Rate limiting check
	if err := sm.checkRateLimit(sessionID); err != nil {
		sm.recordSecurityEvent(sessionID, operation, "RATE_LIMIT_EXCEEDED", "HIGH", 
			fmt.Sprintf("Rate limit exceeded for session: %s", sessionID), args)
		return err
	}
	
	// Session validation
	if err := sm.validateSession(sessionID); err != nil {
		sm.recordSecurityEvent(sessionID, operation, "INVALID_SESSION", "HIGH",
			fmt.Sprintf("Invalid session access attempt: %s", sessionID), args)
		return err
	}
	
	// Operation-specific validation
	switch operation {
	case "pull":
		return sm.validatePullOperation(sessionID, args)
	case "push":
		return sm.validatePushOperation(sessionID, args)
	case "tag":
		return sm.validateTagOperation(sessionID, args)
	default:
		// For unknown operations, perform basic validation
		sm.recordSecurityEvent(sessionID, operation, "OPERATION_VALIDATED", "INFO",
			fmt.Sprintf("Generic operation validated: %s", operation), args)
		return nil
	}
}

// validatePullOperation validates Docker pull operations
func (sm *SecurityManager) validatePullOperation(sessionID string, args map[string]interface{}) error {
	imageRef, ok := args["image_ref"].(string)
	if !ok || imageRef == "" {
		return fmt.Errorf("invalid image reference")
	}
	
	// Validate image reference format
	if !sm.isValidImageReference(imageRef) {
		sm.recordSecurityEvent(sessionID, "pull", "INVALID_IMAGE_FORMAT", "HIGH",
			fmt.Sprintf("Invalid image reference format: %s", imageRef), args)
		return fmt.Errorf("invalid image reference format: %s", imageRef)
	}
	
	// Check against allowed registries
	if !sm.isRegistryAllowed(imageRef) {
		sm.recordSecurityEvent(sessionID, "pull", "UNAUTHORIZED_REGISTRY", "HIGH",
			fmt.Sprintf("Unauthorized registry access: %s", imageRef), args)
		return fmt.Errorf("registry not allowed: %s", sm.extractRegistry(imageRef))
	}
	
	// Check against blocked images
	if sm.isImageBlocked(imageRef) {
		sm.recordSecurityEvent(sessionID, "pull", "BLOCKED_IMAGE", "HIGH",
			fmt.Sprintf("Blocked image access attempt: %s", imageRef), args)
		return fmt.Errorf("image is blocked: %s", imageRef)
	}
	
	// Log successful validation
	sm.recordSecurityEvent(sessionID, "pull", "OPERATION_VALIDATED", "INFO",
		fmt.Sprintf("Pull operation validated for image: %s", imageRef), args)
	
	return nil
}

// validatePushOperation validates Docker push operations
func (sm *SecurityManager) validatePushOperation(sessionID string, args map[string]interface{}) error {
	imageRef, ok := args["image_ref"].(string)
	if !ok || imageRef == "" {
		return fmt.Errorf("invalid image reference")
	}
	
	// Validate image reference
	if !sm.isValidImageReference(imageRef) {
		sm.recordSecurityEvent(sessionID, "push", "INVALID_IMAGE_FORMAT", "HIGH",
			fmt.Sprintf("Invalid image reference format: %s", imageRef), args)
		return fmt.Errorf("invalid image reference format: %s", imageRef)
	}
	
	// Check registry permissions for push
	if !sm.canPushToRegistry(imageRef) {
		sm.recordSecurityEvent(sessionID, "push", "UNAUTHORIZED_PUSH", "HIGH",
			fmt.Sprintf("Unauthorized push attempt: %s", imageRef), args)
		return fmt.Errorf("push not allowed to registry: %s", sm.extractRegistry(imageRef))
	}
	
	// Check for sensitive data in image name
	if sm.containsSensitiveData(imageRef) {
		sm.recordSecurityEvent(sessionID, "push", "SENSITIVE_DATA_DETECTED", "HIGH",
			fmt.Sprintf("Sensitive data detected in image name: %s", imageRef), args)
		return fmt.Errorf("sensitive data detected in image reference")
	}
	
	sm.recordSecurityEvent(sessionID, "push", "OPERATION_VALIDATED", "INFO",
		fmt.Sprintf("Push operation validated for image: %s", imageRef), args)
	
	return nil
}

// validateTagOperation validates Docker tag operations
func (sm *SecurityManager) validateTagOperation(sessionID string, args map[string]interface{}) error {
	sourceRef, sourceOk := args["source_ref"].(string)
	targetRef, targetOk := args["target_ref"].(string)
	
	if !sourceOk || !targetOk || sourceRef == "" || targetRef == "" {
		return fmt.Errorf("invalid source or target reference")
	}
	
	// Validate both references
	if !sm.isValidImageReference(sourceRef) || !sm.isValidImageReference(targetRef) {
		sm.recordSecurityEvent(sessionID, "tag", "INVALID_IMAGE_FORMAT", "HIGH",
			"Invalid image reference format in tag operation", args)
		return fmt.Errorf("invalid image reference format")
	}
	
	// Prevent tag bombing (excessive tags)
	if sm.isTagBombing(sessionID, sourceRef, targetRef) {
		sm.recordSecurityEvent(sessionID, "tag", "TAG_BOMBING_DETECTED", "HIGH",
			"Potential tag bombing detected", args)
		return fmt.Errorf("excessive tagging detected")
	}
	
	sm.recordSecurityEvent(sessionID, "tag", "OPERATION_VALIDATED", "INFO",
		fmt.Sprintf("Tag operation validated: %s -> %s", sourceRef, targetRef), args)
	
	return nil
}

// SecureOperationWrapper wraps operations with security validation
func (sm *SecurityManager) SecureOperationWrapper(ctx context.Context, sessionID, operation string, args map[string]interface{}, operationFunc func() error) error {
	// Pre-operation security validation
	if err := sm.ValidateDockerOperation(ctx, sessionID, operation, args); err != nil {
		return fmt.Errorf("security validation failed: %w", err)
	}
	
	// Create security context
	secureCtx := sm.createSecureContext(ctx, sessionID, operation)
	
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
	
	sm.recordSecurityEvent(sessionID, operation, eventType, severity,
		fmt.Sprintf("Operation %s completed in %v", operation, duration), 
		map[string]interface{}{
			"duration": duration,
			"error":    err,
			"args":     args,
		})
	
	// Check for suspicious patterns
	sm.detectSuspiciousActivity(secureCtx, sessionID, operation, duration, err)
	
	return err
}

// GetSecurityMetrics returns current security metrics
func (sm *SecurityManager) GetSecurityMetrics() SecurityMetrics {
	sm.auditMutex.RLock()
	defer sm.auditMutex.RUnlock()
	
	metrics := SecurityMetrics{
		TotalEvents:        len(sm.auditLog),
		SecurityViolations: 0,
		BlockedOperations:  0,
		RateLimitHits:      0,
		LastSecurityEvent:  time.Time{},
	}
	
	for _, event := range sm.auditLog {
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

// Private helper methods

func (sm *SecurityManager) isValidImageReference(imageRef string) bool {
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

func (sm *SecurityManager) isRegistryAllowed(imageRef string) bool {
	if len(sm.allowedRegistries) == 0 {
		return true // No restrictions if list is empty
	}
	
	registry := sm.extractRegistry(imageRef)
	for _, allowed := range sm.allowedRegistries {
		if registry == allowed || strings.HasSuffix(registry, "."+allowed) {
			return true
		}
	}
	
	return false
}

func (sm *SecurityManager) isImageBlocked(imageRef string) bool {
	for _, blocked := range sm.blockedImages {
		if strings.Contains(imageRef, blocked) {
			return true
		}
	}
	return false
}

func (sm *SecurityManager) extractRegistry(imageRef string) string {
	parts := strings.Split(imageRef, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		return parts[0]
	}
	return "docker.io" // Default registry
}

func (sm *SecurityManager) canPushToRegistry(imageRef string) bool {
	registry := sm.extractRegistry(imageRef)
	
	// Implement registry-specific push permissions
	// For now, allow push to allowed registries
	return sm.isRegistryAllowed(imageRef) && !strings.Contains(registry, "public")
}

func (sm *SecurityManager) containsSensitiveData(imageRef string) bool {
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

func (sm *SecurityManager) checkRateLimit(sessionID string) error {
	sm.rateMutex.Lock()
	defer sm.rateMutex.Unlock()
	
	now := time.Now()
	entry, exists := sm.rateLimiter[sessionID]
	
	if !exists {
		sm.rateLimiter[sessionID] = &RateLimitEntry{
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
		return fmt.Errorf("rate limit exceeded for session: %s", sessionID)
	}
	
	entry.Count++
	return nil
}

func (sm *SecurityManager) validateSession(sessionID string) error {
	sessionData, err := sm.sessionManager.GetSessionData(sessionID)
	if err != nil {
		return fmt.Errorf("session validation failed: %w", err)
	}
	
	// Check session expiration
	if time.Now().After(sessionData.ExpiresAt) {
		return fmt.Errorf("session expired: %s", sessionID)
	}
	
	// Check maximum session duration
	if sm.maxSessionDuration > 0 && time.Since(sessionData.CreatedAt) > sm.maxSessionDuration {
		return fmt.Errorf("session duration exceeded maximum allowed time")
	}
	
	return nil
}

func (sm *SecurityManager) isTagBombing(sessionID, sourceRef, targetRef string) bool {
	// Simple heuristic: check if too many tags from same source in short time
	// This would be expanded in production
	return false
}

func (sm *SecurityManager) createSecureContext(ctx context.Context, sessionID, operation string) context.Context {
	// Add security metadata to context
	secureCtx := context.WithValue(ctx, "security_session_id", sessionID)
	secureCtx = context.WithValue(secureCtx, "security_operation", operation)
	secureCtx = context.WithValue(secureCtx, "security_start_time", time.Now())
	return secureCtx
}

func (sm *SecurityManager) detectSuspiciousActivity(ctx context.Context, sessionID, operation string, duration time.Duration, err error) {
	// Detect anomalous patterns
	if duration > 30*time.Minute {
		sm.recordSecurityEvent(sessionID, operation, "LONG_RUNNING_OPERATION", "WARN",
			fmt.Sprintf("Unusually long operation duration: %v", duration), nil)
	}
	
	// Add more detection logic here
}

func (sm *SecurityManager) recordSecurityEvent(sessionID, operation, eventType, severity, description string, context map[string]interface{}) {
	sm.auditMutex.Lock()
	defer sm.auditMutex.Unlock()
	
	event := SecurityEvent{
		ID:          sm.generateEventID(),
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		Operation:   operation,
		EventType:   eventType,
		Severity:    severity,
		Description: description,
		Context:     context,
	}
	
	sm.auditLog = append(sm.auditLog, event)
	
	// Log high-severity events immediately
	if severity == "HIGH" {
		sm.logger.Warn().
			Str("event_id", event.ID).
			Str("session_id", sessionID).
			Str("operation", operation).
			Str("event_type", eventType).
			Str("description", description).
			Msg("Security event recorded")
	}
}

func (sm *SecurityManager) generateEventID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (sm *SecurityManager) startSecurityMaintenance() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for range ticker.C {
		sm.cleanupAuditLog()
		sm.cleanupRateLimiter()
	}
}

func (sm *SecurityManager) cleanupAuditLog() {
	sm.auditMutex.Lock()
	defer sm.auditMutex.Unlock()
	
	// Keep only last 24 hours of events
	cutoff := time.Now().Add(-24 * time.Hour)
	var filteredLog []SecurityEvent
	
	for _, event := range sm.auditLog {
		if event.Timestamp.After(cutoff) {
			filteredLog = append(filteredLog, event)
		}
	}
	
	sm.auditLog = filteredLog
}

func (sm *SecurityManager) cleanupRateLimiter() {
	sm.rateMutex.Lock()
	defer sm.rateMutex.Unlock()
	
	// Remove old rate limit entries
	cutoff := time.Now().Add(-1 * time.Hour)
	for sessionID, entry := range sm.rateLimiter {
		if entry.LastReset.Before(cutoff) {
			delete(sm.rateLimiter, sessionID)
		}
	}
}

// SecurityMetrics represents security-related metrics
type SecurityMetrics struct {
	TotalEvents        int       `json:"total_events"`
	SecurityViolations int       `json:"security_violations"`
	BlockedOperations  int       `json:"blocked_operations"`
	RateLimitHits      int       `json:"rate_limit_hits"`
	LastSecurityEvent  time.Time `json:"last_security_event"`
}

