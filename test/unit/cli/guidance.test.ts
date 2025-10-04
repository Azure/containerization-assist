/**
 * Unit tests for CLI contextual guidance module
 */

import { provideContextualGuidance } from '@/cli/guidance';
import { jest } from '@jest/globals';

describe('provideContextualGuidance', () => {
  let consoleErrorSpy: jest.SpyInstance;

  beforeEach(() => {
    consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('should display Docker-related guidance for Docker errors', () => {
    const error = new Error('Docker connection failed');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('🔍 Error: Docker connection failed'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('💡 Docker-related issue detected:'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Ensure Docker Desktop/Engine is running'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('docker version'));
  });

  it('should display Docker-related guidance for ENOENT errors', () => {
    const error = new Error('ENOENT: no such file or directory');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('💡 Docker-related issue detected:'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Check Docker socket path'));
  });

  it('should display permission guidance for permission errors', () => {
    const error = new Error('Permission denied accessing workspace');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('💡 Permission issue detected:'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Check file/directory permissions'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Docker socket permissions'));
  });

  it('should display permission guidance for EACCES errors', () => {
    const error = new Error('EACCES: permission denied');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('💡 Permission issue detected:'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('add user to docker group'));
  });

  it('should display configuration guidance for config errors', () => {
    const error = new Error('Configuration file not found');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('💡 Configuration issue:'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Copy .env.example to .env'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Validate configuration'));
  });

  it('should display general troubleshooting steps for all errors', () => {
    const error = new Error('Generic error message');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('🛠️ General troubleshooting steps:'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Run health check'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Validate config'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Check Docker'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('Enable debug logging'));
  });

  it('should show stack trace in dev mode', () => {
    const error = new Error('Test error');
    error.stack = 'Error: Test error\n    at line 1\n    at line 2';

    provideContextualGuidance(error, { dev: true });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('📍 Stack trace (dev mode):'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(error.stack);
  });

  it('should suggest dev flag when not in dev mode', () => {
    const error = new Error('Test error');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('For detailed error information, use --dev flag'));
  });

  it('should not show stack trace when not in dev mode', () => {
    const error = new Error('Test error');
    error.stack = 'Error: Test error\n    at line 1';

    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).not.toHaveBeenCalledWith(expect.stringContaining('📍 Stack trace'));
    expect(consoleErrorSpy).not.toHaveBeenCalledWith(error.stack);
  });

  it('should handle options parameter being undefined', () => {
    const error = new Error('Test error');

    expect(() => provideContextualGuidance(error)).not.toThrow();
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('🔍 Error:'));
  });

  it('should detect multiple error categories and show relevant guidance', () => {
    const error = new Error('Docker permission denied: EACCES');
    provideContextualGuidance(error, { dev: false });

    // Should detect Docker category first (as per implementation priority)
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('💡 Docker-related issue detected:'));

    // Should always show general troubleshooting
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('🛠️ General troubleshooting steps:'));
  });

  it('should format general troubleshooting steps as numbered list', () => {
    const error = new Error('Test error');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('1. Run health check'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('2. Validate config'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('3. Check Docker'));
  });

  it('should format category-specific steps with bullet points', () => {
    const error = new Error('Docker error');
    provideContextualGuidance(error, { dev: false });

    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('  • Ensure Docker Desktop/Engine is running'));
    expect(consoleErrorSpy).toHaveBeenCalledWith(expect.stringContaining('  • Verify Docker socket access permissions'));
  });
});
