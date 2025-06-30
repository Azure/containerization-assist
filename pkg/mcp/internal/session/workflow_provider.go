package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/utils"
)

type WorkflowLabelProvider struct {
	ToolBasedLabels  bool
	TimeBasedLabels  bool
	StageBasedLabels bool
	ProgressLabels   bool
}

type LabelProvider interface {
	GetLabels(session *SessionState) ([]string, error)
	GetK8sLabels(session *SessionState) (map[string]string, error)
	GetName() string
	IsEnabled() bool
}

func NewWorkflowLabelProvider() *WorkflowLabelProvider {
	return &WorkflowLabelProvider{
		ToolBasedLabels:  true,
		TimeBasedLabels:  true,
		StageBasedLabels: true,
		ProgressLabels:   true,
	}
}

func (w *WorkflowLabelProvider) GetName() string {
	return "workflow"
}

func (w *WorkflowLabelProvider) IsEnabled() bool {
	return w.ToolBasedLabels || w.TimeBasedLabels || w.StageBasedLabels || w.ProgressLabels
}

func (w *WorkflowLabelProvider) GetLabels(session *SessionState) ([]string, error) {
	var labels []string

	if w.TimeBasedLabels {
		timeLabels := w.generateTimeLabels(session)
		labels = append(labels, timeLabels...)
	}

	if w.ToolBasedLabels {
		toolLabels := w.generateToolLabels(session)
		labels = append(labels, toolLabels...)
	}

	if w.StageBasedLabels {
		stageLabels := w.generateStageLabels(session)
		labels = append(labels, stageLabels...)
	}

	if w.ProgressLabels {
		progressLabels := w.generateProgressLabels(session)
		labels = append(labels, progressLabels...)
	}

	return labels, nil
}

func (w *WorkflowLabelProvider) GetK8sLabels(session *SessionState) (map[string]string, error) {
	k8sLabels := make(map[string]string)

	k8sLabels["mcp.session.id"] = session.SessionID

	k8sLabels["mcp.session.created"] = session.CreatedAt.Format("2006-01-02")

	if session.ImageRef.String() != "" {
		imageName := utils.SanitizeForKubernetes(session.ImageRef.Repository)
		if imageName != "" {
			k8sLabels["mcp.image.name"] = imageName
		}

		if session.ImageRef.Tag != "" {
			imageTag := utils.SanitizeForKubernetes(session.ImageRef.Tag)
			if imageTag != "" {
				k8sLabels["mcp.image.tag"] = imageTag
			}
		}
	}

	if session.RepoURL != "" {
		repoName := w.extractRepoName(session.RepoURL)
		if repoName != "" {
			k8sLabels["mcp.repo.name"] = utils.SanitizeForKubernetes(repoName)
		}
	}

	if stage := w.determineWorkflowStage(session); stage != "" {
		k8sLabels["mcp.workflow.stage"] = stage
	}

	return k8sLabels, nil
}

func (w *WorkflowLabelProvider) generateTimeLabels(session *SessionState) []string {
	var labels []string

	now := time.Now()
	created := session.CreatedAt

	labels = append(labels, fmt.Sprintf("created:%s", created.Format("2006-01")))
	labels = append(labels, fmt.Sprintf("day:%s", strings.ToLower(created.Weekday().String())))

	hour := created.Hour()
	if hour >= 9 && hour < 17 {
		labels = append(labels, "shift:business-hours")
	} else {
		labels = append(labels, "shift:after-hours")
	}

	age := now.Sub(created)
	if age < time.Hour {
		labels = append(labels, "age:fresh")
	} else if age < 24*time.Hour {
		labels = append(labels, "age:recent")
	} else if age < 7*24*time.Hour {
		labels = append(labels, "age:week")
	} else {
		labels = append(labels, "age:old")
	}

	return labels
}

