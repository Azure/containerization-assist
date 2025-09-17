# Cross-Platform Compatibility Guide

This document outlines the cross-platform compatibility improvements made to the Container Copilot codebase to ensure it works seamlessly on Windows, macOS, and Linux systems.

## Overview

The Container Copilot project has been updated with comprehensive cross-platform support to address Windows-specific path and command execution issues that could cause failures when running on different operating systems.

## Key Issues Addressed

### 1. Path Handling Issues

**Problem**: Hard-coded Unix paths and inconsistent path separators caused failures on Windows.

**Examples of Fixed Issues**:
- Hard-coded `/tmp` directory (doesn't exist on Windows)
- Unix socket paths like `/var/run/docker.sock` (Windows uses named pipes)
- Shell-dependent path operations using `&&` and `/bin/sh`
- Inconsistent use of forward vs backward slashes

**Solution**: Created cross-platform utilities that automatically detect the operating system and use appropriate paths.

### 2. Command Execution Issues

**Problem**: Shell commands were written for Unix systems only.

**Examples of Fixed Issues**:
- Using `/bin/sh` directly (not available on Windows)
- Shell-specific commands like `chmod +x` (Windows uses file extensions for executability)
- Process spawning without proper shell handling

**Solution**: Implemented cross-platform command execution that uses appropriate shells (cmd.exe on Windows, /bin/sh on Unix).

### 3. Configuration Path Issues

**Problem**: Configuration files used hard-coded Unix paths.

**Examples of Fixed Issues**:
- Docker socket configuration defaulting to `/var/run/docker.sock`
- Kubernetes config defaulting to `~/.kube/config` without proper expansion
- Temporary directory configuration hard-coded to `/tmp`

**Solution**: Configuration system now uses cross-platform utilities to determine appropriate paths.

## Implementation Details

### Cross-Platform Utilities

#### System Detection (`src/lib/system-utils.ts`)

```typescript
export interface SystemInfo {
  platform: NodeJS.Platform;
  arch: string;
  release: string;
  homedir: string;
  isWindows: boolean;
}

// Automatically detects the current operating system
export function getSystemInfo(): SystemInfo
```

Key functions:
- `getSystemInfo()` - Detects current OS and architecture
- `getDownloadOS()` - Returns OS name suitable for binary downloads
- `getDownloadArch()` - Returns architecture name suitable for binary downloads
- `commandExists(command)` - Checks if a command is available in PATH

#### Path Utilities (`src/lib/path-utils.ts`)

```typescript
// Cross-platform path utilities
export function getTempDir(): string
export function getDockerSocketPath(): string  
export function getKubeConfigPath(): string
export function resolvePath(inputPath: string): string
```

Key features:
- **`getTempDir()`**: Returns appropriate temp directory (`%TEMP%` on Windows, `/tmp` on Unix)
- **`getDockerSocketPath()`**: Returns correct Docker socket path (named pipe on Windows, Unix socket on Linux/macOS)
- **`getKubeConfigPath()`**: Returns properly expanded kubeconfig path
- **`resolvePath()`**: Resolves `~`, relative paths, and environment variables cross-platform

#### Command Execution (`src/lib/command-utils.ts`)

```typescript
export interface CommandResult {
  stdout: string;
  stderr: string;
  exitCode: number;
  success: boolean;
}

// Cross-platform command execution
export async function executeCommand(command: string, options?: CommandOptions): Promise<CommandResult>
export async function executeSafeCommand(command: string, options?: CommandOptions): Promise<CommandResult>
```

Key features:
- **Shell Detection**: Automatically uses `cmd.exe` on Windows, `/bin/sh` on Unix
- **Error Handling**: Consistent error handling across platforms
- **Retry Logic**: Built-in retry mechanism for flaky operations
- **Timeout Support**: Configurable timeouts for long-running commands

### File Operations

Cross-platform file operations that handle Windows vs Unix differences:

```typescript
// Cross-platform file operations
export async function createTempFile(content: string, extension?: string): Promise<string>
export async function deleteTempFile(filePath: string): Promise<void>
export async function ensureDirectory(dirPath: string): Promise<void>
export async function fileExists(filePath: string): Promise<boolean>
export async function downloadFile(url: string, destination: string): Promise<void>
export async function makeExecutable(filePath: string): Promise<void>
```

## Updated Components

### Configuration System

**Before**: Hard-coded Unix paths in configuration defaults
```typescript
// Old approach - Unix-only
DOCKER_SOCKET: '/var/run/docker.sock',
KUBECONFIG: '~/.kube/config',
tempDir: '/tmp'
```

**After**: Cross-platform paths using utilities
```typescript
// New approach - cross-platform
DOCKER_SOCKET: getDockerSocketPath(),
KUBECONFIG: getKubeConfigPath(),
tempDir: getTempDir()
```

### Tool Implementations

Major tools updated for cross-platform compatibility:

#### prepare-cluster Tool
- **Kind Installation**: Cross-platform binary download and installation
- **File Operations**: Uses temporary files with proper cleanup
- **Command Execution**: Uses cross-platform command utilities

**Key Improvements**:
```typescript
// Before: Unix-only approach
await execAsync(`curl -Lo ./kind ${kindUrl}`);
await execAsync('chmod +x ./kind');
await execAsync('sudo mv ./kind /usr/local/bin/kind');

// After: Cross-platform approach
await downloadFile(kindUrl, `./${kindExecutable}`);
if (!systemInfo.isWindows) {
  await makeExecutable(`./${kindExecutable}`);
}
// Platform-specific installation logic
```

#### Configuration Loading
- **Path Resolution**: Automatic tilde and environment variable expansion
- **Default Values**: Platform-appropriate defaults
- **Validation**: Cross-platform path validation

### Testing

Comprehensive test suite (`test/unit/cross-platform.test.ts`) covering:
- System detection across platforms
- Path utilities for Windows, macOS, and Linux
- Command execution with different shells
- File operations and cleanup
- Integration tests with real file system operations

## Platform-Specific Considerations

### Windows
- Uses named pipes for Docker socket: `//./pipe/docker_engine`
- Temporary directory from `%TEMP%` environment variable
- Command execution through `cmd.exe`
- File executability determined by extension, not permissions
- Path separators are backslashes (`\`)

### macOS/Linux (Unix-like)
- Uses Unix domain socket for Docker: `/var/run/docker.sock`
- Temporary directory from `$TMPDIR` or `/tmp`
- Command execution through `/bin/sh`
- File executability set via `chmod +x`
- Path separators are forward slashes (`/`)

## Best Practices for Developers

### When Adding New Code

1. **Use Cross-Platform Utilities**: Always use the provided utilities instead of hard-coding paths
```typescript
// Good
import { getTempDir } from '@/lib/path-utils';
const tempFile = path.join(getTempDir(), 'myfile.tmp');

// Bad  
const tempFile = '/tmp/myfile.tmp';
```

2. **Command Execution**: Use the cross-platform command utilities
```typescript
// Good
import { executeCommand } from '@/lib/command-utils';
const result = await executeCommand('echo "hello"');

// Bad
const { exec } = require('child_process');
exec('/bin/sh -c "echo hello"', callback);
```

3. **Path Resolution**: Always resolve paths that might contain `~` or be relative
```typescript
// Good
import { resolvePath } from '@/lib/path-utils';
const configPath = resolvePath('~/.myapp/config.json');

// Bad
const configPath = '~/.myapp/config.json'; // Won't work properly
```

4. **Environment Variables**: Use cross-platform variable expansion
```typescript
// Good
import { expandEnvironmentVariables } from '@/lib/command-utils';
const expanded = expandEnvironmentVariables('Path: $HOME/bin');

// Bad - only works on Unix
const expanded = process.env.HOME + '/bin';
```

### Testing Cross-Platform Code

Always test your code on multiple platforms or use the provided test utilities:

```typescript
// Mock different platforms in tests
jest.mock('os');
const mockedOs = jest.mocked(os);

mockedOs.platform.mockReturnValue('win32'); // Test Windows
mockedOs.platform.mockReturnValue('darwin'); // Test macOS  
mockedOs.platform.mockReturnValue('linux');  // Test Linux
```

## Migration Guide

### For Existing Code

1. **Replace Hard-Coded Paths**:
   - Search for `/tmp`, `/var/run`, `~/.` patterns
   - Replace with cross-platform utilities

2. **Update Command Execution**:
   - Replace `child_process.exec` calls with `executeCommand`
   - Remove shell-specific syntax like `&&` chains

3. **Fix Configuration**:
   - Update default values to use cross-platform utilities
   - Add proper path resolution for user-provided paths

### Example Migration

**Before** (Unix-only):
```typescript
// Shell command with hard-coded paths
const result = await exec(`echo "content" > /tmp/file.txt && chmod +x /tmp/file.txt`);

// Hard-coded configuration
const config = {
  dockerSocket: '/var/run/docker.sock',
  tempDir: '/tmp',
  kubeconfig: '~/.kube/config'
};
```

**After** (Cross-platform):
```typescript
// Cross-platform approach
const tempFile = await createTempFile('content', '.txt');
await makeExecutable(tempFile);

// Cross-platform configuration
const config = {
  dockerSocket: getDockerSocketPath(),
  tempDir: getTempDir(),
  kubeconfig: resolvePath(getKubeConfigPath())
};
```

## Troubleshooting

### Common Issues

1. **"Command not found" errors on Windows**
   - Ensure you're using `executeCommand` which handles shell differences
   - Check that the command exists using `commandExists()`

2. **Path not found errors**
   - Use `resolvePath()` for user-provided paths
   - Use appropriate utility functions for system paths

3. **Permission errors**
   - Use `makeExecutable()` instead of `chmod +x`
   - Check that directories exist with `ensureDirectory()`

### Debug Information

Enable debug logging to see cross-platform decisions:
```typescript
const systemInfo = getSystemInfo();
console.log('Platform:', systemInfo.platform);
console.log('Docker socket:', getDockerSocketPath());
console.log('Temp dir:', getTempDir());
```

## Future Considerations

- **Container Runtime**: Support for other container runtimes (Podman, etc.)
- **ARM Support**: Enhanced ARM64 support for Apple Silicon and ARM servers  
- **Cloud Platforms**: Cloud-specific optimizations for Azure, AWS, GCP
- **CI/CD Integration**: Better integration with Windows-based CI/CD systems

This cross-platform compatibility ensures Container Copilot works reliably across all major development platforms, providing a consistent experience regardless of the user's operating system.
