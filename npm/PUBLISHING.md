# Publishing Container Kit MCP Server to NPM

## Prerequisites

1. **NPM Account**: Create an account at [npmjs.com](https://www.npmjs.com)
2. **NPM Token**: Generate an access token with publish permissions
3. **GitHub Secret**: Add `NPM_TOKEN` to repository secrets

## First-Time Setup

### 1. Login to NPM locally
```bash
npm login
# Enter your username, password, and email
```

### 2. Add NPM Token to GitHub
- Go to: https://github.com/Azure/container-kit/settings/secrets/actions
- Add new secret: `NPM_TOKEN`
- Value: Your NPM access token

### 3. Verify Package Name Availability
```bash
npm view @container-kit/mcp-server
# Should return "npm ERR! 404" if available
```

## Publishing Process

### Option 1: Automated Release (Recommended)

1. **Create a GitHub Release**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
   Then create release on GitHub UI

2. **GitHub Actions will automatically**:
   - Build binaries for all platforms
   - Update package version
   - Publish to NPM
   - Test installation on multiple platforms

### Option 2: Manual Workflow Trigger

1. Go to: Actions → "Publish to NPM" workflow
2. Click "Run workflow"
3. Enter version (e.g., "1.0.1")
4. Optionally check "Dry run" for testing

### Option 3: Local Publishing

```bash
cd npm/

# 1. Build all binaries
npm run build

# 2. Update version
npm version patch  # or minor/major

# 3. Test locally
npm test
npm pack
npm install -g container-assist-mcp-server-*.tgz
container-assist-mcp --version

# 4. Publish
npm publish --access public
```

## Version Management

### Semantic Versioning
- **Patch** (1.0.x): Bug fixes, minor updates
- **Minor** (1.x.0): New features, backward compatible
- **Major** (x.0.0): Breaking changes

### Version Sources (Priority Order)
1. Git tags (`v1.0.0`)
2. Go source code (`main.go`)
3. package.json (fallback)

### Sync Version
```bash
# Automatically sync from Git/Go
npm run version

# Or manually set
node scripts/sync-version.js 1.0.5
```

## Testing Before Release

### Local Testing
```bash
# Build and test
cd npm/
npm run build:current
npm test

# Pack and install locally
npm pack
npm install -g container-assist-mcp-server-*.tgz
container-assist-mcp --version
```

### Dry Run
```bash
# See what would be published
npm publish --dry-run
```

## Post-Publishing Verification

### Check NPM Registry
```bash
# View published package
npm view @container-assist/mcp-server

# Test installation
npx @container-assist/mcp-server --version
```

### Monitor GitHub Actions
- Check workflow status
- Review test results across platforms
- Verify all platform binaries work

## Troubleshooting

### Common Issues

1. **"Permission denied" during publish**
   - Verify NPM_TOKEN is set correctly
   - Check npm login status: `npm whoami`

2. **"Package name unavailable"**
   - The scoped name might be taken
   - Consider alternative: `@azure/container-assist-mcp`

3. **Binary missing for platform**
   - Run `npm run build` to build all platforms
   - Check Go cross-compilation support

4. **Version mismatch**
   - Run `npm run version` to sync
   - Ensure Git tags match package.json

### Emergency Unpublish
```bash
# Within 24 hours only
npm unpublish @container-assist/mcp-server@1.0.0

# Deprecate instead (recommended)
npm deprecate @container-assist/mcp-server@1.0.0 "Critical bug, use 1.0.1"
```

## Maintenance

### Update Dependencies
```bash
# Check for updates
npm outdated

# Update package.json
npm update
```

### Platform Support
When adding new platform:
1. Update `build-all.sh` script
2. Add to platform matrix in workflow
3. Update README platform table
4. Test installation on new platform

### Security Releases
For security updates:
1. Fix vulnerability in Go code
2. Bump patch version
3. Add security notice to release notes
4. Consider npm security advisory

## Monitoring

### Download Stats
- View at: https://www.npmjs.com/package/@container-assist/mcp-server
- Or use: `npm-stat` tool

### Issue Tracking
- Monitor GitHub issues for npm-specific problems
- Tag with `npm-package` label

---

**Questions?** Open an issue or contact the maintainers.