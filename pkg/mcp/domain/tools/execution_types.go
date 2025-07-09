package tools

// ExecutionResult represents the result of tool execution
type ExecutionResult struct {
	Content  []ContentBlock `json:"content"`
	IsError  bool           `json:"is_error"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ContentBlock represents a content block in the execution result
type ContentBlock struct {
	Type string                 `json:"type"`
	Text string                 `json:"text,omitempty"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// IsSuccess returns whether the execution was successful
func (r *ExecutionResult) IsSuccess() bool {
	return !r.IsError
}

// AddTextContent adds a text content block to the result
func (r *ExecutionResult) AddTextContent(text string) {
	r.Content = append(r.Content, ContentBlock{
		Type: "text",
		Text: text,
	})
}

// AddDataContent adds a data content block to the result
func (r *ExecutionResult) AddDataContent(data map[string]interface{}) {
	r.Content = append(r.Content, ContentBlock{
		Type: "data",
		Data: data,
	})
}

// SetMetadata sets metadata for the execution result
func (r *ExecutionResult) SetMetadata(key string, value any) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}
	r.Metadata[key] = value
}

// NewExecutionResult creates a new execution result
func NewExecutionResult() *ExecutionResult {
	return &ExecutionResult{
		Content: make([]ContentBlock, 0),
		IsError: false,
	}
}

// NewErrorResult creates a new error result with a text message
func NewErrorResult(message string) *ExecutionResult {
	return &ExecutionResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: message,
		}},
		IsError: true,
	}
}

// NewSuccessResult creates a new success result with text content
func NewSuccessResult(text string) *ExecutionResult {
	return &ExecutionResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: text,
		}},
		IsError: false,
	}
}

// NewDataResult creates a new success result with data content
func NewDataResult(data map[string]interface{}) *ExecutionResult {
	return &ExecutionResult{
		Content: []ContentBlock{{
			Type: "data",
			Data: data,
		}},
		IsError: false,
	}
}

// NewMixedResult creates a new result with both text and data content
func NewMixedResult(text string, data map[string]interface{}) *ExecutionResult {
	return &ExecutionResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
			{
				Type: "data",
				Data: data,
			},
		},
		IsError: false,
	}
}
