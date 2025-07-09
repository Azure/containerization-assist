package conversation

// Services provides access to all conversation-related services
type Services interface {
	// PromptService returns the prompt processing service
	PromptService() PromptService
}

// conversationServices implements Services
type conversationServices struct {
	promptService PromptService
}

// NewConversationServices creates a new Services container
func NewConversationServices(config PromptManagerConfig) Services {
	// Create the prompt service
	promptService := NewPromptService(config)

	return &conversationServices{
		promptService: promptService,
	}
}

// NewConversationServicesFromManager creates services from an existing PromptManager
// This is useful for gradual migration
func NewConversationServicesFromManager(manager *PromptManager) Services {
	return &conversationServices{
		promptService: manager,
	}
}

func (cs *conversationServices) PromptService() PromptService {
	return cs.promptService
}
