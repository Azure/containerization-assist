package conversation

// Services provides access to all conversation-related services
type ConversationServices interface {
	// PromptService returns the prompt processing service
	PromptService() ConversationPromptService
}

// conversationServices implements ConversationServices
type conversationServices struct {
	promptService ConversationPromptService
}

// NewConversationServices creates a new ConversationServices container
func NewConversationServices(config PromptManagerConfig) ConversationServices {
	// Create the prompt service
	promptService := NewPromptService(config)

	return &conversationServices{
		promptService: promptService,
	}
}

// NewConversationServicesFromManager creates services from an existing PromptManager
// This is useful for gradual migration
func NewConversationServicesFromManager(manager *PromptManager) ConversationServices {
	return &conversationServices{
		promptService: manager,
	}
}

func (cs *conversationServices) PromptService() ConversationPromptService {
	return cs.promptService
}
