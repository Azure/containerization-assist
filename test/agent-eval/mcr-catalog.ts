export type McrCatalog = Map<string, Set<string>>;

export type McrTagLookup = 'present' | 'missing-repository' | 'missing-tag';

export function lookupMcrTag(catalog: McrCatalog, repo: string, tag: string): McrTagLookup {
  const tags = catalog.get(repo);
  if (!tags) return 'missing-repository';
  return tags.has(tag) ? 'present' : 'missing-tag';
}

export function catalogCoversVersion(catalog: McrCatalog, repo: string, version: string): boolean {
  const tags = catalog.get(repo);
  if (!tags) return false;
  for (const tag of tags) {
    if (tag === version || tag.startsWith(`${version}-`) || tag.startsWith(`${version}.`)) {
      return true;
    }
  }
  return false;
}
