import { normalizePath, mapNodeArchToPlatform, isPlatformCompatible } from '../../../src/lib/platform';

describe('mapNodeArchToPlatform', () => {
  it('should map common amd64 architectures to linux/amd64', () => {
    expect(mapNodeArchToPlatform('amd64')).toBe('linux/amd64');
    expect(mapNodeArchToPlatform('x86_64')).toBe('linux/amd64');
    expect(mapNodeArchToPlatform('AMD64')).toBe('linux/amd64'); // case insensitive
    expect(mapNodeArchToPlatform('X86_64')).toBe('linux/amd64');
  });

  it('should map common arm64 architectures to linux/arm64', () => {
    expect(mapNodeArchToPlatform('arm64')).toBe('linux/arm64');
    expect(mapNodeArchToPlatform('aarch64')).toBe('linux/arm64');
    expect(mapNodeArchToPlatform('ARM64')).toBe('linux/arm64');
    expect(mapNodeArchToPlatform('AARCH64')).toBe('linux/arm64');
  });

  it('should map arm v7 and v6 architectures', () => {
    expect(mapNodeArchToPlatform('armv7l')).toBe('linux/arm/v7');
    expect(mapNodeArchToPlatform('armv6l')).toBe('linux/arm/v6');
    expect(mapNodeArchToPlatform('ARMV7L')).toBe('linux/arm/v7');
  });

  it('should map 386 architectures to linux/386', () => {
    expect(mapNodeArchToPlatform('386')).toBe('linux/386');
    expect(mapNodeArchToPlatform('i386')).toBe('linux/386');
    expect(mapNodeArchToPlatform('i686')).toBe('linux/386');
    expect(mapNodeArchToPlatform('I386')).toBe('linux/386');
  });

  it('should map other architectures', () => {
    expect(mapNodeArchToPlatform('ppc64le')).toBe('linux/ppc64le');
    expect(mapNodeArchToPlatform('s390x')).toBe('linux/s390x');
    expect(mapNodeArchToPlatform('riscv64')).toBe('linux/riscv64');
  });

  it('should support custom OS parameter', () => {
    expect(mapNodeArchToPlatform('amd64', 'windows')).toBe('windows/amd64');
    expect(mapNodeArchToPlatform('amd64', 'linux')).toBe('linux/amd64');
  });

  it('should return null for unknown architectures', () => {
    expect(mapNodeArchToPlatform('unknown')).toBe(null);
    expect(mapNodeArchToPlatform('mips')).toBe(null);
    expect(mapNodeArchToPlatform('sparc')).toBe(null);
    expect(mapNodeArchToPlatform('')).toBe(null);
  });

  it('should return null for unsupported OS/arch combinations', () => {
    // windows/arm64 is not in the valid platforms list
    expect(mapNodeArchToPlatform('arm64', 'windows')).toBe(null);
    expect(mapNodeArchToPlatform('amd64', 'darwin')).toBe(null);
  });

  it('should handle edge cases', () => {
    expect(mapNodeArchToPlatform('  amd64  ')).toBe(null); // whitespace not trimmed
    expect(mapNodeArchToPlatform('amd64\n')).toBe(null);
  });
});

