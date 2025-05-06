package llmvalidator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/container-copilot/pkg/logger"
)

type LLMConfig struct {
	Endpoint     string
	APIKey       string
	DeploymentID string
}

func ValidateLLM(llmConfig LLMConfig) error {

	_, err := url.ParseRequestURI(llmConfig.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if llmConfig.APIKey == "" {
		return errors.New("API key is missing")
	}

	if llmConfig.DeploymentID == "" {
		return errors.New("deployment ID is missing")
	}

	// Attempt to send a minimal test request to the endpoint
	testPayload := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
		"max_tokens":  5,
		"temperature": 0,
		"stop":        []string{"\n"},
	}

	payloadBytes, _ := json.Marshal(testPayload)
	// Doc here: https://learn.microsoft.com/en-us/azure/ai-services/openai/reference#rest-api-versioning
	// POST https://YOUR_RESOURCE_NAME.openai.azure.com/openai/deployments/YOUR_DEPLOYMENT_NAME/chat/completions?api-version=2024-06-01
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=2024-06-01", llmConfig.Endpoint, llmConfig.DeploymentID), bytes.NewBuffer(payloadBytes))
	if err != nil {
		logger.Errorf("failed to create test request: %v", err)
		return fmt.Errorf("failed to create test request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", llmConfig.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		logger.Errorf("failed to contact LLM endpoint: %v", err)
		return fmt.Errorf("failed to contact LLM endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Errorf("LLM validation failed: received status %d", resp.StatusCode)
		return fmt.Errorf("LLM validation failed: received status %d", resp.StatusCode)
	}

	return nil
}
