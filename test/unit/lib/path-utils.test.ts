import { safeNormalizePath } from '../../../src/lib/path-utils';

describe('safeNormalizePath', () => {
  it('should convert Windows backslashes to forward slashes', () => {
    expect(safeNormalizePath('C:\\foobar\\test')).toBe('C:/foobar/test');
    expect(safeNormalizePath('D:\\projects\\my-app')).toBe('D:/projects/my-app');
    expect(safeNormalizePath('relative\\path\\test')).toBe('relative/path/test');
  });

  it('should prevent escape sequence interpretation', () => {
    // These contain \f and \t which could be interpreted as form feed and tab
    expect(safeNormalizePath('C:\\foo\\fbar\\tfile')).toBe('C:/foo/fbar/tfile');
    expect(safeNormalizePath('C:\\temp\\new')).toBe('C:/temp/new');
  });

  it('should handle paths with spaces', () => {
    expect(safeNormalizePath('C:\\Program Files\\My App')).toBe('C:/Program Files/My App');
  });

  it('should leave Unix paths unchanged', () => {
    expect(safeNormalizePath('/usr/local/bin')).toBe('/usr/local/bin');
    expect(safeNormalizePath('./relative/path')).toBe('./relative/path');
  });

  it('should handle double slashes', () => {
    expect(safeNormalizePath('C:\\\\share\\\\folder')).toBe('C:/share/folder');
    expect(safeNormalizePath('///path//with///slashes')).toBe('/path/with/slashes');
    expect(safeNormalizePath('path//with//double')).toBe('path/with/double');
  });

  it('should handle empty and null inputs', () => {
    expect(safeNormalizePath('')).toBe('');
    expect(safeNormalizePath(null as any)).toBe(null);
    expect(safeNormalizePath(undefined as any)).toBe(undefined);
  });

  it('should handle duplicate drive letter patterns from Windows shells', () => {
    // Git Bash/MinGW style paths that get duplicated
    expect(safeNormalizePath('/c/c/Users/test')).toBe('/c/Users/test');
    expect(safeNormalizePath('/d/d/projects/app')).toBe('/d/projects/app');
    expect(safeNormalizePath('c:/c/Users/test')).toBe('c:/Users/test');
    expect(safeNormalizePath('D:/D/projects/app')).toBe('D:/projects/app');
    // Case-insensitive duplicates (exactly like the screenshot)
    expect(safeNormalizePath('C:/c/Users/chentomy/Git/BugBashRepos')).toBe('C:/Users/chentomy/Git/BugBashRepos');
    expect(safeNormalizePath('C:\\c\\Users\\test')).toBe('C:/Users/test');
  });

  it('should preserve UNC paths', () => {
    expect(safeNormalizePath('//server/share/folder')).toBe('//server/share/folder');
    expect(safeNormalizePath('\\\\server\\share\\folder')).toBe('//server/share/folder');
  });

  it('should not affect normal paths without duplicate drive letters', () => {
    expect(safeNormalizePath('/c/Users/test')).toBe('/c/Users/test');
    expect(safeNormalizePath('C:/Users/test')).toBe('C:/Users/test');
    expect(safeNormalizePath('/d/projects/app')).toBe('/d/projects/app');
  });
});