describe('isPlatformCompatible', () => {
  it('should return true for exact platform matches', () => {
    expect(isPlatformCompatible('linux/amd64', 'linux/amd64')).toBe(true);
    expect(isPlatformCompatible('linux/arm64', 'linux/arm64')).toBe(true);
    expect(isPlatformCompatible('linux/arm/v7', 'linux/arm/v7')).toBe(true);
    expect(isPlatformCompatible('windows/amd64', 'windows/amd64')).toBe(true);
  });

  it('should allow ARM64 cluster to run ARM/v7 and ARM/v6 images', () => {
    expect(isPlatformCompatible('linux/arm/v7', 'linux/arm64')).toBe(true);
    expect(isPlatformCompatible('linux/arm/v6', 'linux/arm64')).toBe(true);
  });

  it('should not allow ARM/v7 or ARM/v6 cluster to run ARM64 images', () => {
    expect(isPlatformCompatible('linux/arm64', 'linux/arm/v7')).toBe(false);
    expect(isPlatformCompatible('linux/arm64', 'linux/arm/v6')).toBe(false);
  });

  it('should allow AMD64 cluster to run 386 images', () => {
    expect(isPlatformCompatible('linux/386', 'linux/amd64')).toBe(true);
  });

  it('should not allow 386 cluster to run AMD64 images', () => {
    expect(isPlatformCompatible('linux/amd64', 'linux/386')).toBe(false);
  });

  it('should return false for incompatible platforms', () => {
    expect(isPlatformCompatible('linux/amd64', 'linux/arm64')).toBe(false);
    expect(isPlatformCompatible('linux/arm64', 'linux/amd64')).toBe(false);
    expect(isPlatformCompatible('linux/amd64', 'linux/arm/v7')).toBe(false);
    expect(isPlatformCompatible('linux/arm/v7', 'linux/amd64')).toBe(false);
  });

  it('should return false for cross-OS platforms', () => {
    expect(isPlatformCompatible('linux/amd64', 'windows/amd64')).toBe(false);
    expect(isPlatformCompatible('windows/amd64', 'linux/amd64')).toBe(false);
  });

  it('should handle all supported platform combinations', () => {
    // Test a few more edge cases
    expect(isPlatformCompatible('linux/ppc64le', 'linux/ppc64le')).toBe(true);
    expect(isPlatformCompatible('linux/s390x', 'linux/s390x')).toBe(true);
    expect(isPlatformCompatible('linux/riscv64', 'linux/riscv64')).toBe(true);

    // These should be incompatible
    expect(isPlatformCompatible('linux/ppc64le', 'linux/amd64')).toBe(false);
    expect(isPlatformCompatible('linux/s390x', 'linux/arm64')).toBe(false);
  });
});

