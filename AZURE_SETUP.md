# Azure OpenAI Environment Setup

## Correct Environment Variables

**IMPORTANT**: Always use real Azure OpenAI credentials, NEVER use mock endpoints.

### For LLM Integration Tests

```bash
source ../azure_keys.sh
```

This sets up the real Azure OpenAI GPT-5 environment with:
- Real Azure endpoint (https://containerization-assist-e2e.cognitiveservices.azure.com/)
- Valid API key
- GPT-5 deployment ID

### NEVER Use Mock Endpoints

❌ **DO NOT USE**: `OPENAI_BASE_URL=http://localhost:4141/v1`
❌ **DO NOT USE**: `OPENAI_API_KEY=dummy`

These mock endpoints should never be used for actual testing or development.

## Running Tests

Correct way to run LLM integration tests:

```bash
source ../azure_keys.sh && npm test test/llm-integration/simple-containerization.test.ts
```

## Background Processes

The autonomous containerization agents are running in the background and should use the real Azure credentials loaded via `source ../azure_keys.sh`.