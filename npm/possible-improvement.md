# NPM Package Improvement Plan: GitHub Release Binary Downloads

## Current Architecture
The npm package currently includes pre-built binaries for all platforms:
- Package size: ~150-200MB (all platforms)
- User downloads: All binaries but uses only one (~35MB)
- Build process: Happens during npm publish
- Installation: Fast but downloads unnecessary data

## Proposed Improvement: Download from GitHub Releases

### Overview
Instead of bundling all binaries in the npm package, download only the required binary from GitHub releases during post-install.

### Benefits
- **Package size reduction**: From ~200MB to ~100KB (99% reduction)
- **Faster npm installs**: Download only what's needed
- **Reduced registry storage**: Lower costs and bandwidth
- **Version flexibility**: Can update binaries without npm republish
- **CDN benefits**: GitHub's CDN for binary distribution

### Implementation Plan

#### Phase 1: Modify Package Structure
```
npm/
├── package.json          # No binaries in "files"
├── index.js             # Unchanged
├── scripts/
│   ├── postinstall.js   # Modified to download from GitHub
│   └── download.js      # New: Binary download logic
└── bin/                 # Created during postinstall
    └── .gitkeep
```

#### Phase 2: Create Download Script (`scripts/download.js`)
```javascript
const https = require('https');
const fs = require('fs');
const path = require('path');
const { pipeline } = require('stream/promises');

class BinaryDownloader {
  constructor() {
    this.baseUrl = 'https://github.com/Azure/container-kit/releases/download';
    this.version = this.getVersion();
  }

  getVersion() {
    // Get version from package.json
    const pkg = require('../package.json');
    return pkg.version;
  }

  getBinaryName(platform, arch) {
    const platformMap = {
      'darwin': 'darwin',
      'linux': 'linux',
      'win32': 'win'
    };
    
    const archMap = {
      'x64': 'x64',
      'arm64': 'arm64'
    };

    const plat = platformMap[platform];
    const arc = archMap[arch];
    
    let name = `mcp-server-${plat}-${arc}`;
    if (platform === 'win32') name += '.exe';
    
    return name;
  }

  async downloadBinary(platform, arch) {
    const binaryName = this.getBinaryName(platform, arch);
    const url = `${this.baseUrl}/v${this.version}/${binaryName}`;
    const destPath = path.join(__dirname, '..', 'bin', binaryName);
    
    console.log(`Downloading ${binaryName} from GitHub releases...`);
    
    // Ensure bin directory exists
    fs.mkdirSync(path.dirname(destPath), { recursive: true });
    
    return new Promise((resolve, reject) => {
      https.get(url, { 
        headers: { 'User-Agent': 'container-kit-npm' },
        timeout: 30000 
      }, (response) => {
        // Handle redirects
        if (response.statusCode === 302 || response.statusCode === 301) {
          https.get(response.headers.location, async (redirectResponse) => {
            if (redirectResponse.statusCode === 200) {
              await pipeline(redirectResponse, fs.createWriteStream(destPath));
              fs.chmodSync(destPath, 0o755);
              resolve(destPath);
            } else {
              reject(new Error(`Failed to download: ${redirectResponse.statusCode}`));
            }
          });
        } else if (response.statusCode === 200) {
          pipeline(response, fs.createWriteStream(destPath))
            .then(() => {
              fs.chmodSync(destPath, 0o755);
              resolve(destPath);
            })
            .catch(reject);
        } else {
          reject(new Error(`Failed to download: ${response.statusCode}`));
        }
      }).on('error', reject);
    });
  }

  async downloadWithRetry(platform, arch, retries = 3) {
    for (let i = 0; i < retries; i++) {
      try {
        return await this.downloadBinary(platform, arch);
      } catch (err) {
        console.log(`Download attempt ${i + 1} failed: ${err.message}`);
        if (i === retries - 1) throw err;
        await new Promise(r => setTimeout(r, 1000 * (i + 1)));
      }
    }
  }
}

module.exports = BinaryDownloader;
```

#### Phase 3: Update postinstall.js
```javascript
#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const BinaryDownloader = require('./download');

async function postinstall() {
  const { platform, arch } = process;
  
  // Check if binary already exists (for offline/cached installs)
  const binDir = path.join(__dirname, '..', 'bin');
  const binaryName = getBinaryName(platform, arch);
  const binaryPath = path.join(binDir, binaryName);
  
  if (!fs.existsSync(binaryPath)) {
    // Download from GitHub
    const downloader = new BinaryDownloader();
    
    try {
      await downloader.downloadWithRetry(platform, arch);
      console.log('✅ Binary downloaded successfully from GitHub releases');
    } catch (err) {
      console.error('❌ Failed to download binary from GitHub');
      console.error('Please try:');
      console.error('1. Check your internet connection');
      console.error('2. Manually download from: https://github.com/Azure/container-kit/releases');
      console.error('3. Or build from source: npm run build:current');
      process.exit(1);
    }
  }
  
  // Create symlink as before
  createSymlink(binaryPath);
}

postinstall().catch(console.error);
```

#### Phase 4: Fallback Strategy
Include a minimal fallback binary or provide clear instructions:

