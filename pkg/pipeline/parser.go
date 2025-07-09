package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Parser defines an interface for extracting content from string responses
type Parser interface {
	// ExtractContent extracts content between specified tags in the given text
	ExtractContent(content string, tag string) (string, error)
}

// DefaultParser provides the default implementation of the Parser interface
type DefaultParser struct{}

// ExtractContent extracts content between tags in the format <TAG>content</TAG>
func (p *DefaultParser) ExtractContent(content string, tag string) (string, error) {
	// Create pattern to extract content between the last matching tag pair of <TAG>content<TAG> format
	pattern := fmt.Sprintf("(?s).*<%s>([\\s\\S]*?)</%s>", regexp.QuoteMeta(tag), regexp.QuoteMeta(tag))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)

	if len(matches) < 2 {
		return "", mcperrors.NewError().Messagef("content between tags <%s> not found", tag).WithLocation().Build()
	}

	innerContent := strings.TrimSpace(matches[1])
	if innerContent == "" {
		return "", mcperrors.NewError().Messagef("content between tags <%s> is empty", tag).WithLocation().Build()
	}

	return innerContent, nil
}
