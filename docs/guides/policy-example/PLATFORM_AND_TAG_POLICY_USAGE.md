# Platform and Tag Policy Usage Guide

## Overview

This policy enforces that all generated Dockerfiles:
1. **Use linux/arm64 platform** - All `FROM` statements must include `--platform=linux/arm64`
2. **Use "demo" tag** - All Dockerfiles must include a `LABEL tag="demo"` to indicate the intended image tag

## Policy Files

- **Policy**: `policies/platform-and-tag.rego`
- **Tests**: `policies/platform-and-tag_test.rego`

## How It Works

The policy validates Dockerfile content at multiple enforcement points:
- **generate-dockerfile** - Validates generated Dockerfile plans before returning
- **fix-dockerfile** - Validates actual Dockerfile content and provides fix recommendations

### Platform Detection

When `generate-dockerfile` runs, it automatically detects your system's architecture and uses it as the default platform:

| Your System | Auto-Detected Platform |
|-------------|------------------------|
| Intel/AMD (x64) | `linux/amd64` |
| Apple Silicon (M1/M2/M3) | `linux/arm64` |
| ARM 64-bit | `linux/arm64` |
| ARM 32-bit | `linux/arm/v7` |

**Example**: If you're on an x64 system but your policy requires `linux/arm64`, the policy will detect the mismatch and report a violation.

### Rule 1: Platform Enforcement (Priority: 100)

**What it checks**: All `FROM` statements must specify `--platform=linux/arm64`

**Severity**: Blocking violation

**Examples**:

✅ **Correct** - Platform specified:
```dockerfile
FROM --platform=linux/arm64 node:20-alpine
LABEL tag="demo"
CMD ["node", "app.js"]
```

❌ **Incorrect** - Missing platform:
```dockerfile
FROM node:20-alpine
LABEL tag="demo"
CMD ["node", "app.js"]
```

❌ **Incorrect** - Wrong platform:
```dockerfile
FROM --platform=linux/amd64 node:20-alpine
LABEL tag="demo"
CMD ["node", "app.js"]
```

### Rule 2: Tag Label Enforcement (Priority: 95)

**What it checks**: Dockerfile must include `LABEL tag="demo"` to indicate the intended image tag

**Severity**: Blocking violation

**Examples**:

✅ **Correct** - Demo tag label present:
```dockerfile
FROM --platform=linux/arm64 node:20-alpine
LABEL tag="demo"
CMD ["node", "app.js"]
```

❌ **Incorrect** - Missing tag label:
```dockerfile
FROM --platform=linux/arm64 node:20-alpine
CMD ["node", "app.js"]
```

❌ **Incorrect** - Wrong tag:
```dockerfile
FROM --platform=linux/arm64 node:20-alpine
LABEL tag="production"
CMD ["node", "app.js"]
```

## Multi-Stage Builds

For multi-stage builds, **all** `FROM` statements must include the platform specification:

✅ **Correct**:
```dockerfile
FROM --platform=linux/arm64 golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go build

FROM --platform=linux/arm64 alpine:latest
LABEL tag="demo"
COPY --from=builder /build/app /app
CMD ["/app"]
```

❌ **Incorrect** - Second stage missing platform:
```dockerfile
FROM --platform=linux/arm64 golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go build

FROM alpine:latest  # ❌ Missing platform!
LABEL tag="demo"
COPY --from=builder /build/app /app
CMD ["/app"]
```

## Activating the Policy

### Option 1: Add to Built-in Policies Directory

Place this policy file in the `policies/` directory and it will be automatically discovered and merged with other built-in policies.

### Option 2: Use Environment Variable

Set the environment variable to use your custom policy in addition to built-in policies:

```bash
export CONTAINERIZATION_ASSIST_POLICY_PATH=/path/to/policies/platform-and-tag.rego
```

**Note**: Custom policies specified via environment variable are **merged with** built-in policies, not replacing them. Both your custom policy and built-in policies will be enforced together.

## Testing the Policy

Run the test suite to verify the policy works correctly:

```bash
# Run all tests
opa test policies/platform-and-tag.rego policies/platform-and-tag_test.rego -v

# Run with coverage
opa test policies/platform-and-tag.rego policies/platform-and-tag_test.rego --coverage
```

### Test Results

All 11 tests pass:
- ✅ Block Dockerfile without platform specification
- ✅ Block Dockerfile with wrong platform (amd64)
- ✅ Block Dockerfile without demo tag label
- ✅ Block Dockerfile with wrong tag label
- ✅ Allow valid Dockerfile with platform and demo tag
- ✅ Allow Dockerfile with correct format variations
- ✅ Block multi-stage Dockerfile with one stage missing platform
- ✅ Allow multi-stage Dockerfile with all platforms correct
- ✅ Verify suggestion appears for correct usage
- ✅ Verify specific violation rules trigger correctly

## Important Notes

### About Image Tags

**Image tags are applied at build time, not in the Dockerfile itself.** The `LABEL tag="demo"` approach is used because:

1. Dockerfiles don't contain image tag specifications
2. Tags are applied when running `docker build -t myimage:demo`
3. The LABEL serves as documentation and policy enforcement

When using the `build-image` tool, make sure to specify the tag:

```bash
# The build-image tool should tag the image as "demo"
# Policy ensures the Dockerfile has LABEL tag="demo" for consistency
```

### About Platform Specification

The `--platform=linux/arm64` flag in FROM statements ensures:
- Docker pulls arm64-compatible base images
- Builds are architecture-specific
- Images can run on ARM64 systems (Apple Silicon, AWS Graviton, etc.)

## Violation Messages

When the policy detects violations, you'll see clear error messages:

**Platform violation**:
```
Rule: require-arm64-platform
Severity: block
Message: All FROM statements must specify --platform=linux/arm64.
         Example: FROM --platform=linux/arm64 node:20-alpine
```

**Tag violation**:
```
Rule: require-demo-tag-label
Severity: block
Message: Dockerfile must include LABEL with tag=demo. Add: LABEL tag="demo"
```

## Troubleshooting

### Policy not enforcing

1. Verify the policy file is in the `policies/` directory
2. Check that policy syntax is valid: `opa check policies/platform-and-tag.rego`
3. Ensure you're using the latest version of containerization-assist

### Tests failing

Run tests with verbose output to see details:
```bash
opa test -v policies/platform-and-tag.rego policies/platform-and-tag_test.rego
```

### Regex not matching

Test the policy manually:
```bash
# Create a test input
echo 'FROM node:20-alpine
LABEL tag="demo"' > test-dockerfile.txt

# Test evaluation
opa eval -d policies/platform-and-tag.rego -i test-dockerfile.txt "data.containerization.platform.result"
```

## Customization

You can modify the policy to:

1. **Change the required tag**: Edit line with `tag\s*=\s*["']?demo["']?` to use a different tag
2. **Change the platform**: Edit `--platform=linux/arm64` to require different architectures
3. **Add additional rules**: Follow the pattern in the policy file to add more validations

## Additional Resources

- [OPA Documentation](https://www.openpolicyagent.org/docs/latest/)
- [Rego Language Reference](https://www.openpolicyagent.org/docs/latest/policy-language/)
- [Writing Rego Policies Guide](../docs/guides/writing-rego-policies.md)
