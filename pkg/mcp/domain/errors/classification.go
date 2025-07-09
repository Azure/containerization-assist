package errors

import (
	"strings"
	"time"
)

const (
	CriticalMaxRetries = 5
	HighMaxRetries     = 4
	MediumMaxRetries   = 3
	LowMaxRetries      = 2

	BaseRetryDelay       = 1 * time.Second
	NetworkRetryDelay    = 2 * time.Second
	ResourceRetryDelay   = 3 * time.Second
	TimeoutRetryDelay    = 5 * time.Second
	KubernetesRetryDelay = 10 * time.Second
	ContainerRetryDelay  = 8 * time.Second

	MaxRetryDelay = 30 * time.Second
)

// ErrorClassification provides classification metadata for errors
type ErrorClassification struct {
	Category     ErrorType     `json:"category"`
	Severity     ErrorSeverity `json:"severity"`
	Retryable    bool          `json:"retryable"`
	Recoverable  bool          `json:"recoverable"`
	UserFacing   bool          `json:"user_facing"`
	RequiresAuth bool          `json:"requires_auth"`
	Tags         []string      `json:"tags,omitempty"`
}

// ClassifyError classifies an error based on its characteristics
func ClassifyError(err error) *ErrorClassification {
	classification := &ErrorClassification{
		Category:    ErrTypeInternal,
		Severity:    SeverityMedium,
		Retryable:   false,
		Recoverable: true,
		UserFacing:  false,
	}

	if richErr, ok := err.(*RichError); ok {
		classification.Category = richErr.Type
		classification.Severity = richErr.Severity

		classification.Retryable = isRetryable(richErr)
		classification.Recoverable = isRecoverable(richErr)

		switch richErr.Type {
		case ErrTypeValidation, ErrTypePermission, ErrTypeConfiguration:
			classification.UserFacing = true
		case ErrTypeInternal, ErrTypeNetwork:
			classification.UserFacing = false
		case ErrTypeSecurity:
			classification.UserFacing = true
			if strings.Contains(string(richErr.Code), "VULNERABILITY") {
				classification.Tags = append(classification.Tags, "vulnerability")
			}
		case ErrTypeContainer, ErrTypeKubernetes:
			classification.UserFacing = true
		}

		if richErr.Type == ErrTypePermission {
			classification.RequiresAuth = true
		}

		return classification
	}

	return classification
}

// isRetryable determines if an error should be retryable based on its characteristics
func isRetryable(err *RichError) bool {
	if err.Type == ErrTypeNetwork || err.Type == ErrTypeTimeout {
		return true
	}

	if err.Type == ErrTypeResource {
		return true
	}

	switch err.Code {
	case CodeNetworkTimeout, CodeResourceExhausted, CodeKubernetesAPIError:
		return true
	case CodeValidationFailed, CodeInvalidParameter, CodeDockerfileSyntaxError:
		return false
	}

	if err.Type == ErrTypeContainer {
		return true
	}

	if err.Type == ErrTypeKubernetes {
		return true
	}

	return false
}

// isRecoverable determines if an error is recoverable
func isRecoverable(err *RichError) bool {
	if err.Type == ErrTypeSecurity && err.Severity == SeverityCritical {
		return false
	}

	if err.Type == ErrTypeInternal && err.Severity == SeverityCritical {
		return false
	}

	return true
}

// ShouldRetry determines if an error should be retried based on classification
func ShouldRetry(err error, attemptNumber int) bool {
	classification := ClassifyError(err)

	if !classification.Retryable {
		return false
	}

	maxAttempts := MediumMaxRetries
	switch classification.Severity {
	case SeverityCritical:
		maxAttempts = CriticalMaxRetries
	case SeverityHigh:
		maxAttempts = HighMaxRetries
	case SeverityMedium:
		maxAttempts = MediumMaxRetries
	case SeverityLow:
		maxAttempts = LowMaxRetries
	}

	return attemptNumber < maxAttempts
}

// GetRetryDelay calculates retry delay based on error and attempt
func GetRetryDelay(err error, attemptNumber int) time.Duration {
	baseDelay := BaseRetryDelay

	classification := ClassifyError(err)

	switch classification.Category {
	case ErrTypeNetwork:
		baseDelay = NetworkRetryDelay
	case ErrTypeResource:
		baseDelay = ResourceRetryDelay
	case ErrTypeTimeout:
		baseDelay = TimeoutRetryDelay
	case ErrTypeKubernetes:
		baseDelay = KubernetesRetryDelay
	case ErrTypeContainer:
		baseDelay = ContainerRetryDelay
	}

	delay := baseDelay * time.Duration(1<<uint(attemptNumber-1))

	if delay > MaxRetryDelay {
		delay = MaxRetryDelay
	}

	return delay
}

// IsUserFacing determines if an error should be shown to the user
func IsUserFacing(err error) bool {
	classification := ClassifyError(err)
	return classification.UserFacing
}

// IsRichErrorRetryable determines if a RichError is retryable
func IsRichErrorRetryable(err error) bool {
	classification := ClassifyError(err)
	return classification.Retryable
}

// IsRichErrorRecoverable determines if a RichError is recoverable
func IsRichErrorRecoverable(err error) bool {
	classification := ClassifyError(err)
	return classification.Recoverable
}

// RequiresAuth determines if an error requires authentication
func RequiresAuth(err error) bool {
	classification := ClassifyError(err)
	return classification.RequiresAuth
}

// GetErrorSeverity returns the severity level of an error
func GetErrorSeverity(err error) ErrorSeverity {
	classification := ClassifyError(err)
	return classification.Severity
}

// GetErrorCategory returns the category of an error
func GetErrorCategory(err error) ErrorType {
	classification := ClassifyError(err)
	return classification.Category
}

// HasTag checks if an error has a specific tag
func HasTag(err error, tag string) bool {
	classification := ClassifyError(err)
	for _, t := range classification.Tags {
		if t == tag {
			return true
		}
	}
	return false
}
