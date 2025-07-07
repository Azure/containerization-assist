package core

import (
	"context"
	"time"
)

// BuildParams provides parameters for build operations without importing internal packages
type BuildParams struct {
	SessionID  string
	Image      string
	Dockerfile string
	Context    string
	BuildArgs  map[string]string
	Tags       []string
	NoCache    bool
	Platform   string
}

// BuildResult provides results for build operations
type BuildResult struct {
	ImageID   string
	Tags      []string
	BuildTime time.Duration
	Size      int64
	Digest    string
}

// PullParams provides parameters for pull operations
type PullParams struct {
	SessionID string
	Image     string
	Platform  string
	Auth      AuthConfig
}

// PullResult provides results for pull operations
type PullResult struct {
	ImageID string
	Digest  string
	Size    int64
	Layers  int
}

// PushParams provides parameters for push operations
type PushParams struct {
	SessionID string
	Image     string
	Registry  string
	Auth      AuthConfig
}

// PushResult provides results for push operations
type PushResult struct {
	Digest   string
	Size     int64
	PushTime time.Duration
}

// TagParams provides parameters for tag operations
type TagParams struct {
	SessionID   string
	SourceImage string
	TargetImage string
}

// TagResult provides results for tag operations
type TagResult struct {
	Success bool
	Message string
}

// DeployParams provides parameters for deployment operations
type DeployParams struct {
	SessionID string
	Manifests []string
	Namespace string
	DryRun    bool
	Force     bool
	Wait      bool
	Timeout   time.Duration
}

// DeployResult provides results for deployment operations
type DeployResult struct {
	Success   bool
	Message   string
	Resources []DeployedResource
}

// ScanParams provides parameters for security scan operations
type ScanParams struct {
	SessionID string
	Image     string
	ScanType  string
	Severity  string
}

// ScanResult provides results for security scan operations
type ScanResult struct {
	Success         bool
	Vulnerabilities []Vulnerability
	Summary         ScanSummary
}

// DeployedResource represents a deployed Kubernetes resource
type DeployedResource struct {
	Kind      string
	Name      string
	Namespace string
	Status    string
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string
	Severity    string
	Package     string
	Version     string
	FixVersion  string
	Description string
}

// ScanSummary provides a summary of scan results
type ScanSummary struct {
	Total    int
	Critical int
	High     int
	Medium   int
	Low      int
}

// AuthConfig provides authentication configuration
type AuthConfig struct {
	Username string
	Password string
	Token    string
}

// Tool execution interfaces that avoid internal package imports

// BuildTool defines the interface for build tools
type BuildTool interface {
	Build(ctx context.Context, params BuildParams) (BuildResult, error)
}

// PullTool defines the interface for pull tools
type PullTool interface {
	Pull(ctx context.Context, params PullParams) (PullResult, error)
}

// PushTool defines the interface for push tools
type PushTool interface {
	Push(ctx context.Context, params PushParams) (PushResult, error)
}

// TagTool defines the interface for tag tools
type TagTool interface {
	Tag(ctx context.Context, params TagParams) (TagResult, error)
}

// DeployTool defines the interface for deployment tools
type DeployTool interface {
	Deploy(ctx context.Context, params DeployParams) (DeployResult, error)
}

// ScanTool defines the interface for security scan tools
type ScanTool interface {
	Scan(ctx context.Context, params ScanParams) (ScanResult, error)
}
