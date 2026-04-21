import { describe, it, expect, beforeEach, afterEach, jest } from '@jest/globals';
import { mkdirSync, writeFileSync, rmSync, existsSync } from 'node:fs';
import os from 'node:os';
import { join, resolve } from 'node:path';
import { createLogger } from '@/lib/logger';
import {
  discoverBuiltInPolicies,
  discoverGlobalPolicies,
  discoverProjectPolicies,
  discoverUserPolicies,
  discoverCustomPolicies,
  discoverPolicies,
  discoverPolicyPaths,
} from '@/app/orchestrator';
import {
  ENV_VARS,
  POLICY_GLOBAL_APP_NAME,
  POLICY_PROJECT_DIR,
  POLICY_SUBDIR,
} from '@/config/constants';

describe('Policy Discovery', () => {
  let testDir: string;
  let logger: ReturnType<typeof createLogger>;
  let originalCwd: string;
  let originalEnv: string | undefined;
  let originalXdg: string | undefined;

  beforeEach(() => {
    testDir = join(
      os.tmpdir(),
      `policy-discovery-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    );
    mkdirSync(testDir, { recursive: true });
    logger = createLogger({ name: 'test', level: 'silent' });
    originalCwd = process.cwd();
    originalEnv = process.env[ENV_VARS.CUSTOM_POLICY_PATH];
    originalXdg = process.env.XDG_CONFIG_HOME;
  });

  afterEach(() => {
    process.chdir(originalCwd);
    if (existsSync(testDir)) {
      rmSync(testDir, { recursive: true, force: true });
    }
    if (originalEnv !== undefined) {
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = originalEnv;
    } else {
      delete process.env[ENV_VARS.CUSTOM_POLICY_PATH];
    }
    if (originalXdg !== undefined) {
      process.env.XDG_CONFIG_HOME = originalXdg;
    } else {
      delete process.env.XDG_CONFIG_HOME;
    }
  });

  describe('discoverBuiltInPolicies', () => {
    it('discovers built-in policies from policies directory', () => {
      const policies = discoverBuiltInPolicies(logger);
      expect(policies.length).toBeGreaterThanOrEqual(3);
      expect(policies.some((p) => p.endsWith('security-baseline.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('base-images.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('container-best-practices.rego'))).toBe(true);
    });

    it('excludes built-in test files', () => {
      const policies = discoverBuiltInPolicies(logger);
      expect(policies.every((p) => !p.endsWith('_test.rego'))).toBe(true);
    });
  });

  describe('discoverGlobalPolicies', () => {
    it('uses XDG_CONFIG_HOME path when set', () => {
      const xdgHome = join(testDir, 'xdg');
      const globalDir = join(xdgHome, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);
      mkdirSync(globalDir, { recursive: true });
      writeFileSync(join(globalDir, 'global.rego'), 'package g\ndefault allow := true');
      process.env.XDG_CONFIG_HOME = xdgHome;

      const policies = discoverGlobalPolicies(logger);

      expect(policies).toContain(resolve(join(globalDir, 'global.rego')));
    });

    it('falls back to homedir .config when XDG_CONFIG_HOME is unset', () => {
      delete process.env.XDG_CONFIG_HOME;
      const homeSpy = jest.spyOn(os, 'homedir').mockReturnValue(join(testDir, 'fake-home'));

      const policies = discoverGlobalPolicies(logger);

      expect(Array.isArray(policies)).toBe(true);
      homeSpy.mockRestore();
    });

    it('returns rego files from directory and excludes test files', () => {
      const xdgHome = join(testDir, 'xdg');
      const globalDir = join(xdgHome, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);
      mkdirSync(globalDir, { recursive: true });
      writeFileSync(join(globalDir, 'a.rego'), 'package a\ndefault allow := true');
      writeFileSync(join(globalDir, 'b.rego'), 'package b\ndefault allow := true');
      writeFileSync(join(globalDir, 'skip_test.rego'), 'package skip\ndefault allow := true');
      process.env.XDG_CONFIG_HOME = xdgHome;

      const policies = discoverGlobalPolicies(logger);

      expect(policies.length).toBe(2);
      expect(policies.some((p) => p.endsWith('a.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('b.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('skip_test.rego'))).toBe(false);
    });

    it('returns empty silently when directory does not exist', () => {
      process.env.XDG_CONFIG_HOME = join(testDir, 'missing-xdg');
      const warnSpy = jest.spyOn(logger, 'warn');

      const policies = discoverGlobalPolicies(logger);

      expect(policies).toEqual([]);
      expect(warnSpy).not.toHaveBeenCalled();
      warnSpy.mockRestore();
    });

    it('returns empty on inaccessible path and logs warning', () => {
      delete process.env.XDG_CONFIG_HOME;
      const warnSpy = jest.spyOn(logger, 'warn');
      const osModule = require('node:os') as typeof import('node:os');
      const homeSpy = jest.spyOn(osModule, 'homedir').mockImplementation(() => {
        throw new Error('homedir-failed');
      });

      const policies = discoverGlobalPolicies(logger);

      expect(policies).toEqual([]);
      expect(warnSpy).toHaveBeenCalled();
      homeSpy.mockRestore();
      warnSpy.mockRestore();
    });
  });

  describe('discoverProjectPolicies', () => {
    it('returns policies when project directory exists in workspacePath', () => {
      const repo = join(testDir, 'repo');
      const projectDir = join(repo, POLICY_PROJECT_DIR, POLICY_SUBDIR);
      mkdirSync(projectDir, { recursive: true });
      writeFileSync(join(projectDir, 'project.rego'), 'package p\ndefault allow := true');

      const policies = discoverProjectPolicies(logger, repo);

      expect(policies.length).toBe(1);
      const expectedSuffix = join(POLICY_PROJECT_DIR, POLICY_SUBDIR, 'project.rego');
      expect(policies[0].endsWith(expectedSuffix)).toBe(true);
    });

    it('returns empty when workspacePath has no project directory', () => {
      const repo = join(testDir, 'repo');
      mkdirSync(repo, { recursive: true });

      const policies = discoverProjectPolicies(logger, repo);

      expect(policies).toEqual([]);
    });

    it('returns empty silently when workspacePath has no policy dir', () => {
      const dir = join(testDir, 'nogit');
      mkdirSync(dir, { recursive: true });
      const warnSpy = jest.spyOn(logger, 'warn');

      const policies = discoverProjectPolicies(logger, dir);

      expect(policies).toEqual([]);
      expect(warnSpy).not.toHaveBeenCalled();
      warnSpy.mockRestore();
    });

    it('does not walk up to parent directories', () => {
      const repo = join(testDir, 'repo');
      const nested = join(repo, 'x', 'y');
      const projectDir = join(repo, POLICY_PROJECT_DIR, POLICY_SUBDIR);
      mkdirSync(projectDir, { recursive: true });
      mkdirSync(nested, { recursive: true });
      writeFileSync(join(projectDir, 'project.rego'), 'package w\ndefault allow := true');

      // Passing nested dir as workspacePath should NOT find policies at repo level
      const policies = discoverProjectPolicies(logger, nested);

      expect(policies).toEqual([]);
    });

  });

  describe('discoverUserPolicies (deprecated)', () => {
    it('finds legacy directory and logs deprecation warning', () => {
      const repo = join(testDir, 'repo');
      const legacy = join(repo, 'policies.user');
      mkdirSync(legacy, { recursive: true });
      writeFileSync(join(legacy, 'legacy.rego'), 'package l\ndefault allow := true');
      process.chdir(repo);
      const warnSpy = jest.spyOn(logger, 'warn');

      const policies = discoverUserPolicies(logger);

      expect(policies.length).toBe(1);
      expect(policies[0].endsWith('legacy.rego')).toBe(true);
      expect(
        warnSpy.mock.calls.some(
          (call) =>
            call[0] ===
            'policies.user/ is deprecated. Move policies to .containerization-assist/policy/ at your project root, or ~/.config/containerization-assist/policy/ for global policies.',
        ),
      ).toBe(true);
      warnSpy.mockRestore();
    });

    it('logs deprecation warning once per session', () => {
      const repoA = join(testDir, 'repo-a');
      const repoB = join(testDir, 'repo-b');
      mkdirSync(join(repoA, 'policies.user'), { recursive: true });
      mkdirSync(join(repoB, 'policies.user'), { recursive: true });
      writeFileSync(join(repoA, 'policies.user', 'a.rego'), 'package a\ndefault allow := true');
      writeFileSync(join(repoB, 'policies.user', 'b.rego'), 'package b\ndefault allow := true');
      const warnSpy = jest.spyOn(logger, 'warn');

      process.chdir(repoA);
      discoverUserPolicies(logger);
      process.chdir(repoB);
      discoverUserPolicies(logger);

      const count = warnSpy.mock.calls.filter(
        (call) =>
          call[0] ===
          'policies.user/ is deprecated. Move policies to .containerization-assist/policy/ at your project root, or ~/.config/containerization-assist/policy/ for global policies.',
      ).length;
      expect(count).toBeLessThanOrEqual(1);
      warnSpy.mockRestore();
    });

    it('does not walk up to parent directories', () => {
      const repo = join(testDir, 'repo');
      const nested = join(repo, 'src', 'deep', 'tool');
      const legacy = join(repo, 'policies.user');
      mkdirSync(legacy, { recursive: true });
      mkdirSync(nested, { recursive: true });
      writeFileSync(join(legacy, 'up.rego'), 'package up\ndefault allow := true');

      // Passing nested dir as workspacePath should NOT find policies at repo level
      const policies = discoverUserPolicies(logger, nested);

      expect(policies).toEqual([]);
    });
  });

  describe('discoverCustomPolicies', () => {
    it('discovers custom directory policies and excludes tests', () => {
      const custom = join(testDir, 'custom');
      mkdirSync(custom, { recursive: true });
      writeFileSync(join(custom, 'one.rego'), 'package one\ndefault allow := true');
      writeFileSync(join(custom, 'two_test.rego'), 'package two\ndefault allow := true');

      const policies = discoverCustomPolicies(custom, logger);

      expect(policies.length).toBe(1);
      expect(policies[0].endsWith('one.rego')).toBe(true);
    });

    it('supports single rego file path', () => {
      const file = join(testDir, 'single.rego');
      writeFileSync(file, 'package s\ndefault allow := true');

      const policies = discoverCustomPolicies(file, logger);

      expect(policies).toEqual([file]);
    });
  });

  describe('discoverPolicies (priority ordering)', () => {
    it('orders built-in, global, project, custom when all tiers populated', () => {
      const xdg = join(testDir, 'xdg');
      const globalDir = join(xdg, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);
      const repo = join(testDir, 'repo');
      const projectDir = join(repo, POLICY_PROJECT_DIR, POLICY_SUBDIR);
      const custom = join(testDir, 'custom');

      mkdirSync(globalDir, { recursive: true });
      mkdirSync(join(repo, '.git'), { recursive: true });
      mkdirSync(projectDir, { recursive: true });
      mkdirSync(custom, { recursive: true });
      writeFileSync(join(globalDir, 'global.rego'), 'package g\ndefault allow := true');
      writeFileSync(join(projectDir, 'project.rego'), 'package p\ndefault allow := true');
      writeFileSync(join(custom, 'custom.rego'), 'package c\ndefault allow := true');
      process.env.XDG_CONFIG_HOME = xdg;
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = custom;
      process.chdir(repo);

      const discovered = discoverPolicies(logger);
      const index = (suffix: string) => discovered.findIndex((p) => p.path.endsWith(suffix));
      const builtInLast = Math.max(...discovered.map((p, i) => (p.source === 'built-in' ? i : -1)));

      expect(index('global.rego')).toBeGreaterThan(builtInLast);
      expect(index('project.rego')).toBeGreaterThan(index('global.rego'));
      expect(index('custom.rego')).toBeGreaterThan(index('project.rego'));
    });

    it('returns source metadata for discovered policies', () => {
      const xdg = join(testDir, 'xdg');
      const globalDir = join(xdg, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);
      const repo = join(testDir, 'repo');
      const projectDir = join(repo, POLICY_PROJECT_DIR, POLICY_SUBDIR);
      const custom = join(testDir, 'custom');

      mkdirSync(globalDir, { recursive: true });
      mkdirSync(join(repo, '.git'), { recursive: true });
      mkdirSync(projectDir, { recursive: true });
      mkdirSync(custom, { recursive: true });
      writeFileSync(join(globalDir, 'global-source.rego'), 'package g\ndefault allow := true');
      writeFileSync(join(projectDir, 'project-source.rego'), 'package p\ndefault allow := true');
      writeFileSync(join(custom, 'custom-source.rego'), 'package c\ndefault allow := true');
      process.env.XDG_CONFIG_HOME = xdg;
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = custom;
      process.chdir(repo);

      const discovered = discoverPolicies(logger);

      expect(discovered.find((p) => p.path.endsWith('global-source.rego'))?.source).toBe('global');
      expect(discovered.find((p) => p.path.endsWith('project-source.rego'))?.source).toBe(
        'project',
      );
      expect(discovered.find((p) => p.path.endsWith('custom-source.rego'))?.source).toBe('custom');
      expect(discovered.filter((p) => p.source === 'built-in').length).toBeGreaterThanOrEqual(0);
    });

    it('tags legacy policies with source legacy', () => {
      const repo = join(testDir, 'repo');
      const legacy = join(repo, 'policies.user');
      mkdirSync(legacy, { recursive: true });
      writeFileSync(join(legacy, 'legacy-source.rego'), 'package l\ndefault allow := true');
      process.chdir(repo);

      const discovered = discoverPolicies(logger);

      expect(discovered.find((p) => p.path.endsWith('legacy-source.rego'))?.source).toBe('legacy');
    });

    it('maintains ordering when global tier empty', () => {
      const repo = join(testDir, 'repo');
      const projectDir = join(repo, POLICY_PROJECT_DIR, POLICY_SUBDIR);
      const custom = join(testDir, 'custom');
      mkdirSync(join(repo, '.git'), { recursive: true });
      mkdirSync(projectDir, { recursive: true });
      mkdirSync(custom, { recursive: true });
      writeFileSync(
        join(projectDir, 'project-empty-global.rego'),
        'package p\ndefault allow := true',
      );
      writeFileSync(join(custom, 'custom-empty-global.rego'), 'package c\ndefault allow := true');
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = custom;
      process.chdir(repo);

      const discovered = discoverPolicies(logger);

      expect(
        discovered.findIndex((p) => p.path.endsWith('custom-empty-global.rego')),
      ).toBeGreaterThan(discovered.findIndex((p) => p.path.endsWith('project-empty-global.rego')));
    });

    it('maintains ordering when project tier empty', () => {
      const xdg = join(testDir, 'xdg');
      const globalDir = join(xdg, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);
      const repo = join(testDir, 'repo');
      const custom = join(testDir, 'custom');
      mkdirSync(globalDir, { recursive: true });
      mkdirSync(join(repo, '.git'), { recursive: true });
      mkdirSync(custom, { recursive: true });
      writeFileSync(
        join(globalDir, 'global-empty-project.rego'),
        'package g\ndefault allow := true',
      );
      writeFileSync(join(custom, 'custom-empty-project.rego'), 'package c\ndefault allow := true');
      process.env.XDG_CONFIG_HOME = xdg;
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = custom;
      process.chdir(repo);

      const discovered = discoverPolicies(logger);

      expect(
        discovered.findIndex((p) => p.path.endsWith('custom-empty-project.rego')),
      ).toBeGreaterThan(discovered.findIndex((p) => p.path.endsWith('global-empty-project.rego')));
    });

    it('maintains ordering when custom tier empty', () => {
      const xdg = join(testDir, 'xdg');
      const globalDir = join(xdg, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);
      const repo = join(testDir, 'repo');
      const projectDir = join(repo, POLICY_PROJECT_DIR, POLICY_SUBDIR);
      mkdirSync(globalDir, { recursive: true });
      mkdirSync(join(repo, '.git'), { recursive: true });
      mkdirSync(projectDir, { recursive: true });
      writeFileSync(
        join(globalDir, 'global-empty-custom.rego'),
        'package g\ndefault allow := true',
      );
      writeFileSync(
        join(projectDir, 'project-empty-custom.rego'),
        'package p\ndefault allow := true',
      );
      process.env.XDG_CONFIG_HOME = xdg;
      delete process.env[ENV_VARS.CUSTOM_POLICY_PATH];
      process.chdir(repo);

      const discovered = discoverPolicies(logger);

      expect(
        discovered.findIndex((p) => p.path.endsWith('project-empty-custom.rego')),
      ).toBeGreaterThan(discovered.findIndex((p) => p.path.endsWith('global-empty-custom.rego')));
      expect(discovered.some((p) => p.source === 'custom')).toBe(false);
    });

    it('returns paths only through discoverPolicyPaths helper', () => {
      const repo = join(testDir, 'repo');
      const legacy = join(repo, 'policies.user');
      mkdirSync(legacy, { recursive: true });
      writeFileSync(join(legacy, 'legacy-only.rego'), 'package l\ndefault allow := true');
      process.chdir(repo);

      const discovered = discoverPolicies(logger);
      const paths = discoverPolicyPaths(logger);

      expect(paths).toEqual(discovered.map((p) => p.path));
      expect(paths.every((p) => typeof p === 'string')).toBe(true);
    });
  });
});