describe('normalizePath', () => {
  it('should convert Windows backslashes to forward slashes', () => {
    expect(normalizePath('C:\\foobar\\test')).toBe('C:/foobar/test');
    expect(normalizePath('D:\\projects\\my-app')).toBe('D:/projects/my-app');
    expect(normalizePath('relative\\path\\test')).toBe('relative/path/test');
  });

  it('should prevent escape sequence interpretation', () => {
    // These contain \f and \t which could be interpreted as form feed and tab
    expect(normalizePath('C:\\foo\\fbar\\tfile')).toBe('C:/foo/fbar/tfile');
    expect(normalizePath('C:\\temp\\new')).toBe('C:/temp/new');
  });

  it('should handle paths with spaces', () => {
    expect(normalizePath('C:\\Program Files\\My App')).toBe('C:/Program Files/My App');
  });

  it('should leave Unix paths unchanged', () => {
    expect(normalizePath('/usr/local/bin')).toBe('/usr/local/bin');
    expect(normalizePath('./relative/path')).toBe('relative/path');
  });

  it('should handle double slashes', () => {
    expect(normalizePath('C:\\\\share\\\\folder')).toBe('C:/share/folder');
    expect(normalizePath('//network//path')).toBe('/network/path');
  });

  it('should handle empty and null inputs', () => {
    expect(normalizePath('')).toBe('');
    expect(normalizePath(null as any)).toBe(null);
    expect(normalizePath(undefined as any)).toBe(undefined);
  });

  it('should normalize complex paths using path.posix.normalize', () => {
    expect(normalizePath('C:\\folder\\..\\other\\file.txt')).toBe('C:/other/file.txt');
    expect(normalizePath('\\\\server\\share\\folder\\..\\file')).toBe('/server/share/file');
    expect(normalizePath('./folder\\subfolder\\..\\file.js')).toBe('folder/file.js');
  });

  it('should handle mixed separators', () => {
    expect(normalizePath('C:\\mixed/path\\to/file')).toBe('C:/mixed/path/to/file');
    expect(normalizePath('/unix\\windows/mixed\\path')).toBe('/unix/windows/mixed/path');
  });

  it('should handle Windows-specific edge cases', () => {
    // UNC paths
    expect(normalizePath('\\\\server\\share\\file')).toBe('/server/share/file');
    expect(normalizePath('\\\\?\\C:\\very\\long\\path')).toBe('/?/C:/very/long/path');
    
    // Drive letters with different cases
    expect(normalizePath('c:\\users\\test')).toBe('c:/users/test');
    expect(normalizePath('D:\\Program Files (x86)\\app')).toBe('D:/Program Files (x86)/app');
  });

  it('should handle potentially problematic escape sequences', () => {
    // These could be interpreted as escape sequences in some contexts
    expect(normalizePath('C:\\new\\folder')).toBe('C:/new/folder');  // \n could be newline
    expect(normalizePath('C:\\temp\\file')).toBe('C:/temp/file');    // \t could be tab
    expect(normalizePath('C:\\form\\feed')).toBe('C:/form/feed');    // \f could be form feed
    expect(normalizePath('C:\\return\\path')).toBe('C:/return/path');// \r could be carriage return
    expect(normalizePath('C:\\backup\\file')).toBe('C:/backup/file');// \b could be backspace
    expect(normalizePath('C:\\vertical\\tab')).toBe('C:/vertical/tab');// \v could be vertical tab
  });

  it('should handle Docker-specific path scenarios', () => {
    // Common Docker build context paths
    expect(normalizePath('.\\docker\\Dockerfile')).toBe('docker/Dockerfile');
    expect(normalizePath('..\\parent\\project')).toBe('../parent/project');
    expect(normalizePath('.\\src\\..\\dist\\app.js')).toBe('dist/app.js');
    
    // Docker volume mount paths (Windows)
    expect(normalizePath('C:\\Users\\user\\project:/app')).toBe('C:/Users/user/project:/app');
    expect(normalizePath('/c/Users/user/project')).toBe('/c/Users/user/project');
  });

  it('should handle Kubernetes and container registry paths', () => {
    // Container image paths with backslashes (shouldn't happen but test anyway)
    expect(normalizePath('registry\\namespace\\image:tag')).toBe('registry/namespace/image:tag');
    
    // File paths for manifests
    expect(normalizePath('.\\k8s\\deployment.yaml')).toBe('k8s/deployment.yaml');
    expect(normalizePath('..\\config\\secrets.yaml')).toBe('../config/secrets.yaml');
  });

  it('should handle repository analysis path scenarios', () => {
    // Package manager file paths
    expect(normalizePath('.\\package.json')).toBe('package.json');
    expect(normalizePath('subfolder\\package.json')).toBe('subfolder/package.json');
    expect(normalizePath('.\\node_modules\\@types\\node')).toBe('node_modules/@types/node');
    
    // Build system file paths
    expect(normalizePath('.\\src\\main\\java\\App.java')).toBe('src/main/java/App.java');
    expect(normalizePath('src\\test\\..\\main\\resources')).toBe('src/main/resources');
  });

  it('should handle CI/CD and build path scenarios', () => {
    // GitHub Actions paths (Windows runners)
    expect(normalizePath('D:\\a\\project\\project\\.github\\workflows')).toBe('D:/a/project/project/.github/workflows');
    
    // Build output paths
    expect(normalizePath('.\\dist\\..\\build\\output')).toBe('build/output');
    expect(normalizePath('target\\classes\\..\\..\\src\\main')).toBe('src/main');
  });

  it('should preserve important path characteristics', () => {
    // Absolute vs relative paths
    expect(normalizePath('C:\\absolute\\path')).toBe('C:/absolute/path');
    expect(normalizePath('.\\relative\\path')).toBe('relative/path');
    expect(normalizePath('..\\parent\\path')).toBe('../parent/path');
    
    // Trailing slashes - path.posix.normalize preserves trailing slashes for directories
    expect(normalizePath('folder\\')).toBe('folder/');
    expect(normalizePath('folder/')).toBe('folder/');
  });

  it('should handle special characters in paths', () => {
    // Paths with spaces, hyphens, underscores
    expect(normalizePath('C:\\Program Files\\My App\\file-name_v1.txt')).toBe('C:/Program Files/My App/file-name_v1.txt');
    
    // Paths with Unicode characters
    expect(normalizePath('C:\\Users\\José\\Documents\\café.txt')).toBe('C:/Users/José/Documents/café.txt');
    
    // Paths with parentheses (common in Windows)
    expect(normalizePath('C:\\Program Files (x86)\\app')).toBe('C:/Program Files (x86)/app');
  });
});