```javascript
// In download.js
async downloadWithFallback(platform, arch) {
  try {
    // Try GitHub first
    return await this.downloadWithRetry(platform, arch);
  } catch (err) {
    // Try npm CDN mirror (if we upload there)
    try {
      return await this.downloadFromNpmCdn(platform, arch);
    } catch (err2) {
      // Try direct S3/Azure blob storage
      try {
        return await this.downloadFromBackupCdn(platform, arch);
      } catch (err3) {
        throw new Error('All download sources failed');
      }
    }
  }
}
```

#### Phase 5: Update package.json
```json
{
  "files": [
    "index.js",
    "scripts/",
    "README.md",
    "LICENSE"
    // Note: No bin/ directory
  ],
  "scripts": {
    "postinstall": "node scripts/postinstall.js",
    "download-binary": "node scripts/download.js"
  }
}
```

### Additional Improvements

#### 1. Offline Support
```javascript
// Check for CONTAINER_KIT_OFFLINE env var
if (process.env.CONTAINER_KIT_OFFLINE) {
  console.log('Offline mode: Skipping binary download');
  console.log('Please ensure binary is available in bin/ directory');
  return;
}
```

#### 2. Custom Binary Location
```javascript
// Allow custom binary path
const customBinary = process.env.CONTAINER_KIT_BINARY_PATH;
if (customBinary && fs.existsSync(customBinary)) {
  console.log(`Using custom binary: ${customBinary}`);
  fs.symlinkSync(customBinary, defaultBinaryPath);
  return;
}
```

#### 3. Progress Indicator
```javascript
// Show download progress
const ProgressBar = require('progress');

https.get(url, (response) => {
  const len = parseInt(response.headers['content-length'], 10);
  const bar = new ProgressBar('Downloading [:bar] :percent :etas', {
    complete: '=',
    incomplete: ' ',
    width: 40,
    total: len
  });
  
  response.on('data', chunk => bar.tick(chunk.length));
});
```

#### 4. Checksum Verification
```javascript
// Verify binary integrity
const crypto = require('crypto');

async function verifyChecksum(filePath, expectedHash) {
  const hash = crypto.createHash('sha256');
  const stream = fs.createReadStream(filePath);
  
  for await (const chunk of stream) {
    hash.update(chunk);
  }
  
  const fileHash = hash.digest('hex');
  return fileHash === expectedHash;
}

// Download checksums from release
const checksumsUrl = `${baseUrl}/v${version}/checksums.txt`;
const checksums = await downloadChecksums(checksumsUrl);
const expectedHash = checksums[binaryName];

if (!await verifyChecksum(binaryPath, expectedHash)) {
  throw new Error('Checksum verification failed');
}
```

#### 5. Version Mismatch Handling
```javascript
// Allow version override for testing
const targetVersion = process.env.CONTAINER_KIT_VERSION || packageVersion;

// Warn on mismatch
if (targetVersion !== packageVersion) {
  console.warn(`Warning: Downloading v${targetVersion} instead of v${packageVersion}`);
}
```

### CI/CD Changes

#### Update GitHub Release Workflow
```yaml
- name: Create Release with Binaries
  uses: softprops/action-gh-release@v1
  with:
    files: |
      npm/bin/mcp-server-*
      checksums.txt
    tag_name: v${{ steps.version.outputs.version }}
    
- name: Publish Lightweight NPM Package
  run: |
    # Remove binaries before publish
    rm -rf npm/bin/mcp-server-*
    npm publish --access public
```

### Migration Strategy

#### Phase 1: Dual Support (v2.0.0)
- Keep current approach
- Add download capability as opt-in via env var
- `CONTAINER_KIT_DOWNLOAD_BINARY=true npm install`

#### Phase 2: Default to Download (v3.0.0)
- Download from GitHub by default
- Fall back to bundled binary if download fails
- Include only current platform binary as fallback

#### Phase 3: Full Migration (v4.0.0)
- Remove bundled binaries entirely
- Pure download-based installation
- Multiple CDN fallbacks

### Performance Metrics

#### Before (Current)
- NPM package size: ~200MB
- Install time: 10-30s (depends on connection)
- Registry storage: High
- User disk space: ~200MB

#### After (Proposed)
- NPM package size: ~100KB
- Install time: 5-15s (only needed binary)
- Registry storage: Minimal
- User disk space: ~35MB

### Risk Mitigation

1. **GitHub Outage**: Multiple CDN fallbacks
2. **Rate Limiting**: Use authenticated requests with GitHub token
3. **Corporate Firewalls**: Provide manual download instructions
4. **Offline Environments**: Document offline installation process
5. **Version Mismatches**: Strict version checking with clear errors

### Testing Plan

1. **Unit Tests**: Test download logic with mocked HTTP
2. **Integration Tests**: Test actual downloads in CI
3. **Platform Matrix**: Test on all supported platforms
4. **Network Conditions**: Test with slow/interrupted connections
5. **Fallback Testing**: Simulate primary source failures

### Success Criteria

- ✅ 99% reduction in npm package size
- ✅ Successful installation rate > 99%
- ✅ Install time reduced by 50%
- ✅ Works behind corporate proxies
- ✅ Clear error messages and recovery steps


---

This improvement would significantly enhance the user experience while reducing infrastructure costs and installation times.