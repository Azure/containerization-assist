import type { CommandEntry } from 'docker-file-parser';
import * as dockerParser from 'docker-file-parser';
import { createLogger } from '@/lib/logger';

const logger = createLogger({ name: 'dockerfile-fixer' });

export interface FixOperation {
  ruleId: string;
  fix: (commands: CommandEntry[]) => CommandEntry[];
}

export interface FixResult {
  fixed: string;
  applied: string[];
}

/**
 * Available fixes for common Dockerfile issues
 */
const FIXES: FixOperation[] = [
  {
    ruleId: 'no-root-user',
    fix: (commands) => {
      // Only apply fix if there's a valid FROM command
      const hasFromCommand = commands.some((cmd) => cmd.name === 'FROM');
      if (!hasFromCommand) {
        return commands; // Don't modify invalid Dockerfiles
      }

      // Check if USER exists
      const hasUser = commands.some((cmd) => cmd.name === 'USER');
      if (!hasUser) {
        // Find the last instruction before CMD/ENTRYPOINT
        let lastIndex = -1;
        for (let i = commands.length - 1; i >= 0; i--) {
          const cmd = commands[i];
          if (cmd && (cmd.name === 'CMD' || cmd.name === 'ENTRYPOINT')) {
            lastIndex = i;
            break;
          }
        }
        const insertIndex = lastIndex > 0 ? lastIndex : commands.length;

        // Insert user creation and USER directive
        const newCommands = [...commands];

        // Detect base image to use appropriate user creation command
        const fromCommand = commands.find((cmd) => cmd.name === 'FROM');
        const baseImage = typeof fromCommand?.args === 'string' ? fromCommand.args : '';

        let userCreationCmd: CommandEntry;
        if (baseImage.includes('alpine')) {
          userCreationCmd = {
            name: 'RUN',
            args: 'adduser -D -u 1001 appuser',
            lineno: 0,
            raw: 'RUN adduser -D -u 1001 appuser',
          } as CommandEntry;
        } else if (baseImage.includes('debian') || baseImage.includes('ubuntu')) {
          userCreationCmd = {
            name: 'RUN',
            args: 'useradd -m -u 1001 -s /bin/sh appuser',
            lineno: 0,
            raw: 'RUN useradd -m -u 1001 -s /bin/sh appuser',
          } as CommandEntry;
        } else {
          // Generic fallback
          userCreationCmd = {
            name: 'RUN',
            args: 'useradd -m -u 1001 appuser || adduser -D -u 1001 appuser',
            lineno: 0,
            raw: 'RUN useradd -m -u 1001 appuser || adduser -D -u 1001 appuser',
          } as CommandEntry;
        }

        newCommands.splice(insertIndex, 0, userCreationCmd, {
          name: 'USER',
          args: 'appuser',
          lineno: 0,
          raw: 'USER appuser',
        } as CommandEntry);
        return newCommands;
      }
      return commands;
    },
  },
  {
    ruleId: 'specific-base-image',
    fix: (commands) => {
      return commands.map((cmd) => {
        if (cmd.name === 'FROM' && typeof cmd.args === 'string') {
          // Replace :latest with specific version
          if (cmd.args.includes(':latest')) {
            // Use knowledge pack or hardcoded defaults
            const baseImageMap: Record<string, string> = {
              'node:latest': 'node:20-alpine',
              'python:latest': 'python:3.12-slim',
              'ubuntu:latest': 'ubuntu:24.04',
              'alpine:latest': 'alpine:3.19',
              'nginx:latest': 'nginx:1.25-alpine',
              'postgres:latest': 'postgres:16-alpine',
              'redis:latest': 'redis:7-alpine',
              'golang:latest': 'golang:1.21-alpine',
              'ruby:latest': 'ruby:3.3-slim',
              'openjdk:latest': 'openjdk:21-slim',
            };

            for (const [pattern, replacement] of Object.entries(baseImageMap)) {
              if (cmd.args === pattern) {
                return { ...cmd, args: replacement };
              }
            }

            // Handle partial matches (e.g., "node:latest AS builder")
            for (const [pattern, replacement] of Object.entries(baseImageMap)) {
              if (cmd.args.includes(pattern)) {
                return { ...cmd, args: cmd.args.replace(pattern, replacement) };
              }
            }
          }
        }
        return cmd;
      });
    },
  },
  {
    ruleId: 'optimize-package-install',
    fix: (commands) => {
      return commands.map((cmd) => {
        if (cmd.name === 'RUN' && typeof cmd.args === 'string') {
          let fixed = cmd.args;
          let wasModified = false;

          // Fix apt-get patterns
          if (fixed.includes('apt-get install')) {
            // Add update if missing
            if (!fixed.includes('apt-get update')) {
              fixed = `apt-get update && ${fixed}`;
              wasModified = true;
            }

            // Add --no-install-recommends if missing
            if (!fixed.includes('--no-install-recommends')) {
              fixed = fixed.replace('apt-get install', 'apt-get install --no-install-recommends');
              wasModified = true;
            }

            // Add cleanup if missing
            if (!fixed.includes('rm -rf /var/lib/apt/lists/*')) {
              fixed = `${fixed} && rm -rf /var/lib/apt/lists/*`;
              wasModified = true;
            }
          }

          // Fix apk patterns (Alpine)
          if (fixed.includes('apk add')) {
            // Add --no-cache if missing
            if (!fixed.includes('--no-cache')) {
              fixed = fixed.replace('apk add', 'apk add --no-cache');
              wasModified = true;
            }
          }

          // Fix yum/dnf patterns
          if (fixed.includes('yum install') || fixed.includes('dnf install')) {
            // Add -y if missing
            if (!fixed.includes(' -y')) {
              fixed = fixed.replace(/(yum|dnf) install/, '$1 install -y');
              wasModified = true;
            }

            // Add cleanup if missing
            if (!fixed.includes('yum clean all') && !fixed.includes('dnf clean all')) {
              const cleanCmd = fixed.includes('yum') ? 'yum clean all' : 'dnf clean all';
              fixed = `${fixed} && ${cleanCmd}`;
              wasModified = true;
            }
          }

          return wasModified ? { ...cmd, args: fixed } : cmd;
        }
        return cmd;
      });
    },
  },
];

