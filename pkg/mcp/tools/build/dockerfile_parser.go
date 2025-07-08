package build

import (
	"strings"
)

// ContextInstruction represents a Dockerfile instruction that affects build context
type ContextInstruction struct {
	Type string
	Args []string
	Line int
}

// extractContextInstructions extracts context-related instructions from Dockerfile content
func extractContextInstructions(content string) []ContextInstruction {
	var instructions []ContextInstruction
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "COPY ") || strings.HasPrefix(upper, "ADD ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				instructions = append(instructions, ContextInstruction{
					Type: strings.ToUpper(parts[0]),
					Args: parts[1:],
					Line: i + 1,
				})
			}
		}
	}

	return instructions
}

// extractBaseImages extracts base images from FROM instructions in Dockerfile content
func extractBaseImages(content string) []string {
	var images []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "FROM ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Handle multi-stage builds (FROM image AS stage)
				imageRef := parts[1]
				// Skip stage references (stage names in FROM stage AS newstage)
				if !strings.Contains(strings.ToLower(imageRef), "scratch") || imageRef == "scratch" {
					images = append(images, imageRef)
				}
			}
		}
	}

	return images
}
