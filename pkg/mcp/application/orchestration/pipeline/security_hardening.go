package pipeline

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// SecurityService provides security hardening for Docker operations
type SecurityService interface {
	ValidateDockerOperation(ctx context.Context, sessionID, operation string, args map[string]interface{}) error
	SecureOperationWrapper(ctx context.Context, sessionID, operation string, args map[string]interface{}, operationFunc func() error) error
	GetSecurityMetrics() SecurityMetrics
}

// SecurityServiceImpl implements SecurityService
type SecurityServiceImpl struct {
	sessionManager session.SessionManager
	logger         *slog.Logger

	allowedRegistries     []string
	blockedImages         []string
	maxImageSize          int64
	maxSessionDuration    time.Duration
	requireAuthentication bool

	auditLog   []SecurityEvent
	auditMutex *sync.RWMutex

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

// Type alias for backward compatibility
type SecurityManager = SecurityServiceImpl

// NewSecurityService creates a new security service
func NewSecurityService(sessionManager session.SessionManager, config SecurityConfig) SecurityService {
	s := &SecurityServiceImpl{
		sessionManager:        sessionManager,
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

	go s.startSecurityMaintenance()

	return s
}

// NewSecurityManager creates a new security manager (backward compatibility)
func NewSecurityManager(sessionManager session.SessionManager, config SecurityConfig) *SecurityManager {
	sm := &SecurityManager{
		sessionManager:        sessionManager,
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

	go sm.startSecurityMaintenance()

	return sm
}

// SecurityMetrics represents security-related metrics
type SecurityMetrics struct {
	TotalEvents        int       `json:"total_events"`
	SecurityViolations int       `json:"security_violations"`
	BlockedOperations  int       `json:"blocked_operations"`
	RateLimitHits      int       `json:"rate_limit_hits"`
	LastSecurityEvent  time.Time `json:"last_security_event"`
}

// Backward compatibility methods for SecurityManager
// All methods now delegate to the new SecurityService

// ValidateDockerOperation validates a Docker operation for security compliance
func (s *SecurityServiceImpl) ValidateDockerOperation(_ context.Context, sessionID, operation string, args map[string]interface{}) error {
	// Basic validation logic
	if s.requireAuthentication && sessionID == "" {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Message("session ID required for authentication").
			WithLocation().
			Build()
	}

	// Check if operation is allowed
	if operation == "pull" || operation == "push" {
		if imageRef, ok := args["image"].(string); ok {
			// Check blocked images
			for _, blocked := range s.blockedImages {
				if strings.Contains(imageRef, blocked) {
					return errors.NewError().
						Code(errors.CodeSecurityViolation).
						Type(errors.ErrTypeSecurity).
						Messagef("image %s is blocked", imageRef).
						WithLocation().
						Build()
				}
			}
		}
	}

	return nil
}

// SecureOperationWrapper wraps operations with security validation
func (s *SecurityServiceImpl) SecureOperationWrapper(ctx context.Context, sessionID, operation string, args map[string]interface{}, operationFunc func() error) error {
	// First validate the operation
	if err := s.ValidateDockerOperation(ctx, sessionID, operation, args); err != nil {
		return err
	}

	// Then execute the operation
	return operationFunc()
}

// GetSecurityMetrics returns current security metrics
func (s *SecurityServiceImpl) GetSecurityMetrics() SecurityMetrics {
	s.auditMutex.RLock()
	defer s.auditMutex.RUnlock()

	return SecurityMetrics{
		TotalEvents:        len(s.auditLog),
		SecurityViolations: 0, // TODO: implement counter
		BlockedOperations:  0, // TODO: implement counter
		RateLimitHits:      0, // TODO: implement counter
		LastSecurityEvent:  time.Now(),
	}
}

// Private helper method for maintenance routine
func (s *SecurityServiceImpl) startSecurityMaintenance() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		// Cleanup rate limiter entries
		s.rateMutex.Lock()
		now := time.Now()
		for key, entry := range s.rateLimiter {
			if now.Sub(entry.LastReset) > time.Hour {
				delete(s.rateLimiter, key)
			}
		}
		s.rateMutex.Unlock()
	}
}