/**
 * Apply fixes to Dockerfile content
 *
 * @param content - Original Dockerfile content
 * @param ruleIds - List of rule IDs to apply fixes for
 * @returns Fixed content and list of applied fixes
 */
export function applyFixes(content: string, ruleIds: string[]): FixResult {
  try {
    const commands = dockerParser.parse(content);
    let fixedCommands = commands;
    const applied: string[] = [];

    for (const fix of FIXES) {
      if (ruleIds.includes(fix.ruleId)) {
        const before = JSON.stringify(fixedCommands);
        fixedCommands = fix.fix(fixedCommands);
        const after = JSON.stringify(fixedCommands);

        if (before !== after) {
          applied.push(fix.ruleId);
          logger.debug({ ruleId: fix.ruleId }, 'Applied fix');
        }
      }
    }

    // Reconstruct Dockerfile
    const fixed = reconstructDockerfile(fixedCommands);

    return { fixed, applied };
  } catch (error) {
    logger.error({ error }, 'Failed to apply fixes - returning original content');
    // Return original content if parsing fails - no fixes applied
    return { fixed: content, applied: [] };
  }
}

/**
 * Apply all available fixes to Dockerfile content
 */
export function applyAllFixes(content: string): FixResult {
  const allRuleIds = FIXES.map((f) => f.ruleId);
  return applyFixes(content, allRuleIds);
}

/**
 * Reconstruct Dockerfile from parsed commands
 */
function reconstructDockerfile(commands: CommandEntry[]): string {
  const lines: string[] = [];

  for (const cmd of commands) {
    let line = cmd.name;

    if (cmd.args) {
      if (typeof cmd.args === 'string') {
        line += ` ${cmd.args}`;
      } else if (Array.isArray(cmd.args)) {
        // JSON array format (for CMD, ENTRYPOINT, etc.) - add space after commas
        line += ` ${JSON.stringify(cmd.args).replace(/,/g, ', ')}`;
      } else if (typeof cmd.args === 'object') {
        // Object format (for ENV)
        const pairs = Object.entries(cmd.args).map(([k, v]) => {
          // Check if value needs quotes
          if (typeof v === 'string' && (v.includes(' ') || v.includes('"'))) {
            // Escape backslashes first, then double quotes
            const escaped = v.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
            return `${k}="${escaped}"`;
          }
          return `${k}=${v}`;
        });
        line += ` ${pairs.join(' ')}`;
      }
    }

    lines.push(line);
  }

  return lines.join('\n');
}

/**
 * Check if a fix is available for a given rule
 */
export function hasFixForRule(ruleId: string): boolean {
  return FIXES.some((fix) => fix.ruleId === ruleId);
}

/**
 * Get list of all fixable rules
 */
export function getFixableRules(): string[] {
  return FIXES.map((fix) => fix.ruleId);
}
