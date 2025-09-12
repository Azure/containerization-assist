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
    expect(safeNormalizePath('//network//path')).toBe('/network/path');
  });

  it('should handle empty and null inputs', () => {
    expect(safeNormalizePath('')).toBe('');
    expect(safeNormalizePath(null as any)).toBe(null);
    expect(safeNormalizePath(undefined as any)).toBe(undefined);
  });
});
