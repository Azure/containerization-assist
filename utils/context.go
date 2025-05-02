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
	// Create pattern to  extract content between the last matching tag pair of <TAG>content<TAG> format
	pattern := fmt.Sprintf("(?s).*<%s>([\\s\\S]*?)</%s>", regexp.QuoteMeta(tag), regexp.QuoteMeta(tag))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return "", fmt.Errorf("content between tags <%s> not found", tag)
	}
	
	return strings.TrimSpace(matches[1]), nil
}
