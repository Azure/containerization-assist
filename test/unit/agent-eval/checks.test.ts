import { beforeAll, describe, expect, it } from '@jest/globals';
import { promises as fs } from 'node:fs';
import { resolve } from 'node:path';
import {
  catalogCoversVersion,
  lookupMcrTag,
  type McrCatalog,
} from '../../agent-eval/mcr-catalog.js';

describe('agent-eval MCR validation', () => {
  let catalog: McrCatalog;

  beforeAll(async () => {
    const raw = await fs.readFile(
      resolve(process.cwd(), 'knowledge/catalogs/mcr-base-images.json'),
      'utf8',
    );
    const data = JSON.parse(raw) as { repos: Record<string, string[]> };
    catalog = new Map(Object.entries(data.repos).map(([repo, tags]) => [repo, new Set(tags)]));
  });

  it('accepts an exact published MCR tag', async () => {
    expect(lookupMcrTag(catalog, 'azurelinux/base/nodejs', '20')).toBe('present');
  });

  it('rejects an MCR repository absent from the catalog', async () => {
    expect(lookupMcrTag(catalog, 'fake/image', 'anything')).toBe('missing-repository');
  });

  it('rejects a fabricated patch tag instead of accepting its version family', async () => {
    expect(lookupMcrTag(catalog, 'dotnet/sdk', '8.0.999')).toBe('missing-tag');
  });

  it('recognizes Java major coverage through published distro variants', () => {
    expect(catalogCoversVersion(catalog, 'openjdk/jdk', '21')).toBe(true);
  });
});
