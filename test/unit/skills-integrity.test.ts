import { describe, it, expect } from '@jest/globals';
import { readdirSync, readFileSync, statSync } from 'fs';
import { join } from 'path';
import { TOOL_NAME } from '@/tools/shared/toolDefinition';

/**
 * Skills Integrity Tests
 *
 * Skills under `skills/` are static markdown files contributed to VS Code chat
 * via the VAT extension. They are not executed in unit tests, but several
 * regressions are mechanically detectable:
 *
 *   - Folder name must equal frontmatter `name`. VS Code silently fails to
 *     register the skill otherwise.
 *   - Frontmatter must have `name` and `description`.
 *   - Every tool name referenced inside a SKILL.md (matched against the
 *     authoritative `TOOL_NAME` map) must still exist. Catches the case where
 *     a tool is renamed or deleted without updating the skill prose.
 *   - Cross-skill invocations must resolve to a real skill folder.
 */

const SKILLS_DIR = join(__dirname, '../../skills');

interface SkillFile {
  name: string;
  description: string;
  folder: string;
  body: string;
}

/**
 * Parse just `name:` and `description:` from frontmatter without a full YAML
 * parser. The descriptions contain free-form prose with `:` characters, which
 * a strict parser interprets as new mapping keys. Frontmatter here is always
 * a flat key/value block, one entry per line, so a regex is sufficient and
 * tolerant of prose.
 */
function parseFrontmatter(raw: string): { name: string; description: string } {
  const nameMatch = raw.match(/^name:\s*(.+?)\s*$/m);
  const descMatch = raw.match(/^description:\s*(.+?)\s*$/m);
  return {
    name: nameMatch?.[1] ?? '',
    description: descMatch?.[1] ?? '',
  };
}

function loadSkills(): SkillFile[] {
  const entries = readdirSync(SKILLS_DIR).filter((entry) => {
    return statSync(join(SKILLS_DIR, entry)).isDirectory();
  });

  return entries.map((folder) => {
    const skillPath = join(SKILLS_DIR, folder, 'SKILL.md');
    const raw = readFileSync(skillPath, 'utf-8');
    const match = raw.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n([\s\S]*)$/);
    if (!match) {
      throw new Error(`Skill ${folder} is missing YAML frontmatter`);
    }
    const fm = parseFrontmatter(match[1]);
    return {
      name: fm.name,
      description: fm.description,
      folder,
      body: match[2],
    };
  });
}

describe('Skills integrity', () => {
  const skills = loadSkills();
  const skillNames = new Set(skills.map((s) => s.name));
  const toolNames = new Set<string>(Object.values(TOOL_NAME));
  it('discovers at least one skill', () => {
    expect(skills.length).toBeGreaterThan(0);
  });

  describe.each(skills)('skill: $folder', (skill) => {
    it('has a non-empty `name` in frontmatter', () => {
      expect(skill.name.length).toBeGreaterThan(0);
    });

    it('has a non-empty `description` in frontmatter', () => {
      expect(skill.description.length).toBeGreaterThan(20);
    });

    it('folder name matches frontmatter `name`', () => {
      expect(skill.name).toBe(skill.folder);
    });

    it('references only known tool names', () => {
      // Only inspect backticked tokens that prose explicitly labels as a
      // "tool" — e.g. "`build-image-context` tool" or "**`scan-image`** tool".
      // This avoids false positives from unrelated kebab-case identifiers in
      // command snippets, table headings, etc.
      const re = /\*{0,2}`([a-z][a-z0-9-]+)`\*{0,2}\s+tool/gi;
      const referenced = new Set<string>();
      let m: RegExpExecArray | null;
      while ((m = re.exec(skill.body)) !== null) {
        referenced.add(m[1]);
      }

      const stale: string[] = [];
      for (const token of referenced) {
        if (!toolNames.has(token)) stale.push(token);
      }
      expect(stale).toEqual([]);
    });

    it('cross-skill invocations resolve to real skills', () => {
      // Match patterns like: `analyze-repo` skill / `generate-dockerfile` skill
      const re = /`([a-z][a-z0-9-]+)`\s+skill/gi;
      const referenced = new Set<string>();
      let m: RegExpExecArray | null;
      while ((m = re.exec(skill.body)) !== null) {
        referenced.add(m[1]);
      }
      for (const ref of referenced) {
        expect(skillNames.has(ref)).toBe(true);
      }
    });
  });
});

describe('Skills package manifest', () => {
  it('every skill folder is shipped via the package `files` glob', () => {
    const pkg = JSON.parse(
      readFileSync(join(__dirname, '../../package.json'), 'utf-8'),
    ) as { files?: string[] };
    expect(pkg.files).toBeDefined();
    expect(pkg.files).toEqual(
      expect.arrayContaining([expect.stringContaining('skills/')]),
    );
  });
});
