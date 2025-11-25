# Packaging Tests

Tests for validating the package structure and build artifacts. These tests ensure the package can be built and installed correctly.

## Overview

The `test/packaging/` directory contains Jest-based tests for validating build artifacts and package integrity:

- `build-validation.test.ts` - Validates ESM and CJS build outputs
- `cli-functionality.test.ts` - Tests CLI functionality
- `client-api.test.ts` - Tests public API exports
- `package-integrity.test.ts` - Validates package.json and npm packaging

## Packed Application Testing

**Important:** Tests for the **packed/deployed version** (via `npm pack` and `npm install -g`) are located in:

### `scripts/test-spring-petclinic-mcp.mjs`
Tests the basic packed CLI functionality using stdio JSON-RPC protocol.

**What it tests:**
- `npm pack` and `npm install -g` workflow
- `ca-mcp` command availability
- Basic tool execution (analyze-repo, generate-dockerfile)
- Knowledge pack recommendations (verifies azurelinux images appear)

**Usage:**
```bash
# From the Spring PetClinic directory
node /path/to/test-spring-petclinic-mcp.mjs $(pwd) .
```

**CI Workflow:** `.github/workflows/test-packed-spring-petclinic.yml`

### `scripts/test-spring-petclinic-policy-filtering.mjs`
Tests that policy filtering works in the packed/deployed version.

**What it tests:**
- Policy loading via `CONTAINERIZATION_ASSIST_POLICY_PATH` environment variable
- Knowledge pack filtering based on policy constraints
- Only policy-compliant images are recommended
- Non-compliant images are filtered out

**Usage:**
```bash
# From the Spring PetClinic directory
node /path/to/test-spring-petclinic-policy-filtering.mjs $(pwd) /path/to/base-images.rego
```

**CI Workflow:** `.github/workflows/test-packed-policy-filtering.yml`

## Why Two Different Test Approaches?

### Jest Tests (`test/packaging/*.test.ts`)
- Test build artifacts and package structure
- Fast, run as part of regular test suite
- Don't require global installation
- Use `node dist/src/cli/cli.js` directly

### Script-based Tests (`scripts/test-*.mjs`)
- Test the **actual deployed version** (via `npm pack` + `npm install -g`)
- Use the installed `ca-mcp` command (not direct node execution)
- Catch issues with:
  - Binary command setup
  - Module resolution in installed packages
  - Policy path resolution in deployed environment
  - Global installation and package exports

**The script-based tests are critical because they test what users actually install!**

## Running the Tests

### Packaging Tests (Jest)
```bash
npm run build
npm test -- test/packaging
```

### Packed Application Tests (Scripts)
These run in GitHub Actions via the workflows. To run locally:

```bash
# Build and pack
npm run build
npm pack
PACKED_FILE=$(ls containerization-assist-mcp-*.tgz)

# Install globally
npm install -g $PACKED_FILE

# Clone Spring PetClinic
git clone --depth 1 https://github.com/spring-projects/spring-petclinic.git
cd spring-petclinic

# Run basic test
node ../containerization-assist/scripts/test-spring-petclinic-mcp.mjs $(pwd) .

# Run policy filtering test
node ../containerization-assist/scripts/test-spring-petclinic-policy-filtering.mjs \
  $(pwd) \
  ../containerization-assist/policies/base-images.rego

# Cleanup
npm uninstall -g containerization-assist-mcp
cd ..
rm -rf spring-petclinic
```

## CI/CD Integration

### GitHub Actions Workflows

1. **`.github/workflows/test-packed-spring-petclinic.yml`**
   - Tests basic packed CLI functionality
   - Runs on push/PR to main/develop
   - Validates azurelinux images appear in recommendations

2. **`.github/workflows/test-packed-policy-filtering.yml`**
   - Tests policy filtering in packed version
   - Runs on push/PR to main/develop
   - Validates only Microsoft images are recommended when base-images.rego is used

Both workflows:
1. Build the project
2. Run `npm pack` to create tarball
3. Install globally with `npm install -g`
4. Clone Spring PetClinic
5. Run test script using `ca-mcp` command
6. Verify expected behavior

## What Makes These Tests Valid?

The script-based tests are the **only way** to catch issues with the deployed version because:

1. **They use `ca-mcp` command** - The actual installed binary, not direct node execution
2. **They test `npm pack` → `npm install -g` flow** - The exact deployment process
3. **They run in a clean environment** - Separate from the development source
4. **They test policy path resolution** - As it works in installed packages
5. **They validate module exports** - As defined in package.json `bin` field

A test that uses `node dist/src/cli/cli.js` would **not catch**:
- Issues with the `bin` field in package.json
- Problems with the `files` field (missing files in package)
- Module resolution issues in installed packages
- Policy path resolution in deployed environment
- Global command availability

## Common Issues

### "ca-mcp: command not found"

The package isn't installed globally. Run:
```bash
npm install -g ./containerization-assist-mcp-*.tgz
```

### "Policy file not found"

Ensure the policy path is absolute or relative to the working directory:
```bash
node test-policy-filtering.mjs $(pwd) $(pwd)/../policies/base-images.rego
```

### "No Microsoft images in output"

This indicates policy filtering is NOT working. Possible causes:
1. Policy not loaded (check CONTAINERIZATION_ASSIST_POLICY_PATH)
2. Knowledge pack matcher not respecting policy
3. Policy file not included in packaged version (check package.json `files` field)

## Adding New Packed Application Tests

To add a new packed application test:

1. Create a new test script in `scripts/test-*.mjs`
2. Use `ca-mcp start` to spawn the server (not `node dist/...`)
3. Set required environment variables (e.g., `CONTAINERIZATION_ASSIST_POLICY_PATH`)
4. Communicate via stdio JSON-RPC protocol
5. Create a corresponding GitHub Actions workflow in `.github/workflows/`

Example test structure:
```javascript
// spawn ca-mcp with environment
const server = spawn('ca-mcp', ['start'], {
  stdio: ['pipe', 'pipe', 'pipe'],
  env: {
    ...process.env,
    MCP_QUIET: 'true',
    CONTAINERIZATION_ASSIST_POLICY_PATH: policyPath
  }
});

// Send JSON-RPC request
server.stdin.write(JSON.stringify({
  jsonrpc: '2.0',
  id: 1,
  method: 'tools/call',
  params: { name: 'tool-name', arguments: {...} }
}) + '\n');

// Parse JSON-RPC response from stdout
server.stdout.on('data', (data) => {
  const response = JSON.parse(data);
  // validate response
});
```
