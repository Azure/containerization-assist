# Publishing Optimized Packages

## Package Size Reduction Summary
- **Before**: 200MB+ (all platforms bundled)
- **After**: 3-11MB per platform (85-95% reduction)
  - Linux: 3MB (with UPX compression)
  - Windows: 3-10MB 
  - macOS: 11MB (UPX not used due to code signing)

## Publishing Steps

### 1. Verify Version
```bash
# Check current version in package.json
grep version package.json
```

### 2. Build Optimized Binaries
```bash
# Build all platforms with optimization
npm run build
```

### 3. Create Platform Packages
```bash
# Generate platform-specific npm packages
npm run build:packages
```

### 4. Publish Everything
```bash
# This script publishes platform packages first, then main package
npm run publish:all
```

## Manual Publishing (if needed)

### Publish Platform Packages
```bash
cd platform-packages
for dir in */; do
  cd "$dir"
  npm publish --access public
  cd ..
done
```

### Publish Main Package
```bash
cd /home/tng/workspace/containerization-assist/npm
npm publish --access public
```

## Package Structure

### Main Package
- `@thgamble/containerization-assist-mcp` (22KB)
  - Contains JS files, scripts, no binaries
  - Has optionalDependencies for platform packages
  - Postinstall script links platform binary

### Platform Packages
- `@thgamble/containerization-assist-mcp-darwin-x64` (11MB)
- `@thgamble/containerization-assist-mcp-darwin-arm64` (11MB)  
- `@thgamble/containerization-assist-mcp-linux-x64` (3MB)
- `@thgamble/containerization-assist-mcp-linux-arm64` (3MB)
- `@thgamble/containerization-assist-mcp-win32-x64` (3MB)
- `@thgamble/containerization-assist-mcp-win32-arm64` (10MB)

## User Installation
Users continue to install normally:
```bash
npm install @thgamble/containerization-assist-mcp
```

NPM automatically:
1. Installs the main package
2. Detects the user's platform
3. Downloads only the matching platform package
4. Postinstall script creates the binary symlink

## Verification
After publishing, test on different platforms:
```bash
# Create test directory
mkdir test-install && cd test-install
npm init -y

# Install package
npm install @thgamble/containerization-assist-mcp

# Test binary
npx containerization-assist-mcp --version
npx ckmcp --version
```

## Troubleshooting

### If postinstall fails
- Check that platform package was published
- Verify optionalDependencies versions match
- Ensure postinstall-slim.js is included in package

### If binary not found
- Check node_modules/@thgamble/containerization-assist-mcp-{platform}/
- Verify binary exists in bin/{platform}/ subdirectory
- Check symlink creation in postinstall output