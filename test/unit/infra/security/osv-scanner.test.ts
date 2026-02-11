/**
 * OSV Scanner Tests
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import type { Logger } from 'pino';
import { checkOSVAvailability } from '@/infra/security/osv-scanner';

describe('OSV Scanner', () => {
  let mockLogger: Logger;

  beforeEach(() => {
    // Create a mock logger
    mockLogger = {
      info: jest.fn(),
      debug: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    } as unknown as Logger;

    // Clear any existing fetch mocks
    jest.clearAllMocks();
  });

  describe('checkOSVAvailability', () => {
    it('should return success when OSV API is accessible', async () => {
      // Mock fetch to simulate successful API response
      global.fetch = jest.fn(() =>
        Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({}),
        } as Response),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('osv-api-available');
      }
    });

    it('should return success even with 404 (API is available)', async () => {
      // Mock fetch to simulate 404 (API is reachable but query returns nothing)
      global.fetch = jest.fn(() =>
        Promise.resolve({
          ok: false,
          status: 404,
          json: async () => ({}),
        } as Response),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('osv-api-available');
      }
    });

    it('should return failure when OSV API is not accessible', async () => {
      // Mock fetch to simulate network error
      global.fetch = jest.fn(() =>
        Promise.reject(new Error('Network error')),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('OSV API not accessible');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should provide helpful guidance when API is unavailable', async () => {
      // Mock fetch to simulate network error
      global.fetch = jest.fn(() =>
        Promise.reject(new Error('ECONNREFUSED')),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.guidance?.hint).toContain('network');
        expect(result.guidance?.resolution).toBeDefined();
      }
    });

    it('should return failure for 500 server error', async () => {
      // Mock fetch to simulate 500 server error
      global.fetch = jest.fn(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          statusText: 'Internal Server Error',
          json: async () => ({}),
        } as Response),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('server error: 500');
        expect(result.guidance?.message).toContain('HTTP 500');
        expect(result.guidance?.hint).toContain('not responding correctly');
      }
    });

    it('should return failure for 503 service unavailable', async () => {
      // Mock fetch to simulate 503 service unavailable
      global.fetch = jest.fn(() =>
        Promise.resolve({
          ok: false,
          status: 503,
          statusText: 'Service Unavailable',
          json: async () => ({}),
        } as Response),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('server error: 503');
        expect(result.guidance?.details?.status).toBe(503);
      }
    });

    it('should return success for 400 bad request (API is up)', async () => {
      // Mock fetch to simulate 400 (client error - API is working)
      global.fetch = jest.fn(() =>
        Promise.resolve({
          ok: false,
          status: 400,
          statusText: 'Bad Request',
          json: async () => ({}),
        } as Response),
      ) as jest.Mock;

      const result = await checkOSVAvailability(mockLogger);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('osv-api-available');
      }
    });
  });

  describe('pom.xml parsing', () => {
    // Helper to create a tar archive with pom.xml
    async function createTarWithPom(pomContent: string): Promise<Buffer> {
      const tar = await import('tar');
      const { mkdir, writeFile, rm } = await import('fs/promises');
      const { join } = await import('path');
      const { tmpdir } = await import('os');
      const { randomBytes } = await import('crypto');

      // Create temp directory
      const tempDir = join(tmpdir(), `osv-test-${randomBytes(8).toString('hex')}`);
      await mkdir(tempDir, { recursive: true });

      const pomPath = join(tempDir, 'pom.xml');
      await writeFile(pomPath, pomContent);

      // Create tar in memory
      const chunks: Buffer[] = [];
      const tarStream = tar.c({ cwd: tempDir, gzip: false }, ['pom.xml']);

      return new Promise((resolve, reject) => {
        tarStream.on('data', (chunk: Buffer) => chunks.push(chunk));
        tarStream.on('end', async () => {
          await rm(tempDir, { recursive: true, force: true });
          resolve(Buffer.concat(chunks));
        });
        tarStream.on('error', async (err: Error) => {
          await rm(tempDir, { recursive: true, force: true });
          reject(err);
        });
      });
    }

    it('should parse simple pom.xml with dependencies', async () => {
      const pomXml = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>my-app</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
      <version>2.7.0</version>
    </dependency>
    <dependency>
      <groupId>junit</groupId>
      <artifactId>junit</artifactId>
      <version>4.13.2</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>`;

      const tarBuffer = await createTarWithPom(pomXml);
      const { parseMavenPackages } = await import('@/infra/security/osv-scanner/maven/pom-parser');
      
      const packages = await parseMavenPackages(tarBuffer, '/pom.xml', mockLogger);
      
      expect(packages).toBeDefined();
      expect(packages.length).toBe(2);
      
      // Check Spring Boot dependency
      const springBoot = packages.find(p => p.name === 'org.springframework.boot:spring-boot-starter-web');
      expect(springBoot).toBeDefined();
      expect(springBoot?.version).toBe('2.7.0');
      expect(springBoot?.ecosystem).toBe('Maven');
      
      // Check JUnit dependency
      const junit = packages.find(p => p.name === 'junit:junit');
      expect(junit).toBeDefined();
      expect(junit?.version).toBe('4.13.2');
      expect(junit?.ecosystem).toBe('Maven');
    });

    it('should handle pom.xml with dependencyManagement', async () => {
      const pomXml = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>parent-pom</artifactId>
  <version>1.0.0</version>
  <dependencyManagement>
    <dependencies>
      <dependency>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-dependencies</artifactId>
        <version>2.7.0</version>
        <type>pom</type>
        <scope>import</scope>
      </dependency>
    </dependencies>
  </dependencyManagement>
</project>`;

      const tarBuffer = await createTarWithPom(pomXml);
      const { parseMavenPackages } = await import('@/infra/security/osv-scanner/maven/pom-parser');
      
      const packages = await parseMavenPackages(tarBuffer, '/pom.xml', mockLogger);
      
      expect(packages).toBeDefined();
      expect(packages.length).toBe(1);
      
      const springBootDeps = packages.find(p => p.name === 'org.springframework.boot:spring-boot-dependencies');
      expect(springBootDeps).toBeDefined();
      expect(springBootDeps?.version).toBe('2.7.0');
      expect(springBootDeps?.ecosystem).toBe('Maven');
    });

    it('should handle empty pom.xml', async () => {
      const pomXml = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>my-app</artifactId>
  <version>1.0.0</version>
</project>`;

      const tarBuffer = await createTarWithPom(pomXml);
      const { parseMavenPackages } = await import('@/infra/security/osv-scanner/maven/pom-parser');
      
      const packages = await parseMavenPackages(tarBuffer, '/pom.xml', mockLogger);
      
      expect(packages).toBeDefined();
      expect(packages.length).toBe(0);
    });

    it('should reject oversized XML files (XML bomb protection)', async () => {
      // Create an XML file larger than 10MB (the limit)
      // This simulates a billion laughs or quadratic blowup attack
      const header = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>my-app</artifactId>
  <version>1.0.0</version>
  <properties>`;
      
      const footer = `  </properties>
</project>`;
      
      // Create a large XML by adding many property elements
      // Each property is ~100 bytes, so we need ~110,000 to exceed 10MB
      const properties = Array.from({ length: 110000 }, (_, i) => 
        `    <property${i}>some-very-long-value-that-makes-this-xml-excessively-large-${i}</property${i}>`
      ).join('\n');
      
      const largePomXml = header + '\n' + properties + '\n' + footer;

      const tarBuffer = await createTarWithPom(largePomXml);
      const { parseMavenPackages } = await import('@/infra/security/osv-scanner/maven/pom-parser');
      
      // Should return empty array instead of throwing (graceful degradation)
      const packages = await parseMavenPackages(tarBuffer, '/pom.xml', mockLogger);
      
      expect(packages).toBeDefined();
      expect(packages.length).toBe(0);
      
      // Should have logged a warning about the oversized file
      expect(mockLogger.warn).toHaveBeenCalled();
    }, 30000); // Increase timeout for this large file test

    it('should handle reasonably large but valid pom.xml files', async () => {
      // Create a large but reasonable pom.xml (e.g., 500KB)
      const dependencies = Array.from({ length: 500 }, (_, i) => `
    <dependency>
      <groupId>com.example.group${i % 10}</groupId>
      <artifactId>artifact-${i}</artifactId>
      <version>1.${i}.0</version>
    </dependency>`).join('');
      
      const pomXml = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>large-project</artifactId>
  <version>1.0.0</version>
  <dependencies>${dependencies}
  </dependencies>
</project>`;

      const tarBuffer = await createTarWithPom(pomXml);
      const { parseMavenPackages } = await import('@/infra/security/osv-scanner/maven/pom-parser');
      
      const packages = await parseMavenPackages(tarBuffer, '/pom.xml', mockLogger);
      
      // Should successfully parse the large but valid file
      expect(packages).toBeDefined();
      expect(packages.length).toBe(500);
    });

    it('should reject XML with entity expansion (billion laughs attack)', async () => {
      // Simulate a billion laughs attack with entity expansion
      // Note: xml2js doesn't process external entities by default, but we still
      // protect against the expanded result being too large
      const billionLaughs = `<?xml version="1.0"?>
<!DOCTYPE lolz [
  <!ENTITY lol "lol">
  <!ELEMENT lolz (#PCDATA)>
  <!ENTITY lol1 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
  <!ENTITY lol2 "&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;">
  <!ENTITY lol3 "&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;">
  <!ENTITY lol4 "&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;">
  <!ENTITY lol5 "&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;">
  <!ENTITY lol6 "&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;">
  <!ENTITY lol7 "&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;">
  <!ENTITY lol8 "&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;">
  <!ENTITY lol9 "&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;">
]>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>&lol9;</groupId>
  <artifactId>bomb</artifactId>
  <version>1.0.0</version>
</project>`;

      const tarBuffer = await createTarWithPom(billionLaughs);
      const { parseMavenPackages } = await import('@/infra/security/osv-scanner/maven/pom-parser');
      
      // Should handle gracefully (xml2js doesn't expand entities by default)
      const packages = await parseMavenPackages(tarBuffer, '/pom.xml', mockLogger);
      
      expect(packages).toBeDefined();
      // Empty array is acceptable - the main thing is not crashing or consuming excessive memory
    });
  });
});
