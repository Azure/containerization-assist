# Docker Client Error Handling Integration Tests

This directory contains comprehensive integration tests for the enhanced Docker error handling implementation.

## Overview

These tests verify that the Docker client correctly identifies and categorizes real Docker daemon errors, transforming generic "Unknown error" messages into specific, actionable diagnostic information.

## Test Coverage

### Network Connectivity Error Detection
- Registry connectivity issues (ENOTFOUND)
- Registry connection refused (ECONNREFUSED)

### Registry Authentication Error Detection
- Authentication failures (401/403)
- Private registry access denial

### Image Not Found Error Detection
- Missing base images (404)
- Non-existent repositories

### Registry Server Error Detection
- Registry server errors (5xx)
- Service unavailable scenarios

### Dockerfile Syntax Error Detection
- Malformed Dockerfile syntax
- Missing FROM instruction

### Context Path Error Detection
- Invalid build context paths
- Non-existent directories

### Disk Space Error Detection
- Insufficient disk space handling
- Large image build failures

### Image Operations Error Detection
- Getting non-existent images
- Tagging non-existent images
- Pushing to unauthorized registries

### Build Progress Error Handling
- Errors in build progress stream
- Detailed error information from build steps

### Error Message Quality
- Elimination of generic "Unknown error" messages
- Actionable error message verification

## Prerequisites

- Docker daemon must be running
- Network connectivity for registry tests
- Sufficient disk space for image operations
- Node.js environment with Jest testing framework

## Running the Tests

### Individual Test Execution
```bash
# Run the Docker error handling integration tests
npm test -- test/integration/infrastructure/docker/client-error-handling.test.ts
```

### With Docker Daemon
```bash
# Ensure Docker daemon is running
docker info

# Run the integration tests
npm run test:integration -- --testPathPattern="docker/client-error-handling"
```

### Environment Variables

For comprehensive testing, you can set:

```bash
# Optional: Private registry for authentication tests
export TEST_PRIVATE_REGISTRY="your-private-registry.com"

# Run tests with registry authentication scenarios
npm test -- test/integration/infrastructure/docker/client-error-handling.test.ts
```

## Test Behavior

### Expected Outcomes

After running these tests, you should see:

1. **Network errors are clearly identified**: Instead of "Unknown error", you'll see "Network connectivity issue - getaddrinfo ENOTFOUND registry-1.docker.io"

2. **Authentication problems are explicit**: "Registry authentication issue - Access denied" instead of generic failures

3. **Missing images show clear 404 errors**: "Base image not found - Image does not exist" with specific image names

4. **Build failures provide context**: Detailed error messages from Docker build steps instead of generic build failures

### Test Duration

- Individual tests: 5-60 seconds each (depending on network conditions)
- Full suite: 5-10 minutes (with extended timeout for large image tests)
- Network timeout tests: May take longer depending on DNS resolution timeouts

## Test Design Philosophy

These integration tests are designed to:

1. **Test real Docker daemon interactions**: Unlike unit tests, these actually call Docker APIs
2. **Verify error message quality**: Ensure developers get actionable error information
3. **Cover common failure scenarios**: Network issues, authentication, missing images, etc.
4. **Maintain clean test environment**: Minimal resource usage and cleanup

## Troubleshooting

### Common Issues

1. **Docker daemon not running**:
   ```
   Error: connect ENOENT /var/run/docker.sock
   ```
   Solution: Start Docker daemon

2. **Network connectivity issues**:
   ```
   Error: getaddrinfo ENOTFOUND registry-1.docker.io
   ```
   Solution: Check internet connection

3. **Insufficient permissions**:
   ```
   Error: permission denied while trying to connect to Docker daemon
   ```
   Solution: Add user to docker group or run with sudo

4. **Disk space issues**:
   ```
   Error: no space left on device
   ```
   Solution: Clean up Docker images (`docker system prune`)

### Test Debugging

To debug failing tests:

1. **Enable debug logging**:
   ```bash
   LOG_LEVEL=debug npm test -- test/integration/infrastructure/docker/client-error-handling.test.ts
   ```

2. **Run individual test cases**:
   ```bash
   npm test -- test/integration/infrastructure/docker/client-error-handling.test.ts -t "should detect registry connectivity issues"
   ```

3. **Check Docker daemon logs**:
   ```bash
   # On macOS
   cat ~/Library/Containers/com.docker.docker/Data/log/vm/dockerd.log

   # On Linux
   journalctl -u docker.service
   ```

## Integration with CI/CD

These tests can be integrated into CI/CD pipelines with Docker-in-Docker (DinD) setup:

```yaml
# Example GitHub Actions workflow
test-docker-integration:
  runs-on: ubuntu-latest
  services:
    docker:
      image: docker:dind
  steps:
    - uses: actions/checkout@v2
    - run: npm ci
    - run: npm run test:integration -- --testPathPattern="docker"
```

## Maintenance

### Adding New Error Scenarios

When adding new Docker error detection:

1. Add test case to appropriate describe block
2. Use realistic Docker scenarios that trigger the error
3. Verify error message quality and actionability
4. Update this README with new test coverage

### Performance Considerations

- Tests use minimal Docker images (alpine) for speed
- Network tests use invalid domains to avoid external dependencies
- Large image tests include timeout extensions
- Cleanup is handled gracefully to avoid resource leaks

## Related Files

- `src/infrastructure/docker/client.ts` - Enhanced error handling implementation
- `test/unit/infrastructure/docker/client.test.ts` - Unit tests for structure validation
- `DOCKER_BUILD_ERROR_ANALYSIS.md` - Detailed analysis and recommendations
