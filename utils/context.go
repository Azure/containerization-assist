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
	// Create pattern for <TAG>content<TAG> format
	pattern := fmt.Sprintf(`<%s>([\s\S]*?)</%s>`, tag, tag)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)

	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}

	return "", fmt.Errorf("content between tags <%s> not found", tag)
}
