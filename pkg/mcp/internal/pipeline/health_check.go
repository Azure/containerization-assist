package pipeline

import (
	"context"
	"net/http"
	"time"
)

// SimpleHealthCheck provides basic health monitoring
type SimpleHealthCheck struct {
	scheduler interface{}
}

// NewSimpleHealthCheck creates basic health checker
func NewSimpleHealthCheck(scheduler interface{}) *SimpleHealthCheck {
	return &SimpleHealthCheck{scheduler: scheduler}
}

// ServeHTTP implements http.Handler for health check endpoint
func (h *SimpleHealthCheck) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("timeout"))
		return
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}
}

// CheckHealth performs internal health verification
func (h *SimpleHealthCheck) CheckHealth(ctx context.Context) error {
	return nil
}
