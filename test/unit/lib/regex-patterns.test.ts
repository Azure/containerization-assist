/**
 * Tests for centralized regex patterns
 */

import {
  AS_CLAUSE,
  LATEST_TAG,
  SUDO_INSTALL,
  PACKAGE_FILES,
  PASSWORD_PATTERN,
  API_KEY_PATTERN,
  SECRET_PATTERN,
  TOKEN_PATTERN,
} from '@/lib/regex-patterns';

describe('regex-patterns', () => {
  describe('AS_CLAUSE', () => {
    it('should match Docker multi-stage AS clause', () => {
      expect(' AS builder').toMatch(AS_CLAUSE);
      expect(' as builder').toMatch(AS_CLAUSE);
      expect(' As builder').toMatch(AS_CLAUSE);
      expect(' aS builder').toMatch(AS_CLAUSE);
    });

    it('should not match without proper spacing', () => {
      expect('ASbuilder').not.toMatch(AS_CLAUSE);
      expect('as builder').not.toMatch(AS_CLAUSE); // no leading space
    });

    it('should match in multi-stage Dockerfile context', () => {
      const line1 = 'FROM node:20 AS builder';
      const line2 = 'FROM alpine:latest  AS  runtime';

      expect(line1).toMatch(AS_CLAUSE);
      expect(line2).toMatch(AS_CLAUSE);
    });
  });

  describe('LATEST_TAG', () => {
    it('should match :latest tag', () => {
      expect(':latest').toMatch(LATEST_TAG);
      expect(':latest ').toMatch(LATEST_TAG);
      expect(':latest\n').toMatch(LATEST_TAG);
    });

    it('should not match latest in other contexts', () => {
      expect('latest').not.toMatch(LATEST_TAG);
      expect('node:latest-alpine').not.toMatch(LATEST_TAG);
      expect(':latestversion').not.toMatch(LATEST_TAG);
    });

    it('should match in Docker image references', () => {
      const image1 = 'FROM nginx:latest';
      const image2 = 'node:latest ';
      const image3 = 'alpine:latest\n';

      expect(image1).toMatch(LATEST_TAG);
      expect(image2).toMatch(LATEST_TAG);
      expect(image3).toMatch(LATEST_TAG);
    });
  });

  describe('SUDO_INSTALL', () => {
    it('should match sudo in install commands', () => {
      expect('apt-get install sudo').toMatch(SUDO_INSTALL);
      expect('yum install sudo wget').toMatch(SUDO_INSTALL);
      expect('apk add sudo curl').toMatch(SUDO_INSTALL);
    });

    it('should match install with sudo', () => {
      expect('install curl sudo').toMatch(SUDO_INSTALL);
    });

    it('should be case-sensitive', () => {
      expect('INSTALL SUDO').not.toMatch(SUDO_INSTALL);
      // Pattern matches 'sudo' (lowercase) so this actually matches
      expect('install SUDO').not.toMatch(SUDO_INSTALL);
    });

    it('should not match sudo alone', () => {
      expect('sudo').not.toMatch(SUDO_INSTALL);
      expect('apt-get install curl').not.toMatch(SUDO_INSTALL);
    });

    it('should match in typical Dockerfile RUN commands', () => {
      const cmd1 = 'RUN apt-get update && apt-get install -y sudo curl';
      const cmd2 = 'RUN yum install sudo';
      const cmd3 = 'RUN apk add sudo bash';

      expect(cmd1).toMatch(SUDO_INSTALL);
      expect(cmd2).toMatch(SUDO_INSTALL);
      expect(cmd3).toMatch(SUDO_INSTALL);
    });
  });

  describe('PACKAGE_FILES', () => {
    it('should match package.json', () => {
      expect('package.json').toMatch(PACKAGE_FILES);
      expect('package-lock.json').toMatch(PACKAGE_FILES);
    });

    it('should match requirements.txt', () => {
      expect('requirements.txt').toMatch(PACKAGE_FILES);
      // Pattern requires exact 'requirements.txt', not variations
      expect('requirements-dev.txt').not.toMatch(PACKAGE_FILES);
    });

    it('should match go.mod', () => {
      expect('go.mod').toMatch(PACKAGE_FILES);
    });

    it('should match pom.xml', () => {
      expect('pom.xml').toMatch(PACKAGE_FILES);
    });

    it('should not match unrelated files', () => {
      expect('index.js').not.toMatch(PACKAGE_FILES);
      expect('README.md').not.toMatch(PACKAGE_FILES);
      expect('Dockerfile').not.toMatch(PACKAGE_FILES);
    });

    it('should match in file paths', () => {
      expect('/app/package.json').toMatch(PACKAGE_FILES);
      expect('./requirements.txt').toMatch(PACKAGE_FILES);
      expect('/project/go.mod').toMatch(PACKAGE_FILES);
      expect('src/main/pom.xml').toMatch(PACKAGE_FILES);
    });
  });

  describe('PASSWORD_PATTERN', () => {
    it('should match password assignments', () => {
      expect('password=secret123').toMatch(PASSWORD_PATTERN);
      expect('password = "secret"').toMatch(PASSWORD_PATTERN);
      expect("password='secret'").toMatch(PASSWORD_PATTERN);
      expect('db_password=abc123').toMatch(PASSWORD_PATTERN);
      expect('userpassword=test').toMatch(PASSWORD_PATTERN);
    });

    it('should be case-insensitive', () => {
      expect('PASSWORD=secret').toMatch(PASSWORD_PATTERN);
      expect('Password=secret').toMatch(PASSWORD_PATTERN);
      expect('PaSsWoRd=secret').toMatch(PASSWORD_PATTERN);
    });

    it('should match various password formats', () => {
      expect('DB_PASSWORD=mypass').toMatch(PASSWORD_PATTERN);
      expect('root_password="complex!pass"').toMatch(PASSWORD_PATTERN);
      expect('adminPassword = value').toMatch(PASSWORD_PATTERN);
    });

    it('should not match unrelated content', () => {
      expect('username=admin').not.toMatch(PASSWORD_PATTERN);
      expect('port=3306').not.toMatch(PASSWORD_PATTERN);
    });
  });

  describe('API_KEY_PATTERN', () => {
    it('should match API key assignments', () => {
      expect('api_key=abc123').toMatch(API_KEY_PATTERN);
      expect('api-key=xyz789').toMatch(API_KEY_PATTERN);
      expect('apikey=secret').toMatch(API_KEY_PATTERN);
      expect('API_KEY="value"').toMatch(API_KEY_PATTERN);
    });

    it('should be case-insensitive', () => {
      expect('API_KEY=secret').toMatch(API_KEY_PATTERN);
      expect('Api_Key=secret').toMatch(API_KEY_PATTERN);
      expect('api_key=secret').toMatch(API_KEY_PATTERN);
    });

    it('should match various API key formats', () => {
      expect('stripe_api_key=sk_test_123').toMatch(API_KEY_PATTERN);
      expect('aws-api-key=AKIA123').toMatch(API_KEY_PATTERN);
      expect('GOOGLE_API_KEY = "AIza123"').toMatch(API_KEY_PATTERN);
    });

    it('should match with underscores or hyphens', () => {
      expect('my_api_key=value').toMatch(API_KEY_PATTERN);
      expect('my-api-key=value').toMatch(API_KEY_PATTERN);
      expect('myapikey=value').toMatch(API_KEY_PATTERN);
    });
  });

  describe('SECRET_PATTERN', () => {
    it('should match secret assignments', () => {
      expect('secret=value').toMatch(SECRET_PATTERN);
      expect('client_secret="xyz"').toMatch(SECRET_PATTERN);
      expect('app_secret=abc').toMatch(SECRET_PATTERN);
    });

    it('should be case-insensitive', () => {
      expect('SECRET=value').toMatch(SECRET_PATTERN);
      expect('Secret=value').toMatch(SECRET_PATTERN);
      expect('sEcReT=value').toMatch(SECRET_PATTERN);
    });

    it('should match various secret formats', () => {
      expect('JWT_SECRET=mysecret').toMatch(SECRET_PATTERN);
      expect('oauth_client_secret="secret"').toMatch(SECRET_PATTERN);
      expect('SESSION_SECRET=random').toMatch(SECRET_PATTERN);
    });
  });

  describe('TOKEN_PATTERN', () => {
    it('should match token assignments', () => {
      expect('token=abc123').toMatch(TOKEN_PATTERN);
      expect('access_token="xyz"').toMatch(TOKEN_PATTERN);
      expect('auth_token=value').toMatch(TOKEN_PATTERN);
    });

    it('should be case-insensitive', () => {
      expect('TOKEN=value').toMatch(TOKEN_PATTERN);
      expect('Token=value').toMatch(TOKEN_PATTERN);
      expect('ToKeN=value').toMatch(TOKEN_PATTERN);
    });

    it('should match various token formats', () => {
      expect('GITHUB_TOKEN=ghp_123').toMatch(TOKEN_PATTERN);
      expect('refresh_token="rt_xyz"').toMatch(TOKEN_PATTERN);
      expect('bearer_token = jwt123').toMatch(TOKEN_PATTERN);
    });
  });

  describe('secret detection patterns integration', () => {
    it('should detect secrets in environment variable assignments', () => {
      const envVars = [
        'DB_PASSWORD=secretpass',
        'API_KEY=abc123',
        'JWT_SECRET=mysecret',
        'AUTH_TOKEN=token123',
      ];

      expect(envVars[0]).toMatch(PASSWORD_PATTERN);
      expect(envVars[1]).toMatch(API_KEY_PATTERN);
      expect(envVars[2]).toMatch(SECRET_PATTERN);
      expect(envVars[3]).toMatch(TOKEN_PATTERN);
    });

    it('should detect secrets in Dockerfile ENV commands', () => {
      const envCommands = [
        'ENV DB_PASSWORD=secretpass',
        'ENV API_KEY="abc123"',
        'ENV SECRET_KEY=mysecret',
        'ENV ACCESS_TOKEN=token',
      ];

      expect(envCommands[0]).toMatch(PASSWORD_PATTERN);
      expect(envCommands[1]).toMatch(API_KEY_PATTERN);
      expect(envCommands[2]).toMatch(SECRET_PATTERN);
      expect(envCommands[3]).toMatch(TOKEN_PATTERN);
    });

    it('should not match safe configuration', () => {
      const safeConfig = [
        'PORT=3000',
        'NODE_ENV=production',
        'LOG_LEVEL=info',
        'DATABASE_URL=postgres://localhost',
      ];

      safeConfig.forEach((config) => {
        expect(config).not.toMatch(PASSWORD_PATTERN);
        expect(config).not.toMatch(API_KEY_PATTERN);
        expect(config).not.toMatch(SECRET_PATTERN);
        expect(config).not.toMatch(TOKEN_PATTERN);
      });
    });
  });

  describe('pattern immutability', () => {
    it('should have global flag when appropriate', () => {
      // Some patterns might need global flag for multiple matches
      // This test documents the current state
      expect(AS_CLAUSE.global).toBe(false);
      expect(LATEST_TAG.global).toBe(false);
    });

    it('should have case-insensitive flag where needed', () => {
      expect(AS_CLAUSE.ignoreCase).toBe(true);
      expect(PASSWORD_PATTERN.ignoreCase).toBe(true);
      expect(API_KEY_PATTERN.ignoreCase).toBe(true);
      expect(SECRET_PATTERN.ignoreCase).toBe(true);
      expect(TOKEN_PATTERN.ignoreCase).toBe(true);
    });

    it('should be reusable across multiple test calls', () => {
      // Test that patterns can be reused
      expect('password=test1').toMatch(PASSWORD_PATTERN);
      expect('password=test2').toMatch(PASSWORD_PATTERN);
      expect('password=test3').toMatch(PASSWORD_PATTERN);

      expect(':latest').toMatch(LATEST_TAG);
      expect(':latest ').toMatch(LATEST_TAG);
      expect(':latest\n').toMatch(LATEST_TAG);
    });
  });
});
