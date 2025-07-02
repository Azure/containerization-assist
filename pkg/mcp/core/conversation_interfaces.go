package core

// Conversation-specific types and configurations
// These are primarily used by the conversation/chat functionality

// ConversationConfig holds configuration for conversation mode
type ConversationConfig struct {
	EnableTelemetry          bool
	TelemetryPort            int
	PreferencesDBPath        string
	PreferencesEncryptionKey string // Optional encryption key for preference store

	// OpenTelemetry configuration
	EnableOTEL      bool
	OTELEndpoint    string
	OTELHeaders     map[string]string
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// ConversationStage represents different stages of conversation
type ConversationStage string

const (
	ConversationStagePreFlight  ConversationStage = "preflight"
	ConversationStageAnalyze    ConversationStage = "analyze"
	ConversationStageDockerfile ConversationStage = "dockerfile"
	ConversationStageBuild      ConversationStage = "build"
	ConversationStagePush       ConversationStage = "push"
	ConversationStageManifests  ConversationStage = "manifests"
	ConversationStageDeploy     ConversationStage = "deploy"
	ConversationStageScan       ConversationStage = "scan"
	ConversationStageCompleted  ConversationStage = "completed"
	ConversationStageError      ConversationStage = "error"
)
