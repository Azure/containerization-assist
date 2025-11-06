/**
 * Unit tests for analyze-repo parsers
 * Tests individual parser functions for better coverage
 */

import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { promises as fs } from 'node:fs';
import {
  parsePackageJson,
  parsePomXml,
  parseGradle,
  parsePythonConfig,
  parseCargoToml,
  parseCsProj,
  parseGoMod,
} from '@/tools/analyze-repo/parsers';

// Mock fs module
jest.mock('node:fs', () => ({
  promises: {
    readFile: jest.fn(),
  },
}));

describe('analyze-repo parsers', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('parsePackageJson', () => {
    it('should parse NestJS project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'nestjs-app',
          dependencies: { '@nestjs/core': '^10.0.0', '@nestjs/platform-express': '^10.0.0' },
          engines: { node: '20.x' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.framework).toBe('nestjs');
      expect(result.language).toBe('javascript');
    });

    it('should parse Next.js project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'nextjs-app',
          dependencies: { next: '^14.0.0', react: '^18.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.framework).toBe('next');
      expect(result.ports).toContain(3000);
    });

    it('should parse Nuxt project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'nuxt-app',
          dependencies: { nuxt: '^3.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.framework).toBe('nuxt');
      expect(result.ports).toContain(3000);
    });

    it('should parse Vue project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'vue-app',
          dependencies: { vue: '^3.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.framework).toBe('vue');
      expect(result.ports).toContain(3000);
    });

    it('should parse Angular project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'angular-app',
          dependencies: { '@angular/core': '^17.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.framework).toBe('angular');
      expect(result.ports).toContain(4200);
    });

    it('should parse React project without Next', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'react-app',
          dependencies: { react: '^18.0.0', 'react-dom': '^18.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.framework).toBe('react');
      expect(result.ports).toContain(3000);
    });

    it('should detect TypeScript projects', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'ts-app',
          dependencies: { express: '^4.0.0' },
          devDependencies: { typescript: '^5.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.language).toBe('typescript');
    });

    it('should detect TypeScript with ts-node', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'ts-app',
          dependencies: { 'ts-node': '^10.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.language).toBe('typescript');
    });

    it('should detect TypeScript with @typescript-eslint/parser', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'ts-app',
          devDependencies: { '@typescript-eslint/parser': '^5.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.language).toBe('typescript');
    });

    it('should extract ports from scripts with PORT=', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          scripts: {
            start: 'PORT=8080 node server.js',
          },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.ports).toContain(8080);
    });

    it('should extract ports from scripts with --port=', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          scripts: {
            start: 'node server.js --port=4000',
          },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.ports).toContain(4000);
    });

    it('should extract ports from scripts with --port (space)', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          scripts: {
            start: 'node server.js --port 5000',
          },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.ports).toContain(5000);
    });

    it('should ignore invalid port numbers', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          scripts: {
            start: 'PORT=99999 node server.js', // Port > 65535
          },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.ports).not.toContain(99999);
    });

    it('should ignore zero port', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          scripts: {
            start: 'PORT=0 node server.js',
          },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.ports).not.toContain(0);
    });

    it('should extract entry point from start script with node', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          scripts: {
            start: 'node dist/server.js',
          },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.entryPoint).toBe('dist/server.js');
    });

    it('should extract entry point from start script with ts-node', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          scripts: {
            start: 'ts-node src/main.ts',
          },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.entryPoint).toBe('src/main.ts');
    });

    it('should use main field as fallback for entry point', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          main: 'lib/index.js',
          dependencies: {},
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.entryPoint).toBe('lib/index.js');
    });

    it('should default to index.js when no main or start script', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.entryPoint).toBe('index.js');
    });

    it('should include languageVersion from engines.node', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: {},
          engines: { node: '>=18.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.languageVersion).toBe('>=18.0.0');
    });

    it('should handle project without framework', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(
        JSON.stringify({
          name: 'test-app',
          dependencies: { lodash: '^4.0.0' },
        }),
      );

      const result = await parsePackageJson('/test/package.json');
      expect(result.framework).toBeUndefined();
      expect(result.ports).toContain(3000);
    });

    it('should throw error on invalid JSON', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue('{ invalid json');

      await expect(parsePackageJson('/test/package.json')).rejects.toThrow(
        'Failed to parse package.json',
      );
    });

    it('should throw error on file read failure', async () => {
      (fs.readFile as jest.Mock).mockRejectedValue(new Error('ENOENT'));

      await expect(parsePackageJson('/test/package.json')).rejects.toThrow(
        'Failed to parse package.json',
      );
    });
  });

  describe('parsePomXml', () => {
    it('should parse Quarkus project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        <project>
          <artifactId>quarkus-app</artifactId>
          <version>1.0.0</version>
          <dependencies>
            <dependency>
              <groupId>io.quarkus</groupId>
              <artifactId>quarkus-resteasy</artifactId>
            </dependency>
          </dependencies>
        </project>
      `);

      const result = await parsePomXml('/test/pom.xml');
      expect(result.framework).toBe('quarkus');
      expect(result.ports).toContain(8080);
    });

    it('should parse Micronaut project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        <project>
          <artifactId>micronaut-app</artifactId>
          <version>1.0.0</version>
          <dependencies>
            <dependency>
              <groupId>io.micronaut</groupId>
              <artifactId>micronaut-http-server-netty</artifactId>
            </dependency>
          </dependencies>
        </project>
      `);

      const result = await parsePomXml('/test/pom.xml');
      expect(result.framework).toBe('micronaut');
      expect(result.ports).toContain(8080);
    });

    it('should parse Jakarta EE project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        <project>
          <artifactId>jakarta-app</artifactId>
          <version>1.0.0</version>
          <dependencies>
            <dependency>
              <groupId>jakarta.ee</groupId>
              <artifactId>jakarta-api</artifactId>
            </dependency>
          </dependencies>
        </project>
      `);

      const result = await parsePomXml('/test/pom.xml');
      expect(result.framework).toBe('jakarta-ee');
    });

    it('should extract Java version from maven.compiler.source', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        <project>
          <artifactId>test-app</artifactId>
          <properties>
            <maven.compiler.source>17</maven.compiler.source>
          </properties>
        </project>
      `);

      const result = await parsePomXml('/test/pom.xml');
      expect(result.languageVersion).toBe('17');
    });

    it('should extract Java version from maven.compiler.target', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        <project>
          <artifactId>test-app</artifactId>
          <properties>
            <maven.compiler.target>11</maven.compiler.target>
          </properties>
        </project>
      `);

      const result = await parsePomXml('/test/pom.xml');
      expect(result.languageVersion).toBe('11');
    });

    it('should include modelVersion in buildSystem', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        <project>
          <modelVersion>4.0.0</modelVersion>
          <artifactId>test-app</artifactId>
          <version>1.0.0</version>
        </project>
      `);

      const result = await parsePomXml('/test/pom.xml');
      expect(result.buildSystem?.version).toBe('4.0.0');
    });

    it('should handle project without framework', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        <project>
          <artifactId>plain-java-app</artifactId>
          <version>1.0.0</version>
          <dependencies>
            <dependency>
              <groupId>commons-lang</groupId>
              <artifactId>commons-lang</artifactId>
            </dependency>
          </dependencies>
        </project>
      `);

      const result = await parsePomXml('/test/pom.xml');
      expect(result.framework).toBeUndefined();
      expect(result.language).toBe('java');
    });

    it('should throw error on invalid XML', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue('<project><unclosed>');

      await expect(parsePomXml('/test/pom.xml')).rejects.toThrow('Failed to parse pom.xml');
    });
  });

  describe('parseGradle', () => {
    it('should parse Quarkus Gradle project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        plugins {
          id 'io.quarkus'
        }
        dependencies {
          implementation 'io.quarkus:quarkus-resteasy'
        }
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.framework).toBe('quarkus');
    });

    it('should parse Micronaut Gradle project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        plugins {
          id 'io.micronaut.application'
        }
        dependencies {
          implementation 'io.micronaut:micronaut-http-server-netty'
        }
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.framework).toBe('micronaut');
    });

    it('should extract Java version from languageVersion', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        java {
          toolchain {
            languageVersion = JavaLanguageVersion.of(21)
          }
        }
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.languageVersion).toBe('21');
    });

    it('should extract Java version from sourceCompatibility', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        sourceCompatibility = '17'
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.languageVersion).toBe('17');
    });

    it('should extract Java version from targetCompatibility', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        targetCompatibility = 11
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.languageVersion).toBe('11');
    });

    it('should prioritize languageVersion over sourceCompatibility', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        java {
          toolchain {
            languageVersion = JavaLanguageVersion.of(21)
          }
        }
        sourceCompatibility = '17'
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.languageVersion).toBe('21');
    });

    it('should extract dependencies', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
        dependencies {
          implementation 'org.springframework.boot:spring-boot-starter-web'
          implementation 'com.google.guava:guava:31.0-jre'
        }
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.dependencies).toContain('org.springframework.boot:spring-boot-starter-web');
      expect(result.dependencies).toContain('com.google.guava:guava:31.0-jre');
    });

    it('should limit dependencies to 20', async () => {
      const deps = Array.from({ length: 25 }, (_, i) => `implementation 'dep:dep${i}:1.0'`).join(
        '\n',
      );
      (fs.readFile as jest.Mock).mockResolvedValue(`
        dependencies {
          ${deps}
        }
      `);

      const result = await parseGradle('/test/build.gradle');
      expect(result.dependencies?.length).toBeLessThanOrEqual(20);
    });

    it('should throw error on file read failure', async () => {
      (fs.readFile as jest.Mock).mockRejectedValue(new Error('ENOENT'));

      await expect(parseGradle('/test/build.gradle')).rejects.toThrow(
        'Failed to parse build.gradle',
      );
    });
  });

  describe('parsePythonConfig', () => {
    it('should parse Django project in requirements.txt', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
django==4.2.0
psycopg2==2.9.0
      `);

      const result = await parsePythonConfig('/test/requirements.txt');
      expect(result.framework).toBe('django');
      expect(result.ports).toContain(8000);
    });

    it('should parse Flask project in requirements.txt', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
flask==2.3.0
gunicorn==20.1.0
      `);

      const result = await parsePythonConfig('/test/requirements.txt');
      expect(result.framework).toBe('flask');
      expect(result.ports).toContain(5000);
    });

    it('should parse FastAPI project in requirements.txt', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
fastapi==0.100.0
uvicorn==0.23.0
      `);

      const result = await parsePythonConfig('/test/requirements.txt');
      expect(result.framework).toBe('fastapi');
      expect(result.ports).toContain(8000);
    });

    it('should parse Django project in pyproject.toml', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[project]
dependencies = ["django>=4.0", "psycopg2-binary"]
requires_python = ">=3.11"
      `);

      const result = await parsePythonConfig('/test/pyproject.toml');
      expect(result.framework).toBe('django');
      expect(result.languageVersion).toBe('>=3.11');
    });

    it('should parse Flask project in pyproject.toml', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[project]
dependencies = ["flask>=2.0"]
      `);

      const result = await parsePythonConfig('/test/pyproject.toml');
      expect(result.framework).toBe('flask');
    });

    it('should parse FastAPI project in pyproject.toml', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[project]
dependencies = ["fastapi>=0.100"]
      `);

      const result = await parsePythonConfig('/test/pyproject.toml');
      expect(result.framework).toBe('fastapi');
    });

    it('should parse Tornado project in pyproject.toml', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[project]
dependencies = ["tornado>=6.0"]
      `);

      const result = await parsePythonConfig('/test/pyproject.toml');
      expect(result.framework).toBe('tornado');
    });

    it('should filter comments in requirements.txt', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
# This is a comment
flask==2.3.0
# Another comment
requests==2.31.0
      `);

      const result = await parsePythonConfig('/test/requirements.txt');
      expect(result.dependencies).not.toContain('# This is a comment');
      expect(result.dependencies).toContain('flask==2.3.0');
    });

    it('should filter empty lines in requirements.txt', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
flask==2.3.0

requests==2.31.0
      `);

      const result = await parsePythonConfig('/test/requirements.txt');
      expect(result.dependencies?.filter((d) => d === '')).toHaveLength(0);
    });

    it('should use pip buildSystem for requirements.txt', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue('flask==2.3.0');

      const result = await parsePythonConfig('/test/requirements.txt');
      expect(result.buildSystem?.type).toBe('pip');
    });

    it('should use poetry buildSystem for pyproject.toml', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue('[project]\ndependencies = []');

      const result = await parsePythonConfig('/test/pyproject.toml');
      expect(result.buildSystem?.type).toBe('poetry');
    });

    it('should throw error on invalid TOML', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue('[project\n invalid toml');

      await expect(parsePythonConfig('/test/pyproject.toml')).rejects.toThrow(
        'Failed to parse Python config',
      );
    });
  });

  describe('parseCargoToml', () => {
    it('should parse Actix-web project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[package]
name = "actix-app"
edition = "2021"

[dependencies]
actix-web = "4.0"
      `);

      const result = await parseCargoToml('/test/Cargo.toml');
      expect(result.framework).toBe('actix-web');
      expect(result.ports).toContain(8080);
    });

    it('should parse Rocket project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[dependencies]
rocket = "0.5"
      `);

      const result = await parseCargoToml('/test/Cargo.toml');
      expect(result.framework).toBe('rocket');
    });

    it('should parse Warp project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[dependencies]
warp = "0.3"
      `);

      const result = await parseCargoToml('/test/Cargo.toml');
      expect(result.framework).toBe('warp');
    });

    it('should parse Axum project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[dependencies]
axum = "0.6"
      `);

      const result = await parseCargoToml('/test/Cargo.toml');
      expect(result.framework).toBe('axum');
    });

    it('should extract rust-version', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[package]
name = "rust-app"
rust-version = "1.70"
      `);

      const result = await parseCargoToml('/test/Cargo.toml');
      expect(result.languageVersion).toBe('1.70');
    });

    it('should use edition as fallback for rust version', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[package]
name = "rust-app"
edition = "2021"
      `);

      const result = await parseCargoToml('/test/Cargo.toml');
      expect(result.languageVersion).toBe('2021');
    });

    it('should handle project without framework', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
[package]
name = "cli-app"

[dependencies]
clap = "4.0"
      `);

      const result = await parseCargoToml('/test/Cargo.toml');
      expect(result.framework).toBeUndefined();
      expect(result.ports).toEqual([]);
    });

    it('should throw error on invalid TOML', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue('[package\n invalid');

      await expect(parseCargoToml('/test/Cargo.toml')).rejects.toThrow(
        'Failed to parse Cargo.toml',
      );
    });
  });

  describe('parseCsProj', () => {
    it('should parse ASP.NET Core project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
<Project>
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.App" />
  </ItemGroup>
</Project>
      `);

      const result = await parseCsProj('/test/app.csproj');
      expect(result.framework).toBe('aspnet-core');
      expect(result.ports).toContain(5000);
      expect(result.ports).toContain(5001);
    });

    it('should parse Entity Framework project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
<Project>
  <ItemGroup>
    <PackageReference Include="Microsoft.EntityFrameworkCore" />
  </ItemGroup>
</Project>
      `);

      const result = await parseCsProj('/test/app.csproj');
      expect(result.framework).toBe('entity-framework');
    });

    it('should extract target framework', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
<Project>
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
  </PropertyGroup>
</Project>
      `);

      const result = await parseCsProj('/test/app.csproj');
      expect(result.languageVersion).toBe('net7.0');
    });

    it('should handle project without framework', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
<Project>
  <ItemGroup>
    <PackageReference Include="Newtonsoft.Json" />
  </ItemGroup>
</Project>
      `);

      const result = await parseCsProj('/test/app.csproj');
      expect(result.framework).toBeUndefined();
      expect(result.ports).toContain(5000);
    });

    it('should throw error on invalid XML', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue('<Project><unclosed>');

      await expect(parseCsProj('/test/app.csproj')).rejects.toThrow('Failed to parse .csproj');
    });
  });

  describe('parseGoMod', () => {
    it('should parse Gin project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
module myapp

go 1.21

require (
  github.com/gin-gonic/gin v1.9.0
)
      `);

      const result = await parseGoMod('/test/go.mod');
      expect(result.framework).toBe('gin');
      expect(result.languageVersion).toBe('1.21');
    });

    it('should parse Echo project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
module myapp

require (
  github.com/labstack/echo v4.11.0
)
      `);

      const result = await parseGoMod('/test/go.mod');
      expect(result.framework).toBe('echo');
    });

    it('should parse Fiber project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
require (
  github.com/gofiber/fiber/v2 v2.50.0
)
      `);

      const result = await parseGoMod('/test/go.mod');
      expect(result.framework).toBe('fiber');
    });

    it('should parse Gorilla project', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
require (
  github.com/gorilla/mux v1.8.0
)
      `);

      const result = await parseGoMod('/test/go.mod');
      expect(result.framework).toBe('gorilla');
    });

    it('should extract go version', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
module myapp

go 1.22
      `);

      const result = await parseGoMod('/test/go.mod');
      expect(result.languageVersion).toBe('1.22');
    });

    it('should limit dependencies to 20', async () => {
      const deps = Array.from({ length: 25 }, (_, i) => `  github.com/dep${i} v1.0.0`).join('\n');
      (fs.readFile as jest.Mock).mockResolvedValue(`
module myapp

require (
${deps}
)
      `);

      const result = await parseGoMod('/test/go.mod');
      expect(result.dependencies?.length).toBeLessThanOrEqual(20);
    });

    it('should handle project without framework', async () => {
      (fs.readFile as jest.Mock).mockResolvedValue(`
module myapp

require (
  github.com/spf13/cobra v1.7.0
)
      `);

      const result = await parseGoMod('/test/go.mod');
      expect(result.framework).toBeUndefined();
      expect(result.ports).toContain(8080);
    });

    it('should throw error on file read failure', async () => {
      (fs.readFile as jest.Mock).mockRejectedValue(new Error('ENOENT'));

      await expect(parseGoMod('/test/go.mod')).rejects.toThrow('Failed to parse go.mod');
    });
  });
});