func (w *WorkflowLabelProvider) generateToolLabels(session *SessionState) []string {
	var labels []string
	var toolsUsed []string

	for _, execution := range session.StageHistory {
		toolName := w.extractToolName(execution.Tool)
		if toolName != "" && !w.contains(toolsUsed, toolName) {
			toolsUsed = append(toolsUsed, toolName)
		}
	}

	for _, tool := range toolsUsed {
		labels = append(labels, fmt.Sprintf("tool:%s", tool))
	}

	if len(toolsUsed) > 1 {
		labels = append(labels, fmt.Sprintf("tools:%s", strings.Join(toolsUsed, ",")))
	}

	if len(session.StageHistory) > 0 {
		lastExecution := session.StageHistory[len(session.StageHistory)-1]
		lastTool := w.extractToolName(lastExecution.Tool)
		if lastTool != "" {
			labels = append(labels, fmt.Sprintf("last-tool:%s", lastTool))
		}
	}

	return labels
}

func (w *WorkflowLabelProvider) generateStageLabels(session *SessionState) []string {
	var labels []string

	stage := w.determineWorkflowStage(session)
	if stage != "" {
		labels = append(labels, fmt.Sprintf("workflow.stage/%s", stage))
	}

	status := w.determineSessionStatus(session)
	if status != "" {
		labels = append(labels, fmt.Sprintf("status:%s", status))
	}

	return labels
}

func (w *WorkflowLabelProvider) generateProgressLabels(session *SessionState) []string {
	var labels []string

	progress := w.calculateProgress(session)
	if progress >= 0 {
		roundedProgress := (progress / 25) * 25
		labels = append(labels, fmt.Sprintf("progress/%d", roundedProgress))
	}

	return labels
}

func (w *WorkflowLabelProvider) determineWorkflowStage(session *SessionState) string {
	if session.LastError != nil {
		return "failed"
	}

	if len(session.ActiveJobs) > 0 {
		return "in-progress"
	}

	var hasAnalysis, hasBuild, hasDeploy bool

	for _, execution := range session.StageHistory {
		toolName := strings.ToLower(execution.Tool)

		if strings.Contains(toolName, "analyze") || strings.Contains(toolName, "scan") {
			hasAnalysis = true
		} else if strings.Contains(toolName, "build") || strings.Contains(toolName, "dockerfile") {
			hasBuild = true
		} else if strings.Contains(toolName, "deploy") || strings.Contains(toolName, "manifest") {
			hasDeploy = true
		}
	}

	if hasDeploy {
		return "completed"
	} else if hasBuild {
		return "deploy"
	} else if hasAnalysis {
		return "build"
	} else {
		return "analysis"
	}
}

func (w *WorkflowLabelProvider) determineSessionStatus(session *SessionState) string {
	if session.LastError != nil {
		return "error"
	}

	if len(session.ActiveJobs) > 0 {
		return "in-progress"
	}

	now := time.Now()
	if now.Sub(session.LastAccessed) < time.Hour {
		return "active"
	} else if now.Sub(session.LastAccessed) < 24*time.Hour {
		return "idle"
	} else {
		return "stale"
	}
}

func (w *WorkflowLabelProvider) calculateProgress(session *SessionState) int {
	progress := 0

	if len(session.RepoAnalysis) > 0 {
		progress += 25
	}

	if session.Dockerfile.Built {
		progress += 25
	}

	if len(session.K8sManifests) > 0 {
		progress += 25
	}

	if session.Dockerfile.Pushed {
		progress += 25
	}

	return progress
}

func (w *WorkflowLabelProvider) extractToolName(fullName string) string {
	name := strings.ToLower(fullName)
	name = strings.TrimPrefix(name, "atomic_")
	name = strings.TrimSuffix(name, "_tool")
	name = strings.TrimSuffix(name, "_atomic")

	if strings.Contains(name, "build") {
		return "build"
	} else if strings.Contains(name, "deploy") {
		return "deploy"
	} else if strings.Contains(name, "analyze") {
		return "analyze"
	} else if strings.Contains(name, "manifest") {
		return "manifest"
	} else if strings.Contains(name, "scan") {
		return "scan"
	}

	return name
}

func (w *WorkflowLabelProvider) extractRepoName(repoURL string) string {
	if strings.Contains(repoURL, "github.com/") {
		parts := strings.Split(repoURL, "/")
		if len(parts) >= 2 {
			return parts[len(parts)-1]
		}
	}

	return ""
}

func (w *WorkflowLabelProvider) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
