package registrar

import (
	"errors"
	"testing"
	"time"
)

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		patterns []string
		want     bool
	}{
		{
			name:     "nil error",
			err:      nil,
			patterns: []string{"timeout", "connection reset"},
			want:     false,
		},
		{
			name:     "exact match",
			err:      errors.New("connection reset"),
			patterns: []string{"connection reset", "timeout"},
			want:     true,
		},
		{
			name:     "partial match",
			err:      errors.New("error: connection reset by peer"),
			patterns: []string{"connection reset", "timeout"},
			want:     true,
		},
		{
			name:     "case insensitive match",
			err:      errors.New("Connection Reset"),
			patterns: []string{"connection reset"},
			want:     true,
		},
		{
			name:     "no match",
			err:      errors.New("authentication failed"),
			patterns: []string{"connection reset", "timeout"},
			want:     false,
		},
		{
			name:     "empty patterns",
			err:      errors.New("some error"),
			patterns: []string{},
			want:     false,
		},
		{
			name:     "multiple matches",
			err:      errors.New("timeout: connection reset"),
			patterns: []string{"timeout", "connection reset"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetry(tt.err, tt.patterns); got != tt.want {
				t.Errorf("shouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExponentialBackoff_GetDelay(t *testing.T) {
	tests := []struct {
		name    string
		backoff *ExponentialBackoff
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "first attempt",
			backoff: &ExponentialBackoff{
				BaseDelay: 1 * time.Second,
				MaxDelay:  30 * time.Second,
			},
			attempt: 1,
			wantMin: 1 * time.Second,
			wantMax: 1250 * time.Millisecond, // base + 25% jitter
		},
		{
			name: "second attempt",
			backoff: &ExponentialBackoff{
				BaseDelay: 1 * time.Second,
				MaxDelay:  30 * time.Second,
			},
			attempt: 2,
			wantMin: 2 * time.Second,
			wantMax: 2500 * time.Millisecond, // 2s + 25% jitter
		},
		{
			name: "third attempt",
			backoff: &ExponentialBackoff{
				BaseDelay: 1 * time.Second,
				MaxDelay:  30 * time.Second,
			},
			attempt: 3,
			wantMin: 4 * time.Second,
			wantMax: 5 * time.Second, // 4s + 25% jitter
		},
		{
			name: "max delay cap",
			backoff: &ExponentialBackoff{
				BaseDelay: 10 * time.Second,
				MaxDelay:  30 * time.Second,
			},
			attempt: 5,
			wantMin: 30 * time.Second,
			wantMax: 37500 * time.Millisecond, // maxDelay + 25% jitter
		},
		{
			name: "zero attempt",
			backoff: &ExponentialBackoff{
				BaseDelay: 1 * time.Second,
				MaxDelay:  30 * time.Second,
			},
			attempt: 0,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "negative attempt",
			backoff: &ExponentialBackoff{
				BaseDelay: 1 * time.Second,
				MaxDelay:  30 * time.Second,
			},
			attempt: -1,
			wantMin: 0,
			wantMax: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for randomness
			for i := 0; i < 10; i++ {
				got := tt.backoff.GetDelay(tt.attempt)
				if got < tt.wantMin || got > tt.wantMax {
					t.Errorf("GetDelay() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

func TestGetBackoffStrategy(t *testing.T) {
	tests := []struct {
		name         string
		toolName     string
		wantBaseMin  time.Duration
		wantBaseMax  time.Duration
		wantMaxDelay time.Duration
	}{
		{
			name:         "configured tool - analyze_repository",
			toolName:     "analyze_repository",
			wantBaseMin:  1 * time.Second,
			wantBaseMax:  1 * time.Second,
			wantMaxDelay: 10 * time.Second,
		},
		{
			name:         "configured tool - push_image",
			toolName:     "push_image",
			wantBaseMin:  3 * time.Second,
			wantBaseMax:  3 * time.Second,
			wantMaxDelay: 60 * time.Second,
		},
		{
			name:         "unconfigured tool",
			toolName:     "unknown_tool",
			wantBaseMin:  2 * time.Second,
			wantBaseMax:  2 * time.Second,
			wantMaxDelay: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := GetBackoffStrategy(tt.toolName)
			if strategy == nil {
				t.Fatal("GetBackoffStrategy() returned nil")
			}

			// Check it's an ExponentialBackoff
			expBackoff, ok := strategy.(*ExponentialBackoff)
			if !ok {
				t.Fatal("GetBackoffStrategy() did not return ExponentialBackoff")
			}

			// Check base delay
			if expBackoff.BaseDelay < tt.wantBaseMin || expBackoff.BaseDelay > tt.wantBaseMax {
				t.Errorf("BaseDelay = %v, want between %v and %v", expBackoff.BaseDelay, tt.wantBaseMin, tt.wantBaseMax)
			}

			// Check max delay
			if expBackoff.MaxDelay != tt.wantMaxDelay {
				t.Errorf("MaxDelay = %v, want %v", expBackoff.MaxDelay, tt.wantMaxDelay)
			}
		})
	}
}

func TestDefaultRetryConfigs(t *testing.T) {
	expectedTools := []string{
		"analyze_repository",
		"generate_dockerfile",
		"build_image",
		"scan_image",
		"tag_image",
		"push_image",
		"generate_k8s_manifests",
		"prepare_cluster",
		"deploy_application",
		"verify_deployment",
	}

	// Check all expected tools have configs
	for _, tool := range expectedTools {
		t.Run(tool, func(t *testing.T) {
			config, exists := DefaultRetryConfigs[tool]
			if !exists {
				t.Errorf("Missing retry config for tool: %s", tool)
				return
			}

			// Validate config
			if config.MaxRetries <= 0 {
				t.Errorf("Invalid MaxRetries for %s: %d", tool, config.MaxRetries)
			}

			if len(config.RetryableErrors) == 0 {
				t.Errorf("No retryable errors defined for %s", tool)
			}

			if config.BackoffBase <= 0 {
				t.Errorf("Invalid BackoffBase for %s: %v", tool, config.BackoffBase)
			}

			if config.BackoffMax <= config.BackoffBase {
				t.Errorf("BackoffMax should be greater than BackoffBase for %s", tool)
			}
		})
	}
}
