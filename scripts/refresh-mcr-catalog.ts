import { promises as fs } from 'node:fs';
import { fileURLToPath } from 'node:url';

const REPOS = [
  'openjdk/jdk',
  'azurelinux/base/nodejs',
  'azurelinux/distroless/nodejs',
  'azurelinux/base/python',
  'azurelinux/distroless/python',
  'dotnet/sdk',
  'dotnet/aspnet',
  'dotnet/runtime',
];

const CATALOG_DIR = new URL('../knowledge/catalogs/', import.meta.url);
const OUT = fileURLToPath(new URL('mcr-base-images.json', CATALOG_DIR));

function isNoiseTag(tag: string): boolean {
  if (/-(amd64|arm64(v8)?|arm32(v[0-9]+)?|ppc64le|s390x)$/i.test(tag)) return true;
  if (/(nightly|preview|-rc\.?\d|alpha|beta|-cbld|-cm[0-9])/i.test(tag)) return true;
  if (/^\d+\.\d+\.\d+/.test(tag)) return true;
  return false;
}

async function fetchTags(repo: string): Promise<string[]> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 15_000);
  try {
    const resp = await fetch(`https://mcr.microsoft.com/v2/${repo}/tags/list`, {
      signal: controller.signal,
    });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const body = (await resp.json()) as { tags?: string[] };
    return [...(body.tags ?? [])].filter((t) => !isNoiseTag(t)).sort();
  } finally {
    clearTimeout(timer);
  }
}

async function readPrevious(): Promise<Record<string, string[]>> {
  try {
    const raw = await fs.readFile(OUT, 'utf8');
    return (JSON.parse(raw) as { repos?: Record<string, string[]> }).repos ?? {};
  } catch {
    return {};
  }
}

async function main(): Promise<void> {
  const previous = await readPrevious();
  const repos: Record<string, string[]> = {};
  const failures: string[] = [];
  for (const repo of REPOS) {
    try {
      repos[repo] = await fetchTags(repo);
      console.log(`[mcr-catalog] ${repo}: ${repos[repo].length} tags`);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      failures.push(`${repo}: ${msg}`);
      if (previous[repo]) repos[repo] = previous[repo];
      console.error(`[mcr-catalog] FAILED ${repo}: ${msg}`);
    }
  }
  if (Object.keys(repos).length === 0) {
    throw new Error(`No MCR repos could be fetched:\n${failures.join('\n')}`);
  }
  const doc = {
    source: 'https://mcr.microsoft.com/v2/<repo>/tags/list',
    repos,
  };
  await fs.mkdir(fileURLToPath(CATALOG_DIR), { recursive: true });
  await fs.writeFile(OUT, `${JSON.stringify(doc, null, 2)}\n`, 'utf8');
  console.log(`[mcr-catalog] wrote ${OUT} (${Object.keys(repos).length} repos)`);
  if (failures.length) {
    console.error(`[mcr-catalog] ${failures.length} repo(s) failed; kept previous entries for those.`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
