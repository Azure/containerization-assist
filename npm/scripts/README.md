# NPM Scripts

## Publishing Workflow

### One Command to Publish Everything
```bash
npm run publish:packages
```

This single command handles the entire publishing workflow:
1. Builds optimized binaries for all platforms
2. Creates platform-specific npm packages
3. Syncs versions across all packages
4. Publishes platform packages to npm
5. Publishes main package to npm
6. Verifies publication success

## Individual Scripts

### build.sh
Builds optimized binaries for all platforms with UPX compression where possible.
- Output: `bin/{platform}/containerization-assist-mcp`
- Platforms: darwin-x64, darwin-arm64, linux-x64, linux-arm64, win32-x64, win32-arm64

### create-platform-packages.sh
Creates separate npm packages for each platform.
- Output: `platform-packages/{platform}/`
- Each package contains only its platform's binary

### postinstall.js
Runs after npm install to:
- Detect user's platform
- Find the platform-specific binary
- Create symlink in main package's bin directory

### publish.sh
Complete publishing workflow with all steps automated.
- Checks npm authentication
- Builds, packages, and publishes everything
- Provides detailed progress and verification

### sync-versions.js
Ensures version numbers are synchronized:
- Updates optionalDependencies in package.json
- Runs automatically on `npm version`

### test.js
Tests the MCP server functionality.

## Usage Examples

```bash
# Build binaries only
npm run build

# Run tests
npm test

# Bump version (auto-syncs dependencies)
npm version patch

# Publish everything
npm run publish:packages
```

## Package Sizes
- Main package: ~25KB (no binaries)
- Linux: ~3MB (UPX compressed)
- Windows: 3-10MB (UPX compressed where possible)
- macOS: ~11MB (no UPX due to code signing)