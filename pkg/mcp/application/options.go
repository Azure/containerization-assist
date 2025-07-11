package application

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
)

// Option represents a functional option for configuring the server
type Option func(*workflow.ServerConfig)

// WithWorkspace sets the workspace directory
func WithWorkspace(dir string) Option {
	return func(c *workflow.ServerConfig) {
		c.WorkspaceDir = dir
	}
}

// WithStorePath sets the store path for session persistence
func WithStorePath(path string) Option {
	return func(c *workflow.ServerConfig) {
		c.StorePath = path
	}
}

// WithMaxSessions sets the maximum number of concurrent sessions
func WithMaxSessions(max int) Option {
	return func(c *workflow.ServerConfig) {
		c.MaxSessions = max
	}
}

// WithSessionTTL sets the session time-to-live
func WithSessionTTL(ttl time.Duration) Option {
	return func(c *workflow.ServerConfig) {
		c.SessionTTL = ttl
	}
}

// WithMaxDiskPerSession sets the maximum disk usage per session
func WithMaxDiskPerSession(size int64) Option {
	return func(c *workflow.ServerConfig) {
		c.MaxDiskPerSession = size
	}
}

// WithTotalDiskLimit sets the total disk limit for all sessions
func WithTotalDiskLimit(size int64) Option {
	return func(c *workflow.ServerConfig) {
		c.TotalDiskLimit = size
	}
}

// WithCleanupInterval sets the cleanup interval for expired sessions
func WithCleanupInterval(interval time.Duration) Option {
	return func(c *workflow.ServerConfig) {
		c.CleanupInterval = interval
	}
}

// WithTransport sets the transport type (stdio, http)
func WithTransport(transport string) Option {
	return func(c *workflow.ServerConfig) {
		c.TransportType = transport
	}
}

// WithHTTPAddress sets the HTTP server address
func WithHTTPAddress(addr string) Option {
	return func(c *workflow.ServerConfig) {
		c.HTTPAddr = addr
	}
}

// WithHTTPPort sets the HTTP server port
func WithHTTPPort(port int) Option {
	return func(c *workflow.ServerConfig) {
		c.HTTPPort = port
	}
}

// WithCORSOrigins sets the allowed CORS origins
func WithCORSOrigins(origins []string) Option {
	return func(c *workflow.ServerConfig) {
		c.CORSOrigins = origins
	}
}

// WithLogLevel sets the logging level
func WithLogLevel(level string) Option {
	return func(c *workflow.ServerConfig) {
		c.LogLevel = level
	}
}

// WithLogHTTPBodies enables/disables HTTP body logging
func WithLogHTTPBodies(enabled bool) Option {
	return func(c *workflow.ServerConfig) {
		c.LogHTTPBodies = enabled
	}
}

// WithMaxBodyLogSize sets the maximum body size for logging
func WithMaxBodyLogSize(size int64) Option {
	return func(c *workflow.ServerConfig) {
		c.MaxBodyLogSize = size
	}
}

// WithSandbox enables/disables sandbox mode
func WithSandbox(enabled bool) Option {
	return func(c *workflow.ServerConfig) {
		c.SandboxEnabled = enabled
	}
}

// WithServiceName sets the service name for telemetry
func WithServiceName(name string) Option {
	return func(c *workflow.ServerConfig) {
		c.ServiceName = name
	}
}

// WithServiceVersion sets the service version for telemetry
func WithServiceVersion(version string) Option {
	return func(c *workflow.ServerConfig) {
		c.ServiceVersion = version
	}
}

// WithEnvironment sets the deployment environment
func WithEnvironment(env string) Option {
	return func(c *workflow.ServerConfig) {
		c.Environment = env
	}
}

// WithTraceSampleRate sets the trace sampling rate
func WithTraceSampleRate(rate float64) Option {
	return func(c *workflow.ServerConfig) {
		c.TraceSampleRate = rate
	}
}
