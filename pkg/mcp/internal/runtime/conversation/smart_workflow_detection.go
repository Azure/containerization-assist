package conversation

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

type WorkflowIntent int

const (
	IntentUnknown WorkflowIntent = iota
	IntentContainerizeApp
	IntentInteractiveGuide
	IntentExploreOptions
	IntentSpecificTask
)

type ContainerizationRequest struct {
	Intent      WorkflowIntent
	RepoURL     string
	LocalPath   string
	AppType     string
	HasSpecific bool
	AutoPilot   bool
}
type SmartWorkflowDetector struct {
	pm *PromptManager
}

func NewSmartWorkflowDetector(pm *PromptManager) *SmartWorkflowDetector {
	return &SmartWorkflowDetector{
		pm: pm,
	}
}
func (swd *SmartWorkflowDetector) DetectContainerizationIntent(ctx context.Context, userInput string) *ContainerizationRequest {
	input := strings.ToLower(strings.TrimSpace(userInput))

	request := &ContainerizationRequest{
		Intent:    IntentUnknown,
		AutoPilot: true,
	}
	containerizeKeywords := []string{
		"containerize", "dockerize", "docker", "container",
		"build image", "create dockerfile", "deploy", "kubernetes", "k8s",
		"make container", "package app", "ship app", "deploy app",
	}

	hasContainerizeIntent := false
	for _, keyword := range containerizeKeywords {
		if strings.Contains(input, keyword) {
			hasContainerizeIntent = true
			break
		}
	}
	repoIndicators := []string{
		"github.com", "gitlab.com", "bitbucket.com", "git@",
		"https://", "http://",
		"/", "./", "../", "~/",
		"my app", "my project", "this repo", "this project",
	}

	hasRepoIndicator := false
	for _, indicator := range repoIndicators {
		if strings.Contains(input, indicator) {
			hasRepoIndicator = true

			if strings.Contains(input, "http") {
				request.RepoURL = swd.extractURL(input)
			} else if strings.Contains(input, "/") {
				request.LocalPath = swd.extractPath(input)
			}
			break
		}
	}
	directActionPhrases := []string{
		"containerize my app", "dockerize my app", "build a docker image",
		"create a container", "deploy my app", "make this into a container",
		"package this app", "ship this application", "containerize this",
		"turn this into a container", "build and deploy", "full containerization",
	}

	hasDirectAction := false
	for _, phrase := range directActionPhrases {
		if strings.Contains(input, phrase) {
			hasDirectAction = true
			break
		}
	}
	interactiveKeywords := []string{
		"help me", "guide me", "step by step", "walk me through",
		"explain", "show me how", "what should i do",
		"interactive", "manually", "one step at a time",
	}

	hasInteractiveIntent := false
	for _, keyword := range interactiveKeywords {
		if strings.Contains(input, keyword) {
			hasInteractiveIntent = true
			break
		}
	}
	switch {
	case hasDirectAction || (hasContainerizeIntent && hasRepoIndicator):
		request.Intent = IntentContainerizeApp
		request.AutoPilot = true

	case hasContainerizeIntent && hasInteractiveIntent:
		request.Intent = IntentInteractiveGuide
		request.AutoPilot = false

	case hasContainerizeIntent:
		request.Intent = IntentContainerizeApp
		request.AutoPilot = true

	case hasInteractiveIntent:
		request.Intent = IntentInteractiveGuide
		request.AutoPilot = false

	default:
		request.Intent = IntentExploreOptions
		request.AutoPilot = false
	}
	appTypeHints := map[string]string{
		"node":    "Node.js",
		"react":   "React",
		"vue":     "Vue.js",
		"angular": "Angular",
		"python":  "Python",
		"flask":   "Flask",
		"django":  "Django",
		"fastapi": "FastAPI",
		"go":      "Go",
		"golang":  "Go",
		"java":    "Java",
		"spring":  "Spring Boot",
		"rust":    "Rust",
		"dotnet":  ".NET",
		"php":     "PHP",
		"laravel": "Laravel",
		"ruby":    "Ruby",
		"rails":   "Ruby on Rails",
	}

	for hint, appType := range appTypeHints {
		if strings.Contains(input, hint) {
			request.AppType = appType
			break
		}
	}

	return request
}
func (swd *SmartWorkflowDetector) HandleSmartWorkflow(ctx context.Context, state *ConversationState, userInput string) *ConversationResponse {
	request := swd.DetectContainerizationIntent(ctx, userInput)

	switch request.Intent {
	case IntentContainerizeApp:
		return swd.handleContainerizeAppIntent(ctx, state, userInput, request)

	case IntentInteractiveGuide:
		return swd.handleInteractiveGuideIntent(ctx, state, userInput, request)

	default:

		return swd.pm.handleWelcomeStage(ctx, state, userInput)
	}
}
func (swd *SmartWorkflowDetector) handleContainerizeAppIntent(ctx context.Context, state *ConversationState, userInput string, request *ContainerizationRequest) *ConversationResponse {

	swd.pm.enableAutopilot(state)
	state.Context["skip_confirmations"] = true
	state.Context["smart_workflow_detected"] = true
	state.Context["detected_intent"] = "containerize_app"

	if request.AppType != "" {
		state.Context["detected_app_type"] = request.AppType
	}
	message := "🚀 **Smart Containerization Workflow Detected!**\n\n"
	message += "I'll help you containerize your application automatically with minimal confirmations.\n\n"

	if request.AppType != "" {
		message += fmt.Sprintf("**Detected App Type**: %s\n", request.AppType)
	}

	message += "**Autopilot Mode**: ✅ Enabled (I'll proceed through all stages automatically)\n"
	message += "**Control**: You can type 'stop' or 'pause' at any time to take manual control\n\n"
	if request.RepoURL != "" {
		state.Context["repo_url"] = request.RepoURL
		state.SetStage(convertFromTypesStage(types.StageAnalysis))
		message += fmt.Sprintf("**Repository**: %s\n\n", request.RepoURL)
		message += "Starting repository analysis..."

		response := &ConversationResponse{
			Message: message,
			Stage:   convertFromTypesStage(types.StageAnalysis),
			Status:  ResponseStatusProcessing,
		}

		return response.WithAutoAdvance(convertFromTypesStage(types.StageAnalysis), AutoAdvanceConfig{
			DelaySeconds:  1,
			Confidence:    0.9,
			Reason:        "Repository URL detected, starting analysis",
			CanCancel:     true,
			DefaultAction: "analyze",
		})

	} else if request.LocalPath != "" {
		state.Context["local_path"] = request.LocalPath
		state.SetStage(convertFromTypesStage(types.StageAnalysis))
		message += fmt.Sprintf("**Local Path**: %s\n\n", request.LocalPath)
		message += "Starting repository analysis..."

		response := &ConversationResponse{
			Message: message,
			Stage:   convertFromTypesStage(types.StageAnalysis),
			Status:  ResponseStatusProcessing,
		}

		return response.WithAutoAdvance(convertFromTypesStage(types.StageAnalysis), AutoAdvanceConfig{
			DelaySeconds:  1,
			Confidence:    0.9,
			Reason:        "Local path detected, starting analysis",
			CanCancel:     true,
			DefaultAction: "analyze",
		})

	} else {

		state.SetStage(convertFromTypesStage(types.StageInit))
		message += "Please provide your repository URL or local path to get started:"

		return &ConversationResponse{
			Message: message,
			Stage:   convertFromTypesStage(types.StageInit),
			Status:  ResponseStatusWaitingInput,
			Options: []Option{
				{
					ID:          "github",
					Label:       "GitHub Repository",
					Description: "e.g., https://github.com/user/repo",
				},
				{
					ID:          "local",
					Label:       "Local Path",
					Description: "e.g., /path/to/your/project or ./my-app",
				},
			},
		}
	}
}
func (swd *SmartWorkflowDetector) handleInteractiveGuideIntent(ctx context.Context, state *ConversationState, userInput string, request *ContainerizationRequest) *ConversationResponse {

	swd.pm.disableAutopilot(state)
	state.Context["smart_workflow_detected"] = true
	state.Context["detected_intent"] = "interactive_guide"

	message := "📚 **Interactive Containerization Guide**\n\n"
	message += "I'll guide you step-by-step through containerizing your application.\n\n"
	message += "**Interactive Mode**: ✅ Enabled (I'll ask for confirmation at each stage)\n"
	message += "**Control**: You can type 'autopilot on' at any time to enable automatic progression\n\n"

	if request.AppType != "" {
		message += fmt.Sprintf("**Detected App Type**: %s\n\n", request.AppType)
		state.Context["detected_app_type"] = request.AppType
	}

	state.SetStage(convertFromTypesStage(types.StageInit))
	message += "Let's start by analyzing your repository. Please provide the repository URL or local path:"

	return &ConversationResponse{
		Message: message,
		Stage:   convertFromTypesStage(types.StageInit),
		Status:  ResponseStatusWaitingInput,
		Options: []Option{
			{
				ID:          "github",
				Label:       "GitHub Repository",
				Description: "e.g., https://github.com/user/repo",
				Recommended: true,
			},
			{
				ID:          "local",
				Label:       "Local Path",
				Description: "e.g., /path/to/your/project",
			},
		},
	}
}
func (swd *SmartWorkflowDetector) extractURL(input string) string {

	words := strings.Fields(input)
	for _, word := range words {
		if strings.HasPrefix(word, "http") || strings.Contains(word, "github.com") || strings.Contains(word, "gitlab.com") {
			return strings.TrimRight(word, ".,!?")
		}
	}
	return ""
}
func (swd *SmartWorkflowDetector) extractPath(input string) string {

	words := strings.Fields(input)
	for _, word := range words {
		if strings.HasPrefix(word, "/") || strings.HasPrefix(word, "./") || strings.HasPrefix(word, "../") || strings.HasPrefix(word, "~/") {
			return strings.TrimRight(word, ".,!?")
		}
	}
	return ""
}
