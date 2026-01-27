# OSV Scanner Integration Test with Real Network Calls

## Overview

This integration test (`osv-scanner-network.test.ts`) validates the OSV (Open Source Vulnerabilities) scanner by making **real network calls** to the OSV API (https://api.osv.dev).

Unlike unit tests that mock API responses, this test:
- Builds actual Docker images with vulnerable dependencies
- Makes real HTTP requests to the OSV API
- Verifies detection of known CVEs in Maven dependencies

## Test Fixture

### Location
- `test/__support__/fixtures/vulnerable-pom/pom.xml`

### Contents
The pom.xml contains dependencies with **known, documented vulnerabilities**:

| Dependency | Version | Known CVEs |
|------------|---------|------------|
| log4j-core | 2.14.1 | CVE-2021-44228 (Log4Shell) |
| spring-core | 5.2.0.RELEASE | Multiple CVEs |
| jackson-databind | 2.9.8 | Multiple CVEs |
| commons-collections | 3.2.1 | CVE-2015-6420 |
| netty-all | 4.1.42.Final | Multiple CVEs |

## Test Structure

### Test Scenarios

1. **API Availability**
   - Verifies OSV API is reachable
   - Checks network connectivity

2. **Individual CVE Detection**
   - Log4Shell (CVE-2021-44228) in log4j-core 2.14.1
   - Jackson Databind vulnerabilities in 2.9.8
   - Commons Collections vulnerabilities in 3.2.1

3. **Multiple Dependencies**
   - Tests scanning images with multiple vulnerable dependencies
   - Validates vulnerability aggregation across packages

4. **Clean Dependencies**
   - Tests scanning recent, patched versions
   - Ensures no false positives

5. **Error Handling**
   - Empty pom.xml (no dependencies)
   - Malformed version numbers

### Test Flow

For each test:
1. Create temporary directory
2. Write pom.xml with specific dependencies
3. Create Dockerfile that copies pom.xml
4. Build Docker image
5. Run OSV scanner on the image
6. Verify expected vulnerabilities are detected
7. Cleanup Docker images and temp files

## Prerequisites

### Required
- **Network connectivity** - Tests make real HTTP requests to api.osv.dev
- **Docker daemon running** - Tests build real images
- **OSV API availability** - Tests will skip if API is down

### Optional
- For faster tests, pull base image first:
  ```bash
  docker pull maven:3.9-eclipse-temurin-11-alpine
  ```

## Running the Tests

### Run All Integration Tests
```bash
npm test -- test/integration
```

### Run Only OSV Integration Tests
```bash
npm test -- test/integration/osv-scanner-network.test.ts
```

### Run with Verbose Output
```bash
npm test -- test/integration/osv-scanner-network.test.ts --verbose
```

## Expected Behavior

### When API is Available
- All tests should pass
- Log4Shell should be detected with CRITICAL/HIGH severity
- Multiple vulnerabilities should be found in old dependencies
- Clean dependencies should have minimal/no vulnerabilities

### When API is Unavailable
- Tests will skip gracefully with warning:
  ```
  OSV API not available, skipping integration tests
  Reason: <error message>
  ```

### When Docker is Unavailable
- Tests will skip gracefully with warning:
  ```
  Docker not available, skipping integration tests
  ```

## Test Timeouts

- API availability check: 10 seconds
- Single dependency scan: 60 seconds (build + scan)
- Multiple dependencies scan: 90 seconds (build + multiple API calls)

## Known Limitations

1. **Network Dependency**
   - Tests require stable internet connection
   - May fail on slow/unstable networks

2. **API Rate Limits**
   - OSV API rate limit: 10 requests/second
   - Tests include rate limiting to stay within limits

3. **Vulnerability Data Changes**
   - New CVEs may be discovered over time
   - Old CVEs may be reclassified
   - Tests use >= assertions where appropriate

4. **Docker Build Time**
   - First run downloads base image (~200MB)
   - Subsequent runs use cached layers

## Troubleshooting

### Test Failures

**"OSV API not available"**
- Check network connectivity
- Verify api.osv.dev is reachable: `curl https://api.osv.dev/v1/query`
- Check for proxy/firewall issues

**"Docker not available"**
- Verify Docker daemon is running: `docker ps`
- Check Docker socket permissions

**"Timeout exceeded"**
- Slow network or Docker build
- Increase timeout in test file
- Pull base image manually first

**"Expected vulnerabilities not found"**
- OSV database may have changed
- Vulnerability may have been fixed/reclassified
- Check OSV API directly: https://osv.dev/vulnerability/{CVE-ID}

### Debugging

Enable detailed logging:
```typescript
// In beforeAll
logger = createLogger({ level: 'debug' }); // Change from 'warn'
```

Check OSV API directly:
```bash
curl -X POST https://api.osv.dev/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "package": {"name": "org.apache.logging.log4j:log4j-core", "ecosystem": "Maven"},
    "version": "2.14.1"
  }'
```

## Maintenance

### Updating Test Dependencies

When updating vulnerable dependencies:
1. Verify CVEs still exist in OSV database
2. Update expected vulnerability counts
3. Document changes in CHANGELOG

### Adding New Test Cases

1. Find package with known CVEs in OSV
2. Add pom.xml with specific version
3. Add test case with expected CVE ID
4. Verify test passes with real API

## Related Files

- `src/infra/security/osv-scanner/index.ts` - OSV scanner implementation
- `src/infra/security/osv-scanner/osv-api.ts` - OSV API client
- `src/infra/security/osv-scanner/maven/pom-parser.ts` - Maven pom.xml parser
- `test/unit/infra/security/osv-scanner.test.ts` - Unit tests (mocked)

## References

- [OSV.dev](https://osv.dev) - Open Source Vulnerabilities database
- [OSV API Docs](https://google.github.io/osv.dev/api/) - API documentation
- [CVE-2021-44228](https://nvd.nist.gov/vuln/detail/CVE-2021-44228) - Log4Shell details
