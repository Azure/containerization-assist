package cmd

import (
	"strings"

	"github.com/Azure/container-copilot/pkg/logger"
)

// isLLMValidationError checks if the error is related to LLM validation failure
func isLLMValidationError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "LLM configuration validation failed") ||
		strings.Contains(errStr, "failed to get chat completion") ||
		strings.Contains(errStr, "empty chat response content")
}

// printLLMValidationHelp displays helpful guidance for LLM validation failures
func printLLMValidationHelp() {
	logger.Error("\n🔧 Troubleshooting Azure OpenAI Connection Issues:")
	logger.Error("   • Your Azure OpenAI deployment may be expired or invalid")
	logger.Error("   • The deployment might have been deleted or modified")
	logger.Error("   • Network connectivity issues or authentication problems")
	logger.Error("   • Environment variables may point to outdated resources")
	logger.Error("\n💡 Solution:")
	logger.Error("   Run 'container-copilot setup --force-setup' to recreate your Azure OpenAI resources")
	logger.Error("   This will provision new resources with fresh deployments")
	logger.Error("")
}
