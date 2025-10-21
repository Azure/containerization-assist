# LLM Integration Tests

This directory contains tests that validate the complete LLM → MCP → Tool workflow with real LLM interactions.

## Test Structure

- `infrastructure/` - LLM client adapters and test utilities
- `scenarios/` - End-to-end workflow test scenarios
- `fixtures/` - Real-world project templates and test data
- `validation/` - Response quality and correctness validation
- `performance/` - Load and performance testing

## Environment Setup

LLM integration tests use a simplified setup with ChatClient that connects to the test harness.
No additional environment variables required for basic testing.

## Usage

```bash
# Run all LLM integration tests
npm run test:llm

# Run specific scenario
npm run test:llm -- --testNamePattern="Dockerfile Generation"
```

## Test Categories

1. **Single Tool Tests** - Individual tool usage with LLM
2. **Workflow Tests** - Multi-tool containerization scenarios
3. **Error Handling** - Edge cases and failure recovery
4. **Performance Tests** - Latency and throughput validation
5. **Quality Tests** - Response accuracy and usefulness