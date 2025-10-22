import { normalizePath } from '../../../src/lib/platform';

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
