package build

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/knowledge"
)

// CrossToolKnowledgeBase is now in the knowledge package
// This file provides backward compatibility

// CrossToolKnowledgeBase is an alias for the knowledge package type
type CrossToolKnowledgeBase = knowledge.CrossToolKnowledgeBase

// NewCrossToolKnowledgeBase creates a new knowledge base using the knowledge package
func NewCrossToolKnowledgeBase(logger *slog.Logger) *CrossToolKnowledgeBase {
	return knowledge.NewCrossToolKnowledgeBase(logger)
}
