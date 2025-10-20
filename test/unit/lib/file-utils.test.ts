/**
 * Tests for file utility functions
 */

import { promises as fs } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import http from 'node:http';
import type { Server } from 'node:http';
import {
  downloadFile,
  makeExecutable,
  createTempFile,
  deleteTempFile,
} from '@/lib/file-utils';

describe('file-utils', () => {
  describe('createTempFile', () => {
    const tempFiles: string[] = [];

    afterEach(async () => {
      // Clean up created temp files
      await Promise.all(tempFiles.map((file) => deleteTempFile(file)));
      tempFiles.length = 0;
    });

    it('should create temp file with content', async () => {
      const content = 'test content';
      const filePath = await createTempFile(content);
      tempFiles.push(filePath);

      expect(filePath).toBeDefined();
      expect(filePath.startsWith(tmpdir())).toBe(true);

      const readContent = await fs.readFile(filePath, 'utf8');
      expect(readContent).toBe(content);
    });

    it('should create temp file with extension', async () => {
      const content = '{ "test": true }';
      const filePath = await createTempFile(content, '.json');
      tempFiles.push(filePath);

      expect(filePath).toMatch(/\.json$/);
      const readContent = await fs.readFile(filePath, 'utf8');
      expect(readContent).toBe(content);
    });

    it('should create temp file with empty content', async () => {
      const filePath = await createTempFile('');
      tempFiles.push(filePath);

      const readContent = await fs.readFile(filePath, 'utf8');
      expect(readContent).toBe('');
    });

    it('should create temp file with multiline content', async () => {
      const content = 'line 1\nline 2\nline 3';
      const filePath = await createTempFile(content);
      tempFiles.push(filePath);

      const readContent = await fs.readFile(filePath, 'utf8');
      expect(readContent).toBe(content);
    });

    it('should create unique temp files', async () => {
      const file1 = await createTempFile('content1');
      const file2 = await createTempFile('content2');
      tempFiles.push(file1, file2);

      expect(file1).not.toBe(file2);

      const content1 = await fs.readFile(file1, 'utf8');
      const content2 = await fs.readFile(file2, 'utf8');
      expect(content1).toBe('content1');
      expect(content2).toBe('content2');
    });

    it('should set restrictive permissions (0o600)', async () => {
      const filePath = await createTempFile('secret data');
      tempFiles.push(filePath);

      const stats = await fs.stat(filePath);
      // On Unix systems, check permissions are restrictive
      if (process.platform !== 'win32') {
        // eslint-disable-next-line no-bitwise
        expect(stats.mode & 0o777).toBe(0o600);
      }
    });
  });

  describe('deleteTempFile', () => {
    it('should delete existing file', async () => {
      const filePath = await createTempFile('test');

      // Verify file exists
      await expect(fs.access(filePath)).resolves.not.toThrow();

      // Delete file
      await deleteTempFile(filePath);

      // Verify file is deleted
      await expect(fs.access(filePath)).rejects.toThrow();
    });

    it('should ignore errors when deleting non-existent file', async () => {
      const nonExistentPath = path.join(tmpdir(), 'non-existent-file-12345.txt');

      // Should not throw
      await expect(deleteTempFile(nonExistentPath)).resolves.not.toThrow();
    });

    it('should ignore permission errors', async () => {
      // This test simulates error handling, actual permission errors are hard to create safely
      await expect(deleteTempFile('/invalid/path/that/does/not/exist')).resolves.not.toThrow();
    });
  });

  describe('makeExecutable', () => {
    let tempFile: string | null = null;

    afterEach(async () => {
      if (tempFile) {
        await deleteTempFile(tempFile);
        tempFile = null;
      }
    });

    it('should make file executable', async () => {
      tempFile = await createTempFile('#!/bin/bash\necho "hello"', '.sh');

      await makeExecutable(tempFile);

      const stats = await fs.stat(tempFile);

      // On Unix systems, check executable bit is set
      if (process.platform !== 'win32') {
        // eslint-disable-next-line no-bitwise
        expect(stats.mode & 0o111).toBeGreaterThan(0);
        // eslint-disable-next-line no-bitwise
        expect(stats.mode & 0o777).toBe(0o755);
      }
    });

    it('should fail when file does not exist', async () => {
      const nonExistentPath = path.join(tmpdir(), 'non-existent-file.sh');

      await expect(makeExecutable(nonExistentPath)).rejects.toThrow();
    });
  });

  describe('downloadFile', () => {
    let server: Server;
    let serverUrl: string;

    beforeAll((done) => {
      // Create a simple HTTP server for testing
      server = http.createServer((req, res) => {
        if (req.url === '/test.txt') {
          res.writeHead(200, { 'Content-Type': 'text/plain' });
          res.end('test file content');
        } else if (req.url === '/large.txt') {
          res.writeHead(200, { 'Content-Type': 'text/plain' });
          res.end('a'.repeat(10000));
        } else if (req.url === '/error') {
          res.writeHead(500, { 'Content-Type': 'text/plain' });
          res.end('Server error');
        } else {
          res.writeHead(404);
          res.end('Not found');
        }
      });

      server.listen(0, 'localhost', () => {
        const address = server.address();
        if (address && typeof address === 'object') {
          serverUrl = `http://localhost:${address.port}`;
        }
        done();
      });
    });

    afterAll((done) => {
      server.close(done);
    });

    const tempFiles: string[] = [];

    afterEach(async () => {
      await Promise.all(tempFiles.map((file) => deleteTempFile(file)));
      tempFiles.length = 0;
    });

    it('should download file from HTTP URL', async () => {
      const destPath = path.join(tmpdir(), 'downloaded-test.txt');
      tempFiles.push(destPath);

      await downloadFile(`${serverUrl}/test.txt`, destPath);

      const content = await fs.readFile(destPath, 'utf8');
      expect(content).toBe('test file content');
    });

    it('should download large file', async () => {
      const destPath = path.join(tmpdir(), 'downloaded-large.txt');
      tempFiles.push(destPath);

      await downloadFile(`${serverUrl}/large.txt`, destPath);

      const content = await fs.readFile(destPath, 'utf8');
      expect(content).toBe('a'.repeat(10000));
    });

    it('should download response even for non-200 status codes', async () => {
      // Note: downloadFile doesn't check HTTP status codes, it downloads whatever is returned
      const destPath = path.join(tmpdir(), 'downloaded-404.txt');
      tempFiles.push(destPath);

      await downloadFile(`${serverUrl}/nonexistent`, destPath);

      // File exists with the 404 response body
      const content = await fs.readFile(destPath, 'utf8');
      expect(content).toBe('Not found');
    });

    it('should handle invalid URL', async () => {
      const destPath = path.join(tmpdir(), 'downloaded-invalid.txt');

      await expect(downloadFile('not-a-valid-url', destPath)).rejects.toThrow();
    });

    it('should delete partial file on error', async () => {
      const destPath = path.join(tmpdir(), 'downloaded-partial.txt');

      // Create a URL that will cause an error
      const invalidUrl = 'http://localhost:99999/test.txt';

      await expect(downloadFile(invalidUrl, destPath)).rejects.toThrow();

      // File should be deleted on error
      await expect(fs.access(destPath)).rejects.toThrow();
    });
  });
});
