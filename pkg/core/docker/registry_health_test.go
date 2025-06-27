package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryHealthChecker_normalizeRegistryURL(t *testing.T) {
	logger := zerolog.Nop()
	rhc := NewRegistryHealthChecker(logger)

	tests := []struct {
		name     string
		registry string
		want     string
	}{
		{
			name:     "docker.io",
			registry: "docker.io",
			want:     "https://registry-1.docker.io",
		},
		{
			name:     "index.docker.io",
			registry: "index.docker.io",
			want:     "https://registry-1.docker.io",
		},
		{
			name:     "plain registry",
			registry: "myregistry.com",
			want:     "https://myregistry.com",
		},
		{
			name:     "registry with https",
			registry: "https://myregistry.com",
			want:     "https://myregistry.com",
		},
		{
			name:     "registry with http",
			registry: "http://localhost:5000",
			want:     "http://localhost:5000",
		},
		{
			name:     "registry with trailing slash",
			registry: "https://myregistry.com/",
			want:     "https://myregistry.com",
		},
		{
			name:     "registry with port",
			registry: "myregistry.com:5000",
			want:     "https://myregistry.com:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rhc.normalizeRegistryURL(tt.registry)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRegistryHealthChecker_CheckRegistryHealth(t *testing.T) {
	// Create a test server that simulates a healthy registry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/v2/":
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		case "/v2/_catalog":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"repositories": []string{"test/image"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := zerolog.Nop()
	rhc := NewRegistryHealthChecker(logger)

	ctx := context.Background()
	health, err := rhc.CheckRegistryHealth(ctx, server.URL)

	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.True(t, health.Healthy)
	assert.Equal(t, server.URL, health.Registry)
	assert.Equal(t, "registry/2.0", health.APIVersion)
	assert.True(t, health.Endpoints.Base.Reachable)
	assert.True(t, health.Endpoints.V2API.Reachable)
	assert.Equal(t, http.StatusOK, health.Endpoints.V2API.StatusCode)
	assert.Contains(t, health.Capabilities, "v2-api")
	assert.Contains(t, health.Capabilities, "catalog")
}

func TestRegistryHealthChecker_CheckRegistryHealth_Unhealthy(t *testing.T) {
	// Create a test server that simulates an unhealthy registry
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	rhc := NewRegistryHealthChecker(logger)

	ctx := context.Background()
	health, err := rhc.CheckRegistryHealth(ctx, server.URL)

	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.False(t, health.Healthy)
	assert.True(t, health.Endpoints.Base.Reachable)
	assert.Equal(t, http.StatusInternalServerError, health.Endpoints.Base.StatusCode)
}

func TestRegistryHealthChecker_CheckRegistryHealth_Auth(t *testing.T) {
	// Create a test server that requires authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/v2/":
			w.Header().Set("WWW-Authenticate", `Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`)
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := zerolog.Nop()
	rhc := NewRegistryHealthChecker(logger)

	ctx := context.Background()
	health, err := rhc.CheckRegistryHealth(ctx, server.URL)

	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.True(t, health.Healthy) // Should be healthy even with 401 (auth required)
	assert.Equal(t, http.StatusUnauthorized, health.Endpoints.V2API.StatusCode)
	assert.Contains(t, health.Capabilities, "bearer-auth")
	assert.True(t, health.Endpoints.Auth.Reachable)
	assert.Equal(t, "https://auth.docker.io/token", health.Endpoints.Auth.URL)
}

func TestRegistryHealthChecker_CheckMultipleRegistries(t *testing.T) {
	// Create test servers
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer unhealthyServer.Close()

	logger := zerolog.Nop()
	rhc := NewRegistryHealthChecker(logger)

	ctx := context.Background()
	registries := []string{healthyServer.URL, unhealthyServer.URL}
	results := rhc.CheckMultipleRegistries(ctx, registries)

	assert.Len(t, results, 2)
	assert.True(t, results[healthyServer.URL].Healthy)
	assert.False(t, results[unhealthyServer.URL].Healthy)
}

func TestRegistryHealthChecker_QuickHealthCheck(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v2/") {
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	logger := zerolog.Nop()
	rhc := NewRegistryHealthChecker(logger)

	// Test with our test server URL directly
	ctx := context.Background()
	health, err := rhc.CheckRegistryHealth(ctx, server.URL)

	require.NoError(t, err)
	assert.NotNil(t, health)
	assert.True(t, health.Healthy)

	// Test the quick check method
	// Note: This will actually try to contact real registries in tests
	// For unit tests, we should mock this differently or skip this test
	t.Skip("QuickHealthCheck contacts real registries, skipping in unit tests")
}

func TestRegistryHealthChecker_Cache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	rhc := NewRegistryHealthChecker(logger)

	ctx := context.Background()

	// First check - should hit the server
	health1, err := rhc.CheckRegistryHealth(ctx, server.URL)
	require.NoError(t, err)
	assert.True(t, health1.Healthy)

	// Second check - should use cache
	health2, err := rhc.CheckRegistryHealth(ctx, server.URL)
	require.NoError(t, err)
	assert.True(t, health2.Healthy)
	assert.Equal(t, health1.CheckTime, health2.CheckTime) // Same timestamp = cached

	// Get health summary
	summary := rhc.GetHealthSummary()
	assert.Len(t, summary, 1)
	assert.Equal(t, health1, summary[server.URL])
}

func TestExtractRealm(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		want       string
	}{
		{
			name:       "bearer with realm",
			authHeader: `Bearer realm="https://auth.docker.io/token",service="registry.docker.io"`,
			want:       "https://auth.docker.io/token",
		},
		{
			name:       "bearer without realm",
			authHeader: `Bearer service="registry.docker.io"`,
			want:       "",
		},
		{
			name:       "basic auth",
			authHeader: `Basic`,
			want:       "",
		},
		{
			name:       "empty header",
			authHeader: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRealm(tt.authHeader)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHealthCache(t *testing.T) {
	cache := newHealthCache(100 * time.Millisecond)

	health := &RegistryHealth{
		Registry: "test.registry",
		Healthy:  true,
	}

	// Set and get
	cache.set("test.registry", health)
	got := cache.get("test.registry")
	assert.Equal(t, health, got)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)
	got = cache.get("test.registry")
	assert.Nil(t, got)

	// Test metrics
	metrics := &HealthMetrics{
		TotalChecks: 10,
		SuccessRate: 90.0,
	}
	cache.setMetrics("test.registry", metrics)
	gotMetrics := cache.getMetrics("test.registry")
	assert.Equal(t, metrics, gotMetrics)
}
