package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// GrabContentBetweenTags extracts content between start and end tags
// For example, LLM should produce a response that would include a new file inbetween two tags
// for easy regex grabbing
func GrabContentBetweenTags(content string, tag string) (string, error) {
	// Create pattern for <<<TAG>>>content<<<TAG>>> format
	pattern := fmt.Sprintf(`<<<%s>>>([\s\S]*?)<<<%s>>>`, tag, tag)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)

	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}

	return "", fmt.Errorf("content between tags <%s> not found", tag)
}

// SplitContentAtTags extracts both the content inside the specified tags and everything outside of the tags
// It returns two strings: the content inside the tags and all content outside the tags.
// The tags themselves are excluded from both returned strings.
// For example, with content "Before <<<TAG>>>Middle<<<TAG>>> After", it returns "Middle" and "Before  After"
func SplitContentAtTags(content string, tag string) (string, string, error) {
	// Create pattern for <<<TAG>>>content<<<TAG>>> format
	pattern := fmt.Sprintf(`<<<%s>>>([\s\S]*?)<<<%s>>>`, tag, tag)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)

	if len(matches) > 1 {
		insideContent := strings.TrimSpace(matches[1])

		// Get the outside content by replacing the entire match (including tags) with empty string
		fullMatch := matches[0]
		outsideContent := strings.Replace(content, fullMatch, "", 1)
		outsideContent = strings.TrimSpace(outsideContent)

		return insideContent, outsideContent, nil
	}

	return "", content, fmt.Errorf("content between tags <%s> not found", tag)
}
