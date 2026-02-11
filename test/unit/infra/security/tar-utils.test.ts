import { describe, it, expect } from '@jest/globals';
import { parseTarStream } from '@/infra/security/osv-scanner/tar-utils';

describe('tar-utils', () => {
  describe('parseTarStream', () => {
    /**
     * Helper to create a tar buffer with test files
     * Creates a minimal valid tar archive manually
     */
    function createTarBuffer(files: Array<{ path: string; content: string }>): Buffer {
      const chunks: Buffer[] = [];

      for (const file of files) {
        const content = Buffer.from(file.content);
        const header = Buffer.alloc(512);

        // Write file name (offset 0, 100 bytes)
        header.write(file.path, 0, Math.min(100, file.path.length), 'utf-8');

        // Write mode (offset 100, 8 bytes) - octal 0644
        header.write('0000644\0', 100, 8, 'utf-8');

        // Write owner UID (offset 108, 8 bytes)
        header.write('0000000\0', 108, 8, 'utf-8');

        // Write group GID (offset 116, 8 bytes)
        header.write('0000000\0', 116, 8, 'utf-8');

        // Write file size (offset 124, 12 bytes) - octal
        const sizeOctal = content.length.toString(8).padStart(11, '0') + '\0';
        header.write(sizeOctal, 124, 12, 'utf-8');

        // Write modification time (offset 136, 12 bytes)
        header.write('00000000000\0', 136, 12, 'utf-8');

        // Write checksum placeholder (offset 148, 8 bytes) - spaces for now
        header.write('        ', 148, 8, 'utf-8');

        // Write type flag (offset 156, 1 byte) - '0' for regular file
        header.write('0', 156, 1, 'utf-8');

        // Write magic "ustar\0" (offset 257, 6 bytes)
        header.write('ustar\0', 257, 6, 'utf-8');

        // Write version "00" (offset 263, 2 bytes)
        header.write('00', 263, 2, 'utf-8');

        // Calculate checksum (sum of all bytes, treating checksum field as spaces)
        let checksum = 0;
        for (let i = 0; i < 512; i++) {
          checksum += header[i];
        }

        // Write checksum (offset 148, 8 bytes) - octal with trailing null and space
        const checksumOctal = checksum.toString(8).padStart(6, '0') + '\0 ';
        header.write(checksumOctal, 148, 8, 'utf-8');

        chunks.push(header);

        // Write content with padding to 512-byte boundary
        const paddedContent = Buffer.alloc(Math.ceil(content.length / 512) * 512);
        content.copy(paddedContent);
        chunks.push(paddedContent);
      }

      // Add two zero blocks at the end (EOF marker)
      chunks.push(Buffer.alloc(512));
      chunks.push(Buffer.alloc(512));

      return Buffer.concat(chunks);
    }

    describe('normal file extraction', () => {
      it('should extract file by exact path', async () => {
        const tarBuffer = createTarBuffer([
          { path: 'etc/os-release', content: 'NAME="Alpine Linux"' },
          { path: 'var/lib/dpkg/status', content: 'Package: test' },
        ]);

        const result = await parseTarStream(tarBuffer, 'etc/os-release');
        expect(result).toBe('NAME="Alpine Linux"');
      });

      it('should extract file by normalized path (without leading slash)', async () => {
        const tarBuffer = createTarBuffer([
          { path: 'etc/os-release', content: 'NAME="Alpine Linux"' },
        ]);

        const result = await parseTarStream(tarBuffer, '/etc/os-release');
        expect(result).toBe('NAME="Alpine Linux"');
      });

      it('should extract file by basename when available in tar', async () => {
        const tarBuffer = createTarBuffer([{ path: 'status', content: 'Package: test' }]);

        const result = await parseTarStream(tarBuffer, 'status');
        expect(result).toBe('Package: test');
      });

      it('should return null for non-existent file', async () => {
        const tarBuffer = createTarBuffer([
          { path: 'etc/os-release', content: 'NAME="Alpine Linux"' },
        ]);

        const result = await parseTarStream(tarBuffer, 'nonexistent.txt');
        expect(result).toBeNull();
      });

      it('should handle empty tar archive', async () => {
        // Create minimal valid tar (just two zero blocks)
        const tarBuffer = Buffer.concat([Buffer.alloc(512), Buffer.alloc(512)]);

        const result = await parseTarStream(tarBuffer, 'anyfile.txt');
        expect(result).toBeNull();
      });

      it('should handle files with UTF-8 content', async () => {
        const tarBuffer = createTarBuffer([{ path: 'test.txt', content: 'Hello 世界 🌍' }]);

        const result = await parseTarStream(tarBuffer, 'test.txt');
        expect(result).toBe('Hello 世界 🌍');
      });

      it('should handle large file content', async () => {
        const largeContent = 'x'.repeat(10000);
        const tarBuffer = createTarBuffer([{ path: 'large.txt', content: largeContent }]);

        const result = await parseTarStream(tarBuffer, 'large.txt');
        expect(result).toBe(largeContent);
      });
    });

    describe('path traversal protection', () => {
      it('should reject paths with .. at the beginning', async () => {
        const tarBuffer = createTarBuffer([
          { path: '../../../etc/passwd', content: 'root:x:0:0' },
          { path: 'safe.txt', content: 'safe content' },
        ]);

        const result = await parseTarStream(tarBuffer, '../../../etc/passwd');
        expect(result).toBeNull();
      });

      it('should reject paths with .. in the middle', async () => {
        const tarBuffer = createTarBuffer([
          { path: 'var/../etc/passwd', content: 'root:x:0:0' },
          { path: 'safe.txt', content: 'safe content' },
        ]);

        const result = await parseTarStream(tarBuffer, 'var/../etc/passwd');
        expect(result).toBeNull();
      });

      it('should reject paths with .. at the end', async () => {
        const tarBuffer = createTarBuffer([
          { path: 'var/lib/..', content: 'malicious' },
          { path: 'safe.txt', content: 'safe content' },
        ]);

        const result = await parseTarStream(tarBuffer, 'var/lib/..');
        expect(result).toBeNull();
      });

      it('should reject multiple .. sequences', async () => {
        const tarBuffer = createTarBuffer([
          { path: '../../var/../etc/passwd', content: 'root:x:0:0' },
          { path: 'safe.txt', content: 'safe content' },
        ]);

        const result = await parseTarStream(tarBuffer, '../../var/../etc/passwd');
        expect(result).toBeNull();
      });

      it('should still extract safe files when malicious paths are present', async () => {
        const tarBuffer = createTarBuffer([
          { path: '../../../etc/passwd', content: 'malicious' },
          { path: 'safe.txt', content: 'safe content' },
          { path: 'var/../etc/shadow', content: 'malicious' },
        ]);

        const result = await parseTarStream(tarBuffer, 'safe.txt');
        expect(result).toBe('safe content');
      });

      it('should handle absolute paths with traversal', async () => {
        const tarBuffer = createTarBuffer([
          { path: '/var/../etc/passwd', content: 'malicious' },
          { path: 'safe.txt', content: 'safe content' },
        ]);

        const result = await parseTarStream(tarBuffer, '/var/../etc/passwd');
        expect(result).toBeNull();
      });

      it('should allow paths with valid dots (not ..)', async () => {
        const tarBuffer = createTarBuffer([
          { path: 'file.txt', content: 'content with dots' },
          { path: '.hidden', content: 'hidden file' },
          { path: 'dir/.dotfile', content: 'dotfile in dir' },
        ]);

        expect(await parseTarStream(tarBuffer, 'file.txt')).toBe('content with dots');
        expect(await parseTarStream(tarBuffer, '.hidden')).toBe('hidden file');
        expect(await parseTarStream(tarBuffer, 'dir/.dotfile')).toBe('dotfile in dir');
      });
    });

    describe('edge cases', () => {
      it('should handle tar with only malicious entries', async () => {
        const tarBuffer = createTarBuffer([
          { path: '../../../etc/passwd', content: 'malicious' },
          { path: '../../root/.ssh/id_rsa', content: 'malicious' },
        ]);

        expect(await parseTarStream(tarBuffer, '../../../etc/passwd')).toBeNull();
        expect(await parseTarStream(tarBuffer, '../../root/.ssh/id_rsa')).toBeNull();
      });

      it('should handle empty file content', async () => {
        const tarBuffer = createTarBuffer([{ path: 'empty.txt', content: '' }]);

        const result = await parseTarStream(tarBuffer, 'empty.txt');
        expect(result).toBe('');
      });

      it('should handle paths with multiple slashes', async () => {
        const tarBuffer = createTarBuffer([{ path: 'var//lib///dpkg/status', content: 'test' }]);

        const result = await parseTarStream(tarBuffer, 'var//lib///dpkg/status');
        expect(result).toBe('test');
      });

      it('should match first file when basename matches', async () => {
        const tarBuffer = createTarBuffer([{ path: 'config.txt', content: 'first' }]);

        const result = await parseTarStream(tarBuffer, 'config.txt');
        expect(result).toBe('first');
      });
    });

    describe('error handling', () => {
      it('should return null for invalid tar data', async () => {
        const invalidTar = Buffer.from('this is not a valid tar file');

        const result = await parseTarStream(invalidTar, 'anyfile.txt');
        expect(result).toBeNull();
      });

      it('should return null for corrupted tar header', async () => {
        const corruptedTar = Buffer.alloc(1024);
        corruptedTar.write('corrupted data', 0);

        const result = await parseTarStream(corruptedTar, 'anyfile.txt');
        expect(result).toBeNull();
      });
    });
  });
});
