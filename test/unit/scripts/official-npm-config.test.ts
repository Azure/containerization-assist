import { afterEach, describe, expect, it } from '@jest/globals';
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import * as tar from 'tar';
import { parse } from 'yaml';

const template: { steps: Array<{ bash?: string }> } = parse(
  readFileSync('.azure-pipelines/templates/npm-ci.yml', 'utf8'),
);
const configureScript = template.steps[0]?.bash;
if (!configureScript) {
  throw new Error('Missing npm feed configuration step');
}

describe('official npm configuration', () => {
  const directories: string[] = [];

  afterEach(() => {
    for (const directory of directories.splice(0)) {
      rmSync(directory, { recursive: true, force: true });
    }
  });

  const configure = (registry: string) => {
    const directory = mkdtempSync(join(tmpdir(), 'ca-npm-config-'));
    directories.push(directory);
    const config = join(directory, 'official.npmrc');
    const env = {
      ...process.env,
      NPM_CONFIG_REGISTRY: registry,
      NPM_CONFIG_USERCONFIG: config,
    };
    const result = spawnSync('bash', ['-c', configureScript], {
      env,
      encoding: 'utf8',
    });
    return { result, config, env };
  };

  it.each([
    'https://pkgs.dev.azure.com/example/project/_packaging/approved/npm/registry/',
    'https://pkgs.dev.azure.com/example/_packaging/approved/npm/registry/',
    'https://example.pkgs.visualstudio.com/_packaging/approved/npm/registry/',
  ])('configures an isolated npm user file for %s', (registry) => {
    const { result, config, env } = configure(registry);
    expect(result.status).toBe(0);
    expect(readFileSync(config, 'utf8')).toBe(`registry=${registry}\n`);
    if (process.platform !== 'win32') {
      expect(statSync(config).mode & 0o777).toBe(0o600);
    }
    const npm = spawnSync('npm', ['config', 'get', 'registry'], {
      env,
      encoding: 'utf8',
      shell: process.platform === 'win32',
    });
    expect(npm.status).toBe(0);
    expect(npm.stdout.trim()).toBe(registry);
  });

  it.each([
    '',
    '$(ApprovedNpmRegistry)',
    'https://registry.npmjs.org/',
    'http://pkgs.dev.azure.com/example/_packaging/approved/npm/registry/',
    'https://pkgs.dev.azure.com/example/_packaging/approved/npm/registry/?token=secret',
    'https://user:secret@pkgs.dev.azure.com/example/_packaging/approved/npm/registry/',
  ])(
    'rejects missing or unsuitable feed configuration without exposing credentials',
    (registry) => {
      const { result } = configure(registry);
      expect(result.status).not.toBe(0);
      expect(result.stderr).toMatch(/npmRegistry/);
      expect(result.stdout + result.stderr).not.toContain('secret');
    },
  );

  it('restores a public lockfile through the configured registry without rewriting it', async () => {
    const directory = mkdtempSync(join(tmpdir(), 'ca-npm-routing-'));
    directories.push(directory);
    const packageDirectory = join(directory, 'package');
    mkdirSync(packageDirectory);
    const dependency = { name: 'ca-feed-routing-fixture', version: '1.0.0' };
    writeFileSync(join(packageDirectory, 'package.json'), JSON.stringify(dependency));
    const archive = join(directory, 'fixture.tgz');
    await tar.c({ gzip: true, cwd: directory, file: archive }, ['package']);
    const requests: string[] = [];
    const server = createServer((request, response) => {
      requests.push(request.url ?? '');
      response.end(readFileSync(archive));
    });
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    try {
      const address = server.address();
      if (!address || typeof address === 'string') {
        throw new Error('Missing fixture registry address');
      }
      const project = join(directory, 'consumer');
      mkdirSync(project);
      writeFileSync(
        join(project, 'package.json'),
        JSON.stringify({
          name: 'consumer',
          version: '1.0.0',
          dependencies: { [dependency.name]: '1.0.0' },
        }),
      );
      const lockfile = JSON.stringify({
        name: 'consumer',
        version: '1.0.0',
        lockfileVersion: 3,
        packages: {
          '': { name: 'consumer', version: '1.0.0', dependencies: { [dependency.name]: '1.0.0' } },
          [`node_modules/${dependency.name}`]: {
            version: '1.0.0',
            resolved: `https://registry.npmjs.org/${dependency.name}/-/${dependency.name}-1.0.0.tgz`,
          },
        },
      });
      writeFileSync(join(project, 'package-lock.json'), lockfile);
      const config = join(directory, 'empty.npmrc');
      writeFileSync(config, '');
      await new Promise<void>((resolve, reject) => {
        const child = spawn('npm', ['ci', '--ignore-scripts', '--no-audit', '--no-fund'], {
          cwd: project,
          shell: process.platform === 'win32',
          env: {
            ...process.env,
            NPM_CONFIG_REGISTRY: `http://127.0.0.1:${address.port}/npm/registry/`,
            NPM_CONFIG_USERCONFIG: config,
            NPM_CONFIG_REPLACE_REGISTRY_HOST: 'npmjs',
            NPM_CONFIG_CACHE: join(directory, 'cache'),
          },
        });
        let output = '';
        child.stdout.on('data', (data) => {
          output += data;
        });
        child.stderr.on('data', (data) => {
          output += data;
        });
        child.on('error', reject);
        child.on('close', (code) => {
          if (code === 0) resolve();
          else reject(new Error(`Fixture npm ci failed (${code}): ${output}`));
        });
      });
      expect(requests).toContain(`/npm/registry/${dependency.name}/-/${dependency.name}-1.0.0.tgz`);
      expect(readFileSync(join(project, 'package-lock.json'), 'utf8')).toBe(lockfile);
      expect(
        JSON.parse(
          readFileSync(join(project, 'node_modules', dependency.name, 'package.json'), 'utf8'),
        ),
      ).toEqual(dependency);
    } finally {
      await new Promise<void>((resolve, reject) => {
        server.close((error) => {
          if (error) reject(error);
          else resolve();
        });
      });
    }
  }, 30000);
});
