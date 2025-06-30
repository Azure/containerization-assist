# InfraBot Integration Guide

## Overview

This guide provides detailed instructions for integrating with InfraBot's core infrastructure. Whether you're building atomic tools for BuildSecBot, orchestrating workflows with OrchBot, or implementing sandbox environments with AdvancedBot, this guide will help you leverage InfraBot's capabilities effectively.

## Table of Contents

1. [Integration Patterns](#integration-patterns)
2. [Team-Specific Integration](#team-specific-integration)
3. [API Integration](#api-integration)
4. [SDK Usage](#sdk-usage)
5. [Testing Integration](#testing-integration)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

## Integration Patterns

### 1. Direct API Integration

The simplest integration pattern using REST APIs for basic operations.

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type APIClient struct {
    baseURL string
    client  *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
    return &APIClient{
        baseURL: baseURL,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *APIClient) CreateSession(team string) (*Session, error) {
    payload := map[string]interface{}{
        "team": team,
        "config": map[string]interface{}{
            "timeout": "30m",
            "max_operations": 100,
        },
    }
    
    data, _ := json.Marshal(payload)
    resp, err := c.client.Post(
        c.baseURL+"/api/v1/sessions",
        "application/json",
        bytes.NewBuffer(data),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var session Session
    err = json.NewDecoder(resp.Body).Decode(&session)
    return &session, err
}

func (c *APIClient) PullImage(sessionID, imageRef string) error {
    payload := map[string]interface{}{
        "session_id": sessionID,
        "image_ref":  imageRef,
    }
    
    data, _ := json.Marshal(payload)
    resp, err := c.client.Post(
        c.baseURL+"/api/v1/docker/pull",
        "application/json",
        bytes.NewBuffer(data),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("pull failed with status: %d", resp.StatusCode)
    }
    
    return nil
}
```

### 2. SDK Integration

Using the Go SDK for type-safe integration with InfraBot.

```go
package main

import (
    "context"
    "time"
    
    "github.com/Azure/container-kit/pkg/mcp/client"
    "github.com/Azure/container-kit/pkg/mcp/internal/session"
    "github.com/rs/zerolog/log"
)

func main() {
    // Initialize InfraBot client
    config := client.Config{
        BaseURL: "http://localhost:8080",
        Timeout: 30 * time.Second,
        RetryPolicy: client.RetryPolicy{
            MaxRetries: 3,
            BackoffPolicy: client.ExponentialBackoff,
        },
    }
    
    infrabot, err := client.NewInfraBotClient(config)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create InfraBot client")
    }
    
    ctx := context.Background()
    
    // Create session
    sessionConfig := session.SessionConfig{
        Timeout:        30 * time.Minute,
        MaxOperations:  100,
        EnableTracking: true,
        Tags:          []string{"integration", "sdk"},
    }
    
    sess, err := infrabot.CreateSession(ctx, "my-team", sessionConfig)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create session")
    }
    
    log.Info().Str("session_id", sess.ID).Msg("Session created")
    
    // Perform Docker operations
    err = infrabot.PullImage(ctx, sess.ID, "nginx:latest")
    if err != nil {
        log.Error().Err(err).Msg("Failed to pull image")
    }
    
    // Clean up session
    defer func() {
        err := infrabot.DeleteSession(ctx, sess.ID)
        if err != nil {
            log.Warn().Err(err).Msg("Failed to cleanup session")
        }
    }()
}
```

### 3. Event-Driven Integration

Using webhooks and event streams for real-time integration.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    
    "github.com/gorilla/mux"
    "github.com/rs/zerolog/log"
)

type EventHandler struct {
    infrabot InfraBotClient
}

func (h *EventHandler) HandleDockerEvent(w http.ResponseWriter, r *http.Request) {
    var event DockerEvent
    err := json.NewDecoder(r.Body).Decode(&event)
    if err != nil {
        http.Error(w, "Invalid event payload", http.StatusBadRequest)
        return
    }
    
    switch event.Type {
    case "pull_completed":
        h.handlePullCompleted(event)
    case "push_completed":
        h.handlePushCompleted(event)
    case "operation_failed":
        h.handleOperationFailed(event)
    }
    
    w.WriteHeader(http.StatusOK)
}

func (h *EventHandler) handlePullCompleted(event DockerEvent) {
    log.Info().
        Str("session_id", event.SessionID).
        Str("image_ref", event.ImageRef).
        Msg("Docker pull completed")
    
    // Trigger next step in workflow
    err := h.triggerNextWorkflowStep(event.SessionID, event.ImageRef)
    if err != nil {
        log.Error().Err(err).Msg("Failed to trigger next workflow step")
    }
}

func (h *EventHandler) triggerNextWorkflowStep(sessionID, imageRef string) error {
    // Implementation depends on your workflow system
    return nil
}

func main() {
    handler := &EventHandler{
        infrabot: NewInfraBotClient(),
    }
    
    r := mux.NewRouter()
    r.HandleFunc("/webhooks/docker", handler.HandleDockerEvent).Methods("POST")
    
    log.Info().Msg("Starting webhook server on :8081")
    http.ListenAndServe(":8081", r)
}
```

## Team-Specific Integration

### BuildSecBot Integration

BuildSecBot integrates with InfraBot's atomic tool framework for security scanning operations.

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/Azure/container-kit/pkg/mcp/internal/runtime"
    "github.com/rs/zerolog/log"
)

type SecurityScanner struct {
    atomicTool runtime.AtomicToolBase
    infrabot   InfraBotClient
}

func NewSecurityScanner(infrabot InfraBotClient) *SecurityScanner {
    config := runtime.AtomicToolConfig{
        EnableProgressTracking:   true,
        EnableResourceMonitoring: true,
        PerformanceTargets: runtime.PerformanceTargets{
            MaxLatency:    300 * time.Microsecond,
            MinThroughput: 1000.0,
        },
    }
    
    atomicTool := runtime.NewAtomicTool(config, log.Logger)
    
    return &SecurityScanner{
        atomicTool: atomicTool,
        infrabot:   infrabot,
    }
}

func (s *SecurityScanner) ScanImage(ctx context.Context, sessionID, imageRef string) (*ScanResult, error) {
    log.Info().
        Str("session_id", sessionID).
        Str("image_ref", imageRef).
        Msg("Starting security scan")
    
    var result *ScanResult
    
    // Use atomic tool framework for the scan
    err := s.atomicTool.ExecuteWithProgress(ctx, func(progress runtime.ProgressCallback) error {
        progress(0.0, "Initializing security scan")
        
        // Pull image if not available locally
        progress(0.1, "Pulling image for scanning")
        err := s.infrabot.PullImage(ctx, sessionID, imageRef)
        if err != nil {
            return fmt.Errorf("failed to pull image: %w", err)
        }
        
        // Perform vulnerability scan
        progress(0.3, "Scanning for vulnerabilities")
        vulns, err := s.scanVulnerabilities(imageRef)
        if err != nil {
            return fmt.Errorf("vulnerability scan failed: %w", err)
        }
        
        // Perform secret scan
        progress(0.6, "Scanning for secrets")
        secrets, err := s.scanSecrets(imageRef)
        if err != nil {
            return fmt.Errorf("secret scan failed: %w", err)
        }
        
        // Generate compliance report
        progress(0.8, "Generating compliance report")
        compliance, err := s.generateComplianceReport(vulns, secrets)
        if err != nil {
            return fmt.Errorf("compliance report generation failed: %w", err)
        }
        
        result = &ScanResult{
            ImageRef:         imageRef,
            Vulnerabilities:  vulns,
            Secrets:         secrets,
            ComplianceReport: compliance,
            ScanTimestamp:   time.Now(),
        }
        
        progress(1.0, "Security scan completed")
        return nil
    })
    
    if err != nil {
        // Track error in session
        s.infrabot.TrackError(ctx, sessionID, err, map[string]interface{}{
            "operation": "security_scan",
            "image_ref": imageRef,
        })
        return nil, err
    }
    
    // Track successful operation
    s.infrabot.TrackOperation(ctx, sessionID, "security_scan", map[string]interface{}{
        "image_ref":          imageRef,
        "vulnerabilities":    len(result.Vulnerabilities),
        "secrets_detected":   len(result.Secrets),
        "compliance_score":   result.ComplianceReport.Score,
    })
    
    return result, nil
}

func (s *SecurityScanner) scanVulnerabilities(imageRef string) ([]Vulnerability, error) {
    // Implementation would use tools like Trivy, Grype, etc.
    return []Vulnerability{}, nil
}

func (s *SecurityScanner) scanSecrets(imageRef string) ([]Secret, error) {
    // Implementation would use tools like TruffleHog, GitLeaks, etc.
    return []Secret{}, nil
}

func (s *SecurityScanner) generateComplianceReport(vulns []Vulnerability, secrets []Secret) (*ComplianceReport, error) {
    // Generate compliance report based on organizational policies
    return &ComplianceReport{
        Score:           85.5,
        PolicyViolations: []string{},
        Recommendations: []string{"Update base image", "Remove test secrets"},
    }, nil
}

// Usage example
func main() {
    infrabot := NewInfraBotClient()
    scanner := NewSecurityScanner(infrabot)
    
    ctx := context.Background()
    sessionID := "scan-session-123"
    
    result, err := scanner.ScanImage(ctx, sessionID, "myapp:latest")
    if err != nil {
        log.Fatal().Err(err).Msg("Security scan failed")
    }
    
    log.Info().
        Int("vulnerabilities", len(result.Vulnerabilities)).
        Int("secrets", len(result.Secrets)).
        Float64("compliance_score", result.ComplianceReport.Score).
        Msg("Security scan completed")
}
```

### OrchBot Integration

OrchBot uses InfraBot for workflow orchestration and context sharing.

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/rs/zerolog/log"
)

type WorkflowOrchestrator struct {
    infrabot InfraBotClient
    workflows map[string]*Workflow
    mutex    sync.RWMutex
}

type Workflow struct {
    ID          string
    SessionID   string
    Steps       []WorkflowStep
    CurrentStep int
    Status      WorkflowStatus
    Context     map[string]interface{}
}

type WorkflowStep struct {
    Name        string
    Type        StepType
    Config      map[string]interface{}
    DependsOn   []string
    MaxRetries  int
    Timeout     time.Duration
}

func NewWorkflowOrchestrator(infrabot InfraBotClient) *WorkflowOrchestrator {
    return &WorkflowOrchestrator{
        infrabot:  infrabot,
        workflows: make(map[string]*Workflow),
    }
}

func (o *WorkflowOrchestrator) CreateContainerizationWorkflow(appName, sourceRef, targetRef string) (*Workflow, error) {
    ctx := context.Background()
    
    // Create session for the workflow
    session, err := o.infrabot.CreateSession(ctx, "orchbot", SessionConfig{
        Timeout:        60 * time.Minute,
        MaxOperations:  50,
        EnableTracking: true,
        Tags:          []string{"workflow", "containerization", appName},
        Metadata: map[string]interface{}{
            "app_name":   appName,
            "source_ref": sourceRef,
            "target_ref": targetRef,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create workflow session: %w", err)
    }
    
    workflow := &Workflow{
        ID:        fmt.Sprintf("workflow-%s-%d", appName, time.Now().Unix()),
        SessionID: session.ID,
        Status:    WorkflowStatusCreated,
        Context: map[string]interface{}{
            "app_name":   appName,
            "source_ref": sourceRef,
            "target_ref": targetRef,
        },
        Steps: []WorkflowStep{
            {
                Name:    "analyze_source",
                Type:    StepTypeAnalysis,
                Timeout: 10 * time.Minute,
                Config: map[string]interface{}{
                    "source_ref": sourceRef,
                },
            },
            {
                Name:     "build_image",
                Type:     StepTypeBuild,
                DependsOn: []string{"analyze_source"},
                Timeout:  20 * time.Minute,
                Config: map[string]interface{}{
                    "dockerfile_path": "./Dockerfile",
                    "build_context":   "./",
                },
            },
            {
                Name:     "security_scan",
                Type:     StepTypeSecurityScan,
                DependsOn: []string{"build_image"},
                Timeout:  15 * time.Minute,
                Config: map[string]interface{}{
                    "scan_type": "comprehensive",
                },
            },
            {
                Name:     "push_image",
                Type:     StepTypePush,
                DependsOn: []string{"security_scan"},
                Timeout:  10 * time.Minute,
                Config: map[string]interface{}{
                    "target_ref": targetRef,
                },
            },
            {
                Name:     "generate_manifests",
                Type:     StepTypeManifestGeneration,
                DependsOn: []string{"push_image"},
                Timeout:  5 * time.Minute,
                Config: map[string]interface{}{
                    "platform": "kubernetes",
                },
            },
        },
    }
    
    o.mutex.Lock()
    o.workflows[workflow.ID] = workflow
    o.mutex.Unlock()
    
    log.Info().
        Str("workflow_id", workflow.ID).
        Str("session_id", workflow.SessionID).
        Int("steps", len(workflow.Steps)).
        Msg("Containerization workflow created")
    
    return workflow, nil
}

func (o *WorkflowOrchestrator) ExecuteWorkflow(ctx context.Context, workflowID string) error {
    o.mutex.RLock()
    workflow, exists := o.workflows[workflowID]
    o.mutex.RUnlock()
    
    if !exists {
        return fmt.Errorf("workflow %s not found", workflowID)
    }
    
    workflow.Status = WorkflowStatusRunning
    
    log.Info().
        Str("workflow_id", workflowID).
        Msg("Starting workflow execution")
    
    // Track workflow start in session
    err := o.infrabot.TrackOperation(ctx, workflow.SessionID, "workflow_start", map[string]interface{}{
        "workflow_id": workflowID,
        "step_count":  len(workflow.Steps),
    })
    if err != nil {
        log.Warn().Err(err).Msg("Failed to track workflow start")
    }
    
    for i, step := range workflow.Steps {
        workflow.CurrentStep = i
        
        log.Info().
            Str("workflow_id", workflowID).
            Str("step_name", step.Name).
            Int("step_index", i).
            Msg("Executing workflow step")
        
        err := o.executeStep(ctx, workflow, step)
        if err != nil {
            workflow.Status = WorkflowStatusFailed
            
            // Track error in session
            o.infrabot.TrackError(ctx, workflow.SessionID, err, map[string]interface{}{
                "workflow_id": workflowID,
                "step_name":   step.Name,
                "step_index":  i,
            })
            
            return fmt.Errorf("workflow step %s failed: %w", step.Name, err)
        }
        
        // Track step completion
        o.infrabot.TrackOperation(ctx, workflow.SessionID, "workflow_step", map[string]interface{}{
            "workflow_id": workflowID,
            "step_name":   step.Name,
            "step_index":  i,
            "status":      "completed",
        })
    }
    
    workflow.Status = WorkflowStatusCompleted
    
    // Track workflow completion
    err = o.infrabot.TrackOperation(ctx, workflow.SessionID, "workflow_complete", map[string]interface{}{
        "workflow_id":  workflowID,
        "total_steps": len(workflow.Steps),
        "status":      "success",
    })
    if err != nil {
        log.Warn().Err(err).Msg("Failed to track workflow completion")
    }
    
    log.Info().
        Str("workflow_id", workflowID).
        Msg("Workflow execution completed successfully")
    
    return nil
}

func (o *WorkflowOrchestrator) executeStep(ctx context.Context, workflow *Workflow, step WorkflowStep) error {
    // Create step context with timeout
    stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
    defer cancel()
    
    switch step.Type {
    case StepTypeAnalysis:
        return o.executeAnalysisStep(stepCtx, workflow, step)
    case StepTypeBuild:
        return o.executeBuildStep(stepCtx, workflow, step)
    case StepTypeSecurityScan:
        return o.executeSecurityScanStep(stepCtx, workflow, step)
    case StepTypePush:
        return o.executePushStep(stepCtx, workflow, step)
    case StepTypeManifestGeneration:
        return o.executeManifestGenerationStep(stepCtx, workflow, step)
    default:
        return fmt.Errorf("unknown step type: %s", step.Type)
    }
}

func (o *WorkflowOrchestrator) executePushStep(ctx context.Context, workflow *Workflow, step WorkflowStep) error {
    targetRef := step.Config["target_ref"].(string)
    
    err := o.infrabot.PushImage(ctx, workflow.SessionID, targetRef)
    if err != nil {
        return fmt.Errorf("failed to push image %s: %w", targetRef, err)
    }
    
    // Update workflow context with pushed image reference
    workflow.Context["pushed_image"] = targetRef
    
    return nil
}

// Additional step execution methods would be implemented here...
```

### AdvancedBot Integration

AdvancedBot integrates with InfraBot for sandbox environment management.

```go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    
    "github.com/rs/zerolog/log"
)

type SandboxManager struct {
    infrabot     InfraBotClient
    sandboxes    map[string]*Sandbox
    baseImageRef string
}

type Sandbox struct {
    ID           string
    SessionID    string
    WorkspaceDir string
    ContainerID  string
    Status       SandboxStatus
    Resources    SandboxResources
    Config       SandboxConfig
}

type SandboxConfig struct {
    BaseImage     string
    WorkspaceSize int64
    MemoryLimit   int64
    CPULimit      float64
    NetworkMode   string
    Volumes       []VolumeMount
}

func NewSandboxManager(infrabot InfraBotClient, baseImageRef string) *SandboxManager {
    return &SandboxManager{
        infrabot:     infrabot,
        sandboxes:    make(map[string]*Sandbox),
        baseImageRef: baseImageRef,
    }
}

func (sm *SandboxManager) CreateSandbox(ctx context.Context, config SandboxConfig) (*Sandbox, error) {
    // Create session for sandbox
    session, err := sm.infrabot.CreateSession(ctx, "advancedbot", SessionConfig{
        Timeout:        120 * time.Minute,
        MaxOperations:  200,
        EnableTracking: true,
        Tags:          []string{"sandbox", "isolation"},
        Metadata: map[string]interface{}{
            "base_image":    config.BaseImage,
            "workspace_size": config.WorkspaceSize,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create sandbox session: %w", err)
    }
    
    sandboxID := fmt.Sprintf("sandbox-%d", time.Now().Unix())
    
    // Create isolated workspace directory
    workspaceDir := filepath.Join("/tmp/sandboxes", sandboxID)
    err = os.MkdirAll(workspaceDir, 0755)
    if err != nil {
        return nil, fmt.Errorf("failed to create workspace directory: %w", err)
    }
    
    sandbox := &Sandbox{
        ID:           sandboxID,
        SessionID:    session.ID,
        WorkspaceDir: workspaceDir,
        Status:       SandboxStatusCreating,
        Config:       config,
    }
    
    // Pull base image
    log.Info().
        Str("sandbox_id", sandboxID).
        Str("base_image", config.BaseImage).
        Msg("Pulling base image for sandbox")
    
    err = sm.infrabot.PullImage(ctx, session.ID, config.BaseImage)
    if err != nil {
        sm.cleanupSandbox(sandbox)
        return nil, fmt.Errorf("failed to pull base image: %w", err)
    }
    
    // Create and start container
    containerID, err := sm.createSandboxContainer(ctx, sandbox)
    if err != nil {
        sm.cleanupSandbox(sandbox)
        return nil, fmt.Errorf("failed to create sandbox container: %w", err)
    }
    
    sandbox.ContainerID = containerID
    sandbox.Status = SandboxStatusRunning
    
    sm.sandboxes[sandboxID] = sandbox
    
    // Track sandbox creation
    err = sm.infrabot.TrackOperation(ctx, session.ID, "sandbox_create", map[string]interface{}{
        "sandbox_id":   sandboxID,
        "container_id": containerID,
        "base_image":   config.BaseImage,
        "workspace_dir": workspaceDir,
    })
    if err != nil {
        log.Warn().Err(err).Msg("Failed to track sandbox creation")
    }
    
    log.Info().
        Str("sandbox_id", sandboxID).
        Str("container_id", containerID).
        Msg("Sandbox created successfully")
    
    return sandbox, nil
}

func (sm *SandboxManager) createSandboxContainer(ctx context.Context, sandbox *Sandbox) (string, error) {
    // This would use Docker API to create an isolated container
    // For demonstration, we'll simulate container creation
    
    containerConfig := ContainerConfig{
        Image:       sandbox.Config.BaseImage,
        WorkingDir:  "/workspace",
        Memory:      sandbox.Config.MemoryLimit,
        CPUShares:   int64(sandbox.Config.CPULimit * 1024),
        NetworkMode: sandbox.Config.NetworkMode,
        Volumes: []VolumeMount{
            {
                HostPath:      sandbox.WorkspaceDir,
                ContainerPath: "/workspace",
                ReadOnly:      false,
            },
        },
    }
    
    // Simulate container creation
    containerID := fmt.Sprintf("container-%s", sandbox.ID)
    
    log.Debug().
        Str("sandbox_id", sandbox.ID).
        Str("container_id", containerID).
        Interface("config", containerConfig).
        Msg("Creating sandbox container")
    
    return containerID, nil
}

func (sm *SandboxManager) ExecuteInSandbox(ctx context.Context, sandboxID string, command []string) (*ExecutionResult, error) {
    sandbox, exists := sm.sandboxes[sandboxID]
    if !exists {
        return nil, fmt.Errorf("sandbox %s not found", sandboxID)
    }
    
    if sandbox.Status != SandboxStatusRunning {
        return nil, fmt.Errorf("sandbox %s is not running", sandboxID)
    }
    
    log.Info().
        Str("sandbox_id", sandboxID).
        Strs("command", command).
        Msg("Executing command in sandbox")
    
    // Track command execution
    err := sm.infrabot.TrackOperation(ctx, sandbox.SessionID, "sandbox_execute", map[string]interface{}{
        "sandbox_id": sandboxID,
        "command":    command,
    })
    if err != nil {
        log.Warn().Err(err).Msg("Failed to track sandbox execution")
    }
    
    // Execute command in container (simulated)
    result := &ExecutionResult{
        ExitCode: 0,
        Stdout:   "Command executed successfully",
        Stderr:   "",
        Duration: 2 * time.Second,
    }
    
    log.Info().
        Str("sandbox_id", sandboxID).
        Int("exit_code", result.ExitCode).
        Dur("duration", result.Duration).
        Msg("Command execution completed")
    
    return result, nil
}

func (sm *SandboxManager) BuildImageInSandbox(ctx context.Context, sandboxID, dockerfilePath, imageTag string) error {
    sandbox, exists := sm.sandboxes[sandboxID]
    if !exists {
        return fmt.Errorf("sandbox %s not found", sandboxID)
    }
    
    log.Info().
        Str("sandbox_id", sandboxID).
        Str("dockerfile_path", dockerfilePath).
        Str("image_tag", imageTag).
        Msg("Building image in sandbox")
    
    // Use InfraBot to build the image
    // This would integrate with Docker build functionality
    
    // Track build operation
    err := sm.infrabot.TrackOperation(ctx, sandbox.SessionID, "sandbox_build", map[string]interface{}{
        "sandbox_id":      sandboxID,
        "dockerfile_path": dockerfilePath,
        "image_tag":       imageTag,
    })
    if err != nil {
        log.Warn().Err(err).Msg("Failed to track sandbox build")
    }
    
    // After successful build, tag the image
    err = sm.infrabot.TagImage(ctx, sandbox.SessionID, "temp-build-image", imageTag)
    if err != nil {
        return fmt.Errorf("failed to tag built image: %w", err)
    }
    
    log.Info().
        Str("sandbox_id", sandboxID).
        Str("image_tag", imageTag).
        Msg("Image built successfully in sandbox")
    
    return nil
}

func (sm *SandboxManager) DestroySandbox(ctx context.Context, sandboxID string) error {
    sandbox, exists := sm.sandboxes[sandboxID]
    if !exists {
        return fmt.Errorf("sandbox %s not found", sandboxID)
    }
    
    log.Info().
        Str("sandbox_id", sandboxID).
        Msg("Destroying sandbox")
    
    // Stop and remove container
    err := sm.stopSandboxContainer(ctx, sandbox)
    if err != nil {
        log.Warn().Err(err).Msg("Failed to stop sandbox container")
    }
    
    // Clean up workspace
    err = sm.cleanupSandbox(sandbox)
    if err != nil {
        log.Warn().Err(err).Msg("Failed to cleanup sandbox workspace")
    }
    
    // Track sandbox destruction
    err = sm.infrabot.TrackOperation(ctx, sandbox.SessionID, "sandbox_destroy", map[string]interface{}{
        "sandbox_id":   sandboxID,
        "container_id": sandbox.ContainerID,
    })
    if err != nil {
        log.Warn().Err(err).Msg("Failed to track sandbox destruction")
    }
    
    // Clean up session
    err = sm.infrabot.DeleteSession(ctx, sandbox.SessionID)
    if err != nil {
        log.Warn().Err(err).Msg("Failed to delete sandbox session")
    }
    
    delete(sm.sandboxes, sandboxID)
    
    log.Info().
        Str("sandbox_id", sandboxID).
        Msg("Sandbox destroyed successfully")
    
    return nil
}

func (sm *SandboxManager) cleanupSandbox(sandbox *Sandbox) error {
    return os.RemoveAll(sandbox.WorkspaceDir)
}

func (sm *SandboxManager) stopSandboxContainer(ctx context.Context, sandbox *Sandbox) error {
    // This would use Docker API to stop and remove the container
    log.Debug().
        Str("container_id", sandbox.ContainerID).
        Msg("Stopping sandbox container")
    
    return nil
}
```

## Best Practices

### 1. Error Handling and Resilience

```go
type ResilientClient struct {
    client      *InfraBotClient
    retryPolicy RetryPolicy
    circuitBreaker *CircuitBreaker
}

type RetryPolicy struct {
    MaxRetries    int
    BackoffPolicy BackoffPolicy
    RetryableErrors []string
}

func (c *ResilientClient) PullImageWithRetry(ctx context.Context, sessionID, imageRef string) error {
    return c.withRetry(ctx, func() error {
        return c.client.PullImage(ctx, sessionID, imageRef)
    })
}

func (c *ResilientClient) withRetry(ctx context.Context, operation func() error) error {
    var lastErr error
    
    for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
        // Check circuit breaker
        if !c.circuitBreaker.AllowRequest() {
            return fmt.Errorf("circuit breaker open: %w", lastErr)
        }
        
        err := operation()
        if err == nil {
            c.circuitBreaker.RecordSuccess()
            return nil
        }
        
        lastErr = err
        c.circuitBreaker.RecordFailure()
        
        if !c.isRetryableError(err) {
            return err
        }
        
        if attempt < c.retryPolicy.MaxRetries {
            delay := c.calculateBackoffDelay(attempt)
            select {
            case <-time.After(delay):
                continue
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }
    
    return lastErr
}
```

### 2. Resource Management

```go
type ResourceManager struct {
    infrabot InfraBotClient
    activeSessions map[string]*SessionContext
    mutex    sync.RWMutex
}

type SessionContext struct {
    Session     *Session
    Operations  []string
    Resources   []ResourceHandle
    Cleanup     []func() error
}

func (rm *ResourceManager) WithSession(ctx context.Context, team string, fn func(*Session) error) error {
    session, err := rm.infrabot.CreateSession(ctx, team, SessionConfig{})
    if err != nil {
        return err
    }
    
    sessionCtx := &SessionContext{
        Session:    session,
        Operations: make([]string, 0),
        Resources:  make([]ResourceHandle, 0),
        Cleanup:    make([]func() error, 0),
    }
    
    rm.mutex.Lock()
    rm.activeSessions[session.ID] = sessionCtx
    rm.mutex.Unlock()
    
    defer func() {
        rm.cleanupSession(ctx, session.ID)
    }()
    
    return fn(session)
}

func (rm *ResourceManager) cleanupSession(ctx context.Context, sessionID string) {
    rm.mutex.Lock()
    sessionCtx, exists := rm.activeSessions[sessionID]
    if !exists {
        rm.mutex.Unlock()
        return
    }
    delete(rm.activeSessions, sessionID)
    rm.mutex.Unlock()
    
    // Run cleanup functions
    for _, cleanup := range sessionCtx.Cleanup {
        if err := cleanup(); err != nil {
            log.Warn().Err(err).Msg("Cleanup function failed")
        }
    }
    
    // Delete session
    err := rm.infrabot.DeleteSession(ctx, sessionID)
    if err != nil {
        log.Warn().Err(err).Msg("Failed to delete session")
    }
}
```

### 3. Performance Optimization

```go
type PerformanceOptimizedClient struct {
    client  *InfraBotClient
    cache   *sync.Map
    metrics *Metrics
}

func (c *PerformanceOptimizedClient) PullImageCached(ctx context.Context, sessionID, imageRef string) error {
    // Check cache first
    if cached, exists := c.cache.Load(imageRef); exists {
        cachedTime := cached.(time.Time)
        if time.Since(cachedTime) < 1*time.Hour {
            c.metrics.CacheHits.Inc()
            return nil
        }
    }
    
    c.metrics.CacheMisses.Inc()
    
    start := time.Now()
    err := c.client.PullImage(ctx, sessionID, imageRef)
    duration := time.Since(start)
    
    c.metrics.OperationDuration.Observe(duration.Seconds())
    
    if err == nil {
        c.cache.Store(imageRef, time.Now())
    }
    
    return err
}

func (c *PerformanceOptimizedClient) PullImagesParallel(ctx context.Context, sessionID string, imageRefs []string) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(imageRefs))
    
    for _, imageRef := range imageRefs {
        wg.Add(1)
        go func(ref string) {
            defer wg.Done()
            err := c.PullImageCached(ctx, sessionID, ref)
            if err != nil {
                errChan <- fmt.Errorf("failed to pull %s: %w", ref, err)
            }
        }(imageRef)
    }
    
    wg.Wait()
    close(errChan)
    
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("parallel pull errors: %v", errors)
    }
    
    return nil
}
```

## Testing Integration

### Integration Test Example

```go
package integration_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCrossTeamIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Setup test environment
    infrabot := setupTestInfraBot(t)
    defer infrabot.Cleanup()
    
    // Test BuildSecBot integration
    t.Run("BuildSecBot_SecurityScan", func(t *testing.T) {
        testBuildSecBotIntegration(t, ctx, infrabot)
    })
    
    // Test OrchBot integration
    t.Run("OrchBot_WorkflowExecution", func(t *testing.T) {
        testOrchBotIntegration(t, ctx, infrabot)
    })
    
    // Test AdvancedBot integration
    t.Run("AdvancedBot_SandboxOperations", func(t *testing.T) {
        testAdvancedBotIntegration(t, ctx, infrabot)
    })
}

func testBuildSecBotIntegration(t *testing.T, ctx context.Context, infrabot *InfraBotClient) {
    // Create session
    session, err := infrabot.CreateSession(ctx, "buildsecbot", SessionConfig{
        Timeout: 10 * time.Minute,
        Tags:    []string{"integration-test", "security"},
    })
    require.NoError(t, err)
    defer infrabot.DeleteSession(ctx, session.ID)
    
    // Pull test image
    err = infrabot.PullImage(ctx, session.ID, "alpine:latest")
    require.NoError(t, err)
    
    // Simulate security scan using atomic framework
    scanner := NewSecurityScanner(infrabot)
    result, err := scanner.ScanImage(ctx, session.ID, "alpine:latest")
    require.NoError(t, err)
    
    // Verify scan results
    assert.NotNil(t, result)
    assert.Equal(t, "alpine:latest", result.ImageRef)
    assert.True(t, result.ComplianceReport.Score > 0)
    
    // Verify session tracking
    sessionDetails, err := infrabot.GetSession(ctx, session.ID)
    require.NoError(t, err)
    assert.Contains(t, sessionDetails.Operations, "security_scan")
}

func testOrchBotIntegration(t *testing.T, ctx context.Context, infrabot *InfraBotClient) {
    orchestrator := NewWorkflowOrchestrator(infrabot)
    
    // Create workflow
    workflow, err := orchestrator.CreateContainerizationWorkflow(
        "test-app",
        "source:latest",
        "target:v1.0",
    )
    require.NoError(t, err)
    
    // Execute workflow
    err = orchestrator.ExecuteWorkflow(ctx, workflow.ID)
    require.NoError(t, err)
    
    // Verify workflow completion
    assert.Equal(t, WorkflowStatusCompleted, workflow.Status)
    assert.Equal(t, len(workflow.Steps)-1, workflow.CurrentStep)
}

func testAdvancedBotIntegration(t *testing.T, ctx context.Context, infrabot *InfraBotClient) {
    sandboxManager := NewSandboxManager(infrabot, "ubuntu:20.04")
    
    // Create sandbox
    config := SandboxConfig{
        BaseImage:   "ubuntu:20.04",
        MemoryLimit: 1024 * 1024 * 1024, // 1GB
        CPULimit:    1.0,
    }
    
    sandbox, err := sandboxManager.CreateSandbox(ctx, config)
    require.NoError(t, err)
    defer sandboxManager.DestroySandbox(ctx, sandbox.ID)
    
    // Execute command in sandbox
    result, err := sandboxManager.ExecuteInSandbox(ctx, sandbox.ID, []string{"echo", "test"})
    require.NoError(t, err)
    assert.Equal(t, 0, result.ExitCode)
    
    // Verify sandbox isolation
    assert.Equal(t, SandboxStatusRunning, sandbox.Status)
    assert.NotEmpty(t, sandbox.ContainerID)
}
```

## Troubleshooting

### Common Integration Issues

#### 1. Session Management Issues

```go
// Problem: Sessions not being cleaned up properly
// Solution: Use context and defer patterns

func (c *Client) WithManagedSession(ctx context.Context, team string, fn func(*Session) error) error {
    session, err := c.CreateSession(ctx, team, SessionConfig{})
    if err != nil {
        return err
    }
    
    // Always clean up session
    defer func() {
        if err := c.DeleteSession(context.Background(), session.ID); err != nil {
            log.Warn().Err(err).Str("session_id", session.ID).Msg("Failed to cleanup session")
        }
    }()
    
    return fn(session)
}
```

#### 2. Authentication Issues

```go
// Problem: Docker registry authentication failures
// Solution: Implement credential management

type CredentialManager struct {
    credentials map[string]RegistryCredentials
    mutex       sync.RWMutex
}

func (cm *CredentialManager) GetCredentials(registry string) (RegistryCredentials, error) {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()
    
    creds, exists := cm.credentials[registry]
    if !exists {
        return RegistryCredentials{}, fmt.Errorf("no credentials for registry %s", registry)
    }
    
    // Check if credentials are expired
    if time.Now().After(creds.ExpiresAt) {
        return RegistryCredentials{}, fmt.Errorf("credentials expired for registry %s", registry)
    }
    
    return creds, nil
}
```

#### 3. Performance Issues

```go
// Problem: Slow operations due to blocking calls
// Solution: Use context cancellation and timeouts

func (c *Client) PullImageWithTimeout(ctx context.Context, sessionID, imageRef string, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    done := make(chan error, 1)
    go func() {
        done <- c.PullImage(ctx, sessionID, imageRef)
    }()
    
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return fmt.Errorf("pull operation timed out after %v", timeout)
    }
}
```

### Debugging Tips

1. **Enable Debug Logging**: Set log level to debug for detailed operation traces
2. **Monitor Metrics**: Use Prometheus metrics to identify bottlenecks
3. **Check Session State**: Always verify session status before operations
4. **Validate Dependencies**: Ensure all required services are running
5. **Test with Small Images**: Use small test images for faster debugging

### Support Resources

- **Documentation**: [InfraBot Documentation](README.md)
- **API Reference**: [API Reference](api-reference.md)
- **Issue Tracking**: [GitHub Issues](https://github.com/Azure/container-kit/issues)
- **Team Contact**: infrabot-team@example.com

---

This integration guide provides comprehensive examples and best practices for integrating with InfraBot. For additional support or specific integration questions, please refer to the support resources above.